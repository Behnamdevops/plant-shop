package returnpkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Behnamdevops/plant-shop/backend/internal/notification"
	"github.com/Behnamdevops/plant-shop/backend/internal/order"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db            *pgxpool.Pool
	orderRepo     *order.Repository
	notifications *notification.Repository
}

func NewRepository(db *pgxpool.Pool, orderRepo *order.Repository) *Repository {
	return &Repository{db: db, orderRepo: orderRepo}
}

// WithNotifications sets the notification repository for return requests.
func (r *Repository) WithNotifications(notifRepo *notification.Repository) *Repository {
	r.notifications = notifRepo
	return r
}

// CheckEligibility verifies whether an order is eligible for a return request.
// It checks:
// - Order exists and belongs to user
// - Order status is delivered
// - Payment status is paid
// - No existing return request
func (r *Repository) CheckEligibility(ctx context.Context, orderID, userID int64) error {
	var status, paymentStatus string
	var hasReturnRequest bool

	// Check order status and payment status
	err := r.db.QueryRow(ctx, `
		SELECT o.status, o.payment_status,
			   EXISTS (
					SELECT 1 FROM return_requests WHERE order_id = $1
			   )
		FROM orders o
		WHERE o.id = $1 AND o.user_id = $2
	`, orderID, userID).Scan(&status, &paymentStatus, &hasReturnRequest)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReturnRequestNotFound
		}
		return err
	}

	// Order must be delivered
	if status != order.StatusDelivered {
		return fmt.Errorf("%w: order must be delivered (current status: %s)", ErrReturnNotEligible, status)
	}

	// Payment must be paid (not pending, failed, or refunded)
	if paymentStatus != order.PaymentStatusPaid {
		return fmt.Errorf("%w: payment must be completed (current status: %s)", ErrReturnNotEligible, paymentStatus)
	}

	// No existing return request
	if hasReturnRequest {
		return ErrReturnAlreadyExists
	}

	return nil
}

// CreateRequest creates a new return request for the given order.
// It validates eligibility first and returns an error if the order is not eligible.
func (r *Repository) CreateRequest(ctx context.Context, orderID, userID int64, input RequestInput) (*ReturnRequest, error) {
	// First check eligibility
	if err := r.CheckEligibility(ctx, orderID, userID); err != nil {
		return nil, err
	}

	var rr ReturnRequest
	err := r.db.QueryRow(ctx, `
		INSERT INTO return_requests (order_id, user_id, status, reason, customer_note)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, order_id, user_id, status, reason, customer_note, admin_note,
				  requested_at, reviewed_at, received_at, created_at, updated_at
	`, orderID, userID, StatusRequested, input.Reason, nullableString(input.CustomerNote)).Scan(
		&rr.ID, &rr.OrderID, &rr.UserID, &rr.Status, &rr.Reason, &rr.CustomerNote, &rr.AdminNote,
		&rr.RequestedAt, &rr.ReviewedAt, &rr.ReceivedAt, &rr.CreatedAt, &rr.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &rr, nil
}

// GetByOrder returns the return request for the given order ID.
// Returns ErrReturnRequestNotFound if no request exists for this order.
func (r *Repository) GetByOrder(ctx context.Context, orderID int64) (*ReturnRequest, error) {
	var rr ReturnRequest
	err := r.db.QueryRow(ctx, `
		SELECT id, order_id, user_id, status, reason, customer_note, admin_note,
			   requested_at, reviewed_at, received_at, created_at, updated_at
		FROM return_requests WHERE order_id = $1
	`, orderID).Scan(
		&rr.ID, &rr.OrderID, &rr.UserID, &rr.Status, &rr.Reason, &rr.CustomerNote, &rr.AdminNote,
		&rr.RequestedAt, &rr.ReviewedAt, &rr.ReceivedAt, &rr.CreatedAt, &rr.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReturnRequestNotFound
		}
		return nil, err
	}
	return &rr, nil
}

