package product

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

const productColumns = `
	id,
	name,
	slug,
	description,
	price,
	stock,
	image_url,
	category_id,
	created_at,
	updated_at
`

func scanProduct(row pgx.Row, p *Product) error {
	return row.Scan(
		&p.ID,
		&p.Name,
		&p.Slug,
		&p.Description,
		&p.Price,
		&p.Stock,
		&p.ImageURL,
		&p.CategoryID,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
}

// List returns products for the public catalog listing, applying search,
// filter, sort, and pagination according to filters. Sort is resolved via
// the sortColumns allow-list only — never interpolated from raw client
// input. All filter values are passed as parameterized query arguments.
func (r *Repository) List(ctx context.Context, filters ListFilters) (ListResult, error) {
	orderBy, ok := sortColumns[filters.Sort]
	if !ok {
		orderBy = sortColumns[DefaultSort]
	}

	var where []string
	var args []any

	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if q := strings.TrimSpace(filters.Query); q != "" {
		placeholder := arg("%" + q + "%")
		where = append(where, fmt.Sprintf(
			"(name ILIKE %s OR description ILIKE %s OR slug ILIKE %s)",
			placeholder, placeholder, placeholder,
		))
	}
	if filters.CategoryID != nil {
		where = append(where, fmt.Sprintf("category_id = %s", arg(*filters.CategoryID)))
	}
	if filters.InStock {
		where = append(where, "stock > 0")
	}
	if filters.MinPrice != nil {
		where = append(where, fmt.Sprintf("price >= %s", arg(*filters.MinPrice)))
	}
	if filters.MaxPrice != nil {
		where = append(where, fmt.Sprintf("price <= %s", arg(*filters.MaxPrice)))
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM products %s`, whereClause)
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return ListResult{}, err
	}

	page := filters.Page
	pageSize := filters.PageSize
	offset := (page - 1) * pageSize

	limitArg := arg(pageSize)
	offsetArg := arg(offset)

	listQuery := fmt.Sprintf(`
		SELECT %s
		FROM products
		%s
		ORDER BY %s
		LIMIT %s OFFSET %s
	`, productColumns, whereClause, orderBy, limitArg, offsetArg)

	rows, err := r.db.Query(ctx, listQuery, args...)
	if err != nil {
		return ListResult{}, err
	}
	defer rows.Close()

	items := make([]Product, 0)
	for rows.Next() {
		var p Product
		if err := scanProduct(rows, &p); err != nil {
			return ListResult{}, err
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, err
	}

	totalPages := 0
	if pageSize > 0 {
		totalPages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	return ListResult{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

func (r *Repository) GetBySlug(ctx context.Context, slug string) (Product, error) {
	var p Product
	err := scanProduct(r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT %s
		FROM products
		WHERE slug = $1
	`, productColumns), slug), &p)
	return p, err
}

func (r *Repository) GetByID(ctx context.Context, id int64) (Product, error) {
	var p Product
	err := scanProduct(r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT %s
		FROM products
		WHERE id = $1
	`, productColumns), id), &p)
	return p, err
}

func (r *Repository) Create(ctx context.Context, input CreateProductInput) (Product, error) {
	var p Product
	err := scanProduct(r.db.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO products (name, slug, description, price, stock, image_url, category_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING %s
	`, productColumns), input.Name, input.Slug, input.Description, input.Price, input.Stock, input.ImageURL, input.CategoryID), &p)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return Product{}, ErrDuplicateSlug
			case "23503":
				return Product{}, ErrCategoryNotFound
			}
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
	var categoryID *int64
	if !input.ClearCategory {
		categoryID = input.CategoryID
	}

	var p Product
	err := scanProduct(r.db.QueryRow(ctx, fmt.Sprintf(`
		UPDATE products
		SET name = $1, slug = $2, description = $3, price = $4, stock = $5, image_url = $6,
			category_id = CASE WHEN $8 THEN NULL ELSE COALESCE($7, category_id) END,
			updated_at = NOW()
		WHERE id = $9
		RETURNING %s
	`, productColumns),
		*input.Name, *input.Slug, *input.Description, *input.Price, *input.Stock, input.ImageURL,
		categoryID, input.ClearCategory, id,
	), &p)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Product{}, err
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return Product{}, ErrDuplicateSlug
			case "23503":
				return Product{}, ErrCategoryNotFound
			}
		}
		return Product{}, err
	}
	return p, nil
}
