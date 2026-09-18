package product

import (
	"errors"
	"time"
)

type Product struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Price       int64     `json:"price"`
	Stock       int       `json:"stock"`
	ImageURL    *string   `json:"image_url"`
	CategoryID  *int64    `json:"category_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateProductInput struct {
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description string  `json:"description"`
	Price       int64   `json:"price"`
	Stock       int     `json:"stock"`
	ImageURL    *string `json:"image_url"`
	CategoryID  *int64  `json:"category_id"`
}

type UpdateProductInput struct {
	Name        *string `json:"name"`
	Slug        *string `json:"slug"`
	Description *string `json:"description"`
	Price       *int64  `json:"price"`
	Stock       *int    `json:"stock"`
	ImageURL    *string `json:"image_url"`
	CategoryID  *int64  `json:"category_id"`
	// ClearCategory, when true, sets category_id to NULL regardless of
	// CategoryID. This distinguishes "leave unset" (nil CategoryID, keep
	// existing) from an explicit client request to uncategorize a product,
	// since CategoryID being nil is ambiguous between "not provided" and
	// "set to null" once decoded from JSON without this flag.
	ClearCategory bool `json:"-"`
}

// ListFilters holds validated public-listing query parameters. Backend is
// authoritative for sort mode and pagination bounds; sort is restricted to
// the SortModes allow-list to avoid any dynamic SQL injection through
// ORDER BY.
type ListFilters struct {
	Query      string
	CategoryID *int64
	InStock    bool
	MinPrice   *int64
	MaxPrice   *int64
	Sort       string
	Page       int
	PageSize   int
}

// ListResult is the structured, paginated response shape for the public
// product listing.
type ListResult struct {
	Items      []Product `json:"items"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
	Total      int64     `json:"total"`
	TotalPages int       `json:"total_pages"`
}

const (
	SortNewest    = "newest"
	SortPriceAsc  = "price_asc"
	SortPriceDesc = "price_desc"
	SortNameAsc   = "name_asc"

	DefaultSort = SortNewest

	DefaultPageSize = 20
	MaxPageSize     = 100
)

// sortColumns is the explicit allow-list mapping a validated sort mode to
// its ORDER BY clause. Never build ORDER BY from unvalidated client input.
var sortColumns = map[string]string{
	SortNewest:    "id DESC",
	SortPriceAsc:  "price ASC, id DESC",
	SortPriceDesc: "price DESC, id DESC",
	SortNameAsc:   "name ASC, id DESC",
}

func IsValidSort(sort string) bool {
	_, ok := sortColumns[sort]
	return ok
}

var (
	ErrDuplicateSlug = errors.New("duplicate slug")
	// ErrProductReferenced is returned when a product cannot be deleted
	// because it is still referenced by historical data (e.g. order_items),
	// which must never be silently cascade-deleted.
	ErrProductReferenced = errors.New("product is referenced by existing orders")
	// ErrCategoryNotFound is returned when a create/update input references
	// a category_id that does not exist.
	ErrCategoryNotFound = errors.New("category not found")
)
