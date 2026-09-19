package order

import (
	"context"
	"errors"
	"testing"

	"github.com/Behnamdevops/plant-shop/backend/internal/coupon"
)

// couponTestEnv extends testEnv with a coupon repository so these tests can
// seed coupons directly against the same database.
type couponTestEnv struct {
	*testEnv
	coupons *coupon.Repository
}

func newCouponTestEnv(t *testing.T) *couponTestEnv {
	t.Helper()
	env := newTestEnv(t)
	return &couponTestEnv{testEnv: env, coupons: coupon.NewRepository(env.db)}
}

func (e *couponTestEnv) createCoupon(t *testing.T, in coupon.CreateInput) coupon.Coupon {
	t.Helper()
	validated, err := coupon.ValidateCreateInput(in)
	if err != nil {
		t.Fatalf("ValidateCreateInput failed: %v", err)
	}
	c, err := e.coupons.Create(context.Background(), validated)
	if err != nil {
		t.Fatalf("create coupon failed: %v", err)
	}
	t.Cleanup(func() {
		e.db.Exec(context.Background(), "DELETE FROM coupons WHERE id = $1", c.ID)
	})
	return c
}

func boolPtr(b bool) *bool { return &b }

// ---------- Order creation: no coupon preserves existing behavior ----------

func TestCreateFromCartWithoutCouponPreservesExistingTotals(t *testing.T) {
	env := newCouponTestEnv(t)
	userID := env.createUser(t)
	p := env.createProduct(t, 10)
	if _, err := env.cartRepo.AddItem(context.Background(), userID, p.ID, 2); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	o, err := env.orderRepo.CreateFromCart(context.Background(), userID, validCheckoutInput())
	if err != nil {
		t.Fatalf("CreateFromCart failed: %v", err)
	}
	if o.CouponCode != nil {
		t.Errorf("expected nil coupon_code, got %v", *o.CouponCode)
	}
	if o.DiscountAmount != 0 {
		t.Errorf("expected discount_amount 0, got %d", o.DiscountAmount)
	}
	wantSubtotal := p.Price * 2
	wantTotal := wantSubtotal + ShippingFeeStandard
	if o.Total != wantTotal {
		t.Errorf("expected total %d, got %d", wantTotal, o.Total)
	}
}

// ---------- Order creation: coupon snapshots code + discount amount ----------

func TestCreateFromCartWithCouponSnapshotsCodeAndDiscount(t *testing.T) {
	env := newCouponTestEnv(t)
	userID := env.createUser(t)
	p := env.createProduct(t, 10) // price = 100 (see testEnv.createProduct)
	if _, err := env.cartRepo.AddItem(context.Background(), userID, p.ID, 10); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	// itemsSubtotal = 100 * 10 = 1000

	code := "ORDERCOUPON-" + uniqueSuffix()
	env.createCoupon(t, coupon.CreateInput{
		Code: code, DiscountType: coupon.DiscountTypePercent, Value: 20, IsActive: boolPtr(true),
	})

	input := validCheckoutInput()
	input.CouponCode = code

	o, err := env.orderRepo.CreateFromCart(context.Background(), userID, input)
	if err != nil {
		t.Fatalf("CreateFromCart failed: %v", err)
	}
	if o.CouponCode == nil || *o.CouponCode != code {
		t.Errorf("expected coupon_code %q, got %v", code, o.CouponCode)
	}
	wantDiscount := int64(200) // floor(1000 * 20 / 100)
	if o.DiscountAmount != wantDiscount {
		t.Errorf("expected discount_amount %d, got %d", wantDiscount, o.DiscountAmount)
	}
	wantTotal := (int64(1000) - wantDiscount) + ShippingFeeStandard
	if o.Total != wantTotal {
		t.Errorf("expected total %d, got %d", wantTotal, o.Total)
	}
	if o.ItemsSubtotal != 1000 {
		t.Errorf("expected items_subtotal to remain the pre-discount 1000, got %d", o.ItemsSubtotal)
	}
}

// ---------- Shipping is never discounted ----------

