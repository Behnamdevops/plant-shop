package product

import (
	"context"
	"errors"

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
