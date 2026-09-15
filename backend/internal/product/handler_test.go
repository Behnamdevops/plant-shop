package product

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

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
	return NewHandler(NewRepository(db))
}

func TestHandlerCreate(t *testing.T) {
	handler := newTestHandler(t)

	tests := []struct {
		name       string
		payload    interface{}
		wantStatus int
	}{
		{
			name: "valid product",
			payload: map[string]any{
				"name":  "Test Plant",
				"slug":  "test-plant",
				"price": 100,
				"stock": 5,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing name",
			payload: map[string]any{
				"slug":  "no-name",
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
				"slug":  "neg-price",
				"price": -1,
				"stock": 5,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "negative stock",
			payload: map[string]any{
				"name":  "Negative Stock",
				"slug":  "neg-stock",
				"price": 100,
				"stock": -1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "duplicate slug",
			payload: map[string]any{
				"name":  "Duplicate Plant",
				"slug":  "test-plant",
				"price": 200,
				"stock": 1,
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader(body))
			w := httptest.NewRecorder()

			handler.Create(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandlerUpdate(t *testing.T) {
	handler := newTestHandler(t)

	// First create a product to update
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader([]byte(`{"name":"Original","slug":"original-slug","price":100,"stock":5}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.Create(createRec, createReq)

	var created Product
	json.NewDecoder(createRec.Body).Decode(&created)

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
				"slug":        "updated-slug",
				"description": "New description",
				"price":       200,
				"stock":       10,
				"image_url":   nil,
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid id (non-numeric)",
			id:         "abc",
			payload:    map[string]any{"name": "Test", "slug": "test", "price": 100, "stock": 1},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid id (zero)",
			id:         "0",
			payload:    map[string]any{"name": "Test", "slug": "test", "price": 100, "stock": 1},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "empty name",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":  "",
				"slug":  "updated-slug",
				"price": 100,
				"stock": 1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "whitespace-only name",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":  "   ",
				"slug":  "updated-slug",
				"price": 100,
				"stock": 1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "empty slug",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":  "Updated",
				"slug":  "",
				"price": 100,
				"stock": 1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "whitespace-only slug",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":  "Updated",
				"slug":  "   ",
				"price": 100,
				"stock": 1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "negative price",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":  "Updated",
				"slug":  "updated-slug",
				"price": -1,
				"stock": 1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "negative stock",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":  "Updated",
				"slug":  "updated-slug",
				"price": 100,
				"stock": -1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "duplicate slug",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":  "Updated",
				"slug":  "original-slug",
				"price": 100,
				"stock": 1,
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "not found",
			id:   "999999",
			payload: map[string]any{
				"name":        "Updated",
				"slug":        "updated-slug",
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
				"slug":  "updated-slug",
				"price": 100,
				"stock": 1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing slug",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":  "Updated",
				"price": 100,
				"stock": 1,
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing description",
			id:   strconv.FormatInt(created.ID, 10),
			payload: map[string]any{
				"name":  "Updated",
				"slug":  "updated-slug",
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
				"slug":        "updated-slug",
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
				"slug":        "updated-slug",
				"description": "New description",
				"price":       100,
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/products/"+tt.id, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Update(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}
