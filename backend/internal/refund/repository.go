package refund

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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

// WithNotifications sets the notification repository for refunds.
func (r *Repository) WithNotifications(notifRepo *notification.Repository) *Repository {
	r.notifications = notifRepo
	return r
}

// CheckEligibility verifies whether an order is eligible for a refund.
// It checks:
// - Order exists
// - Payment status is 'paid' (not already refunded)
// - Order is not cancelled
func (r *Repository) CheckEligibility(ctx context.Context, orderID int64) error {
	var status, paymentStatus string
	err := r.db.QueryRow(ctx, `
		SELECT status, payment_status FROM orders WHERE id = $1
	`, orderID).Scan(&status, &paymentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRefundNotFound
		}
		return err
	}

	// Order must be paid (not pending, failed, or already refunded)
	if paymentStatus != order.PaymentStatusPaid {
		if paymentStatus == order.PaymentStatusRefunded {
			return ErrRefundAlreadyRefunded
		}
		return fmt.Errorf("%w: payment must be completed (current status: %s)", ErrRefundNotEligible, paymentStatus)
	}

	// Order must not be cancelled
	if status == order.StatusCancelled {
		return fmt.Errorf("%w: cancelled order cannot be refunded", ErrRefundNotEligible)
	}

	return nil
}

// Create creates a new refund record for the given order.
// It verifies eligibility and locks the order row to prevent concurrent refunds.
func (r *Repository) Create(ctx context.Context, orderID int64, returnRequestID *int64, amount int64, paymentMethod string, requestedByAdminID *int64) (*Refund, error) {
	// First check eligibility
	if err := r.CheckEligibility(ctx, orderID); err != nil {
		return nil, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Lock the order row to prevent concurrent refunds
	var currentPaymentStatus string
	err = tx.QueryRow(ctx, `
		SELECT payment_status FROM orders WHERE id = $1 FOR UPDATE
	`, orderID).Scan(&currentPaymentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRefundNotFound
		}
		return nil, err
	}

	// Check again after locking (double-check)
	if currentPaymentStatus != order.PaymentStatusPaid {
		if currentPaymentStatus == order.PaymentStatusRefunded {
			return nil, ErrRefundAlreadyRefunded
		}
		return nil, fmt.Errorf("%w: payment must be completed", ErrRefundNotEligible)
	}

	// Create the refund record
	var refund Refund
	var providerResponse sql.NullString
	err = tx.QueryRow(ctx, `
		INSERT INTO refunds (order_id, return_request_id, amount, payment_method, status, requested_by_admin_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, order_id, return_request_id, amount, payment_method, status,
				  provider_refund_id, provider_response, requested_by_admin_id, last_error,
				  created_at, updated_at, completed_at
	`, orderID, returnRequestID, amount, paymentMethod, StatusPending, requestedByAdminID).Scan(
		&refund.ID, &refund.OrderID, &refund.ReturnRequestID, &refund.Amount, &refund.PaymentMethod, &refund.Status,
		&refund.ProviderRefundID, &providerResponse, &refund.RequestedByAdminID, &refund.LastError,
		&refund.CreatedAt, &refund.UpdatedAt, &refund.CompletedAt,
	)
	if err != nil {
		return nil, err
	}

	if providerResponse.Valid {
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(providerResponse.String), &raw); err != nil {
			tx.Rollback(ctx)
			return nil, fmt.Errorf("invalid provider response: %v", err)
		}
		refund.ProviderResponse = raw
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &refund, nil
}

// GetByOrder returns the refund for the given order ID.
// Returns ErrRefundNotFound if no refund exists for this order.
func (r *Repository) GetByOrder(ctx context.Context, orderID int64) (*Refund, error) {
	var refund Refund
	var providerResponse sql.NullString
	err := r.db.QueryRow(ctx, `
		SELECT id, order_id, return_request_id, amount, payment_method, status,
			   provider_refund_id, provider_response, requested_by_admin_id, last_error,
			   created_at, updated_at, completed_at
		FROM refunds WHERE order_id = $1
	`, orderID).Scan(
		&refund.ID, &refund.OrderID, &refund.ReturnRequestID, &refund.Amount, &refund.PaymentMethod, &refund.Status,
		&refund.ProviderRefundID, &providerResponse, &refund.RequestedByAdminID, &refund.LastError,
		&refund.CreatedAt, &refund.UpdatedAt, &refund.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRefundNotFound
		}
		return nil, err
	}

	if providerResponse.Valid {
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(providerResponse.String), &raw); err != nil {
			return nil, fmt.Errorf("invalid provider response: %v", err)
		}
		refund.ProviderResponse = raw
	}

	return &refund, nil
}

