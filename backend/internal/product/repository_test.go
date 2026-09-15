package product

import (
	"context"
	"errors"
	"os"
	"testing"

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
	return NewRepository(db)
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
func int64Ptr(i int64) *int64 { return &i }

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

func TestRepositoryUpdate(t *testing.T) {
	repo := newTestRepository(t)

	created, err := repo.Create(context.Background(), CreateProductInput{
		Name:  "Original",
		Slug:  "original-slug",
		Price: 100,
		Stock: 5,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	name := "Updated"
	slug := "updated-slug"
	desc := "New description"
	price := int64(200)
	stock := 10
	updated, err := repo.Update(context.Background(), created.ID, UpdateProductInput{
		Name:        &name,
		Slug:        &slug,
		Description: &desc,
		Price:       &price,
		Stock:       &stock,
		ImageURL:    nil,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, updated.ID)
	}
	if updated.Name != "Updated" {
		t.Errorf("expected name 'Updated', got %q", updated.Name)
	}
	if updated.Slug != "updated-slug" {
		t.Errorf("expected slug 'updated-slug', got %q", updated.Slug)
	}
	if updated.Description != "New description" {
		t.Errorf("expected description 'New description', got %q", updated.Description)
	}
	if updated.Price != 200 {
		t.Errorf("expected price 200, got %d", updated.Price)
	}
	if updated.Stock != 10 {
		t.Errorf("expected stock 10, got %d", updated.Stock)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestRepositoryUpdateNotFound(t *testing.T) {
	repo := newTestRepository(t)

	_, err := repo.Update(context.Background(), 999999, UpdateProductInput{
		Name:        strPtr("Test"),
		Slug:        strPtr("test-slug"),
		Description: strPtr("desc"),
		Price:       int64Ptr(100),
		Stock:       intPtr(1),
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows, got %v", err)
	}
}

func TestRepositoryUpdateDuplicateSlug(t *testing.T) {
	repo := newTestRepository(t)

	_, err := repo.Create(context.Background(), CreateProductInput{
		Name:  "First",
		Slug:  "first-slug",
		Price: 10,
		Stock: 1,
	})
	if err != nil {
		t.Fatalf("first Create failed: %v", err)
	}

	second, err := repo.Create(context.Background(), CreateProductInput{
		Name:  "Second",
		Slug:  "second-slug",
		Price: 20,
		Stock: 2,
	})
	if err != nil {
		t.Fatalf("second Create failed: %v", err)
	}

	_, err = repo.Update(context.Background(), second.ID, UpdateProductInput{
		Name:        strPtr("Second Updated"),
		Slug:        strPtr("first-slug"),
		Description: strPtr("desc"),
		Price:       int64Ptr(30),
		Stock:       intPtr(3),
	})
	if err != ErrDuplicateSlug {
		t.Fatalf("expected ErrDuplicateSlug, got %v", err)
	}
}
