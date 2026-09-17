BEGIN;

LOCK TABLE orders IN ACCESS EXCLUSIVE MODE;
LOCK TABLE payment_attempts IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (
        SELECT order_id FROM payment_attempts
        WHERE status = 'paid'
        GROUP BY order_id HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'Payment settlement preflight failed: multiple paid attempts for one order; manually reconcile payment history before retrying migration 009';
    END IF;
    IF EXISTS (
        SELECT 1 FROM payment_attempts
        WHERE status = 'paid' AND (ref_id IS NULL OR ref_id <= 0)
    ) THEN
        RAISE EXCEPTION 'Payment settlement preflight failed: paid attempts have missing or nonpositive reference IDs; manually reconcile payment history before retrying migration 009';
    END IF;
    IF EXISTS (
        SELECT provider, ref_id FROM payment_attempts
        WHERE status = 'paid'
        GROUP BY provider, ref_id HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'Payment settlement preflight failed: duplicate paid provider references in legacy history; manually reconcile payment history before retrying migration 009';
    END IF;
END;
$$;

ALTER TABLE orders
    ADD COLUMN currency TEXT NOT NULL DEFAULT 'IRR',
    ADD CONSTRAINT orders_currency_check CHECK (currency = 'IRR');

ALTER TABLE payment_attempts
    ADD COLUMN currency TEXT NOT NULL DEFAULT 'IRR',
    ADD COLUMN environment TEXT NOT NULL DEFAULT 'legacy',
    ADD COLUMN account_key TEXT NOT NULL DEFAULT '',
    DROP CONSTRAINT payment_attempts_status_check,
    ADD CONSTRAINT payment_attempts_status_check
        CHECK (status IN ('pending', 'paid', 'failed', 'reconciliation')),
    ADD CONSTRAINT payment_attempts_currency_check CHECK (currency = 'IRR'),
    ADD CONSTRAINT payment_attempts_settlement_check CHECK (
        status NOT IN ('paid', 'reconciliation') OR (
            authority IS NOT NULL AND ref_id IS NOT NULL AND ref_id > 0
            AND provider_code IS NOT NULL AND provider_code IN (100, 101)
            AND verified_at IS NOT NULL
        )
    ) NOT VALID;

CREATE UNIQUE INDEX idx_payment_attempts_paid_order
    ON payment_attempts (order_id) WHERE status = 'paid';

CREATE UNIQUE INDEX idx_payment_attempts_paid_reference
    ON payment_attempts (provider, environment, account_key, ref_id)
    WHERE status = 'paid';

CREATE FUNCTION enforce_payment_attempt_binding() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.order_id, NEW.amount, NEW.currency, NEW.provider, NEW.environment, NEW.account_key)
        IS DISTINCT FROM ROW(OLD.order_id, OLD.amount, OLD.currency, OLD.provider, OLD.environment, OLD.account_key)
        OR (OLD.authority IS NOT NULL AND NEW.authority IS DISTINCT FROM OLD.authority) THEN
        RAISE EXCEPTION 'Payment attempt binding is immutable';
    END IF;
    IF OLD.status IN ('paid', 'reconciliation') AND
        ROW(NEW.status, NEW.ref_id, NEW.provider_code, NEW.verified_at)
        IS DISTINCT FROM ROW(OLD.status, OLD.ref_id, OLD.provider_code, OLD.verified_at) THEN
        RAISE EXCEPTION 'Payment attempt settlement is immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER payment_attempts_immutable_binding
    BEFORE UPDATE ON payment_attempts
    FOR EACH ROW EXECUTE FUNCTION enforce_payment_attempt_binding();

CREATE TABLE payment_attempt_events (
    id BIGSERIAL PRIMARY KEY,
    attempt_id BIGINT NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN (
        'backfill', 'created', 'updated', 'paid', 'reconciliation', 'verify_rejected', 'verify_uncertain'
    )),
    snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payment_attempt_events_attempt_id ON payment_attempt_events (attempt_id, id);

CREATE FUNCTION payment_attempt_event_snapshot(a payment_attempts) RETURNS JSONB
LANGUAGE sql IMMUTABLE AS $$
    SELECT jsonb_build_object(
        'order_id', a.order_id,
        'amount', a.amount,
        'currency', a.currency,
        'provider', a.provider,
        'environment', a.environment,
        'account_key', a.account_key,
        'authority', a.authority,
        'status', a.status,
        'provider_code', a.provider_code,
        'ref_id', a.ref_id,
        'verified_at', a.verified_at
    );
$$;

INSERT INTO payment_attempt_events (attempt_id, event_type, snapshot)
SELECT id, 'backfill', payment_attempt_event_snapshot(payment_attempts)
FROM payment_attempts;

CREATE FUNCTION record_payment_attempt_event() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    kind TEXT;
BEGIN
    IF TG_OP = 'INSERT' THEN
        kind := 'created';
    ELSIF NEW.status IS DISTINCT FROM OLD.status AND NEW.status IN ('paid', 'reconciliation') THEN
        kind := NEW.status;
    ELSE
        kind := 'updated';
    END IF;
    INSERT INTO payment_attempt_events (attempt_id, event_type, snapshot)
    VALUES (NEW.id, kind, payment_attempt_event_snapshot(NEW));
    RETURN NEW;
END;
$$;

CREATE TRIGGER payment_attempts_audit
    AFTER INSERT OR UPDATE ON payment_attempts
    FOR EACH ROW EXECUTE FUNCTION record_payment_attempt_event();

CREATE FUNCTION enforce_payment_attempt_events_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'Payment attempt events are append-only';
END;
$$;

CREATE TRIGGER payment_attempt_events_append_only
    BEFORE UPDATE OR DELETE ON payment_attempt_events
    FOR EACH STATEMENT EXECUTE FUNCTION enforce_payment_attempt_events_append_only();

CREATE TRIGGER payment_attempt_events_no_truncate
    BEFORE TRUNCATE ON payment_attempt_events
    FOR EACH STATEMENT EXECUTE FUNCTION enforce_payment_attempt_events_append_only();

COMMIT;