func TestCreateFromCartCouponNeverDiscountsShipping(t *testing.T) {
	env := newCouponTestEnv(t)
	userID := env.createUser(t)
	p := env.createProduct(t, 10)
	if _, err := env.cartRepo.AddItem(context.Background(), userID, p.ID, 10); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	code := "SHIPCOUPON-" + uniqueSuffix()
	// 100% off items — even so, shipping_fee must be charged in full.
	env.createCoupon(t, coupon.CreateInput{
		Code: code, DiscountType: coupon.DiscountTypePercent, Value: 100, IsActive: boolPtr(true),
	})

	input := validCheckoutInput()
	input.CouponCode = code
	input.ShippingMethod = ShippingMethodExpress

	o, err := env.orderRepo.CreateFromCart(context.Background(), userID, input)
	if err != nil {
		t.Fatalf("CreateFromCart failed: %v", err)
	}
	if o.ShippingFee != ShippingFeeExpress {
		t.Errorf("expected full shipping fee %d, got %d", ShippingFeeExpress, o.ShippingFee)
	}
	if o.DiscountAmount != 1000 {
		t.Errorf("expected discount_amount 1000 (full items subtotal), got %d", o.DiscountAmount)
	}
	if o.Total != ShippingFeeExpress {
		t.Errorf("expected total to equal shipping fee only, got %d", o.Total)
	}
}

// ---------- Fixed discount greater than subtotal clamps ----------

func TestCreateFromCartFixedDiscountClampsToSubtotal(t *testing.T) {
	env := newCouponTestEnv(t)
	userID := env.createUser(t)
	p := env.createProduct(t, 10)
	if _, err := env.cartRepo.AddItem(context.Background(), userID, p.ID, 1); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	// itemsSubtotal = 100

	code := "BIGFIXED-" + uniqueSuffix()
	env.createCoupon(t, coupon.CreateInput{
		Code: code, DiscountType: coupon.DiscountTypeFixed, Value: 999_999, IsActive: boolPtr(true),
	})

	input := validCheckoutInput()
	input.CouponCode = code

	o, err := env.orderRepo.CreateFromCart(context.Background(), userID, input)
	if err != nil {
		t.Fatalf("CreateFromCart failed: %v", err)
	}
	if o.DiscountAmount != 100 {
		t.Errorf("expected discount clamped to items_subtotal 100, got %d", o.DiscountAmount)
	}
	if o.Total != ShippingFeeStandard {
		t.Errorf("expected total to equal shipping fee only (fully discounted items), got %d", o.Total)
	}
}

// ---------- Invalid coupon at checkout time is rejected, order not created ----------

func TestCreateFromCartRejectsInvalidCoupon(t *testing.T) {
	env := newCouponTestEnv(t)
	userID := env.createUser(t)
	p := env.createProduct(t, 10)
	if _, err := env.cartRepo.AddItem(context.Background(), userID, p.ID, 1); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	input := validCheckoutInput()
	input.CouponCode = "does-not-exist-" + uniqueSuffix()

	_, err := env.orderRepo.CreateFromCart(context.Background(), userID, input)
	var cerr *ErrCouponEligibility
	if !errors.As(err, &cerr) {
		t.Fatalf("expected *ErrCouponEligibility, got %v", err)
	}
	if !errors.Is(err, ErrCouponInvalid) {
		t.Error("expected errors.Is(err, ErrCouponInvalid) to be true")
	}

	// The order must not have been created.
	orders, err := env.orderRepo.ListByUser(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListByUser failed: %v", err)
	}
	if len(orders) != 0 {
		t.Fatalf("expected no order to be created when coupon is invalid, got %d", len(orders))
	}

	// The cart must remain intact so the customer can retry without the
	// coupon.
	c, err := env.cartRepo.GetCart(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetCart failed: %v", err)
	}
	if len(c.Items) != 1 {
		t.Fatalf("expected cart to remain intact, got %d items", len(c.Items))
	}
}

// ---------- One redemption is created per successful order ----------

