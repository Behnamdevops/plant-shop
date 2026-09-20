package refund

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
	returnpkg "github.com/Behnamdevops/plant-shop/backend/internal/return"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testHandlerEnv wires the refund Handler against a real return.Repository
// (not a stub), exactly as main.go does, so these tests exercise the real
// cross-package gating that prevents a refund from bypassing the return
// workflow.
type testHandlerEnv struct {
	handler      *Handler
	repository   *Repository
	returnRepo   *returnpkg.Repository
	authHandler  *auth.Handler
	orderRepo    *order.Repository
	productRepo  *product.Repository
	cartRepo     *cart.Repository
	fakeProvider *FakeRefundProvider
	db           *pgxpool.Pool
}

func newTestHandlerEnv(t *testing.T, providerSucceeds bool) *testHandlerEnv {
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
	returnRepo := returnpkg.NewRepository(db, orderRepo)
	repository := NewRepository(db, orderRepo)
	authHandler := auth.NewHandler(auth.NewRepository(db))
	provider := NewFakeRefundProvider(providerSucceeds)
	handler := NewHandler(repository, provider, authHandler, returnRepo)

	return &testHandlerEnv{
		handler:      handler,
		repository:   repository,
		returnRepo:   returnRepo,
		authHandler:  authHandler,
		orderRepo:    orderRepo,
		productRepo:  productRepo,
		cartRepo:     cartRepo,
		fakeProvider: provider,
		db:           db,
	}
}

func (e *testHandlerEnv) cleanup(t *testing.T) {
	t.Helper()
	e.db.Exec(context.Background(), `DELETE FROM payment_attempts WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%refhandler%'))`)
	e.db.Exec(context.Background(), `DELETE FROM refunds WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%refhandler%'))`)
	e.db.Exec(context.Background(), `DELETE FROM return_requests WHERE order_id IN (SELECT id FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%refhandler%'))`)
	e.db.Exec(context.Background(), `DELETE FROM cart_items WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%refhandler%')`)
	e.db.Exec(context.Background(), `DELETE FROM orders WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%refhandler%')`)
	e.db.Exec(context.Background(), `DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE email LIKE '%refhandler%')`)
	e.db.Exec(context.Background(), `DELETE FROM users WHERE email LIKE '%refhandler%'`)
}

var handlerTestSeq int

func uniqueSuffix() string {
	handlerTestSeq++
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), handlerTestSeq)
}

