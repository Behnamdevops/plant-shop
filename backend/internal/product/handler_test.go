package product

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
