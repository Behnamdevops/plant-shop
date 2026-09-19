-- Customer Account V2: adds phone to users and user_addresses table for
-- profile management and address book functionality.
--
-- Design notes:
--
-- * Phone is added as VARCHAR(20) NULLABLE to users table:
--   - Length 20 accommodates international formats with country code
--   - NULLABLE because existing users don't have phone numbers
--   - Validation (format, requiredness) happens in application layer
--   - No default value: NULL means "phone not provided"
--
-- * user_addresses table stores saved shipping addresses:
--   - References users(id) ON DELETE CASCADE
--   - Label is optional (default empty string) for user's own reference
--   - All other fields (recipient_name, phone, address_line1, city, postal_code, country) are required
--   - address_line2 is optional (nullable)
--   - is_default marks one address per user as default for checkout prefilling
--   - created_at/updated_at track timestamps
--
-- * Unique partial index ensures at most one default address per user:
--   CREATE UNIQUE INDEX idx_user_addresses_one_default_per_user ON user_addresses(user_id) WHERE is_default = true
--   This is race-safe: concurrent inserts/updates that would create multiple defaults
--   will fail with a uniqueness violation, and the transaction must retry.
--
-- * Additional indexes:
--   - user_id for efficient listing/deleting by user
--   - (user_id, is_default) for quickly finding the default address
--
-- * Delivery snapshot in orders remains unchanged and independent:
--   - Saved addresses are only for checkout convenience
--   - Order snapshots are immutable and never reference address_id
--   - Changing/deleting saved addresses does not affect historical orders

-- Add phone column to users (nullable, no default)
ALTER TABLE users
    ADD COLUMN phone VARCHAR(20) NULL;

-- Create user_addresses table
CREATE TABLE user_addresses (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label TEXT NOT NULL DEFAULT '',
    recipient_name TEXT NOT NULL,
    phone TEXT NOT NULL,
    address_line1 TEXT NOT NULL,
    address_line2 TEXT NULL,
    city TEXT NOT NULL,
    postal_code TEXT NOT NULL,
    country TEXT NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for efficient queries
CREATE INDEX idx_user_addresses_user_id ON user_addresses(user_id);
CREATE INDEX idx_user_addresses_user_id_is_default ON user_addresses(user_id, is_default DESC);

-- Ensure at most one default address per user
CREATE UNIQUE INDEX idx_user_addresses_one_default_per_user 
    ON user_addresses(user_id) 
    WHERE is_default = true;