package order

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
)

// authenticator is satisfied by auth.Handler. Using an interface keeps the
// order package decoupled from auth's concrete type while still reusing its
// session-cookie validation logic (no JWT/Redis, no duplicated auth code).
type authenticator interface {
	Authenticate(r *http.Request) (int64, error)
	RequireAdmin(r *http.Request) (int64, error)
}

type Handler struct {
	repository *Repository
	auth       authenticator
}

func NewHandler(repository *Repository, authHandler *auth.Handler) *Handler {
	return &Handler{repository: repository, auth: authHandler}
}

// requireAdmin enforces admin-only access and writes the appropriate error
// response (401 for no/invalid session, 403 for an authenticated
// non-admin). Mirrors the identical helper in the product package: the
// role is always determined server-side from the authenticated user stored
// in the database, never trusted from the request itself.
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

// checkoutInput is the wire shape of the checkout request body. It mirrors
// CheckoutInput field-for-field; the user id always comes from the
// authenticated session, never from this body.
type checkoutInput struct {
	RecipientName  string `json:"recipient_name"`
	Phone          string `json:"phone"`
	AddressLine1   string `json:"address_line1"`
	AddressLine2   string `json:"address_line2"`
	City           string `json:"city"`
	PostalCode     string `json:"postal_code"`
	Country        string `json:"country"`
	ShippingMethod string `json:"shipping_method"`
}

func (in checkoutInput) toModel() CheckoutInput {
	return CheckoutInput{
		RecipientName:  in.RecipientName,
		Phone:          in.Phone,
		AddressLine1:   in.AddressLine1,
		AddressLine2:   in.AddressLine2,
		City:           in.City,
		PostalCode:     in.PostalCode,
		Country:        in.Country,
		ShippingMethod: in.ShippingMethod,
	}
}

// Create builds an order from the authenticated user's current cart plus
// the delivery/shipping details supplied in the request body. The
// authenticated user id always comes from the session; nothing about the
// user's identity, prices, fees, or totals is ever trusted from the
// request body.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var body checkoutInput
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

	o, err := h.repository.CreateFromCart(r.Context(), userID, input)
	if err != nil {
		var verr *ErrValidation
		switch {
		case errors.Is(err, ErrEmptyCart):
			http.Error(w, "cart is empty", http.StatusBadRequest)
		case errors.Is(err, ErrInsufficientStock):
			http.Error(w, "insufficient stock", http.StatusConflict)
		case errors.Is(err, ErrProductNotFound):
			http.Error(w, "product not found", http.StatusNotFound)
		case errors.As(err, &verr):
			http.Error(w, verr.Error(), http.StatusBadRequest)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(o)
}

// List returns all orders belonging to the authenticated user.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	orders, err := h.repository.ListByUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

// GetByID returns a single order (with items) belonging to the authenticated
// user, or 404 if it doesn't exist or belongs to someone else.
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
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

	o, err := h.repository.GetByIDForUser(r.Context(), userID, orderID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			http.Error(w, "order not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(o)
}

// AdminList returns every order in the system, newest first, for admin
// inspection. Admin-only.
func (h *Handler) AdminList(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	orders, err := h.repository.ListAll(r.Context())
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

// AdminGetByID returns a single order (with items and customer identity)
// regardless of which user it belongs to. Admin-only.
func (h *Handler) AdminGetByID(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || orderID <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	o, err := h.repository.GetByID(r.Context(), orderID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			http.Error(w, "order not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(o)
}

// updateStatusInput is the request body for AdminUpdateStatus.
type updateStatusInput struct {
	Status string `json:"status"`
}

// AdminUpdateStatus transitions an order to a new status, enforcing both
// that the status value is one of the known statuses and that the
// transition is allowed from the order's current status. Admin-only.
func (h *Handler) AdminUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	orderID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || orderID <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var input updateStatusInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Status is always validated server-side; the frontend's own validation
	// is never trusted.
	if !IsValidStatus(input.Status) {
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}

	o, err := h.repository.UpdateStatus(r.Context(), orderID, input.Status)
	if err != nil {
		switch {
		case errors.Is(err, ErrOrderNotFound):
			http.Error(w, "order not found", http.StatusNotFound)
		case errors.Is(err, ErrInvalidTransition):
			http.Error(w, "invalid status transition", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(o)
}
