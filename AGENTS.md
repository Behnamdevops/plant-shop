# Project architecture

Go 1.26 API with PostgreSQL lives in backend/. React/TypeScript lives in frontend/.
Public routes use Node SSR via frontend/server.mjs; account/admin routes use SPA.
API, uploads, robots and sitemap are proxied same-origin. Production Compose has
PostgreSQL, backend, storefront and nginx. Root scripts/local.mjs launches the
non-Docker development or SSR mode with coordinated ports and incremental migrations.
Read README.md, RUN-LOCAL.md and UPGRADE.md for setup and feature scope.

# Verification

Run frontend npm run lint and npm run build; backend go vet ./... and go test ./....
Integration tests require DATABASE_URL pointing ONLY to a separate disposable,
migrated database, preferably with a _test suffix. Browser tests need a separate
preview database with at least one available product. Preserve the uploaded original.
Amounts are integer rial in the API/database; public prices and price inputs use toman.
Do not invent gateway credentials, SMTP credentials or business policies.

# Data preservation

Never run destructive database, Docker volume, or filesystem operations without
explicit user approval. Never delete normal development volumes/data, run
`docker compose down -v`, `docker volume rm`, `git reset --hard` or `git clean -fd`.
Never run tests or restore against an active store database. Restore must create a
new database and new image directory. Historical databases without migration
history require explicit, schema-verified adoption using an empty reference database.
