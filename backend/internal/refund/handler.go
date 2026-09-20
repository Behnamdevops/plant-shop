package refund

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	returnpkg "github.com/Behnamdevops/plant-shop/backend/internal/return"
)

// authenticator is satisfied by auth.Handler.
type authenticator interface {
	RequireAdmin(r *http.Request) (int64, error)
}

// returnRequestGetter is satisfied by returnpkg.Repository. It is the only
// way the refund handler learns about a return request's state; the refund
// action endpoints use it to enforce that a refund can only be triggered for
// a return request that has actually completed the received/refund_pending
// steps of the return workflow. This prevents a paid order from being
// refunded directly over HTTP while bypassing the return workflow.
type returnRequestGetter interface {
	GetByID(ctx context.Context, returnRequestID int64) (*returnpkg.ReturnRequest, error)
}

type Handler struct {
	repository *Repository
	provider   RefundProvider
	auth       authenticator
	returns    returnRequestGetter
}

// NewHandler constructs a refund Handler. provider handles refund attempts
// for non-manual payment methods (e.g. ZarinPal); manual payment method
// orders are never routed through it — they only ever complete via
// AdminManualRefund's explicit admin confirmation, which does not call any
// provider at all.
func NewHandler(repository *Repository, provider RefundProvider, authHandler *auth.Handler, returns returnRequestGetter) *Handler {
	return &Handler{repository: repository, provider: provider, auth: authHandler, returns: returns}
}

// refundEligibleReturnStatuses lists the return_requests statuses from which
// a refund action may be triggered. A refund must never be reachable from a
// paid order alone; the order's associated return request must have
// progressed through the return workflow to at least "received" (the admin
// has confirmed the item came back) before any money can move.
var refundEligibleReturnStatuses = map[string]bool{
	returnpkg.StatusReceived:      true,
	returnpkg.StatusRefundPending: true,
}

// requireAdmin enforces admin-only access.
func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	_, err := h.auth.RequireAdmin(r)
	if err != nil {
		if errors.Is(err, auth.ErrForbidden) {
			http.Error(w, "forbidden", http.StatusForbidden)
		} else {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}
		return false
	}
	return true
}

// updateStatusInput is the request body for AdminUpdateStatus.
type updateStatusInput struct {
	Status           string          `json:"status"`
	ProviderRefundID *string         `json:"provider_refund_id,omitempty"`
	ProviderResponse json.RawMessage `json:"provider_response,omitempty"`
	LastError        *string         `json:"last_error,omitempty"`
}

// AdminList returns all refunds in the system (for admin review).
// GET /api/v1/admin/refunds
func (h *Handler) AdminList(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	refunds, err := h.repository.ListAll(r.Context())
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(refunds)
}

