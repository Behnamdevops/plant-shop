package returnpkg

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
	"github.com/Behnamdevops/plant-shop/backend/internal/order"
	"github.com/Behnamdevops/plant-shop/backend/internal/product"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testHandlerEnv wires a real auth.Handler backed by the test database, so
// tests exercise the exact RequireAdmin/Authenticate logic used in
// production instead of a stand-in.
type testHandlerEnv struct {
	handler     *Handler
	repository  *Repository
	authHandler *auth.Handler
	orderRepo   *order.Repository
	productRepo *product.Repository
	cartRepo    *cart.Repository
	db          *pgxpool.Pool
}

func newTestHandlerEnv(t *testing.T) *testHandlerEnv {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL_TEST")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping test")
	}
	db, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Skip("cannot connect to database: ", err)
	}
	t.Cleanup(db.Close)

	orderRepo := order.NewRepository(db)
	productRepo := product.NewRepository(db)
	cartRepo := cart.NewRepository(db)
	repository := NewRepository(db, orderRepo)
	authHandler := auth.NewHandler(auth.NewRepository(db))
	handler := NewHandler(repository, orderRepo, authHandler)

	return &testHandlerEnv{
		handler:     handler,
		repository:  repository,
		authHandler: authHandler,
		orderRepo:   orderRepo,
		productRepo: productRepo,
		cartRepo:    cartRepo,
		db:          db,
	}
}

func (e *testHandlerEnv) cleanup(t *testing.T) {
	t.Helper()
	e.db.Exec(context.Background(), `DELETE FROM payment_attempts WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%rethandler%'))`)
	e.db.Exec(context.Background(), `DELETE FROM refunds WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%rethandler%'))`)
	e.db.Exec(context.Background(), `DELETE FROM return_requests WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%rethandler%'))`)
	e.db.Exec(context.Background(), `DELETE FROM cart_items WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%rethandler%')`)
	e.db.Exec(context.Background(), `DELETE FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%rethandler%')`)
	e.db.Exec(context.Background(), `DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%rethandler%')`)
	e.db.Exec(context.Background(), `DELETE FROM users WHERE email LIKE '%rethandler%'`)
}

var handlerTestSeq int

func uniqueSuffix() string {
	handlerTestSeq++
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), handlerTestSeq)
}

// registerAndLogin creates a user (optionally promoted to admin) via the
// real auth handler and returns its session cookie, exactly mirroring what
// a browser client would receive.
func (e *testHandlerEnv) registerAndLogin(t *testing.T, role string) (*http.Cookie, int64) {
	t.Helper()
	suffix := uniqueSuffix()
	email := "rethandler-" + suffix + "@example.com"
	body, _ := json.Marshal(map[string]any{
		"name":     "rethandler-" + suffix,
		"email":    email,
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	e.authHandler.Register(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup registration failed: got %d, body=%s", w.Code, w.Body.String())
	}

	var userID int64
	if err := e.db.QueryRow(context.Background(), `SELECT id FROM users WHERE email = $1`, email).Scan(&userID); err != nil {
		t.Fatalf("failed to look up created user: %v", err)
	}

	if role == auth.RoleAdmin {
		if _, err := e.db.Exec(context.Background(), `UPDATE users SET role = $1 WHERE email = $2`, auth.RoleAdmin, email); err != nil {
			t.Fatalf("failed to promote test user to admin: %v", err)
		}
	}

	res := w.Result()
	for _, c := range res.Cookies() {
		if c.Name == "session_token" {
			return c, userID
		}
	}
	t.Fatal("no session_token cookie set on registration")
	return nil, 0
}

// createDeliveredPaidOrder checks out a fresh product for userID and marks
// the resulting order as delivered/paid, the minimum state required for a
// return request to be eligible.
func (e *testHandlerEnv) createDeliveredPaidOrder(t *testing.T, userID int64) order.Order {
	t.Helper()
	p, err := e.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Return Handler Test Product",
		Slug:  "return-handler-product-" + uniqueSuffix(),
		Price: 10000,
		Stock: 100,
	})
	if err != nil {
		t.Fatalf("failed to create product: %v", err)
	}

	if _, err := e.cartRepo.AddItem(context.Background(), userID, p.ID, 1); err != nil {
		t.Fatalf("failed to add to cart: %v", err)
	}

	o, err := e.orderRepo.CreateFromCart(context.Background(), userID, order.CheckoutInput{
		RecipientName:  "Test Recipient",
		Phone:          "09123456789",
		AddressLine1:   "Test Address",
		City:           "Test City",
		PostalCode:     "1234567890",
		Country:        "Iran",
		ShippingMethod: order.ShippingMethodStandard,
	})
	if err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	if _, err := e.db.Exec(context.Background(), `
		UPDATE orders SET status = $1, payment_status = $2 WHERE id = $3
	`, order.StatusDelivered, order.PaymentStatusPaid, o.ID); err != nil {
		t.Fatalf("failed to mark order delivered/paid: %v", err)
	}

	return o
}

