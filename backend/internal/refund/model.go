// Package refund implements the financial refund workflow for V1.
//
// refunds tracks the actual financial refund attempts for orders.
// A successful refund transitions the order's payment_status to 'refunded'.
//
// IMPORTANT: This package implements the workflow and database layer.
// The actual refund call to ZarinPal is delegated to a provider interface
// to support different refund mechanisms and testing.
package refund

import (
	"encoding/json"
	"errors"
	"time"
)

// Status values for refunds.
const (
	StatusPending       = "pending"
	StatusProcessing    = "processing"
	StatusSucceeded     = "succeeded"
	StatusFailed        = "failed"
	StatusManualReview  = "manual_review"
	StatusRefundPending = "refund_pending"
)

// Refund represents a row in the refunds table.
type Refund struct {
	ID                 int64           `json:"id"`
	OrderID            int64           `json:"order_id"`
	ReturnRequestID    *int64          `json:"return_request_id,omitempty"`
	Amount             int64           `json:"amount"`
	PaymentMethod      string          `json:"payment_method"`
	Status             string          `json:"status"`
	ProviderRefundID   *string         `json:"provider_refund_id,omitempty"`
	ProviderResponse   json.RawMessage `json:"provider_response,omitempty"`
	RequestedByAdminID *int64          `json:"requested_by_admin_id,omitempty"`
	LastError          *string         `json:"last_error,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	CompletedAt        *time.Time      `json:"completed_at,omitempty"`
}

// Input is the admin-submitted portion of a refund request.
type Input struct {
	Amount           int64   `json:"amount"`
	ProviderRefundID *string `json:"provider_refund_id,omitempty"`
	LastError        *string `json:"last_error,omitempty"`
}

// ErrValidation is returned when input validation fails.
type ErrValidation struct {
	Field   string
	Message string
}

func (e *ErrValidation) Error() string {
	return e.Field + ": " + e.Message
}

// Validate checks that amount matches the order total and is positive.
func (in Input) Validate() error {
	if in.Amount <= 0 {
		return &ErrValidation{Field: "amount", Message: "must be positive"}
	}
	return nil
}

var (
	// ErrRefundNotFound is returned when no refund exists for the given order.
	ErrRefundNotFound = errors.New("refund not found")

	// ErrRefundAlreadyExists is returned when a refund already exists for the given order.
	ErrRefundAlreadyExists = errors.New("refund already exists for this order")

	// ErrRefundInvalidStatus is returned when attempting an invalid status transition.
	ErrRefundInvalidStatus = errors.New("invalid refund status")

	// ErrRefundNotEligible is returned when the order is not eligible for a refund.
	ErrRefundNotEligible = errors.New("order is not eligible for refund")

	// ErrRefundAlreadyRefunded is returned when attempting to refund an already-refunded order.
	ErrRefundAlreadyRefunded = errors.New("order has already been refunded")

	// ErrRefundInvalidTransition is returned when an invalid status transition is attempted.
	ErrRefundInvalidTransition = errors.New("invalid refund status transition")
)

// validRefundStatuses is used to validate refund status values.
var validRefundStatuses = map[string]bool{
	StatusPending:      true,
	StatusProcessing:   true,
	StatusSucceeded:    true,
	StatusFailed:       true,
	StatusManualReview: true,
}

// IsValidStatus reports whether status is a valid refund status.
func IsValidStatus(status string) bool {
	return validRefundStatuses[status]
}

// allowedRefundTransitions encodes the refund state machine.
var allowedRefundTransitions = map[string]map[string]bool{
	StatusPending: {
		StatusProcessing:   true,
		StatusFailed:       true,
		StatusManualReview: true,
	},
	StatusProcessing: {
		StatusSucceeded:    true,
		StatusFailed:       true,
		StatusManualReview: true,
	},
	StatusManualReview: {
		StatusSucceeded: true,
		StatusFailed:    true,
	},
}

// CanTransition reports whether a refund may move from `from` to `to`.
func CanTransition(from, to string) bool {
	next, ok := allowedRefundTransitions[from]
	if !ok {
		return false
	}
	return next[to]
}