// AdminGetByID returns a single refund by ID.
// GET /api/v1/admin/refunds/{id}
func (h *Handler) AdminGetByID(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	refundID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || refundID <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	refund, err := h.repository.GetByID(r.Context(), refundID)
	if err != nil {
		if errors.Is(err, ErrRefundNotFound) {
			http.Error(w, "refund not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(refund)
}

// AdminGetByOrder returns the refund for a given order.
// GET /api/v1/admin/orders/{id}/refund
func (h *Handler) AdminGetByOrder(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || orderID <= 0 {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	refund, err := h.repository.GetByOrder(r.Context(), orderID)
	if err != nil {
		if errors.Is(err, ErrRefundNotFound) {
			http.Error(w, "refund not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(refund)
}

// AdminCreate creates a new refund for an order.
// POST /api/v1/admin/orders/{id}/refund
func (h *Handler) AdminCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || orderID <= 0 {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	// Get the order to determine payment method
	var paymentMethod string
	if err := h.repository.db.QueryRow(r.Context(), `
		SELECT payment_method FROM orders WHERE id = $1
	`, orderID).Scan(&paymentMethod); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var input struct {
		ProviderRefundID *string `json:"provider_refund_id,omitempty"`
		LastError        *string `json:"last_error,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	adminID, _ := h.auth.RequireAdmin(r)

	refund, err := h.repository.Create(r.Context(), orderID, nil, 0, paymentMethod, &adminID)
	if err != nil {
		switch {
		case errors.Is(err, ErrRefundNotFound):
			http.Error(w, "order not found", http.StatusNotFound)
		case errors.Is(err, ErrRefundAlreadyRefunded):
			http.Error(w, "order has already been refunded", http.StatusConflict)
		case errors.Is(err, ErrRefundNotEligible):
			http.Error(w, "order is not eligible for refund", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(refund)
}

// AdminUpdateStatus transitions a refund to a new status.
// Admin-only.
func (h *Handler) AdminUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	refundID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || refundID <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var input updateStatusInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Status is always validated server-side
	if !IsValidStatus(input.Status) {
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}

	rr, err := h.repository.UpdateStatus(r.Context(), refundID, input.Status, input.ProviderRefundID, input.ProviderResponse, input.LastError, nil)
	if err != nil {
		switch {
		case errors.Is(err, ErrRefundNotFound):
			http.Error(w, "refund not found", http.StatusNotFound)
		case errors.Is(err, ErrRefundInvalidStatus):
			http.Error(w, "invalid status", http.StatusBadRequest)
		case errors.Is(err, ErrRefundInvalidTransition):
			http.Error(w, "invalid status transition", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rr)
}

// AdminManualRefund processes a manual refund confirmation for manual payment method orders.
// POST /api/v1/admin/returns/{id}/refund-manual
//
// This requires explicit, separate admin confirmation: it is a dedicated
// action distinct from AdminRefund and only ever transitions a refund that
// is already in manual_review or processing to succeeded when payment_method
// is "manual". It cannot be triggered implicitly by any other action.
func (h *Handler) AdminManualRefund(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	returnRequestID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || returnRequestID <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	returnRequest, refund, ok := h.loadRefundableReturn(w, r, returnRequestID)
	if !ok {
		return
	}
	_ = returnRequest

	// Only manual payment method can be manually confirmed
	if refund.PaymentMethod != "manual" {
		http.Error(w, "only manual payment method can be manually confirmed", http.StatusBadRequest)
		return
	}

	// Idempotent: if already succeeded, just return the current state
	// instead of attempting another transition.
	if refund.Status == StatusSucceeded {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(refund)
		return
	}

	// The manual provider always succeeds once explicitly confirmed by an
	// admin; it never runs implicitly. The refund state machine allows
	// pending -> processing -> succeeded and manual_review -> succeeded
	// directly, so claim "processing" first only when starting from
	// pending (mirrors AdminRefund's atomic claim step and gets the same
	// row-lock-based concurrency guarantee).
	if refund.Status == StatusPending {
		if _, err := h.repository.UpdateStatus(r.Context(), refund.ID, StatusProcessing, nil, nil, nil, nil); err != nil {
			switch {
			case errors.Is(err, ErrRefundInvalidTransition):
				http.Error(w, "invalid status transition", http.StatusConflict)
			default:
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
			return
		}
	}

	now := time.Now()
	updated, err := h.repository.UpdateStatus(r.Context(), refund.ID, StatusSucceeded, nil, nil, nil, &now)
	if err != nil {
		switch {
		case errors.Is(err, ErrRefundInvalidTransition):
			http.Error(w, "invalid status transition", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}

// loadRefundableReturn loads the return request and its associated refund,
// enforcing that the return request has actually progressed through the
// return workflow (received or refund_pending) before any refund action may
// proceed. This is the gate that prevents a paid order from being refunded
// directly over HTTP while bypassing the return workflow: "the order is
// paid" is never sufficient on its own.
func (h *Handler) loadRefundableReturn(w http.ResponseWriter, r *http.Request, returnRequestID int64) (*returnpkg.ReturnRequest, *Refund, bool) {
	returnRequest, err := h.returns.GetByID(r.Context(), returnRequestID)
	if err != nil {
		if errors.Is(err, returnpkg.ErrReturnRequestNotFound) {
			http.Error(w, "return request not found", http.StatusNotFound)
			return nil, nil, false
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return nil, nil, false
	}

	if !refundEligibleReturnStatuses[returnRequest.Status] {
		http.Error(w, "return request must be received before a refund can be processed", http.StatusConflict)
		return nil, nil, false
	}

	refund, err := h.repository.GetByOrder(r.Context(), returnRequest.OrderID)
	if err != nil {
		if errors.Is(err, ErrRefundNotFound) {
			http.Error(w, "refund not found", http.StatusNotFound)
			return nil, nil, false
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return nil, nil, false
	}

	return returnRequest, refund, true
}

// AdminRefund processes a refund through the provider for a return request
// that has completed the return workflow.
// POST /api/v1/admin/returns/{id}/refund
func (h *Handler) AdminRefund(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	returnRequestID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || returnRequestID <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	returnRequest, refund, ok := h.loadRefundableReturn(w, r, returnRequestID)
	if !ok {
		return
	}

	// Manual payment method refunds never call an external provider; they
	// require the admin to use the dedicated manual confirmation endpoint
	// instead, which makes the "money moved" action an explicit, separate
	// admin decision rather than something a generic action can trigger.
	if refund.PaymentMethod == "manual" {
		http.Error(w, "manual payment refunds require explicit confirmation via refund-manual", http.StatusBadRequest)
		return
	}

	// Idempotent: already succeeded, don't attempt to refund again.
	if refund.Status == StatusSucceeded {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(refund)
		return
	}

	// Claim the "processing" transition. This is the atomic, lock-guarded
	// step: only one concurrent request can win it, so only one caller
	// ever proceeds to call the provider below.
	_, err = h.repository.UpdateStatus(r.Context(), refund.ID, StatusProcessing, nil, nil, nil, nil)
	if err != nil {
		switch {
		case errors.Is(err, ErrRefundInvalidTransition):
			// Someone else already claimed processing, or this refund
			// already reached a terminal/manual_review state. Return the
			// current state rather than a generic error so the caller
			// can see what happened without retrying blindly.
			current, getErr := h.repository.GetByID(r.Context(), refund.ID)
			if getErr == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(current)
				return
			}
			http.Error(w, "invalid status transition", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Get the order total (authoritative source for refund amount; never
	// trusts any client-provided amount).
	var orderTotal int64
	if err := h.repository.db.QueryRow(r.Context(), `
		SELECT total FROM orders WHERE id = $1
	`, returnRequest.OrderID).Scan(&orderTotal); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Process refund through provider outside of any DB transaction/lock.
	result, err := h.provider.Refund(r.Context(), RefundInput{
		OrderID:          returnRequest.OrderID,
		Amount:           orderTotal,
		PaymentMethod:    refund.PaymentMethod,
		ProviderRefundID: refund.ProviderRefundID,
	})

	providerResponse, _ := json.Marshal(result)

	if err != nil || (!result.Success && result.ErrorMessage != "") {
		// Ambiguous/erroring provider outcomes are never treated as
		// success and never retried automatically. Non-manual methods
		// (e.g. ZarinPal, which has no verified refund contract) always
		// go to manual_review so a human confirms the outcome; the order
		// stays paid until that confirmation happens.
		lastErr := result.ErrorMessage
		if lastErr == "" && err != nil {
			lastErr = err.Error()
		}
		_, _ = h.repository.UpdateStatus(r.Context(), refund.ID, StatusManualReview, nil, providerResponse, &lastErr, nil)
		http.Error(w, "refund requires manual review", http.StatusConflict)
		return
	}

	if !result.Success {
		_, _ = h.repository.UpdateStatus(r.Context(), refund.ID, StatusManualReview, nil, providerResponse, nil, nil)
		http.Error(w, "refund requires manual review", http.StatusConflict)
		return
	}

	now := time.Now()
	updated, err := h.repository.UpdateStatus(r.Context(), refund.ID, StatusSucceeded, result.ProviderRefundID, providerResponse, nil, &now)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updated)
}
