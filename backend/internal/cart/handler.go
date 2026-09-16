package cart

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
)

// authenticator is satisfied by auth.Handler. Using an interface keeps the
// cart package decoupled from auth's concrete type while still reusing its
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

func (h *Handler) GetCart(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	c, err := h.repository.GetCart(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}

func (h *Handler) AddItem(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var input AddItemInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if input.ProductID <= 0 {
		http.Error(w, "product_id is required", http.StatusBadRequest)
		return
	}
	if input.Quantity <= 0 {
		http.Error(w, "quantity must be greater than 0", http.StatusBadRequest)
		return
	}

	item, err := h.repository.AddItem(r.Context(), userID, input.ProductID, input.Quantity)
	if err != nil {
		switch {
		case errors.Is(err, ErrProductNotFound):
			http.Error(w, "product not found", http.StatusNotFound)
		case errors.Is(err, ErrInsufficientStock):
			http.Error(w, "insufficient stock", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item)
}

func (h *Handler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := r.PathValue("id")
	itemID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || itemID <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var input UpdateItemInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if input.Quantity <= 0 {
		http.Error(w, "quantity must be greater than 0", http.StatusBadRequest)
		return
	}

	item, err := h.repository.UpdateItemQuantity(r.Context(), userID, itemID, input.Quantity)
	if err != nil {
		switch {
		case errors.Is(err, ErrItemNotFound):
			http.Error(w, "cart item not found", http.StatusNotFound)
		case errors.Is(err, ErrProductNotFound):
			http.Error(w, "product not found", http.StatusNotFound)
		case errors.Is(err, ErrInsufficientStock):
			http.Error(w, "insufficient stock", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := r.PathValue("id")
	itemID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || itemID <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	err = h.repository.DeleteItem(r.Context(), userID, itemID)
	if err != nil {
		if errors.Is(err, ErrItemNotFound) {
			http.Error(w, "cart item not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
