package order

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Order represents a row in the orders table.
//
// Delivery snapshot fields (RecipientName, Phone, AddressLine1,
// AddressLine2, City, PostalCode, Country) are nullable because orders
// placed before the Checkout & Fulfillment V2 migration never collected
// this information; they are pointers so a missing historical value is
// represented as JSON `null` rather than a misleading empty string. Every
// order created through Create (the checkout flow) populates all of these
// except AddressLine2, which is genuinely optional (e.g. no apartment/unit
// number).
//
// ItemsSubtotal, ShippingFee, and Total are always related by
// Total = ItemsSubtotal + ShippingFee, enforced server-side at checkout
// time; the frontend never supplies any of these three values.
type Order struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"-"`
	Status    string    `json:"status"`
	Total     int64     `json:"total"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	RecipientName *string `json:"recipient_name"`
	Phone         *string `json:"phone"`
	AddressLine1  *string `json:"address_line1"`
	AddressLine2  *string `json:"address_line2"`
	City          *string `json:"city"`
	PostalCode    *string `json:"postal_code"`
	Country       *string `json:"country"`

	ShippingMethod string `json:"shipping_method"`
	ShippingFee    int64  `json:"shipping_fee"`
	ItemsSubtotal  int64  `json:"items_subtotal"`

	PaymentStatus string `json:"payment_status"`
	PaymentMethod string `json:"payment_method"`

	// CouponCode and DiscountAmount are an immutable snapshot taken at
	// checkout time (see coupon package). CouponCode is nil for orders
	// that did not use a coupon (including every order placed before the
	// Coupons & Discounts V1 migration, which are backfilled with NULL /
	// 0). Historical display must always use this snapshot, never the
	// coupon's current definition — a coupon can be edited or deactivated
	// after an order that used it was placed, and that must never change
	// what the order shows. DiscountAmount already only ever discounts
	// ItemsSubtotal; Total = (ItemsSubtotal - DiscountAmount) + ShippingFee.
	CouponCode     *string `json:"coupon_code"`
	DiscountAmount int64   `json:"discount_amount"`
}

// Item is an immutable snapshot of a product at the time an order was
// placed. It is stored separately from products so that later changes to a
// product's name, slug, or price never affect historical orders.
type Item struct {
	ID          int64  `json:"id"`
	OrderID     int64  `json:"-"`
	ProductID   int64  `json:"product_id"`
	ProductName string `json:"product_name"`
	ProductSlug string `json:"product_slug"`
	UnitPrice   int64  `json:"unit_price"`
	Quantity    int    `json:"quantity"`
	Subtotal    int64  `json:"subtotal"`
}

// OrderWithItems is the full response payload for GET /orders/{id}.
type OrderWithItems struct {
	Order
	Items []Item `json:"items"`
}

// Customer is a safe, minimal identity snapshot of the user who placed an
// order. It intentionally excludes password_hash and any session/auth data
// and is only ever populated for admin-facing responses.
type Customer struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// AdminOrder is the response shape for GET /admin/orders: an order plus the
// safe identity of the customer who placed it. Kept separate from Order so
// the customer-facing API surface never changes shape.
type AdminOrder struct {
	Order
	Customer Customer `json:"customer"`
}

// AdminOrderWithItems is the response shape for GET /admin/orders/{id}.
type AdminOrderWithItems struct {
	Order
	Customer Customer `json:"customer"`
	Items    []Item   `json:"items"`
}

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusShipped    = "shipped"
	StatusDelivered  = "delivered"
	StatusCancelled  = "cancelled"
)

// Payment statuses. No real payment provider is integrated yet; every
// order starts as StatusPaymentPending and there is currently no code path
// that moves an order to any other payment status. These exist so the
// schema/API are ready for a real provider to be wired in later without
// another migration.
const (
	PaymentStatusPending  = "pending"
	PaymentStatusPaid     = "paid"
	PaymentStatusFailed   = "failed"
	PaymentStatusRefunded = "refunded"
)

