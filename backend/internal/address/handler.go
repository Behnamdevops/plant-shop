package address

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
)

type Handler struct {
	repository *Repository
	auth       *auth.Handler
}

func NewHandler(repository *Repository, authHandler *auth.Handler) *Handler {
	return &Handler{repository: repository, auth: authHandler}
}

// List handles GET /api/v1/account/addresses
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	addresses, err := h.repository.List(r.Context(), userID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(addresses)
}

// Create handles POST /api/v1/account/addresses
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var input CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	trimmed := input.Trimmed()
	if err := trimmed.Validate(); err != nil {
		if vErr, ok := err.(*ErrValidation); ok {
			http.Error(w, vErr.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "validation error", http.StatusBadRequest)
		return
	}

	addr, err := h.repository.Create(r.Context(), userID, trimmed)
	if err != nil {
		if errors.Is(err, ErrDuplicateDefault) {
			http.Error(w, "cannot have multiple default addresses", http.StatusConflict)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(addr)
}

// Update handles PUT /api/v1/account/addresses/{id}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	addressID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid address ID", http.StatusBadRequest)
		return
	}

	var input UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	trimmed := input.Trimmed()
	if err := trimmed.Validate(); err != nil {
		if vErr, ok := err.(*ErrValidation); ok {
			http.Error(w, vErr.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "validation error", http.StatusBadRequest)
		return
	}

	addr, err := h.repository.Update(r.Context(), userID, addressID, trimmed)
	if err != nil {
		if errors.Is(err, ErrAddressNotFound) {
			http.Error(w, "address not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrDuplicateDefault) {
			http.Error(w, "cannot have multiple default addresses", http.StatusConflict)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(addr)
}

// Delete handles DELETE /api/v1/account/addresses/{id}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	addressID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid address ID", http.StatusBadRequest)
		return
	}

	if err := h.repository.Delete(r.Context(), userID, addressID); err != nil {
		if errors.Is(err, ErrAddressNotFound) {
			http.Error(w, "address not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}