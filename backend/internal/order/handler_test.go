package order

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
	"github.com/Behnamdevops/plant-shop/backend/internal/cart"
	"github.com/Behnamdevops/plant-shop/backend/internal/product"
	"github.com/jackc/pgx/v5/pgxpool"
)

type testHandlerEnv struct {
	handler     *Handler
	authHandler *auth.Handler
	productRepo *product.Repository
	cartHandler *cart.Handler
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
	authHandler := auth.NewHandler(auth.NewRepository(db))
	orderHandler := NewHandler(NewRepository(db), authHandler)
	cartHandler := cart.NewHandler(cart.NewRepository(db), authHandler)
	return &testHandlerEnv{
		handler:     orderHandler,
		authHandler: authHandler,
		productRepo: product.NewRepository(db),
		cartHandler: cartHandler,
		db:          db,
	}
}

var handlerSeq int

func handlerUniqueSuffix() string {
	handlerSeq++
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), handlerSeq)
}

// registerAndLogin creates a new user via the real auth handler and returns
// the session cookie so order requests can authenticate the same way a real
// client would (never trusting a client-supplied user_id).
func (e *testHandlerEnv) registerAndLogin(t *testing.T) *http.Cookie {
	t.Helper()
	cookie, _ := e.registerAndLoginWithRole(t, auth.RoleUser)
	return cookie
}

// registerAndLoginWithRole creates a new user with the given role via the
// real auth handler/repository and returns both the session cookie and the
// created user's email, so admin requests authenticate the same way a real
// client would (role is never trusted from the request itself) and callers
// can look up the user's id/email if needed. Cleans up the user and their
// sessions after the test.
func (e *testHandlerEnv) registerAndLoginWithRole(t *testing.T, role string) (*http.Cookie, string) {
	t.Helper()
	suffix := handlerUniqueSuffix()
	email := "orderuser-" + suffix + "@example.com"
	body, _ := json.Marshal(map[string]any{
		"name":     "orderuser-" + suffix,
		"email":    email,
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	e.authHandler.Register(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup registration failed: got %d", w.Code)
	}

	if role == auth.RoleAdmin {
		if _, err := e.db.Exec(t.Context(), `UPDATE users SET role = $1 WHERE email = $2`, auth.RoleAdmin, email); err != nil {
			t.Fatalf("failed to promote test user to admin: %v", err)
		}
	}

	t.Cleanup(func() {
		e.db.Exec(context.Background(), `DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE email = $1)`, email)
		e.db.Exec(context.Background(), `DELETE FROM orders WHERE user_id IN (SELECT id FROM users WHERE email = $1)`, email)
		e.db.Exec(context.Background(), `DELETE FROM users WHERE email = $1`, email)
	})

	res := w.Result()
	for _, c := range res.Cookies() {
		if c.Name == "session_token" {
			return c, email
		}
	}
	t.Fatal("no session_token cookie set on registration")
	return nil, ""
}

func (e *testHandlerEnv) createProduct(t *testing.T, stock int) product.Product {
	t.Helper()
	suffix := handlerUniqueSuffix()
	p, err := e.productRepo.Create(t.Context(), product.CreateProductInput{
		Name:  "Order Product " + suffix,
		Slug:  "order-product-" + suffix,
		Price: 200,
		Stock: stock,
	})
	if err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	return p
}

// addToCart adds quantity of productID to cookie's cart via the real cart
// handler, mirroring how a real checkout flow builds up a cart.
func (e *testHandlerEnv) addToCart(t *testing.T, cookie *http.Cookie, productID int64, quantity int) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"product_id": productID, "quantity": quantity})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/cart/items", bytes.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	e.cartHandler.AddItem(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup addToCart failed: got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestHandlerCreateUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := checkoutRequest(nil)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerListUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	w := httptest.NewRecorder()
	env.handler.List(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerGetByIDUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/1", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	env.handler.GetByID(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerCreateEmptyCart(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)

	req := checkoutRequest(cookie)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandlerCreateSuccess(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	p := env.createProduct(t, 10)
	env.addToCart(t, cookie, p.ID, 3)

	req := checkoutRequest(cookie)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusCreated, w.Body.String())
	}

	var o Order
	if err := json.NewDecoder(w.Body).Decode(&o); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if o.Status != StatusPending {
		t.Errorf("expected status %q, got %q", StatusPending, o.Status)
	}
	wantSubtotal := p.Price * 3
	wantTotal := wantSubtotal + ShippingFeeStandard
	if o.ItemsSubtotal != wantSubtotal {
		t.Errorf("expected items_subtotal %d, got %d", wantSubtotal, o.ItemsSubtotal)
	}
	if o.ShippingFee != ShippingFeeStandard {
		t.Errorf("expected shipping_fee %d, got %d", ShippingFeeStandard, o.ShippingFee)
	}
	if o.Total != wantTotal {
		t.Errorf("expected total %d, got %d", wantTotal, o.Total)
	}
	if o.PaymentStatus != PaymentStatusPending {
		t.Errorf("expected payment_status %q, got %q", PaymentStatusPending, o.PaymentStatus)
	}
	if o.PaymentMethod != PaymentMethodManual {
		t.Errorf("expected payment_method %q, got %q", PaymentMethodManual, o.PaymentMethod)
	}
	if o.RecipientName == nil || *o.RecipientName != "Test Recipient" {
		t.Errorf("expected recipient_name snapshot to be stored, got %v", o.RecipientName)
	}
	if o.ShippingMethod != ShippingMethodStandard {
		t.Errorf("expected shipping_method %q, got %q", ShippingMethodStandard, o.ShippingMethod)
	}

	// Cart should now be empty.
	getCartReq := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
	getCartReq.AddCookie(cookie)
	getCartW := httptest.NewRecorder()
	env.cartHandler.GetCart(getCartW, getCartReq)
	var c cart.Cart
	json.NewDecoder(getCartW.Body).Decode(&c)
	if len(c.Items) != 0 {
		t.Errorf("expected cart to be empty after order, got %d items", len(c.Items))
	}
}

