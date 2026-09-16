-- Constrains orders.status to the full set of statuses supported by the
-- admin order management workflow. Previously status was an unconstrained
-- VARCHAR(32) defaulting to 'pending' with no other values in use; this
-- adds a CHECK constraint (defense in depth to match the pattern used for
-- users.role in 005_add_user_role.sql) now that admins can transition
-- orders through processing/shipped/delivered/cancelled.
ALTER TABLE orders
    ADD CONSTRAINT orders_status_check
    CHECK (status IN ('pending', 'processing', 'shipped', 'delivered', 'cancelled'));
