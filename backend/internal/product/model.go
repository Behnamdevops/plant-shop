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
}

type UpdateProductInput struct {
	Name        *string `json:"name"`
	Slug        *string `json:"slug"`
	Description *string `json:"description"`
	Price       *int64  `json:"price"`
	Stock       *int    `json:"stock"`
	ImageURL    *string `json:"image_url"`
}

var (
	ErrDuplicateSlug = errors.New("duplicate slug")
	// ErrProductReferenced is returned when a product cannot be deleted
	// because it is still referenced by historical data (e.g. order_items),
	// which must never be silently cascade-deleted.
	ErrProductReferenced = errors.New("product is referenced by existing orders")
)
