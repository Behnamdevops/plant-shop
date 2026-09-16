package payment

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

// fakeClient is an in-memory Client used by every test in this file, so no
// test ever makes a real network call to ZarinPal. Behavior is controlled
// per-test via the exported fields/funcs below.
type fakeClient struct {
	requestFunc func(ctx context.Context, in RequestPaymentInput) (RequestPaymentOutput, error)
	verifyFunc  func(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error)
}

func (f *fakeClient) RequestPayment(ctx context.Context, in RequestPaymentInput) (RequestPaymentOutput, error) {
	return f.requestFunc(ctx, in)
}

func (f *fakeClient) VerifyPayment(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error) {
	return f.verifyFunc(ctx, in)
}

// succeedingClient returns a fakeClient whose RequestPayment always
// succeeds with a fresh, unique authority, and whose VerifyPayment always
// reports success (code 100) for whatever authority/amount it's given.
func succeedingClient() *fakeClient {
	return &fakeClient{
		requestFunc: func(ctx context.Context, in RequestPaymentInput) (RequestPaymentOutput, error) {
			authority := "A" + uniqueSuffixPayment()
			return RequestPaymentOutput{Authority: authority, RedirectURL: "https://sandbox.zarinpal.com/pg/StartPay/" + authority, Code: 100}, nil
		},
		verifyFunc: func(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error) {
			return VerifyPaymentOutput{Code: 100, RefID: 123456}, nil
		},
	}
}

// failingRequestClient returns a fakeClient whose RequestPayment always
// fails (simulating a provider/network failure), so the handler must not
// mark anything paid and must not corrupt order state.
func failingRequestClient() *fakeClient {
	return &fakeClient{
		requestFunc: func(ctx context.Context, in RequestPaymentInput) (RequestPaymentOutput, error) {
			return RequestPaymentOutput{}, fmt.Errorf("simulated network failure")
		},
		verifyFunc: func(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error) {
			t := VerifyPaymentOutput{}
			return t, fmt.Errorf("should not be called")
		},
	}
}

var paymentSeq int

func uniqueSuffixPayment() string {
	paymentSeq++
	return fmt.Sprintf("%d%d", time.Now().UnixNano(), paymentSeq)
}

type testEnv struct {
	handler      *Handler
	repository   *Repository
	authHandler  *auth.Handler
	orderHandler *order.Handler
	cartHandler  *cart.Handler
	productRepo  *product.Repository
	db           *pgxpool.Pool
}

func newTestEnv(t *testing.T, client Client) *testEnv {
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
	orderHandler := order.NewHandler(order.NewRepository(db), authHandler)
	cartHandler := cart.NewHandler(cart.NewRepository(db), authHandler)
	repo := NewRepository(db)
	h := NewHandler(repo, authHandler, client, "https://example.test/api/v1/payments/zarinpal/callback")
	return &testEnv{
		handler:      h,
		repository:   repo,
		authHandler:  authHandler,
		orderHandler: orderHandler,
		cartHandler:  cartHandler,
		productRepo:  product.NewRepository(db),
		db:           db,
	}
}

