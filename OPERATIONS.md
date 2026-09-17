# Plant Shop — Operations Runbook

Concise operational reference for running Plant Shop in production.
Deployment details are in `README.md`; this page is for running it.

## Deployment

### Required environment variables

`docker-compose.prod.yml` reads them from an untracked `.env.prod` (copy
from `.env.prod.example`):

| Variable | Notes |
|---|---|
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | Postgres credentials. Must be strong in production (backend rejects placeholder passwords when `APP_ENV=production`). |
| `FRONTEND_PORT` | Host port for the nginx frontend (default 8081). |
| `FRONTEND_BASE_URL` | Public HTTPS origin of the storefront; used for payment-result redirects. Required in production. |
| `ZARINPAL_ENABLED` | `true` to enable payments; otherwise endpoints return 503. |
| `ZARINPAL_SANDBOX` | `true` for ZarinPal sandbox. Required (explicit) whenever payments are enabled. |
| `ZARINPAL_LIVE_ENABLED` | Must be `true` for sandbox=false; refuses live payments otherwise (fail-closed). |
| `ZARINPAL_MERCHANT_ID` | ZarinPal merchant UUID; never commit. Placeholders are rejected. |
| `ZARINPAL_CALLBACK_URL` | Must be `https://<domain>/api/v1/payments/zarinpal/callback`; HTTP allowed only for localhost in sandbox. |

Optional backend tuning (all have safe defaults): `DB_MAX_CONNS` (10),
`DB_MIN_CONNS` (2), `DB_MIN_IDLE_CONNS`, `DB_MAX_CONN_LIFETIME` (1h),
`DB_MAX_CONN_IDLE_TIME` (15m), `DB_HEALTH_CHECK_PERIOD` (1m), `PORT`.

Invalid production configuration fails startup with a clear error and no
secret values in the message.

### Migrations

The backend applies pending migrations at startup when
`APP_ENV=production` (advisory-locked, one transaction per migration).
Startup fails if a migration fails; nothing is marked applied on failure.

Prefer running the dedicated command before deploying, so schema changes
are decoupled from the app rollout:

```sh
docker compose -f docker-compose.prod.yml run --rm backend /app/server ... # not used; use the migrate command below
```

Actually:

```sh
# from repo root, with the .env.prod loaded (docker compose resolves service networking)
docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/server backend   # wrong; see below
```

The supported path is the dedicated command from the backend module:

```sh
cd backend
set DATABASE_URL=postgres://<user>:<password>@<host>:<port>/<db>?sslmode=require
go run ./cmd/migrate
```

Use `go run ./cmd/migrate -adopt-through=9` only for an existing database
whose schema was created by the old manual `psql` procedure and which has
no `schema_migrations` table. It builds the historical schema in a
separate empty **reference database** (`REFERENCE_DATABASE_URL`), byte-compares
the target's schema against it, and refuses to proceed on any mismatch.
The reference database must be a separate, disposable one.

### Startup order

`postgres` healthcheck → `backend` starts (runs migrations, then HTTP) →
`frontend` nginx (depends on backend `/readyz`-based health). `depends_on`
is not a readiness guarantee by itself — the backend's `/readyz` endpoint
must return 200 before traffic is sent; the compose `service_healthy`
condition for the frontend uses exactly that.

### Docker Compose production startup

```sh
cp .env.prod.example .env.prod   # fill in real values; never commit
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --build
```

### Health / readiness checks

- `GET /healthz` (or legacy `/health`) — liveness, no DB dependency.
- `GET /readyz` — checks database connectivity with a 1s timeout; 503 if
  the database is unreachable. Compose backend healthcheck uses `/readyz`.

### ZarinPal callback requirements

- Must be publicly reachable HTTPS on your deployed domain.
- Must path-match `/api/v1/payments/zarinpal/callback` exactly.
- Sandbox mode allows HTTP only on localhost (development only).
- Live mode requires `ZARINPAL_LIVE_ENABLED=true` and a public HTTPS host;
  localhost/private/CGNAT/placeholder hosts are rejected at startup.

### TLS / reverse-proxy assumptions

The frontend nginx container terminates plain HTTP only. TLS terminates
on an outer layer (host nginx/Caddy/traefik or your cloud LB) that
proxies to `FRONTEND_PORT`. Configure HSTS on that TLS layer, not in
`frontend/nginx.conf`. Keep session cookies Secure (backend sets them
Secure when `APP_ENV=production`) — which requires the public origin to
be HTTPS.

## Operations

### View logs

Backend logs are structured JSON on stdout (one event per request, no
request/response bodies, no auth headers or secrets):

```sh
docker compose -f docker-compose.prod.yml logs -f backend
docker compose -f docker-compose.prod.yml logs -f frontend
```

`request_id` is also set as the `X-Request-ID` response header.

### Database backup

```sh
POSTGRES_USER=plantshop POSTGRES_DB=plantshop ./ops/backup.sh
```

Writes `backups/<db>-<UTCtimestamp>.sql` on the host. Then:

1. **Verify** the dump: `grep -c "CREATE TABLE" backups/<file>.sql` should
   list the app tables; test-restore into a disposable database (below)
   before relying on it.
2. **Copy it off the host immediately** (object storage or a second
   machine). Backups on the app host do not protect against host loss.
3. **Encrypt at rest** (e.g. `age`, `gpg`, or your storage provider's
   server-side encryption) and lock down access (bucket policy / SSH
   keys) — a database dump contains customer data.

**Retention** is an operator decision: pick a schedule (e.g. nightly +
weekly offsite copies) and a retention window, and actually test the
restore path on a disposable database periodically.

### Database restore

```sh
docker compose -f docker-compose.prod.yml stop backend
POSTGRES_USER=... POSTGRES_DB=... RESTORE_FILE=backups/<file>.sql ./ops/restore.sh
docker compose -f docker-compose.prod.yml up -d backend
```

Verification after restore:

```sql
SELECT count(*) FROM products;
SELECT count(*) FROM orders;
SELECT count(*) FROM users;
SELECT count(*) FROM schema_migrations;
```

### Migration failure behavior

Migrations run before serving in production. If a migration fails:

- startup aborts (non-zero exit; container restarts and fails again);
- the failed migration's transaction is rolled back — the database is
  left at the previous version;
- nothing is recorded in `schema_migrations` for the failed migration;
- a `pg_advisory_lock` prevents two instances migrating concurrently.

Fix the migration (or the preconditions it reports), redeploy. Never
edit an already-applied migration file; add a new one instead.

### Payment reconciliation

Reconciliation state is viewed at `GET /api/v1/admin/payments/reconciliation`
(admin UI: Admin → Payment Reconciliation page), and a stuck attempt can
be re-verified via `POST /api/v1/admin/payments/{id}/reconcile`. There is
no background reconciler in V1 (by design).

### Basic incident response

1. **Site down / 5xx**: check `logs -f backend`; if `/readyz` returns 503
   the database is the problem — check the postgres container health and
   disk space.
2. **Bad deploy**: `docker compose -f docker-compose.prod.yml up -d --build`
   redeploys; migrations are forward-only, so a code rollback must be
   schema-compatible (new columns are nullable/defaulted by convention).
3. **Suspected data loss**: stop the backend, restore the latest verified
   backup per above, then restart.
4. **Payment disputes**: use the admin reconciliation page for attempt
   state; consult `payment_attempt_events` (append-only audit) before
   manually adjusting orders.
