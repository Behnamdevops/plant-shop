ALTER TABLE payment_attempts DROP CONSTRAINT payment_attempts_status_check;
ALTER TABLE payment_attempts ADD CONSTRAINT payment_attempts_status_check CHECK(status IN('pending','paid','failed','reconciliation','expired'));
CREATE INDEX orders_reservation_expiry ON orders(created_at) WHERE status='pending' AND payment_status IN('pending','failed');