func (e *testHandlerEnv) registerAndLogin(t *testing.T, role string) (*http.Cookie, int64) {
	t.Helper()
	suffix := uniqueSuffix()
	email := "refhandler-" + suffix + "@example.com"
	body, _ := json.Marshal(map[string]any{
		"name":     "refhandler-" + suffix,
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

// setupOrderAtStatus creates a delivered/paid order, a return request, and
// drives the return request to returnStatus. If returnStatus requires a
// refund row to exist (refund_pending), one is created via the refund
// repository so tests can exercise the handler against realistic state.
func (e *testHandlerEnv) setupOrderAtStatus(t *testing.T, userID int64, returnStatus string) (order.Order, *returnpkg.ReturnRequest) {
	t.Helper()
	p, err := e.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Refund Handler Test Product",
		Slug:  "refund-handler-product-" + uniqueSuffix(),
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

	rr, err := e.returnRepo.CreateRequest(context.Background(), o.ID, userID, returnpkg.RequestInput{Reason: "Defective"})
	if err != nil {
		t.Fatalf("failed to create return request: %v", err)
	}

	if returnStatus == returnpkg.StatusRequested {
		return o, rr
	}

	rr, err = e.returnRepo.UpdateStatus(context.Background(), rr.ID, returnpkg.StatusApproved, nil, nil)
	if err != nil {
		t.Fatalf("failed to approve return request: %v", err)
	}
	if returnStatus == returnpkg.StatusApproved {
		return o, rr
	}

	rr, err = e.returnRepo.UpdateStatus(context.Background(), rr.ID, returnpkg.StatusReceived, nil, nil)
	if err != nil {
		t.Fatalf("failed to mark return request received: %v", err)
	}
	if returnStatus == returnpkg.StatusReceived {
		return o, rr
	}

	if returnStatus == returnpkg.StatusRefundPending {
		if _, err := e.repository.Create(context.Background(), o.ID, &rr.ID, o.Total, "zarinpal", nil); err != nil {
			t.Fatalf("failed to create refund: %v", err)
		}
		rr, err = e.returnRepo.UpdateStatus(context.Background(), rr.ID, returnpkg.StatusRefundPending, nil, nil)
		if err != nil {
			t.Fatalf("failed to mark return request refund_pending: %v", err)
		}
		return o, rr
	}

	t.Fatalf("unsupported returnStatus %q in test setup", returnStatus)
	return o, rr
}

func postRefund(env *testHandlerEnv, returnRequestID int64, cookie *http.Cookie) *httptest.ResponseRecorder {
	idStr := strconv.FormatInt(returnRequestID, 10)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/returns/"+idStr+"/refund", nil)
	req.SetPathValue("id", idStr)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	env.handler.AdminRefund(w, req)
	return w
}

// ---------- Bypass prevention (task 4 / issue: direct-refund bypass) ----------

func TestAdminRefund_CannotBypassReturnWorkflow_FromRequested(t *testing.T) {
	env := newTestHandlerEnv(t, true)
	defer env.cleanup(t)

	_, userID := env.registerAndLogin(t, auth.RoleUser)
	_, rr := env.setupOrderAtStatus(t, userID, returnpkg.StatusRequested)

	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)

	w := postRefund(env, rr.ID, adminCookie)
	if w.Code != http.StatusNotFound && w.Code != http.StatusConflict {
		t.Fatalf("expected refund to be rejected before a refund row exists, got %d body=%s", w.Code, w.Body.String())
	}
	if env.fakeProvider.calls() != 0 {
		t.Errorf("provider must never be called when bypassing the return workflow, got %d calls", env.fakeProvider.calls())
	}
}

func TestAdminRefund_CannotBypassReturnWorkflow_FromApproved(t *testing.T) {
	env := newTestHandlerEnv(t, true)
	defer env.cleanup(t)

	_, userID := env.registerAndLogin(t, auth.RoleUser)
	_, rr := env.setupOrderAtStatus(t, userID, returnpkg.StatusApproved)

	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)

	w := postRefund(env, rr.ID, adminCookie)
	if w.Code != http.StatusConflict && w.Code != http.StatusNotFound {
		t.Fatalf("expected refund to be rejected from 'approved' (not yet received), got %d body=%s", w.Code, w.Body.String())
	}
	if env.fakeProvider.calls() != 0 {
		t.Errorf("provider must never be called before the item is received, got %d calls", env.fakeProvider.calls())
	}

	// Confirm the order is still paid — no money was moved.
	var paymentStatus string
	if err := env.db.QueryRow(context.Background(), `SELECT payment_status FROM orders WHERE id = $1`, rr.OrderID).Scan(&paymentStatus); err != nil {
		t.Fatalf("failed to query payment_status: %v", err)
	}
	if paymentStatus != order.PaymentStatusPaid {
		t.Errorf("expected order to remain paid, got %q", paymentStatus)
	}
}

func TestAdminRefund_RequiresReceivedOrRefundPending_NotJustPaid(t *testing.T) {
	env := newTestHandlerEnv(t, true)
	defer env.cleanup(t)

	// A merely "paid" order (no return request in a refundable state at
	// all) must never be refundable through the admin refund endpoint.
	_, userID := env.registerAndLogin(t, auth.RoleUser)
	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)

	p, err := env.productRepo.Create(context.Background(), product.CreateProductInput{
		Name:  "Refund Handler Bypass Product",
		Slug:  "refund-handler-bypass-" + uniqueSuffix(),
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
	if _, err := env.db.Exec(context.Background(), `UPDATE orders SET payment_status = $1 WHERE id = $2`, order.PaymentStatusPaid, o.ID); err != nil {
		t.Fatalf("failed to mark order paid: %v", err)
	}

	// There is no return request at all for this order. Attempting to
	// refund via a made-up return request id must fail, never reach the
	// provider, and never touch payment_status.
	w := postRefund(env, 999999999, adminCookie)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent return request, got %d", w.Code)
	}
	if env.fakeProvider.calls() != 0 {
		t.Errorf("provider must never be called, got %d calls", env.fakeProvider.calls())
	}

	var paymentStatus string
	if err := env.db.QueryRow(context.Background(), `SELECT payment_status FROM orders WHERE id = $1`, o.ID).Scan(&paymentStatus); err != nil {
		t.Fatalf("failed to query payment_status: %v", err)
	}
	if paymentStatus != order.PaymentStatusPaid {
		t.Errorf("expected order to remain paid, got %q", paymentStatus)
	}
}

// ---------- Auth ----------

func TestAdminRefund_Unauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t, true)
	defer env.cleanup(t)

	w := postRefund(env, 1, nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAdminRefund_NonAdminForbidden(t *testing.T) {
	env := newTestHandlerEnv(t, true)
	defer env.cleanup(t)

	userCookie, _ := env.registerAndLogin(t, auth.RoleUser)
	w := postRefund(env, 1, userCookie)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// ---------- Successful refund reaches the real workflow ----------

func TestAdminRefund_FromReceivedReachesProviderAndSucceeds(t *testing.T) {
	env := newTestHandlerEnv(t, true)
	defer env.cleanup(t)

	_, userID := env.registerAndLogin(t, auth.RoleUser)
	o, rr := env.setupOrderAtStatus(t, userID, returnpkg.StatusReceived)

	if _, err := env.repository.Create(context.Background(), o.ID, &rr.ID, o.Total, "zarinpal", nil); err != nil {
		t.Fatalf("failed to create refund: %v", err)
	}

	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)
	w := postRefund(env, rr.ID, adminCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("expected refund to succeed, got %d body=%s", w.Code, w.Body.String())
	}
	if env.fakeProvider.calls() != 1 {
		t.Errorf("expected exactly 1 provider call, got %d", env.fakeProvider.calls())
	}

	var paymentStatus string
	if err := env.db.QueryRow(context.Background(), `SELECT payment_status FROM orders WHERE id = $1`, o.ID).Scan(&paymentStatus); err != nil {
		t.Fatalf("failed to query payment_status: %v", err)
	}
	if paymentStatus != order.PaymentStatusRefunded {
		t.Errorf("expected order payment_status %q, got %q", order.PaymentStatusRefunded, paymentStatus)
	}
}

// ---------- Idempotency: repeated refund action cannot double execute ----------

func TestAdminRefund_RepeatedCallCannotDoubleExecute(t *testing.T) {
	env := newTestHandlerEnv(t, true)
	defer env.cleanup(t)

	_, userID := env.registerAndLogin(t, auth.RoleUser)
	o, rr := env.setupOrderAtStatus(t, userID, returnpkg.StatusReceived)
	if _, err := env.repository.Create(context.Background(), o.ID, &rr.ID, o.Total, "zarinpal", nil); err != nil {
		t.Fatalf("failed to create refund: %v", err)
	}

	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)

	w1 := postRefund(env, rr.ID, adminCookie)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected first refund call to succeed, got %d body=%s", w1.Code, w1.Body.String())
	}

	w2 := postRefund(env, rr.ID, adminCookie)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected repeated refund call to be handled idempotently with 200, got %d body=%s", w2.Code, w2.Body.String())
	}

	if env.fakeProvider.calls() != 1 {
		t.Errorf("expected exactly 1 provider call across both requests, got %d", env.fakeProvider.calls())
	}
}

