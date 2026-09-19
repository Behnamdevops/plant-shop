package payment

import (
	"context"
	"errors"

	"github.com/Behnamdevops/plant-shop/backend/internal/notification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db           *pgxpool.Pool
	environment  string
	identity     string
	notifications *notification.Repository
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db, environment: "test", identity: "test"}
}

func (r *Repository) ConfigureProvider(environment, accountKey string) {
	r.environment = environment
	r.identity = accountKey
}

// WithNotifications sets the notification repository for the payment repository.
func (r *Repository) WithNotifications(notifRepo *notification.Repository) *Repository {
	r.notifications = notifRepo
	return r
}

type orderForPayment struct {
	ID            int64
	UserID        int64
	Status        string
	Total         int64
	Currency      string
	PaymentStatus string
}

const attemptColumns = `id, order_id, provider, currency, environment, account_key, authority, amount, status, ref_id, provider_code, created_at, updated_at, verified_at`

func scanAttempt(row pgx.Row) (Attempt, error) {
	var a Attempt
	err := row.Scan(
		&a.ID, &a.OrderID, &a.Provider, &a.Currency, &a.Environment, &a.AccountKey,
		&a.Authority, &a.Amount, &a.Status, &a.RefID, &a.ProviderCode,
		&a.CreatedAt, &a.UpdatedAt, &a.VerifiedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Attempt{}, ErrAttemptNotFound
	}
	return a, err
}

func lockOrderForPayment(ctx context.Context, tx pgx.Tx, orderID int64) (orderForPayment, error) {
	var o orderForPayment
	err := tx.QueryRow(ctx, `
		SELECT id, user_id, status, total, currency, payment_status
		FROM orders WHERE id = $1 FOR UPDATE
	`, orderID).Scan(&o.ID, &o.UserID, &o.Status, &o.Total, &o.Currency, &o.PaymentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return o, ErrOrderNotFound
	}
	return o, err
}

func lockPaymentAttempt(ctx context.Context, tx pgx.Tx, attemptID int64) (orderForPayment, Attempt, error) {
	var orderID int64
	err := tx.QueryRow(ctx, `SELECT order_id FROM payment_attempts WHERE id = $1`, attemptID).Scan(&orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return orderForPayment{}, Attempt{}, ErrAttemptNotFound
	}
	if err != nil {
		return orderForPayment{}, Attempt{}, err
	}
	o, err := lockOrderForPayment(ctx, tx, orderID)
	if err != nil {
		return o, Attempt{}, err
	}
	a, err := scanAttempt(tx.QueryRow(ctx, `SELECT `+attemptColumns+` FROM payment_attempts WHERE id = $1 AND order_id = $2 FOR UPDATE`, attemptID, orderID))
	return o, a, err
}

func lockPaymentAuthority(ctx context.Context, tx pgx.Tx, authority string) (orderForPayment, Attempt, error) {
	var attemptID int64
	err := tx.QueryRow(ctx, `SELECT id FROM payment_attempts WHERE authority = $1`, authority).Scan(&attemptID)
	if errors.Is(err, pgx.ErrNoRows) {
		return orderForPayment{}, Attempt{}, ErrAttemptNotFound
	}
	if err != nil {
		return orderForPayment{}, Attempt{}, err
	}
	return lockPaymentAttempt(ctx, tx, attemptID)
}

func (r *Repository) matchesProvider(a Attempt) bool {
	if a.Provider != ProviderZarinPal || a.Environment != r.environment || a.Environment == "legacy" || a.Environment == "" {
		return false
	}
	if r.environment == "test" && r.identity == "test" && a.AccountKey == "" {
		return true
	}
	return r.identity != "" && a.AccountKey == r.identity
}

