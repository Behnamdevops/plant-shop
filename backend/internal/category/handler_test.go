package category

import (
	"bytes"
	"context"
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

var handlerSeq int

func handlerUniqueSuffix() string {
	handlerSeq++
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), handlerSeq)
}

func (e *testHandlerEnv) registerAndLogin(t *testing.T, role string) *http.Cookie {
	t.Helper()
	suffix := handlerUniqueSuffix()
	email := "catuser-" + suffix + "@example.com"
	body, _ := json.Marshal(map[string]any{
		"name":     "catuser-" + suffix,
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

func (e *testHandlerEnv) createCategory(t *testing.T, slug string) Category {
	t.Helper()
	c, err := e.handler.repository.Create(t.Context(), CreateCategoryInput{
		Name: "Handler Test Category " + slug,
		Slug: slug,
	})
	if err != nil {
		t.Fatalf("setup create category failed: %v", err)
	}
	t.Cleanup(func() {
		e.db.Exec(context.Background(), "DELETE FROM categories WHERE id = $1", c.ID)
	})
	return c
}

// ---------- Public List ----------

func TestHandlerListPublic(t *testing.T) {
	env := newTestHandlerEnv(t)
	c := env.createCategory(t, "list-public-"+handlerUniqueSuffix())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	w := httptest.NewRecorder()
	env.handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}
	var got []Category
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	found := false
	for _, cat := range got {
		if cat.ID == c.ID {
			found = true
		}
	}
	if !found {
		t.Error("expected created category to be present in public list")
	}
}

// ---------- Create ----------

func TestHandlerCreateUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	body, _ := json.Marshal(map[string]any{"name": "X", "slug": "x-" + handlerUniqueSuffix()})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/categories", bytes.NewReader(body))
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerCreateNonAdminForbidden(t *testing.T) {
	env := newTestHandlerEnv(t)
	user := env.registerAndLogin(t, auth.RoleUser)
	body, _ := json.Marshal(map[string]any{"name": "X", "slug": "x-" + handlerUniqueSuffix()})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/categories", bytes.NewReader(body))
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
	slug := "admin-create-cat-" + handlerUniqueSuffix()
	body, _ := json.Marshal(map[string]any{"name": "Admin Created", "slug": slug})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/categories", bytes.NewReader(body))
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusCreated, w.Body.String())
	}
	var c Category
	json.NewDecoder(w.Body).Decode(&c)
	env.db.Exec(t.Context(), "DELETE FROM categories WHERE id = $1", c.ID)
}

func TestHandlerCreateDuplicateSlugConflict(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)
	slug := "dup-handler-cat-" + handlerUniqueSuffix()
	existing := env.createCategory(t, slug)
	_ = existing

	body, _ := json.Marshal(map[string]any{"name": "Duplicate", "slug": slug})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/categories", bytes.NewReader(body))
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusConflict, w.Body.String())
	}
}

func TestHandlerCreateMissingFields(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)

	tests := []struct {
		name    string
		payload map[string]any
	}{
		{"missing name", map[string]any{"slug": "no-name-" + handlerUniqueSuffix()}},
		{"missing slug", map[string]any{"name": "No Slug"}},
		{"whitespace name", map[string]any{"name": "   ", "slug": "ws-name-" + handlerUniqueSuffix()}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/categories", bytes.NewReader(body))
			req.AddCookie(admin)
			w := httptest.NewRecorder()
			env.handler.Create(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
			}
		})
	}
}

// ---------- Update ----------

func TestHandlerUpdateAdminSuccess(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)
	c := env.createCategory(t, "upd-cat-"+handlerUniqueSuffix())

	body, _ := json.Marshal(map[string]any{"name": "Updated", "slug": c.Slug})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/categories/"+strconv.FormatInt(c.ID, 10), bytes.NewReader(body))
	req.SetPathValue("id", strconv.FormatInt(c.ID, 10))
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Update(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandlerUpdateNotFound(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)

	body, _ := json.Marshal(map[string]any{"name": "X", "slug": "nf-" + handlerUniqueSuffix()})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/categories/999999999", bytes.NewReader(body))
	req.SetPathValue("id", "999999999")
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Update(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

// ---------- Delete ----------

func TestHandlerDeleteUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/categories/1", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	env.handler.Delete(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerDeleteAdminSuccess(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)
	c := env.createCategory(t, "del-cat-"+handlerUniqueSuffix())

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/categories/"+strconv.FormatInt(c.ID, 10), nil)
	req.SetPathValue("id", strconv.FormatInt(c.ID, 10))
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Delete(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusNoContent, w.Body.String())
	}
}

func TestHandlerDeleteReferencedByProductConflict(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin := env.registerAndLogin(t, auth.RoleAdmin)
	c := env.createCategory(t, "del-ref-cat-"+handlerUniqueSuffix())

	var productID int64
	err := env.db.QueryRow(t.Context(), `
		INSERT INTO products (name, slug, price, stock, category_id)
		VALUES ($1, $2, 100, 1, $3)
		RETURNING id
	`, "Ref Product "+handlerUniqueSuffix(), "ref-product-"+handlerUniqueSuffix(), c.ID).Scan(&productID)
	if err != nil {
		t.Fatalf("seed product failed: %v", err)
	}
	t.Cleanup(func() {
		env.db.Exec(context.Background(), "DELETE FROM products WHERE id = $1", productID)
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/categories/"+strconv.FormatInt(c.ID, 10), nil)
	req.SetPathValue("id", strconv.FormatInt(c.ID, 10))
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.Delete(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusConflict, w.Body.String())
	}
}

// ---------- AdminList authorization ----------

func TestHandlerAdminListNonAdminForbidden(t *testing.T) {
	env := newTestHandlerEnv(t)
	user := env.registerAndLogin(t, auth.RoleUser)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/categories", nil)
	req.AddCookie(user)
	w := httptest.NewRecorder()
	env.handler.AdminList(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusForbidden)
	}
}
