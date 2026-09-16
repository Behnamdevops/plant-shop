package product

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/jackc/pgx/v5"
)

// authenticator is satisfied by auth.Handler. Using an interface keeps the
// product package decoupled from auth's concrete type while still reusing
// its session-cookie validation and admin-role enforcement (mirrors the
// pattern used by the cart and order packages).
type authenticator interface {
	Authenticate(r *http.Request) (int64, error)
	RequireAdmin(r *http.Request) (int64, error)
}

type Handler struct {
	repository *Repository
	auth       authenticator
}

func NewHandler(repository *Repository, authHandler *auth.Handler) *Handler {
	return &Handler{
		repository: repository,
		auth:       authHandler,
	}
}

// requireAdmin enforces admin-only access and writes the appropriate error
// response (401 for no/invalid session, 403 for an authenticated non-admin).
// The role is always determined server-side from the authenticated user
// stored in the database, never trusted from the request itself.
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

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	products, err := h.repository.List(r.Context())
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	var input CreateProductInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if input.Name == "" || input.Slug == "" {
		http.Error(w, "name and slug are required", http.StatusBadRequest)
		return
	}
	if input.Price < 0 {
		http.Error(w, "price must be >= 0", http.StatusBadRequest)
		return
	}
	if input.Stock < 0 {
		http.Error(w, "stock must be >= 0", http.StatusBadRequest)
		return
	}

	product, err := h.repository.Create(r.Context(), input)
	if err != nil {
		if errors.Is(err, ErrDuplicateSlug) {
			http.Error(w, "slug must be unique", http.StatusConflict)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(product)
}

func (h *Handler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	p, err := h.repository.GetBySlug(r.Context(), slug)

	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}

	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	p, err := h.repository.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	err = h.repository.Delete(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			http.Error(w, "product not found", http.StatusNotFound)
		case errors.Is(err, ErrProductReferenced):
			http.Error(w, "product cannot be deleted because it is referenced by existing orders", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var input UpdateProductInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if input.Name == nil {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if input.Slug == nil {
		http.Error(w, "slug is required", http.StatusBadRequest)
		return
	}
	if input.Description == nil {
		http.Error(w, "description is required", http.StatusBadRequest)
		return
	}
	if input.Price == nil {
		http.Error(w, "price is required", http.StatusBadRequest)
		return
	}
	if input.Stock == nil {
		http.Error(w, "stock is required", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(*input.Name)
	slug := strings.TrimSpace(*input.Slug)
	if name == "" || slug == "" {
		http.Error(w, "name and slug must be non-empty", http.StatusBadRequest)
		return
	}
	input.Name = &name
	input.Slug = &slug
	if *input.Price < 0 {
		http.Error(w, "price must be >= 0", http.StatusBadRequest)
		return
	}
	if *input.Stock < 0 {
		http.Error(w, "stock must be >= 0", http.StatusBadRequest)
		return
	}

	product, err := h.repository.Update(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrDuplicateSlug) {
			http.Error(w, "slug must be unique", http.StatusConflict)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(product)
}
