// Package return implements the return request workflow for V1.
//
// return_requests tracks customer-initiated return/refund requests and
// their state machine transitions. This package handles the return request
// lifecycle but not the actual financial refund (see the refund package).
package returnpkg

import (
	"errors"
	"time"
)

// Status values for return_requests.
// State machine:
//
//	requested -> approved | rejected
//	approved -> received
//	received -> refund_pending
//	refund_pending -> refunded | refund_failed
const (
	StatusRequested     = "requested"
	StatusApproved      = "approved"
	StatusRejected      = "rejected"
	StatusReceived      = "received"
	StatusRefundPending = "refund_pending"
	StatusRefunded      = "refunded"
	StatusRefundFailed  = "refund_failed"
)

var validReturnStatuses = map[string]bool{
	StatusRequested:     true,
	StatusApproved:      true,
	StatusRejected:      true,
	StatusReceived:      true,
	StatusRefundPending: true,
	StatusRefunded:      true,
	StatusRefundFailed:  true,
}

// IsValidStatus reports whether status is a valid return request status.
func IsValidStatus(status string) bool {
	return validReturnStatuses[status]
}

// allowedTransitions encodes the return request state machine.
var allowedTransitions = map[string]map[string]bool{
	StatusRequested: {
		StatusApproved: true,
		StatusRejected: true,
	},
	StatusApproved: {
		StatusReceived: true,
	},
	StatusReceived: {
		StatusRefundPending: true,
	},
	StatusRefundPending: {
		StatusRefunded:     true,
		StatusRefundFailed: true,
	},
}

// CanTransition reports whether a return request may move from `from` to `to`.
func CanTransition(from, to string) bool {
	next, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	return next[to]
}

// ReturnRequest represents a row in the return_requests table.
type ReturnRequest struct {
	ID           int64      `json:"id"`
	OrderID      int64      `json:"order_id"`
	UserID       int64      `json:"-"`
	Status       string     `json:"status"`
	Reason       string     `json:"reason"`
	CustomerNote *string    `json:"customer_note,omitempty"`
	AdminNote    *string    `json:"admin_note,omitempty"`
	RequestedAt  time.Time  `json:"requested_at"`
	ReviewedAt   *time.Time `json:"reviewed_at,omitempty"`
	ReceivedAt   *time.Time `json:"received_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// RequestInput is the customer-submitted portion of a return request.
type RequestInput struct {
	Reason       string `json:"reason"`
	CustomerNote string `json:"customer_note,omitempty"`
}

// Field length limits for return request fields.
const (
	MaxReasonLen       = 1024
	MaxCustomerNoteLen = 2048
)

// ErrValidation is returned when input validation fails.
type ErrValidation struct {
	Field   string
	Message string
}

func (e *ErrValidation) Error() string {
	return e.Field + ": " + e.Message
}

// Validate checks that reason is non-empty and within bounds,
// and customer_note is within bounds if provided.
func (in RequestInput) Validate() error {
	if len(in.Reason) == 0 {
		return &ErrValidation{Field: "reason", Message: "is required"}
	}
	if len(in.Reason) > MaxReasonLen {
		return &ErrValidation{Field: "reason", Message: "is too long (max 1024 characters)"}
	}

	if len(in.CustomerNote) > MaxCustomerNoteLen {
		return &ErrValidation{Field: "customer_note", Message: "is too long (max 2048 characters)"}
	}

	return nil
}

// Trimmed returns a copy of in with trimmed fields.
func (in RequestInput) Trimmed() RequestInput {
	return RequestInput{
		Reason:       in.Reason,
		CustomerNote: in.CustomerNote,
	}
}

var (
	// ErrReturnRequestNotFound is returned when no return request exists
	// for the given order or user.
	ErrReturnRequestNotFound = errors.New("return request not found")

	// ErrReturnAlreadyExists is returned when a return request already
	// exists for the given order.
	ErrReturnAlreadyExists = errors.New("return request already exists for this order")

	// ErrReturnInvalidStatus is returned when attempting an invalid
	// status transition or using an invalid status value.
	ErrReturnInvalidStatus = errors.New("invalid return request status")

	// ErrReturnNotEligible is returned when the order is not eligible
	// for a return request (e.g., not delivered, not paid).
	ErrReturnNotEligible = errors.New("order is not eligible for return request")

	// ErrReturnInvalidTransition is returned when an invalid status
	// transition is attempted on a return request.
	ErrReturnInvalidTransition = errors.New("invalid return request status transition")
)
