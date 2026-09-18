package product

import (
	"encoding/json"
	"errors"
	"io"
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

// parseListFilters validates and normalizes public-listing query
// parameters. Invalid sort/pagination/price values return a descriptive
// error so the handler can respond 400 instead of silently falling back
// (falling back only applies to genuinely optional params like q).
func parseListFilters(r *http.Request) (ListFilters, error) {
	q := r.URL.Query()
	filters := ListFilters{
		Query:    strings.TrimSpace(q.Get("q")),
		InStock:  q.Get("in_stock") == "true" || q.Get("in_stock") == "1",
		Sort:     DefaultSort,
		Page:     1,
		PageSize: DefaultPageSize,
	}

	if sort := q.Get("sort"); sort != "" {
		if !IsValidSort(sort) {
			return ListFilters{}, errors.New("invalid sort value")
		}
		filters.Sort = sort
	}

	if categoryStr := q.Get("category"); categoryStr != "" {
		categoryID, err := strconv.ParseInt(categoryStr, 10, 64)
		if err != nil || categoryID <= 0 {
			return ListFilters{}, errors.New("invalid category value")
		}
		filters.CategoryID = &categoryID
	}

	if minStr := q.Get("min_price"); minStr != "" {
		minPrice, err := strconv.ParseInt(minStr, 10, 64)
		if err != nil || minPrice < 0 {
			return ListFilters{}, errors.New("invalid min_price value")
		}
		filters.MinPrice = &minPrice
	}

	if maxStr := q.Get("max_price"); maxStr != "" {
		maxPrice, err := strconv.ParseInt(maxStr, 10, 64)
		if err != nil || maxPrice < 0 {
			return ListFilters{}, errors.New("invalid max_price value")
		}
		filters.MaxPrice = &maxPrice
	}

	if filters.MinPrice != nil && filters.MaxPrice != nil && *filters.MinPrice > *filters.MaxPrice {
		return ListFilters{}, errors.New("min_price must be <= max_price")
	}

	if pageStr := q.Get("page"); pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			return ListFilters{}, errors.New("invalid page value")
		}
		filters.Page = page
	}

	if pageSizeStr := q.Get("page_size"); pageSizeStr != "" {
		pageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || pageSize < 1 {
			return ListFilters{}, errors.New("invalid page_size value")
		}
		if pageSize > MaxPageSize {
			pageSize = MaxPageSize
		}
		filters.PageSize = pageSize
	}

	return filters, nil
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	filters, err := parseListFilters(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.repository.List(r.Context(), filters)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
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
		if errors.Is(err, ErrCategoryNotFound) {
			http.Error(w, "category not found", http.StatusBadRequest)
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

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var input UpdateProductInput
	if err := json.Unmarshal(body, &input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// json.Unmarshal cannot distinguish "category_id omitted" from
	// "category_id explicitly null" when the field is a *int64 — both
	// leave input.CategoryID as nil. Inspect the raw payload to tell them
	// apart: only an explicit `"category_id": null` clears the category.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err == nil {
		if rawVal, present := raw["category_id"]; present && input.CategoryID == nil {
			if strings.TrimSpace(string(rawVal)) == "null" {
				input.ClearCategory = true
			}
		}
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
		if errors.Is(err, ErrCategoryNotFound) {
			http.Error(w, "category not found", http.StatusBadRequest)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(product)
}
