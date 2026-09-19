-- Coupons & Discounts V1: adds normalized coupons, a concurrency-safe
-- redemption ledger, and nullable order-level discount snapshot fields.
--
-- Design notes:
--
-- * `code` is stored as-entered (for admin display) but uniqueness and
--   customer lookup are case-insensitive. This is enforced with a unique
--   index on UPPER(code) rather than normalizing the stored value itself,
--   so admins see the code exactly as they typed it while "spring20" and
--   "SPRING20" are still treated as the same coupon everywhere.
--
-- * `discount_type` is either 'percent' (value is an integer 1..100) or
--   'fixed' (value is a positive IRR amount, same minor-unit convention as
--   products.price / orders.total). All monetary/percentage values are
--   BIGINT — no floating point anywhere in this feature.
--
-- * `usage_limit` / `per_user_limit` are nullable: NULL means unlimited.
--   When present they must be positive integers (enforced by CHECK and by
--   the admin handler).
--
-- * `starts_at` / `ends_at` are nullable TIMESTAMPTZ; NULL means "no lower
--   bound" / "no upper bound" respectively. When both are present,
--   starts_at must be strictly before ends_at.
--
-- * coupons are never hard-deleted in V1 (see AGENTS.md and the task
--   spec); admins deactivate via is_active = false instead, since a used
--   coupon must remain resolvable for historical order display via the
--   order-level snapshot (see below) even if the coupon row is later
--   deactivated. Nothing in this schema prevents deleting a coupon, but
--   the admin API never does so.
--
-- * orders.coupon_id intentionally has no FK NOT NULL requirement and no
--   ON DELETE restriction beyond SET NULL: historical order display must
--   never depend on the coupon row still existing. orders.coupon_code and
--   orders.discount_amount are the authoritative historical snapshot;
--   coupon_id is a best-effort convenience link only.
--
-- * Existing orders are backfilled with coupon_code = NULL and
--   discount_amount = 0 (the column default), which preserves
--   total = items_subtotal + shipping_fee for every historical order
--   exactly as before this migration — no historical totals are rewritten.
--
-- * coupon_redemptions is the concurrency-safe usage ledger. One row per
--   order that redeemed a coupon (order_id UNIQUE — at most one coupon per
--   order, enforced structurally, not just by application logic).
--   released_at is NULL while the redemption is active (counts toward
--   usage_limit / per_user_limit) and is set exactly once when the
--   originating order is cancelled through the existing safe cancellation
--   flow. An unreleased redemption for a still-pending, unpaid order
--   continues to reserve coupon usage until that order is cancelled or
--   otherwise leaves the system — this is intended V1 behavior (documented
--   further in the order/coupon package).
CREATE TABLE coupons (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL,
    discount_type TEXT NOT NULL CHECK (discount_type IN ('percent', 'fixed')),
    value BIGINT NOT NULL,
    min_order_amount BIGINT NOT NULL DEFAULT 0 CHECK (min_order_amount >= 0),
    usage_limit INTEGER NULL CHECK (usage_limit IS NULL OR usage_limit > 0),
    per_user_limit INTEGER NULL CHECK (per_user_limit IS NULL OR per_user_limit > 0),
    starts_at TIMESTAMPTZ NULL,
    ends_at TIMESTAMPTZ NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT coupons_code_not_blank CHECK (length(trim(code)) > 0),
    CONSTRAINT coupons_date_range_check CHECK (starts_at IS NULL OR ends_at IS NULL OR starts_at < ends_at),
    CONSTRAINT coupons_percent_range_check CHECK (
        discount_type <> 'percent' OR (value >= 1 AND value <= 100)
    ),
    CONSTRAINT coupons_fixed_positive_check CHECK (
        discount_type <> 'fixed' OR value > 0
    )
);

-- Case-insensitive uniqueness on the trimmed code. Also the lookup path
-- used by coupon validation (WHERE UPPER(code) = UPPER($1)), so this index
-- serves both correctness and performance.
CREATE UNIQUE INDEX idx_coupons_code_upper ON coupons (UPPER(code));

-- Supports the admin "active coupons" listing/filtering; is_active alone is
-- low-cardinality but combined with the code lookup path above this keeps
-- the common "is this active coupon still usable" check index-friendly.
CREATE INDEX idx_coupons_is_active ON coupons (is_active);

CREATE TABLE coupon_redemptions (
    id BIGSERIAL PRIMARY KEY,
    coupon_id BIGINT NOT NULL REFERENCES coupons(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    order_id BIGINT NOT NULL UNIQUE REFERENCES orders(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    released_at TIMESTAMPTZ NULL
);

-- Supports the two race-safe usage checks performed inside the checkout
-- transaction: global usage (COUNT WHERE coupon_id = $1 AND released_at IS
-- NULL) and per-user usage (same, plus user_id = $2). Partial index scoped
-- to unreleased rows only, since only those count toward limits.
CREATE INDEX idx_coupon_redemptions_active_by_coupon
    ON coupon_redemptions (coupon_id) WHERE released_at IS NULL;
CREATE INDEX idx_coupon_redemptions_active_by_coupon_user
    ON coupon_redemptions (coupon_id, user_id) WHERE released_at IS NULL;

ALTER TABLE orders
    ADD COLUMN coupon_id BIGINT NULL REFERENCES coupons(id) ON DELETE SET NULL,
    ADD COLUMN coupon_code TEXT NULL,
    ADD COLUMN discount_amount BIGINT NOT NULL DEFAULT 0 CHECK (discount_amount >= 0);
