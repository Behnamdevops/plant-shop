# AGENTS.md

Early-stage repo: only `backend/` has real code. `frontend/`, `deploy/` are empty placeholders (`.gitkeep` only). `README.md` is empty.

## Structure

- `backend/main.go` — entrypoint: pgxpool connect, `http.NewServeMux`, `ListenAndServe :8080`.
- `backend/internal/<domain>/` — per-domain `model.go` + `repository.go` + `handler.go` (currently only `product/`). Repository takes `*pgxpool.Pool`; handler takes `*Repository`.
- `backend/migrations/migrations/001_create_products.sql` — note double-nested `migrations/migrations/` path. No migration runner is configured; apply manually.
- `docker-compose.yml` — defines `postgres:17-alpine` only (no app service).

## Run / verify (from `backend/`)

Module: `github.com/Behnamdevops/plant-shop/backend`, Go 1.25+ (toolchain 1.26 works).

```sh
docker compose up -d                    # from repo root; postgres on localhost:5432 (plantshop/plantshop/plantshop)
# apply schema manually, e.g.:
psql "postgres://plantshop:plantshop@localhost:5432/plantshop" -f backend/migrations/migrations/001_create_products.sql
DATABASE_URL="postgres://plantshop:plantshop@localhost:5432/plantshop" go run .
go build ./... && go vet ./... && go test ./...   # no tests exist yet
```

- `DATABASE_URL` is required; `main.go` does `log.Fatal` if unset or unreachable.
- Health check: `GET /health` → `{"status":"ok"}`.

## API (stdlib `net/http`, Go 1.22+ `METHOD /pattern` routing)

- `GET /api/v1/products` — list, `ORDER BY id DESC`, returns `[]` (not null) when empty.
- `GET /api/v1/products/{slug}` — route param read via `r.PathValue("slug")`; `pgx.ErrNoRows` → 404.
- Handlers write JSON with `encoding/json` only; no router framework.

## Data conventions

- `products.price` is `BIGINT NOT NULL CHECK (price >= 0)` mapped to Go `int64` (minor units); `stock INTEGER DEFAULT 0 CHECK (stock >= 0)`; `image_url TEXT NULL` → `*string`.
- `slug` is `UNIQUE NOT NULL`; no `updated_at` trigger — callers must set it on writes.

## Destructive operations

Never run destructive database, Docker volume, or filesystem operations without
explicit user approval.

Forbidden by default:
- docker compose down -v
- docker volume rm
- DROP DATABASE on the normal development database
- deleting/recreating the development PostgreSQL volume
- rm -rf on project/data directories
- git reset --hard
- git clean -fd

Tests that require a clean database must create and use a separate disposable
test database, preferably with a `_test` suffix.

The normal development database and Docker volume must be treated as persistent
user data, even if test fixtures are present.

If a clean database is required and cannot be created safely, stop and ask the
user rather than deleting existing data.