// ---------- Customer: Create ----------

func TestHandlerCreateUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup(t)

	body, _ := json.Marshal(map[string]any{"reason": "Defective"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/1/return-request", bytes.NewReader(body))
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()

	env.handler.Create(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandlerCreateOwnEligibleOrderSucceeds(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup(t)

	cookie, userID := env.registerAndLogin(t, auth.RoleUser)
	o := env.createDeliveredPaidOrder(t, userID)

	idStr := strconv.FormatInt(o.ID, 10)
	body, _ := json.Marshal(map[string]any{"reason": "Defective item"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+idStr+"/return-request", bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()

	env.handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d, body=%s", w.Code, w.Body.String())
	}
	var rr ReturnRequest
	if err := json.Unmarshal(w.Body.Bytes(), &rr); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if rr.Status != StatusRequested {
		t.Errorf("expected status %q, got %q", StatusRequested, rr.Status)
	}
}

func TestHandlerCreateOtherUsersOrderNotFound(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup(t)

	_, ownerID := env.registerAndLogin(t, auth.RoleUser)
	o := env.createDeliveredPaidOrder(t, ownerID)

	attackerCookie, _ := env.registerAndLogin(t, auth.RoleUser)

	idStr := strconv.FormatInt(o.ID, 10)
	body, _ := json.Marshal(map[string]any{"reason": "Defective item"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+idStr+"/return-request", bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	req.AddCookie(attackerCookie)
	w := httptest.NewRecorder()

	env.handler.Create(w, req)

	// Cross-user access must be indistinguishable from "order does not
	// exist" — no 403, no leaking that the order belongs to someone else.
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for cross-user access, got %d", w.Code)
	}
}

func TestHandlerCreateUnpaidOrNonDeliveredOrderRejected(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup(t)

	cookie, userID := env.registerAndLogin(t, auth.RoleUser)

	p, err := env.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Return Handler Test Product Unpaid",
		Slug:  "return-handler-product-unpaid-" + uniqueSuffix(),
		Price: 10000,
		Stock: 100,
	})
	if err != nil {
		t.Fatalf("failed to create product: %v", err)
	}
	if _, err := env.cartRepo.AddItem(context.Background(), userID, p.ID, 1); err != nil {
		t.Fatalf("failed to add to cart: %v", err)
	}
	o, err := env.orderRepo.CreateFromCart(context.Background(), userID, order.CheckoutInput{
		RecipientName:  "Test Recipient",
		Phone:          "09123456789",
		AddressLine1:   "Test Address",
		City:           "Test City",
		PostalCode:     "1234567890",
		Country:        "Iran",
		ShippingMethod: order.ShippingMethodStandard,
	})
	if err != nil {
		t.Fatalf("failed to create order: %v", err)
	}
	// Order is left in its default pending/unpaid state (not delivered, not paid).

	idStr := strconv.FormatInt(o.ID, 10)
	body, _ := json.Marshal(map[string]any{"reason": "Defective item"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+idStr+"/return-request", bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()

	env.handler.Create(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 for unpaid/non-delivered order, got %d, body=%s", w.Code, w.Body.String())
	}
}

// ---------- Customer: GetByID ----------

func TestHandlerGetByIDCrossUserNotFound(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup(t)

	ownerCookie, ownerID := env.registerAndLogin(t, auth.RoleUser)
	o := env.createDeliveredPaidOrder(t, ownerID)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+strconv.FormatInt(o.ID, 10)+"/return-request",
		bytes.NewReader(mustJSON(map[string]any{"reason": "Defective"})))
	createReq.SetPathValue("id", strconv.FormatInt(o.ID, 10))
	createReq.AddCookie(ownerCookie)
	createW := httptest.NewRecorder()
	env.handler.Create(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("setup: failed to create return request: %d %s", createW.Code, createW.Body.String())
	}

	attackerCookie, _ := env.registerAndLogin(t, auth.RoleUser)

	idStr := strconv.FormatInt(o.ID, 10)
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+idStr+"/return-request", nil)
	getReq.SetPathValue("id", idStr)
	getReq.AddCookie(attackerCookie)
	getW := httptest.NewRecorder()

	env.handler.GetByID(getW, getReq)

	if getW.Code != http.StatusNotFound {
		t.Errorf("expected 404 for cross-user access, got %d", getW.Code)
	}
}

// ---------- Admin authorization ----------

func TestHandlerAdminListUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/returns", nil)
	w := httptest.NewRecorder()
	env.handler.AdminList(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandlerAdminListNonAdminForbidden(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup(t)

	cookie, _ := env.registerAndLogin(t, auth.RoleUser)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/returns", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	env.handler.AdminList(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestHandlerAdminApproveOnlyFromRequested(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup(t)

	ownerCookie, ownerID := env.registerAndLogin(t, auth.RoleUser)
	o := env.createDeliveredPaidOrder(t, ownerID)
	rr, err := env.repository.CreateRequest(context.Background(), o.ID, ownerID, RequestInput{Reason: "Defective"})
	if err != nil {
		t.Fatalf("setup: failed to create return request: %v", err)
	}
	_ = ownerCookie

	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)

	idStr := strconv.FormatInt(rr.ID, 10)
	approve := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/returns/"+idStr+"/approve", bytes.NewReader([]byte(`{}`)))
		req.SetPathValue("id", idStr)
		req.AddCookie(adminCookie)
		w := httptest.NewRecorder()
		env.handler.AdminApprove(w, req)
		return w
	}

	w1 := approve()
	if w1.Code != http.StatusOK {
		t.Fatalf("expected first approve to succeed, got %d body=%s", w1.Code, w1.Body.String())
	}

	// Approving again from "approved" must fail (only requested -> approved is valid).
	w2 := approve()
	if w2.Code != http.StatusConflict {
		t.Errorf("expected second approve to be rejected with 409, got %d", w2.Code)
	}
}

func TestHandlerAdminRejectOnlyFromRequested(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup(t)

	_, ownerID := env.registerAndLogin(t, auth.RoleUser)
	o := env.createDeliveredPaidOrder(t, ownerID)
	rr, err := env.repository.CreateRequest(context.Background(), o.ID, ownerID, RequestInput{Reason: "Defective"})
	if err != nil {
		t.Fatalf("setup: failed to create return request: %v", err)
	}

	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)
	idStr := strconv.FormatInt(rr.ID, 10)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/returns/"+idStr+"/reject", bytes.NewReader([]byte(`{}`)))
	req.SetPathValue("id", idStr)
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	env.handler.AdminReject(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected reject to succeed, got %d body=%s", w.Code, w.Body.String())
	}

	// A rejected return can never later become received/refunded — reject
	// again (or received) from "rejected" must fail.
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/admin/returns/"+idStr+"/received", bytes.NewReader([]byte(`{}`)))
	req2.SetPathValue("id", idStr)
	req2.AddCookie(adminCookie)
	w2 := httptest.NewRecorder()
	env.handler.AdminReceived(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Errorf("expected received-after-rejected to be rejected with 409, got %d", w2.Code)
	}
}

func TestHandlerAdminReceivedOnlyFromApproved(t *testing.T) {
	env := newTestHandlerEnv(t)
	defer env.cleanup(t)

	_, ownerID := env.registerAndLogin(t, auth.RoleUser)
	o := env.createDeliveredPaidOrder(t, ownerID)
	rr, err := env.repository.CreateRequest(context.Background(), o.ID, ownerID, RequestInput{Reason: "Defective"})
	if err != nil {
		t.Fatalf("setup: failed to create return request: %v", err)
	}

	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)
	idStr := strconv.FormatInt(rr.ID, 10)

	// Skip approve entirely: received directly from "requested" must fail.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/returns/"+idStr+"/received", bytes.NewReader([]byte(`{}`)))
	req.SetPathValue("id", idStr)
	req.AddCookie(adminCookie)
	w := httptest.NewRecorder()
	env.handler.AdminReceived(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("expected received-without-approve to be rejected with 409, got %d", w.Code)
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
