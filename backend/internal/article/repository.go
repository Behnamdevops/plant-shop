package article

import (
	"context"
	"database/sql"
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

const articleColumns = `
	id,
	title,
	slug,
	excerpt,
	content,
	cover_image_url,
	category_id,
	status,
	published_at,
	seo_title,
	seo_description,
	created_at,
	updated_at
`

func scanArticle(row pgx.Row, a *Article) error {
	return row.Scan(
		&a.ID,
		&a.Title,
		&a.Slug,
		&a.Excerpt,
		&a.Content,
		&a.CoverImageURL,
		&a.CategoryID,
		&a.Status,
		&a.PublishedAt,
		&a.SeoTitle,
		&a.SeoDescription,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
}

// scanCategoryID safely extracts an int64 from sql.NullString
func scanCategoryID(v sql.NullString) (int64, bool) {
	if !v.Valid {
		return 0, false
	}
	var id int64
	_, err := fmt.Sscan(v.String, &id)
	if err != nil {
		return 0, false
	}
	return id, true
}

// List returns published articles for the public listing with search,
// filter, and pagination.
func (r *Repository) List(ctx context.Context, filters ListFilters) (ListResult, error) {
	var where []string
	var args []any

	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	// Only published articles for public listing
	where = append(where, "status = 'published'")

	if q := strings.TrimSpace(filters.Query); q != "" {
		placeholder := arg("%" + q + "%")
		where = append(where, fmt.Sprintf(
			"(title ILIKE %s OR excerpt ILIKE %s OR slug ILIKE %s)",
			placeholder, placeholder, placeholder,
		))
	}
	if filters.CategoryID != nil {
		where = append(where, fmt.Sprintf("category_id = %s", arg(*filters.CategoryID)))
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM articles %s`, whereClause)
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return ListResult{}, err
	}

	page := filters.Page
	pageSize := filters.PageSize
	offset := (page - 1) * pageSize

	limitArg := arg(pageSize)
	offsetArg := arg(offset)

	// Join with article_categories to get category info
	listQuery := fmt.Sprintf(`
		SELECT a.id, a.title, a.slug, a.excerpt, a.content, a.cover_image_url,
		       a.category_id, a.status, a.published_at, a.seo_title, a.seo_description,
		       a.created_at, a.updated_at,
		       c.id, c.name, c.slug
		FROM articles a
		LEFT JOIN article_categories c ON a.category_id = c.id
		%s
		ORDER BY a.published_at DESC NULLS LAST, a.id DESC
		LIMIT %s OFFSET %s
	`, whereClause, limitArg, offsetArg)

	rows, err := r.db.Query(ctx, listQuery, args...)
	if err != nil {
		return ListResult{}, err
	}
	defer rows.Close()

	items := make([]Article, 0)
	for rows.Next() {
		var a Article
		var categoryID, categoryName, categorySlug sql.NullString
		if err := rows.Scan(
			&a.ID, &a.Title, &a.Slug, &a.Excerpt, &a.Content, &a.CoverImageURL,
			&a.CategoryID, &a.Status, &a.PublishedAt, &a.SeoTitle, &a.SeoDescription,
			&a.CreatedAt, &a.UpdatedAt,
			&categoryID, &categoryName, &categorySlug,
		); err != nil {
			return ListResult{}, err
		}
		if catID, ok := scanCategoryID(categoryID); ok {
			a.Category = &Category{
				ID:   catID,
				Name: categoryName.String,
				Slug: categorySlug.String,
			}
		}
		items = append(items, a)
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

// AdminList returns all articles (draft and published) for admin listing.
func (r *Repository) AdminList(ctx context.Context, page, pageSize int) (ListResult, error) {
	offset := (page - 1) * pageSize

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM articles`).Scan(&total); err != nil {
		return ListResult{}, err
	}

	listQuery := `
		SELECT a.id, a.title, a.slug, a.excerpt, a.content, a.cover_image_url,
		       a.category_id, a.status, a.published_at, a.seo_title, a.seo_description,
		       a.created_at, a.updated_at,
		       c.id, c.name, c.slug
		FROM articles a
		LEFT JOIN article_categories c ON a.category_id = c.id
		ORDER BY a.id DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, listQuery, pageSize, offset)
	if err != nil {
		return ListResult{}, err
	}
	defer rows.Close()

	items := make([]Article, 0)
	for rows.Next() {
		var a Article
		var categoryID, categoryName, categorySlug sql.NullString
		if err := rows.Scan(
			&a.ID, &a.Title, &a.Slug, &a.Excerpt, &a.Content, &a.CoverImageURL,
			&a.CategoryID, &a.Status, &a.PublishedAt, &a.SeoTitle, &a.SeoDescription,
			&a.CreatedAt, &a.UpdatedAt,
			&categoryID, &categoryName, &categorySlug,
		); err != nil {
			return ListResult{}, err
		}
		if catID, ok := scanCategoryID(categoryID); ok {
			a.Category = &Category{
				ID:   catID,
				Name: categoryName.String,
				Slug: categorySlug.String,
			}
		}
		items = append(items, a)
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

// GetBySlug returns a published article by slug for public access.
func (r *Repository) GetBySlug(ctx context.Context, slug string) (Article, error) {
	var a Article
	var categoryID, categoryName, categorySlug sql.NullString
	query := `
		SELECT a.id, a.title, a.slug, a.excerpt, a.content, a.cover_image_url,
		       a.category_id, a.status, a.published_at, a.seo_title, a.seo_description,
		       a.created_at, a.updated_at,
		       c.id, c.name, c.slug
		FROM articles a
		LEFT JOIN article_categories c ON a.category_id = c.id
		WHERE a.slug = $1 AND a.status = 'published'
	`
	err := r.db.QueryRow(ctx, query, slug).Scan(
		&a.ID, &a.Title, &a.Slug, &a.Excerpt, &a.Content, &a.CoverImageURL,
		&a.CategoryID, &a.Status, &a.PublishedAt, &a.SeoTitle, &a.SeoDescription,
		&a.CreatedAt, &a.UpdatedAt,
		&categoryID, &categoryName, &categorySlug,
	)
	if err != nil {
		return Article{}, err
	}
	if catID, ok := scanCategoryID(categoryID); ok {
		a.Category = &Category{
			ID:   catID,
			Name: categoryName.String,
			Slug: categorySlug.String,
		}
	}
	return a, nil
}

// AdminGetByID returns any article (draft or published) by ID for admin access.
func (r *Repository) AdminGetByID(ctx context.Context, id int64) (Article, error) {
	var a Article
	var categoryID, categoryName, categorySlug sql.NullString
	query := `
		SELECT a.id, a.title, a.slug, a.excerpt, a.content, a.cover_image_url,
		       a.category_id, a.status, a.published_at, a.seo_title, a.seo_description,
		       a.created_at, a.updated_at,
		       c.id, c.name, c.slug
		FROM articles a
		LEFT JOIN article_categories c ON a.category_id = c.id
		WHERE a.id = $1
	`
	err := r.db.QueryRow(ctx, query, id).Scan(
		&a.ID, &a.Title, &a.Slug, &a.Excerpt, &a.Content, &a.CoverImageURL,
		&a.CategoryID, &a.Status, &a.PublishedAt, &a.SeoTitle, &a.SeoDescription,
		&a.CreatedAt, &a.UpdatedAt,
		&categoryID, &categoryName, &categorySlug,
	)
	if err != nil {
		return Article{}, err
	}
	if catID, ok := scanCategoryID(categoryID); ok {
		a.Category = &Category{
			ID:   catID,
			Name: categoryName.String,
			Slug: categorySlug.String,
		}
	}
	return a, nil
}

// Create inserts a new article.
func (r *Repository) Create(ctx context.Context, input CreateArticleInput) (Article, error) {
	var a Article
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO articles (title, slug, excerpt, content, cover_image_url, category_id, status, published_at, seo_title, seo_description)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING %s
	`, articleColumns),
		input.Title, input.Slug, input.Excerpt, input.Content, input.CoverImageURL,
		input.CategoryID, input.Status, input.PublishedAt, input.SeoTitle, input.SeoDescription,
	).Scan(
		&a.ID, &a.Title, &a.Slug, &a.Excerpt, &a.Content, &a.CoverImageURL,
		&a.CategoryID, &a.Status, &a.PublishedAt, &a.SeoTitle, &a.SeoDescription,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return Article{}, ErrDuplicateSlug
			case "23503":
				return Article{}, ErrCategoryNotFound
			}
		}
		return Article{}, err
	}
	return a, nil
}

