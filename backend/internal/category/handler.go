package category

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/jackc/pgx/v5"
)

// authenticator mirrors the interface used by the product package so this
// package stays decoupled from auth's concrete type while reusing its
// session-cookie validation and admin-role enforcement.
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

// List is the public endpoint: GET /api/v1/categories.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	categories, err := h.repository.List(r.Context())
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(categories)
}

// AdminList is the admin endpoint: GET /api/v1/admin/categories.
func (h *Handler) AdminList(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	h.List(w, r)
}

func validateNameSlug(name, slug string) (string, string, error) {
	trimmedName := strings.TrimSpace(name)
	trimmedSlug := strings.TrimSpace(slug)
	if trimmedName == "" || trimmedSlug == "" {
		return "", "", errors.New("name and slug are required")
	}
	return trimmedName, trimmedSlug, nil
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	var input CreateCategoryInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	name, slug, err := validateNameSlug(input.Name, input.Slug)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	input.Name = name
	input.Slug = slug

	c, err := h.repository.Create(r.Context(), input)
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
	json.NewEncoder(w).Encode(c)
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

	var input UpdateCategoryInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if input.Name == nil || input.Slug == nil {
		http.Error(w, "name and slug are required", http.StatusBadRequest)
		return
	}

	name, slug, err := validateNameSlug(*input.Name, *input.Slug)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	input.Name = &name
	input.Slug = &slug

	c, err := h.repository.Update(r.Context(), id, input)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "category not found", http.StatusNotFound)
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
	json.NewEncoder(w).Encode(c)
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
			http.Error(w, "category not found", http.StatusNotFound)
		case errors.Is(err, ErrCategoryReferenced):
			http.Error(w, "category cannot be deleted because it is assigned to existing products", http.StatusConflict)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
