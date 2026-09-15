package auth

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

func TestHandlerRegister(t *testing.T) {
	handler := newTestHandler(t)

	tests := []struct {
		name       string
		payload    interface{}
		wantStatus int
	}{
		{
			name: "valid registration",
			payload: map[string]any{
				"name":     "newuser",
				"email":    "newuser@example.com",
				"password": "password123",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing name",
			payload: map[string]any{
				"email":    "test@example.com",
				"password": "password123",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing email",
			payload: map[string]any{
				"name":     "newuser",
				"password": "password123",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing password",
			payload: map[string]any{
				"name":  "newuser",
				"email": "test@example.com",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "password too short",
			payload: map[string]any{
				"name":     "newuser",
				"email":    "test@example.com",
				"password": "short",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "duplicate email",
			payload: map[string]any{
				"name":     "dupuser",
				"email":    "dupuser@example.com",
				"password": "password123",
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
			w := httptest.NewRecorder()
			handler.Register(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandlerLogin(t *testing.T) {
	handler := newTestHandler(t)

	body, _ := json.Marshal(map[string]any{
		"name":     "loginuser",
		"email":    "loginuser@example.com",
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.Register(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup registration failed: got %d, want %d", w.Code, http.StatusCreated)
	}

	tests := []struct {
		name       string
		payload    interface{}
		wantStatus int
	}{
		{
			name: "valid login",
			payload: map[string]any{
				"email":    "loginuser@example.com",
				"password": "password123",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid password",
			payload: map[string]any{
				"email":    "loginuser@example.com",
				"password": "wrongpassword",
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "nonexistent user",
			payload: map[string]any{
				"email":    "nobody@example.com",
				"password": "password123",
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "missing email",
			payload: map[string]any{
				"password": "password123",
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
			w := httptest.NewRecorder()
			handler.Login(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}
