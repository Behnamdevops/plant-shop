package product

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context) ([]Product, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			id,
			name,
			slug,
			description,
			price,
			stock,
			image_url,
			created_at,
			updated_at
		FROM products
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := make([]Product, 0)

	for rows.Next() {
		var p Product

		err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Slug,
			&p.Description,
			&p.Price,
			&p.Stock,
			&p.ImageURL,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		products = append(products, p)
	}

	return products, rows.Err()
}

func (r *Repository) GetBySlug(ctx context.Context, slug string) (Product, error) {
	var p Product

	err := r.db.QueryRow(ctx, `
		SELECT
			id,
			name,
			slug,
			description,
			price,
			stock,
			image_url,
			created_at,
			updated_at
		FROM products
		WHERE slug = $1
	`, slug).Scan(
		&p.ID,
		&p.Name,
		&p.Slug,
		&p.Description,
		&p.Price,
		&p.Stock,
		&p.ImageURL,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	return p, err
}

func (r *Repository) GetByID(ctx context.Context, id int64) (Product, error) {
	var p Product

	err := r.db.QueryRow(ctx, `
		SELECT
			id,
			name,
			slug,
			description,
			price,
			stock,
			image_url,
			created_at,
			updated_at
		FROM products
		WHERE id = $1
	`, id).Scan(
		&p.ID,
		&p.Name,
		&p.Slug,
		&p.Description,
		&p.Price,
		&p.Stock,
		&p.ImageURL,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	return p, err
}

func (r *Repository) Create(ctx context.Context, input CreateProductInput) (Product, error) {
	var p Product
	err := r.db.QueryRow(ctx, `
		INSERT INTO products (name, slug, description, price, stock, image_url)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, slug, description, price, stock, image_url, created_at, updated_at
	`, input.Name, input.Slug, input.Description, input.Price, input.Stock, input.ImageURL).Scan(
		&p.ID,
		&p.Name,
		&p.Slug,
		&p.Description,
		&p.Price,
		&p.Stock,
		&p.ImageURL,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Product{}, ErrDuplicateSlug
		}
		return Product{}, err
	}
	return p, nil
}

// Delete removes a product by ID. cart_items reference products with
// ON DELETE CASCADE, so any in-progress carts referencing this product are
// cleaned up automatically. order_items has no ON DELETE clause (defaults to
// RESTRICT), so if this product was ever part of a placed order, Postgres
// rejects the delete with a foreign-key violation (23503) — this is mapped
// to ErrProductReferenced so historical order data is never lost.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM products WHERE id = $1`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrProductReferenced
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, id int64, input UpdateProductInput) (Product, error) {
	var p Product
	err := r.db.QueryRow(ctx, `
		UPDATE products
		SET name = $1, slug = $2, description = $3, price = $4, stock = $5, image_url = $6, updated_at = NOW()
		WHERE id = $7
		RETURNING id, name, slug, description, price, stock, image_url, created_at, updated_at
	`, *input.Name, *input.Slug, *input.Description, *input.Price, *input.Stock, input.ImageURL, id).Scan(
		&p.ID,
		&p.Name,
		&p.Slug,
		&p.Description,
		&p.Price,
		&p.Stock,
		&p.ImageURL,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Product{}, err
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Product{}, ErrDuplicateSlug
		}
		return Product{}, err
	}
	return p, nil
}
