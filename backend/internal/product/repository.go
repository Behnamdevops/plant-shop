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
	updated_at, details, image_urls, COALESCE((SELECT SUM(oi.quantity) FROM order_items oi JOIN orders o ON o.id=oi.order_id WHERE oi.product_id=products.id AND o.payment_status='paid' AND o.status!='cancelled'),0)
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
		&p.UpdatedAt, &p.Details, &p.ImageURLs, &p.SoldQuantity,
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

	for _, f := range []struct{ key, value string }{{"kind", filters.Kind}, {"brand", filters.Brand}, {"formulation", filters.Formulation}, {"article_slug", filters.Guide}} {
		if f.value != "" {
			where = append(where, fmt.Sprintf("details->>'%s' = %s", f.key, arg(f.value)))
		}
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
		WHERE slug = $1 OR (id IN(SELECT product_id FROM product_slug_aliases WHERE slug=$1) AND NOT EXISTS(SELECT 1 FROM products WHERE slug=$1))
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
		INSERT INTO products (name, slug, description, price, stock, image_url, category_id,details,image_urls)
		VALUES ($1, $2, $3, $4, $5, $6, $7,$8,$9)
		RETURNING %s
	`, productColumns), input.Name, input.Slug, input.Description, input.Price, input.Stock, input.ImageURL, input.CategoryID, input.Details, normalizedImages(input.ImageURLs)), &p)
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

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Product{}, err
	}
	defer tx.Rollback(ctx)
	var oldStock int
	var oldSlug string
	if err = tx.QueryRow(ctx, "SELECT stock,slug FROM products WHERE id=$1 FOR UPDATE", id).Scan(&oldStock, &oldSlug); err != nil {
		return Product{}, err
	}
	var p Product
	err = scanProduct(tx.QueryRow(ctx, fmt.Sprintf(`
		UPDATE products
		SET name = $1, slug = $2, description = $3, price = $4, stock = $5, image_url = $6,
			category_id = CASE WHEN $8 THEN NULL ELSE COALESCE($7, category_id) END,
			details=COALESCE($10,details),image_urls=COALESCE($11,image_urls),updated_at = NOW()
		WHERE id = $9
		RETURNING %s
	`, productColumns),
		*input.Name, *input.Slug, *input.Description, *input.Price, *input.Stock, input.ImageURL,
		categoryID, input.ClearCategory, id, input.Details, input.ImageURLs,
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
	if oldStock != *input.Stock {
		reason := strings.TrimSpace(input.StockReason)
		if reason == "" {
			reason = "ویرایش موجودی از فرم محصول"
		}
		if _, err = tx.Exec(ctx, "INSERT INTO inventory_adjustments(product_id,admin_user_id,delta,stock_before,stock_after,reason) VALUES($1,$2,$3,$4,$5,$6)", id, input.AdminUserID, *input.Stock-oldStock, oldStock, *input.Stock, reason); err != nil {
			return Product{}, err
		}
	}
	if oldSlug != p.Slug {
		if _, err = tx.Exec(ctx, "INSERT INTO product_slug_aliases(slug,product_id) VALUES($1,$2) ON CONFLICT(slug) DO NOTHING", oldSlug, id); err != nil {
			return Product{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return Product{}, err
	}
	return p, nil
}

func normalizedImages(images []string) []string {
	if images == nil {
		return []string{}
	}
	return images
}
func (r *Repository) ImageReferenced(ctx context.Context, url string) (bool, error) {
	var used bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM products WHERE image_url=$1 OR image_urls ? $1) OR EXISTS(SELECT 1 FROM articles WHERE cover_image_url=$1 OR position($1 in content)>0)`, url).Scan(&used)
	return used, err
}

func (r *Repository) Related(ctx context.Context, p Product) ([]Product, error) {
	ids := p.Details.RelatedIDs
	if ids == nil {
		ids = []int64{}
	}
	rows, err := r.db.Query(ctx, "SELECT "+productColumns+" FROM products WHERE id!=$1 AND stock>0 AND (id=ANY($2) OR (cardinality($2::bigint[])=0 AND category_id=$3)) ORDER BY array_position($2::bigint[],id) NULLS LAST,id DESC LIMIT 8", p.ID, ids, p.CategoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Product{}
	for rows.Next() {
		var item Product
		if err = scanProduct(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
