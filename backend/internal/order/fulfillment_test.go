package order

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
)

// ---------- ZarinPal fulfillment gate ----------

// TestHandlerAdminUpdateStatusUnpaidZarinPalOrderCannotAdvance verifies
// that an order whose payment_method is "zarinpal" (i.e. it went through
// the payment-request flow) but whose payment_status is still "pending"
// cannot be advanced to processing — this must be enforced server-side and
// return 409, independent of whether the frontend attempted to prevent it.
func TestHandlerAdminUpdateStatusUnpaidZarinPalOrderCannotAdvance(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	buyer, _ := env.registerAndLoginWithRole(t, auth.RoleUser)
	o := env.createOrderForUser(t, buyer)

	// Simulate having gone through the ZarinPal payment-request flow
	// without a successful verification yet (payment_method set, but
	// payment_status still pending) — this is exactly the state
	// payment.Repository.CreateAttemptForOrder leaves an order in before
	// a callback verifies it.
	if _, err := env.db.Exec(t.Context(), `UPDATE orders SET payment_method = 'zarinpal' WHERE id = $1`, o.ID); err != nil {
		t.Fatalf("failed to force zarinpal payment_method: %v", err)
	}

	idStr := strconv.FormatInt(o.ID, 10)
	body, _ := json.Marshal(map[string]any{"status": StatusProcessing})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusConflict, w.Body.String())
	}
}

// TestHandlerAdminUpdateStatusPaidZarinPalOrderCanAdvance verifies that
// once payment_status is "paid" (as a successful callback verification
// would set it), the same transition succeeds normally.
func TestHandlerAdminUpdateStatusPaidZarinPalOrderCanAdvance(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	buyer, _ := env.registerAndLoginWithRole(t, auth.RoleUser)
	o := env.createOrderForUser(t, buyer)

	if _, err := env.db.Exec(t.Context(), `UPDATE orders SET payment_method = 'zarinpal', payment_status = 'paid' WHERE id = $1`, o.ID); err != nil {
		t.Fatalf("failed to force paid zarinpal state: %v", err)
	}

	idStr := strconv.FormatInt(o.ID, 10)
	body, _ := json.Marshal(map[string]any{"status": StatusProcessing})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

// TestHandlerAdminUpdateStatusManualOrderUnaffectedByPaymentGate verifies
// the payment gate does not block "manual" orders, which have no
// server-verifiable payment step in V1.
func TestHandlerAdminUpdateStatusManualOrderUnaffectedByPaymentGate(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	buyer, _ := env.registerAndLoginWithRole(t, auth.RoleUser)
	o := env.createOrderForUser(t, buyer)
	// createOrderForUser already leaves payment_method = manual by default.

	idStr := strconv.FormatInt(o.ID, 10)
	body, _ := json.Marshal(map[string]any{"status": StatusProcessing})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/orders/"+idStr+"/status", bytes.NewReader(body))
	req.SetPathValue("id", idStr)
	req.AddCookie(admin)
	w := httptest.NewRecorder()
	env.handler.AdminUpdateStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

// ---------- Customer self-cancellation ----------

func TestHandlerCancelUnauthenticated(t *testing.T) {
	env := newTestHandlerEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/1/cancel", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	env.handler.Cancel(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandlerCancelRestoresStockExactlyOnce(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	p := env.createProduct(t, 10)
	env.addToCart(t, cookie, p.ID, 4)

	req := checkoutRequest(cookie)
	w := httptest.NewRecorder()
	env.handler.Create(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("setup order failed: got %d, body=%s", w.Code, w.Body.String())
	}
	var o Order
	json.NewDecoder(w.Body).Decode(&o)

	var stockAfterCheckout int
	env.db.QueryRow(t.Context(), `SELECT stock FROM products WHERE id = $1`, p.ID).Scan(&stockAfterCheckout)
	if stockAfterCheckout != 6 {
		t.Fatalf("expected stock 6 after checkout (10-4), got %d", stockAfterCheckout)
	}

	idStr := strconv.FormatInt(o.ID, 10)
	cancelReq := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+idStr+"/cancel", nil)
	cancelReq.SetPathValue("id", idStr)
	cancelReq.AddCookie(cookie)
	cancelW := httptest.NewRecorder()
	env.handler.Cancel(cancelW, cancelReq)
	if cancelW.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d, body=%s", cancelW.Code, http.StatusOK, cancelW.Body.String())
	}

	var stockAfterCancel int
	env.db.QueryRow(t.Context(), `SELECT stock FROM products WHERE id = $1`, p.ID).Scan(&stockAfterCancel)
	if stockAfterCancel != 10 {
		t.Fatalf("expected stock restored to 10, got %d", stockAfterCancel)
	}

	// Cancelling again must be rejected (terminal state) and must not
	// restore stock a second time.
	cancelReq2 := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+idStr+"/cancel", nil)
	cancelReq2.SetPathValue("id", idStr)
	cancelReq2.AddCookie(cookie)
	cancelW2 := httptest.NewRecorder()
	env.handler.Cancel(cancelW2, cancelReq2)
	if cancelW2.Code != http.StatusConflict {
		t.Fatalf("expected second cancel to be rejected with 409, got %d", cancelW2.Code)
	}

	var stockAfterSecondAttempt int
	env.db.QueryRow(t.Context(), `SELECT stock FROM products WHERE id = $1`, p.ID).Scan(&stockAfterSecondAttempt)
	if stockAfterSecondAttempt != 10 {
		t.Fatalf("expected stock to remain 10 after rejected re-cancel, got %d", stockAfterSecondAttempt)
	}
}