// UpdateStatus transitions a refund to a new status.
// It validates the transition, updates timestamps, and atomically updates the order's payment_status
// when the refund succeeds.
func (r *Repository) UpdateStatus(ctx context.Context, refundID int64, newStatus string, providerRefundID *string, providerResponse json.RawMessage, lastError *string, completedAt *time.Time) (*Refund, error) {
	// Validate new status
	if !IsValidStatus(newStatus) {
		return nil, ErrRefundInvalidStatus
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Lock the refund row for update
	var currentStatus string
	var orderID int64
	var paymentMethod string
	var returnRequestID sql.NullInt64
	err = tx.QueryRow(ctx, `
		SELECT status, order_id, return_request_id, payment_method
		FROM refunds WHERE id = $1 FOR UPDATE
	`, refundID).Scan(&currentStatus, &orderID, &returnRequestID, &paymentMethod)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRefundNotFound
		}
		return nil, err
	}

	// Validate transition
	if !CanTransition(currentStatus, newStatus) {
		return nil, ErrRefundInvalidTransition
	}

	// Update refund record
	var refund Refund
	var completedAtVal sql.NullTime
	var providerResponseStr sql.NullString

	if len(providerResponse) > 0 {
		if err := json.Unmarshal(providerResponse, &providerResponseStr); err != nil {
			return nil, fmt.Errorf("invalid provider response JSON: %v", err)
		}
	}

	err = tx.QueryRow(ctx, `
		UPDATE refunds
		SET status = $1,
			provider_refund_id = $2,
			provider_response = $3,
			last_error = $4,
			completed_at = $5,
			updated_at = NOW()
		WHERE id = $6
		RETURNING id, order_id, return_request_id, amount, payment_method, status,
				  provider_refund_id, provider_response, requested_by_admin_id, last_error,
				  created_at, updated_at, completed_at
	`, newStatus, providerRefundID, providerResponseStr, lastError, completedAtVal, refundID).Scan(
		&refund.ID, &refund.OrderID, &refund.ReturnRequestID, &refund.Amount, &refund.PaymentMethod, &refund.Status,
		&refund.ProviderRefundID, &providerResponseStr, &refund.RequestedByAdminID, &refund.LastError,
		&refund.CreatedAt, &refund.UpdatedAt, &completedAtVal,
	)
	if err != nil {
		return nil, err
	}

	if completedAtVal.Valid {
		t := completedAtVal.Time
		refund.CompletedAt = &t
	}

	if providerResponseStr.Valid {
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(providerResponseStr.String), &raw); err != nil {
			return nil, fmt.Errorf("invalid provider response: %v", err)
		}
		refund.ProviderResponse = raw
	}

	// If refund succeeded, update the order's payment_status atomically
	if newStatus == StatusSucceeded {
		_, err = tx.Exec(ctx, `
			UPDATE orders SET payment_status = $1, updated_at = NOW()
			WHERE id = $2 AND payment_status = $3
		`, order.PaymentStatusRefunded, orderID, order.PaymentStatusPaid)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Enqueue notification after successful status update
	if r.notifications != nil {
		_ = r.enqueueStatusNotification(ctx, refundID, newStatus, orderID)
	}

	return &refund, nil
}

// enqueueStatusNotification enqueues a notification for a refund status change.
func (r *Repository) enqueueStatusNotification(ctx context.Context, refundID int64, status string, orderID int64) error {
	var email string
	err := r.db.QueryRow(ctx, `
		SELECT u.email FROM orders o JOIN users u ON u.id = o.user_id WHERE o.id = $1
	`, orderID).Scan(&email)
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
	case StatusSucceeded:
		eventType = "refund_succeeded"
		message = "مبلغ بازپرداخت شد"
	case StatusFailed:
		eventType = "refund_failed"
		message = "بازپرداخت انجام نشد"
	case StatusManualReview:
		eventType = "refund_manual_review"
		message = "بازپرداخت نیاز به بررسی دستی دارد"
	default:
		return nil
	}

	payload := notification.RefundPayload(orderID, "مشتری", email, status, message, total)

	_, _ = r.notifications.Enqueue(ctx, notification.EventKeyForRefund(refundID, status), 0, email, eventType, payload)
	return nil
}