// ---------- Manual payment method cannot use the generic refund action ----------

func TestAdminRefund_RejectsManualPaymentMethod(t *testing.T) {
	env := newTestHandlerEnv(t, true)
	defer env.cleanup(t)

	_, userID := env.registerAndLogin(t, auth.RoleUser)
	o, rr := env.setupOrderAtStatus(t, userID, returnpkg.StatusReceived)
	if _, err := env.repository.Create(context.Background(), o.ID, &rr.ID, o.Total, "manual", nil); err != nil {
		t.Fatalf("failed to create refund: %v", err)
	}

	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)
	w := postRefund(env, rr.ID, adminCookie)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected manual payment method to be rejected by the generic refund action, got %d", w.Code)
	}
	if env.fakeProvider.calls() != 0 {
		t.Errorf("provider must never be called for manual payment methods, got %d calls", env.fakeProvider.calls())
	}
}

// ---------- Manual refund confirmation ----------

func TestAdminManualRefund_RequiresExplicitConfirmationAndIsIdempotent(t *testing.T) {
	env := newTestHandlerEnv(t, true)
	defer env.cleanup(t)

	_, userID := env.registerAndLogin(t, auth.RoleUser)
	o, rr := env.setupOrderAtStatus(t, userID, returnpkg.StatusReceived)
	if _, err := env.repository.Create(context.Background(), o.ID, &rr.ID, o.Total, "manual", nil); err != nil {
		t.Fatalf("failed to create refund: %v", err)
	}

	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)
	idStr := strconv.FormatInt(rr.ID, 10)

	confirm := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/returns/"+idStr+"/refund-manual", nil)
		req.SetPathValue("id", idStr)
		req.AddCookie(adminCookie)
		w := httptest.NewRecorder()
		env.handler.AdminManualRefund(w, req)
		return w
	}

	w1 := confirm()
	if w1.Code != http.StatusOK {
		t.Fatalf("expected manual refund confirmation to succeed, got %d body=%s", w1.Code, w1.Body.String())
	}

	var paymentStatus string
	if err := env.db.QueryRow(context.Background(), `SELECT payment_status FROM orders WHERE id = $1`, o.ID).Scan(&paymentStatus); err != nil {
		t.Fatalf("failed to query payment_status: %v", err)
	}
	if paymentStatus != order.PaymentStatusRefunded {
		t.Errorf("expected order payment_status %q, got %q", order.PaymentStatusRefunded, paymentStatus)
	}

	// Repeated confirmation must not error and must not double-process —
	// the fake provider is never called by the manual path, so we assert
	// idempotency by checking the refund stays in a single succeeded row.
	w2 := confirm()
	if w2.Code != http.StatusOK {
		t.Errorf("expected repeated manual refund confirmation to be idempotent (200), got %d body=%s", w2.Code, w2.Body.String())
	}

	var refundCount int
	if err := env.db.QueryRow(context.Background(), `SELECT COUNT(*) FROM refunds WHERE order_id = $1 AND status = 'succeeded'`, o.ID).Scan(&refundCount); err != nil {
		t.Fatalf("failed to count succeeded refunds: %v", err)
	}
	if refundCount != 1 {
		t.Errorf("expected exactly 1 succeeded refund row, got %d", refundCount)
	}
}

