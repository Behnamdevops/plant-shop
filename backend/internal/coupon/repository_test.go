package coupon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Behnamdevops/plant-shop/backend/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type testEnv struct {
	repo     *Repository
	authRepo *auth.Repository
	db       *pgxpool.Pool
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	db, err := pgxpool.New(t.Context(), dbURL)
	if err != nil {
		t.Skip("cannot connect to database; skipping integration test: ", err)
	}
	t.Cleanup(db.Close)
	return &testEnv{repo: NewRepository(db), authRepo: auth.NewRepository(db), db: db}
}

var seq int
var seqMu sync.Mutex

func uniqueSuffix() string {
	seqMu.Lock()
	defer seqMu.Unlock()
	seq++
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), seq)
}

func (e *testEnv) createUser(t *testing.T) int64 {
	t.Helper()
	suffix := uniqueSuffix()
	u, err := e.authRepo.CreateUser(context.Background(), "coupon-user-"+suffix, "coupon-user-"+suffix+"@example.com", "hash")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	t.Cleanup(func() {
		e.db.Exec(context.Background(), "DELETE FROM users WHERE id = $1", u.ID)
	})
	return u.ID
}

// createOrderStub creates a minimal orders row sufficient to satisfy the
// coupon_redemptions.order_id FK, without going through the full checkout
// flow (which lives in the order package and would create an import
// cycle). Coupon package tests only need a valid, unique order_id to
// exercise redemption bookkeeping directly.
func (e *testEnv) createOrderStub(t *testing.T, userID int64) int64 {
	t.Helper()
	var orderID int64
	err := e.db.QueryRow(context.Background(), `
		INSERT INTO orders (user_id, status, total, shipping_method, shipping_fee, items_subtotal, payment_status, payment_method)
		VALUES ($1, 'pending', 1000, 'standard', 0, 1000, 'pending', 'manual')
		RETURNING id
	`, userID).Scan(&orderID)
	if err != nil {
		t.Fatalf("seed order failed: %v", err)
	}
	t.Cleanup(func() {
		e.db.Exec(context.Background(), "DELETE FROM orders WHERE id = $1", orderID)
	})
	return orderID
}

func (e *testEnv) createCoupon(t *testing.T, in CreateInput) Coupon {
	t.Helper()
	validated, err := ValidateCreateInput(in)
	if err != nil {
		t.Fatalf("ValidateCreateInput failed: %v", err)
	}
	c, err := e.repo.Create(context.Background(), validated)
	if err != nil {
		t.Fatalf("Create coupon failed: %v", err)
	}
	t.Cleanup(func() {
		e.db.Exec(context.Background(), "DELETE FROM coupons WHERE id = $1", c.ID)
	})
	return c
}

func uniqueCode(prefix string) string {
	return prefix + "-" + uniqueSuffix()
}

// ---------- FindByCode / case-insensitivity ----------

