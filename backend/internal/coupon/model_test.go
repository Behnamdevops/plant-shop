package coupon

import (
	"testing"
	"time"
)

// ---------- CalculateDiscount ----------

func TestCalculateDiscountPercent(t *testing.T) {
	got := CalculateDiscount(DiscountTypePercent, 20, 5_000_000)
	want := int64(1_000_000)
	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestCalculateDiscountPercentFloorsTowardZero(t *testing.T) {
	// 999 * 33 / 100 = 329.67 -> floor to 329, never rounds up.
	got := CalculateDiscount(DiscountTypePercent, 33, 999)
	want := int64(329)
	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestCalculateDiscountFixed(t *testing.T) {
	got := CalculateDiscount(DiscountTypeFixed, 300_000, 5_000_000)
	want := int64(300_000)
	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestCalculateDiscountFixedClampsToSubtotal(t *testing.T) {
	got := CalculateDiscount(DiscountTypeFixed, 10_000_000, 5_000_000)
	want := int64(5_000_000)
	if got != want {
		t.Errorf("fixed discount greater than subtotal must clamp: got %d, want %d", got, want)
	}
}

func TestCalculateDiscountPercentNeverExceedsSubtotal(t *testing.T) {
	got := CalculateDiscount(DiscountTypePercent, 100, 5_000_000)
	want := int64(5_000_000)
	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestCalculateDiscountZeroSubtotal(t *testing.T) {
	got := CalculateDiscount(DiscountTypePercent, 50, 0)
	if got != 0 {
		t.Errorf("expected 0 discount on zero subtotal, got %d", got)
	}
}

func TestCalculateDiscountUnknownTypeIsZero(t *testing.T) {
	got := CalculateDiscount("bogus", 50, 5_000_000)
	if got != 0 {
		t.Errorf("expected 0 discount for unknown discount type, got %d", got)
	}
}

// ---------- CheckEligibility ----------

func baseCoupon() Coupon {
	return Coupon{
		ID:             1,
		Code:           "SPRING20",
		DiscountType:   DiscountTypePercent,
		Value:          20,
		MinOrderAmount: 100_000,
		IsActive:       true,
	}
}

func TestCheckEligibilityValidPercentCoupon(t *testing.T) {
	c := baseCoupon()
	if err := CheckEligibility(c, 1_000_000, time.Now()); err != nil {
		t.Errorf("expected valid coupon to pass, got %v", err)
	}
}

func TestCheckEligibilityValidFixedCoupon(t *testing.T) {
	c := baseCoupon()
	c.DiscountType = DiscountTypeFixed
	c.Value = 50_000
	if err := CheckEligibility(c, 1_000_000, time.Now()); err != nil {
		t.Errorf("expected valid fixed coupon to pass, got %v", err)
	}
}

func TestCheckEligibilityInactiveCoupon(t *testing.T) {
	c := baseCoupon()
	c.IsActive = false
	err := CheckEligibility(c, 1_000_000, time.Now())
	assertEligibilityReason(t, err, ReasonInactive)
}

func TestCheckEligibilityFutureCoupon(t *testing.T) {
	c := baseCoupon()
	future := time.Now().Add(24 * time.Hour)
	c.StartsAt = &future
	err := CheckEligibility(c, 1_000_000, time.Now())
	assertEligibilityReason(t, err, ReasonNotStarted)
}

func TestCheckEligibilityExpiredCoupon(t *testing.T) {
	c := baseCoupon()
	past := time.Now().Add(-24 * time.Hour)
	c.EndsAt = &past
	err := CheckEligibility(c, 1_000_000, time.Now())
	assertEligibilityReason(t, err, ReasonExpired)
}

func TestCheckEligibilityMinimumOrderNotMet(t *testing.T) {
	c := baseCoupon()
	c.MinOrderAmount = 2_000_000
	err := CheckEligibility(c, 1_000_000, time.Now())
	assertEligibilityReason(t, err, ReasonMinOrder)
}

func TestCheckEligibilityInvalidConfigPercentOutOfRange(t *testing.T) {
	c := baseCoupon()
	c.Value = 150 // invalid but bypasses the DB CHECK constraint in this pure test
	err := CheckEligibility(c, 1_000_000, time.Now())
	assertEligibilityReason(t, err, ReasonInvalidConfig)
}

func TestCheckEligibilityInvalidConfigFixedNonPositive(t *testing.T) {
	c := baseCoupon()
	c.DiscountType = DiscountTypeFixed
	c.Value = 0
	err := CheckEligibility(c, 1_000_000, time.Now())
	assertEligibilityReason(t, err, ReasonInvalidConfig)
}

func assertEligibilityReason(t *testing.T, err error, want string) {
	t.Helper()
	eerr, ok := err.(*EligibilityError)
	if !ok || eerr == nil {
		t.Fatalf("expected *EligibilityError with reason %q, got %v", want, err)
	}
	if eerr.Code != want {
		t.Errorf("got reason %q, want %q", eerr.Code, want)
	}
}

// ---------- Code normalization / case-insensitivity ----------

func TestNormalizeCodeTrimsWhitespace(t *testing.T) {
	got := NormalizeCode("  SPRING20  ")
	if got != "SPRING20" {
		t.Errorf("got %q, want %q", got, "SPRING20")
	}
}

// ---------- ValidateCreateInput ----------

func TestValidateCreateInputRejectsEmptyCode(t *testing.T) {
	_, err := ValidateCreateInput(CreateInput{Code: "   ", DiscountType: DiscountTypePercent, Value: 10})
	if err == nil {
		t.Error("expected validation error for empty code")
	}
}

func TestValidateCreateInputRejectsInvalidPercent(t *testing.T) {
	cases := []int64{0, -1, 101, 1000}
	for _, v := range cases {
		_, err := ValidateCreateInput(CreateInput{Code: "X", DiscountType: DiscountTypePercent, Value: v})
		if err == nil {
			t.Errorf("expected validation error for percent value %d", v)
		}
	}
}

func TestValidateCreateInputRejectsInvalidFixed(t *testing.T) {
	cases := []int64{0, -100}
	for _, v := range cases {
		_, err := ValidateCreateInput(CreateInput{Code: "X", DiscountType: DiscountTypeFixed, Value: v})
		if err == nil {
			t.Errorf("expected validation error for fixed value %d", v)
		}
	}
}

func TestValidateCreateInputRejectsBadDateRange(t *testing.T) {
	start := time.Now()
	end := start.Add(-time.Hour)
	_, err := ValidateCreateInput(CreateInput{
		Code: "X", DiscountType: DiscountTypeFixed, Value: 10, StartsAt: &start, EndsAt: &end,
	})
	if err == nil {
		t.Error("expected validation error when starts_at is not before ends_at")
	}
}

func TestValidateCreateInputRejectsInvalidUsageLimits(t *testing.T) {
	zero := 0
	negative := -5
	for _, v := range []*int{&zero, &negative} {
		_, err := ValidateCreateInput(CreateInput{Code: "X", DiscountType: DiscountTypeFixed, Value: 10, UsageLimit: v})
		if err == nil {
			t.Errorf("expected validation error for usage_limit %d", *v)
		}
		_, err = ValidateCreateInput(CreateInput{Code: "X", DiscountType: DiscountTypeFixed, Value: 10, PerUserLimit: v})
		if err == nil {
			t.Errorf("expected validation error for per_user_limit %d", *v)
		}
	}
}

func TestValidateCreateInputRejectsNegativeMinOrderAmount(t *testing.T) {
	_, err := ValidateCreateInput(CreateInput{Code: "X", DiscountType: DiscountTypeFixed, Value: 10, MinOrderAmount: -1})
	if err == nil {
		t.Error("expected validation error for negative min_order_amount")
	}
}

func TestValidateCreateInputAcceptsValidPercentCoupon(t *testing.T) {
	in, err := ValidateCreateInput(CreateInput{Code: "  spring20  ", DiscountType: DiscountTypePercent, Value: 20})
	if err != nil {
		t.Fatalf("expected valid input to pass, got %v", err)
	}
	if in.Code != "spring20" {
		t.Errorf("expected trimmed code, got %q", in.Code)
	}
	if in.IsActive == nil || !*in.IsActive {
		t.Error("expected is_active to default to true")
	}
}
