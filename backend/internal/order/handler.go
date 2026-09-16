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
}

type Handler struct {
	repository *Repository
	auth       authenticator
}

func NewHandler(repository *Repository, authHandler *auth.Handler) *Handler {
	return &Handler{repository: repository, auth: authHandler}
}

// Create builds an order from the authenticated user's current cart.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	o, err := h.repository.CreateFromCart(r.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmptyCart):
			http.Error(w, "cart is empty", http.StatusBadRequest)
		case errors.Is(err, ErrInsufficientStock):
			http.Error(w, "insufficient stock", http.StatusConflict)
		case errors.Is(err, ErrProductNotFound):
			http.Error(w, "product not found", http.StatusNotFound)
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
