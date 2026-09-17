# Plant Shop

A small e-commerce app: Go + PostgreSQL backend, React + Vite + TypeScript
frontend, session-cookie authentication with user/admin roles, product
catalog, cart, and checkout/orders.

This README covers running it locally, promoting an admin, deploying with
Docker Compose, and reusing this codebase as a starter for a different
store.

## Project structure

- `backend/` — Go API (`main.go` entrypoint, `internal/<domain>/` packages:
  `auth`, `product`, `cart`, `order`).
- `backend/migrations/` — SQL migration files, applied manually (see below).
- `frontend/` — React + Vite + TypeScript SPA.
- `docker-compose.yml` — Postgres only, for local development.
- `docker-compose.prod.yml` — full production stack (postgres + backend +
  frontend).

## Development

### Prerequisites

- Go 1.25+ (toolchain 1.26 works)
- Node.js 20+ and npm
- Docker (for the Postgres container)

### 1. Start Postgres

```sh
docker compose up -d
```

This starts `postgres:17-alpine` on `localhost:5432` with database/user/
password all set to `plantshop` (see `docker-compose.yml`).

### 2. Configure and apply migrations

Set `DATABASE_URL` (see `backend/.env.example`):

```sh
$env:DATABASE_URL = "postgres://plantshop:plantshop@localhost:5432/plantshop"
```

Apply migrations manually, in order, against that database. There is no
migration runner configured — this is a deliberate choice for a small
project; see "Migrations" below for details:

```sh
psql $env:DATABASE_URL -f backend/migrations/001_create_products.sql
psql $env:DATABASE_URL -f backend/migrations/002_create_users_and_sessions.sql
psql $env:DATABASE_URL -f backend/migrations/003_create_cart_items.sql
psql $env:DATABASE_URL -f backend/migrations/004_create_orders.sql
psql $env:DATABASE_URL -f backend/migrations/005_add_user_role.sql
psql $env:DATABASE_URL -f backend/migrations/006_add_order_status_constraint.sql
psql $env:DATABASE_URL -f backend/migrations/007_add_order_fulfillment.sql
psql $env:DATABASE_URL -f backend/migrations/008_add_zarinpal_payments.sql
```

### 3. Run the backend

```sh
cd backend
go run .
```

Optional env vars (see `backend/.env.example`):

- `PORT` — defaults to `8080`.
- `APP_ENV` — `development` (default) or `production`. In production,
  session cookies are marked `Secure` (requires HTTPS); keep this as
  `development` for local HTTP testing.

Health check: `GET http://localhost:8080/health` → `{"status":"ok",...}`.

### 4. Run the frontend

```sh
cd frontend
npm install
npm run dev
```

The Vite dev server proxies `/api/*` to `http://localhost:8080` (see
`frontend/vite.config.ts`), so the frontend never needs a hard-coded API
base URL, in dev or in production.

### Payment result redirects

Set the backend environment variable `FRONTEND_BASE_URL=http://localhost:5173`
when running Vite separately from the API on port 8080. Payment callbacks then
redirect to `http://localhost:5173/payment/result` with URL-encoded `order_id`
and `outcome` parameters. `ZARINPAL_CALLBACK_URL` still points at the backend's
`/api/v1/payments/zarinpal/callback` endpoint, not the frontend result page.
Export these variables before starting Go; the backend does not load `.env` automatically.

For production, set `FRONTEND_BASE_URL=https://shop.example.com` to your public
storefront origin. Compose passes this variable to the backend. With nginx,
use the same public origin that serves the SPA and proxies `/api/`; do not use
an internal container address. Leaving the variable unset preserves relative,
same-origin `/payment/result` redirects (not suitable for split-origin development).

The base URL must be an HTTP(S) origin with no credentials, query, fragment,
or path prefix; an optional trailing slash is normalized. HTTP is allowed only
for localhost, `*.localhost`, `127.0.0.1`, or `::1` during development.
`APP_ENV=production` or non-sandbox payments require HTTPS and a public host.
Invalid configured URLs fail startup, even when payments are disabled.
Callback/query parameters never determine the redirect destination.

### 5. Run backend tests

```sh
cd backend
go test ./... -count=1
```

Tests are integration tests that hit a real Postgres via `DATABASE_URL`.
If `DATABASE_URL` is unset, they skip automatically. Point it at your local
`docker compose` Postgres to run them for real; they clean up their own
rows.

## ZarinPal Payment V1

- `POST /api/v1/orders/{id}/payments/zarinpal` — authenticated; starts a
  ZarinPal payment attempt for the caller's own order (amount is always the
  order's persisted total) and returns `{"redirect_url": "..."}` to send the
  browser to.
- `GET /api/v1/payments/zarinpal/callback` — public; ZarinPal redirects the
  browser here after the gateway flow. Looks up the attempt by `Authority`,
  verifies server-to-server against ZarinPal (never trusts the callback's
  `Status` alone), then redirects to the frontend's `/payment/result` page.
- `POST /api/v1/orders/{id}/cancel` — authenticated; lets a customer cancel
  their own order while it's still pending/processing and unpaid, restoring
  inventory via the same code path as admin cancellation.
- Orders paid via ZarinPal cannot be advanced to processing/shipped/
  delivered by an admin until payment_status is `paid` (409 otherwise).
- Payment attempts are stored in `payment_attempts`
  (`backend/migrations/008_add_zarinpal_payments.sql`), separate from
  `orders`, so a retried/abandoned attempt never overwrites order history.
