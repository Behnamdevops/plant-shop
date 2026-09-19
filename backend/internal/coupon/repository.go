package coupon

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

const couponColumns = `
	id, code, discount_type, value, min_order_amount, usage_limit, per_user_limit,
	starts_at, ends_at, is_active, created_at, updated_at
`

func scanCoupon(row pgx.Row, c *Coupon) error {
	return row.Scan(
		&c.ID, &c.Code, &c.DiscountType, &c.Value, &c.MinOrderAmount, &c.UsageLimit, &c.PerUserLimit,
		&c.StartsAt, &c.EndsAt, &c.IsActive, &c.CreatedAt, &c.UpdatedAt,
	)
}

// FindByCode looks up a coupon by its case-insensitively normalized code
// (no row lock; used by the advisory preview endpoint). Returns
// ErrNotFound if no coupon matches.
func (r *Repository) FindByCode(ctx context.Context, code string) (Coupon, error) {
	var c Coupon
	err := scanCoupon(r.db.QueryRow(ctx, `
		SELECT `+couponColumns+`
		FROM coupons
		WHERE UPPER(code) = UPPER($1)
	`, code), &c)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Coupon{}, ErrNotFound
		}
		return Coupon{}, err
	}
	return c, nil
}

// FindByCodeForUpdate looks up a coupon by its case-insensitively
// normalized code and locks the row (FOR UPDATE) within tx. Callers must
// already be inside a transaction (order creation). Locking the coupon row
// here, combined with re-counting redemptions inside the same transaction,
// is what makes usage_limit / per_user_limit race-safe: two concurrent
// checkouts against the same coupon serialize on this lock, so the second
// transaction always sees the first transaction's committed redemption
// count before deciding whether the limit still permits it.
func (r *Repository) FindByCodeForUpdate(ctx context.Context, tx pgx.Tx, code string) (Coupon, error) {
	var c Coupon
	err := scanCoupon(tx.QueryRow(ctx, `
		SELECT `+couponColumns+`
		FROM coupons
		WHERE UPPER(code) = UPPER($1)
		FOR UPDATE
	`, code), &c)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Coupon{}, ErrNotFound
		}
		return Coupon{}, err
	}
	return c, nil
}

// CountActiveRedemptions returns the number of unreleased redemptions for
// couponID, i.e. how many currently count toward its global usage_limit.
// Must be called after FindByCodeForUpdate's lock has been acquired inside
// the same transaction to get a race-safe count.
func CountActiveRedemptions(ctx context.Context, tx pgx.Tx, couponID int64) (int64, error) {
	var count int64
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM coupon_redemptions
		WHERE coupon_id = $1 AND released_at IS NULL
	`, couponID).Scan(&count)
	return count, err
}

// CountActiveRedemptionsForUser returns the number of unreleased
// redemptions for couponID belonging to userID, i.e. how many currently
// count toward its per_user_limit.
func CountActiveRedemptionsForUser(ctx context.Context, tx pgx.Tx, couponID, userID int64) (int64, error) {
	var count int64
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM coupon_redemptions
		WHERE coupon_id = $1 AND user_id = $2 AND released_at IS NULL
	`, couponID, userID).Scan(&count)
	return count, err
}

