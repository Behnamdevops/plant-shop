-- Checkout & Fulfillment V2: adds a delivery snapshot, V1 shipping method
-- and fee, an explicit items/shipping/total split, and payment-state
-- foundations to the orders table.
--
-- Nullability / backfill decisions for existing rows:
--
-- * Delivery snapshot fields (recipient_name, phone, address_line1,
--   address_line2, city, postal_code, country) are added as NULLable with
--   no default. Historical orders were placed before any delivery
--   information was ever collected, so there is no honest non-empty value
--   to backfill them with; NULL means "no delivery snapshot was captured
--   for this order" rather than falsely implying an empty string was
--   provided by a customer. The checkout handler (application layer)
--   requires all of these except address_line2 for *new* orders; the
--   database does not enforce NOT NULL here so old rows remain valid.
--
-- * shipping_method defaults to 'standard' and shipping_fee defaults to 0
--   for existing rows, since pre-V2 checkout never charged shipping.
--
-- * items_subtotal defaults to 0 and is then backfilled to equal the
--   existing `total` column for every pre-existing row (see UPDATE below),
--   preserving the invariant `total = items_subtotal + shipping_fee` for
--   historical orders (shipping_fee = 0, so items_subtotal = total).
--
-- * payment_status defaults to 'pending' and payment_method to 'manual'
--   for existing rows. This is a reasonable placeholder: no real payment
--   integration exists yet (pre- or post-V2), so there is no historical
--   payment state to recover. Admins/support can be told that orders
--   placed before this migration have an unknown/approximate payment
--   status.
--
-- The `total` column itself is unchanged and keeps its existing meaning
-- (the final, all-in order total); it continues to be calculated
-- server-side as items_subtotal + shipping_fee for every order going
-- forward.

ALTER TABLE orders
    ADD COLUMN recipient_name VARCHAR(255),
    ADD COLUMN phone VARCHAR(64),
    ADD COLUMN address_line1 VARCHAR(255),
    ADD COLUMN address_line2 VARCHAR(255),
    ADD COLUMN city VARCHAR(128),
    ADD COLUMN postal_code VARCHAR(32),
    ADD COLUMN country VARCHAR(128),
    ADD COLUMN shipping_method VARCHAR(20) NOT NULL DEFAULT 'standard',
    ADD COLUMN shipping_fee BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN items_subtotal BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN payment_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    ADD COLUMN payment_method VARCHAR(20) NOT NULL DEFAULT 'manual';

ALTER TABLE orders
    ADD CONSTRAINT orders_shipping_method_check
        CHECK (shipping_method IN ('standard', 'express')),
    ADD CONSTRAINT orders_shipping_fee_check
        CHECK (shipping_fee >= 0),
    ADD CONSTRAINT orders_items_subtotal_check
        CHECK (items_subtotal >= 0),
    ADD CONSTRAINT orders_payment_status_check
        CHECK (payment_status IN ('pending', 'paid', 'failed', 'refunded')),
    ADD CONSTRAINT orders_payment_method_check
        CHECK (payment_method IN ('manual'));

-- Backfill items_subtotal for pre-existing rows so the
-- total = items_subtotal + shipping_fee invariant holds for historical
-- orders (their shipping_fee is 0, so items_subtotal = total).
UPDATE orders SET items_subtotal = total WHERE items_subtotal = 0;
