package address

import (
	"context"
	"errors"

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

// Create creates a new address for the given user.
// If isDefault is true, it clears any existing default address for this user.
// The entire operation is transactional to ensure data consistency.
func (r *Repository) Create(ctx context.Context, userID int64, input CreateInput) (Address, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Address{}, err
	}
	defer tx.Rollback(ctx)

	// If this address should be default, clear any existing default
	if input.IsDefault {
		_, err := tx.Exec(ctx, `
			UPDATE user_addresses 
			SET is_default = false, updated_at = NOW() 
			WHERE user_id = $1 AND is_default = true
		`, userID)
		if err != nil {
			return Address{}, err
		}
	}

	var addr Address
	err = tx.QueryRow(ctx, `
		INSERT INTO user_addresses (
			user_id, label, recipient_name, phone, 
			address_line1, address_line2, city, postal_code, country, is_default
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, user_id, label, recipient_name, phone, 
			address_line1, address_line2, city, postal_code, country, 
			is_default, created_at, updated_at
	`,
		userID, input.Label, input.RecipientName, input.Phone,
		input.AddressLine1, nullableString(input.AddressLine2), input.City, input.PostalCode, input.Country, input.IsDefault,
	).Scan(
		&addr.ID, &addr.UserID, &addr.Label, &addr.RecipientName, &addr.Phone,
		&addr.AddressLine1, &addr.AddressLine2, &addr.City, &addr.PostalCode, &addr.Country,
		&addr.IsDefault, &addr.CreatedAt, &addr.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Unique violation for one-default-per-user constraint
			return Address{}, ErrDuplicateDefault
		}
		return Address{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Address{}, err
	}

	return addr, nil
}

// List returns all addresses for the given user, ordered by default first, then most recent.
func (r *Repository) List(ctx context.Context, userID int64) ([]Address, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, label, recipient_name, phone, 
			address_line1, address_line2, city, postal_code, country, 
			is_default, created_at, updated_at
		FROM user_addresses
		WHERE user_id = $1
		ORDER BY is_default DESC, updated_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	addresses := make([]Address, 0)
	for rows.Next() {
		var addr Address
		if err := rows.Scan(
			&addr.ID, &addr.UserID, &addr.Label, &addr.RecipientName, &addr.Phone,
			&addr.AddressLine1, &addr.AddressLine2, &addr.City, &addr.PostalCode, &addr.Country,
			&addr.IsDefault, &addr.CreatedAt, &addr.UpdatedAt,
		); err != nil {
			return nil, err
		}
		addresses = append(addresses, addr)
	}
	return addresses, rows.Err()
}

// GetByID returns an address by ID, but only if it belongs to the given user.
// Returns ErrAddressNotFound if the address doesn't exist or doesn't belong to the user.
func (r *Repository) GetByID(ctx context.Context, userID, addressID int64) (Address, error) {
	var addr Address
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, label, recipient_name, phone, 
			address_line1, address_line2, city, postal_code, country, 
			is_default, created_at, updated_at
		FROM user_addresses
		WHERE id = $1 AND user_id = $2
	`, addressID, userID).Scan(
		&addr.ID, &addr.UserID, &addr.Label, &addr.RecipientName, &addr.Phone,
		&addr.AddressLine1, &addr.AddressLine2, &addr.City, &addr.PostalCode, &addr.Country,
		&addr.IsDefault, &addr.CreatedAt, &addr.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Address{}, ErrAddressNotFound
		}
		return Address{}, err
	}
	return addr, nil
}

// GetDefault returns the user's default address, if one exists.
func (r *Repository) GetDefault(ctx context.Context, userID int64) (Address, error) {
	var addr Address
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, label, recipient_name, phone, 
			address_line1, address_line2, city, postal_code, country, 
			is_default, created_at, updated_at
		FROM user_addresses
		WHERE user_id = $1 AND is_default = true
	`, userID).Scan(
		&addr.ID, &addr.UserID, &addr.Label, &addr.RecipientName, &addr.Phone,
		&addr.AddressLine1, &addr.AddressLine2, &addr.City, &addr.PostalCode, &addr.Country,
		&addr.IsDefault, &addr.CreatedAt, &addr.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Address{}, ErrAddressNotFound
		}
		return Address{}, err
	}
	return addr, nil
}

// Update updates an existing address.
// If isDefault is true, it clears any existing default address for this user.
// The entire operation is transactional to ensure data consistency.
func (r *Repository) Update(ctx context.Context, userID, addressID int64, input UpdateInput) (Address, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Address{}, err
	}
	defer tx.Rollback(ctx)

	// First check if the address exists and belongs to the user
	var currentAddr Address
	err = tx.QueryRow(ctx, `
		SELECT id, user_id, is_default FROM user_addresses 
		WHERE id = $1 AND user_id = $2
		FOR UPDATE
	`, addressID, userID).Scan(&currentAddr.ID, &currentAddr.UserID, &currentAddr.IsDefault)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Address{}, ErrAddressNotFound
		}
		return Address{}, err
	}

	// If this address should become default (and isn't already), clear any existing default
	if input.IsDefault && !currentAddr.IsDefault {
		_, err := tx.Exec(ctx, `
			UPDATE user_addresses 
			SET is_default = false, updated_at = NOW() 
			WHERE user_id = $1 AND is_default = true
		`, userID)
		if err != nil {
			return Address{}, err
		}
	}

	var addr Address
	err = tx.QueryRow(ctx, `
		UPDATE user_addresses 
		SET label = $1, recipient_name = $2, phone = $3, 
			address_line1 = $4, address_line2 = $5, city = $6, 
			postal_code = $7, country = $8, is_default = $9, 
			updated_at = NOW()
		WHERE id = $10 AND user_id = $11
		RETURNING id, user_id, label, recipient_name, phone, 
			address_line1, address_line2, city, postal_code, country, 
			is_default, created_at, updated_at
	`,
		input.Label, input.RecipientName, input.Phone,
		input.AddressLine1, nullableString(input.AddressLine2), input.City, input.PostalCode, input.Country, input.IsDefault,
		addressID, userID,
	).Scan(
		&addr.ID, &addr.UserID, &addr.Label, &addr.RecipientName, &addr.Phone,
		&addr.AddressLine1, &addr.AddressLine2, &addr.City, &addr.PostalCode, &addr.Country,
		&addr.IsDefault, &addr.CreatedAt, &addr.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Unique violation for one-default-per-user constraint
			return Address{}, ErrDuplicateDefault
		}
		return Address{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Address{}, err
	}

	return addr, nil
}

// Delete removes an address by ID, but only if it belongs to the given user.
// Returns ErrAddressNotFound if the address doesn't exist or doesn't belong to the user.
func (r *Repository) Delete(ctx context.Context, userID, addressID int64) error {
	result, err := r.db.Exec(ctx, `
		DELETE FROM user_addresses 
		WHERE id = $1 AND user_id = $2
	`, addressID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrAddressNotFound
	}
	return nil
}

// nullableString returns nil for an empty string and a pointer to s
// otherwise, so optional fields (address_line2) are stored as SQL NULL
// rather than an empty string when not provided.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}