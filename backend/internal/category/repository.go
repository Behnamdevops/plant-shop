package category

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

func (r *Repository) List(ctx context.Context) ([]Category, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, slug, created_at, updated_at
		FROM categories
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := make([]Category, 0)

	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}

	return categories, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, id int64) (Category, error) {
	var c Category
	err := r.db.QueryRow(ctx, `
		SELECT id, name, slug, created_at, updated_at
		FROM categories
		WHERE id = $1
	`, id).Scan(&c.ID, &c.Name, &c.Slug, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (r *Repository) GetBySlug(ctx context.Context, slug string) (Category, error) {
	var c Category
	err := r.db.QueryRow(ctx, `
		SELECT id, name, slug, created_at, updated_at
		FROM categories
		WHERE slug = $1
	`, slug).Scan(&c.ID, &c.Name, &c.Slug, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (r *Repository) Create(ctx context.Context, input CreateCategoryInput) (Category, error) {
	var c Category
	err := r.db.QueryRow(ctx, `
		INSERT INTO categories (name, slug)
		VALUES ($1, $2)
		RETURNING id, name, slug, created_at, updated_at
	`, input.Name, input.Slug).Scan(&c.ID, &c.Name, &c.Slug, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Category{}, ErrDuplicateSlug
		}
		return Category{}, err
	}
	return c, nil
}

func (r *Repository) Update(ctx context.Context, id int64, input UpdateCategoryInput) (Category, error) {
	var c Category
	err := r.db.QueryRow(ctx, `
		UPDATE categories
		SET name = $1, slug = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING id, name, slug, created_at, updated_at
	`, *input.Name, *input.Slug, id).Scan(&c.ID, &c.Name, &c.Slug, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Category{}, err
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Category{}, ErrDuplicateSlug
		}
		return Category{}, err
	}
	return c, nil
}

// Delete removes a category by ID. products.category_id has ON DELETE SET
// NULL, so we explicitly refuse deletion while any product still
// references this category rather than silently uncategorizing products
// implicitly — the admin must consciously move products first. The
// existence check and delete run inside one transaction to avoid a
// check-then-act race against a concurrent product update.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())

	var inUse bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE category_id = $1)`, id).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return ErrCategoryReferenced
	}

	tag, err := tx.Exec(ctx, `DELETE FROM categories WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return tx.Commit(ctx)
}
