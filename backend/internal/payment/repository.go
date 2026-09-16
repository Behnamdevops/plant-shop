package payment

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository persists payment_attempts rows and coordinates the small
// amount of orders-table state (payment_status, payment_method) that a
// payment attempt affects. It talks to the orders table directly (via raw
// SQL) rather than depending on the order package's Repository type, to
// avoid a circular package dependency — order and payment are siblings
// under internal/, and only main.go wires them together.
type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// orderForPayment is the minimal order snapshot needed to decide whether a
// new payment attempt may be created and, if so, for how much.
type orderForPayment struct {
	ID            int64
	UserID        int64
	Status        string
	Total         int64
	PaymentStatus string
}

// nonPayableStatuses are order statuses that can never be paid, regardless
// of payment_status: a cancelled order should never be resurrected by a
// stray payment, and a delivered order has already completed its
// lifecycle (V1 has no post-delivery payment/refund flow).
var nonPayableStatuses = map[string]bool{
	"cancelled": true,
	"delivered": true,
}

// CreateAttemptForOrder starts a new payment attempt for orderID, but only
// if the order belongs to userID, is not already paid, and is not in a
// terminal non-payable status (cancelled/delivered). The attempt amount is
// always the order's persisted total — never a client-supplied value. The
// order's payment_method is set to "zarinpal" as part of the same
// transaction, so a freshly created attempt and the order's payment_method
// change together atomically.
func (r *Repository) CreateAttemptForOrder(ctx context.Context, userID, orderID int64) (Attempt, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Attempt{}, err
	}
	defer tx.Rollback(ctx)

	var o orderForPayment
	err = tx.QueryRow(ctx, `
		SELECT id, user_id, status, total, payment_status
		FROM orders WHERE id = $1 FOR UPDATE
	`, orderID).Scan(&o.ID, &o.UserID, &o.Status, &o.Total, &o.PaymentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Attempt{}, ErrOrderNotFound
		}
		return Attempt{}, err
	}
	if o.UserID != userID {
		return Attempt{}, ErrOrderNotFound
	}
	if o.PaymentStatus == "paid" {
		return Attempt{}, ErrOrderAlreadyPaid
	}
	if nonPayableStatuses[o.Status] {
		return Attempt{}, ErrOrderNotPayable
	}

	var a Attempt
	err = tx.QueryRow(ctx, `
		INSERT INTO payment_attempts (order_id, provider, amount, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, order_id, provider, authority, amount, status, ref_id, provider_code, created_at, updated_at, verified_at
	`, orderID, ProviderZarinPal, o.Total, StatusPending).Scan(
		&a.ID, &a.OrderID, &a.Provider, &a.Authority, &a.Amount, &a.Status,
		&a.RefID, &a.ProviderCode, &a.CreatedAt, &a.UpdatedAt, &a.VerifiedAt,
	)
	if err != nil {
		return Attempt{}, err
	}

	// Record that this order now has a ZarinPal attempt in flight. This is
	// set regardless of whether the upcoming request.json call actually
	// succeeds, since a retried attempt on the same order is still a
	// ZarinPal order from the customer's perspective; payment_status stays
	// "pending" either way.
	_, err = tx.Exec(ctx, `UPDATE orders SET payment_method = $1, updated_at = NOW() WHERE id = $2`, "zarinpal", orderID)
	if err != nil {
		return Attempt{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Attempt{}, err
	}
	return a, nil
}

