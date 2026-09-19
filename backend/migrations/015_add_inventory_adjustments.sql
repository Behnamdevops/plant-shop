-- Inventory Adjustments V1: tracks manual stock adjustments made by admins
--
-- This table records all manual inventory adjustments (not automatic
-- checkout decrements or cancellation restorations). It provides an audit
-- trail for stock changes made through the /admin/inventory endpoints.

CREATE TABLE inventory_adjustments (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    admin_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    delta INTEGER NOT NULL CHECK (delta != 0),
    stock_before INTEGER NOT NULL CHECK (stock_before >= 0),
    stock_after INTEGER NOT NULL CHECK (stock_after >= 0),
    reason TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX idx_inventory_adjustments_product_id ON inventory_adjustments(product_id);
CREATE INDEX idx_inventory_adjustments_created_at ON inventory_adjustments(created_at);
CREATE INDEX idx_inventory_adjustments_admin_user_id ON inventory_adjustments(admin_user_id);
