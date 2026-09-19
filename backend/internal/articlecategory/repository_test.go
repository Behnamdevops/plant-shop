package articlecategory

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

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
	return NewRepository(db)
}

func uniqueSlug(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func TestRepositoryCreate(t *testing.T) {
	repo := newTestRepository(t)

	slug := uniqueSlug("test-cat")
	category, err := repo.Create(context.Background(), CreateCategoryInput{
		Name: "Test Category",
		Slug: slug,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	t.Cleanup(func() {
		repo.db.Exec(context.Background(), "DELETE FROM article_categories WHERE id = $1", category.ID)
	})
	if category.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if category.Name != "Test Category" {
		t.Errorf("expected name 'Test Category', got %q", category.Name)
	}
	if category.Slug != slug {
		t.Errorf("expected slug %q, got %q", slug, category.Slug)
	}
}

func TestRepositoryCreateDuplicateSlug(t *testing.T) {
	repo := newTestRepository(t)

	slug := uniqueSlug("dup-test")
	_, err := repo.Create(context.Background(), CreateCategoryInput{
		Name: "Duplicate Test",
		Slug: slug,
	})
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	// Try to create another with the same slug
	_, err = repo.Create(context.Background(), CreateCategoryInput{
		Name: "Duplicate Test 2",
		Slug: slug,
	})
	if err != ErrDuplicateSlug {
		t.Errorf("expected ErrDuplicateSlug, got %v", err)
	}
}

func TestRepositoryUpdate(t *testing.T) {
	repo := newTestRepository(t)

	// Create first
	category, err := repo.Create(context.Background(), CreateCategoryInput{
		Name: "Original Name",
		Slug: uniqueSlug("update-test"),
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Update to a unique slug to avoid conflicts on repeat runs
	newSlug := uniqueSlug("updated-slug")
	updated, err := repo.Update(context.Background(), category.ID, UpdateCategoryInput{
		Name: strPtr("Updated Name"),
		Slug: &newSlug,
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if updated.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got %q", updated.Name)
	}
	if updated.Slug != newSlug {
		t.Errorf("expected slug %q, got %q", newSlug, updated.Slug)
	}
}

func TestRepositoryDelete(t *testing.T) {
	repo := newTestRepository(t)

	// Create a category
	category, err := repo.Create(context.Background(), CreateCategoryInput{
		Name: "Delete Test",
		Slug: uniqueSlug("delete-test"),
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Delete it
	err = repo.Delete(context.Background(), category.ID)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Verify it's gone
	_, err = repo.GetByID(context.Background(), category.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func strPtr(s string) *string {
	return &s
}