func TestHandlerCreateInsufficientStock(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	p := env.createProduct(t, 5)
	env.addToCart(t, cookie, p.ID, 3)

	// Simulate a race: stock drops below the cart's requested quantity
	// after the item was added (e.g. another checkout consumed it).
	_, err := env.db.Exec(t.Context(), `UPDATE products SET stock = 1 WHERE id = $1`, p.ID)
	if err != nil {
		t.Fatalf("failed to force low stock: %v", err)
	}

	req := checkoutRequest(cookie)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusConflict, w.Body.String())
	}

	// Nothing should have been created, and the cart should remain intact.
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	listReq.AddCookie(cookie)
	listW := httptest.NewRecorder()
	env.handler.List(listW, listReq)
	var orders []Order
	json.NewDecoder(listW.Body).Decode(&orders)
	if len(orders) != 0 {
		t.Errorf("expected no orders to be created, got %d", len(orders))
	}

	getCartReq := httptest.NewRequest(http.MethodGet, "/api/v1/cart", nil)
	getCartReq.AddCookie(cookie)
	getCartW := httptest.NewRecorder()
	env.cartHandler.GetCart(getCartW, getCartReq)
	var c cart.Cart
	json.NewDecoder(getCartW.Body).Decode(&c)
	if len(c.Items) != 1 {
		t.Errorf("expected cart to still hold 1 item after failed checkout, got %d", len(c.Items))
	}
}