func (r *Repository) CreateAttemptForOrder(ctx context.Context, userID, orderID int64) (Attempt, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Attempt{}, err
	}
	defer tx.Rollback(ctx)

	o, err := lockOrderForPayment(ctx, tx, orderID)
	if err != nil {
		return Attempt{}, err
	}
	if o.UserID != userID {
		return Attempt{}, ErrOrderNotFound
	}
	if o.PaymentStatus == "paid" {
		return Attempt{}, ErrOrderAlreadyPaid
	}
	if o.Status != "pending" || o.PaymentStatus != "pending" && o.PaymentStatus != "failed" || o.Total < 10000 || o.Currency != "IRR" {
		return Attempt{}, ErrOrderNotPayable
	}
	if !r.matchesProvider(Attempt{Provider: ProviderZarinPal, Environment: r.environment, AccountKey: r.identity}) {
		return Attempt{}, ErrProviderMismatch
	}
	var active bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM payment_attempts WHERE order_id = $1
			AND (status IN ('pending', 'paid', 'reconciliation') OR authority IS NOT NULL)
		)
	`, orderID).Scan(&active)
	if err != nil {
		return Attempt{}, err
	}
	if active {
		return Attempt{}, ErrPaymentInProgress
	}
	a, err := scanAttempt(tx.QueryRow(ctx, `
		INSERT INTO payment_attempts (order_id, provider, amount, currency, environment, account_key, status)
		VALUES ($1, $2, $3, 'IRR', $4, $5, 'pending')
		RETURNING `+attemptColumns, orderID, ProviderZarinPal, o.Total, r.environment, r.identity))
	if err != nil {
		return Attempt{}, err
	}
	_, err = tx.Exec(ctx, `UPDATE orders SET payment_method = 'zarinpal', updated_at = NOW() WHERE id = $1`, orderID)
	if err != nil {
		return Attempt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Attempt{}, err
	}
	return a, nil
}

func (r *Repository) SetAuthority(ctx context.Context, attemptID int64, authority string) error {
	if len(authority) > 64 || !validAuthority(authority) {
		return ErrInvalidAuthority
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, a, err := lockPaymentAttempt(ctx, tx, attemptID)
	if err != nil {
		return err
	}
	if !r.matchesProvider(a) {
		return ErrProviderMismatch
	}
	tag, err := tx.Exec(ctx, `
		UPDATE payment_attempts SET authority = $1, updated_at = NOW()
		WHERE id = $2 AND status = 'pending' AND (authority IS NULL OR authority = $1)
	`, authority, attemptID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrAlreadyProcessed
	}
	return tx.Commit(ctx)
}

func (r *Repository) MarkAttemptFailed(ctx context.Context, attemptID int64, providerCode *int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, a, err := lockPaymentAttempt(ctx, tx, attemptID)
	if err != nil {
		return err
	}
	if !r.matchesProvider(a) {
		return ErrProviderMismatch
	}
	if a.Status != StatusPending || a.Authority != nil {
		return ErrAlreadyProcessed
	}
	tag, err := tx.Exec(ctx, `
		UPDATE payment_attempts SET status = 'failed', provider_code = $1, updated_at = NOW()
		WHERE id = $2 AND status = 'pending' AND authority IS NULL
	`, providerCode, attemptID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrAlreadyProcessed
	}
	return tx.Commit(ctx)
}

func (r *Repository) GetAttemptByAuthority(ctx context.Context, authority string) (Attempt, error) {
	return scanAttempt(r.db.QueryRow(ctx, `SELECT `+attemptColumns+` FROM payment_attempts WHERE authority = $1`, authority))
}

type VerifyResult struct {
	OrderID        int64
	AlreadySettled bool
	FinalStatus    string
}

// EnqueuePaymentSucceeded enqueues a payment_succeeded notification for the given order.
func (r *Repository) EnqueuePaymentSucceeded(ctx context.Context, orderID int64) error {
	if r.notifications == nil {
		return nil
	}

	var email string
	err := r.db.QueryRow(ctx, `
		SELECT u.email FROM orders o JOIN users u ON u.id = o.user_id WHERE o.id = $1
	`, orderID).Scan(&email)
	if err != nil {
		return err
	}

	var total int64
	err = r.db.QueryRow(ctx, `SELECT total FROM orders WHERE id = $1`, orderID).Scan(&total)
	if err != nil {
		return err
	}

	payload := notification.PaymentSucceededPayload(orderID, "مشتری", email, total)

	// Get a transaction for the notification enqueue
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO notification_outbox (event_key, user_id, recipient_email, event_type, payload)
		SELECT $1, o.user_id, u.email, $2, $3
		FROM orders o
		JOIN users u ON u.id = o.user_id
		WHERE o.id = $4
		ON CONFLICT (event_key) DO NOTHING
	`, notification.EventKey(orderID, "payment_succeeded"), notification.EventTypePaymentSucceeded, payload)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// EnqueuePaymentFailed enqueues a payment_failed notification for the given order.