func TestRepositoryFindByCodeUnknownCoupon(t *testing.T) {
	env := newTestEnv(t)
	_, err := env.repo.FindByCode(context.Background(), "does-not-exist-"+uniqueSuffix())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRepositoryFindByCodeCaseInsensitive(t *testing.T) {
	env := newTestEnv(t)
	code := uniqueCode("SPRING")
	env.createCoupon(t, CreateInput{Code: code, DiscountType: DiscountTypePercent, Value: 10})

	got, err := env.repo.FindByCode(context.Background(), lower(code))
	if err != nil {
		t.Fatalf("expected case-insensitive lookup to succeed, got %v", err)
	}
	if got.Code != code {
		t.Errorf("expected code %q, got %q", code, got.Code)
	}
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// ---------- Create: duplicate code (case-insensitive) -> conflict ----------

func TestRepositoryCreateDuplicateCodeCaseInsensitive(t *testing.T) {
	env := newTestEnv(t)
	code := uniqueCode("DUPE")
	env.createCoupon(t, CreateInput{Code: code, DiscountType: DiscountTypePercent, Value: 10})

	validated, err := ValidateCreateInput(CreateInput{Code: upper(code), DiscountType: DiscountTypeFixed, Value: 100})
	if err != nil {
		t.Fatalf("ValidateCreateInput failed: %v", err)
	}
	_, err = env.repo.Create(context.Background(), validated)
	if !errors.Is(err, ErrDuplicateCode) {
		t.Fatalf("expected ErrDuplicateCode, got %v", err)
	}
}

func upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}

// ---------- Eligible: end-to-end validation + usage limits ----------

func TestRepositoryEligibleValidPercentCoupon(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	code := uniqueCode("PCT")
	c := env.createCoupon(t, CreateInput{Code: code, DiscountType: DiscountTypePercent, Value: 20})

	tx, err := env.db.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	defer tx.Rollback(context.Background())

	got, discount, err := Eligible(context.Background(), tx, env.repo, code, userID, 1_000_000, time.Now())
	if err != nil {
		t.Fatalf("expected eligible coupon, got %v", err)
	}
	if got.ID != c.ID {
		t.Errorf("expected coupon id %d, got %d", c.ID, got.ID)
	}
	if discount != 200_000 {
		t.Errorf("expected discount 200000, got %d", discount)
	}
}

func TestRepositoryEligibleValidFixedCoupon(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	code := uniqueCode("FIX")
	env.createCoupon(t, CreateInput{Code: code, DiscountType: DiscountTypeFixed, Value: 150_000})

	tx, err := env.db.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	defer tx.Rollback(context.Background())

	_, discount, err := Eligible(context.Background(), tx, env.repo, code, userID, 1_000_000, time.Now())
	if err != nil {
		t.Fatalf("expected eligible coupon, got %v", err)
	}
	if discount != 150_000 {
		t.Errorf("expected discount 150000, got %d", discount)
	}
}

func TestRepositoryEligibleInactiveCoupon(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	code := uniqueCode("INACTIVE")
	active := false
	env.createCoupon(t, CreateInput{Code: code, DiscountType: DiscountTypePercent, Value: 10, IsActive: &active})

	tx, _ := env.db.Begin(context.Background())
	defer tx.Rollback(context.Background())

	_, _, err := Eligible(context.Background(), tx, env.repo, code, userID, 1_000_000, time.Now())
	assertEligibilityReason(t, err, ReasonInactive)
}

func TestRepositoryEligibleFutureCoupon(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	code := uniqueCode("FUTURE")
	starts := time.Now().Add(24 * time.Hour)
	env.createCoupon(t, CreateInput{Code: code, DiscountType: DiscountTypePercent, Value: 10, StartsAt: &starts})

	tx, _ := env.db.Begin(context.Background())
	defer tx.Rollback(context.Background())

	_, _, err := Eligible(context.Background(), tx, env.repo, code, userID, 1_000_000, time.Now())
	assertEligibilityReason(t, err, ReasonNotStarted)
}

func TestRepositoryEligibleExpiredCoupon(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	code := uniqueCode("EXPIRED")
	// Use a starts_at safely in the past so the create-time CHECK
	// (starts_at < ends_at) is satisfiable while ends_at is also in the past.
	starts := time.Now().Add(-48 * time.Hour)
	ends := time.Now().Add(-24 * time.Hour)
	env.createCoupon(t, CreateInput{Code: code, DiscountType: DiscountTypePercent, Value: 10, StartsAt: &starts, EndsAt: &ends})

	tx, _ := env.db.Begin(context.Background())
	defer tx.Rollback(context.Background())

	_, _, err := Eligible(context.Background(), tx, env.repo, code, userID, 1_000_000, time.Now())
	assertEligibilityReason(t, err, ReasonExpired)
}

func TestRepositoryEligibleMinimumOrderNotMet(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	code := uniqueCode("MINORDER")
	env.createCoupon(t, CreateInput{Code: code, DiscountType: DiscountTypePercent, Value: 10, MinOrderAmount: 2_000_000})

	tx, _ := env.db.Begin(context.Background())
	defer tx.Rollback(context.Background())

	_, _, err := Eligible(context.Background(), tx, env.repo, code, userID, 1_000_000, time.Now())
	assertEligibilityReason(t, err, ReasonMinOrder)
}

func TestRepositoryEligibleUnknownCoupon(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)

	tx, _ := env.db.Begin(context.Background())
	defer tx.Rollback(context.Background())

	_, _, err := Eligible(context.Background(), tx, env.repo, "does-not-exist-"+uniqueSuffix(), userID, 1_000_000, time.Now())
	assertEligibilityReason(t, err, ReasonNotFound)
}

