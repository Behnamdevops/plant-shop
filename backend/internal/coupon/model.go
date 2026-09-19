// Package coupon implements Coupons & Discounts V1: a single, server-
// authoritative coupon per order, applied to items_subtotal only (never to
// shipping_fee). See README-level notes in repository.go for the
// concurrency-safe redemption strategy.
package coupon

import (
	"errors"
	"strings"
	"time"
)

const (
	DiscountTypePercent = "percent"
	DiscountTypeFixed   = "fixed"
)

// Coupon represents a row in the coupons table.
type Coupon struct {
	ID             int64      `json:"id"`
	Code           string     `json:"code"`
	DiscountType   string     `json:"discount_type"`
	Value          int64      `json:"value"`
	MinOrderAmount int64      `json:"min_order_amount"`
	UsageLimit     *int       `json:"usage_limit"`
	PerUserLimit   *int       `json:"per_user_limit"`
	StartsAt       *time.Time `json:"starts_at"`
	EndsAt         *time.Time `json:"ends_at"`
	IsActive       bool       `json:"is_active"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// AdminCoupon additionally exposes current usage counts for the admin
// list/detail views. Usage is computed from unreleased redemptions, never
// stored redundantly on the coupon row itself.
type AdminCoupon struct {
	Coupon
	UsageCount int64 `json:"usage_count"`
}

// Preview is the response shape for POST /api/v1/coupons/preview. It is
// advisory only: order creation always revalidates and recalculates
// everything from scratch inside the checkout transaction.
type Preview struct {
	Code                    string `json:"code"`
	DiscountType            string `json:"discount_type"`
	ItemsSubtotal           int64  `json:"items_subtotal"`
	DiscountAmount          int64  `json:"discount_amount"`
	DiscountedItemsSubtotal int64  `json:"discounted_items_subtotal"`
}

// CreateInput is the admin-provided shape for creating a coupon.
type CreateInput struct {
	Code           string     `json:"code"`
	DiscountType   string     `json:"discount_type"`
	Value          int64      `json:"value"`
	MinOrderAmount int64      `json:"min_order_amount"`
	UsageLimit     *int       `json:"usage_limit"`
	PerUserLimit   *int       `json:"per_user_limit"`
	StartsAt       *time.Time `json:"starts_at"`
	EndsAt         *time.Time `json:"ends_at"`
	IsActive       *bool      `json:"is_active"`
}

// UpdateInput is the admin-provided shape for updating a coupon. All
// fields are pointers so the handler can distinguish "not provided" (keep
// existing) from an explicit value; callers must still require every field
// that must always be present in this V1 API (see handler.go).
type UpdateInput struct {
	Code            *string    `json:"code"`
	DiscountType    *string    `json:"discount_type"`
	Value           *int64     `json:"value"`
	MinOrderAmount  *int64     `json:"min_order_amount"`
	UsageLimit      *int       `json:"usage_limit"`
	HasUsageLimit   bool       `json:"-"`
	PerUserLimit    *int       `json:"per_user_limit"`
	HasPerUserLimit bool       `json:"-"`
	StartsAt        *time.Time `json:"starts_at"`
	HasStartsAt     bool       `json:"-"`
	EndsAt          *time.Time `json:"ends_at"`
	HasEndsAt       bool       `json:"-"`
	IsActive        *bool      `json:"is_active"`
}

var (
	// ErrNotFound is returned when a coupon id does not exist.
	ErrNotFound = errors.New("coupon not found")
	// ErrDuplicateCode is returned when a coupon code collides
	// case-insensitively with an existing coupon.
	ErrDuplicateCode = errors.New("coupon code already exists")

	// ErrInvalid wraps a safe, user-facing validation message. Never wrap
	// a raw SQL/internal error in ErrInvalid.
	ErrInvalid = errors.New("invalid coupon")
)

// ValidationError carries a safe, specific message about why admin input
// failed validation (mirrors order.ErrValidation's shape/usage).
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }
func (e *ValidationError) Unwrap() error { return ErrInvalid }

// NormalizeCode trims surrounding whitespace from a user- or admin-
// entered coupon code. Case-insensitive comparison happens at the
// database layer (UPPER(code) index / query), so the stored/display value
// keeps the admin's original casing.
func NormalizeCode(code string) string {
	return strings.TrimSpace(code)
}

// ValidateCreateInput validates admin-provided coupon fields before
// insert. It does not touch the database (uniqueness is enforced by the
// repository via the DB constraint).
func ValidateCreateInput(in CreateInput) (CreateInput, error) {
	in.Code = NormalizeCode(in.Code)
	if in.Code == "" {
		return in, &ValidationError{Field: "code", Message: "is required"}
	}
	if len(in.Code) > 64 {
		return in, &ValidationError{Field: "code", Message: "is too long"}
	}

	if in.DiscountType != DiscountTypePercent && in.DiscountType != DiscountTypeFixed {
		return in, &ValidationError{Field: "discount_type", Message: "must be percent or fixed"}
	}

	if err := validateValue(in.DiscountType, in.Value); err != nil {
		return in, err
	}

	if in.MinOrderAmount < 0 {
		return in, &ValidationError{Field: "min_order_amount", Message: "must be >= 0"}
	}

	if in.UsageLimit != nil && *in.UsageLimit <= 0 {
		return in, &ValidationError{Field: "usage_limit", Message: "must be > 0 when provided"}
	}
	if in.PerUserLimit != nil && *in.PerUserLimit <= 0 {
		return in, &ValidationError{Field: "per_user_limit", Message: "must be > 0 when provided"}
	}

	if in.StartsAt != nil && in.EndsAt != nil && !in.StartsAt.Before(*in.EndsAt) {
		return in, &ValidationError{Field: "starts_at", Message: "must be before ends_at"}
	}

	if in.IsActive == nil {
		defaultActive := true
		in.IsActive = &defaultActive
	}

	return in, nil
}

func validateValue(discountType string, value int64) error {
	switch discountType {
	case DiscountTypePercent:
		if value < 1 || value > 100 {
			return &ValidationError{Field: "value", Message: "must be between 1 and 100 for percent coupons"}
		}
	case DiscountTypeFixed:
		if value <= 0 {
			return &ValidationError{Field: "value", Message: "must be > 0 for fixed coupons"}
		}
	}
	return nil
}

// ApplyUpdate merges a validated UpdateInput onto an existing Coupon,
// returning the resulting Coupon to persist. It performs the same field
// validation as ValidateCreateInput on the merged result.
func ApplyUpdate(existing Coupon, in UpdateInput) (Coupon, error) {
	result := existing

	if in.Code != nil {
		result.Code = NormalizeCode(*in.Code)
	}
	if in.DiscountType != nil {
		result.DiscountType = *in.DiscountType
	}
	if in.Value != nil {
		result.Value = *in.Value
	}
	if in.MinOrderAmount != nil {
		result.MinOrderAmount = *in.MinOrderAmount
	}
	if in.HasUsageLimit {
		result.UsageLimit = in.UsageLimit
	}
	if in.HasPerUserLimit {
		result.PerUserLimit = in.PerUserLimit
	}
	if in.HasStartsAt {
		result.StartsAt = in.StartsAt
	}
	if in.HasEndsAt {
		result.EndsAt = in.EndsAt
	}
	if in.IsActive != nil {
		result.IsActive = *in.IsActive
	}

	if result.Code == "" {
		return Coupon{}, &ValidationError{Field: "code", Message: "is required"}
	}
	if len(result.Code) > 64 {
		return Coupon{}, &ValidationError{Field: "code", Message: "is too long"}
	}
	if result.DiscountType != DiscountTypePercent && result.DiscountType != DiscountTypeFixed {
		return Coupon{}, &ValidationError{Field: "discount_type", Message: "must be percent or fixed"}
	}
	if err := validateValue(result.DiscountType, result.Value); err != nil {
		return Coupon{}, err
	}
	if result.MinOrderAmount < 0 {
		return Coupon{}, &ValidationError{Field: "min_order_amount", Message: "must be >= 0"}
	}
	if result.UsageLimit != nil && *result.UsageLimit <= 0 {
		return Coupon{}, &ValidationError{Field: "usage_limit", Message: "must be > 0 when provided"}
	}
	if result.PerUserLimit != nil && *result.PerUserLimit <= 0 {
		return Coupon{}, &ValidationError{Field: "per_user_limit", Message: "must be > 0 when provided"}
	}
	if result.StartsAt != nil && result.EndsAt != nil && !result.StartsAt.Before(*result.EndsAt) {
		return Coupon{}, &ValidationError{Field: "starts_at", Message: "must be before ends_at"}
	}

	return result, nil
}

// CalculateDiscount computes the discount amount for a coupon against a
// given items subtotal, using integer-only math. The discount never
// exceeds itemsSubtotal. Shipping is never discounted — callers must apply
// this only to items_subtotal and then add shipping_fee unchanged.
func CalculateDiscount(discountType string, value int64, itemsSubtotal int64) int64 {
	if itemsSubtotal <= 0 {
		return 0
	}
	var discount int64
	switch discountType {
	case DiscountTypePercent:
		// floor(itemsSubtotal * value / 100), integer division truncates
		// toward zero which is floor for non-negative operands.
		discount = (itemsSubtotal * value) / 100
	case DiscountTypeFixed:
		discount = value
	default:
		return 0
	}
	if discount > itemsSubtotal {
		discount = itemsSubtotal
	}
	if discount < 0 {
		discount = 0
	}
	return discount
}

// EligibilityError is returned by Validate (repository-level) when a
// coupon fails a business rule check (as opposed to a malformed request).
// Message is always safe to show the customer.
type EligibilityError struct {
	Code    string
	Message string
}

func (e *EligibilityError) Error() string { return e.Message }

const (
	ReasonNotFound      = "not_found"
	ReasonInactive      = "inactive"
	ReasonNotStarted    = "not_started"
	ReasonExpired       = "expired"
	ReasonMinOrder      = "min_order_not_met"
	ReasonUsageLimit    = "usage_limit_reached"
	ReasonPerUserLimit  = "per_user_limit_reached"
	ReasonInvalidConfig = "invalid_configuration"
	ReasonCodeRequired  = "code_required"
)

// CheckEligibility runs the pure (non-DB) business rules for c against
// itemsSubtotal and now. Usage-limit checks (which require DB counts) are
// performed separately by the repository inside the checkout/preview
// transaction. Returns nil if c passes every check performed here.
func CheckEligibility(c Coupon, itemsSubtotal int64, now time.Time) error {
	if !c.IsActive {
		return &EligibilityError{Code: ReasonInactive, Message: "این کد تخفیف غیرفعال است"}
	}
	if c.StartsAt != nil && now.Before(*c.StartsAt) {
		return &EligibilityError{Code: ReasonNotStarted, Message: "این کد تخفیف هنوز فعال نشده است"}
	}
	if c.EndsAt != nil && now.After(*c.EndsAt) {
		return &EligibilityError{Code: ReasonExpired, Message: "این کد تخفیف منقضی شده است"}
	}
	if itemsSubtotal < c.MinOrderAmount {
		return &EligibilityError{Code: ReasonMinOrder, Message: "مبلغ سفارش برای استفاده از این کد تخفیف کافی نیست"}
	}
	if c.DiscountType != DiscountTypePercent && c.DiscountType != DiscountTypeFixed {
		return &EligibilityError{Code: ReasonInvalidConfig, Message: "پیکربندی این کد تخفیف نامعتبر است"}
	}
	if c.DiscountType == DiscountTypePercent && (c.Value < 1 || c.Value > 100) {
		return &EligibilityError{Code: ReasonInvalidConfig, Message: "پیکربندی این کد تخفیف نامعتبر است"}
	}
	if c.DiscountType == DiscountTypeFixed && c.Value <= 0 {
		return &EligibilityError{Code: ReasonInvalidConfig, Message: "پیکربندی این کد تخفیف نامعتبر است"}
	}
	return nil
}