- **Remaining gap**: there is no background job to expire/release stock for
  orders whose ZarinPal attempt was abandoned without ever returning to the
  callback. Customers can self-cancel manually (above); an automatic
  timeout worker is not implemented in V1.

### 6. Lint / build the frontend

```sh
cd frontend
npm run lint
npm run build
```

## Admin setup

There is intentionally **no self-promotion endpoint or API** to grant the
`admin` role — this must be done directly in the database:

```sql
UPDATE users SET role = 'admin' WHERE email = 'someone@example.com';
```

Register the user through the app first, then run the update above against
your Postgres instance.

## Production

### Required environment variables

Backend (`backend/.env.example`):

| Variable | Required | Default | Notes |
|---|---|---|---|
| `DATABASE_URL` | yes | — | Postgres connection string |
| `PORT` | no | `8080` | HTTP listen port |
| `APP_ENV` | no | `development` | Set to `production` to mark session cookies `Secure` |
| `ZARINPAL_MERCHANT_ID` | for payments | — | Your 36-character ZarinPal Merchant ID. Never commit a real value. |
| `ZARINPAL_CALLBACK_URL` | for payments | — | Full URL ZarinPal redirects the browser back to; **must be a publicly reachable HTTPS domain in production** — see "ZarinPal payments in production" below. |
| `ZARINPAL_SANDBOX` | no | unset (production) | Set to `true` to use ZarinPal's sandbox environment instead of real payments. |

If `ZARINPAL_MERCHANT_ID`/`ZARINPAL_CALLBACK_URL` are unset, the payment
endpoints stay registered but any request to them fails cleanly with a
provider error (ZarinPal always rejects an empty merchant id) — the rest of
the app (browsing, cart, "manual" checkout) is unaffected.

Frontend build args (`frontend/.env.example`), baked in at build time:

| Variable | Required | Default |
|---|---|---|
| `VITE_STORE_NAME` | no | `Plant Shop` |
| `VITE_CURRENCY_SYMBOL` | no | `$` (unused in the Persian storefront, which always displays تومان) |

Compose-level (`.env.prod.example`): `POSTGRES_USER`, `POSTGRES_PASSWORD`,
`POSTGRES_DB`, `FRONTEND_PORT`, `ZARINPAL_MERCHANT_ID`,
`ZARINPAL_CALLBACK_URL`, `ZARINPAL_SANDBOX`.

### ZarinPal payments in production

`ZARINPAL_CALLBACK_URL` must be a **publicly reachable HTTPS URL** pointing
at `https://<your-domain>/api/v1/payments/zarinpal/callback`. ZarinPal's
servers redirect the customer's browser to this URL after the gateway flow
completes, and the browser must be able to reach it from the public
internet — `http://localhost:...` or any private/internal address will not
work for real production traffic. Use ZarinPal's sandbox
(`ZARINPAL_SANDBOX=true`) while developing, and only point a real Merchant
ID + production callback URL at a domain you've actually deployed the app
to with a valid TLS certificate.

### Running the production stack

```sh
cp .env.prod.example .env.prod   # edit with real values, never commit it
docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --build
```

- **Where it's exposed**: only the `frontend` (nginx) service is published
  to the host, on `FRONTEND_PORT` (default `8081`). It serves the built SPA
  and reverse-proxies `/api/*` to the `backend` service by Docker service
  name, so the browser only ever talks to one origin — no CORS
  configuration is needed, and cookies stay same-site.
- **Networking**: `postgres` and `backend` are not published to the host;
  they're reachable only on the internal compose network, by service name
  (`postgres`, `backend`).
- **Database persistence**: Postgres data lives in the named volume
  `postgres_data`, which survives `docker compose down` (but not
  `docker compose down -v`).
- **Migrations**: still applied manually. Once `postgres` is up, either
  `psql` in through a temporary port mapping, or run `psql` from inside the
  postgres container:
  ```sh
  docker compose -f docker-compose.prod.yml exec -T postgres \
    psql -U <POSTGRES_USER> -d <POSTGRES_DB> < backend/migrations/001_create_products.sql
  ```
  (repeat for each migration file, in numeric order, skipping the ones
  already applied).

## Reuse as a starter for another store

To fork this for a different store:

1. Change branding: set `VITE_STORE_NAME` and `VITE_CURRENCY_SYMBOL` in
   `frontend/.env` (dev) or as build args / in `.env.prod` (production).
   These are read from `frontend/src/config.ts`, the single place store
   branding is defined — no need to hunt through components.
2. Change `POSTGRES_DB`/`POSTGRES_USER`/`POSTGRES_PASSWORD` and
   `DATABASE_URL` for your own database.
3. Update the Go module path in `backend/go.mod` if you're publishing under
   a different repo/org.
4. Review `backend/migrations/` if you need a different product schema —
   there's no migration framework, just plain SQL files applied in order.
5. **Never commit secrets.** Only `.env.example` and `.env.prod.example`
   files are tracked; real `.env`/`.env.prod` files are gitignored.

## Migration cleanup note

The repo previously had two conflicting `002_*` migrations:
`002_create_users.sql` (a `username`/no-sessions schema) and
`002_create_users_and_sessions.sql` (`name`/`email`/`role`/`sessions`
schema). Only the latter matches what the Go code actually queries — the
former was dead, unused, and would break auth if applied. It has been
removed. If you already applied `002_create_users.sql` to an existing
database, do not re-run migrations blindly; compare your schema against
`backend/migrations/002_create_users_and_sessions.sql` and reconcile
manually before proceeding.
