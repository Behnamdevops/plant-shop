-- Adds normalized product categories. Existing products are preserved and
-- may remain uncategorized: products.category_id is nullable and uses
-- ON DELETE SET NULL so removing a category never deletes or orphans
-- product history.
CREATE TABLE categories (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE products
    ADD COLUMN category_id BIGINT NULL REFERENCES categories(id) ON DELETE SET NULL;

-- Supports category filtering on the public listing and admin category
-- deletion "safe" checks (is this category referenced by any product?).
CREATE INDEX idx_products_category_id ON products(category_id);

-- Supports the in_stock filter (stock > 0) and the price range filters plus
-- price_asc/price_desc sort modes on the public listing. Text search uses a
-- planner-chosen sequential scan for now (no dedicated GIN/trigram index) —
-- catalog size does not currently justify the extra index maintenance cost;
-- add one later if EXPLAIN shows it's needed.
CREATE INDEX idx_products_stock ON products(stock);
CREATE INDEX idx_products_price ON products(price);
