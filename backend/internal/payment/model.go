// Package payment implements ZarinPal Payment V1: a server-side-only
// integration between orders and the ZarinPal payment gateway. No package
// outside of payment (and the order package, for the small amount of
// coordination described in handler.go) ever talks to ZarinPal directly,
// and the Merchant ID/gateway responses are never exposed to the browser
// beyond the redirect URL itself.
package payment

import (
	"errors"
	"time"
)

// Attempt is a row in the payment_attempts table: one ZarinPal payment
// attempt for a single order. An order may have several attempts over time
// (e.g. an abandoned or failed attempt followed by a retry); the current
// state of "has this order been paid" always lives on orders.payment_status,
// which is only ever updated by a verified attempt (see Repository.
// MarkVerified).
type Attempt struct {
	ID           int64      `json:"id"`
	OrderID      int64      `json:"order_id"`
	Provider     string     `json:"provider"`
	Currency     string     `json:"currency"`
	Environment  string     `json:"environment"`
	AccountKey   string     `json:"-"`
	Authority    *string    `json:"authority"`
	Amount       int64      `json:"amount"`
	Status       string     `json:"status"`
	RefID        *int64     `json:"ref_id"`
	ProviderCode *int       `json:"provider_code"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	VerifiedAt   *time.Time `json:"verified_at"`
}

// Provider identifiers. ZarinPal is the only supported provider in V1; the
// column still carries a value (rather than being implied) so a second
// provider could be added later without a schema change.
const ProviderZarinPal = "zarinpal"

// Attempt statuses. This is intentionally a smaller state machine than
// orders.status: an attempt is either awaiting a provider response/
// callback (pending), successfully verified server-to-server (paid), or
// conclusively not going to succeed (failed — covers both "the payment/
// request call itself failed" and "verification failed/was rejected").
const (
	StatusPending        = "pending"
	StatusPaid           = "paid"
	StatusFailed         = "failed"
	StatusReconciliation = "reconciliation"
)

var (
	ErrPaymentInProgress   = errors.New("payment is in progress")
	ErrProviderMismatch    = errors.New("payment provider binding mismatch")
	ErrInvalidAuthority    = errors.New("invalid payment authority")
	ErrInvalidVerification = errors.New("invalid payment verification")

	// ErrOrderNotFound is returned when the referenced order does not
	// exist or does not belong to the requesting user.
	ErrOrderNotFound = errors.New("order not found")

	// ErrOrderAlreadyPaid is returned when attempting to initiate a new
	// payment for an order whose payment_status is already "paid".
	ErrOrderAlreadyPaid = errors.New("order is already paid")

	// ErrOrderNotPayable is returned when attempting to initiate a new
	// payment for an order that is cancelled or delivered — orders in
	// either of those states are no longer eligible to be paid.
	ErrOrderNotPayable = errors.New("order is not payable")

	// ErrAttemptNotFound is returned when a callback references an
	// authority with no matching payment_attempts row.
	ErrAttemptNotFound = errors.New("payment attempt not found")

	// ErrAlreadyProcessed is returned by MarkVerified/MarkFailed when the
	// attempt has already reached a terminal status (paid or failed) —
	// this is the idempotency guard for repeated/duplicate callbacks.
	ErrAlreadyProcessed = errors.New("payment attempt already processed")
)