// GetByID returns the return request identified by its primary key.
// Returns ErrReturnRequestNotFound if no such return request exists.
func (r *Repository) GetByID(ctx context.Context, returnRequestID int64) (*ReturnRequest, error) {
	var rr ReturnRequest
	err := r.db.QueryRow(ctx, `
		SELECT id, order_id, user_id, status, reason, customer_note, admin_note,
			   requested_at, reviewed_at, received_at, created_at, updated_at
		FROM return_requests WHERE id = $1
	`, returnRequestID).Scan(
		&rr.ID, &rr.OrderID, &rr.UserID, &rr.Status, &rr.Reason, &rr.CustomerNote, &rr.AdminNote,
		&rr.RequestedAt, &rr.ReviewedAt, &rr.ReceivedAt, &rr.CreatedAt, &rr.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReturnRequestNotFound
		}
		return nil, err
	}
	return &rr, nil
}

// GetByUserAndOrder returns the return request for the given order owned by the user.
// Returns ErrReturnRequestNotFound if no request exists for this order or it belongs to another user.
func (r *Repository) GetByUserAndOrder(ctx context.Context, orderID, userID int64) (*ReturnRequest, error) {
	var rr ReturnRequest
	err := r.db.QueryRow(ctx, `
		SELECT id, order_id, user_id, status, reason, customer_note, admin_note,
			   requested_at, reviewed_at, received_at, created_at, updated_at
		FROM return_requests WHERE order_id = $1 AND user_id = $2
	`, orderID, userID).Scan(
		&rr.ID, &rr.OrderID, &rr.UserID, &rr.Status, &rr.Reason, &rr.CustomerNote, &rr.AdminNote,
		&rr.RequestedAt, &rr.ReviewedAt, &rr.ReceivedAt, &rr.CreatedAt, &rr.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReturnRequestNotFound
		}
		return nil, err
	}
	return &rr, nil
}