// Update modifies an existing article.
func (r *Repository) Update(ctx context.Context, id int64, input UpdateArticleInput) (Article, error) {
	var categoryID *int64
	if !input.ClearCategory {
		categoryID = input.CategoryID
	}

	var a Article
	query := `
		UPDATE articles
		SET title = $1, slug = $2, excerpt = $3, content = $4, cover_image_url = $5,
		    category_id = CASE WHEN $11 THEN NULL ELSE COALESCE($6, category_id) END,
		    status = $7, published_at = $8, seo_title = $9, seo_description = $10,
		    updated_at = NOW()
		WHERE id = $12
		RETURNING ` + articleColumns
	err := r.db.QueryRow(ctx, query,
		input.Title, input.Slug, input.Excerpt, input.Content, input.CoverImageURL,
		categoryID, *input.Status, input.PublishedAt, input.SeoTitle, input.SeoDescription,
		input.ClearCategory, id,
	).Scan(
		&a.ID, &a.Title, &a.Slug, &a.Excerpt, &a.Content, &a.CoverImageURL,
		&a.CategoryID, &a.Status, &a.PublishedAt, &a.SeoTitle, &a.SeoDescription,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Article{}, err
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return Article{}, ErrDuplicateSlug
			case "23503":
				return Article{}, ErrCategoryNotFound
			}
		}
		return Article{}, err
	}
	return a, nil
}

// Delete removes an article by ID.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM articles WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