func TestRepositoryEligibleCaseInsensitiveCode(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)
	code := uniqueCode("CASEY")
	env.createCoupon(t, CreateInput{Code: code, DiscountType: DiscountTypePercent, Value: 10})

	tx, _ := env.db.Begin(context.Background())
	defer tx.Rollback(context.Background())

	_, _, err := Eligible(context.Background(), tx, env.repo, upper(code), userID, 1_000_000, time.Now())
	if err != nil {
		t.Fatalf("expected case-insensitive match, got %v", err)
	}
}

func TestRepositoryEligibleCodeRequired(t *testing.T) {
	env := newTestEnv(t)
	userID := env.createUser(t)

	tx, _ := env.db.Begin(context.Background())
	defer tx.Rollback(context.Background())

	_, _, err := Eligible(context.Background(), tx, env.repo, "   ", userID, 1_000_000, time.Now())
	assertEligibilityReason(t, err, ReasonCodeRequired)
}

// ---------- Usage limits ----------

func (e *testEnv) redeem(t *testing.T, couponID, userID int64) int64 {
	t.Helper()
	orderID := e.createOrderStub(t, userID)
	tx, err := e.db.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	defer tx.Rollback(context.Background())
	if err := InsertRedemption(context.Background(), tx, couponID, userID, orderID); err != nil {
		t.Fatalf("InsertRedemption failed: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit failed: %v", err)
	}
	return orderID
}

func TestRepositoryGlobalUsageLimitReached(t *testing.T) {
	env := newTestEnv(t)
	code := uniqueCode("GLOBAL1")
	limit := 1
	c := env.createCoupon(t, CreateInput{Code: code, DiscountType: DiscountTypePercent, Value: 10, UsageLimit: &limit})

	userA := env.createUser(t)
	userB := env.createUser(t)
	env.redeem(t, c.ID, userA)

	tx, _ := env.db.Begin(context.Background())
	defer tx.Rollback(context.Background())
	_, _, err := Eligible(context.Background(), tx, env.repo, code, userB, 1_000_000, time.Now())
	assertEligibilityReason(t, err, ReasonUsageLimit)
}

func TestRepositoryPerUserLimitReached(t *testing.T) {
	env := newTestEnv(t)
	code := uniqueCode("PERUSER1")
	limit := 1
	c := env.createCoupon(t, CreateInput{Code: code, DiscountType: DiscountTypePercent, Value: 10, PerUserLimit: &limit})

	userA := env.createUser(t)
	env.redeem(t, c.ID, userA)

	tx, _ := env.db.Begin(context.Background())
	defer tx.Rollback(context.Background())
	_, _, err := Eligible(context.Background(), tx, env.repo, code, userA, 1_000_000, time.Now())
	assertEligibilityReason(t, err, ReasonPerUserLimit)
}

func TestRepositoryPerUserLimitDoesNotBlockOtherUsers(t *testing.T) {
	env := newTestEnv(t)
	code := uniqueCode("PERUSER2")
	limit := 1
	c := env.createCoupon(t, CreateInput{Code: code, DiscountType: DiscountTypePercent, Value: 10, PerUserLimit: &limit})

	userA := env.createUser(t)
	userB := env.createUser(t)
	env.redeem(t, c.ID, userA)

	tx, _ := env.db.Begin(context.Background())
	defer tx.Rollback(context.Background())
	_, _, err := Eligible(context.Background(), tx, env.repo, code, userB, 1_000_000, time.Now())
	if err != nil {
		t.Fatalf("expected userB to still be eligible, got %v", err)
	}
}

// ---------- Preview does not consume usage ----------

