package category

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestRepository(t *testing.T) *Repository {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	db, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Skip("cannot connect to database; skipping integration test: ", err)
	}
	t.Cleanup(db.Close)
	return NewRepository(db)
}

func strPtr(s string) *string { return &s }

func uniqueSlug(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }

func (r *Repository) cleanup(t *testing.T, id int64) {
	t.Helper()
	t.Cleanup(func() {
		r.db.Exec(context.Background(), "DELETE FROM categories WHERE id = $1", id)
	})
}

func TestRepositoryCreate(t *testing.T) {
	repo := newTestRepository(t)

	slug := uniqueSlug("repo-cat")
	c, err := repo.Create(context.Background(), CreateCategoryInput{Name: "Indoor Plants", Slug: slug})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	repo.cleanup(t, c.ID)

	if c.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if c.Name != "Indoor Plants" {
		t.Errorf("expected name 'Indoor Plants', got %q", c.Name)
	}
	if c.Slug != slug {
		t.Errorf("expected slug %q, got %q", slug, c.Slug)
	}
}

func TestRepositoryCreateDuplicateSlug(t *testing.T) {
	repo := newTestRepository(t)

	dupSlug := uniqueSlug("dup-cat-slug")
	first, err := repo.Create(context.Background(), CreateCategoryInput{Name: "First", Slug: dupSlug})
	if err != nil {
		t.Fatalf("first Create failed: %v", err)
	}
	repo.cleanup(t, first.ID)

	_, err = repo.Create(context.Background(), CreateCategoryInput{Name: "Second", Slug: dupSlug})
	if err != ErrDuplicateSlug {
		t.Fatalf("expected ErrDuplicateSlug, got %v", err)
	}
}

func TestRepositoryUpdate(t *testing.T) {
	repo := newTestRepository(t)

	created, err := repo.Create(context.Background(), CreateCategoryInput{Name: "Original", Slug: uniqueSlug("orig-cat")})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	repo.cleanup(t, created.ID)

	newSlug := uniqueSlug("updated-cat")
	updated, err := repo.Update(context.Background(), created.ID, UpdateCategoryInput{
		Name: strPtr("Updated"),
		Slug: strPtr(newSlug),
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Name != "Updated" {
		t.Errorf("expected name 'Updated', got %q", updated.Name)
	}
	if updated.Slug != newSlug {
		t.Errorf("expected slug %q, got %q", newSlug, updated.Slug)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) && updated.UpdatedAt != created.UpdatedAt {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestRepositoryUpdateNotFound(t *testing.T) {
	repo := newTestRepository(t)

	_, err := repo.Update(context.Background(), 999999999, UpdateCategoryInput{
		Name: strPtr("Test"),
		Slug: strPtr(uniqueSlug("not-found-cat")),
	})
	if err != pgx.ErrNoRows {
		t.Fatalf("expected pgx.ErrNoRows, got %v", err)
	}
}

func TestRepositoryUpdateDuplicateSlug(t *testing.T) {
	repo := newTestRepository(t)

	firstSlug := uniqueSlug("first-cat-slug")
	secondSlug := uniqueSlug("second-cat-slug")

	first, err := repo.Create(context.Background(), CreateCategoryInput{Name: "First", Slug: firstSlug})
	if err != nil {
		t.Fatalf("first Create failed: %v", err)
	}
	repo.cleanup(t, first.ID)

	second, err := repo.Create(context.Background(), CreateCategoryInput{Name: "Second", Slug: secondSlug})
	if err != nil {
		t.Fatalf("second Create failed: %v", err)
	}
	repo.cleanup(t, second.ID)

	_, err = repo.Update(context.Background(), second.ID, UpdateCategoryInput{
		Name: strPtr("Second Updated"),
		Slug: strPtr(firstSlug),
	})
	if err != ErrDuplicateSlug {
		t.Fatalf("expected ErrDuplicateSlug, got %v", err)
	}
}

func TestRepositoryDeleteNotFound(t *testing.T) {
	repo := newTestRepository(t)

	err := repo.Delete(context.Background(), 999999999)
	if err != pgx.ErrNoRows {
		t.Fatalf("expected pgx.ErrNoRows, got %v", err)
	}
}

func TestRepositoryDeleteSafe(t *testing.T) {
	repo := newTestRepository(t)

	created, err := repo.Create(context.Background(), CreateCategoryInput{Name: "Deletable", Slug: uniqueSlug("del-cat")})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := repo.Delete(context.Background(), created.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = repo.GetByID(context.Background(), created.ID)
	if err != pgx.ErrNoRows {
		t.Fatalf("expected category to be gone, got %v", err)
	}
}

func TestRepositoryDeleteReferencedByProductReturnsConflict(t *testing.T) {
	repo := newTestRepository(t)

	created, err := repo.Create(context.Background(), CreateCategoryInput{Name: "In Use", Slug: uniqueSlug("inuse-cat")})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	t.Cleanup(func() {
		repo.db.Exec(context.Background(), "DELETE FROM categories WHERE id = $1", created.ID)
	})

	var productID int64
	err = repo.db.QueryRow(context.Background(), `
		INSERT INTO products (name, slug, price, stock, category_id)
		VALUES ($1, $2, 100, 1, $3)
		RETURNING id
	`, "Category Product "+uniqueSlug("p"), uniqueSlug("cat-product"), created.ID).Scan(&productID)
	if err != nil {
		t.Fatalf("seed product failed: %v", err)
	}
	t.Cleanup(func() {
		repo.db.Exec(context.Background(), "DELETE FROM products WHERE id = $1", productID)
	})

	err = repo.Delete(context.Background(), created.ID)
	if err != ErrCategoryReferenced {
		t.Fatalf("expected ErrCategoryReferenced, got %v", err)
	}
}
