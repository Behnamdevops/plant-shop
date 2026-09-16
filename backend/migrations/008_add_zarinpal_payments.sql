-- ZarinPal Payment V1: allows orders.payment_method to record 'zarinpal' in
-- addition to the existing 'manual' placeholder, and introduces a dedicated
-- payment_attempts table to track (possibly multiple, retried) ZarinPal
-- payment attempts per order.
--
-- Design notes:
--
-- * A dedicated payment_attempts table (rather than gateway-specific columns
--   bolted onto orders) is used because a single order may have multiple
--   payment attempts over time (e.g. the customer abandons the gateway or a
--   verification fails, then retries). Putting retry state on orders itself
--   would require either overwriting history on every retry or an
--   unbounded number of nullable columns.
--
-- * `authority` is UNIQUE but nullable: it is only known once ZarinPal's
--   payment/request.json call succeeds. Postgres treats multiple NULLs as
--   distinct for a UNIQUE constraint, so several attempts for the same (or
--   different) orders can sit with authority = NULL simultaneously (e.g.
--   between "attempt created" and "provider responded", or if the request
--   call itself failed) without violating uniqueness. Once an authority is
--   assigned, it can never be reused across orders — the row itself is the
--   only place linking an authority back to an order_id, so the callback
--   handler can never be tricked into crediting the wrong order.
--
-- * `amount` is a snapshot of the order's total (in the same canonical
--   minor-unit/Rial column type as orders.total) at the time the attempt
--   was created, not read live from orders again during verification. This
--   guards against the order's total ever changing between request and
--   verify (it currently never does post-checkout, but snapshotting is the
--   safer invariant to hold regardless).
--
-- * `status` is intentionally a smaller state machine than orders: pending
--   (created, awaiting provider response/callback), paid (verified
--   successfully — including ZarinPal's "already verified" code, handled
--   idempotently by the application layer), failed (provider rejected the
--   request/verification, or the callback reported a non-OK status). There
--   is no "refunded" here; refunds are out of scope for V1 and remain an
--   order-level concern (orders.payment_status already supports
--   'refunded').
--
-- * `ref_id` and `provider_code` are nullable and only ever populated by the
--   verify step; no card/bank details are ever stored (ZarinPal's response
--   may include a masked card_pan/card_hash, which this schema deliberately
--   has no column for, per the "no sensitive card/bank information"
--   requirement).
ALTER TABLE orders
    DROP CONSTRAINT orders_payment_method_check,
    ADD CONSTRAINT orders_payment_method_check
        CHECK (payment_method IN ('manual', 'zarinpal'));

CREATE TABLE payment_attempts (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    provider VARCHAR(20) NOT NULL DEFAULT 'zarinpal'
        CHECK (provider IN ('zarinpal')),
    authority VARCHAR(64) UNIQUE,
    amount BIGINT NOT NULL CHECK (amount >= 0),
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'paid', 'failed')),
    ref_id BIGINT,
    provider_code INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    verified_at TIMESTAMPTZ
);

CREATE INDEX idx_payment_attempts_order_id ON payment_attempts (order_id);