// IsOrderRefunded checks if the given order has been refunded.
func (r *Repository) IsOrderRefunded(ctx context.Context, orderID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM refunds WHERE order_id = $1 AND status = $2)
	`, orderID, StatusSucceeded).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// ListAll returns all refunds in the system.
// Intended for admin use only.
func (r *Repository) ListAll(ctx context.Context) ([]Refund, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, order_id, return_request_id, amount, payment_method, status,
			   provider_refund_id, provider_response, requested_by_admin_id, last_error,
			   created_at, updated_at, completed_at
		FROM refunds ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refunds []Refund
	for rows.Next() {
		var refund Refund
		var providerResponseStr sql.NullString
		if err := rows.Scan(
			&refund.ID, &refund.OrderID, &refund.ReturnRequestID, &refund.Amount, &refund.PaymentMethod, &refund.Status,
			&refund.ProviderRefundID, &providerResponseStr, &refund.RequestedByAdminID, &refund.LastError,
			&refund.CreatedAt, &refund.UpdatedAt, &refund.CompletedAt,
		); err != nil {
			rows.Close()
			return nil, err
		}

		if providerResponseStr.Valid {
			var raw json.RawMessage
			if err := json.Unmarshal([]byte(providerResponseStr.String), &raw); err != nil {
				rows.Close()
				return nil, fmt.Errorf("invalid provider response: %v", err)
			}
			refund.ProviderResponse = raw
		}

		refunds = append(refunds, refund)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return refunds, nil
}

// GetByID returns a refund by its ID.
func (r *Repository) GetByID(ctx context.Context, refundID int64) (*Refund, error) {
	var refund Refund
	var providerResponseStr sql.NullString
	err := r.db.QueryRow(ctx, `
		SELECT id, order_id, return_request_id, amount, payment_method, status,
			   provider_refund_id, provider_response, requested_by_admin_id, last_error,
			   created_at, updated_at, completed_at
		FROM refunds WHERE id = $1
	`, refundID).Scan(
		&refund.ID, &refund.OrderID, &refund.ReturnRequestID, &refund.Amount, &refund.PaymentMethod, &refund.Status,
		&refund.ProviderRefundID, &providerResponseStr, &refund.RequestedByAdminID, &refund.LastError,
		&refund.CreatedAt, &refund.UpdatedAt, &refund.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRefundNotFound
		}
		return nil, err
	}

	if providerResponseStr.Valid {
		var raw json.RawMessage
		if err := json.Unmarshal([]byte(providerResponseStr.String), &raw); err != nil {
			return nil, fmt.Errorf("invalid provider response: %v", err)
		}
		refund.ProviderResponse = raw
	}

	return &refund, nil
}

// GetPendingRefunds returns all refunds that are pending or processing.
func (r *Repository) GetPendingRefunds(ctx context.Context) ([]Refund, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, order_id, return_request_id, amount, payment_method, status,
			   provider_refund_id, provider_response, requested_by_admin_id, last_error,
			   created_at, updated_at, completed_at
		FROM refunds WHERE status IN ($1, $2) ORDER BY id
	`, StatusPending, StatusProcessing)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refunds []Refund
	for rows.Next() {
		var refund Refund
		var providerResponseStr sql.NullString
		if err := rows.Scan(
			&refund.ID, &refund.OrderID, &refund.ReturnRequestID, &refund.Amount, &refund.PaymentMethod, &refund.Status,
			&refund.ProviderRefundID, &providerResponseStr, &refund.RequestedByAdminID, &refund.LastError,
			&refund.CreatedAt, &refund.UpdatedAt, &refund.CompletedAt,
		); err != nil {
			rows.Close()
			return nil, err
		}

		if providerResponseStr.Valid {
			var raw json.RawMessage
			if err := json.Unmarshal([]byte(providerResponseStr.String), &raw); err != nil {
				rows.Close()
				return nil, fmt.Errorf("invalid provider response: %v", err)
			}
			refund.ProviderResponse = raw
		}

		refunds = append(refunds, refund)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return refunds, nil
}