func TestHandlerListOnlyCurrentUser(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookieA := env.registerAndLogin(t)
	cookieB := env.registerAndLogin(t)
	p := env.createProduct(t, 10)

	env.addToCart(t, cookieA, p.ID, 1)
	createReqA := checkoutRequest(cookieA)
	createWA := httptest.NewRecorder()
	env.handler.Create(createWA, createReqA)
	if createWA.Code != http.StatusCreated {
		t.Fatalf("setup order for user A failed: got %d", createWA.Code)
	}

	listReqB := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	listReqB.AddCookie(cookieB)
	listWB := httptest.NewRecorder()
	env.handler.List(listWB, listReqB)
	if listWB.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", listWB.Code, http.StatusOK)
	}
	var ordersB []Order
	json.NewDecoder(listWB.Body).Decode(&ordersB)
	if len(ordersB) != 0 {
		t.Fatalf("expected user B to see 0 orders, got %d", len(ordersB))
	}

	listReqA := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	listReqA.AddCookie(cookieA)
	listWA := httptest.NewRecorder()
	env.handler.List(listWA, listReqA)
	var ordersA []Order
	json.NewDecoder(listWA.Body).Decode(&ordersA)
	if len(ordersA) != 1 {
		t.Fatalf("expected user A to see 1 order, got %d", len(ordersA))
	}
}

func TestHandlerGetByIDCrossUserReturns404(t *testing.T) {
	env := newTestHandlerEnv(t)
	owner := env.registerAndLogin(t)
	attacker := env.registerAndLogin(t)
	p := env.createProduct(t, 10)

	env.addToCart(t, owner, p.ID, 1)
	createReq := checkoutRequest(owner)
	createW := httptest.NewRecorder()
	env.handler.Create(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("setup order failed: got %d", createW.Code)
	}
	var created Order
	json.NewDecoder(createW.Body).Decode(&created)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+strconv.FormatInt(created.ID, 10), nil)
	getReq.SetPathValue("id", strconv.FormatInt(created.ID, 10))
	getReq.AddCookie(attacker)
	getW := httptest.NewRecorder()
	env.handler.GetByID(getW, getReq)
	if getW.Code != http.StatusNotFound {
		t.Fatalf("expected cross-user get to be rejected with 404, got %d", getW.Code)
	}

	// Confirm the owner can still fetch it, including its items.
	getReq2 := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+strconv.FormatInt(created.ID, 10), nil)
	getReq2.SetPathValue("id", strconv.FormatInt(created.ID, 10))
	getReq2.AddCookie(owner)
	getW2 := httptest.NewRecorder()
	env.handler.GetByID(getW2, getReq2)
	if getW2.Code != http.StatusOK {
		t.Fatalf("expected owner get to succeed, got %d", getW2.Code)
	}
	var full OrderWithItems
	json.NewDecoder(getW2.Body).Decode(&full)
	if len(full.Items) != 1 {
		t.Fatalf("expected 1 order item, got %d", len(full.Items))
	}
}

