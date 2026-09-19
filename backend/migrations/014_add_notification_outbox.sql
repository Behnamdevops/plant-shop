-- Notifications V1: database-backed notification outbox for reliable email delivery
-- Events are enqueued transactionally and processed by a separate outbox worker
-- to avoid blocking business transactions on SMTP network calls.

CREATE TYPE notification_status AS ENUM ('pending', 'processing', 'sent', 'failed');

CREATE TABLE notification_outbox (
    id BIGSERIAL PRIMARY KEY,
    event_key TEXT NOT NULL UNIQUE,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    recipient_email TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'sent', 'failed')),
    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notification_outbox_status_pending ON notification_outbox (status) WHERE status = 'pending';
CREATE INDEX idx_notification_outbox_status_next_attempt ON notification_outbox (status, next_attempt_at) WHERE status IN ('pending', 'processing');
CREATE INDEX idx_notification_outbox_user_id ON notification_outbox (user_id);
CREATE INDEX idx_notification_outbox_event_key ON notification_outbox (event_key);
