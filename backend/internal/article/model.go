package article

import (
	"errors"
	"time"
)

// Status constants for article publishing.
const (
	StatusDraft     = "draft"
	StatusPublished = "published"
)

type Category struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Article struct {
	ID             int64      `json:"id"`
	Title          string     `json:"title"`
	Slug           string     `json:"slug"`
	Excerpt        string     `json:"excerpt"`
	Content        string     `json:"content"`
	CoverImageURL  *string    `json:"cover_image_url"`
	CategoryID     *int64     `json:"category_id"`
	Category       *Category  `json:"category,omitempty"`
	Status         string     `json:"status"`
	PublishedAt    *time.Time `json:"published_at"`
	SeoTitle       *string    `json:"seo_title"`
	SeoDescription *string    `json:"seo_description"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type CreateArticleInput struct {
	Title          string     `json:"title"`
	Slug           string     `json:"slug"`
	Excerpt        string     `json:"excerpt"`
	Content        string     `json:"content"`
	CoverImageURL  *string    `json:"cover_image_url"`
	CategoryID     *int64     `json:"category_id"`
	Status         string     `json:"status"`
	PublishedAt    *time.Time `json:"-"`
	SeoTitle       *string    `json:"seo_title"`
	SeoDescription *string    `json:"seo_description"`
}

type UpdateArticleInput struct {
	Title          *string    `json:"title"`
	Slug           *string    `json:"slug"`
	Excerpt        *string    `json:"excerpt"`
	Content        *string    `json:"content"`
	CoverImageURL  *string    `json:"cover_image_url"`
	CategoryID     *int64     `json:"category_id"`
	Status         *string    `json:"status"`
	PublishedAt    *time.Time `json:"-"`
	SeoTitle       *string    `json:"seo_title"`
	SeoDescription *string    `json:"seo_description"`
	// ClearCategory, when true, sets category_id to NULL regardless of
	// CategoryID. This distinguishes "leave unset" (nil CategoryID, keep
	// existing) from an explicit client request to uncategorize.
	ClearCategory bool `json:"-"`
}

// ListFilters holds validated public-listing query parameters.
type ListFilters struct {
	Query      string
	CategoryID *int64
	Page       int
	PageSize   int
}

// ListResult is the structured, paginated response for article listing.
type ListResult struct {
	Items      []Article `json:"items"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
	Total      int64     `json:"total"`
	TotalPages int       `json:"total_pages"`
}

const (
	DefaultPageSize = 12
	MaxPageSize     = 100
	MaxTitleLen     = 200
	MaxExcerptLen   = 500
	MaxSeoTitleLen  = 70
	MaxSeoDescLen   = 160
)

var (
	ErrDuplicateSlug    = errors.New("duplicate slug")
	ErrCategoryNotFound = errors.New("category not found")
	ErrInvalidStatus    = errors.New("invalid status")
)