// InsertRedemption records a new active redemption for orderID inside tx.
// Must be called in the same transaction as order creation so the
// redemption and the order are committed (or rolled back) atomically.
// order_id is UNIQUE, so this also structurally enforces "at most one
// coupon per order".
func InsertRedemption(ctx context.Context, tx pgx.Tx, couponID, userID, orderID int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO coupon_redemptions (coupon_id, user_id, order_id)
		VALUES ($1, $2, $3)
	`, couponID, userID, orderID)
	return err
}

// ReleaseRedemptionForOrder marks orderID's redemption (if any) as
// released, but only if it is not already released. Must be called inside
// the same transaction as order cancellation / stock restoration. Returns
// the number of rows updated (0 or 1) so callers can distinguish "no
// coupon was used on this order" / "already released" from "released just
// now" without a separate SELECT, and so repeated cancellation attempts
// (which reach this only once per successful cancellation, but might be
// called defensively) never double-release.
func ReleaseRedemptionForOrder(ctx context.Context, tx pgx.Tx, orderID int64) (int64, error) {
	tag, err := tx.Exec(ctx, `
		UPDATE coupon_redemptions
		SET released_at = NOW()
		WHERE order_id = $1 AND released_at IS NULL
	`, orderID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// Eligible resolves and fully validates a coupon code for use against
// itemsSubtotal by userID, including the race-safe usage checks, inside
// tx. It is the single entry point used by both order creation (inside the
// checkout transaction) and, for a lighter-weight non-locking variant, the
// preview endpoint (see EligiblePreview). Returns the loaded Coupon and the
// integer discount amount on success.
func Eligible(ctx context.Context, tx pgx.Tx, repo *Repository, code string, userID, itemsSubtotal int64, now time.Time) (Coupon, int64, error) {
	code = NormalizeCode(code)
	if code == "" {
		return Coupon{}, 0, &EligibilityError{Code: ReasonCodeRequired, Message: "کد تخفیف الزامی است"}
	}

	c, err := repo.FindByCodeForUpdate(ctx, tx, code)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Coupon{}, 0, &EligibilityError{Code: ReasonNotFound, Message: "کد تخفیف یافت نشد"}
		}
		return Coupon{}, 0, err
	}

	if err := CheckEligibility(c, itemsSubtotal, now); err != nil {
		return Coupon{}, 0, err
	}

	if c.UsageLimit != nil {
		count, err := CountActiveRedemptions(ctx, tx, c.ID)
		if err != nil {
			return Coupon{}, 0, err
		}
		if count >= int64(*c.UsageLimit) {
			return Coupon{}, 0, &EligibilityError{Code: ReasonUsageLimit, Message: "ظرفیت استفاده از این کد تخفیف به پایان رسیده است"}
		}
	}
	if c.PerUserLimit != nil {
		count, err := CountActiveRedemptionsForUser(ctx, tx, c.ID, userID)
		if err != nil {
			return Coupon{}, 0, err
		}
		if count >= int64(*c.PerUserLimit) {
			return Coupon{}, 0, &EligibilityError{Code: ReasonPerUserLimit, Message: "شما قبلاً از این کد تخفیف استفاده کرده‌اید"}
		}
	}

	discount := CalculateDiscount(c.DiscountType, c.Value, itemsSubtotal)
	return c, discount, nil
}

// EligiblePreview resolves and validates a coupon code for the advisory
// preview endpoint. It performs the same eligibility and usage-limit
// checks as Eligible but WITHOUT taking a row lock and WITHOUT running
// inside the caller's write transaction, since preview never persists a
// redemption. Usage counts here are read using a plain (unlocked) query,
// so they are only a best-effort snapshot — order creation is the sole
// authority and always re-validates transactionally.
func (r *Repository) EligiblePreview(ctx context.Context, code string, userID, itemsSubtotal int64, now time.Time) (Coupon, int64, error) {
	code = NormalizeCode(code)
	if code == "" {
		return Coupon{}, 0, &EligibilityError{Code: ReasonCodeRequired, Message: "کد تخفیف الزامی است"}
	}

	c, err := r.FindByCode(ctx, code)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Coupon{}, 0, &EligibilityError{Code: ReasonNotFound, Message: "کد تخفیف یافت نشد"}
		}
		return Coupon{}, 0, err
	}

	if err := CheckEligibility(c, itemsSubtotal, now); err != nil {
		return Coupon{}, 0, err
	}

	if c.UsageLimit != nil {
		var count int64
		err := r.db.QueryRow(ctx, `
			SELECT COUNT(*) FROM coupon_redemptions WHERE coupon_id = $1 AND released_at IS NULL
		`, c.ID).Scan(&count)
		if err != nil {
			return Coupon{}, 0, err
		}
		if count >= int64(*c.UsageLimit) {
			return Coupon{}, 0, &EligibilityError{Code: ReasonUsageLimit, Message: "ظرفیت استفاده از این کد تخفیف به پایان رسیده است"}
		}
	}
	if c.PerUserLimit != nil {
		var count int64
		err := r.db.QueryRow(ctx, `
			SELECT COUNT(*) FROM coupon_redemptions WHERE coupon_id = $1 AND user_id = $2 AND released_at IS NULL
		`, c.ID, userID).Scan(&count)
		if err != nil {
			return Coupon{}, 0, err
		}
		if count >= int64(*c.PerUserLimit) {
			return Coupon{}, 0, &EligibilityError{Code: ReasonPerUserLimit, Message: "شما قبلاً از این کد تخفیف استفاده کرده‌اید"}
		}
	}

	discount := CalculateDiscount(c.DiscountType, c.Value, itemsSubtotal)
	return c, discount, nil
}

// --- Admin CRUD ---

// usageCountSubquery computes each coupon's active (unreleased) redemption
// count for the admin list/detail views.
const adminCouponColumns = `
	c.id, c.code, c.discount_type, c.value, c.min_order_amount, c.usage_limit, c.per_user_limit,
	c.starts_at, c.ends_at, c.is_active, c.created_at, c.updated_at,
	COALESCE((SELECT COUNT(*) FROM coupon_redemptions cr WHERE cr.coupon_id = c.id AND cr.released_at IS NULL), 0)
`

func scanAdminCoupon(row pgx.Row, c *AdminCoupon) error {
	return row.Scan(
		&c.ID, &c.Code, &c.DiscountType, &c.Value, &c.MinOrderAmount, &c.UsageLimit, &c.PerUserLimit,
		&c.StartsAt, &c.EndsAt, &c.IsActive, &c.CreatedAt, &c.UpdatedAt, &c.UsageCount,
	)
}

// ListAll returns every coupon, newest first, for admin management.
func (r *Repository) ListAll(ctx context.Context) ([]AdminCoupon, error) {
	rows, err := r.db.Query(ctx, `
		SELECT `+adminCouponColumns+`
		FROM coupons c
		ORDER BY c.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	coupons := make([]AdminCoupon, 0)
	for rows.Next() {
		var c AdminCoupon
		if err := scanAdminCoupon(rows, &c); err != nil {
			return nil, err
		}
		coupons = append(coupons, c)
	}
	return coupons, rows.Err()
}