// SetAuthority persists the authority ZarinPal returned for attemptID once
// the request.json call has succeeded. Left unset (NULL) if the request
// call fails, so a failed attempt never has a usable authority.
func (r *Repository) SetAuthority(ctx context.Context, attemptID int64, authority string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE payment_attempts SET authority = $1, updated_at = NOW() WHERE id = $2
	`, authority, attemptID)
	return err
}

// MarkAttemptFailed marks attemptID as failed. providerCode is stored when
// available (e.g. a rejection code from ZarinPal); pass nil when the
// failure was purely local (e.g. a network error before any response was
// received).
func (r *Repository) MarkAttemptFailed(ctx context.Context, attemptID int64, providerCode *int) error {
	_, err := r.db.Exec(ctx, `
		UPDATE payment_attempts
		SET status = $1, provider_code = $2, updated_at = NOW()
		WHERE id = $3
	`, StatusFailed, providerCode, attemptID)
	return err
}

// attemptByAuthorityForUpdate loads and locks (FOR UPDATE) the attempt
// matching authority, inside tx. Returns ErrAttemptNotFound if no such
// attempt exists — this is the only lookup path the public callback
// endpoint uses, so an authority can never be used to reach any order
// other than the one it was actually issued for.
func attemptByAuthorityForUpdate(ctx context.Context, tx pgx.Tx, authority string) (Attempt, error) {
	var a Attempt
	err := tx.QueryRow(ctx, `
		SELECT id, order_id, provider, authority, amount, status, ref_id, provider_code, created_at, updated_at, verified_at
		FROM payment_attempts
		WHERE authority = $1
		FOR UPDATE
	`, authority).Scan(
		&a.ID, &a.OrderID, &a.Provider, &a.Authority, &a.Amount, &a.Status,
		&a.RefID, &a.ProviderCode, &a.CreatedAt, &a.UpdatedAt, &a.VerifiedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Attempt{}, ErrAttemptNotFound
		}
		return Attempt{}, err
	}
	return a, nil
}

// GetAttemptByAuthority returns the attempt matching authority without
// taking any lock, for read-only lookups (e.g. building a result page).
func (r *Repository) GetAttemptByAuthority(ctx context.Context, authority string) (Attempt, error) {
	var a Attempt
	err := r.db.QueryRow(ctx, `
		SELECT id, order_id, provider, authority, amount, status, ref_id, provider_code, created_at, updated_at, verified_at
		FROM payment_attempts
		WHERE authority = $1
	`, authority).Scan(
		&a.ID, &a.OrderID, &a.Provider, &a.Authority, &a.Amount, &a.Status,
		&a.RefID, &a.ProviderCode, &a.CreatedAt, &a.UpdatedAt, &a.VerifiedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Attempt{}, ErrAttemptNotFound
		}
		return Attempt{}, err
	}
	return a, nil
}

// VerifyResult is what the caller (handler) learns after attempting to
// finalize a callback: which order was affected, and whether this call is
// the one that actually transitioned it to paid (as opposed to a duplicate
// callback that found the attempt already settled).
type VerifyResult struct {
	OrderID        int64
	AlreadySettled bool
	FinalStatus    string // StatusPaid or StatusFailed
}

// FinalizeVerifiedPayment records a successful (or already-verified)
// ZarinPal verification for the attempt matching authority, and — only the
// first time this happens for a given attempt — marks the parent order as
// paid. Everything happens inside one transaction, with the attempt row
// locked FOR UPDATE, so concurrent/duplicate callbacks for the same
// authority serialize against each other: the second caller sees the
// attempt already in status "paid" and returns AlreadySettled = true
// without re-touching the order or ref_id.
func (r *Repository) FinalizeVerifiedPayment(ctx context.Context, authority string, refID int64, providerCode int) (VerifyResult, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return VerifyResult{}, err
	}
	defer tx.Rollback(ctx)

	a, err := attemptByAuthorityForUpdate(ctx, tx, authority)
	if err != nil {
		return VerifyResult{}, err
	}

	if a.Status == StatusPaid {
		return VerifyResult{OrderID: a.OrderID, AlreadySettled: true, FinalStatus: StatusPaid}, tx.Commit(ctx)
	}
	if a.Status == StatusFailed {
		// The attempt was already conclusively marked failed (e.g. a
		// previous verify call rejected it); do not resurrect it into paid
		// on a later, inconsistent callback.
		return VerifyResult{OrderID: a.OrderID, AlreadySettled: true, FinalStatus: StatusFailed}, tx.Commit(ctx)
	}

	_, err = tx.Exec(ctx, `
		UPDATE payment_attempts
		SET status = $1, ref_id = $2, provider_code = $3, verified_at = NOW(), updated_at = NOW()
		WHERE id = $4
	`, StatusPaid, refID, providerCode, a.ID)
	if err != nil {
		return VerifyResult{}, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE orders
		SET payment_status = 'paid', payment_method = 'zarinpal', updated_at = NOW()
		WHERE id = $1
	`, a.OrderID)
	if err != nil {
		return VerifyResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return VerifyResult{}, err
	}
	return VerifyResult{OrderID: a.OrderID, AlreadySettled: false, FinalStatus: StatusPaid}, nil
}

// ListAttemptsForOrder returns every payment attempt for orderID, newest
// first, regardless of which user placed the order. Intended for admin use
// only — callers must enforce admin authorization before calling this (see
// payment.Handler.AdminListAttempts).
func (r *Repository) ListAttemptsForOrder(ctx context.Context, orderID int64) ([]Attempt, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, order_id, provider, authority, amount, status, ref_id, provider_code, created_at, updated_at, verified_at
		FROM payment_attempts
		WHERE order_id = $1
		ORDER BY id DESC
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	attempts := make([]Attempt, 0)
	for rows.Next() {
		var a Attempt
		if err := rows.Scan(
			&a.ID, &a.OrderID, &a.Provider, &a.Authority, &a.Amount, &a.Status,
			&a.RefID, &a.ProviderCode, &a.CreatedAt, &a.UpdatedAt, &a.VerifiedAt,
		); err != nil {
			return nil, err
		}
		attempts = append(attempts, a)
	}
	return attempts, rows.Err()
}

// FinalizeFailedPayment records a failed verification (or a callback that
// reported Status != OK) for the attempt matching authority. Idempotent in
// the same way as FinalizeVerifiedPayment: an attempt already in a
// terminal status is left untouched and reported as AlreadySettled. Never
// changes orders.payment_status — an order that failed one attempt can
// still be retried with a new attempt.
func (r *Repository) FinalizeFailedPayment(ctx context.Context, authority string, providerCode *int) (VerifyResult, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return VerifyResult{}, err
	}
	defer tx.Rollback(ctx)

	a, err := attemptByAuthorityForUpdate(ctx, tx, authority)
	if err != nil {
		return VerifyResult{}, err
	}

	if a.Status == StatusPaid || a.Status == StatusFailed {
		return VerifyResult{OrderID: a.OrderID, AlreadySettled: true, FinalStatus: a.Status}, tx.Commit(ctx)
	}

	_, err = tx.Exec(ctx, `
		UPDATE payment_attempts
		SET status = $1, provider_code = $2, updated_at = NOW()
		WHERE id = $3
	`, StatusFailed, providerCode, a.ID)
	if err != nil {
		return VerifyResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return VerifyResult{}, err
	}
	return VerifyResult{OrderID: a.OrderID, AlreadySettled: false, FinalStatus: StatusFailed}, nil
}
