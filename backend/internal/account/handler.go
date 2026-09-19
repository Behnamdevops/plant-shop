package account

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
)

type Handler struct {
	repository *Repository
	auth       *auth.Handler
}

func NewHandler(repository *Repository, authHandler *auth.Handler) *Handler {
	return &Handler{repository: repository, auth: authHandler}
}

// GetProfile handles GET /api/v1/account/profile
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	profile, err := h.repository.GetProfile(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			http.Error(w, "profile not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}

// UpdateProfile handles PUT /api/v1/account/profile
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.Authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
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

	profile, err := h.repository.UpdateProfile(r.Context(), userID, trimmed)
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			http.Error(w, "profile not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profile)
}