func TestHandlerCancelAnotherUsersOrderNotFound(t *testing.T) {
	env := newTestHandlerEnv(t)
	owner := env.registerAndLogin(t)
	attacker := env.registerAndLogin(t)
	o := env.createOrderForUser(t, owner)

	idStr := strconv.FormatInt(o.ID, 10)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+idStr+"/cancel", nil)
	req.SetPathValue("id", idStr)
	req.AddCookie(attacker)
	w := httptest.NewRecorder()
	env.handler.Cancel(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandlerCancelPaidOrderRejected(t *testing.T) {
	env := newTestHandlerEnv(t)
	cookie := env.registerAndLogin(t)
	o := env.createOrderForUser(t, cookie)

	if _, err := env.db.Exec(t.Context(), `UPDATE orders SET payment_status = 'paid' WHERE id = $1`, o.ID); err != nil {
		t.Fatalf("failed to force paid state: %v", err)
	}

	idStr := strconv.FormatInt(o.ID, 10)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+idStr+"/cancel", nil)
	req.SetPathValue("id", idStr)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	env.handler.Cancel(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("got status %d, want %d, body=%s", w.Code, http.StatusConflict, w.Body.String())
	}
}

func TestHandlerCancelShippedOrderRejected(t *testing.T) {
	env := newTestHandlerEnv(t)
	admin, _ := env.registerAndLoginWithRole(t, auth.RoleAdmin)
	buyer := env.registerAndLogin(t)
	o := env.createOrderForUser(t, buyer)
	idStr := strconv.FormatInt(o.ID, 10)

	for _, status := range []string{StatusProcessing, StatusShipped} {
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

	cancelReq := httptest.NewRequest(http.MethodPost, "/api/v1/orders/"+idStr+"/cancel", nil)
	cancelReq.SetPathValue("id", idStr)
	cancelReq.AddCookie(buyer)
	cancelW := httptest.NewRecorder()
	env.handler.Cancel(cancelW, cancelReq)
	if cancelW.Code != http.StatusConflict {
		t.Fatalf("got status %d, want %d, body=%s", cancelW.Code, http.StatusConflict, cancelW.Body.String())
	}
}
