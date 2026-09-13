package product

import (
	"context"
	"os"
	"testing"

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

func TestRepositoryCreate(t *testing.T) {
	repo := newTestRepository(t)

	product, err := repo.Create(context.Background(), CreateProductInput{
		Name:  "Repo Test Plant",
		Slug:  "repo-test-plant",
		Price: 50,
		Stock: 3,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if product.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if product.Name != "Repo Test Plant" {
		t.Errorf("expected name 'Repo Test Plant', got %q", product.Name)
	}
	if product.Slug != "repo-test-plant" {
		t.Errorf("expected slug 'repo-test-plant', got %q", product.Slug)
	}
	if product.Price != 50 {
		t.Errorf("expected price 50, got %d", product.Price)
	}
	if product.Stock != 3 {
		t.Errorf("expected stock 3, got %d", product.Stock)
	}
}

func TestRepositoryCreateDuplicateSlug(t *testing.T) {
	repo := newTestRepository(t)

	_, err := repo.Create(context.Background(), CreateProductInput{
		Name:  "First",
		Slug:  "dup-slug",
		Price: 10,
		Stock: 1,
	})
	if err != nil {
		t.Fatalf("first Create failed: %v", err)
	}

	_, err = repo.Create(context.Background(), CreateProductInput{
		Name:  "Second",
		Slug:  "dup-slug",
		Price: 20,
		Stock: 2,
	})
	if err != ErrDuplicateSlug {
		t.Fatalf("expected ErrDuplicateSlug, got %v", err)
	}
}
