-- Refund & Returns V1: return_requests and refunds tables
--
-- return_requests tracks customer-initiated return/refund requests and
-- their state machine transitions (requested -> approved/rejected ->
-- received -> refund_pending -> refunded/failed).
--
-- refunds tracks the actual financial refund attempts, including provider
-- integration. One order may have at most one successful refund.
--
-- IMPORTANT: This migration adds the database foundation for V1 refund/return
-- workflow. It does NOT implement any automatic provider refund logic.
-- ZarinPal integration (if enabled) requires careful idempotency handling
-- and may need manual confirmation depending on provider capabilities.

-- Create trigger function for updating updated_at timestamp
CREATE OR REPLACE FUNCTION trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create return_requests table
CREATE TABLE return_requests (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('requested', 'approved', 'rejected', 'received', 'refund_pending', 'refunded', 'refund_failed')),
    reason TEXT NOT NULL CHECK (length(reason) >= 1 AND length(reason) <= 1024),
    customer_note TEXT NULL CHECK (length(customer_note) <= 2048),
    admin_note TEXT NULL CHECK (length(admin_note) <= 2048),
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ NULL,
    received_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX idx_return_requests_order_id ON return_requests(order_id);
CREATE INDEX idx_return_requests_user_id ON return_requests(user_id);
CREATE INDEX idx_return_requests_status ON return_requests(status);

-- Create refunds table
CREATE TABLE refunds (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    return_request_id BIGINT NULL REFERENCES return_requests(id) ON DELETE SET NULL,
    amount BIGINT NOT NULL CHECK (amount > 0),
    payment_method TEXT NOT NULL CHECK (payment_method IN ('manual', 'zarinpal')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'succeeded', 'failed', 'manual_review')),
    provider_refund_id TEXT NULL,
    provider_response JSONB NULL,
    requested_by_admin_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    last_error TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ NULL
);

-- Indexes for efficient querying
CREATE INDEX idx_refunds_order_id ON refunds(order_id);
CREATE INDEX idx_refunds_status ON refunds(status);
CREATE INDEX idx_refunds_return_request_id ON refunds(return_request_id);

-- Trigger to update updated_at timestamp on return_requests
CREATE TRIGGER set_return_requests_updated_at
    BEFORE UPDATE ON return_requests
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_timestamp();

-- Trigger to update updated_at timestamp on refunds
CREATE TRIGGER set_refunds_updated_at
    BEFORE UPDATE ON refunds
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_timestamp();

-- Add check constraint to ensure only one successful refund per order
-- (enforced via partial unique index)
CREATE UNIQUE INDEX idx_refunds_unique_successful ON refunds(order_id) WHERE status = 'succeeded';
