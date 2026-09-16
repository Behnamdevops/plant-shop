package cart

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

// AddItem adds quantity of productID to userID's cart. If the product is
// already in the cart, the quantity is incremented instead of duplicated.
// The resulting quantity must not exceed the product's current stock.
func (r *Repository) AddItem(ctx context.Context, userID, productID int64, quantity int) (Item, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Item{}, err
	}
	defer tx.Rollback(ctx)

	var stock int
	err = tx.QueryRow(ctx, `SELECT stock FROM products WHERE id = $1`, productID).Scan(&stock)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Item{}, ErrProductNotFound
		}
		return Item{}, err
	}

	var existingID int64
	var existingQuantity int
	err = tx.QueryRow(ctx, `
		SELECT id, quantity FROM cart_items
		WHERE user_id = $1 AND product_id = $2
		FOR UPDATE
	`, userID, productID).Scan(&existingID, &existingQuantity)
	exists := true
	if errors.Is(err, pgx.ErrNoRows) {
		exists = false
	} else if err != nil {
		return Item{}, err
	}

	newQuantity := quantity
	if exists {
		newQuantity = existingQuantity + quantity
	}
	if newQuantity > stock {
		return Item{}, ErrInsufficientStock
	}

	var item Item
	if exists {
		err = tx.QueryRow(ctx, `
			UPDATE cart_items
			SET quantity = $1, updated_at = NOW()
			WHERE id = $2
			RETURNING id, user_id, product_id, quantity, created_at, updated_at
		`, newQuantity, existingID).Scan(
			&item.ID, &item.UserID, &item.ProductID, &item.Quantity, &item.CreatedAt, &item.UpdatedAt,
		)
	} else {
		err = tx.QueryRow(ctx, `
			INSERT INTO cart_items (user_id, product_id, quantity)
			VALUES ($1, $2, $3)
			RETURNING id, user_id, product_id, quantity, created_at, updated_at
		`, userID, productID, newQuantity).Scan(
			&item.ID, &item.UserID, &item.ProductID, &item.Quantity, &item.CreatedAt, &item.UpdatedAt,
		)
	}
	if err != nil {
		return Item{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Item{}, err
	}
	return item, nil
}

// UpdateItemQuantity sets the quantity of a cart item belonging to userID.
// Returns ErrItemNotFound if the item does not exist or belongs to another
// user, and ErrInsufficientStock if quantity exceeds the product's stock.
func (r *Repository) UpdateItemQuantity(ctx context.Context, userID, itemID int64, quantity int) (Item, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Item{}, err
	}
	defer tx.Rollback(ctx)

	var productID int64
	err = tx.QueryRow(ctx, `
		SELECT product_id FROM cart_items
		WHERE id = $1 AND user_id = $2
		FOR UPDATE
	`, itemID, userID).Scan(&productID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Item{}, ErrItemNotFound
		}
		return Item{}, err
	}

	var stock int
	err = tx.QueryRow(ctx, `SELECT stock FROM products WHERE id = $1`, productID).Scan(&stock)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Item{}, ErrProductNotFound
		}
		return Item{}, err
	}

	if quantity > stock {
		return Item{}, ErrInsufficientStock
	}

	var item Item
	err = tx.QueryRow(ctx, `
		UPDATE cart_items
		SET quantity = $1, updated_at = NOW()
		WHERE id = $2 AND user_id = $3
		RETURNING id, user_id, product_id, quantity, created_at, updated_at
	`, quantity, itemID, userID).Scan(
		&item.ID, &item.UserID, &item.ProductID, &item.Quantity, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return Item{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Item{}, err
	}
	return item, nil
}

// DeleteItem removes a cart item belonging to userID. Returns ErrItemNotFound
// if the item does not exist or belongs to another user.
func (r *Repository) DeleteItem(ctx context.Context, userID, itemID int64) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM cart_items
		WHERE id = $1 AND user_id = $2
	`, itemID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrItemNotFound
	}
	return nil
}

// GetCart returns all cart items for userID enriched with product data.
func (r *Repository) GetCart(ctx context.Context, userID int64) (Cart, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ci.id, ci.product_id, p.name, p.slug, p.price, ci.quantity
		FROM cart_items ci
		JOIN products p ON p.id = ci.product_id
		WHERE ci.user_id = $1
		ORDER BY ci.id
	`, userID)
	if err != nil {
		return Cart{}, err
	}
	defer rows.Close()

	items := make([]ItemView, 0)
	var total int64

	for rows.Next() {
		var v ItemView
		if err := rows.Scan(&v.ID, &v.ProductID, &v.Name, &v.Slug, &v.Price, &v.Quantity); err != nil {
			return Cart{}, err
		}
		v.Subtotal = v.Price * int64(v.Quantity)
		total += v.Subtotal
		items = append(items, v)
	}
	if err := rows.Err(); err != nil {
		return Cart{}, err
	}

	return Cart{Items: items, Total: total}, nil
}
