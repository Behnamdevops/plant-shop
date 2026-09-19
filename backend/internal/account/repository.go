package account

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetProfile retrieves the user's profile by ID.
func (r *Repository) GetProfile(ctx context.Context, userID int64) (Profile, error) {
	var p Profile
	err := r.db.QueryRow(ctx, `
		SELECT id, name, email, phone, role, created_at, updated_at
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&p.ID, &p.Name, &p.Email, &p.Phone, &p.Role, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Profile{}, ErrProfileNotFound
		}
		return Profile{}, err
	}
	return p, nil
}

// UpdateProfile updates the user's profile (name and phone only).
// Email and role are never modified by this method.
func (r *Repository) UpdateProfile(ctx context.Context, userID int64, input UpdateInput) (Profile, error) {
	trimmed := input.Trimmed()
	
	// Convert empty phone to NULL
	var phone *string
	if trimmed.Phone != "" {
		phone = &trimmed.Phone
	}

	var p Profile
	err := r.db.QueryRow(ctx, `
		UPDATE users 
		SET name = $1, phone = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING id, name, email, phone, role, created_at, updated_at
	`, trimmed.Name, phone, userID).Scan(
		&p.ID, &p.Name, &p.Email, &p.Phone, &p.Role, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Profile{}, ErrProfileNotFound
		}
		return Profile{}, err
	}
	return p, nil
}