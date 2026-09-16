package cart

import (
	"errors"
	"time"
)

// Item represents a single row in cart_items.
type Item struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"-"`
	ProductID int64     `json:"product_id"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ItemView is a cart item enriched with product data for the GET /cart response.
type ItemView struct {
	ID        int64  `json:"id"`
	ProductID int64  `json:"product_id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Price     int64  `json:"price"`
	Quantity  int    `json:"quantity"`
	Subtotal  int64  `json:"subtotal"`
}

// Cart is the full response payload for GET /cart.
type Cart struct {
	Items []ItemView `json:"items"`
	Total int64      `json:"total"`
}

// AddItemInput is the request body for POST /cart/items.
type AddItemInput struct {
	ProductID int64 `json:"product_id"`
	Quantity  int   `json:"quantity"`
}

// UpdateItemInput is the request body for PUT /cart/items/{id}.
type UpdateItemInput struct {
	Quantity int `json:"quantity"`
}

var (
	ErrProductNotFound   = errors.New("product not found")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrItemNotFound      = errors.New("cart item not found")
)
