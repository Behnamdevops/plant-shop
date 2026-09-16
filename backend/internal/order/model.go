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

// Customer is a safe, minimal identity snapshot of the user who placed an
// order. It intentionally excludes password_hash and any session/auth data
// and is only ever populated for admin-facing responses.
type Customer struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// AdminOrder is the response shape for GET /admin/orders: an order plus the
// safe identity of the customer who placed it. Kept separate from Order so
// the customer-facing API surface never changes shape.
type AdminOrder struct {
	Order
	Customer Customer `json:"customer"`
}

// AdminOrderWithItems is the response shape for GET /admin/orders/{id}.
type AdminOrderWithItems struct {
	Order
	Customer Customer `json:"customer"`
	Items    []Item   `json:"items"`
}

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusShipped    = "shipped"
	StatusDelivered  = "delivered"
	StatusCancelled  = "cancelled"
)

// validStatuses is used to validate status values supplied by admins before
// they ever reach the database.
var validStatuses = map[string]bool{
	StatusPending:    true,
	StatusProcessing: true,
	StatusShipped:    true,
	StatusDelivered:  true,
	StatusCancelled:  true,
}

// IsValidStatus reports whether status is one of the known order statuses.
func IsValidStatus(status string) bool {
	return validStatuses[status]
}

// allowedTransitions encodes the order status state machine: for each
// status, the set of statuses it may move to. Statuses not present as keys
// (delivered, cancelled) are terminal and permit no further transitions.
var allowedTransitions = map[string]map[string]bool{
	StatusPending:    {StatusProcessing: true, StatusCancelled: true},
	StatusProcessing: {StatusShipped: true, StatusCancelled: true},
	StatusShipped:    {StatusDelivered: true},
}

// CanTransition reports whether an order may move from `from` to `to`.
// Transitioning to the same status is never allowed (callers should treat
// it the same as any other invalid transition), and unknown/terminal
// statuses simply have no allowed transitions.
func CanTransition(from, to string) bool {
	next, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	return next[to]
}

var (
	ErrEmptyCart         = errors.New("cart is empty")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrProductNotFound   = errors.New("product not found")
	ErrOrderNotFound     = errors.New("order not found")
	ErrInvalidTransition = errors.New("invalid status transition")
)
