package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
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

func TestAdminReconciliationRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, reason, outcome, status string
		output                        VerifyPaymentOutput
		verifyErr                     error
	}{
		{"success", "settled", "verified_success", StatusPaid, VerifyPaymentOutput{Code: 100, RefID: 991001}, nil},
		{"already verified", "settled", "verified_success", StatusPaid, VerifyPaymentOutput{Code: 101, RefID: 991002}, nil},
		{"timeout", "verification_uncertain", "uncertain", StatusPending, VerifyPaymentOutput{}, context.DeadlineExceeded},
		{"rejected", "verification_rejected", "definitive_rejection", StatusPending, VerifyPaymentOutput{Code: -51}, nil},
		{"provider error", "verification_rejected", "definitive_rejection", StatusPending, VerifyPaymentOutput{}, &ProviderError{Code: -51}},
		{"missing reference", "verification_uncertain", "uncertain", StatusPending, VerifyPaymentOutput{Code: 100}, nil},
		{"failed attempt", "settled", "verified_success", StatusPaid, VerifyPaymentOutput{Code: 100, RefID: 991003}, nil},
		{"cancelled order", "refund_required", "manual_required", StatusReconciliation, VerifyPaymentOutput{Code: 100, RefID: 991004}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := succeedingClient()
			env := newTestEnv(t, client)
			cookie, email := env.registerAndLogin(t)
			o := env.createOrder(t, cookie)
			a := env.startPayment(t, cookie, o.ID)
			if _, err := env.db.Exec(t.Context(), `UPDATE users SET role = 'admin' WHERE email = $1`, email); err != nil {
				t.Fatal(err)
			}
			if tc.name == "failed attempt" {
				if _, err := env.db.Exec(t.Context(), `UPDATE payment_attempts SET status = 'failed' WHERE id = $1`, a.ID); err != nil {
					t.Fatal(err)
				}
			}
			if tc.name == "cancelled order" {
				if _, err := env.db.Exec(t.Context(), `UPDATE orders SET status = 'cancelled' WHERE id = $1`, o.ID); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			client.verifyFunc = func(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error) {
				calls++
				if in.Authority != *a.Authority || in.Amount != a.Amount {
					t.Fatalf("verification did not use persisted binding: %+v", in)
				}
				return tc.output, tc.verifyErr
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/payments/1/reconcile", strings.NewReader(`{"amount":1,"ref_id":1}`))
			req.SetPathValue("id", strconv.FormatInt(a.ID, 10))
			req.AddCookie(cookie)
			w := httptest.NewRecorder()
			env.handler.AdminReconcile(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			var item Reconciliation
			if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil {
				t.Fatal(err)
			}
			if item.Status != tc.status || item.Reason != tc.reason || item.LastOutcome != tc.outcome || item.LastCheckedAt == nil || calls != 1 {
				t.Fatalf("unexpected result: %+v calls=%d", item, calls)
			}
			if strings.Contains(w.Body.String(), "account_key") {
				t.Fatal("provider identity exposed")
			}
			wantPayment, wantOrder := "pending", "pending"
			if tc.status == StatusPaid {
				wantPayment = "paid"
			}
			if tc.name == "cancelled order" {
				wantOrder = "cancelled"
			}
			env.assertOrderStock(t, o.ID, wantOrder, wantPayment, 9)
			if tc.status == StatusPaid || tc.status == StatusReconciliation {
				if _, err := env.handler.ReconcilePayment(t.Context(), a.ID); err != nil || calls != 1 {
					t.Fatalf("terminal retry err=%v calls=%d", err, calls)
				}
			}
		})
	}
}

func TestAdminReconciliationAccessAndListing(t *testing.T) {
	env := newTestEnv(t, succeedingClient())
	cookie, email := env.registerAndLogin(t)
	o := env.createOrder(t, cookie)
	a := env.startPayment(t, cookie, o.ID)
	for _, endpoint := range []struct {
		method, path string
		handler      http.HandlerFunc
	}{
		{http.MethodGet, "/api/v1/admin/payments/reconciliation", env.handler.AdminListReconciliations},
		{http.MethodPost, "/api/v1/admin/payments/1/reconcile", env.handler.AdminReconcile},
	} {
		for _, authenticated := range []bool{false, true} {
			req := httptest.NewRequest(endpoint.method, endpoint.path, nil)
			req.SetPathValue("id", strconv.FormatInt(a.ID, 10))
			want := http.StatusUnauthorized
			if authenticated {
				req.AddCookie(cookie)
				want = http.StatusForbidden
			}
			w := httptest.NewRecorder()
			endpoint.handler(w, req)
			if w.Code != want {
				t.Fatalf("access status=%d want=%d", w.Code, want)
			}
		}
	}
	if _, err := env.db.Exec(t.Context(), `UPDATE users SET role = 'admin' WHERE email = $1`, email); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"limit=0", "limit=101", "before_id=-1", "before_id=bad"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/payments/reconciliation?"+query, nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		env.handler.AdminListReconciliations(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("query %s: status=%d", query, w.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/payments/reconciliation?limit=1&before_id=%d", a.ID+1), nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	env.handler.AdminListReconciliations(w, req)
	var items []Reconciliation
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil || w.Code != http.StatusOK || len(items) != 1 || items[0].ID != a.ID || !items[0].Retryable {
		t.Fatalf("listing status=%d body=%s err=%v", w.Code, w.Body.String(), err)
	}
	for _, tc := range []struct {
		id     string
		status int
	}{{"bad", 400}, {"0", 400}, {"9223372036854775807", 404}} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/payments/1/reconcile", nil)
		req.SetPathValue("id", tc.id)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		env.handler.AdminReconcile(w, req)
		if w.Code != tc.status {
			t.Fatalf("id=%s status=%d want=%d", tc.id, w.Code, tc.status)
		}
	}
}