func TestRepositoryPreviewDoesNotConsumeUsage(t *testing.T) {
	env := newTestEnv(t)
	code := uniqueCode("PREVIEW1")
	limit := 1
	c := env.createCoupon(t, CreateInput{Code: code, DiscountType: DiscountTypePercent, Value: 10, UsageLimit: &limit})
	userA := env.createUser(t)

	for i := 0; i < 3; i++ {
		_, _, err := env.repo.EligiblePreview(context.Background(), code, userA, 1_000_000, time.Now())
		if err != nil {
			t.Fatalf("preview %d failed: %v", i, err)
		}
	}

	count, err := func() (int64, error) {
		tx, err := env.db.Begin(context.Background())
		if err != nil {
			return 0, err
		}
		defer tx.Rollback(context.Background())
		return CountActiveRedemptions(context.Background(), tx, c.ID)
	}()
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected preview to never persist a redemption, got count=%d", count)
	}
}

// ---------- Redemption release ----------

func TestRepositoryReleaseRedemptionForOrder(t *testing.T) {
	env := newTestEnv(t)
	code := uniqueCode("RELEASE1")
	c := env.createCoupon(t, CreateInput{Code: code, DiscountType: DiscountTypePercent, Value: 10})
	userA := env.createUser(t)
	orderID := env.redeem(t, c.ID, userA)

	tx, err := env.db.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	affected, err := ReleaseRedemptionForOrder(context.Background(), tx, orderID)
	if err != nil {
		t.Fatalf("ReleaseRedemptionForOrder failed: %v", err)
	}
	if affected != 1 {
		t.Errorf("expected 1 row released, got %d", affected)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	// Repeated release must be a no-op (0 rows affected), never an error
	// and never a double-release.
	tx2, err := env.db.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	defer tx2.Rollback(context.Background())
	affected2, err := ReleaseRedemptionForOrder(context.Background(), tx2, orderID)
	if err != nil {
		t.Fatalf("second ReleaseRedemptionForOrder failed: %v", err)
	}
	if affected2 != 0 {
		t.Errorf("expected repeated release to affect 0 rows, got %d", affected2)
	}
}

func TestRepositoryReleaseFreesUpUsageLimit(t *testing.T) {
	env := newTestEnv(t)
	code := uniqueCode("RELEASE2")
	limit := 1
	c := env.createCoupon(t, CreateInput{Code: code, DiscountType: DiscountTypePercent, Value: 10, UsageLimit: &limit})
	userA := env.createUser(t)
	userB := env.createUser(t)
	orderID := env.redeem(t, c.ID, userA)

	// Usage limit reached for userB while userA's redemption is active.
	tx, _ := env.db.Begin(context.Background())
	_, _, err := Eligible(context.Background(), tx, env.repo, code, userB, 1_000_000, time.Now())
	tx.Rollback(context.Background())
	assertEligibilityReason(t, err, ReasonUsageLimit)

	// Release userA's redemption.
	relTx, err := env.db.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin failed: %v", err)
	}
	if _, err := ReleaseRedemptionForOrder(context.Background(), relTx, orderID); err != nil {
		t.Fatalf("release failed: %v", err)
	}
	if err := relTx.Commit(context.Background()); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	// Now userB should be eligible again.
	tx2, _ := env.db.Begin(context.Background())
	defer tx2.Rollback(context.Background())
	_, _, err = Eligible(context.Background(), tx2, env.repo, code, userB, 1_000_000, time.Now())
	if err != nil {
		t.Fatalf("expected userB eligible after release, got %v", err)
	}
}

// ---------- Concurrency: usage_limit=1 cannot be exceeded ----------

