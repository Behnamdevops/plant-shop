package notification

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type fakeSender struct {
	sentMessages []Message
	sendError    error
}

func (s *fakeSender) Send(ctx context.Context, message Message) error {
	s.sentMessages = append(s.sentMessages, message)
	return s.sendError
}

func TestEnqueueIdempotent(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping test")
	}
	db, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Skip("cannot connect to database: ", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	// Create a test user first
	var userID int64
	err = db.QueryRow(t.Context(), `
		INSERT INTO users (name, email, password_hash)
		VALUES ($1, $2, $3)
		ON CONFLICT (email) DO NOTHING
		RETURNING id
	`, "Test User "+time.Now().Format("20060102150405"), "test"+time.Now().Format("20060102150405")+"@example.com", "hash").Scan(&userID)
	if err != nil && err.Error() != `pq: duplicate key value violates unique constraint "users_email_key"` {
		t.Fatalf("create user failed: %v", err)
	}
	if userID == 0 {
		// User already exists
		err = db.QueryRow(t.Context(), `SELECT id FROM users WHERE email LIKE 'test%example.com'`).Scan(&userID)
		if err != nil {
			t.Fatalf("get user failed: %v", err)
		}
	}

	eventKey := "test-event-key-" + time.Now().Format("20060102150405")
	recipientEmail := "test@example.com"
	eventType := "order_created"
	payload := Payload{"test": "data"}

	// First enqueue should succeed
	n1, err := repo.Enqueue(t.Context(), eventKey, userID, recipientEmail, eventType, payload)
	if err != nil {
		t.Fatalf("first enqueue failed: %v", err)
	}
	if n1 == nil {
		t.Fatal("first enqueue returned nil")
	}

	// Second enqueue with same event_key should return nil (idempotent)
	n2, err := repo.Enqueue(t.Context(), eventKey, userID, recipientEmail, eventType, payload)
	if err != nil {
		t.Fatalf("second enqueue failed: %v", err)
	}
	if n2 != nil {
		t.Fatalf("second enqueue should be idempotent, got %v", n2)
	}
}

func TestGetPendingDue(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping test")
	}
	db, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Skip("cannot connect to database: ", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	// Insert a pending notification with past next_attempt_at
	eventKey := "test-pending-" + time.Now().Format("20060102150405")
	_, err = db.Exec(t.Context(), `
		INSERT INTO notification_outbox (event_key, recipient_email, event_type, payload, status, next_attempt_at)
		VALUES ($1, $2, $3, $4, 'pending', NOW() - INTERVAL '1 minute')
	`, eventKey, "test@example.com", "order_created", Payload{"test": "data"})
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	// Get pending due should return the notification
	notifications, err := repo.GetPendingDue(t.Context())
	if err != nil {
		t.Fatalf("GetPendingDue failed: %v", err)
	}

	found := false
	for _, n := range notifications {
		if n.EventKey == eventKey {
			found = true
			if n.Status != "processing" {
				t.Errorf("expected status 'processing', got '%s'", n.Status)
			}
			break
		}
	}
	if !found {
		t.Error("expected notification to be found")
	}
}

func TestMarkSent(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping test")
	}
	db, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Skip("cannot connect to database: ", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	// Insert a notification in processing state
	eventKey := "test-sent-" + time.Now().Format("20060102150405")
	var id int64
	err = db.QueryRow(t.Context(), `
		INSERT INTO notification_outbox (event_key, recipient_email, event_type, payload, status)
		VALUES ($1, $2, $3, $4, 'processing')
		RETURNING id
	`, eventKey, "test@example.com", "order_created", Payload{}).Scan(&id)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	sentAt := time.Now()
	err = repo.MarkSent(t.Context(), id, sentAt)
	if err != nil {
		t.Fatalf("MarkSent failed: %v", err)
	}

	// Verify status changed to sent
	var status string
	err = db.QueryRow(t.Context(), `SELECT status FROM notification_outbox WHERE id = $1`, id).Scan(&status)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if status != "sent" {
		t.Errorf("expected status 'sent', got '%s'", status)
	}
}

func TestMarkFailedRetry(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping test")
	}
	db, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Skip("cannot connect to database: ", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	// Insert a notification in processing state
	eventKey := "test-failed-" + time.Now().Format("20060102150405")
	var id int64
	var attempts int
	err = db.QueryRow(t.Context(), `
		INSERT INTO notification_outbox (event_key, recipient_email, event_type, payload, status, attempts)
		VALUES ($1, $2, $3, $4, 'processing', 0)
		RETURNING id, attempts
	`, eventKey, "test@example.com", "order_created", Payload{}).Scan(&id, &attempts)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	lastError := "connection timeout"
	nextAttemptAt := time.Now().Add(1 * time.Minute)
	err = repo.MarkFailed(t.Context(), id, lastError, nextAttemptAt)
	if err != nil {
		t.Fatalf("MarkFailed failed: %v", err)
	}

	// Verify attempts incremented and status returned to pending
	var newAttempts int
	var newStatus string
	var storedLastError sql.NullString
	var storedNextAttemptAt time.Time
	err = db.QueryRow(t.Context(), `
		SELECT status, attempts, last_error, next_attempt_at
		FROM notification_outbox WHERE id = $1
	`, id).Scan(&newStatus, &newAttempts, &storedLastError, &storedNextAttemptAt)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if newStatus != "pending" {
		t.Errorf("expected status 'pending', got '%s'", newStatus)
	}
	if newAttempts != 1 {
		t.Errorf("expected attempts 1, got %d", newAttempts)
	}
	if !storedLastError.Valid || storedLastError.String != lastError {
		t.Errorf("expected last_error '%s', got '%v'", lastError, storedLastError)
	}
	if storedNextAttemptAt.Before(nextAttemptAt.Add(-time.Second)) || storedNextAttemptAt.After(nextAttemptAt.Add(time.Second)) {
		t.Errorf("next_attempt_at out of range: %v", storedNextAttemptAt)
	}
}

