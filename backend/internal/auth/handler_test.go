package auth

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

func uniqueHandlerEmail(prefix string) string {
	return prefix + "-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.com"
}

func cleanupHandlerUser(t *testing.T, handler *Handler, email string) {
	t.Helper()

	t.Cleanup(func() {
		ctx := context.Background()

		// Register/Login may create sessions, so remove them first.
		if _, err := handler.repository.db.Exec(
			ctx,
			`DELETE FROM sessions
			 WHERE user_id IN (
				SELECT id FROM users WHERE email = $1
			 )`,
			email,
		); err != nil {
			t.Errorf("cleanup sessions for %s: %v", email, err)
		}

		if _, err := handler.repository.db.Exec(
			ctx,
			"DELETE FROM users WHERE email = $1",
			email,
		); err != nil {
			t.Errorf("cleanup user %s: %v", email, err)
		}
	})
}

func TestHandlerRegister(t *testing.T) {
	handler := newTestHandler(t)

	validEmail := uniqueHandlerEmail("newuser")
	duplicateEmail := uniqueHandlerEmail("dupuser")

	cleanupHandlerUser(t, handler, validEmail)
	cleanupHandlerUser(t, handler, duplicateEmail)

	// Seed one user specifically for the duplicate-email case.
	setupBody, err := json.Marshal(map[string]any{
		"name":     "existing-dup-user",
		"email":    duplicateEmail,
		"password": "password123",
	})
	if err != nil {
		t.Fatalf("marshal duplicate setup payload: %v", err)
	}

	setupReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/register",
		bytes.NewReader(setupBody),
	)
	setupReq.Header.Set("Content-Type", "application/json")

	setupRec := httptest.NewRecorder()
	handler.Register(setupRec, setupReq)

	if setupRec.Code != http.StatusCreated {
		t.Fatalf(
			"duplicate test setup registration failed: got %d, want %d; body=%s",
			setupRec.Code,
			http.StatusCreated,
			setupRec.Body.String(),
		)
	}

	tests := []struct {
		name       string
		payload    interface{}
		wantStatus int
	}{
		{
			name: "valid registration",
			payload: map[string]any{
				"name":     "newuser",
				"email":    validEmail,
				"password": "password123",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing name",
			payload: map[string]any{
				"email":    uniqueHandlerEmail("missing-name"),
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
				"email": uniqueHandlerEmail("missing-password"),
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "password too short",
			payload: map[string]any{
				"name":     "newuser",
				"email":    uniqueHandlerEmail("short-password"),
				"password": "short",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "duplicate email",
			payload: map[string]any{
				"name":     "dupuser",
				"email":    duplicateEmail,
				"password": "password123",
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
				"/api/v1/auth/register",
				bytes.NewReader(body),
			)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			handler.Register(w, req)

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

func TestHandlerLogin(t *testing.T) {
	handler := newTestHandler(t)

	loginEmail := uniqueHandlerEmail("loginuser")
	nonexistentEmail := uniqueHandlerEmail("nobody")

	cleanupHandlerUser(t, handler, loginEmail)

	// Create a real user for the successful and invalid-password login cases.
	registerBody, err := json.Marshal(map[string]any{
		"name":     "loginuser",
		"email":    loginEmail,
		"password": "password123",
	})
	if err != nil {
		t.Fatalf("marshal registration payload: %v", err)
	}

	registerReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/register",
		bytes.NewReader(registerBody),
	)
	registerReq.Header.Set("Content-Type", "application/json")

	registerRec := httptest.NewRecorder()
	handler.Register(registerRec, registerReq)

	if registerRec.Code != http.StatusCreated {
		t.Fatalf(
			"setup registration failed: got %d, want %d; body=%s",
			registerRec.Code,
			http.StatusCreated,
			registerRec.Body.String(),
		)
	}

	tests := []struct {
		name       string
		payload    interface{}
		wantStatus int
	}{
		{
			name: "valid login",
			payload: map[string]any{
				"email":    loginEmail,
				"password": "password123",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid password",
			payload: map[string]any{
				"email":    loginEmail,
				"password": "wrongpassword",
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "nonexistent user",
			payload: map[string]any{
				"email":    nonexistentEmail,
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
			body, err := json.Marshal(tt.payload)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}

			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/auth/login",
				bytes.NewReader(body),
			)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			handler.Login(w, req)

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
