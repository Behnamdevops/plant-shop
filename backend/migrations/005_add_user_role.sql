-- Adds a role column to users for authorization (e.g. gating admin-only
-- endpoints). Existing rows safely default to 'user'. There is no public
-- API for granting the 'admin' role; promote a user directly in the
-- database, e.g.:
--   UPDATE users SET role = 'admin' WHERE email = 'someone@example.com';
ALTER TABLE users
    ADD COLUMN role VARCHAR(20) NOT NULL DEFAULT 'user'
    CHECK (role IN ('user', 'admin'));
