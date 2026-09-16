package order

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// cartLine is a cart item joined with its current product data, used only
// during checkout.
type cartLine struct {
	ProductID   int64
	ProductName string
	ProductSlug string
	Price       int64
	Stock       int
	Quantity    int
}

// CreateFromCart builds an order from userID's current cart in a single
// transaction: it locks the relevant product rows, validates stock,
// decrements stock, snapshots each cart line into an order_items row,
// and clears the cart. Nothing is persisted unless every step succeeds.
func (r *Repository) CreateFromCart(ctx context.Context, userID int64) (Order, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback(ctx)

	// Lock the product rows referenced by the cart so concurrent checkouts
	// or stock updates can't race with this one.
	rows, err := tx.Query(ctx, `
		SELECT ci.product_id, p.name, p.slug, p.price, p.stock, ci.quantity
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		WHERE ci.user_id = $1
		ORDER BY ci.product_id
		FOR UPDATE OF p
	`, userID)
	if err != nil {
		return Order{}, err
	}
	lines := make([]cartLine, 0)
	for rows.Next() {
		var l cartLine
		if err := rows.Scan(&l.ProductID, &l.ProductName, &l.ProductSlug, &l.Price, &l.Stock, &l.Quantity); err != nil {
			rows.Close()
			return Order{}, err
		}
		lines = append(lines, l)
	}
	if err := rows.Err(); err != nil {
		return Order{}, err
	}
	rows.Close()

	if len(lines) == 0 {
		return Order{}, ErrEmptyCart
	}

	for _, l := range lines {
		if l.Quantity > l.Stock {
			return Order{}, ErrInsufficientStock
		}
	}

	var total int64
	for _, l := range lines {
		total += l.Price * int64(l.Quantity)
	}

	var o Order
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (user_id, status, total)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, status, total, created_at, updated_at
	`, userID, StatusPending, total).Scan(
		&o.ID, &o.UserID, &o.Status, &o.Total, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return Order{}, err
	}

	for _, l := range lines {
		subtotal := l.Price * int64(l.Quantity)
		_, err = tx.Exec(ctx, `
			INSERT INTO order_items (order_id, product_id, product_name, product_slug, unit_price, quantity, subtotal)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, o.ID, l.ProductID, l.ProductName, l.ProductSlug, l.Price, l.Quantity, subtotal)
		if err != nil {
			return Order{}, err
		}

		_, err = tx.Exec(ctx, `
			UPDATE products SET stock = stock - $1, updated_at = NOW() WHERE id = $2
		`, l.Quantity, l.ProductID)
		if err != nil {
			return Order{}, err
		}
	}

	_, err = tx.Exec(ctx, `DELETE FROM cart_items WHERE user_id = $1`, userID)
	if err != nil {
		return Order{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Order{}, err
	}
	return o, nil
}

// ListByUser returns all orders belonging to userID, most recent first.
func (r *Repository) ListByUser(ctx context.Context, userID int64) ([]Order, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, status, total, created_at, updated_at
		FROM orders
		WHERE user_id = $1
		ORDER BY id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]Order, 0)
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.Status, &o.Total, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// GetByIDForUser returns the order (with its items) identified by orderID,
// but only if it belongs to userID. Returns ErrOrderNotFound otherwise.
func (r *Repository) GetByIDForUser(ctx context.Context, userID, orderID int64) (OrderWithItems, error) {
	var o Order
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, status, total, created_at, updated_at
		FROM orders
		WHERE id = $1 AND user_id = $2
	`, orderID, userID).Scan(
		&o.ID, &o.UserID, &o.Status, &o.Total, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OrderWithItems{}, ErrOrderNotFound
		}
		return OrderWithItems{}, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, order_id, product_id, product_name, product_slug, unit_price, quantity, subtotal
		FROM order_items
		WHERE order_id = $1
		ORDER BY id
	`, o.ID)
	if err != nil {
		return OrderWithItems{}, err
	}
	defer rows.Close()

	items := make([]Item, 0)
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.OrderID, &it.ProductID, &it.ProductName, &it.ProductSlug, &it.UnitPrice, &it.Quantity, &it.Subtotal); err != nil {
			return OrderWithItems{}, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return OrderWithItems{}, err
	}

	return OrderWithItems{Order: o, Items: items}, nil
}