func (e *testEnv) registerAndLogin(t *testing.T) (*http.Cookie, string) {
	t.Helper()
	suffix := uniqueSuffixPayment()
	email := "paymentuser-" + suffix + "@example.com"
	body, _ := json.Marshal(map[string]any{
		"name":     "paymentuser-" + suffix,
		"email":    email,
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	e.authHandler.Register(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup registration failed: got %d", w.Code)
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

func (e *testEnv) createProduct(t *testing.T, stock int) product.Product {
	t.Helper()
	suffix := uniqueSuffixPayment()
	p, err := e.productRepo.Create(t.Context(), product.CreateProductInput{
		Name:  "Payment Product " + suffix,
		Slug:  "payment-product-" + suffix,
		Price: 20000,
		Stock: stock,
	})
	if err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	return p
}

func (e *testEnv) addToCart(t *testing.T, cookie *http.Cookie, productID int64, quantity int) {
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

func validCheckoutBody() []byte {
	body, _ := json.Marshal(map[string]any{
		"recipient_name":  "Test Recipient",
		"phone":           "+98 912 000 0000",
		"address_line1":   "Valiasr St",
		"address_line2":   "",
		"city":            "Tehran",
		"postal_code":     "1234567890",
		"country":         "Iran",
		"shipping_method": order.ShippingMethodStandard,
	})
	return body
}

// createOrder places a real order for cookie's user (one unit of a
// freshly created product), via the real order handler.
func (e *testEnv) createOrder(t *testing.T, cookie *http.Cookie) order.Order {
	t.Helper()
	p := e.createProduct(t, 10)
	e.addToCart(t, cookie, p.ID, 1)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewReader(validCheckoutBody()))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	e.orderHandler.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup order failed: got %d, body=%s", w.Code, w.Body.String())
	}
	var o order.Order
	json.NewDecoder(w.Body).Decode(&o)
	return o
}

func (e *testEnv) getOrder(t *testing.T, cookie *http.Cookie, orderID int64) order.OrderWithItems {
	t.Helper()
	idStr := strconv.FormatInt(orderID, 10)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+idStr, nil)
	req.SetPathValue("id", idStr)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	e.orderHandler.GetByID(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("getOrder failed: got %d, body=%s", w.Code, w.Body.String())
	}
	var o order.OrderWithItems
	json.NewDecoder(w.Body).Decode(&o)
	return o
}

func requestPaymentReq(cookie *http.Cookie, orderID int64) *http.Request {
	idStr := strconv.FormatInt(orderID, 10)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+idStr+"/payments/zarinpal", nil)
	req.SetPathValue("id", idStr)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

// ---------- Payment request ----------

func TestRequestZarinPalUnauthenticated(t *testing.T) {
	env := newTestEnv(t, succeedingClient())
	req := requestPaymentReq(nil, 1)
	w := httptest.NewRecorder()
	env.handler.RequestZarinPal(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRequestZarinPalAnotherUsersOrderNotFound(t *testing.T) {
	env := newTestEnv(t, succeedingClient())
	owner, _ := env.registerAndLogin(t)
	attacker, _ := env.registerAndLogin(t)
	o := env.createOrder(t, owner)

	req := requestPaymentReq(attacker, o.ID)
	w := httptest.NewRecorder()
	env.handler.RequestZarinPal(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

func TestRequestZarinPalPaidOrderConflict(t *testing.T) {
	env := newTestEnv(t, succeedingClient())
	cookie, _ := env.registerAndLogin(t)
	o := env.createOrder(t, cookie)

	_, err := env.db.Exec(t.Context(), `UPDATE orders SET payment_status = 'paid' WHERE id = $1`, o.ID)
	if err != nil {
		t.Fatalf("failed to force paid state: %v", err)
	}

	req := requestPaymentReq(cookie, o.ID)
	w := httptest.NewRecorder()
	env.handler.RequestZarinPal(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusConflict, w.Body.String())
	}
}

func TestRequestZarinPalAmountComesFromDB(t *testing.T) {
	var gotAmount int64
	client := &fakeClient{
		requestFunc: func(ctx context.Context, in RequestPaymentInput) (RequestPaymentOutput, error) {
			gotAmount = in.Amount
			authority := "A" + uniqueSuffixPayment()
			return RequestPaymentOutput{Authority: authority, RedirectURL: "https://sandbox.zarinpal.com/pg/StartPay/" + authority, Code: 100}, nil
		},
		verifyFunc: func(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error) {
			return VerifyPaymentOutput{Code: 100, RefID: 1}, nil
		},
	}
	env := newTestEnv(t, client)
	cookie, _ := env.registerAndLogin(t)
	o := env.createOrder(t, cookie)

	req := requestPaymentReq(cookie, o.ID)
	w := httptest.NewRecorder()
	env.handler.RequestZarinPal(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if gotAmount != o.Total {
		t.Errorf("expected amount %d (order total), got %d", o.Total, gotAmount)
	}
}

func TestRequestZarinPalProviderFailureDoesNotMarkPaid(t *testing.T) {
	env := newTestEnv(t, failingRequestClient())
	cookie, _ := env.registerAndLogin(t)
	o := env.createOrder(t, cookie)

	req := requestPaymentReq(cookie, o.ID)
	w := httptest.NewRecorder()
	env.handler.RequestZarinPal(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusServiceUnavailable, w.Body.String())
	}

	updated := env.getOrder(t, cookie, o.ID)
	if updated.PaymentStatus != order.PaymentStatusPending {
		t.Errorf("expected payment_status to remain pending, got %q", updated.PaymentStatus)
	}
}

func TestRequestZarinPalSuccessPersistsAuthority(t *testing.T) {
	env := newTestEnv(t, succeedingClient())
	cookie, _ := env.registerAndLogin(t)
	o := env.createOrder(t, cookie)

	req := requestPaymentReq(cookie, o.ID)
	w := httptest.NewRecorder()
	env.handler.RequestZarinPal(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["redirect_url"] == "" {
		t.Fatalf("expected redirect_url in response, got %v", resp)
	}

	var authority *string
	err := env.db.QueryRow(t.Context(), `SELECT authority FROM payment_attempts WHERE order_id = $1`, o.ID).Scan(&authority)
	if err != nil {
		t.Fatalf("failed to query persisted attempt: %v", err)
	}
	if authority == nil || *authority == "" {
		t.Fatalf("expected authority to be persisted, got %v", authority)
	}
}

func TestRequestZarinPalRetryAfterFailureCreatesNewAttempt(t *testing.T) {
	env := newTestEnv(t, failingRequestClient())
	cookie, _ := env.registerAndLogin(t)
	o := env.createOrder(t, cookie)

	req := requestPaymentReq(cookie, o.ID)
	w := httptest.NewRecorder()
	env.handler.RequestZarinPal(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	// Swap in a succeeding client for the retry (simulating the provider
	// recovering), then retry the same order.
	env.handler.client = succeedingClient()
	req2 := requestPaymentReq(cookie, o.ID)
	w2 := httptest.NewRecorder()
	env.handler.RequestZarinPal(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("retry got status %d, want %d, body=%s", w2.Code, http.StatusOK, w2.Body.String())
	}

	var count int
	err := env.db.QueryRow(t.Context(), `SELECT COUNT(*) FROM payment_attempts WHERE order_id = $1`, o.ID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count attempts: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 payment attempts (failed + retried), got %d", count)
	}
}

// ---------- Callback/verify ----------

func TestCallbackUnknownAuthorityHandledSafely(t *testing.T) {
	env := newTestEnv(t, succeedingClient())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/zarinpal/callback?Authority=Anonexistent&Status=OK", nil)
	w := httptest.NewRecorder()
	env.handler.Callback(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusFound)
	}
	loc := w.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected a Location redirect header")
	}
}

func TestCallbackStatusNotOKDoesNotVerify(t *testing.T) {
	verifyCalls := 0
	client := &fakeClient{
		requestFunc: succeedingClient().requestFunc,
		verifyFunc: func(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error) {
			verifyCalls++
			return VerifyPaymentOutput{Code: 100, RefID: 1}, nil
		},
	}
	env := newTestEnv(t, client)
	cookie, _ := env.registerAndLogin(t)
	o := env.createOrder(t, cookie)

	req := requestPaymentReq(cookie, o.ID)
	w := httptest.NewRecorder()
	env.handler.RequestZarinPal(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("setup request failed: got %d", w.Code)
	}
	var authority string
	env.db.QueryRow(t.Context(), `SELECT authority FROM payment_attempts WHERE order_id = $1`, o.ID).Scan(&authority)

	cbReq := httptest.NewRequest(http.MethodGet, "/api/v1/payments/zarinpal/callback?Authority="+authority+"&Status=NOK", nil)
	cbW := httptest.NewRecorder()
	env.handler.Callback(cbW, cbReq)
	if cbW.Code != http.StatusFound {
		t.Fatalf("got status %d, want %d", cbW.Code, http.StatusFound)
	}
	if verifyCalls != 0 {
		t.Errorf("expected VerifyPayment to never be called for Status=NOK, got %d calls", verifyCalls)
	}

	updated := env.getOrder(t, cookie, o.ID)
	if updated.PaymentStatus != order.PaymentStatusPending {
		t.Errorf("expected payment_status to remain pending, got %q", updated.PaymentStatus)
	}
}

func TestCallbackSuccessfulVerificationMarksOrderPaid(t *testing.T) {
	env := newTestEnv(t, succeedingClient())
	cookie, _ := env.registerAndLogin(t)
	o := env.createOrder(t, cookie)

	req := requestPaymentReq(cookie, o.ID)
	w := httptest.NewRecorder()
	env.handler.RequestZarinPal(w, req)
	var authority string
	env.db.QueryRow(t.Context(), `SELECT authority FROM payment_attempts WHERE order_id = $1`, o.ID).Scan(&authority)

	cbReq := httptest.NewRequest(http.MethodGet, "/api/v1/payments/zarinpal/callback?Authority="+authority+"&Status=OK", nil)
	cbW := httptest.NewRecorder()
	env.handler.Callback(cbW, cbReq)
	if cbW.Code != http.StatusFound {
		t.Fatalf("got status %d, want %d", cbW.Code, http.StatusFound)
	}

	updated := env.getOrder(t, cookie, o.ID)
	if updated.PaymentStatus != order.PaymentStatusPaid {
		t.Errorf("expected payment_status paid, got %q", updated.PaymentStatus)
	}
	if updated.PaymentMethod != order.PaymentMethodZarinPal {
		t.Errorf("expected payment_method zarinpal, got %q", updated.PaymentMethod)
	}

	var refID *int64
	env.db.QueryRow(t.Context(), `SELECT ref_id FROM payment_attempts WHERE authority = $1`, authority).Scan(&refID)
	if refID == nil || *refID != 123456 {
		t.Errorf("expected ref_id 123456 to be persisted, got %v", refID)
	}
}

func TestCallbackVerificationFailureDoesNotMarkPaid(t *testing.T) {
	client := &fakeClient{
		requestFunc: succeedingClient().requestFunc,
		verifyFunc: func(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error) {
			return VerifyPaymentOutput{Code: -22, Message: "transaction failed"}, nil
		},
	}
	env := newTestEnv(t, client)
	cookie, _ := env.registerAndLogin(t)
	o := env.createOrder(t, cookie)

	req := requestPaymentReq(cookie, o.ID)
	w := httptest.NewRecorder()
	env.handler.RequestZarinPal(w, req)
	var authority string
	env.db.QueryRow(t.Context(), `SELECT authority FROM payment_attempts WHERE order_id = $1`, o.ID).Scan(&authority)

	cbReq := httptest.NewRequest(http.MethodGet, "/api/v1/payments/zarinpal/callback?Authority="+authority+"&Status=OK", nil)
	cbW := httptest.NewRecorder()
	env.handler.Callback(cbW, cbReq)
	if cbW.Code != http.StatusFound {
		t.Fatalf("got status %d, want %d", cbW.Code, http.StatusFound)
	}

	updated := env.getOrder(t, cookie, o.ID)
	if updated.PaymentStatus != order.PaymentStatusPending {
		t.Errorf("expected payment_status to remain pending after failed verification, got %q", updated.PaymentStatus)
	}

	var status string
	env.db.QueryRow(t.Context(), `SELECT status FROM payment_attempts WHERE authority = $1`, authority).Scan(&status)
	if status != StatusFailed {
		t.Errorf("expected attempt status failed, got %q", status)
	}
}

func TestCallbackDuplicateIsIdempotent(t *testing.T) {
	verifyCalls := 0
	client := &fakeClient{
		requestFunc: succeedingClient().requestFunc,
		verifyFunc: func(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error) {
			verifyCalls++
			return VerifyPaymentOutput{Code: 100, RefID: 999}, nil
		},
	}
	env := newTestEnv(t, client)
	cookie, _ := env.registerAndLogin(t)
	o := env.createOrder(t, cookie)

	req := requestPaymentReq(cookie, o.ID)
	w := httptest.NewRecorder()
	env.handler.RequestZarinPal(w, req)
	var authority string
	env.db.QueryRow(t.Context(), `SELECT authority FROM payment_attempts WHERE order_id = $1`, o.ID).Scan(&authority)

	for i := 0; i < 3; i++ {
		cbReq := httptest.NewRequest(http.MethodGet, "/api/v1/payments/zarinpal/callback?Authority="+authority+"&Status=OK", nil)
		cbW := httptest.NewRecorder()
		env.handler.Callback(cbW, cbReq)
		if cbW.Code != http.StatusFound {
			t.Fatalf("callback %d got status %d, want %d", i, cbW.Code, http.StatusFound)
		}
	}

	var refID *int64
	var attemptStatus string
	env.db.QueryRow(t.Context(), `SELECT ref_id, status FROM payment_attempts WHERE authority = $1`, authority).Scan(&refID, &attemptStatus)
	if refID == nil || *refID != 999 {
		t.Errorf("expected ref_id 999 to remain stable across duplicate callbacks, got %v", refID)
	}
	if attemptStatus != StatusPaid {
		t.Errorf("expected attempt to remain paid, got %q", attemptStatus)
	}

	// Stock/order count sanity: only one order exists and its total stock
	// deduction happened exactly once at checkout (duplicate callbacks
	// must never re-touch stock).
	var orderCount int
	env.db.QueryRow(t.Context(), `SELECT COUNT(*) FROM orders WHERE id = $1`, o.ID).Scan(&orderCount)
	if orderCount != 1 {
		t.Errorf("expected exactly 1 order, got %d", orderCount)
	}
}

func TestCallbackCannotPayAnotherOrderWithSameAuthority(t *testing.T) {
	env := newTestEnv(t, succeedingClient())
	cookieA, _ := env.registerAndLogin(t)
	cookieB, _ := env.registerAndLogin(t)
	orderA := env.createOrder(t, cookieA)
	orderB := env.createOrder(t, cookieB)

	req := requestPaymentReq(cookieA, orderA.ID)
	w := httptest.NewRecorder()
	env.handler.RequestZarinPal(w, req)
	var authority string
	env.db.QueryRow(t.Context(), `SELECT authority FROM payment_attempts WHERE order_id = $1`, orderA.ID).Scan(&authority)

	cbReq := httptest.NewRequest(http.MethodGet, "/api/v1/payments/zarinpal/callback?Authority="+authority+"&Status=OK", nil)
	cbW := httptest.NewRecorder()
	env.handler.Callback(cbW, cbReq)
	if cbW.Code != http.StatusFound {
		t.Fatalf("got status %d, want %d", cbW.Code, http.StatusFound)
	}

	updatedA := env.getOrder(t, cookieA, orderA.ID)
	if updatedA.PaymentStatus != order.PaymentStatusPaid {
		t.Fatalf("expected order A to be paid, got %q", updatedA.PaymentStatus)
	}
	updatedB := env.getOrder(t, cookieB, orderB.ID)
	if updatedB.PaymentStatus != order.PaymentStatusPending {
		t.Errorf("expected order B to remain unaffected (pending), got %q", updatedB.PaymentStatus)
	}
}

func TestCallbackMalformedProviderResponseFailsSafely(t *testing.T) {
	client := &fakeClient{
		requestFunc: succeedingClient().requestFunc,
		verifyFunc: func(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error) {
			return VerifyPaymentOutput{}, fmt.Errorf("zarinpal: malformed response body")
		},
	}
	env := newTestEnv(t, client)
	cookie, _ := env.registerAndLogin(t)
	o := env.createOrder(t, cookie)

	req := requestPaymentReq(cookie, o.ID)
	w := httptest.NewRecorder()
	env.handler.RequestZarinPal(w, req)
	var authority string
	env.db.QueryRow(t.Context(), `SELECT authority FROM payment_attempts WHERE order_id = $1`, o.ID).Scan(&authority)

	cbReq := httptest.NewRequest(http.MethodGet, "/api/v1/payments/zarinpal/callback?Authority="+authority+"&Status=OK", nil)
	cbW := httptest.NewRecorder()
	env.handler.Callback(cbW, cbReq)
	if cbW.Code != http.StatusFound {
		t.Fatalf("got status %d, want %d", cbW.Code, http.StatusFound)
	}

	updated := env.getOrder(t, cookie, o.ID)
	if updated.PaymentStatus != order.PaymentStatusPending {
		t.Errorf("expected payment_status to remain pending after malformed provider response, got %q", updated.PaymentStatus)
	}
}
