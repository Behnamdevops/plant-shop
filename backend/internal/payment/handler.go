package payment

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
)

// authenticator is satisfied by auth.Handler. Mirrors the identical local
// interface in the order/cart packages: this package needs to authenticate
// a customer for the payment-request endpoint and enforce admin access for
// AdminListAttempts, but never for the public callback (which ZarinPal's
// servers/the customer's browser hit without a session in general — see
// Callback below).
type authenticator interface {
	Authenticate(r *http.Request) (int64, error)
	RequireAdmin(r *http.Request) (int64, error)
}

// Handler wires the payment_attempts repository and a ZarinPal Client
// together into HTTP handlers.
type Handler struct {
	repository  *Repository
	auth        authenticator
	client      Client
	merchantID  string
	callbackURL string
	// resultBaseURL is where the browser is redirected after the callback
	// finishes processing, e.g. "/payment/result" (relative — resolved
	// against whatever origin served the frontend, since the backend is
	// only ever reached through the same origin/reverse proxy in this
	// project; see docker-compose.prod.yml).
	resultBaseURL string
}

// NewHandler constructs a payment Handler. callbackURL is the full,
// publicly reachable HTTPS URL ZarinPal should redirect the browser back
// to after the gateway flow completes (see ZARINPAL_CALLBACK_URL in
// backend/.env.example) — it must point at this backend's
// /api/v1/payments/zarinpal/callback route.
func NewHandler(repository *Repository, authHandler *auth.Handler, client Client, callbackURL string) *Handler {
	return &Handler{
		repository:    repository,
		auth:          authHandler,
		client:        client,
		callbackURL:   callbackURL,
		resultBaseURL: "/payment/result",
	}
}

