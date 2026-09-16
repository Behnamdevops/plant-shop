package product

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestHandler(t *testing.T) *Handler {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	db, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Skip("cannot connect to database; skipping integration test: ", err)
	}

	t.Cleanup(db.Close)

	return NewHandler(NewRepository(db))
}

func TestHandlerCreate(t *testing.T) {
	handler := newTestHandler(t)

	runID := strconv.FormatInt(time.Now().UnixNano(), 10)
	testSlug := "test-plant-" + runID

	// The valid-product test creates this row.
	// Keep it around until all subtests finish because the duplicate-slug
	// case intentionally depends on it.
	t.Cleanup(func() {
		_, err := handler.repository.db.Exec(
			context.Background(),
			"DELETE FROM products WHERE slug = $1",
			testSlug,
		)
		if err != nil {
			t.Errorf("cleanup product: %v", err)
		}
	})

	tests := []struct {
		name       string
		payload    interface{}
		wantStatus int
	}{
		{
			name: "valid product",
			payload: map[string]any{
				"name":  "Test Plant",
				"slug":  testSlug,
				"price": 100,
				"stock": 5,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing name",
			payload: map[string]any{
				"slug":  "no-name-" + runID,
				"price": 100,
				"stock": 5,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing slug",
			payload: map[string]any{
				"name":  "No Slug",
				"price": 100,
				"stock": 5,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "negative price",
			payload: map[string]any{
				"name":  "Negative Price",
				"slug":  "neg-price-" + runID,
				"price": -1,
				"stock": 5,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "negative stock",
			payload: map[string]any{
				"name":  "Negative Stock",
				"slug":  "neg-stock-" + runID,
				"price": 100,
				"stock": -1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "duplicate slug",
			payload: map[string]any{
				"name":  "Duplicate Plant",
				"slug":  testSlug,
				"price": 200,
				"stock": 1,
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.payload)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}

			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/admin/products",
				bytes.NewReader(body),
			)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			handler.Create(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf(
					"got status %d, want %d; body=%s",
					w.Code,
					tt.wantStatus,
					w.Body.String(),
				)
			}
		})
	}
}

func TestHandlerUpdate(t *testing.T) {
	handler := newTestHandler(t)

	runID := strconv.FormatInt(time.Now().UnixNano(), 10)

	originalSlug := "original-slug-" + runID
	updatedSlug := "updated-slug-" + runID
	conflictSlug := "conflict-slug-" + runID
	notFoundSlug := "not-found-slug-" + runID

	// Create the product that will be updated.
	createBody, err := json.Marshal(map[string]any{
		"name":  "Original",
		"slug":  originalSlug,
		"price": 100,
		"stock": 5,
	})
	if err != nil {
		t.Fatalf("marshal setup product: %v", err)
	}

	createReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/products",
		bytes.NewReader(createBody),
	)
	createReq.Header.Set("Content-Type", "application/json")

	createRec := httptest.NewRecorder()
	handler.Create(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf(
			"setup Create failed: got status %d, want %d; body=%s",
			createRec.Code,
			http.StatusCreated,
			createRec.Body.String(),
		)
	}

	var created Product
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode created product: %v", err)
	}

	t.Cleanup(func() {
		_, err := handler.repository.db.Exec(
			context.Background(),
			"DELETE FROM products WHERE id = $1",
			created.ID,
		)
		if err != nil {
			t.Errorf("cleanup created product: %v", err)
		}
	})

	// Create a separate product whose slug will be used to test
	// the duplicate-slug update case.
	conflictBody, err := json.Marshal(map[string]any{
		"name":  "Conflict",
		"slug":  conflictSlug,
		"price": 100,
		"stock": 5,
	})
	if err != nil {
		t.Fatalf("marshal conflict product: %v", err)
	}

	conflictReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/products",
		bytes.NewReader(conflictBody),
	)
	conflictReq.Header.Set("Content-Type", "application/json")

	conflictRec := httptest.NewRecorder()
	handler.Create(conflictRec, conflictReq)

	if conflictRec.Code != http.StatusCreated {
		t.Fatalf(
			"setup conflict Create failed: got status %d, want %d; body=%s",
			conflictRec.Code,
			http.StatusCreated,
			conflictRec.Body.String(),
		)
	}

	var conflictProduct Product
	if err := json.NewDecoder(conflictRec.Body).Decode(&conflictProduct); err != nil {
		t.Fatalf("decode conflict product: %v", err)
	}

	t.Cleanup(func() {
		_, err := handler.repository.db.Exec(
			context.Background(),
			"DELETE FROM products WHERE id = $1",
			conflictProduct.ID,
		)
		if err != nil {
			t.Errorf("cleanup conflict product: %v", err)
		}
	})

	tests := []struct {
		name       string
		id         string
		payload    interface{}
		wantStatus int
	}{
		{
			name: "valid update",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":        "Updated",
				"slug":        updatedSlug,
				"description": "New description",
				"price":       200,
				"stock":       10,
				"image_url":   nil,
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid id (non-numeric)",
			id:   "abc",
			payload: map[string]any{
				"name":        "Test",
				"slug":        "invalid-id-" + runID,
				"description": "description",
				"price":       100,
				"stock":       1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid id (zero)",
			id:   "0",
			payload: map[string]any{
				"name":        "Test",
				"slug":        "zero-id-" + runID,
				"description": "description",
				"price":       100,
				"stock":       1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "empty name",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":        "",
				"slug":        updatedSlug,
				"description": "description",
				"price":       100,
				"stock":       1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "whitespace-only name",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":        "   ",
				"slug":        updatedSlug,
				"description": "description",
				"price":       100,
				"stock":       1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "empty slug",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":        "Updated",
				"slug":        "",
				"description": "description",
				"price":       100,
				"stock":       1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "whitespace-only slug",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":        "Updated",
				"slug":        "   ",
				"description": "description",
				"price":       100,
				"stock":       1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "negative price",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":        "Updated",
				"slug":        updatedSlug,
				"description": "description",
				"price":       -1,
				"stock":       1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "negative stock",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":        "Updated",
				"slug":        updatedSlug,
				"description": "description",
				"price":       100,
				"stock":       -1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "duplicate slug",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":        "Updated",
				"slug":        conflictSlug,
				"description": "description",
				"price":       100,
				"stock":       1,
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "not found",
			id:   "999999999999",
			payload: map[string]any{
				"name":        "Updated",
				"slug":        notFoundSlug,
				"description": "New description",
				"price":       100,
				"stock":       1,
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name: "missing name",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"slug":        updatedSlug,
				"description": "New description",
				"price":       100,
				"stock":       1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing slug",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":        "Updated",
				"description": "New description",
				"price":       100,
				"stock":       1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing description",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":  "Updated",
				"slug":  updatedSlug,
				"price": 100,
				"stock": 1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing price",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":        "Updated",
				"slug":        updatedSlug,
				"description": "New description",
				"stock":       1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing stock",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":        "Updated",
				"slug":        updatedSlug,
				"description": "New description",
				"price":       100,
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.payload)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}

			req := httptest.NewRequest(
				http.MethodPut,
				"/api/v1/admin/products/"+tt.id,
				bytes.NewReader(body),
			)
			req.SetPathValue("id", tt.id)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			handler.Update(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf(
					"got status %d, want %d; body=%s",
					w.Code,
					tt.wantStatus,
					w.Body.String(),
				)
			}
		})
	}
}
