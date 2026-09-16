package order

import (
	"errors"
	"time"
)

// Order represents a row in the orders table.
type Order struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"-"`
	Status    string    `json:"status"`
	Total     int64     `json:"total"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Item is an immutable snapshot of a product at the time an order was
// placed. It is stored separately from products so that later changes to a
// product's name, slug, or price never affect historical orders.
type Item struct {
	ID          int64  `json:"id"`
	OrderID     int64  `json:"-"`
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
	ProductSlug string `json:"product_slug"`
	UnitPrice   int64  `json:"unit_price"`
	Quantity    int    `json:"quantity"`
	Subtotal    int64  `json:"subtotal"`
}

// OrderWithItems is the full response payload for GET /orders/{id}.
type OrderWithItems struct {
	Order
	Items []Item `json:"items"`
}

const StatusPending = "pending"

var (
	ErrEmptyCart         = errors.New("cart is empty")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrProductNotFound   = errors.New("product not found")
	ErrOrderNotFound     = errors.New("order not found")
)
