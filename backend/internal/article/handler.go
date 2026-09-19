package article

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// authenticator mirrors the interface used by other packages.
type authenticator interface {
	Authenticate(r *http.Request) (int64, error)
	RequireAdmin(r *http.Request) (int64, error)
}

// imageCleaner is the minimal capability the article handler needs from
// the storage package for cleaning up old images.
type imageCleaner interface {
	Delete(ctx context.Context, key string) error
	KeyFromURL(url string) (key string, ok bool)
}

type Handler struct {
	repository *Repository
	auth       authenticator
	images     imageCleaner
	db         *pgxpool.Pool
}

func NewHandler(repository *Repository, authHandler *auth.Handler, images imageCleaner, db *pgxpool.Pool) *Handler {
	return &Handler{
		repository: repository,
		auth:       authHandler,
		images:     images,
		db:         db,
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

// cleanupImageIfUnused best-effort deletes the local image previously at
// oldURL, but only if it is actually a locally-managed upload.
func (h *Handler) cleanupImageIfUnused(ctx context.Context, oldURL, newURL *string) {
	if h.images == nil || oldURL == nil || *oldURL == "" {
		return
	}
	if newURL != nil && *newURL == *oldURL {
		return
	}
	key, ok := h.images.KeyFromURL(*oldURL)
	if !ok {
		return
	}
	if err := h.images.Delete(ctx, key); err != nil {
		slog.Warn("failed to clean up old article image", "error", err.Error())
	}
}

// parseListFilters validates and normalizes public-listing query parameters.
func parseListFilters(r *http.Request) (ListFilters, error) {
	q := r.URL.Query()
	filters := ListFilters{
		Query:    strings.TrimSpace(q.Get("q")),
		Page:     1,
		PageSize: DefaultPageSize,
	}

	if categoryStr := q.Get("category"); categoryStr != "" {
		categoryID, err := strconv.ParseInt(categoryStr, 10, 64)
		if err != nil || categoryID <= 0 {
			return ListFilters{}, errors.New("invalid category value")
		}
		filters.CategoryID = &categoryID
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

// List is the public endpoint: GET /api/v1/articles
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

	// Hide category details from public response for now (optional optimization)
	// Could add a separate response type if needed

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GetBySlug is the public endpoint: GET /api/v1/articles/{slug}
func (h *Handler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	a, err := h.repository.GetBySlug(r.Context(), slug)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "article not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a)
}

// AdminList is the admin endpoint: GET /api/v1/admin/articles
func (h *Handler) AdminList(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	q := r.URL.Query()
	page := 1
	pageSize := 20

	if pageStr := q.Get("page"); pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil || p < 1 {
			http.Error(w, "invalid page value", http.StatusBadRequest)
			return
		}
		page = p
	}

	if pageSizeStr := q.Get("page_size"); pageSizeStr != "" {
		ps, err := strconv.Atoi(pageSizeStr)
		if err != nil || ps < 1 {
			http.Error(w, "invalid page_size value", http.StatusBadRequest)
			return
		}
		if ps > MaxPageSize {
			ps = MaxPageSize
		}
		pageSize = ps
	}

	result, err := h.repository.AdminList(r.Context(), page, pageSize)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// AdminGetByID is the admin endpoint: GET /api/v1/admin/articles/{id}
func (h *Handler) AdminGetByID(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	a, err := h.repository.AdminGetByID(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "article not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a)
}

// Create handles POST /api/v1/admin/articles
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	var input CreateArticleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validation
	if strings.TrimSpace(input.Title) == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(input.Slug) == "" {
		http.Error(w, "slug is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(input.Content) == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}

	// Normalize
	input.Title = strings.TrimSpace(input.Title)
	input.Slug = normalizeSlug(input.Slug)
	input.Excerpt = strings.TrimSpace(input.Excerpt)

	// Validate status
	if input.Status == "" {
		input.Status = StatusDraft
	}
	if input.Status != StatusDraft && input.Status != StatusPublished {
		http.Error(w, "status must be 'draft' or 'published'", http.StatusBadRequest)
		return
	}

	// Validate lengths
	if len(input.Title) > MaxTitleLen {
		http.Error(w, "title exceeds maximum length", http.StatusBadRequest)
		return
	}
	if len(input.Excerpt) > MaxExcerptLen {
		http.Error(w, "excerpt exceeds maximum length", http.StatusBadRequest)
		return
	}

	// Set published_at if publishing
	var publishedAt *time.Time
	if input.Status == StatusPublished {
		now := time.Now()
		publishedAt = &now
	}

	// Validate SEO fields
	if input.SeoTitle != nil && len(*input.SeoTitle) > MaxSeoTitleLen {
		http.Error(w, "seo_title exceeds maximum length", http.StatusBadRequest)
		return
	}
	if input.SeoDescription != nil && len(*input.SeoDescription) > MaxSeoDescLen {
		http.Error(w, "seo_description exceeds maximum length", http.StatusBadRequest)
		return
	}

	// Validate category exists if provided
	if input.CategoryID != nil {
		exists, err := h.categoryExists(r.Context(), *input.CategoryID)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if !exists {
			http.Error(w, "category not found", http.StatusBadRequest)
			return
		}
	}

	article, err := h.repository.Create(r.Context(), CreateArticleInput{
		Title:          input.Title,
		Slug:           input.Slug,
		Excerpt:        input.Excerpt,
		Content:        input.Content,
		CoverImageURL:  input.CoverImageURL,
		CategoryID:     input.CategoryID,
		Status:         input.Status,
		PublishedAt:    publishedAt,
		SeoTitle:       input.SeoTitle,
		SeoDescription: input.SeoDescription,
	})
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
	json.NewEncoder(w).Encode(article)
}

// Update handles PUT /api/v1/admin/articles/{id}
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

	var input UpdateArticleInput
	if err := json.Unmarshal(body, &input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Parse clear category flag
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err == nil {
		if rawVal, present := raw["category_id"]; present && input.CategoryID == nil {
			if strings.TrimSpace(string(rawVal)) == "null" {
				input.ClearCategory = true
			}
		}
	}

	// Validation
	if input.Title == nil || strings.TrimSpace(*input.Title) == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if input.Slug == nil || strings.TrimSpace(*input.Slug) == "" {
		http.Error(w, "slug is required", http.StatusBadRequest)
		return
	}
	if input.Content == nil || strings.TrimSpace(*input.Content) == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}

	// Normalize
	title := strings.TrimSpace(*input.Title)
	slug := normalizeSlug(*input.Slug)
	var excerpt string
	if input.Excerpt != nil {
		excerpt = strings.TrimSpace(*input.Excerpt)
	}
	input.Title = &title
	input.Slug = &slug
	input.Excerpt = &excerpt

	// Validate status
	if input.Status != nil {
		if *input.Status != StatusDraft && *input.Status != StatusPublished {
			http.Error(w, "status must be 'draft' or 'published'", http.StatusBadRequest)
			return
		}
	}

	// Validate lengths
	if len(*input.Title) > MaxTitleLen {
		http.Error(w, "title exceeds maximum length", http.StatusBadRequest)
		return
	}
	if input.Excerpt != nil && len(*input.Excerpt) > MaxExcerptLen {
		http.Error(w, "excerpt exceeds maximum length", http.StatusBadRequest)
		return
	}

	// Validate SEO fields
	if input.SeoTitle != nil && len(*input.SeoTitle) > MaxSeoTitleLen {
		http.Error(w, "seo_title exceeds maximum length", http.StatusBadRequest)
		return
	}
	if input.SeoDescription != nil && len(*input.SeoDescription) > MaxSeoDescLen {
		http.Error(w, "seo_description exceeds maximum length", http.StatusBadRequest)
		return
	}

	// Get existing article to determine published_at handling
	existing, getErr := h.repository.AdminGetByID(r.Context(), id)
	if getErr != nil {
		if errors.Is(getErr, pgx.ErrNoRows) {
			http.Error(w, "article not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Determine published_at
	var publishedAt *time.Time
	if input.Status != nil {
		// If publishing a draft for the first time, set published_at
		if existing.Status == StatusDraft && *input.Status == StatusPublished && existing.PublishedAt == nil {
			now := time.Now()
			publishedAt = &now
		} else if existing.PublishedAt != nil {
			// Keep existing published_at
			publishedAt = existing.PublishedAt
		}
	} else {
		// No status change, keep existing
		publishedAt = existing.PublishedAt
	}

	// Validate category exists if provided
	if input.CategoryID != nil && !input.ClearCategory {
		exists, err := h.categoryExists(r.Context(), *input.CategoryID)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if !exists {
			http.Error(w, "category not found", http.StatusBadRequest)
			return
		}
	}

	// Read the current cover_image_url before updating
	previousImageURL := existing.CoverImageURL

	article, err := h.repository.Update(r.Context(), id, UpdateArticleInput{
		Title:          input.Title,
		Slug:           input.Slug,
		Excerpt:        input.Excerpt,
		Content:        input.Content,
		CoverImageURL:  input.CoverImageURL,
		CategoryID:     input.CategoryID,
		Status:         input.Status,
		PublishedAt:    publishedAt,
		SeoTitle:       input.SeoTitle,
		SeoDescription: input.SeoDescription,
		ClearCategory:  input.ClearCategory,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "article not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrDuplicateSlug) {
			http.Error(w, "slug must be unique", http.StatusConflict)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.cleanupImageIfUnused(r.Context(), previousImageURL, article.CoverImageURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(article)
}

// Delete handles DELETE /api/v1/admin/articles/{id}
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

	// Fetch the article first for image cleanup
	existing, getErr := h.repository.AdminGetByID(r.Context(), id)

	err = h.repository.Delete(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "article not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if getErr == nil {
		h.cleanupImageIfUnused(r.Context(), existing.CoverImageURL, nil)
	}

	w.WriteHeader(http.StatusNoContent)
}

// normalizeSlug converts a slug to lowercase, replaces spaces with hyphens,
// and removes non-alphanumeric characters (except hyphens).
func normalizeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	var result strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			result.WriteRune(c)
		}
	}
	return result.String()
}

// categoryExists checks if a category with the given ID exists.
func (h *Handler) categoryExists(ctx context.Context, id int64) (bool, error) {
	var exists bool
	err := h.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM article_categories WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}