// Payment methods. "manual" is the original placeholder (no gateway
// integration); "zarinpal" is set once an order has an associated ZarinPal
// payment attempt (see the payment package), regardless of whether that
// attempt has been verified yet.
const (
	PaymentMethodManual   = "manual"
	PaymentMethodZarinPal = "zarinpal"
)

// Shipping methods supported by checkout in V1. Carrier integration is out
// of scope; these are just labels used to select a fixed backend fee (see
// ShippingFeeFor in shipping.go).
const (
	ShippingMethodStandard = "standard"
	ShippingMethodExpress  = "express"
)

var validShippingMethods = map[string]bool{
	ShippingMethodStandard: true,
	ShippingMethodExpress:  true,
}

// IsValidShippingMethod reports whether method is one of the supported V1
// shipping methods.
func IsValidShippingMethod(method string) bool {
	return validShippingMethods[method]
}

// validStatuses is used to validate status values supplied by admins before
// they ever reach the database.
var validStatuses = map[string]bool{
	StatusPending:    true,
	StatusProcessing: true,
	StatusShipped:    true,
	StatusDelivered:  true,
	StatusCancelled:  true,
}

// IsValidStatus reports whether status is one of the known order statuses.
func IsValidStatus(status string) bool {
	return validStatuses[status]
}

// allowedTransitions encodes the order status state machine: for each
// status, the set of statuses it may move to. Statuses not present as keys
// (delivered, cancelled) are terminal and permit no further transitions.
var allowedTransitions = map[string]map[string]bool{
	StatusPending:    {StatusProcessing: true, StatusCancelled: true},
	StatusProcessing: {StatusShipped: true, StatusCancelled: true},
	StatusShipped:    {StatusDelivered: true},
}

// CanTransition reports whether an order may move from `from` to `to`.
// Transitioning to the same status is never allowed (callers should treat
// it the same as any other invalid transition), and unknown/terminal
// statuses simply have no allowed transitions.
func CanTransition(from, to string) bool {
	next, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	return next[to]
}

// fulfillmentStatuses is the set of statuses that represent the seller
// actually fulfilling the order (as opposed to pending or cancelled).
// Advancing a ZarinPal order into any of these requires payment_status =
// "paid" first (see RequiresPaidPayment).
var fulfillmentStatuses = map[string]bool{
	StatusProcessing: true,
	StatusShipped:    true,
	StatusDelivered:  true,
}

// RequiresPaidPayment reports whether transitioning a ZarinPal order to
// newStatus requires payment_status to already be "paid". Orders paid
// through "manual" are unaffected by this rule — it only applies to the
// zarinpal payment method, since that is the only method V1 can verify
// server-side. Cancelling (from any status) is never gated.
func RequiresPaidPayment(paymentMethod, newStatus string) bool {
	return paymentMethod == PaymentMethodZarinPal && fulfillmentStatuses[newStatus]
}

var (
	ErrEmptyCart         = errors.New("cart is empty")
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrProductNotFound   = errors.New("product not found")
	ErrOrderNotFound     = errors.New("order not found")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrRefundRequired    = fmt.Errorf("%w: paid order cannot be cancelled: refund or manual reconciliation required", ErrInvalidTransition)

	// ErrPaymentRequired is returned by UpdateStatus when an admin attempts
	// to advance a ZarinPal order (payment_method = "zarinpal") into
	// processing/shipped/delivered while its payment_status is not yet
	// "paid". Fulfillment of a gateway-paid order must wait for a verified
	// payment; "manual" orders are unaffected by this rule.
	ErrPaymentRequired = errors.New("payment required before fulfillment")

	// ErrOrderNotEligibleForCancel is returned by CancelOwnOrder when the
	// order's payment has already succeeded (payment_status = "paid").
	// Customers may only self-cancel unpaid orders; a paid order requires
	// admin-mediated cancellation/refund instead.
	ErrOrderNotEligibleForCancel = errors.New("order is not eligible for self-service cancellation")

	// ErrCouponInvalid is the sentinel wrapped by ErrCouponEligibility, so
	// callers can check errors.Is(err, ErrCouponInvalid) without caring
	// about the specific message.
	ErrCouponInvalid = errors.New("coupon is not valid for this order")
)