func TestMarkPermanentFailure(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping test")
	}
	db, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Skip("cannot connect to database: ", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	// Insert a notification in pending state
	eventKey := "test-perm-" + time.Now().Format("20060102150405")
	var id int64
	err = db.QueryRow(t.Context(), `
		INSERT INTO notification_outbox (event_key, recipient_email, event_type, payload, status)
		VALUES ($1, $2, $3, $4, 'pending')
		RETURNING id
	`, eventKey, "test@example.com", "order_created", Payload{}).Scan(&id)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	lastError := "max attempts exceeded"
	err = repo.MarkPermanentFailure(t.Context(), id, lastError)
	if err != nil {
		t.Fatalf("MarkPermanentFailure failed: %v", err)
	}

	// Verify status changed to failed
	var status string
	err = db.QueryRow(t.Context(), `SELECT status FROM notification_outbox WHERE id = $1`, id).Scan(&status)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if status != "failed" {
		t.Errorf("expected status 'failed', got '%s'", status)
	}
}

func TestEnqueueTx(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping test")
	}
	db, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Skip("cannot connect to database: ", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	// Create a test user first
	var userID int64
	err = db.QueryRow(t.Context(), `
		INSERT INTO users (name, email, password_hash)
		VALUES ($1, $2, $3)
		ON CONFLICT (email) DO NOTHING
		RETURNING id
	`, "Test User Tx "+time.Now().Format("20060102150405"), "testtx"+time.Now().Format("20060102150405")+"@example.com", "hash").Scan(&userID)
	if err != nil && err.Error() != `pq: duplicate key value violates unique constraint "users_email_key"` {
		t.Fatalf("create user failed: %v", err)
	}
	if userID == 0 {
		err = db.QueryRow(t.Context(), `SELECT id FROM users WHERE email LIKE 'testtx%example.com'`).Scan(&userID)
		if err != nil {
			t.Fatalf("get user failed: %v", err)
		}
	}

	tx, err := db.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin tx failed: %v", err)
	}
	defer tx.Rollback(t.Context())

	eventKey := "test-tx-" + time.Now().Format("20060102150405")
	err = repo.EnqueueTx(t.Context(), tx, eventKey, userID, "test@example.com", "order_created", Payload{"test": "tx"})
	if err != nil {
		t.Fatalf("EnqueueTx failed: %v", err)
	}

	// Commit the transaction so the notification is visible
	err = tx.Commit(t.Context())
	if err != nil {
		t.Fatalf("commit tx failed: %v", err)
	}

	// Verify the notification was created
	var count int
	err = db.QueryRow(t.Context(), `SELECT COUNT(*) FROM notification_outbox WHERE event_key = $1`, eventKey).Scan(&count)
	if err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 notification, got %d", count)
	}
}

func TestGetByID(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping test")
	}
	db, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Skip("cannot connect to database: ", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	// Insert a notification
	eventKey := "test-getbyid-" + time.Now().Format("20060102150405")
	var id int64
	err = db.QueryRow(t.Context(), `
		INSERT INTO notification_outbox (event_key, recipient_email, event_type, payload)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, eventKey, "test@example.com", "order_created", Payload{}).Scan(&id)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	n, err := repo.GetByID(t.Context(), id)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if n.ID != id {
		t.Errorf("expected ID %d, got %d", id, n.ID)
	}
	if n.EventKey != eventKey {
		t.Errorf("expected event_key '%s', got '%s'", eventKey, n.EventKey)
	}
}

func TestGetByEventKey(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping test")
	}
	db, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Skip("cannot connect to database: ", err)
	}
	defer db.Close()

	repo := NewRepository(db)

	eventKey := "test-getbykey-" + time.Now().Format("20060102150405")

	// First check that it doesn't exist
	n, err := repo.GetByEventKey(t.Context(), eventKey)
	if err == nil {
		t.Fatalf("expected error for non-existent event_key, got notification %v", n)
	}
	if err.Error() != "notification not found" {
		t.Fatalf("expected 'notification not found' error, got %v", err)
	}

	// Insert a notification
	var id int64
	err = db.QueryRow(t.Context(), `
		INSERT INTO notification_outbox (event_key, recipient_email, event_type, payload)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, eventKey, "test@example.com", "order_created", Payload{}).Scan(&id)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	n, err = repo.GetByEventKey(t.Context(), eventKey)
	if err != nil {
		t.Fatalf("GetByEventKey failed: %v", err)
	}
	if n.ID != id {
		t.Errorf("expected ID %d, got %d", id, n.ID)
	}
}