func (r *Repository) EnqueuePaymentFailed(ctx context.Context, orderID int64) error {
	if r.notifications == nil {
		return nil
	}

	var email string
	err := r.db.QueryRow(ctx, `
		SELECT u.email FROM orders o JOIN users u ON u.id = o.user_id WHERE o.id = $1
	`, orderID).Scan(&email)
	if err != nil {
		return err
	}

	var total int64
	err = r.db.QueryRow(ctx, `SELECT total FROM orders WHERE id = $1`, orderID).Scan(&total)
	if err != nil {
		return err
	}

	payload := notification.PaymentFailedPayload(orderID, "مشتری", email, total)

	// Get a transaction for the notification enqueue
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO notification_outbox (event_key, user_id, recipient_email, event_type, payload)
		SELECT $1, o.user_id, u.email, $2, $3
		FROM orders o
		JOIN users u ON u.id = o.user_id
		WHERE o.id = $4
		ON CONFLICT (event_key) DO NOTHING
	`, notification.EventKey(orderID, "payment_failed"), notification.EventTypePaymentFailed, payload)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *Repository) FinalizeVerifiedPayment(ctx context.Context, authority string, refID int64, providerCode int) (VerifyResult, error) {
	if refID <= 0 || providerCode != 100 && providerCode != 101 {
		return VerifyResult{}, ErrInvalidVerification
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return VerifyResult{}, err
	}
	defer tx.Rollback(ctx)
	o, a, err := lockPaymentAuthority(ctx, tx, authority)
	if err != nil {
		return VerifyResult{}, err
	}
	if !r.matchesProvider(a) {
		return VerifyResult{}, ErrProviderMismatch
	}
	if a.Status == StatusPaid || a.Status == StatusReconciliation {
		return VerifyResult{OrderID: a.OrderID, AlreadySettled: true, FinalStatus: a.Status}, tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended(jsonb_build_array($1::text, $2::text, $3::text, $4::bigint)::text, 0))
	`, a.Provider, a.Environment, a.AccountKey, refID)
	if err != nil {
		return VerifyResult{}, err
	}
	var conflictingPayment bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM payment_attempts WHERE status = 'paid' AND id <> $1
			AND (order_id = $2 OR (provider = $3 AND environment = $4 AND account_key = $5 AND ref_id = $6))
		)
	`, a.ID, a.OrderID, a.Provider, a.Environment, a.AccountKey, refID).Scan(&conflictingPayment)
	if err != nil {
		return VerifyResult{}, err
	}
	status := StatusPaid
	if o.Status != "pending" && o.Status != "processing" && o.Status != "shipped" ||
		o.PaymentStatus != "pending" && o.PaymentStatus != "failed" ||
		o.Total != a.Amount || a.Amount < 10000 || o.Currency != a.Currency || a.Currency != "IRR" || conflictingPayment {
		status = StatusReconciliation
	}
	tag, err := tx.Exec(ctx, `
		UPDATE payment_attempts
		SET status = $1, ref_id = $2, provider_code = $3, verified_at = NOW(), updated_at = NOW()
		WHERE id = $4 AND status IN ('pending', 'failed') AND authority IS NOT NULL
	`, status, refID, providerCode, a.ID)
	if err != nil {
		return VerifyResult{}, err
	}
	if tag.RowsAffected() != 1 {
		return VerifyResult{}, ErrAlreadyProcessed
	}
	if status == StatusPaid {
		tag, err = tx.Exec(ctx, `
			UPDATE orders SET payment_status = 'paid', payment_method = 'zarinpal', updated_at = NOW()
			WHERE id = $1 AND payment_status IN ('pending', 'failed')
		`, a.OrderID)
		if err != nil {
			return VerifyResult{}, err
		}
		if tag.RowsAffected() != 1 {
			return VerifyResult{}, ErrOrderNotPayable
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return VerifyResult{}, err
	}

	// Enqueue payment_succeeded notification after successful settlement
	if status == StatusPaid && r.notifications != nil {
		_ = r.EnqueuePaymentSucceeded(ctx, a.OrderID)
	}

	return VerifyResult{OrderID: a.OrderID, FinalStatus: status}, nil
}

func (r *Repository) reconciliationRows(ctx context.Context, attemptID, beforeID int64, limit int) ([]Reconciliation, error) {
	rows, err := r.db.Query(ctx, `
		SELECT a.id, a.order_id, a.provider, a.currency, a.environment, a.account_key,
		       a.authority, a.amount, a.status, a.ref_id, a.provider_code,
		       a.created_at, a.updated_at, a.verified_at, o.status, o.payment_status,
		       COALESCE(e.event_type, ''), e.created_at
		FROM payment_attempts a JOIN orders o ON o.id = a.order_id
		LEFT JOIN LATERAL (
			SELECT event_type, created_at FROM payment_attempt_events
			WHERE attempt_id = a.id AND event_type IN ('paid', 'reconciliation', 'verify_rejected', 'verify_uncertain')
			ORDER BY id DESC LIMIT 1
		) e ON true
		WHERE ($1::bigint = 0 OR a.id = $1)
		  AND ($2::bigint = 0 OR a.id < $2)
		  AND ($1::bigint <> 0 OR a.status IN ('pending', 'reconciliation') OR (a.status = 'failed' AND a.authority IS NOT NULL))
		ORDER BY a.id DESC LIMIT $3
	`, attemptID, beforeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Reconciliation, 0)
	for rows.Next() {
		var item Reconciliation
		a := &item.Attempt
		var event string
		if err := rows.Scan(&a.ID, &a.OrderID, &a.Provider, &a.Currency, &a.Environment, &a.AccountKey,
			&a.Authority, &a.Amount, &a.Status, &a.RefID, &a.ProviderCode,
			&a.CreatedAt, &a.UpdatedAt, &a.VerifiedAt, &item.OrderStatus, &item.PaymentStatus,
			&event, &item.LastCheckedAt); err != nil {
			return nil, err
		}
		switch event {
		case "paid":
			item.LastOutcome = "verified_success"
		case "reconciliation":
			item.LastOutcome = "manual_required"
		case "verify_rejected":
			item.LastOutcome = "definitive_rejection"
		case "verify_uncertain":
			item.LastOutcome = "uncertain"
		}
		item.ReconciliationRequired = a.Status != StatusPaid
		item.Reason = "awaiting_verification"
		switch {
		case a.Status == StatusPaid:
			item.Reason = "settled"
		case a.Status == StatusReconciliation:
			item.Reason = "manual_reconciliation_required"
			if item.OrderStatus == "cancelled" || item.PaymentStatus == "paid" || item.PaymentStatus == "refunded" {
				item.Reason = "refund_required"
			}
		case !r.matchesProvider(*a):
			item.Reason = "provider_binding_mismatch"
		case a.Authority == nil:
			item.Reason = "missing_authority"
		case a.Currency != "IRR" || a.Amount < 10000 || len(*a.Authority) > 64 || !validAuthority(*a.Authority):
			item.Reason = "invalid_binding"
		default:
			item.Retryable = true
			if event == "verify_rejected" {
				item.Reason = "verification_rejected"
			} else if event == "verify_uncertain" {
				item.Reason = "verification_uncertain"
			}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetReconciliation(ctx context.Context, attemptID int64) (Reconciliation, error) {
	items, err := r.reconciliationRows(ctx, attemptID, 0, 1)
	if err != nil {
		return Reconciliation{}, err
	}
	if len(items) == 0 {
		return Reconciliation{}, ErrAttemptNotFound
	}
	return items[0], nil
}

func (r *Repository) ListAttemptsForOrder(ctx context.Context, orderID int64) ([]Attempt, error) {
	rows, err := r.db.Query(ctx, `SELECT `+attemptColumns+` FROM payment_attempts WHERE order_id = $1 ORDER BY id DESC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	attempts := make([]Attempt, 0)
	for rows.Next() {
		a, err := scanAttempt(rows)
		if err != nil {
			return nil, err
		}
		attempts = append(attempts, a)
	}
	return attempts, rows.Err()
}