// ErrCouponEligibility wraps a safe, customer-facing message describing
// why a supplied coupon code was rejected during checkout (e.g. the coupon
// became invalid, expired, or fully used between an earlier preview and
// order submission). The order is never created when this is returned.
type ErrCouponEligibility struct {
	Message string
}

func (e *ErrCouponEligibility) Error() string { return e.Message }
func (e *ErrCouponEligibility) Unwrap() error { return ErrCouponInvalid }

// CheckoutInput is the user-provided portion of a checkout request: the
// delivery snapshot and chosen shipping method. It intentionally has no
// price/fee/total fields and no user id field — the authenticated user id
// always comes from the session, and every price is calculated
// server-side.
// CouponCode is optional: an empty string (after trimming) means "no
// coupon", preserving existing checkout behavior exactly when omitted. It
// is never trusted as authoritative for the discount amount — only the
// code itself is taken from the client; the actual discount is always
// recalculated server-side from the coupon's current row inside the
// checkout transaction (see coupon.Eligible).
type CheckoutInput struct {
	RecipientName  string
	Phone          string
	AddressLine1   string
	AddressLine2   string
	City           string
	PostalCode     string
	Country        string
	ShippingMethod string
	CouponCode     string
}

// Field length limits enforced on CheckoutInput, matching the column sizes
// defined in 007_add_order_fulfillment.sql.
const (
	maxRecipientNameLen = 255
	maxPhoneLen         = 64
	maxAddressLineLen   = 255
	maxCityLen          = 128
	maxPostalCodeLen    = 32
	maxCountryLen       = 128
)

// ErrValidation is returned by CheckoutInput.Validate when a field fails
// validation. The Field and Message are safe to surface to the client.
type ErrValidation struct {
	Field   string
	Message string
}

func (e *ErrValidation) Error() string {
	return e.Field + ": " + e.Message
}

// Validate checks that every required field is present (after trimming
// surrounding whitespace), within its maximum length, and that the
// shipping method is one of the supported V1 values. It does not mutate
// the receiver; callers should use the trimmed values returned by
// Trimmed() when persisting.
func (in CheckoutInput) Validate() error {
	trimmed := in.Trimmed()

	required := []struct {
		field  string
		value  string
		maxLen int
	}{
		{"recipient_name", trimmed.RecipientName, maxRecipientNameLen},
		{"phone", trimmed.Phone, maxPhoneLen},
		{"address_line1", trimmed.AddressLine1, maxAddressLineLen},
		{"city", trimmed.City, maxCityLen},
		{"postal_code", trimmed.PostalCode, maxPostalCodeLen},
		{"country", trimmed.Country, maxCountryLen},
	}
	for _, f := range required {
		if f.value == "" {
			return &ErrValidation{Field: f.field, Message: "is required"}
		}
		if len(f.value) > f.maxLen {
			return &ErrValidation{Field: f.field, Message: "is too long"}
		}
	}

	// address_line2 is optional but still length-limited if provided.
	if len(trimmed.AddressLine2) > maxAddressLineLen {
		return &ErrValidation{Field: "address_line2", Message: "is too long"}
	}

	if trimmed.ShippingMethod == "" {
		return &ErrValidation{Field: "shipping_method", Message: "is required"}
	}
	if !IsValidShippingMethod(trimmed.ShippingMethod) {
		return &ErrValidation{Field: "shipping_method", Message: "is not a supported shipping method"}
	}

	return nil
}

// Trimmed returns a copy of in with every string field's surrounding
// whitespace removed.
func (in CheckoutInput) Trimmed() CheckoutInput {
	return CheckoutInput{
		RecipientName:  strings.TrimSpace(in.RecipientName),
		Phone:          strings.TrimSpace(in.Phone),
		AddressLine1:   strings.TrimSpace(in.AddressLine1),
		AddressLine2:   strings.TrimSpace(in.AddressLine2),
		City:           strings.TrimSpace(in.City),
		PostalCode:     strings.TrimSpace(in.PostalCode),
		Country:        strings.TrimSpace(in.Country),
		ShippingMethod: strings.TrimSpace(in.ShippingMethod),
		CouponCode:     strings.TrimSpace(in.CouponCode),
	}
}
