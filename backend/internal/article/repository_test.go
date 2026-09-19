package article

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

func TestRepositoryCreateArticle(t *testing.T) {
	repo := newTestRepository(t)

	slug := uniqueSlug("test-article")
	article, err := repo.Create(context.Background(), CreateArticleInput{
		Title:   "Test Article",
		Slug:    slug,
		Content: "Test content",
		Status:  StatusDraft,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	t.Cleanup(func() {
		repo.db.Exec(context.Background(), "DELETE FROM articles WHERE id = $1", article.ID)
	})
	if article.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if article.Title != "Test Article" {
		t.Errorf("expected title 'Test Article', got %q", article.Title)
	}
	if article.Status != StatusDraft {
		t.Errorf("expected status 'draft', got %q", article.Status)
	}
}

func TestRepositoryCreateDuplicateSlug(t *testing.T) {
	repo := newTestRepository(t)

	slug := uniqueSlug("dup-article")
	_, err := repo.Create(context.Background(), CreateArticleInput{
		Title:   "First Article",
		Slug:    slug,
		Content: "Content 1",
		Status:  StatusDraft,
	})
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	// Try to create another with the same slug
	_, err = repo.Create(context.Background(), CreateArticleInput{
		Title:   "Second Article",
		Slug:    slug,
		Content: "Content 2",
		Status:  StatusDraft,
	})
	if err != ErrDuplicateSlug {
		t.Errorf("expected ErrDuplicateSlug, got %v", err)
	}
}

func TestRepositoryListPublishedOnly(t *testing.T) {
	repo := newTestRepository(t)

	// Create a published article
	publishedSlug := uniqueSlug("published-art")
	_, err := repo.Create(context.Background(), CreateArticleInput{
		Title:   "Published Article",
		Slug:    publishedSlug,
		Content: "Published content",
		Status:  StatusPublished,
	})
	if err != nil {
		t.Fatalf("create published failed: %v", err)
	}

	// Create a draft article
	draftSlug := uniqueSlug("draft-art")
	draft, err := repo.Create(context.Background(), CreateArticleInput{
		Title:   "Draft Article",
		Slug:    draftSlug,
		Content: "Draft content",
		Status:  StatusDraft,
	})
	if err != nil {
		t.Fatalf("create draft failed: %v", err)
	}
	t.Cleanup(func() {
		repo.db.Exec(context.Background(), "DELETE FROM articles WHERE id = $1", draft.ID)
	})

	// List should only return published
	result, err := repo.List(context.Background(), ListFilters{
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	found := false
	for _, a := range result.Items {
		if a.Slug == draftSlug {
			found = true
			break
		}
	}
	if found {
		t.Error("draft article should not appear in public listing")
	}
}

func TestRepositoryGetBySlug(t *testing.T) {
	repo := newTestRepository(t)

	slug := uniqueSlug("getslug-test")
	article, err := repo.Create(context.Background(), CreateArticleInput{
		Title:   "GetBySlug Test",
		Slug:    slug,
		Content: "Content",
		Status:  StatusPublished,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	t.Cleanup(func() {
		repo.db.Exec(context.Background(), "DELETE FROM articles WHERE id = $1", article.ID)
	})

	// GetBySlug should work for published
	found, err := repo.GetBySlug(context.Background(), slug)
	if err != nil {
		t.Fatalf("GetBySlug failed: %v", err)
	}
	if found.Title != "GetBySlug Test" {
		t.Errorf("expected title 'GetBySlug Test', got %q", found.Title)
	}

	// GetBySlug should not return drafts
	_, err = repo.GetBySlug(context.Background(), "nonexistent-slug-12345")
	if err == nil {
		t.Error("expected error for non-existent slug")
	}
}

func TestRepositoryUpdate(t *testing.T) {
	repo := newTestRepository(t)

	article, err := repo.Create(context.Background(), CreateArticleInput{
		Title:   "Original Title",
		Slug:    uniqueSlug("update-test"),
		Excerpt: "Original excerpt",
		Content: "Original content",
		Status:  StatusDraft,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	updated, err := repo.Update(context.Background(), article.ID, UpdateArticleInput{
		Title:   strPtr("Updated Title"),
		Slug:    strPtr(uniqueSlug("updated")),
		Excerpt: strPtr("Updated excerpt"),
		Content: strPtr("Updated content"),
		Status:  strPtr(StatusPublished),
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if updated.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %q", updated.Title)
	}
	if updated.Status != StatusPublished {
		t.Errorf("expected status 'published', got %q", updated.Status)
	}
}

func TestRepositoryDelete(t *testing.T) {
	repo := newTestRepository(t)

	article, err := repo.Create(context.Background(), CreateArticleInput{
		Title:   "Delete Test",
		Slug:    uniqueSlug("delete-test"),
		Content: "Content",
		Status:  StatusDraft,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	err = repo.Delete(context.Background(), article.ID)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	_, err = repo.AdminGetByID(context.Background(), article.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func strPtr(s string) *string {
	return &s
}
