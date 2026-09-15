package auth

import (
	"context"
	"errors"
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
	return NewRepository(db)
}

func TestRepositoryCreate(t *testing.T) {
	repo := newTestRepository(t)
	user, err := repo.CreateUser(context.Background(), "testuser", "testuser@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if user.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if user.Email != "testuser@example.com" {
		t.Errorf("expected email 'testuser@example.com', got %q", user.Email)
	}
}

func TestRepositoryCreateDuplicate(t *testing.T) {
	repo := newTestRepository(t)
	_, err := repo.CreateUser(context.Background(), "dupuser", "dup@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("first CreateUser failed: %v", err)
	}
	_, err = repo.CreateUser(context.Background(), "dupuser2", "dup@example.com", "hashedpassword")
	if !errors.Is(err, ErrDuplicateEmail) {
		t.Fatalf("expected ErrDuplicateEmail, got %v", err)
	}
}

func TestRepositoryFindByEmail(t *testing.T) {
	repo := newTestRepository(t)
	_, err := repo.CreateUser(context.Background(), "finduser", "find@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	user, err := repo.FindUserByEmail(context.Background(), "find@example.com")
	if err != nil {
		t.Fatalf("FindUserByEmail failed: %v", err)
	}
	if user.Email != "find@example.com" {
		t.Errorf("expected email 'find@example.com', got %q", user.Email)
	}
}

func TestRepositoryFindByEmailNotFound(t *testing.T) {
	repo := newTestRepository(t)
	_, err := repo.FindUserByEmail(context.Background(), "nonexistent@example.com")
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows, got %v", err)
	}
}

func TestRepositoryFindByID(t *testing.T) {
	repo := newTestRepository(t)
	created, err := repo.CreateUser(context.Background(), "byiduser", "byid@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	user, err := repo.FindUserByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("FindUserByID failed: %v", err)
	}
	if user.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, user.ID)
	}
}

func TestRepositoryFindByIDNotFound(t *testing.T) {
	repo := newTestRepository(t)
	_, err := repo.FindUserByID(context.Background(), 999999)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows, got %v", err)
	}
}

func TestRepositoryCreateSessionAndFind(t *testing.T) {
	repo := newTestRepository(t)
	created, err := repo.CreateUser(context.Background(), "sessionuser", "session@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	session, err := repo.CreateSession(context.Background(), created.ID, "testtokenhash", time.Now().Add(7*24*time.Hour))
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if session.UserID != created.ID {
		t.Errorf("expected UserID %d, got %d", created.ID, session.UserID)
	}

	found, err := repo.FindSessionByTokenHash(context.Background(), "testtokenhash")
	if err != nil {
		t.Fatalf("FindSessionByTokenHash failed: %v", err)
	}
	if found.ID != session.ID {
		t.Errorf("expected session ID %d, got %d", session.ID, found.ID)
	}
}

func TestRepositoryDeleteSessionByID(t *testing.T) {
	repo := newTestRepository(t)
	created, err := repo.CreateUser(context.Background(), "deluser", "del@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	session, err := repo.CreateSession(context.Background(), created.ID, "deltokenhash", time.Now().Add(7*24*time.Hour))
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	err = repo.DeleteSessionByID(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("DeleteSessionByID failed: %v", err)
	}
	_, err = repo.FindSessionByTokenHash(context.Background(), "deltokenhash")
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows after delete, got %v", err)
	}
}
