package auth

import (
	"context"
	"errors"
	"os"
	"strconv"
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

func uniqueEmail(prefix string) string {
	return prefix + "-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "@example.com"
}

func uniqueToken(prefix string) string {
	return prefix + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func cleanupTestUser(t *testing.T, repo *Repository, userID int64) {
	t.Helper()

	t.Cleanup(func() {
		ctx := context.Background()

		// Delete sessions first so cleanup works even if the FK
		// does not use ON DELETE CASCADE.
		if _, err := repo.db.Exec(
			ctx,
			"DELETE FROM sessions WHERE user_id = $1",
			userID,
		); err != nil {
			t.Errorf("cleanup sessions: %v", err)
		}

		if _, err := repo.db.Exec(
			ctx,
			"DELETE FROM users WHERE id = $1",
			userID,
		); err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	})
}

func TestRepositoryCreate(t *testing.T) {
	repo := newTestRepository(t)

	email := uniqueEmail("testuser")

	user, err := repo.CreateUser(
		context.Background(),
		"testuser",
		email,
		"hashedpassword",
	)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	cleanupTestUser(t, repo, user.ID)

	if user.ID == 0 {
		t.Error("expected non-zero ID")
	}

	if user.Email != email {
		t.Errorf("expected email %q, got %q", email, user.Email)
	}
}

func TestRepositoryCreateDuplicate(t *testing.T) {
	repo := newTestRepository(t)

	email := uniqueEmail("dupuser")

	first, err := repo.CreateUser(
		context.Background(),
		"dupuser",
		email,
		"hashedpassword",
	)
	if err != nil {
		t.Fatalf("first CreateUser failed: %v", err)
	}

	cleanupTestUser(t, repo, first.ID)

	second, err := repo.CreateUser(
		context.Background(),
		"dupuser2",
		email,
		"hashedpassword",
	)

	// In case duplicate protection is broken and the second insert succeeds,
	// still clean up the unexpected row.
	if second.ID != 0 {
		cleanupTestUser(t, repo, second.ID)
	}

	if !errors.Is(err, ErrDuplicateEmail) {
		t.Fatalf("expected ErrDuplicateEmail, got %v", err)
	}
}

func TestRepositoryFindByEmail(t *testing.T) {
	repo := newTestRepository(t)

	email := uniqueEmail("finduser")

	created, err := repo.CreateUser(
		context.Background(),
		"finduser",
		email,
		"hashedpassword",
	)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	cleanupTestUser(t, repo, created.ID)

	user, err := repo.FindUserByEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("FindUserByEmail failed: %v", err)
	}

	if user.Email != email {
		t.Errorf("expected email %q, got %q", email, user.Email)
	}
}

func TestRepositoryFindByEmailNotFound(t *testing.T) {
	repo := newTestRepository(t)

	email := uniqueEmail("nonexistent")

	_, err := repo.FindUserByEmail(context.Background(), email)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows, got %v", err)
	}
}

func TestRepositoryFindByID(t *testing.T) {
	repo := newTestRepository(t)

	email := uniqueEmail("byiduser")

	created, err := repo.CreateUser(
		context.Background(),
		"byiduser",
		email,
		"hashedpassword",
	)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	cleanupTestUser(t, repo, created.ID)

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

	_, err := repo.FindUserByID(context.Background(), -1)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows, got %v", err)
	}
}

func TestRepositoryCreateSessionAndFind(t *testing.T) {
	repo := newTestRepository(t)

	email := uniqueEmail("sessionuser")
	tokenHash := uniqueToken("testtokenhash")

	created, err := repo.CreateUser(
		context.Background(),
		"sessionuser",
		email,
		"hashedpassword",
	)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	cleanupTestUser(t, repo, created.ID)

	session, err := repo.CreateSession(
		context.Background(),
		created.ID,
		tokenHash,
		time.Now().Add(7*24*time.Hour),
	)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.UserID != created.ID {
		t.Errorf(
			"expected UserID %d, got %d",
			created.ID,
			session.UserID,
		)
	}

	found, err := repo.FindSessionByTokenHash(
		context.Background(),
		tokenHash,
	)
	if err != nil {
		t.Fatalf("FindSessionByTokenHash failed: %v", err)
	}

	if found.ID != session.ID {
		t.Errorf(
			"expected session ID %d, got %d",
			session.ID,
			found.ID,
		)
	}
}

func TestRepositoryDeleteSessionByID(t *testing.T) {
	repo := newTestRepository(t)

	email := uniqueEmail("deluser")
	tokenHash := uniqueToken("deltokenhash")

	created, err := repo.CreateUser(
		context.Background(),
		"deluser",
		email,
		"hashedpassword",
	)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	cleanupTestUser(t, repo, created.ID)

	session, err := repo.CreateSession(
		context.Background(),
		created.ID,
		tokenHash,
		time.Now().Add(7*24*time.Hour),
	)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	err = repo.DeleteSessionByID(
		context.Background(),
		session.ID,
	)
	if err != nil {
		t.Fatalf("DeleteSessionByID failed: %v", err)
	}

	_, err = repo.FindSessionByTokenHash(
		context.Background(),
		tokenHash,
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected pgx.ErrNoRows after delete, got %v",
			err,
		)
	}
}