// GetByID returns a single coupon (with usage count) for admin management.
func (r *Repository) GetByID(ctx context.Context, id int64) (AdminCoupon, error) {
	var c AdminCoupon
	err := scanAdminCoupon(r.db.QueryRow(ctx, `
		SELECT `+adminCouponColumns+`
		FROM coupons c
		WHERE c.id = $1
	`, id), &c)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AdminCoupon{}, ErrNotFound
		}
		return AdminCoupon{}, err
	}
	return c, nil
}

// isUniqueViolation reports whether err is a Postgres unique_violation
// (23505), used here to translate the coupons_code_upper unique index
// violation into ErrDuplicateCode.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// Create inserts a new coupon. Callers must have already validated input
// via ValidateCreateInput. Returns ErrDuplicateCode if the (trimmed,
// case-insensitive) code already exists.
func (r *Repository) Create(ctx context.Context, in CreateInput) (Coupon, error) {
	var c Coupon
	err := scanCoupon(r.db.QueryRow(ctx, `
		INSERT INTO coupons (code, discount_type, value, min_order_amount, usage_limit, per_user_limit, starts_at, ends_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+couponColumns+`
	`,
		in.Code, in.DiscountType, in.Value, in.MinOrderAmount, in.UsageLimit, in.PerUserLimit, in.StartsAt, in.EndsAt, *in.IsActive,
	), &c)
	if err != nil {
		if isUniqueViolation(err) {
			return Coupon{}, ErrDuplicateCode
		}
		return Coupon{}, err
	}
	return c, nil
}

// Update persists the full new state of a coupon (as computed by
// ApplyUpdate). Returns ErrNotFound if id does not exist, or
// ErrDuplicateCode if the new code collides with a different coupon.
func (r *Repository) Update(ctx context.Context, id int64, updated Coupon) (Coupon, error) {
	var c Coupon
	err := scanCoupon(r.db.QueryRow(ctx, `
		UPDATE coupons
		SET code = $1, discount_type = $2, value = $3, min_order_amount = $4,
			usage_limit = $5, per_user_limit = $6, starts_at = $7, ends_at = $8,
			is_active = $9, updated_at = NOW()
		WHERE id = $10
		RETURNING `+couponColumns+`
	`,
		updated.Code, updated.DiscountType, updated.Value, updated.MinOrderAmount,
		updated.UsageLimit, updated.PerUserLimit, updated.StartsAt, updated.EndsAt,
		updated.IsActive, id,
	), &c)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Coupon{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return Coupon{}, ErrDuplicateCode
		}
		return Coupon{}, err
	}
	return c, nil
}
