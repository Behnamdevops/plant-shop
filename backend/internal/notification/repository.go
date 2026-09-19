package notification

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// EnqueueTx enqueues a notification event within a transaction with idempotency via event_key.
func (r *Repository) EnqueueTx(ctx context.Context, tx pgx.Tx, eventKey string, userID int64, recipientEmail, eventType string, payload Payload) error {
	payloadJSON, err := payload.MarshalJSON()
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO notification_outbox (event_key, user_id, recipient_email, event_type, payload)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (event_key) DO NOTHING
	`, eventKey, sql.NullInt64{Int64: userID, Valid: userID != 0}, recipientEmail, eventType, payloadJSON)

	return err
}

// Enqueue enqueues a notification event with idempotency via event_key.
// If an event with the same event_key already exists, it returns (nil, nil) to indicate
// the event was not duplicated.
func (r *Repository) Enqueue(ctx context.Context, eventKey string, userID int64, recipientEmail, eventType string, payload Payload) (*NotificationOutbox, error) {
	payloadJSON, err := payload.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var row pgx.Row
	if userID == 0 {
		row = r.db.QueryRow(ctx, `
			INSERT INTO notification_outbox (event_key, recipient_email, event_type, payload)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (event_key) DO NOTHING
			RETURNING id, event_key, user_id, recipient_email, event_type, payload, status, attempts, next_attempt_at, last_error, sent_at, created_at, updated_at
		`, eventKey, recipientEmail, eventType, sql.NullString{String: string(payloadJSON), Valid: true})
	} else {
		row = r.db.QueryRow(ctx, `
			INSERT INTO notification_outbox (event_key, user_id, recipient_email, event_type, payload)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (event_key) DO NOTHING
			RETURNING id, event_key, user_id, recipient_email, event_type, payload, status, attempts, next_attempt_at, last_error, sent_at, created_at, updated_at
		`, eventKey, sql.NullInt64{Int64: userID, Valid: true}, recipientEmail, eventType, sql.NullString{String: string(payloadJSON), Valid: true})
	}

	var n NotificationOutbox
	var payloadStr, lastError, sentAt sql.NullString
	err = row.Scan(
		&n.ID, &n.EventKey, &n.UserID, &n.RecipientEmail, &n.EventType, &payloadStr,
		&n.Status, &n.Attempts, &n.NextAttemptAt, &lastError, &sentAt,
		&n.CreatedAt, &n.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Event with this key already exists - idempotent
			return nil, nil
		}
		return nil, err
	}

	if payloadStr.Valid {
		if err := n.Payload.UnmarshalJSON([]byte(payloadStr.String)); err != nil {
			return nil, err
		}
	}

	// Set LastError and SentAt from null strings
	if lastError.Valid {
		n.LastError = &lastError.String
	}
	if sentAt.Valid && sentAt.String != "" {
		t, _ := time.Parse(time.RFC3339, sentAt.String)
		n.SentAt = &t
	}

	return &n, nil
}

// GetPendingDue returns pending notifications that are due for processing.
// Uses SELECT ... FOR UPDATE SKIP LOCKED for concurrency safety.
func (r *Repository) GetPendingDue(ctx context.Context) ([]NotificationOutbox, error) {
	rows, err := r.db.Query(ctx, `
		UPDATE notification_outbox
		SET status = 'processing', updated_at = NOW()
		WHERE id IN (
			SELECT id FROM notification_outbox
			WHERE status = 'pending' AND next_attempt_at <= NOW()
			ORDER BY next_attempt_at ASC
			LIMIT 50
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, event_key, user_id, recipient_email, event_type, payload, status, attempts, next_attempt_at, last_error, sent_at, created_at, updated_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []NotificationOutbox
	for rows.Next() {
		var n NotificationOutbox
		var payloadStr sql.NullString
		var lastError, sentAt sql.NullString

		err := rows.Scan(
			&n.ID, &n.EventKey, &n.UserID, &n.RecipientEmail, &n.EventType, &payloadStr,
			&n.Status, &n.Attempts, &n.NextAttemptAt, &lastError, &sentAt,
			&n.CreatedAt, &n.UpdatedAt,
		)
		if err != nil {
			rows.Close()
			return nil, err
		}

		if payloadStr.Valid {
			if err := n.Payload.UnmarshalJSON([]byte(payloadStr.String)); err != nil {
				rows.Close()
				return nil, err
			}
		}

		if lastError.Valid {
			n.LastError = &lastError.String
		}
		if sentAt.Valid && sentAt.String != "" {
			t, _ := time.Parse(time.RFC3339, sentAt.String)
			n.SentAt = &t
		}

		notifications = append(notifications, n)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return notifications, nil
}

// MarkSent marks a notification as sent with the given sent_at time.
func (r *Repository) MarkSent(ctx context.Context, id int64, sentAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE notification_outbox
		SET status = 'sent', sent_at = $1, updated_at = NOW()
		WHERE id = $2 AND status = 'processing'
	`, sentAt, id)
	return err
}

// MarkFailed marks a notification as failed with the given error message.
// It also schedules the next retry based on the attempt count.
func (r *Repository) MarkFailed(ctx context.Context, id int64, lastError string, nextAttemptAt time.Time) error {
	result, err := r.db.Exec(ctx, `
		UPDATE notification_outbox
		SET status = 'pending', last_error = $1, next_attempt_at = $2, attempts = attempts + 1, updated_at = NOW()
		WHERE id = $3 AND status = 'processing'
	`, lastError, nextAttemptAt, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("notification not found or not in processing state")
	}

	return nil
}

// MarkPermanentFailure marks a notification as permanently failed (max attempts exceeded).
func (r *Repository) MarkPermanentFailure(ctx context.Context, id int64, lastError string) error {
	result, err := r.db.Exec(ctx, `
		UPDATE notification_outbox
		SET status = 'failed', last_error = $1, updated_at = NOW()
		WHERE id = $2 AND status = 'pending'
	`, lastError, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("notification not found or not in pending state")
	}

	return nil
}

// GetByID returns a notification by its ID.
func (r *Repository) GetByID(ctx context.Context, id int64) (*NotificationOutbox, error) {
	var n NotificationOutbox
	var payloadStr sql.NullString
	var lastError, sentAt sql.NullString

	row := r.db.QueryRow(ctx, `
		SELECT id, event_key, user_id, recipient_email, event_type, payload, status, attempts, next_attempt_at, last_error, sent_at, created_at, updated_at
		FROM notification_outbox WHERE id = $1
	`, id)

	err := row.Scan(
		&n.ID, &n.EventKey, &n.UserID, &n.RecipientEmail, &n.EventType, &payloadStr,
		&n.Status, &n.Attempts, &n.NextAttemptAt, &lastError, &sentAt,
		&n.CreatedAt, &n.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("notification not found")
		}
		return nil, err
	}

	if payloadStr.Valid {
		if err := n.Payload.UnmarshalJSON([]byte(payloadStr.String)); err != nil {
			return nil, err
		}
	}

	if lastError.Valid {
		n.LastError = &lastError.String
	}
	if sentAt.Valid && sentAt.String != "" {
		t, _ := time.Parse(time.RFC3339, sentAt.String)
		n.SentAt = &t
	}

	return &n, nil
}

// GetByEventKey returns a notification by its event_key.
func (r *Repository) GetByEventKey(ctx context.Context, eventKey string) (*NotificationOutbox, error) {
	var n NotificationOutbox
	var payloadStr sql.NullString
	var lastError, sentAt sql.NullString

	row := r.db.QueryRow(ctx, `
		SELECT id, event_key, user_id, recipient_email, event_type, payload, status, attempts, next_attempt_at, last_error, sent_at, created_at, updated_at
		FROM notification_outbox WHERE event_key = $1
	`, eventKey)

	err := row.Scan(
		&n.ID, &n.EventKey, &n.UserID, &n.RecipientEmail, &n.EventType, &payloadStr,
		&n.Status, &n.Attempts, &n.NextAttemptAt, &lastError, &sentAt,
		&n.CreatedAt, &n.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("notification not found")
		}
		return nil, err
	}

	if payloadStr.Valid {
		if err := n.Payload.UnmarshalJSON([]byte(payloadStr.String)); err != nil {
			return nil, err
		}
	}

	if lastError.Valid {
		n.LastError = &lastError.String
	}
	if sentAt.Valid && sentAt.String != "" {
		t, _ := time.Parse(time.RFC3339, sentAt.String)
		n.SentAt = &t
	}

	return &n, nil
}