func TestAdminManualRefund_Unauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t, true)
	defer env.cleanup(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/returns/1/refund-manual", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	env.handler.AdminManualRefund(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAdminManualRefund_NonAdminForbidden(t *testing.T) {
	env := newTestHandlerEnv(t, true)
	defer env.cleanup(t)

	userCookie, _ := env.registerAndLogin(t, auth.RoleUser)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/returns/1/refund-manual", nil)
	req.SetPathValue("id", "1")
	req.AddCookie(userCookie)
	w := httptest.NewRecorder()
	env.handler.AdminManualRefund(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// ---------- Ambiguous provider outcome: manual_review, order stays paid ----------

func TestAdminRefund_AmbiguousProviderOutcomeStaysPaidAndIsNotRetriedAsSuccess(t *testing.T) {
	env := newTestHandlerEnv(t, false) // provider always reports failure/manual_review
	defer env.cleanup(t)

	_, userID := env.registerAndLogin(t, auth.RoleUser)
	o, rr := env.setupOrderAtStatus(t, userID, returnpkg.StatusReceived)
	if _, err := env.repository.Create(context.Background(), o.ID, &rr.ID, o.Total, "zarinpal", nil); err != nil {
		t.Fatalf("failed to create refund: %v", err)
	}

	adminCookie, _ := env.registerAndLogin(t, auth.RoleAdmin)
	w := postRefund(env, rr.ID, adminCookie)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected manual_review conflict response, got %d body=%s", w.Code, w.Body.String())
	}

	var paymentStatus string
	if err := env.db.QueryRow(context.Background(), `SELECT payment_status FROM orders WHERE id = $1`, o.ID).Scan(&paymentStatus); err != nil {
		t.Fatalf("failed to query payment_status: %v", err)
	}
	if paymentStatus != order.PaymentStatusPaid {
		t.Errorf("expected order to remain paid after ambiguous/failed provider outcome, got %q", paymentStatus)
	}

	var refundStatus string
	if err := env.db.QueryRow(context.Background(), `SELECT status FROM refunds WHERE order_id = $1`, o.ID).Scan(&refundStatus); err != nil {
		t.Fatalf("failed to query refund status: %v", err)
	}
	if refundStatus != StatusManualReview {
		t.Errorf("expected refund status %q, got %q", StatusManualReview, refundStatus)
	}

	// Retrying must not blindly succeed either: manual_review only allows
	// succeeded/failed, not another processing attempt.
	w2 := postRefund(env, rr.ID, adminCookie)
	if w2.Code == http.StatusOK {
		t.Errorf("expected retry from manual_review to not silently succeed, got 200")
	}
}