func TestCreateFromCartCreatesExactlyOneRedemption(t *testing.T) {
	env := newCouponTestEnv(t)
	userID := env.createUser(t)
	p := env.createProduct(t, 10)
	if _, err := env.cartRepo.AddItem(context.Background(), userID, p.ID, 1); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	code := "SINGLEREDEEM-" + uniqueSuffix()
	c := env.createCoupon(t, coupon.CreateInput{
		Code: code, DiscountType: coupon.DiscountTypePercent, Value: 10, IsActive: boolPtr(true),
	})

	input := validCheckoutInput()
	input.CouponCode = code
	o, err := env.orderRepo.CreateFromCart(context.Background(), userID, input)
	if err != nil {
		t.Fatalf("CreateFromCart failed: %v", err)
	}

	var count int64
	err = env.db.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM coupon_redemptions WHERE order_id = $1 AND coupon_id = $2 AND released_at IS NULL
	`, o.ID, c.ID).Scan(&count)
	if err != nil {
		t.Fatalf("query redemption count failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 active redemption, got %d", count)
	}
}

// ---------- Usage limit enforced during checkout ----------

func TestCreateFromCartEnforcesGlobalUsageLimit(t *testing.T) {
	env := newCouponTestEnv(t)
	limit := 1
	code := "CHECKOUTLIMIT-" + uniqueSuffix()
	env.createCoupon(t, coupon.CreateInput{
		Code: code, DiscountType: coupon.DiscountTypePercent, Value: 10, UsageLimit: &limit, IsActive: boolPtr(true),
	})

	userA := env.createUser(t)
	pA := env.createProduct(t, 10)
	if _, err := env.cartRepo.AddItem(context.Background(), userA, pA.ID, 1); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	inputA := validCheckoutInput()
	inputA.CouponCode = code
	if _, err := env.orderRepo.CreateFromCart(context.Background(), userA, inputA); err != nil {
		t.Fatalf("first checkout with coupon failed: %v", err)
	}

	userB := env.createUser(t)
	pB := env.createProduct(t, 10)
	if _, err := env.cartRepo.AddItem(context.Background(), userB, pB.ID, 1); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	inputB := validCheckoutInput()
	inputB.CouponCode = code
	_, err := env.orderRepo.CreateFromCart(context.Background(), userB, inputB)
	var cerr *ErrCouponEligibility
	if !errors.As(err, &cerr) {
		t.Fatalf("expected second checkout to be rejected by usage_limit, got %v", err)
	}

	// userB's order must not exist, and their cart must remain intact.
	c, err := env.cartRepo.GetCart(context.Background(), userB)
	if err != nil {
		t.Fatalf("GetCart failed: %v", err)
	}
	if len(c.Items) != 1 {
		t.Fatalf("expected userB's cart to remain intact, got %d items", len(c.Items))
	}
}

// ---------- Cancellation releases redemption exactly once ----------

func TestCancelOwnOrderReleasesRedemptionExactlyOnce(t *testing.T) {
	env := newCouponTestEnv(t)
	userID := env.createUser(t)
	p := env.createProduct(t, 10)
	if _, err := env.cartRepo.AddItem(context.Background(), userID, p.ID, 1); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	code := "CANCELRELEASE-" + uniqueSuffix()
	c := env.createCoupon(t, coupon.CreateInput{
		Code: code, DiscountType: coupon.DiscountTypePercent, Value: 10, IsActive: boolPtr(true),
	})
	input := validCheckoutInput()
	input.CouponCode = code
	o, err := env.orderRepo.CreateFromCart(context.Background(), userID, input)
	if err != nil {
		t.Fatalf("CreateFromCart failed: %v", err)
	}

	if _, err := env.orderRepo.CancelOwnOrder(context.Background(), userID, o.ID); err != nil {
		t.Fatalf("CancelOwnOrder failed: %v", err)
	}

	var releasedCount int64
	err = env.db.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM coupon_redemptions WHERE order_id = $1 AND released_at IS NOT NULL
	`, o.ID).Scan(&releasedCount)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if releasedCount != 1 {
		t.Fatalf("expected redemption to be released exactly once, got %d released rows", releasedCount)
	}

	// A repeated cancellation attempt must fail (already cancelled is
	// terminal) and must not touch the release timestamp again.
	var releasedAtFirst string
	env.db.QueryRow(context.Background(), `SELECT released_at::text FROM coupon_redemptions WHERE order_id = $1`, o.ID).Scan(&releasedAtFirst)

	_, err = env.orderRepo.CancelOwnOrder(context.Background(), userID, o.ID)
	if !errors.Is(err, ErrInvalidTransition) && !errors.Is(err, ErrOrderNotEligibleForCancel) {
		t.Fatalf("expected repeated cancellation to fail, got %v", err)
	}

	var releasedAtSecond string
	env.db.QueryRow(context.Background(), `SELECT released_at::text FROM coupon_redemptions WHERE order_id = $1`, o.ID).Scan(&releasedAtSecond)
	if releasedAtFirst != releasedAtSecond {
		t.Fatalf("released_at changed on repeated cancellation attempt: %q -> %q", releasedAtFirst, releasedAtSecond)
	}

	// The released redemption must free up global usage again.
	userB := env.createUser(t)
	tx, err := env.db.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	defer tx.Rollback(context.Background())
	count, err := coupon.CountActiveRedemptions(context.Background(), tx, c.ID)
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 active redemptions after release, got %d", count)
	}
	_ = userB
}

