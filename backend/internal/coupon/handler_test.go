package coupon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/Behnamdevops/plant-shop/backend/internal/cart"
	"github.com/Behnamdevops/plant-shop/backend/internal/product"
	"github.com/jackc/pgx/v5/pgxpool"
)

type testHandlerEnv struct {
	handler     *Handler
	authHandler *auth.Handler
	cartRepo    *cart.Repository
	productRepo *product.Repository
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
	cartRepo := cart.NewRepository(db)
	handler := NewHandler(NewRepository(db), cartRepo, authHandler)

	return &testHandlerEnv{
		handler:     handler,
		authHandler: authHandler,
		cartRepo:    cartRepo,
		productRepo: product.NewRepository(db),
		db:          db,
	}
}

func (e *testHandlerEnv) registerAndLogin(t *testing.T, role string) (*http.Cookie, int64) {
	t.Helper()
	suffix := uniqueSuffix()
	email := "couponuser-" + suffix + "@example.com"
	body, _ := json.Marshal(map[string]any{
		"name":     "couponuser-" + suffix,
		"email":    email,
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	e.authHandler.Register(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup registration failed: got %d, body=%s", w.Code, w.Body.String())
	}

	var user struct{ ID int64 }
	json.NewDecoder(w.Body).Decode(&user)

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
			return c, user.ID
		}
	}
	t.Fatal("no session_token cookie set on registration")
	return nil, 0
}

func (e *testHandlerEnv) createProduct(t *testing.T, price int64, stock int) product.Product {
	t.Helper()
	suffix := uniqueSuffix()
	p, err := e.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Coupon Test Product " + suffix,
		Slug:  "coupon-test-product-" + suffix,
		Price: price,
		Stock: stock,
	})
	if err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	t.Cleanup(func() {
		e.db.Exec(context.Background(), "DELETE FROM products WHERE id = $1", p.ID)
	})
	return p
}

// ---------- Preview ----------

func TestHandlerPreviewUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	body, _ := json.Marshal(map[string]any{"code": "X"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/coupons/preview", bytes.NewReader(body))
	w := httptest.NewRecorder()
	env.handler.Preview(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerPreviewEmptyCart(t *testing.T) {
	env := newTestHandlerEnv(t)
	userCookie, _ := env.registerAndLogin(t, auth.RoleUser)

	body, _ := json.Marshal(map[string]any{"code": "X"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/coupons/preview", bytes.NewReader(body))
	req.AddCookie(userCookie)
	w := httptest.NewRecorder()
	env.handler.Preview(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandlerPreviewSuccess(t *testing.T) {
	env := newTestHandlerEnv(t)
	userCookie, userID := env.registerAndLogin(t, auth.RoleUser)
	p := env.createProduct(t, 1_000_000, 10)
	if _, err := env.cartRepo.AddItem(context.Background(), userID, p.ID, 2); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	code := uniqueCode("PREVIEWOK")
	repo := env.handler.repository
	repo.Create(context.Background(), CreateInput{Code: code, DiscountType: DiscountTypePercent, Value: 10, IsActive: boolPtr(true)})

	body, _ := json.Marshal(map[string]any{"code": code})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/coupons/preview", bytes.NewReader(body))
	req.AddCookie(userCookie)
	w := httptest.NewRecorder()
	env.handler.Preview(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var preview Preview
	if err := json.NewDecoder(w.Body).Decode(&preview); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if preview.ItemsSubtotal != 2_000_000 {
		t.Errorf("expected items_subtotal 2000000, got %d", preview.ItemsSubtotal)
	}
	if preview.DiscountAmount != 200_000 {
		t.Errorf("expected discount_amount 200000, got %d", preview.DiscountAmount)
	}
	if preview.DiscountedItemsSubtotal != 1_800_000 {
		t.Errorf("expected discounted_items_subtotal 1800000, got %d", preview.DiscountedItemsSubtotal)
	}
}

func TestHandlerPreviewInvalidCoupon(t *testing.T) {
	env := newTestHandlerEnv(t)
	userCookie, userID := env.registerAndLogin(t, auth.RoleUser)
	p := env.createProduct(t, 1_000_000, 10)
	if _, err := env.cartRepo.AddItem(context.Background(), userID, p.ID, 1); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	body, _ := json.Marshal(map[string]any{"code": "does-not-exist-" + uniqueSuffix()})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/coupons/preview", bytes.NewReader(body))
	req.AddCookie(userCookie)
	w := httptest.NewRecorder()
	env.handler.Preview(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func boolPtr(b bool) *bool { return &b }

// ---------- Admin authorization ----------

func TestHandlerAdminListUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/coupons", nil)
	w := httptest.NewRecorder()
	env.handler.AdminList(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerAdminListNonAdminForbidden(t *testing.T) {
	env := newTestHandlerEnv(t)
	userCookie, _ := env.registerAndLogin(t, auth.RoleUser)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/coupons", nil)
	req.AddCookie(userCookie)
	w := httptest.NewRecorder()
	env.handler.AdminList(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandlerAdminListAdminSuccess(t *testing.T) {
	env := newTestHandlerEnv(t)
	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/coupons", nil)
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	env.handler.AdminList(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandlerAdminCreateUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	body, _ := json.Marshal(map[string]any{"code": "X", "discount_type": "percent", "value": 10})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/coupons", bytes.NewReader(body))
	w := httptest.NewRecorder()
	env.handler.AdminCreate(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerAdminCreateNonAdminForbidden(t *testing.T) {
	env := newTestHandlerEnv(t)
	userCookie, _ := env.registerAndLogin(t, auth.RoleUser)
	body, _ := json.Marshal(map[string]any{"code": "X", "discount_type": "percent", "value": 10})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/coupons", bytes.NewReader(body))
	req.AddCookie(userCookie)
	w := httptest.NewRecorder()
	env.handler.AdminCreate(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandlerAdminCreateSuccess(t *testing.T) {
	env := newTestHandlerEnv(t)
	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)
	code := uniqueCode("ADMINCREATE")
	body, _ := json.Marshal(map[string]any{"code": code, "discount_type": "percent", "value": 15})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/coupons", bytes.NewReader(body))
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	env.handler.AdminCreate(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusCreated, w.Body.String())
	}
	var c Coupon
	json.NewDecoder(w.Body).Decode(&c)
	env.db.Exec(context.Background(), "DELETE FROM coupons WHERE id = $1", c.ID)
}

func TestHandlerAdminCreateDuplicateCodeConflict(t *testing.T) {
	env := newTestHandlerEnv(t)
	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)
	code := uniqueCode("ADMINDUPE")

	first, err := env.handler.repository.Create(context.Background(), CreateInput{
		Code: code, DiscountType: DiscountTypePercent, Value: 10, IsActive: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("seed coupon failed: %v", err)
	}
	env.db.Exec(context.Background(), "SELECT 1") // keep parity with other tests
	t.Cleanup(func() { env.db.Exec(context.Background(), "DELETE FROM coupons WHERE id = $1", first.ID) })

	body, _ := json.Marshal(map[string]any{"code": code, "discount_type": "fixed", "value": 1000})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/coupons", bytes.NewReader(body))
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	env.handler.AdminCreate(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusConflict, w.Body.String())
	}
}

func TestHandlerAdminCreateInvalidPercentage(t *testing.T) {
	env := newTestHandlerEnv(t)
	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)
	body, _ := json.Marshal(map[string]any{"code": uniqueCode("BADPCT"), "discount_type": "percent", "value": 150})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/coupons", bytes.NewReader(body))
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	env.handler.AdminCreate(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandlerAdminCreateInvalidFixedAmount(t *testing.T) {
	env := newTestHandlerEnv(t)
	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)
	body, _ := json.Marshal(map[string]any{"code": uniqueCode("BADFIX"), "discount_type": "fixed", "value": 0})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/coupons", bytes.NewReader(body))
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	env.handler.AdminCreate(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandlerAdminCreateInvalidDateRange(t *testing.T) {
	env := newTestHandlerEnv(t)
	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)
	body, _ := json.Marshal(map[string]any{
		"code": uniqueCode("BADDATE"), "discount_type": "fixed", "value": 1000,
		"starts_at": "2030-01-01T00:00:00Z", "ends_at": "2020-01-01T00:00:00Z",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/coupons", bytes.NewReader(body))
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	env.handler.AdminCreate(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandlerAdminCreateInvalidUsageLimits(t *testing.T) {
	env := newTestHandlerEnv(t)
	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)
	body, _ := json.Marshal(map[string]any{
		"code": uniqueCode("BADLIMIT"), "discount_type": "fixed", "value": 1000, "usage_limit": 0,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/coupons", bytes.NewReader(body))
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	env.handler.AdminCreate(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ---------- Admin update: activate/deactivate ----------

func TestHandlerAdminUpdateDeactivate(t *testing.T) {
	env := newTestHandlerEnv(t)
	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)
	code := uniqueCode("DEACTIVATE")
	c, err := env.handler.repository.Create(context.Background(), CreateInput{
		Code: code, DiscountType: DiscountTypePercent, Value: 10, IsActive: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("seed coupon failed: %v", err)
	}
	t.Cleanup(func() { env.db.Exec(context.Background(), "DELETE FROM coupons WHERE id = $1", c.ID) })

	body, _ := json.Marshal(map[string]any{"is_active": false})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/coupons/"+strconv.FormatInt(c.ID, 10), bytes.NewReader(body))
	req.SetPathValue("id", strconv.FormatInt(c.ID, 10))
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	env.handler.AdminUpdate(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	var updated Coupon
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.IsActive {
		t.Error("expected coupon to be deactivated")
	}
}

func TestHandlerAdminUpdateNotFound(t *testing.T) {
	env := newTestHandlerEnv(t)
	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)
	body, _ := json.Marshal(map[string]any{"is_active": false})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/coupons/999999999", bytes.NewReader(body))
	req.SetPathValue("id", "999999999")
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	env.handler.AdminUpdate(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}
