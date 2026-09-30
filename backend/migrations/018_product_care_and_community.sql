ALTER TABLE products ADD COLUMN care JSONB NOT NULL DEFAULT '{}',ADD COLUMN image_urls JSONB NOT NULL DEFAULT '[]';
ALTER TABLE products ADD CONSTRAINT care_object CHECK(jsonb_typeof(care)='object'),ADD CONSTRAINT gallery_array CHECK(jsonb_typeof(image_urls)='array');
CREATE TABLE product_slug_aliases(slug TEXT PRIMARY KEY,product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE);
CREATE TABLE article_slug_aliases(slug TEXT PRIMARY KEY,article_id BIGINT NOT NULL REFERENCES articles(id) ON DELETE CASCADE);
CREATE TABLE wishlists(user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(user_id,product_id));
CREATE TABLE product_reviews(id BIGSERIAL PRIMARY KEY,user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,rating INTEGER NOT NULL CHECK(rating BETWEEN 1 AND 5),comment TEXT NOT NULL CHECK(length(comment) BETWEEN 3 AND 2000),status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN('pending','approved','rejected')),created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),UNIQUE(user_id,product_id));
CREATE INDEX review_public_product ON product_reviews(product_id,created_at) WHERE status='approved';
CREATE INDEX products_care_light ON products((care->>'light'));