// RequestZarinPal starts a new ZarinPal payment attempt for order {id} and
// returns a redirect URL for the frontend to send the browser to. Only the
// order's owner may call this; the amount charged is always the order's
// persisted total, never anything supplied by the client.
func (h *Handler) RequestZarinPal(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := r.PathValue("id")
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || orderID <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	attempt, err := h.repository.CreateAttemptForOrder(r.Context(), userID, orderID)
	if err != nil {
		switch {
		case errors.Is(err, ErrOrderNotFound):
			http.Error(w, "order not found", http.StatusNotFound)
		case errors.Is(err, ErrOrderAlreadyPaid):
			http.Error(w, "order is already paid", http.StatusConflict)
		case errors.Is(err, ErrOrderNotPayable):
			http.Error(w, "order is not payable", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	out, err := h.client.RequestPayment(r.Context(), RequestPaymentInput{
		Amount:      attempt.Amount,
		Description: fmt.Sprintf("Order #%d", attempt.OrderID),
		CallbackURL: h.callbackURL,
	})
	if err != nil {
		// The attempt row already exists (status "pending", no authority).
		// Mark it failed so it's not left in limbo; the customer can
		// retry, which creates a fresh attempt. Never log the error's full
		// text if it could echo back request payload — this client never
		// includes the Merchant ID in returned errors, only in the request
		// body itself, so logging err here is safe, but we still avoid
		// logging the raw response body.
		log.Printf("payment: zarinpal request failed for order %d: %v", attempt.OrderID, err)
		if markErr := h.repository.MarkAttemptFailed(r.Context(), attempt.ID, nil); markErr != nil {
			log.Printf("payment: failed to mark attempt %d failed: %v", attempt.ID, markErr)
		}
		http.Error(w, "payment provider is currently unavailable, please try again", http.StatusServiceUnavailable)
		return
	}

	if err := h.repository.SetAuthority(r.Context(), attempt.ID, out.Authority); err != nil {
		log.Printf("payment: failed to persist authority for attempt %d: %v", attempt.ID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"redirect_url": out.RedirectURL,
	})
}

// Callback is the public endpoint ZarinPal redirects the customer's
// browser to after the gateway flow completes (success, failure, or
// cancellation). It is unauthenticated by necessity — a browser redirect
// carries no session context that ZarinPal can be trusted to preserve —
// but it never trusts anything from the query string except Authority,
// which is used purely to look up our own persisted attempt/order; the
// amount verified is always read from that persisted row. The browser is
// always redirected onward to the frontend's payment result page rather
// than shown any raw JSON, so provider errors/secrets are never exposed in
// a query string here.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	authority := r.URL.Query().Get("Authority")
	status := r.URL.Query().Get("Status")

	if authority == "" {
		h.redirectResult(w, r, 0, "unknown")
		return
	}

	attempt, err := h.repository.GetAttemptByAuthority(r.Context(), authority)
	if err != nil {
		// Unknown authority: never guess which order this might belong to.
		h.redirectResult(w, r, 0, "unknown")
		return
	}

	if status != "OK" {
		// ZarinPal reports the customer cancelled or the gateway declined
		// before any charge occurred. This is not verified server-to-
		// server (there is nothing to verify), but it's also never
		// trusted to mark the order paid — only ever used to mark the
		// attempt failed so a retry is possible.
		if _, err := h.repository.FinalizeFailedPayment(r.Context(), authority, nil); err != nil {
			log.Printf("payment: failed to finalize cancelled/declined attempt for authority: %v", err)
		}
		h.redirectResult(w, r, attempt.OrderID, "cancelled")
		return
	}

	// Status == "OK" is only ZarinPal's hint that the browser flow
	// completed; it is never sufficient on its own. The payment is only
	// ever considered successful after this server-to-server verify call
	// succeeds against our own persisted amount for this attempt.
	verifyOut, err := h.client.VerifyPayment(r.Context(), VerifyPaymentInput{
		Authority: authority,
		Amount:    attempt.Amount,
	})
	if err != nil {
		log.Printf("payment: zarinpal verify failed for authority: %v", err)
		if _, ferr := h.repository.FinalizeFailedPayment(r.Context(), authority, nil); ferr != nil {
			log.Printf("payment: failed to finalize failed verification: %v", ferr)
		}
		h.redirectResult(w, r, attempt.OrderID, "failed")
		return
	}

	if verifyOut.Code != 100 && !verifyOut.AlreadyVerified {
		code := verifyOut.Code
		if _, ferr := h.repository.FinalizeFailedPayment(r.Context(), authority, &code); ferr != nil {
			log.Printf("payment: failed to finalize rejected verification: %v", ferr)
		}
		h.redirectResult(w, r, attempt.OrderID, "failed")
		return
	}

	code := verifyOut.Code
	result, err := h.repository.FinalizeVerifiedPayment(r.Context(), authority, verifyOut.RefID, code)
	if err != nil {
		log.Printf("payment: failed to finalize verified payment: %v", err)
		h.redirectResult(w, r, attempt.OrderID, "unknown")
		return
	}

	if result.FinalStatus == StatusPaid {
		h.redirectResult(w, r, result.OrderID, "success")
		return
	}
	h.redirectResult(w, r, result.OrderID, "failed")
}

// AdminListAttempts returns every ZarinPal payment attempt for order {id},
// newest first, for admin inspection (e.g. to show a ref_id on the admin
// order detail page). Admin-only.
func (h *Handler) AdminListAttempts(w http.ResponseWriter, r *http.Request) {
	if _, err := h.auth.RequireAdmin(r); err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			http.Error(w, "forbidden", http.StatusForbidden)
		} else {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		return
	}

	idStr := r.PathValue("id")
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || orderID <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	attempts, err := h.repository.ListAttemptsForOrder(r.Context(), orderID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(attempts)
}

// redirectResult redirects the browser to the frontend's payment result
// page. orderID may be 0 if it could not be determined (e.g. an unknown
// authority); the frontend treats a missing order_id as "unknown" status
// on its own. No internal error text/provider details are ever included
// in the redirect target.
func (h *Handler) redirectResult(w http.ResponseWriter, r *http.Request, orderID int64, outcome string) {
	q := url.Values{}
	if orderID > 0 {
		q.Set("order_id", strconv.FormatInt(orderID, 10))
	}
	q.Set("outcome", outcome)
	http.Redirect(w, r, h.resultBaseURL+"?"+q.Encode(), http.StatusFound)
}