// ListByUser returns all return requests for the given user.
func (r *Repository) ListByUser(ctx context.Context, userID int64) ([]ReturnRequest, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, order_id, user_id, status, reason, customer_note, admin_note,
			   requested_at, reviewed_at, received_at, created_at, updated_at
		FROM return_requests WHERE user_id = $1 ORDER BY id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []ReturnRequest
	for rows.Next() {
		var rr ReturnRequest
		if err := rows.Scan(
			&rr.ID, &rr.OrderID, &rr.UserID, &rr.Status, &rr.Reason, &rr.CustomerNote, &rr.AdminNote,
			&rr.RequestedAt, &rr.ReviewedAt, &rr.ReceivedAt, &rr.CreatedAt, &rr.UpdatedAt,
		); err != nil {
			rows.Close()
			return nil, err
		}
		requests = append(requests, rr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return requests, nil
}

// ListAll returns all return requests in the system.
// Intended for admin use only.
func (r *Repository) ListAll(ctx context.Context) ([]ReturnRequest, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, order_id, user_id, status, reason, customer_note, admin_note,
			   requested_at, reviewed_at, received_at, created_at, updated_at
		FROM return_requests ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []ReturnRequest
	for rows.Next() {
		var rr ReturnRequest
		if err := rows.Scan(
			&rr.ID, &rr.OrderID, &rr.UserID, &rr.Status, &rr.Reason, &rr.CustomerNote, &rr.AdminNote,
			&rr.RequestedAt, &rr.ReviewedAt, &rr.ReceivedAt, &rr.CreatedAt, &rr.UpdatedAt,
		); err != nil {
			rows.Close()
			return nil, err
		}
		requests = append(requests, rr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return requests, nil
}

// UpdateStatus transitions a return request to a new status.
// It validates the transition and updates the appropriate timestamp fields.
func (r *Repository) UpdateStatus(ctx context.Context, returnRequestID int64, newStatus string, adminUserID *int64, note *string) (*ReturnRequest, error) {
	// Validate new status
	if !IsValidStatus(newStatus) {
		return nil, ErrReturnInvalidStatus
	}

	// Begin transaction to ensure atomic state transition
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Lock the return request row for update
	var currentStatus string
	err = tx.QueryRow(ctx, `
		SELECT status FROM return_requests WHERE id = $1 FOR UPDATE
	`, returnRequestID).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReturnRequestNotFound
		}
		return nil, err
	}

	// Validate transition
	if !CanTransition(currentStatus, newStatus) {
		return nil, ErrReturnInvalidTransition
	}

	// A return request may only move to the customer-facing "refunded"
	// terminal status once the financial refund has actually succeeded.
	// The refunds table (owned by the refund package) is the sole source
	// of truth for whether money has moved; without this check an admin
	// could mark a return "refunded" via the generic status endpoint while
	// the order's payment_status is still "paid" and no refund exists.
	if newStatus == StatusRefunded {
		var orderID int64
		if err := tx.QueryRow(ctx, `SELECT order_id FROM return_requests WHERE id = $1`, returnRequestID).Scan(&orderID); err != nil {
			return nil, err
		}
		var hasSucceededRefund bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM refunds WHERE order_id = $1 AND status = 'succeeded')
		`, orderID).Scan(&hasSucceededRefund); err != nil {
			return nil, err
		}
		if !hasSucceededRefund {
			return nil, ErrReturnInvalidTransition
		}
	}

	// Update status and timestamps
	var rr ReturnRequest
	var reviewedAt, receivedAt sql.NullTime
	query := fmt.Sprintf(`
		UPDATE return_requests
		SET status = $1,
			updated_at = NOW(),
			%s
		WHERE id = $2
		RETURNING id, order_id, user_id, status, reason, customer_note, admin_note,
				  requested_at, reviewed_at, received_at, created_at, updated_at
	`, buildUpdateColumns(newStatus))
	err = tx.QueryRow(ctx, query, newStatus, returnRequestID, note).Scan(
		&rr.ID, &rr.OrderID, &rr.UserID, &rr.Status, &rr.Reason, &rr.CustomerNote, &rr.AdminNote,
		&rr.RequestedAt, &reviewedAt, &receivedAt, &rr.CreatedAt, &rr.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if reviewedAt.Valid {
		t := reviewedAt.Time
		rr.ReviewedAt = &t
	}
	if receivedAt.Valid {
		t := receivedAt.Time
		rr.ReceivedAt = &t
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Enqueue notification after successful status update
	if r.notifications != nil {
		_ = r.enqueueStatusNotification(ctx, returnRequestID, newStatus)
	}

	return &rr, nil
}

// buildUpdateColumns returns the SQL columns to update based on the new status.
func buildUpdateColumns(newStatus string) string {
	switch newStatus {
	case StatusApproved:
		return "reviewed_at = NOW(), admin_note = $3"
	case StatusRejected:
		return "reviewed_at = NOW(), admin_note = $3"
	case StatusReceived:
		return "received_at = NOW(), admin_note = $3"
	default:
		return "admin_note = $3"
	}
}

// enqueueStatusNotification enqueues a notification for a return request status change.
func (r *Repository) enqueueStatusNotification(ctx context.Context, returnRequestID int64, status string) error {
	// Get the return request and associated order info
	var orderID int64
	var email string
	err := r.db.QueryRow(ctx, `
		SELECT rr.order_id, u.email
		FROM return_requests rr
		JOIN users u ON u.id = rr.user_id
		WHERE rr.id = $1
	`, returnRequestID).Scan(&orderID, &email)
	if err != nil {
		return err
	}

	// Get order total for payload
	var total int64
	err = r.db.QueryRow(ctx, `SELECT total FROM orders WHERE id = $1`, orderID).Scan(&total)
	if err != nil {
		return err
	}

	var eventType string
	var message string

	switch status {
	case StatusApproved:
		eventType = "return_approved"
		message = "درخواست مرجوعی شما تایید شد"
	case StatusRejected:
		eventType = "return_rejected"
		message = "درخواست مرجوعی شما رد شد"
	case StatusReceived:
		eventType = "return_received"
		message = "محصول مرجوعی دریافت شد"
	case StatusRefundPending:
		eventType = "refund_initiated"
		message = "بازپرداخت در حال پردازش است"
	case StatusRefunded:
		eventType = "refund_succeeded"
		message = "مبلغ بازپرداخت شد"
	case StatusRefundFailed:
		eventType = "refund_failed"
		message = "بازپرداخت انجام نشد"
	default:
		return nil
	}

	payload := notification.ReturnRequestPayload(orderID, "مشتری", email, status, message, total)

	_, _ = r.notifications.Enqueue(ctx, notification.EventKeyForReturnRequest(returnRequestID, status), 0, email, eventType, payload)
	return nil
}

// nullableString returns nil for an empty string and a pointer to s otherwise.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