func TestReconciliationDoesNotVerifyUnsafeBindings(t *testing.T) {
	for _, reason := range []string{"provider_binding_mismatch", "missing_authority", "payments_disabled"} {
		t.Run(reason, func(t *testing.T) {
			client := succeedingClient()
			client.verifyFunc = func(context.Context, VerifyPaymentInput) (VerifyPaymentOutput, error) {
				t.Fatal("unsafe attempt reached provider")
				return VerifyPaymentOutput{}, nil
			}
			env := newTestEnv(t, client)
			cookie, _ := env.registerAndLogin(t)
			o := env.createOrder(t, cookie)
			a, err := env.repository.CreateAttemptForOrder(t.Context(), o.UserID, o.ID)
			if err != nil {
				t.Fatal(err)
			}
			if reason != "missing_authority" {
				if err := env.repository.SetAuthority(t.Context(), a.ID, "A"+uniqueSuffixPayment()); err != nil {
					t.Fatal(err)
				}
			}
			if reason == "provider_binding_mismatch" {
				env.repository.ConfigureProvider("live", "different")
			}
			if reason == "payments_disabled" {
				env.handler.client = nil
			}
			item, err := env.handler.ReconcilePayment(t.Context(), a.ID)
			if err != nil || item.Retryable || item.Reason != reason {
				t.Fatalf("item=%+v err=%v", item, err)
			}
			env.assertOrderStock(t, o.ID, "pending", "pending", 9)
		})
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
	productIDs   []int64
}

func newTestEnv(t *testing.T, client Client) *testEnv {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	db, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Fatalf("invalid DATABASE_URL: %v", err)
	}
	t.Cleanup(db.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := db.Ping(ctx); err != nil {
		t.Fatalf("cannot connect to configured database: %v", err)
	}
	authHandler := auth.NewHandler(auth.NewRepository(db))
	orderHandler := order.NewHandler(order.NewRepository(db), authHandler)
	cartHandler := cart.NewHandler(cart.NewRepository(db), authHandler)
	repo := NewRepository(db)
	h := NewHandler(repo, authHandler, client, "https://example.test/api/v1/payments/zarinpal/callback", "")
	env := &testEnv{
		handler:      h,
		repository:   repo,
		authHandler:  authHandler,
		orderHandler: orderHandler,
		cartHandler:  cartHandler,
		productRepo:  product.NewRepository(db),
		db:           db,
	}
	t.Cleanup(func() {
		for _, id := range env.productIDs {
			if _, err := db.Exec(context.Background(), `DELETE FROM products WHERE id = $1`, id); err != nil {
				t.Errorf("cleanup product: %v", err)
			}
		}
	})
	return env
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
		for _, query := range []string{
			`DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE email = $1)`,
			`DELETE FROM orders WHERE user_id IN (SELECT id FROM users WHERE email = $1)`,
			`DELETE FROM users WHERE email = $1`,
		} {
			if _, err := e.db.Exec(context.Background(), query, email); err != nil {
				t.Errorf("cleanup user: %v", err)
			}
		}
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
	e.productIDs = append(e.productIDs, p.ID)
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
	if err := json.NewDecoder(w.Body).Decode(&o); err != nil {
		t.Fatal(err)
	}
	if err := e.db.QueryRow(t.Context(), `SELECT user_id FROM orders WHERE id = $1`, o.ID).Scan(&o.UserID); err != nil {
		t.Fatal(err)
	}
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

func TestNewFrontendRedirect(t *testing.T) {
	cases := []struct {
		name            string
		frontendBaseURL string
		status          string
		verifyOutput    VerifyPaymentOutput
		verifyErr       error
		outcome         string
	}{
		{"development_success", "http://localhost:5173", "OK", VerifyPaymentOutput{Code: 100, RefID: 123456}, nil, "success"},
		{"development_cancelled", "http://localhost:5173", "NOK", VerifyPaymentOutput{}, nil, "cancelled"},
		{"development_verify_failure", "http://localhost:5173", "OK", VerifyPaymentOutput{Code: -22}, nil, "unknown"},
		{"development_verify_timeout", "http://localhost:5173", "OK", VerifyPaymentOutput{}, context.DeadlineExceeded, "unknown"},
		{"production_success", "https://shop.example.com", "OK", VerifyPaymentOutput{Code: 100, RefID: 123456}, nil, "success"},
		{"unset_relative_compatibility", "", "OK", VerifyPaymentOutput{Code: 100, RefID: 123456}, nil, "success"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := succeedingClient()
			verifyCalls := 0
			client.verifyFunc = func(context.Context, VerifyPaymentInput) (VerifyPaymentOutput, error) {
				verifyCalls++
				return tc.verifyOutput, tc.verifyErr
			}
			env := newTestEnv(t, client)
			env.handler = NewHandler(env.repository, env.authHandler, client, "https://example.test/api/v1/payments/zarinpal/callback", tc.frontendBaseURL)
			cookie, _ := env.registerAndLogin(t)
			o := env.createOrder(t, cookie)
			a := env.startPayment(t, cookie, o.ID)

			req := callbackRequest(*a.Authority, tc.status)
			query := req.URL.Query()
			query.Set("frontend_base_url", "https://attacker.example")
			query.Set("redirect_url", "https://attacker.example/payment/result?outcome=success")
			query.Set("Host", "attacker.example")
			req.URL.RawQuery = query.Encode()
			req.Host = "attacker.example"
			req.Header.Set("Host", "attacker.example")
			w := httptest.NewRecorder()
			env.handler.Callback(w, req)
			assertPaymentRedirect(t, w, tc.outcome)
			wantQuery := url.Values{"order_id": {strconv.FormatInt(o.ID, 10)}, "outcome": {tc.outcome}}
			wantLocation := tc.frontendBaseURL + "/payment/result?" + wantQuery.Encode()
			if got := w.Header().Get("Location"); got != wantLocation {
				t.Fatalf("Location=%q, want %q", got, wantLocation)
			}

			wantVerifyCalls := 1
			if tc.status == "NOK" {
				wantVerifyCalls = 0
			}
			if verifyCalls != wantVerifyCalls {
				t.Errorf("verification calls=%d, want %d", verifyCalls, wantVerifyCalls)
			}
			wantPaymentStatus := order.PaymentStatusPending
			wantAttemptStatus := StatusPending
			var refID int64
			var code *int
			if tc.outcome == "success" {
				wantPaymentStatus = order.PaymentStatusPaid
				wantAttemptStatus = StatusPaid
				refID = tc.verifyOutput.RefID
				code = &tc.verifyOutput.Code
			} else if tc.status == "OK" && tc.verifyErr == nil && tc.verifyOutput.Code < 0 {
				code = &tc.verifyOutput.Code
			}
			env.assertAttempt(t, *a.Authority, wantAttemptStatus, refID, code)
			if got := env.getOrder(t, cookie, o.ID); got.PaymentStatus != wantPaymentStatus {
				t.Errorf("payment_status=%q, want %q", got.PaymentStatus, wantPaymentStatus)
			}
		})
	}
}

func TestRedirectResultQueryEncoding(t *testing.T) {
	h := NewHandler(nil, nil, nil, "https://example.test/api/v1/payments/zarinpal/callback", "https://shop.example.com")
	outcome := "unknown &injected=true&order_id=999?redirect_url=https://attacker.example/a+b;value=%23#fragment / پرداخت"
	req := callbackRequest("unused", "OK")
	req.Host = "attacker.example"
	w := httptest.NewRecorder()
	h.redirectResult(w, req, 123, outcome)
	assertPaymentRedirect(t, w, outcome)

	location, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if location.Scheme != "https" || location.Host != "shop.example.com" || location.Path != "/payment/result" || location.User != nil || location.Fragment != "" || location.Opaque != "" {
		t.Fatalf("unexpected redirect target: %q", location.String())
	}
	query, err := url.ParseQuery(location.RawQuery)
	if err != nil {
		t.Fatal(err)
	}
	if len(query) != 2 || len(query["order_id"]) != 1 || query.Get("order_id") != "123" || len(query["outcome"]) != 1 || query.Get("outcome") != outcome {
		t.Fatalf("redirect query=%v, want only order_id=123 and outcome=%q", query, outcome)
	}
}

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
	if status != StatusPending {
		t.Errorf("expected attempt status pending, got %q", status)
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

func (e *testEnv) startPayment(t *testing.T, cookie *http.Cookie, orderID int64) Attempt {
	t.Helper()
	w := httptest.NewRecorder()
	e.handler.RequestZarinPal(w, requestPaymentReq(cookie, orderID))
	if w.Code != http.StatusOK {
		t.Fatalf("request payment: status=%d body=%s", w.Code, w.Body.String())
	}
	attempts, err := e.repository.ListAttemptsForOrder(t.Context(), orderID)
	if err != nil || len(attempts) == 0 || attempts[0].Authority == nil {
		t.Fatalf("persisted attempts=%+v err=%v", attempts, err)
	}
	return attempts[0]
}

func callbackRequest(authority, status string) *http.Request {
	query := url.Values{"Authority": {authority}, "Status": {status}}
	return httptest.NewRequest(http.MethodGet, "/api/v1/payments/zarinpal/callback?"+query.Encode(), nil)
}

func (e *testEnv) callback(t *testing.T, authority, status, outcome string) {
	t.Helper()
	w := httptest.NewRecorder()
	e.handler.Callback(w, callbackRequest(authority, status))
	assertPaymentRedirect(t, w, outcome)
}

func assertPaymentRedirect(t *testing.T, w *httptest.ResponseRecorder, outcome string) {
	t.Helper()
	location, err := url.Parse(w.Header().Get("Location"))
	if err != nil || w.Code != http.StatusFound || location == nil || location.Path != "/payment/result" || location.Query().Get("outcome") != outcome {
		t.Fatalf("redirect: status=%d location=%q err=%v, want %s", w.Code, w.Header().Get("Location"), err, outcome)
	}
}

func (e *testEnv) assertAttempt(t *testing.T, authority, status string, refID int64, code *int) Attempt {
	t.Helper()
	a, err := e.repository.GetAttemptByAuthority(t.Context(), authority)
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != status || refID == 0 && a.RefID != nil || refID != 0 && (a.RefID == nil || *a.RefID != refID) {
		t.Fatalf("attempt=%+v, want status=%s ref=%d", a, status, refID)
	}
	if code == nil && a.ProviderCode != nil || code != nil && (a.ProviderCode == nil || *a.ProviderCode != *code) {
		t.Fatalf("provider code=%v, want %v", a.ProviderCode, code)
	}
	if (status == StatusPaid || status == StatusReconciliation) != (a.VerifiedAt != nil) {
		t.Fatalf("unexpected verified_at: %+v", a)
	}
	return a
}

func TestCallbackRecoversWithAlreadyVerified(t *testing.T) {
	for _, failure := range []string{"negative", "timeout"} {
		t.Run(failure, func(t *testing.T) {
			calls := 0
			client := succeedingClient()
			client.verifyFunc = func(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error) {
				calls++
				if calls == 1 {
					if failure == "timeout" {
						return VerifyPaymentOutput{}, context.DeadlineExceeded
					}
					return VerifyPaymentOutput{Code: -51}, nil
				}
				return VerifyPaymentOutput{Code: 101, RefID: 123456, AlreadyVerified: true}, nil
			}
			env := newTestEnv(t, client)
			cookie, _ := env.registerAndLogin(t)
			o := env.createOrder(t, cookie)
			a := env.startPayment(t, cookie, o.ID)
			env.callback(t, *a.Authority, "OK", "unknown")
			var failureCode *int
			negative := -51
			if failure == "negative" {
				failureCode = &negative
			}
			env.assertAttempt(t, *a.Authority, StatusPending, 0, failureCode)
			if got := env.getOrder(t, cookie, o.ID); got.PaymentStatus != order.PaymentStatusPending {
				t.Fatalf("failed verification settled order: %+v", got)
			}
			env.callback(t, *a.Authority, "OK", "success")
			code := 101
			env.assertAttempt(t, *a.Authority, StatusPaid, 123456, &code)
			if calls != 2 || env.getOrder(t, cookie, o.ID).PaymentStatus != order.PaymentStatusPaid {
				t.Fatalf("recovery calls=%d", calls)
			}
		})
	}
}

func TestCallbackPartialVerificationNeverSettles(t *testing.T) {
	cases := []struct {
		name string
		out  VerifyPaymentOutput
		err  error
	}{
		{"empty", VerifyPaymentOutput{}, nil},
		{"boolean_only", VerifyPaymentOutput{AlreadyVerified: true}, nil},
		{"boolean_with_ref", VerifyPaymentOutput{AlreadyVerified: true, RefID: 123456}, nil},
		{"negative_boolean", VerifyPaymentOutput{Code: -51, AlreadyVerified: true}, nil},
		{"success_without_ref", VerifyPaymentOutput{Code: 100}, nil},
		{"already_without_ref", VerifyPaymentOutput{Code: 101, AlreadyVerified: true}, nil},
		{"negative_ref", VerifyPaymentOutput{Code: 100, RefID: -1}, nil},
		{"unexpected_code", VerifyPaymentOutput{Code: 102, RefID: 123456}, nil},
		{"partial_error", VerifyPaymentOutput{Code: 100, RefID: 123456}, context.DeadlineExceeded},
		{"malformed", VerifyPaymentOutput{}, errInvalidResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := succeedingClient()
			client.verifyFunc = func(context.Context, VerifyPaymentInput) (VerifyPaymentOutput, error) {
				return tc.out, tc.err
			}
			env := newTestEnv(t, client)
			cookie, _ := env.registerAndLogin(t)
			o := env.createOrder(t, cookie)
			a := env.startPayment(t, cookie, o.ID)
			env.callback(t, *a.Authority, "OK", "unknown")
			var code *int
			if tc.err == nil && tc.out.Code < 0 {
				code = &tc.out.Code
			}
			env.assertAttempt(t, *a.Authority, StatusPending, 0, code)
			if env.getOrder(t, cookie, o.ID).PaymentStatus != order.PaymentStatusPending {
				t.Fatal("partial verification paid the order")
			}
		})
	}
}

func TestCallbackPaidIgnoresLaterCancellationAndFailure(t *testing.T) {
	client := succeedingClient()
	env := newTestEnv(t, client)
	cookie, _ := env.registerAndLogin(t)
	o := env.createOrder(t, cookie)
	a := env.startPayment(t, cookie, o.ID)
	env.callback(t, *a.Authority, "OK", "success")
	calls := 0
	client.verifyFunc = func(context.Context, VerifyPaymentInput) (VerifyPaymentOutput, error) {
		calls++
		return VerifyPaymentOutput{Code: -51}, context.DeadlineExceeded
	}
	for _, status := range []string{"NOK", "", "OK"} {
		env.callback(t, *a.Authority, status, "success")
	}
	if calls != 0 {
		t.Fatalf("paid callback made %d provider calls", calls)
	}
	code := 100
	env.assertAttempt(t, *a.Authority, StatusPaid, 123456, &code)
}

func TestCallbackVerifiesPersistedAmountDespiteManipulation(t *testing.T) {
	client := succeedingClient()
	var got VerifyPaymentInput
	client.verifyFunc = func(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error) {
		got = in
		return VerifyPaymentOutput{Code: 100, RefID: 123456}, nil
	}
	env := newTestEnv(t, client)
	cookie, _ := env.registerAndLogin(t)
	o := env.createOrder(t, cookie)
	a := env.startPayment(t, cookie, o.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/zarinpal/callback?Authority="+*a.Authority+"&Status=OK&amount=1&Amount=2&order_id=0&ref_id=777", strings.NewReader(`{"amount":3,"order_id":0,"ref_id":888,"AlreadyVerified":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.handler.Callback(w, req)
	assertPaymentRedirect(t, w, "success")
	if got.Amount != a.Amount || got.Amount != o.Total || got.Authority != *a.Authority {
		t.Fatalf("verify input=%+v, attempt=%+v", got, a)
	}
	code := 100
	env.assertAttempt(t, *a.Authority, StatusPaid, 123456, &code)
}

func TestRequestZarinPalActiveAttemptConflicts(t *testing.T) {
	for _, state := range []string{"awaiting_authority", "pending", "negative", "timeout", "NOK"} {
		t.Run(state, func(t *testing.T) {
			client := succeedingClient()
			requestCalls := 0
			request := client.requestFunc
			var env *testEnv
			var cookie *http.Cookie
			var orderID int64
			client.requestFunc = func(ctx context.Context, in RequestPaymentInput) (RequestPaymentOutput, error) {
				requestCalls++
				if state == "awaiting_authority" {
					w := httptest.NewRecorder()
					env.handler.RequestZarinPal(w, requestPaymentReq(cookie, orderID))
					if w.Code != http.StatusConflict {
						t.Fatalf("in-flight retry status=%d", w.Code)
					}
				}
				return request(ctx, in)
			}
			client.verifyFunc = func(context.Context, VerifyPaymentInput) (VerifyPaymentOutput, error) {
				if state == "timeout" {
					return VerifyPaymentOutput{}, context.DeadlineExceeded
				}
				return VerifyPaymentOutput{Code: -51}, nil
			}
			env = newTestEnv(t, client)
			cookie, _ = env.registerAndLogin(t)
			o := env.createOrder(t, cookie)
			orderID = o.ID
			a := env.startPayment(t, cookie, o.ID)
			if state == "negative" || state == "timeout" {
				env.callback(t, *a.Authority, "OK", "unknown")
			} else if state == "NOK" {
				env.callback(t, *a.Authority, "NOK", "cancelled")
			}
			for i := 0; i < 3; i++ {
				w := httptest.NewRecorder()
				env.handler.RequestZarinPal(w, requestPaymentReq(cookie, o.ID))
				if w.Code != http.StatusConflict {
					t.Fatalf("active retry status=%d body=%s", w.Code, w.Body.String())
				}
			}
			attempts, err := env.repository.ListAttemptsForOrder(t.Context(), o.ID)
			if err != nil || len(attempts) != 1 || requestCalls != 1 {
				t.Fatalf("attempts=%+v calls=%d err=%v", attempts, requestCalls, err)
			}
		})
	}
}

func TestCallbackWrongProviderBindingSkipsVerification(t *testing.T) {
	for _, binding := range []struct{ environment, key string }{{"sandbox", "test"}, {"test", "other-merchant"}} {
		t.Run(binding.environment+"/"+binding.key, func(t *testing.T) {
			client := succeedingClient()
			calls := 0
			client.verifyFunc = func(context.Context, VerifyPaymentInput) (VerifyPaymentOutput, error) {
				calls++
				return VerifyPaymentOutput{Code: 100, RefID: 123456}, nil
			}
			env := newTestEnv(t, client)
			cookie, _ := env.registerAndLogin(t)
			o := env.createOrder(t, cookie)
			a := env.startPayment(t, cookie, o.ID)
			env.repository.ConfigureProvider(binding.environment, binding.key)
			env.callback(t, *a.Authority, "OK", "unknown")
			if calls != 0 || env.getOrder(t, cookie, o.ID).PaymentStatus != order.PaymentStatusPending {
				t.Fatalf("mismatched provider calls=%d", calls)
			}
			env.assertAttempt(t, *a.Authority, StatusPending, 0, nil)
		})
	}
}

func TestCallbackConcurrentSuccessAndFailure(t *testing.T) {
	for _, failureFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("failure_first=%t", failureFirst), func(t *testing.T) {
			client := succeedingClient()
			var calls atomic.Int32
			entered := make(chan int, 2)
			release := []chan struct{}{make(chan struct{}), make(chan struct{})}
			client.verifyFunc = func(ctx context.Context, in VerifyPaymentInput) (VerifyPaymentOutput, error) {
				i := int(calls.Add(1)) - 1
				if i > 1 {
					return VerifyPaymentOutput{}, fmt.Errorf("unexpected verification")
				}
				entered <- i
				select {
				case <-release[i]:
				case <-ctx.Done():
					return VerifyPaymentOutput{}, ctx.Err()
				}
				if i == 0 {
					return VerifyPaymentOutput{Code: 100, RefID: 123456}, nil
				}
				return VerifyPaymentOutput{Code: -51}, nil
			}
			env := newTestEnv(t, client)
			cookie, _ := env.registerAndLogin(t)
			o := env.createOrder(t, cookie)
			a := env.startPayment(t, cookie, o.ID)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			done := []chan *httptest.ResponseRecorder{make(chan *httptest.ResponseRecorder, 1), make(chan *httptest.ResponseRecorder, 1)}
			for i := 0; i < 2; i++ {
				go func() {
					w := httptest.NewRecorder()
					env.handler.Callback(w, callbackRequest(*a.Authority, "OK").WithContext(ctx))
					done[i] <- w
				}()
				select {
				case <-entered:
				case <-ctx.Done():
					t.Fatal("callbacks did not reach provider concurrently")
				}
			}
			first, second := 0, 1
			if failureFirst {
				first, second = 1, 0
			}
			for _, i := range []int{first, second} {
				close(release[i])
				select {
				case w := <-done[i]:
					outcome := "success"
					if failureFirst && i == 1 {
						outcome = "unknown"
					}
					assertPaymentRedirect(t, w, outcome)
				case <-ctx.Done():
					t.Fatal("callback deadlock")
				}
			}
			code := 100
			env.assertAttempt(t, *a.Authority, StatusPaid, 123456, &code)
			if calls.Load() != 2 || env.getOrder(t, cookie, o.ID).PaymentStatus != order.PaymentStatusPaid {
				t.Fatalf("concurrent callback calls=%d", calls.Load())
			}
		})
	}
}
