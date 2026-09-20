package returnpkg

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/Behnamdevops/plant-shop/backend/internal/order"
)

// authenticator is satisfied by auth.Handler. Using an interface keeps the
// return package decoupled from auth's concrete type while still reusing its
// session-cookie validation logic.
type authenticator interface {
	Authenticate(r *http.Request) (int64, error)
	RequireAdmin(r *http.Request) (int64, error)
}

// orderRetriever abstracts the order repository methods needed by the handler.
type orderRetriever interface {
	GetByIDForUser(ctx context.Context, userID, orderID int64) (order.OrderWithItems, error)
}

type Handler struct {
	repository *Repository
	orderRepo  orderRetriever
	auth       authenticator
}

func NewHandler(repository *Repository, orderRepo orderRetriever, authHandler *auth.Handler) *Handler {
	return &Handler{repository: repository, orderRepo: orderRepo, auth: authHandler}
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

// requestInput is the wire shape of a return request body.
type requestInput struct {
	Reason       string `json:"reason"`
	CustomerNote string `json:"customer_note,omitempty"`
}

func (in requestInput) toModel() RequestInput {
	return RequestInput{
		Reason:       in.Reason,
		CustomerNote: in.CustomerNote,
	}
}

// Create creates a new return request for the authenticated user's order.
// POST /api/v1/orders/{id}/return-request
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := r.PathValue("id")
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || orderID <= 0 {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	// Verify the order belongs to this user
	if _, err := h.orderRepo.GetByIDForUser(r.Context(), userID, orderID); err != nil {
		if errors.Is(err, order.ErrOrderNotFound) {
			http.Error(w, "order not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var body requestInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	input := body.toModel().Trimmed()
	if err := input.Validate(); err != nil {
		var verr *ErrValidation
		if errors.As(err, &verr) {
			http.Error(w, verr.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	rr, err := h.repository.CreateRequest(r.Context(), orderID, userID, input)
	if err != nil {
		var verr *ErrValidation
		switch {
		case errors.Is(err, ErrReturnRequestNotFound):
			http.Error(w, "order not found", http.StatusNotFound)
		case errors.Is(err, ErrReturnAlreadyExists):
			http.Error(w, "return request already exists for this order", http.StatusConflict)
		case errors.Is(err, ErrReturnNotEligible):
			http.Error(w, "order is not eligible for return request", http.StatusConflict)
		case errors.As(err, &verr):
			http.Error(w, verr.Error(), http.StatusBadRequest)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rr)
}

// GetByID returns the return request for the authenticated user's order.
// GET /api/v1/orders/{id}/return-request
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := r.PathValue("id")
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || orderID <= 0 {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	// Verify the order belongs to this user
	if _, err := h.orderRepo.GetByIDForUser(r.Context(), userID, orderID); err != nil {
		if errors.Is(err, order.ErrOrderNotFound) {
			http.Error(w, "order not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	rr, err := h.repository.GetByUserAndOrder(r.Context(), orderID, userID)
	if err != nil {
		if errors.Is(err, ErrReturnRequestNotFound) {
			http.Error(w, "return request not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rr)
}

// AdminList returns all return requests in the system.
// GET /api/v1/admin/returns
func (h *Handler) AdminList(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	requests, err := h.repository.ListAll(r.Context())
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(requests)
}

// AdminGetByID returns a single return request by ID.
// GET /api/v1/admin/returns/{id}
func (h *Handler) AdminGetByID(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	returnRequestID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || returnRequestID <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	rr, err := h.repository.GetByID(r.Context(), returnRequestID)
	if err != nil {
		if errors.Is(err, ErrReturnRequestNotFound) {
			http.Error(w, "return request not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rr)
}

// updateStatusInput is the request body for AdminUpdateStatus.
type updateStatusInput struct {
	Status string `json:"status"`
	Note   string `json:"note,omitempty"`
}

// AdminUpdateStatus transitions a return request to a new status.
// Admin-only.
func (h *Handler) AdminUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	returnRequestID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || returnRequestID <= 0 {
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

	adminID, _ := h.auth.RequireAdmin(r)

	rr, err := h.repository.UpdateStatus(r.Context(), returnRequestID, input.Status, &adminID, &input.Note)
	if err != nil {
		switch {
		case errors.Is(err, ErrReturnRequestNotFound):
			http.Error(w, "return request not found", http.StatusNotFound)
		case errors.Is(err, ErrReturnInvalidStatus):
			http.Error(w, "invalid status", http.StatusBadRequest)
		case errors.Is(err, ErrReturnInvalidTransition):
			http.Error(w, "invalid status transition", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rr)
}

// AdminApprove approves a return request (requested -> approved).
// POST /api/v1/admin/returns/{id}/approve
func (h *Handler) AdminApprove(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	returnRequestID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || returnRequestID <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var input struct {
		Note string `json:"note,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	adminID, _ := h.auth.RequireAdmin(r)

	rr, err := h.repository.UpdateStatus(r.Context(), returnRequestID, StatusApproved, &adminID, &input.Note)
	if err != nil {
		switch {
		case errors.Is(err, ErrReturnRequestNotFound):
			http.Error(w, "return request not found", http.StatusNotFound)
		case errors.Is(err, ErrReturnInvalidTransition):
			http.Error(w, "invalid status transition", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rr)
}

// AdminReject rejects a return request (requested -> rejected).
// POST /api/v1/admin/returns/{id}/reject
func (h *Handler) AdminReject(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	returnRequestID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || returnRequestID <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var input struct {
		Note string `json:"note,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	adminID, _ := h.auth.RequireAdmin(r)

	rr, err := h.repository.UpdateStatus(r.Context(), returnRequestID, StatusRejected, &adminID, &input.Note)
	if err != nil {
		switch {
		case errors.Is(err, ErrReturnRequestNotFound):
			http.Error(w, "return request not found", http.StatusNotFound)
		case errors.Is(err, ErrReturnInvalidTransition):
			http.Error(w, "invalid status transition", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rr)
}

// AdminReceived marks a return request as received (approved -> received).
// POST /api/v1/admin/returns/{id}/received
func (h *Handler) AdminReceived(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	returnRequestID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || returnRequestID <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var input struct {
		Note string `json:"note,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	adminID, _ := h.auth.RequireAdmin(r)

	rr, err := h.repository.UpdateStatus(r.Context(), returnRequestID, StatusReceived, &adminID, &input.Note)
	if err != nil {
		switch {
		case errors.Is(err, ErrReturnRequestNotFound):
			http.Error(w, "return request not found", http.StatusNotFound)
		case errors.Is(err, ErrReturnInvalidTransition):
			http.Error(w, "invalid status transition", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rr)
}
