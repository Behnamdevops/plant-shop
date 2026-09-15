package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateUser(ctx context.Context, name, email, passwordHash string) (User, error) {
	var u User
	err := r.db.QueryRow(ctx, `
		INSERT INTO users (name, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, name, email, password_hash, created_at, updated_at
	`, name, email, passwordHash).Scan(
		&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return User{}, ErrDuplicateEmail
		}
		return User{}, err
	}
	return u, nil
}

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := r.db.QueryRow(ctx, `
		SELECT id, name, email, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1
	`, email).Scan(
		&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt,
	)
	return u, err
}

func (r *Repository) FindUserByID(ctx context.Context, id int64) (User, error) {
	var u User
	err := r.db.QueryRow(ctx, `
		SELECT id, name, email, password_hash, created_at, updated_at
		FROM users
		WHERE id = $1
	`, id).Scan(
		&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt,
	)
	return u, err
}

func (r *Repository) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) (Session, error) {
	var s Session
	err := r.db.QueryRow(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, token_hash, expires_at, created_at
	`, userID, tokenHash, expiresAt).Scan(
		&s.ID, &s.UserID, &s.TokenHash, &s.ExpiresAt, &s.CreatedAt,
	)
	return s, err
}

func (r *Repository) FindSessionByTokenHash(ctx context.Context, tokenHash string) (Session, error) {
	var s Session
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, token_hash, expires_at, created_at
		FROM sessions
		WHERE token_hash = $1
	`, tokenHash).Scan(
		&s.ID, &s.UserID, &s.TokenHash, &s.ExpiresAt, &s.CreatedAt,
	)
	return s, err
}

func (r *Repository) DeleteSessionByID(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `
		DELETE FROM sessions
		WHERE id = $1
	`, id)
	return err
}
