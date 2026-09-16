package product

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

// sha256HashForTest mirrors auth's internal session-token hashing so tests
// can look up a session's user by the cookie value returned from register.
func sha256HashForTest(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

type testHandlerEnv struct {
	handler     *Handler
	authHandler *auth.Handler
	db          *pgxpool.Pool
}

func newTestHandlerEnv(t *testing.T) *testHandlerEnv {
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

	authHandler := auth.NewHandler(auth.NewRepository(db))
	handler := NewHandler(NewRepository(db), authHandler)

	return &testHandlerEnv{handler: handler, authHandler: authHandler, db: db}
}

// Kept for tests that only need a bare handler without auth flows.
func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	return newTestHandlerEnv(t).handler
}

var handlerSeq int

func handlerUniqueSuffix() string {
	handlerSeq++
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), handlerSeq)
}

// registerAndLogin creates a new user with the given role via the real auth
// handler/repository and returns the session cookie so admin requests
// authenticate the same way a real client would (role is never trusted from
// the request itself).
func (e *testHandlerEnv) registerAndLogin(t *testing.T, role string) *http.Cookie {
	t.Helper()
	suffix := handlerUniqueSuffix()
	email := "produser-" + suffix + "@example.com"
	body, _ := json.Marshal(map[string]any{
		"name":     "produser-" + suffix,
		"email":    email,
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	e.authHandler.Register(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup registration failed: got %d, body=%s", w.Code, w.Body.String())
	}

	if role == auth.RoleAdmin {
		if _, err := e.db.Exec(t.Context(), `UPDATE users SET role = $1 WHERE email = $2`, auth.RoleAdmin, email); err != nil {
			t.Fatalf("failed to promote test user to admin: %v", err)
		}
	}

	t.Cleanup(func() {
		e.db.Exec(context.Background(), `DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE email = $1)`, email)
		e.db.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, email)
	})

	res := w.Result()
	for _, c := range res.Cookies() {
		if c.Name == "session_token" {
			return c
		}
	}
	t.Fatal("no session_token cookie set on registration")
	return nil
}

func (e *testHandlerEnv) createProduct(t *testing.T, slug string) Product {
	t.Helper()
	p, err := e.handler.repository.Create(t.Context(), CreateProductInput{
		Name:  "Handler Test Product " + slug,
		Slug:  slug,
		Price: 100,
		Stock: 5,
	})
	if err != nil {
		t.Fatalf("setup create product failed: %v", err)
	}
	t.Cleanup(func() {
		e.db.Exec(context.Background(), "DELETE FROM products WHERE id = $1", p.ID)
	})
	return p
}

func TestHandlerCreate(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)

	runID := strconv.FormatInt(time.Now().UnixNano(), 10)
	testSlug := "test-plant-" + runID

	// The valid-product test creates this row.
	// Keep it around until all subtests finish because the duplicate-slug
	// case intentionally depends on it.
	t.Cleanup(func() {
		_, err := env.handler.repository.db.Exec(
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
			req.AddCookie(admin)

			w := httptest.NewRecorder()

			env.handler.Create(w, req)

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
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)

	runID := strconv.FormatInt(time.Now().UnixNano(), 10)

	originalSlug := "original-slug-" + runID
	updatedSlug := "updated-slug-" + runID
	conflictSlug := "conflict-slug-" + runID
	notFoundSlug := "not-found-slug-" + runID

	created := env.createProduct(t, originalSlug)
	conflictProduct := env.createProduct(t, conflictSlug)

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
			req.AddCookie(admin)

			w := httptest.NewRecorder()

			env.handler.Update(w, req)

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

	_ = conflictProduct
}

// ---------- Admin authorization ----------

func TestHandlerCreateUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	body, _ := json.Marshal(map[string]any{"name": "X", "slug": "x-" + handlerUniqueSuffix(), "price": 1, "stock": 1})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader(body))
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerCreateNonAdminForbidden(t *testing.T) {
	env := newTestHandlerEnv(t)
	user := env.registerAndLogin(t, auth.RoleUser)
	body, _ := json.Marshal(map[string]any{"name": "X", "slug": "x-" + handlerUniqueSuffix(), "price": 1, "stock": 1})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader(body))
	req.AddCookie(user)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandlerCreateAdminSuccess(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)
	slug := "admin-create-" + handlerUniqueSuffix()
	body, _ := json.Marshal(map[string]any{"name": "Admin Created", "slug": slug, "price": 100, "stock": 1})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader(body))
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusCreated, w.Body.String())
	}
	var p Product
	json.NewDecoder(w.Body).Decode(&p)
	env.db.Exec(t.Context(), "DELETE FROM products WHERE id = $1", p.ID)
}

func TestHandlerUpdateUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	p := env.createProduct(t, "upd-unauth-"+handlerUniqueSuffix())
	body, _ := json.Marshal(map[string]any{"name": "X", "slug": p.Slug, "description": "d", "price": 1, "stock": 1})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/products/"+strconv.FormatInt(p.ID, 10), bytes.NewReader(body))
	req.SetPathValue("id", strconv.FormatInt(p.ID, 10))
	w := httptest.NewRecorder()
	env.handler.Update(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerUpdateNonAdminForbidden(t *testing.T) {
	env := newTestHandlerEnv(t)
	user := env.registerAndLogin(t, auth.RoleUser)
	p := env.createProduct(t, "upd-forbidden-"+handlerUniqueSuffix())
	body, _ := json.Marshal(map[string]any{"name": "X", "slug": p.Slug, "description": "d", "price": 1, "stock": 1})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/products/"+strconv.FormatInt(p.ID, 10), bytes.NewReader(body))
	req.SetPathValue("id", strconv.FormatInt(p.ID, 10))
	req.AddCookie(user)
	w := httptest.NewRecorder()
	env.handler.Update(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandlerUpdateAdminSuccess(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)
	p := env.createProduct(t, "upd-admin-"+handlerUniqueSuffix())
	body, _ := json.Marshal(map[string]any{"name": "Updated", "slug": p.Slug, "description": "d", "price": 999, "stock": 2})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/products/"+strconv.FormatInt(p.ID, 10), bytes.NewReader(body))
	req.SetPathValue("id", strconv.FormatInt(p.ID, 10))
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Update(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

// ---------- GetByID ----------

func TestHandlerGetByIDUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/products/1", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	env.handler.GetByID(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerGetByIDNonAdminForbidden(t *testing.T) {
	env := newTestHandlerEnv(t)
	user := env.registerAndLogin(t, auth.RoleUser)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/products/1", nil)
	req.SetPathValue("id", "1")
	req.AddCookie(user)
	w := httptest.NewRecorder()
	env.handler.GetByID(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandlerGetByIDAdminSuccess(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)
	p := env.createProduct(t, "get-admin-"+handlerUniqueSuffix())
	idStr := strconv.FormatInt(p.ID, 10)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/products/"+idStr, nil)
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.GetByID(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	var got Product
	json.NewDecoder(w.Body).Decode(&got)
	if got.ID != p.ID {
		t.Errorf("expected id %d, got %d", p.ID, got.ID)
	}
}

func TestHandlerGetByIDInvalidID(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/products/abc", nil)
	req.SetPathValue("id", "abc")
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.GetByID(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandlerGetByIDNotFound(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/products/999999999999", nil)
	req.SetPathValue("id", "999999999999")
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.GetByID(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

// ---------- Delete ----------

func TestHandlerDeleteUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/products/1", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	env.handler.Delete(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerDeleteNonAdminForbidden(t *testing.T) {
	env := newTestHandlerEnv(t)
	user := env.registerAndLogin(t, auth.RoleUser)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/products/1", nil)
	req.SetPathValue("id", "1")
	req.AddCookie(user)
	w := httptest.NewRecorder()
	env.handler.Delete(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandlerDeleteAdminSuccess(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)
	p := env.createProduct(t, "del-admin-"+handlerUniqueSuffix())
	idStr := strconv.FormatInt(p.ID, 10)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/products/"+idStr, nil)
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Delete(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusNoContent, w.Body.String())
	}

	// Confirm it's actually gone.
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/products/"+idStr, nil)
	getReq.SetPathValue("id", idStr)
	getReq.AddCookie(admin)
	getW := httptest.NewRecorder()
	env.handler.GetByID(getW, getReq)
	if getW.Code != http.StatusNotFound {
		t.Fatalf("expected deleted product to 404, got %d", getW.Code)
	}
}

func TestHandlerDeleteInvalidID(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/products/abc", nil)
	req.SetPathValue("id", "abc")
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Delete(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandlerDeleteNotFound(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/products/999999999999", nil)
	req.SetPathValue("id", "999999999999")
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Delete(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandlerDeleteReferencedByOrderReturnsConflict(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)
	buyer := env.registerAndLogin(t, auth.RoleUser)
	p := env.createProduct(t, "del-referenced-"+handlerUniqueSuffix())

	// Seed a historical order directly via SQL (order_items.product_id has
	// no ON DELETE clause, i.e. RESTRICT, so this simulates a product that
	// was purchased in the past and must never be silently removed). Look
	// up the buyer's user ID via their session cookie to avoid depending on
	// insertion order of other test data.
	buyerTokenHash := sha256HashForTest(buyer.Value)
	var userID int64
	if err := env.db.QueryRow(t.Context(), `SELECT user_id FROM sessions WHERE token_hash = $1`, buyerTokenHash).Scan(&userID); err != nil {
		t.Fatalf("failed to look up buyer id: %v", err)
	}

	var orderID int64
	if err := env.db.QueryRow(t.Context(), `
		INSERT INTO orders (user_id, status, total) VALUES ($1, 'pending', $2) RETURNING id
	`, userID, p.Price).Scan(&orderID); err != nil {
		t.Fatalf("failed to seed order: %v", err)
	}
	t.Cleanup(func() {
		env.db.Exec(context.Background(), `DELETE FROM orders WHERE id = $1`, orderID)
	})

	if _, err := env.db.Exec(t.Context(), `
		INSERT INTO order_items (order_id, product_id, product_name, product_slug, unit_price, quantity, subtotal)
		VALUES ($1, $2, $3, $4, $5, 1, $5)
	`, orderID, p.ID, p.Name, p.Slug, p.Price); err != nil {
		t.Fatalf("failed to seed order item: %v", err)
	}

	idStr := strconv.FormatInt(p.ID, 10)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/products/"+idStr, nil)
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Delete(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusConflict, w.Body.String())
	}

	// Historical data must remain intact: the product, order, and order
	// item must still exist.
	var stillExists bool
	env.db.QueryRow(t.Context(), `SELECT EXISTS(SELECT 1 FROM products WHERE id = $1)`, p.ID).Scan(&stillExists)
	if !stillExists {
		t.Error("expected product to still exist after failed delete")
	}
	env.db.QueryRow(t.Context(), `SELECT EXISTS(SELECT 1 FROM order_items WHERE order_id = $1)`, orderID).Scan(&stillExists)
	if !stillExists {
		t.Error("expected order_items to still exist after failed delete")
	}
}
