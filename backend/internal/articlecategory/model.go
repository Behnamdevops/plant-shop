package articlecategory

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
)