func TestRepositoryConcurrentUsageLimitOneCannotBeExceeded(t *testing.T) {
	env := newTestEnv(t)
	code := uniqueCode("CONCURRENT1")
	limit := 1
	c := env.createCoupon(t, CreateInput{Code: code, DiscountType: DiscountTypePercent, Value: 10, UsageLimit: &limit})

	const attempts = 8
	userIDs := make([]int64, attempts)
	for i := range userIDs {
		userIDs[i] = env.createUser(t)
	}

	var wg sync.WaitGroup
	successCount := int32(0)
	var mu sync.Mutex
	errs := make([]error, attempts)

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx := context.Background()
			tx, err := env.db.Begin(ctx)
			if err != nil {
				errs[i] = err
				return
			}
			defer tx.Rollback(ctx)

			coup, _, err := Eligible(ctx, tx, env.repo, code, userIDs[i], 1_000_000, time.Now())
			if err != nil {
				errs[i] = err
				return
			}

			// Simulate order creation: insert a stub order + redemption in
			// the same transaction, exactly like order.Repository.CreateFromCart.
			var orderID int64
			if err := tx.QueryRow(ctx, `
				INSERT INTO orders (user_id, status, total, shipping_method, shipping_fee, items_subtotal, payment_status, payment_method)
				VALUES ($1, 'pending', 1000, 'standard', 0, 1000, 'pending', 'manual')
				RETURNING id
			`, userIDs[i]).Scan(&orderID); err != nil {
				errs[i] = err
				return
			}
			if err := InsertRedemption(ctx, tx, coup.ID, userIDs[i], orderID); err != nil {
				errs[i] = err
				return
			}
			if err := tx.Commit(ctx); err != nil {
				errs[i] = err
				return
			}
			mu.Lock()
			successCount++
			mu.Unlock()
			t.Cleanup(func() {
				env.db.Exec(context.Background(), "DELETE FROM coupon_redemptions WHERE order_id = $1", orderID)
				env.db.Exec(context.Background(), "DELETE FROM orders WHERE id = $1", orderID)
			})
		}(i)
	}
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful redemption with usage_limit=1 under concurrency, got %d", successCount)
	}

	finalCount, err := func() (int64, error) {
		tx, err := env.db.Begin(context.Background())
		if err != nil {
			return 0, err
		}
		defer tx.Rollback(context.Background())
		return CountActiveRedemptions(context.Background(), tx, c.ID)
	}()
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if finalCount != 1 {
		t.Fatalf("expected exactly 1 active redemption in DB, got %d", finalCount)
	}
}

// ---------- Admin CRUD / validation ----------

func TestRepositoryUpdateDuplicateCodeConflict(t *testing.T) {
	env := newTestEnv(t)
	codeA := uniqueCode("ADMINA")
	codeB := uniqueCode("ADMINB")
	env.createCoupon(t, CreateInput{Code: codeA, DiscountType: DiscountTypePercent, Value: 10})
	b := env.createCoupon(t, CreateInput{Code: codeB, DiscountType: DiscountTypePercent, Value: 10})

	updated, err := ApplyUpdate(b, UpdateInput{Code: strPtr(codeA)})
	if err != nil {
		t.Fatalf("ApplyUpdate failed: %v", err)
	}
	_, err = env.repo.Update(context.Background(), b.ID, updated)
	if !errors.Is(err, ErrDuplicateCode) {
		t.Fatalf("expected ErrDuplicateCode, got %v", err)
	}
}

func strPtr(s string) *string { return &s }

func TestRepositoryUpdateNotFound(t *testing.T) {
	env := newTestEnv(t)
	_, err := env.repo.Update(context.Background(), 999999999, Coupon{
		Code: "X", DiscountType: DiscountTypeFixed, Value: 10, IsActive: true,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRepositoryGetByIDNotFound(t *testing.T) {
	env := newTestEnv(t)
	_, err := env.repo.GetByID(context.Background(), 999999999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRepositoryListAllIncludesUsageCount(t *testing.T) {
	env := newTestEnv(t)
	code := uniqueCode("USAGECOUNT")
	c := env.createCoupon(t, CreateInput{Code: code, DiscountType: DiscountTypePercent, Value: 10})
	userA := env.createUser(t)
	env.redeem(t, c.ID, userA)

	got, err := env.repo.GetByID(context.Background(), c.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.UsageCount != 1 {
		t.Errorf("expected usage_count 1, got %d", got.UsageCount)
	}
}