func TestHandlerGetByIDNotFound(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/999999", nil)
	req.SetPathValue("id", "999999")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	env.handler.GetByID(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

// ---------- Admin authorization ----------

func TestHandlerAdminListUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders", nil)
	w := httptest.NewRecorder()
	env.handler.AdminList(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerAdminListNonAdminForbidden(t *testing.T) {
	env := newTestHandlerEnv(t)
	user, _ := env.registerAndLoginWithRole(t, auth.RoleUser)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders", nil)
	req.AddCookie(user)
	w := httptest.NewRecorder()
	env.handler.AdminList(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandlerAdminListAdminSuccess(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders", nil)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminList(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandlerAdminGetByIDUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders/1", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	env.handler.AdminGetByID(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerAdminGetByIDNonAdminForbidden(t *testing.T) {
	env := newTestHandlerEnv(t)
	user, _ := env.registerAndLoginWithRole(t, auth.RoleUser)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders/1", nil)
	req.SetPathValue("id", "1")
	req.AddCookie(user)
	w := httptest.NewRecorder()
	env.handler.AdminGetByID(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandlerAdminUpdateStatusUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	body, _ := json.Marshal(map[string]any{"status": "processing"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/1/status", bytes.NewReader(body))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerAdminUpdateStatusNonAdminForbidden(t *testing.T) {
	env := newTestHandlerEnv(t)
	user, _ := env.registerAndLoginWithRole(t, auth.RoleUser)
	body, _ := json.Marshal(map[string]any{"status": "processing"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/1/status", bytes.NewReader(body))
	req.SetPathValue("id", "1")
	req.AddCookie(user)
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusForbidden)
	}
}

// ---------- Admin behavior ----------

// createOrderForUser places an order for cookie's user containing one unit
// of a freshly created product, and returns the created order.
// validCheckoutBody returns a JSON-encoded checkout request body that
// passes server-side validation, for tests that only care about auth/cart/
// stock/status behavior and not about delivery-field validation itself.
func validCheckoutBody() []byte {
	body, _ := json.Marshal(map[string]any{
		"recipient_name":  "Test Recipient",
		"phone":           "+1 555 0100",
		"address_line1":   "123 Greenhouse Ave",
		"address_line2":   "",
		"city":            "Plantville",
		"postal_code":     "12345",
		"country":         "Testland",
		"shipping_method": ShippingMethodStandard,
	})
	return body
}

// checkoutRequest builds a POST /api/v1/orders request carrying a valid
// checkout body, mirroring how a real client would submit the checkout
// form.
func checkoutRequest(cookie *http.Cookie) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(validCheckoutBody()))
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

func (e *testHandlerEnv) createOrderForUser(t *testing.T, cookie *http.Cookie) Order {
	t.Helper()
	p := e.createProduct(t, 10)
	e.addToCart(t, cookie, p.ID, 1)
	req := checkoutRequest(cookie)
	w := httptest.NewRecorder()
	e.handler.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup order failed: got %d, body=%s", w.Code, w.Body.String())
	}
	var o Order
	json.NewDecoder(w.Body).Decode(&o)
	return o
}

func TestHandlerAdminListSeesOrdersFromMultipleUsers(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	userA, emailA := env.registerAndLoginWithRole(t, auth.RoleUser)
	userB, emailB := env.registerAndLoginWithRole(t, auth.RoleUser)

	orderA := env.createOrderForUser(t, userA)
	orderB := env.createOrderForUser(t, userB)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders", nil)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminList(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var orders []AdminOrder
	if err := json.NewDecoder(w.Body).Decode(&orders); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	foundA, foundB := false, false
	for _, o := range orders {
		if o.ID == orderA.ID {
			foundA = true
			if o.Customer.Email != emailA {
				t.Errorf("expected order A customer email %q, got %q", emailA, o.Customer.Email)
			}
		}
		if o.ID == orderB.ID {
			foundB = true
			if o.Customer.Email != emailB {
				t.Errorf("expected order B customer email %q, got %q", emailB, o.Customer.Email)
			}
		}
	}
	if !foundA || !foundB {
		t.Fatalf("expected admin list to include orders from both users; foundA=%v foundB=%v", foundA, foundB)
	}
}

func TestHandlerAdminGetByIDReturnsCustomerAndItems(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	buyer, buyerEmail := env.registerAndLoginWithRole(t, auth.RoleUser)
	_ = buyer

	order := env.createOrderForUser(t, buyer)

	idStr := strconv.FormatInt(order.ID, 10)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders/"+idStr, nil)
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminGetByID(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var full AdminOrderWithItems
	if err := json.NewDecoder(w.Body).Decode(&full); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if full.ID != order.ID {
		t.Errorf("expected order id %d, got %d", order.ID, full.ID)
	}
	if full.Customer.Email != buyerEmail {
		t.Errorf("expected customer email %q, got %q", buyerEmail, full.Customer.Email)
	}
	if len(full.Items) != 1 {
		t.Fatalf("expected 1 order item, got %d", len(full.Items))
	}
}

func TestHandlerAdminGetByIDInvalidID(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders/abc", nil)
	req.SetPathValue("id", "abc")
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminGetByID(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandlerAdminGetByIDNotFound(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders/999999999999", nil)
	req.SetPathValue("id", "999999999999")
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminGetByID(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandlerAdminUpdateStatusInvalidID(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	body, _ := json.Marshal(map[string]any{"status": "processing"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/abc/status", bytes.NewReader(body))
	req.SetPathValue("id", "abc")
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandlerAdminUpdateStatusNotFound(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	body, _ := json.Marshal(map[string]any{"status": "processing"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/999999999999/status", bytes.NewReader(body))
	req.SetPathValue("id", "999999999999")
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandlerAdminUpdateStatusInvalidStatus(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	buyer, _ := env.registerAndLoginWithRole(t, auth.RoleUser)
	order := env.createOrderForUser(t, buyer)

	idStr := strconv.FormatInt(order.ID, 10)
	body, _ := json.Marshal(map[string]any{"status": "not-a-real-status"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandlerAdminUpdateStatusInvalidBody(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	buyer, _ := env.registerAndLoginWithRole(t, auth.RoleUser)
	order := env.createOrderForUser(t, buyer)

	idStr := strconv.FormatInt(order.ID, 10)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader([]byte("not json")))
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandlerAdminUpdateStatusValidTransitionSucceeds(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	buyer, _ := env.registerAndLoginWithRole(t, auth.RoleUser)
	order := env.createOrderForUser(t, buyer)

	idStr := strconv.FormatInt(order.ID, 10)
	body, _ := json.Marshal(map[string]any{"status": StatusProcessing})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var updated AdminOrderWithItems
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Status != StatusProcessing {
		t.Errorf("expected status %q, got %q", StatusProcessing, updated.Status)
	}
}

func TestHandlerAdminUpdateStatusInvalidTransitionConflict(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	buyer, _ := env.registerAndLoginWithRole(t, auth.RoleUser)
	order := env.createOrderForUser(t, buyer)

	// pending -> shipped is not a valid transition (must go through
	// processing first).
	idStr := strconv.FormatInt(order.ID, 10)
	body, _ := json.Marshal(map[string]any{"status": StatusShipped})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusConflict, w.Body.String())
	}
}

func TestHandlerAdminUpdateStatusDeliveredIsTerminal(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	buyer, _ := env.registerAndLoginWithRole(t, auth.RoleUser)
	order := env.createOrderForUser(t, buyer)
	idStr := strconv.FormatInt(order.ID, 10)

	// Walk the order through the full happy path to "delivered".
	for _, status := range []string{StatusProcessing, StatusShipped, StatusDelivered} {
		body, _ := json.Marshal(map[string]any{"status": status})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body))
		req.SetPathValue("id", idStr)
		req.AddCookie(admin)
		w := httptest.NewRecorder()
		env.handler.AdminUpdateStatus(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("transition to %q failed: got %d, body=%s", status, w.Code, w.Body.String())
		}
	}

	// Now delivered -> anything must be rejected.
	body, _ := json.Marshal(map[string]any{"status": StatusCancelled})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected delivered->cancelled to be rejected with 409, got %d", w.Code)
	}
}

func TestHandlerAdminUpdateStatusCancelledIsTerminal(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	buyer, _ := env.registerAndLoginWithRole(t, auth.RoleUser)
	order := env.createOrderForUser(t, buyer)
	idStr := strconv.FormatInt(order.ID, 10)

	body, _ := json.Marshal(map[string]any{"status": StatusCancelled})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("transition to cancelled failed: got %d, body=%s", w.Code, w.Body.String())
	}

	body2, _ := json.Marshal(map[string]any{"status": StatusProcessing})
	req2 := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body2))
	req2.SetPathValue("id", idStr)
	req2.AddCookie(admin)
	w2 := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("expected cancelled->processing to be rejected with 409, got %d", w2.Code)
	}
}

// ---------- Checkout & Fulfillment V2 ----------

func TestHandlerCreateMissingRequiredDeliveryField(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	p := env.createProduct(t, 10)
	env.addToCart(t, cookie, p.ID, 1)

	body, _ := json.Marshal(map[string]any{
		"recipient_name":  "",
		"phone":           "+1 555 0100",
		"address_line1":   "123 Greenhouse Ave",
		"city":            "Plantville",
		"postal_code":     "12345",
		"country":         "Testland",
		"shipping_method": ShippingMethodStandard,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandlerCreateWhitespaceOnlyRequiredFieldRejected(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	p := env.createProduct(t, 10)
	env.addToCart(t, cookie, p.ID, 1)

	body, _ := json.Marshal(map[string]any{
		"recipient_name":  "   ",
		"phone":           "+1 555 0100",
		"address_line1":   "123 Greenhouse Ave",
		"city":            "Plantville",
		"postal_code":     "12345",
		"country":         "Testland",
		"shipping_method": ShippingMethodStandard,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandlerCreateInvalidShippingMethod(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	p := env.createProduct(t, 10)
	env.addToCart(t, cookie, p.ID, 1)

	body, _ := json.Marshal(map[string]any{
		"recipient_name":  "Test Recipient",
		"phone":           "+1 555 0100",
		"address_line1":   "123 Greenhouse Ave",
		"city":            "Plantville",
		"postal_code":     "12345",
		"country":         "Testland",
		"shipping_method": "overnight-drone",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestHandlerCreateExpressShippingUsesExpressFee(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	p := env.createProduct(t, 10)
	env.addToCart(t, cookie, p.ID, 2)

	body, _ := json.Marshal(map[string]any{
		"recipient_name":  "Test Recipient",
		"phone":           "+1 555 0100",
		"address_line1":   "123 Greenhouse Ave",
		"address_line2":   "Apt 4",
		"city":            "Plantville",
		"postal_code":     "12345",
		"country":         "Testland",
		"shipping_method": ShippingMethodExpress,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusCreated, w.Body.String())
	}

	var o Order
	json.NewDecoder(w.Body).Decode(&o)
	wantSubtotal := p.Price * 2
	wantTotal := wantSubtotal + ShippingFeeExpress
	if o.ShippingFee != ShippingFeeExpress {
		t.Errorf("expected shipping_fee %d, got %d", ShippingFeeExpress, o.ShippingFee)
	}
	if o.ItemsSubtotal != wantSubtotal {
		t.Errorf("expected items_subtotal %d, got %d", wantSubtotal, o.ItemsSubtotal)
	}
	if o.Total != wantTotal {
		t.Errorf("expected total %d, got %d", wantTotal, o.Total)
	}
	if o.AddressLine2 == nil || *o.AddressLine2 != "Apt 4" {
		t.Errorf("expected address_line2 snapshot %q, got %v", "Apt 4", o.AddressLine2)
	}
}

func TestHandlerGetByIDIncludesFulfillmentFields(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	order := env.createOrderForUser(t, cookie)

	idStr := strconv.FormatInt(order.ID, 10)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+idStr, nil)
	req.SetPathValue("id", idStr)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	env.handler.GetByID(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	for _, field := range []string{
		"recipient_name", "phone", "address_line1", "address_line2", "city", "postal_code", "country",
		"shipping_method", "shipping_fee", "items_subtotal", "payment_status", "payment_method",
	} {
		if _, ok := raw[field]; !ok {
			t.Errorf("expected customer order detail to contain field %q", field)
		}
	}
	// The response must not expose any auth-sensitive data.
	for _, field := range []string{"password_hash", "user_id", "token_hash", "session_token"} {
		if _, ok := raw[field]; ok {
			t.Errorf("order detail response unexpectedly contains sensitive field %q", field)
		}
	}
}

func TestHandlerAdminGetByIDIncludesFulfillmentFields(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	buyer, _ := env.registerAndLoginWithRole(t, auth.RoleUser)
	order := env.createOrderForUser(t, buyer)

	idStr := strconv.FormatInt(order.ID, 10)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders/"+idStr, nil)
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminGetByID(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	for _, field := range []string{
		"recipient_name", "phone", "address_line1", "city", "postal_code", "country",
		"shipping_method", "shipping_fee", "items_subtotal", "payment_status", "payment_method", "customer",
	} {
		if _, ok := raw[field]; !ok {
			t.Errorf("expected admin order detail to contain field %q", field)
		}
	}
	for _, field := range []string{"password_hash", "token_hash", "session_token"} {
		if _, ok := raw[field]; ok {
			t.Errorf("admin order detail response unexpectedly contains sensitive field %q", field)
		}
	}
}

// ---------- Cancellation stock restoration ----------

func TestHandlerCancelPendingRestoresStock(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	buyer, _ := env.registerAndLoginWithRole(t, auth.RoleUser)

	p := env.createProduct(t, 10)
	env.addToCart(t, buyer, p.ID, 3)
	createReq := checkoutRequest(buyer)
	createW := httptest.NewRecorder()
	env.handler.Create(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("setup order failed: got %d, body=%s", createW.Code, createW.Body.String())
	}
	var order Order
	json.NewDecoder(createW.Body).Decode(&order)

	// Stock was decremented by checkout.
	var stockAfterCheckout int
	env.db.QueryRow(t.Context(), `SELECT stock FROM products WHERE id = $1`, p.ID).Scan(&stockAfterCheckout)
	if stockAfterCheckout != 7 {
		t.Fatalf("expected stock 7 after checkout, got %d", stockAfterCheckout)
	}

	idStr := strconv.FormatInt(order.ID, 10)
	body, _ := json.Marshal(map[string]any{"status": StatusCancelled})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("cancel failed: got %d, body=%s", w.Code, w.Body.String())
	}

	var stockAfterCancel int
	env.db.QueryRow(t.Context(), `SELECT stock FROM products WHERE id = $1`, p.ID).Scan(&stockAfterCancel)
	if stockAfterCancel != 10 {
		t.Errorf("expected stock restored to 10 after cancellation, got %d", stockAfterCancel)
	}
}

func TestHandlerCancelProcessingRestoresStock(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	buyer, _ := env.registerAndLoginWithRole(t, auth.RoleUser)

	p := env.createProduct(t, 10)
	env.addToCart(t, buyer, p.ID, 4)
	createReq := checkoutRequest(buyer)
	createW := httptest.NewRecorder()
	env.handler.Create(createW, createReq)
	var order Order
	json.NewDecoder(createW.Body).Decode(&order)
	idStr := strconv.FormatInt(order.ID, 10)

	// pending -> processing
	body, _ := json.Marshal(map[string]any{"status": StatusProcessing})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("transition to processing failed: got %d, body=%s", w.Code, w.Body.String())
	}

	// processing -> cancelled
	body2, _ := json.Marshal(map[string]any{"status": StatusCancelled})
	req2 := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body2))
	req2.SetPathValue("id", idStr)
	req2.AddCookie(admin)
	w2 := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("cancel from processing failed: got %d, body=%s", w2.Code, w2.Body.String())
	}

	var stock int
	env.db.QueryRow(t.Context(), `SELECT stock FROM products WHERE id = $1`, p.ID).Scan(&stock)
	if stock != 10 {
		t.Errorf("expected stock restored to 10, got %d", stock)
	}
}

func TestHandlerCancelTwiceDoesNotDoubleRestoreStock(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	buyer, _ := env.registerAndLoginWithRole(t, auth.RoleUser)

	p := env.createProduct(t, 10)
	env.addToCart(t, buyer, p.ID, 2)
	createReq := checkoutRequest(buyer)
	createW := httptest.NewRecorder()
	env.handler.Create(createW, createReq)
	var order Order
	json.NewDecoder(createW.Body).Decode(&order)
	idStr := strconv.FormatInt(order.ID, 10)

	body, _ := json.Marshal(map[string]any{"status": StatusCancelled})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first cancel failed: got %d, body=%s", w.Code, w.Body.String())
	}

	// Repeated cancellation attempt: cancelled is terminal, must be
	// rejected and must NOT restore stock a second time.
	req2 := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body))
	req2.SetPathValue("id", idStr)
	req2.AddCookie(admin)
	w2 := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("expected repeated cancellation to be rejected with 409, got %d", w2.Code)
	}

	var stock int
	env.db.QueryRow(t.Context(), `SELECT stock FROM products WHERE id = $1`, p.ID).Scan(&stock)
	if stock != 10 {
		t.Errorf("expected stock restored exactly once (to 10), got %d", stock)
	}
}

func TestHandlerCancelAfterDeliveredRejectedNoStockChange(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	buyer, _ := env.registerAndLoginWithRole(t, auth.RoleUser)

	p := env.createProduct(t, 10)
	env.addToCart(t, buyer, p.ID, 1)
	createReq := checkoutRequest(buyer)
	createW := httptest.NewRecorder()
	env.handler.Create(createW, createReq)
	var order Order
	json.NewDecoder(createW.Body).Decode(&order)
	idStr := strconv.FormatInt(order.ID, 10)

	for _, status := range []string{StatusProcessing, StatusShipped, StatusDelivered} {
		body, _ := json.Marshal(map[string]any{"status": status})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body))
		req.SetPathValue("id", idStr)
		req.AddCookie(admin)
		w := httptest.NewRecorder()
		env.handler.AdminUpdateStatus(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("transition to %q failed: got %d, body=%s", status, w.Code, w.Body.String())
		}
	}

	// delivered -> cancelled must be rejected, and stock must remain
	// decremented (delivered orders never restore stock).
	body, _ := json.Marshal(map[string]any{"status": StatusCancelled})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected delivered->cancelled rejected with 409, got %d", w.Code)
	}

	var stock int
	env.db.QueryRow(t.Context(), `SELECT stock FROM products WHERE id = $1`, p.ID).Scan(&stock)
	if stock != 9 {
		t.Errorf("expected stock to remain decremented at 9 (delivered order, no restoration), got %d", stock)
	}
}

func TestHandlerCancelMultiItemOrderRestoresAllProducts(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	buyer, _ := env.registerAndLoginWithRole(t, auth.RoleUser)

	pA := env.createProduct(t, 10)
	pB := env.createProduct(t, 20)
	pC := env.createProduct(t, 5)
	env.addToCart(t, buyer, pA.ID, 3)
	env.addToCart(t, buyer, pB.ID, 7)
	env.addToCart(t, buyer, pC.ID, 2)

	createReq := checkoutRequest(buyer)
	createW := httptest.NewRecorder()
	env.handler.Create(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("setup order failed: got %d, body=%s", createW.Code, createW.Body.String())
	}
	var order Order
	json.NewDecoder(createW.Body).Decode(&order)
	idStr := strconv.FormatInt(order.ID, 10)

	body, _ := json.Marshal(map[string]any{"status": StatusCancelled})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("cancel failed: got %d, body=%s", w.Code, w.Body.String())
	}

	for _, tc := range []struct {
		id   int64
		want int
	}{
		{pA.ID, 10},
		{pB.ID, 20},
		{pC.ID, 5},
	} {
		var stock int
		env.db.QueryRow(t.Context(), `SELECT stock FROM products WHERE id = $1`, tc.id).Scan(&stock)
		if stock != tc.want {
			t.Errorf("product %d: expected stock restored to %d, got %d", tc.id, tc.want, stock)
		}
	}
}