func (r *Repository) FinalizeFailedPayment(ctx context.Context, authority string, providerCode *int) (VerifyResult, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return VerifyResult{}, err
	}
	defer tx.Rollback(ctx)
	_, a, err := lockPaymentAuthority(ctx, tx, authority)
	if err != nil {
		return VerifyResult{}, err
	}
	if !r.matchesProvider(a) {
		return VerifyResult{}, ErrProviderMismatch
	}
	if a.Status == StatusPaid || a.Status == StatusReconciliation {
		return VerifyResult{OrderID: a.OrderID, AlreadySettled: true, FinalStatus: a.Status}, tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `
		UPDATE payment_attempts SET provider_code = COALESCE($1, provider_code), updated_at = NOW()
		WHERE id = $2
	`, providerCode, a.ID)
	if err != nil {
		return VerifyResult{}, err
	}
	event := "verify_rejected"
	if providerCode == nil {
		event = "verify_uncertain"
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO payment_attempt_events (attempt_id, event_type, snapshot)
		SELECT id, $2, payment_attempt_event_snapshot(payment_attempts)
		FROM payment_attempts WHERE id = $1
	`, a.ID, event)
	if err != nil {
		return VerifyResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return VerifyResult{}, err
	}

	// Enqueue payment_failed notification after authoritative failure
	// Only enqueue if the payment is in a terminal failed state (not reconciliation)
	if a.Status == StatusFailed && r.notifications != nil {
		_ = r.EnqueuePaymentFailed(ctx, a.OrderID)
	}

	return VerifyResult{OrderID: a.OrderID, FinalStatus: a.Status}, nil
}
