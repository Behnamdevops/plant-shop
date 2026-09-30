package product

import (
	"errors"
	"time"
)

// Details describes a care product; legacy plant care stays archived in SQL.
type Details struct {
	Kind         string  `json:"kind,omitempty"`
	Brand        string  `json:"brand,omitempty"`
	WeightVolume string  `json:"weight_volume,omitempty"`
	Formulation  string  `json:"formulation,omitempty"`
	SuitableFor  string  `json:"suitable_for,omitempty"`
	Composition  string  `json:"composition,omitempty"`
	Usage        string  `json:"usage,omitempty"`
	Benefits     string  `json:"benefits,omitempty"`
	Warnings     string  `json:"warnings,omitempty"`
	PackCount    int     `json:"pack_count,omitempty"`
	Country      string  `json:"country,omitempty"`
	ExpiryDate   string  `json:"expiry_date,omitempty"`
	Included     string  `json:"included,omitempty"`
	ArticleSlug  string  `json:"article_slug,omitempty"`
	RelatedIDs   []int64 `json:"related_ids,omitempty"`
	DeliveryInfo string  `json:"delivery_info,omitempty"`
}
type Product struct {
	Details      Details   `json:"details"`
	SoldQuantity int64     `json:"sold_quantity"`
	ImageURLs    []string  `json:"image_urls"`
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Description  string    `json:"description"`
	Price        int64     `json:"price"`
	Stock        int       `json:"stock"`
	ImageURL     *string   `json:"image_url"`
	CategoryID   *int64    `json:"category_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreateProductInput struct {
	Details     Details  `json:"details"`
	ImageURLs   []string `json:"image_urls"`
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Description string   `json:"description"`
	Price       int64    `json:"price"`
	Stock       int      `json:"stock"`
	ImageURL    *string  `json:"image_url"`
	CategoryID  *int64   `json:"category_id"`
}

type UpdateProductInput struct {
	Details     *Details  `json:"details"`
	ImageURLs   *[]string `json:"image_urls"`
	AdminUserID *int64    `json:"-"`
	StockReason string    `json:"stock_reason"`
	Name        *string   `json:"name"`
	Slug        *string   `json:"slug"`
	Description *string   `json:"description"`
	Price       *int64    `json:"price"`
	Stock       *int      `json:"stock"`
	ImageURL    *string   `json:"image_url"`
	CategoryID  *int64    `json:"category_id"`
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
	Kind        string
	Brand       string
	Formulation string
	Guide       string
	Query       string
	CategoryID  *int64
	InStock     bool
	MinPrice    *int64
	MaxPrice    *int64
	Sort        string
	Page        int
	PageSize    int
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
	"bestselling": "COALESCE((SELECT SUM(oi.quantity) FROM order_items oi JOIN orders o ON o.id=oi.order_id WHERE oi.product_id=products.id AND o.payment_status='paid' AND o.status!='cancelled'),0) DESC, id DESC",
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
