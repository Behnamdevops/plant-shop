package category

import (
	"errors"
	"time"
)

type Category struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateCategoryInput struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type UpdateCategoryInput struct {
	Name *string `json:"name"`
	Slug *string `json:"slug"`
}

var (
	ErrDuplicateSlug = errors.New("duplicate slug")
	// ErrCategoryReferenced is returned when a category cannot be deleted
	// because it is still assigned to one or more products.
	ErrCategoryReferenced = errors.New("category is referenced by existing products")
)