// ---------- Failed cancellation does not release ----------

func TestFailedCancellationDoesNotReleaseRedemption(t *testing.T) {
	env := newCouponTestEnv(t)
	userID := env.createUser(t)
	p := env.createProduct(t, 10)
	if _, err := env.cartRepo.AddItem(context.Background(), userID, p.ID, 1); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}

	code := "PAIDNORELEASE-" + uniqueSuffix()
	env.createCoupon(t, coupon.CreateInput{
		Code: code, DiscountType: coupon.DiscountTypePercent, Value: 10, IsActive: boolPtr(true),
	})
	input := validCheckoutInput()
	input.CouponCode = code
	o, err := env.orderRepo.CreateFromCart(context.Background(), userID, input)
	if err != nil {
		t.Fatalf("CreateFromCart failed: %v", err)
	}

	// Mark the order as paid — self-service cancellation must now be
	// rejected, and the coupon redemption must remain active.
	if _, err := env.db.Exec(context.Background(), `UPDATE orders SET payment_status = $1 WHERE id = $2`, PaymentStatusPaid, o.ID); err != nil {
		t.Fatalf("failed to mark order paid: %v", err)
	}

	_, err = env.orderRepo.CancelOwnOrder(context.Background(), userID, o.ID)
	if !errors.Is(err, ErrOrderNotEligibleForCancel) {
		t.Fatalf("expected ErrOrderNotEligibleForCancel for paid order, got %v", err)
	}

	var releasedCount int64
	err = env.db.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM coupon_redemptions WHERE order_id = $1 AND released_at IS NOT NULL
	`, o.ID).Scan(&releasedCount)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if releasedCount != 0 {
		t.Fatalf("expected redemption to remain active after rejected cancellation, got %d released", releasedCount)
	}
}

// ---------- Old/non-coupon order compatibility ----------

func TestPreExistingOrderWithoutCouponFieldsDisplaysNormally(t *testing.T) {
	env := newCouponTestEnv(t)
	userID := env.createUser(t)
	p := env.createProduct(t, 10)
	if _, err := env.cartRepo.AddItem(context.Background(), userID, p.ID, 1); err != nil {
		t.Fatalf("AddItem failed: %v", err)
	}
	o, err := env.orderRepo.CreateFromCart(context.Background(), userID, validCheckoutInput())
	if err != nil {
		t.Fatalf("CreateFromCart failed: %v", err)
	}

	// Simulate a legacy row: explicitly NULL/0 coupon fields (the migration
	// default for pre-existing rows).
	if _, err := env.db.Exec(context.Background(), `
		UPDATE orders SET coupon_id = NULL, coupon_code = NULL, discount_amount = 0 WHERE id = $1
	`, o.ID); err != nil {
		t.Fatalf("simulate legacy row failed: %v", err)
	}

	full, err := env.orderRepo.GetByIDForUser(context.Background(), userID, o.ID)
	if err != nil {
		t.Fatalf("GetByIDForUser failed: %v", err)
	}
	if full.CouponCode != nil {
		t.Errorf("expected nil coupon_code for legacy order, got %v", *full.CouponCode)
	}
	if full.DiscountAmount != 0 {
		t.Errorf("expected discount_amount 0 for legacy order, got %d", full.DiscountAmount)
	}
	if full.Total != full.ItemsSubtotal+full.ShippingFee {
		t.Errorf("expected total = items_subtotal + shipping_fee for legacy order, got total=%d subtotal=%d shipping=%d",
			full.Total, full.ItemsSubtotal, full.ShippingFee)
	}
}
