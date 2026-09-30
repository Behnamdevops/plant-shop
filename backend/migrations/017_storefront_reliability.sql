CREATE TABLE cart_merge_batches(user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,merge_key TEXT NOT NULL CHECK(length(merge_key) BETWEEN 16 AND 128),result JSONB NOT NULL,created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),PRIMARY KEY(user_id,merge_key));
CREATE TABLE password_reset_tokens(token_hash TEXT PRIMARY KEY,user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,expires_at TIMESTAMPTZ NOT NULL,used_at TIMESTAMPTZ);
CREATE INDEX password_reset_expiry ON password_reset_tokens(expires_at);
