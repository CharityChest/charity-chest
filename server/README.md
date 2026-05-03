# Charity Chest — Server

Go HTTP server built with [Echo v4](https://echo.labstack.com/). Supports email/password and Google OAuth authentication with JWT-based sessions.

## Prerequisites

- Go 1.22+ **or** Docker + Docker Compose (no local Go/Postgres needed)
- A Google Cloud project with OAuth 2.0 credentials

---

## Quick start with Docker (recommended for demos)

```bash
# 1. Create the secrets file
cp .docker-dev/.env.example .docker-dev/.env
# Edit .docker-dev/.env — add your GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET

# 2. Start everything (Postgres + Valkey + server, migrations run automatically)
docker compose -f .docker-dev/docker-compose.yml up --build
```

The server is available at `http://localhost:8080`. Postgres is exposed on port `5432` and Valkey on port `6379` for local inspection.

**Useful commands:**

```bash
# Stop and remove containers (data volume is preserved)
docker compose -f .docker-dev/docker-compose.yml down

# Wipe the database volume too
docker compose -f .docker-dev/docker-compose.yml down -v

# Rebuild the server image after code changes
docker compose -f .docker-dev/docker-compose.yml up --build server
```

---

## 1. Environment setup

Copy the example file and fill in your values:

```bash
cp .env.example .env
```

| Variable | Required | Description |
|---|---|---|
| `DATABASE_URL` | yes | PostgreSQL connection string |
| `JWT_SECRET` | yes | Long random string — run `openssl rand -hex 32` |
| `GOOGLE_CLIENT_ID` | yes | From Google Cloud Console |
| `GOOGLE_CLIENT_SECRET` | yes | From Google Cloud Console |
| `GOOGLE_REDIRECT_URL` | no | Server-side OAuth callback URI registered in Google Cloud Console (default `http://localhost:8080/v1/auth/google/callback`) |
| `FRONTEND_URL` | no | Base URL of the webapp — used to redirect the browser back after Google login (default `http://localhost:3000`) |
| `PORT` | no | HTTP listen port (default `8080`) |
| `APP_ENV` | **yes** | Deployment environment. Must be one of: `local`, `testing`, `staging`, `production`. The server refuses to start if this is absent or set to an unrecognised value. |
| `CACHE_ENABLED` | no | Set to `true` to enable Valkey caching (default `false`) |
| `CACHE_URL` | no | Valkey/Redis connection URL (default `redis://localhost:6379`) |
| `CACHE_TTL` | no | TTL for all cache entries, e.g. `30s`, `2m`, `10m` (default `5m`) |
| `STRIPE_SECRET_KEY` | no | Stripe secret key. When unset, all billing endpoints return 503. |
| `STRIPE_WEBHOOK_SECRET` | if Stripe enabled | Stripe webhook signing secret. **Required** when `STRIPE_SECRET_KEY` is set — the server refuses to start without it. Also required at runtime: in production a missing secret causes `POST /stripe/webhook` to return 503 immediately. |
| `STRIPE_PRO_PRICE_ID` | if Stripe enabled | Stripe Price ID for the Pro plan (e.g. `price_xxx`). **Required** when `STRIPE_SECRET_KEY` is set. |

---

## 2. Google OAuth credentials

1. Go to [Google Cloud Console → APIs & Services → Credentials](https://console.cloud.google.com/apis/credentials).
2. Click **Create credentials → OAuth client ID**.
3. Choose **Web application**.
4. Under **Authorized redirect URIs** add:
   ```
   http://localhost:8080/v1/auth/google/callback
   ```
5. Copy the **Client ID** and **Client Secret** into your `.env`.

---

## 3. Build

A `Makefile` is provided with the following targets:

| Target | Output | Description |
|---|---|---|
| `make build` | both | Builds debug and release |
| `make build-debug` | `dist/debug/server` | Race detector on, optimisations off — debugger-friendly |
| `make build-release` | `dist/release/server` | Static binary, debug info stripped, optimised for deployment |
| `make seed-root EMAIL=… PASSWORD=…` | — | Creates the first root user in the database |
| `make tidy` | — | Runs `go mod tidy` |
| `make clean` | — | Removes the `dist/` directory |

```bash
make build-debug    # development
make build-release  # production
```

## 4. Run the server

```bash
# Directly with Go (no build step)
go run .

# From a debug build
./dist/debug/server

# From a release build
./dist/release/server
```

Migrations run automatically on startup. The server listens on `http://localhost:8080` by default.

---

## 5. Database migrations

SQL migration files live in `migrations/`, named `NNNNNN_<description>.{up,down}.sql`. The server applies all pending up migrations automatically at startup. The Makefile targets let you drive migrations manually when needed.

`golang-migrate` is already in `go.mod` — no separate CLI install is required. `DATABASE_URL` must be set in your shell (or in `.env`). All three migration targets validate this at startup and exit immediately with a clear error message if it is missing:

```bash
export DATABASE_URL="postgres://user:password@localhost:5432/charitychest?sslmode=disable"
```

### Quick reference

| Command | What it does |
|---|---|
| `make migrate` | Apply all pending up migrations |
| `make migrate-down N=1` | Roll back the last N migrations |
| `make migrate-version` | Print the current schema version |

### Apply pending migrations

```bash
make migrate
```

This runs every `.up.sql` file that has not yet been applied, in order. The server does the same automatically on startup, so this target is mainly useful when you want to migrate without starting the server.

### Roll back N steps

```bash
# Roll back the last migration
make migrate-down N=1

# Roll back the last two migrations
make migrate-down N=2
```

> **Never** pass `N` equal to the total number of migrations — that wipes the entire schema.

### Check the current schema version

```bash
make migrate-version
```

### Go to a specific version

```bash
go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate \
  -path migrations -database "$DATABASE_URL" goto 5
```

Runs up or down steps as needed to reach exactly version 5.

### Recover from a failed migration

A mid-flight failure leaves the schema in a "dirty" state. Fix the SQL, force the version back to the last clean migration, then re-run:

```bash
# Force version to 5 (last fully applied migration)
go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate \
  -path migrations -database "$DATABASE_URL" force 5

# Re-apply
make migrate
```

### Adding a new migration

1. Create `migrations/NNNNNN_<description>.up.sql` and `migrations/NNNNNN_<description>.down.sql`.
2. The `.down.sql` must exactly undo everything the `.up.sql` does.
3. Never edit a migration that has already been applied to any environment — add a new one instead.

---

## 6. API reference

All application endpoints are versioned under `/v1/`. The health probe is unversioned.

### Response format

Every successful JSON response is wrapped in a `data` envelope:

```json
{ "data": { ... } }
```

Error responses (4xx / 5xx) are **not** wrapped — they use Echo's default format:

```json
{ "message": "error description" }
```

Endpoints that return no body (204 No Content) are also not wrapped.

### Health check

```bash
curl http://localhost:8080/health
# {"data":{"status":"ok"}}
```

### System status (public)

Returns whether the system has been bootstrapped (at least one `root` user exists in the database).

```bash
curl http://localhost:8080/v1/system/status
# {"data":{"configured":false}}
```

### Register (email + password)

```bash
curl -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"secret123","name":"Your Name"}'
```

Response:
```json
{
  "token": "<jwt>",
  "user": { "id": 1, "email": "you@example.com", "name": "Your Name", ... }
}
```

### Login (email + password)

```bash
curl -X POST http://localhost:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"secret123"}'
```

The returned JWT embeds the user's system-level role (`role` claim). Role changes take effect after re-login.

### Access a protected route

```bash
curl http://localhost:8080/v1/api/me \
  -H "Authorization: Bearer <token>"
```

### Login with MFA enabled

When a user has MFA enabled, `POST /v1/auth/login` returns an MFA challenge instead of a token:

```json
{ "mfa_required": true, "mfa_token": "<short-lived-jwt>" }
```

Submit the TOTP code to complete the login:

```bash
curl -X POST http://localhost:8080/v1/auth/mfa/verify \
  -H "Content-Type: application/json" \
  -d '{"mfa_token": "<mfa-token>", "code": "123456"}'
```

Response on success: `{"token": "<jwt>", "user": {...}}`.

### MFA setup and management (authenticated)

```bash
# Generate a TOTP secret and QR code URI
curl http://localhost:8080/v1/api/profile/mfa/setup \
  -H "Authorization: Bearer <token>"
# Returns {"uri": "otpauth://...", "secret": "BASE32SECRET"}

# Verify a code and activate MFA
curl -X POST http://localhost:8080/v1/api/profile/mfa/enable \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"code": "123456"}'

# Deactivate MFA (requires current TOTP code)
curl -X DELETE http://localhost:8080/v1/api/profile/mfa \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"code": "123456"}'
```

### Assign system role (root only)

```bash
curl -X POST http://localhost:8080/v1/api/system/assign-role \
  -H "Authorization: Bearer <root-token>" \
  -H "Content-Type: application/json" \
  -d '{"user_id": 2, "role": "system"}'
# Pass "role": "" to remove the system role
```

### User search (root only)

Search users by email with pagination. Returns each user's system role and org memberships.

```bash
# Search all users (page 1, 20 per page)
curl "http://localhost:8080/v1/api/admin/users" \
  -H "Authorization: Bearer <root-token>"

# Filter by email (partial match)
curl "http://localhost:8080/v1/api/admin/users?email=alice&page=1&size=20" \
  -H "Authorization: Bearer <root-token>"
```

Response:
```json
{
  "data": [
    {
      "id": 3,
      "email": "alice@example.com",
      "name": "Alice",
      "role": "system",
      "mfa_enabled": false,
      "created_at": "...",
      "organizations": [
        { "id": 1, "name": "Acme NGO", "role": "owner" }
      ]
    }
  ],
  "metadata": {
    "page": 1,
    "size": 20,
    "total": 42,
    "total_pages": 3
  }
}
```

Query parameters: `email` (optional, partial match), `page` (default 1), `size` (default 20, max 100).

### Plans & billing

```bash
# Activate enterprise plan (root/system only)
curl -X POST http://localhost:8080/v1/api/orgs/1/plan/enterprise \
  -H "Authorization: Bearer <root-token>"

# Create Stripe Checkout session (org owner, root, or system)
# Redirect the user to the returned URL to complete payment.
curl -X POST "http://localhost:8080/v1/api/orgs/1/billing/checkout?locale=en" \
  -H "Authorization: Bearer <token>"
# Returns: {"data":{"url":"https://checkout.stripe.com/..."}}

# Cancel Pro subscription (org owner, root, or system)
# Plan reverts to free when the webhook fires.
curl -X DELETE http://localhost:8080/v1/api/orgs/1/billing/subscription \
  -H "Authorization: Bearer <token>"
```

Stripe webhooks are received at `POST /stripe/webhook`. Signature verification is enforced when `APP_ENV=production`; outside production, raw unsigned payloads are accepted so local dev and automated tests can POST events without a real Stripe account.

**Webhook behaviour notes:**
- `checkout.session.completed` — if the org is already on the enterprise plan the webhook cancels the new Stripe subscription and refunds the initial payment, then returns 409. The org's plan is not changed.
- `customer.subscription.deleted` — downgrades the org to `free` and clears `stripe_subscription_id`.

**`POST /v1/api/orgs/:orgID/plan/enterprise` behaviour note:** if the org has an active Stripe subscription, it is cancelled before the plan is promoted. If cancellation fails the request returns 500 and the org is not promoted — `stripe_subscription_id` is preserved for manual reconciliation.

To test locally with the Stripe CLI:

```bash
stripe listen --forward-to localhost:8080/stripe/webhook
stripe trigger checkout.session.completed
```

### Organisation management (system/root)

```bash
# Create an organisation
curl -X POST http://localhost:8080/v1/api/orgs \
  -H "Authorization: Bearer <system-token>" \
  -H "Content-Type: application/json" \
  -d '{"name": "Acme NGO"}'

# List organisations
curl http://localhost:8080/v1/api/orgs \
  -H "Authorization: Bearer <system-token>"

# Add a member
curl -X POST http://localhost:8080/v1/api/orgs/1/members \
  -H "Authorization: Bearer <system-token>" \
  -H "Content-Type: application/json" \
  -d '{"user_id": 3, "role": "owner"}'
```

Role hierarchy for member assignment: `owner` → can assign `admin`, `operational`; `admin` → can assign `operational`; `operational` → no assignment rights. Root and system bypass the check.

---

## 7. Roles and initial setup

The system uses two tiers of roles:

**System-level** (stored on `users.role`, embedded in JWT):

| Role | Set by | Can do |
|---|---|---|
| `root` | Direct DB write only | Assign `system` role |
| `system` | `root` via API | Create orgs, assign org members |

**Org-level** (stored in `org_members`):

| Role | Assigned by | Can do |
|---|---|---|
| `owner` | `system`/`root` | Manage `admin` and `operational` members |
| `admin` | `owner`/`system`/`root` | Manage `operational` members |
| `operational` | `admin`/`owner`/`system`/`root` | Day-to-day operations |

### Bootstrap a fresh deployment

Use the `seed-root` Makefile target to create the first root user (runs migrations automatically before inserting):

```bash
# From server/
make seed-root EMAIL=admin@example.com PASSWORD=secret
```

Or run it directly with flags or environment variables:

```bash
go run ./cmd/seed-root/main.go -email admin@example.com -password secret
# or
SEED_ROOT_EMAIL=admin@example.com SEED_ROOT_PASSWORD=secret go run ./cmd/seed-root/main.go
```

If a root user already exists the command exits cleanly without making any changes.

With Docker, run it as a one-off container or exec into a running server container:

```bash
# one-off container (shares the same network and DB as the compose stack)
docker compose -f .docker-dev/docker-compose.yml run --rm \
  -e SEED_ROOT_EMAIL=admin@example.com \
  -e SEED_ROOT_PASSWORD=secret \
  server ./seed-root

# exec into a running server container
docker compose -f .docker-dev/docker-compose.yml exec server \
  env SEED_ROOT_EMAIL=admin@example.com SEED_ROOT_PASSWORD=secret ./seed-root
```

> **Production guard**: when `APP_ENV=production` the command is allowed only during the initial bootstrap (no root user exists yet). Once a root user is present it exits with an error, preventing accidental creation of additional root users on a live deployment.

After the root user exists, `GET /v1/system/status` returns `{"configured":true}` and the webapp allows normal access.

---

## 8. Test Google login

Google OAuth requires a real browser redirect flow. The full end-to-end flow is:

1. Browser navigates to `GET /v1/auth/google?locale=<en|it>` → server stores locale in an `oauth_locale` cookie and redirects to Google consent screen.
2. User approves → Google redirects to `GET /v1/auth/google/callback?code=...&state=...`.
3. Server exchanges the code for a JWT, then redirects the browser to `{FRONTEND_URL}/en/auth/callback?token=<jwt>`.
4. The webapp callback page (`/en/auth/callback`) stores the token in `localStorage` and navigates to `/dashboard`.

To test locally you need both the server and the webapp running:

```bash
# Terminal 1 — server
go run .

# Terminal 2 — webapp (from charity-chest/webapp)
npm run dev
```

Then open `http://localhost:8080/v1/auth/google?locale=en` (or `?locale=it`) in your browser and complete the consent screen. You will land on the webapp dashboard in the correct locale automatically.

> **`FRONTEND_URL`**: defaults to `http://localhost:3000`. Set it in `.env` if your webapp runs on a different port or domain.

---

## 9. Cache (Valkey)

The server supports an optional Valkey (Redis-compatible) cache layer to reduce database load on read-heavy endpoints. It is **disabled by default** — enable it with `CACHE_ENABLED=true`.

### What is cached

| Cache key | Endpoint | Invalidated by |
|---|---|---|
| `system:status` | `GET /v1/system/status` | Only `configured=true` is cached; `configured=false` is never stored (avoids stale response after `seed-root` runs) |
| `user:{id}` | `GET /v1/api/me` | MFA enable/disable, `assign-role`, Google account link |
| `orgs:list` | `GET /v1/api/orgs` | Create / update / delete org |
| `org:{id}` | `GET /v1/api/orgs/:orgID` | Update / delete org |
| `org:{id}:members` | `GET /v1/api/orgs/:orgID/members` | Add / update / remove member, delete org |
| `admin:users:{email}:{page}:{size}` | `GET /v1/api/admin/users` | Register, assign-role, any member change |

### Run locally with cache enabled

```bash
# Start Valkey with Docker
docker run -d -p 6379:6379 valkey/valkey:8-alpine

# Enable in .env
CACHE_ENABLED=true
CACHE_URL=redis://localhost:6379
CACHE_TTL=5m

go run .
```

### Cache failures are non-fatal

If Valkey is unreachable or a cache operation fails, the server logs the error and falls through to the database. API responses remain correct; only performance is affected.

---

## 10. Plans

Every organisation belongs to one of three tiers:

| Plan | Owners | Admins | Operationals | Activation |
|---|---|---|---|---|
| `free` | 1 | not allowed | up to 5 | default |
| `pro` | 1 | up to 3 | up to 15 | Stripe Checkout |
| `enterprise` | unlimited | unlimited | unlimited | manual (root/system API) |

Member-limit enforcement happens in `AddMember` and `UpdateMember`. The org row is locked inside a DB transaction (`SELECT … FOR UPDATE`) so the count check and the insert/update are atomic — concurrent requests cannot race past the cap. Existing members that exceed a plan's limits after a downgrade are not removed; only new additions are blocked.

---

## 11. Deploy to production

```bash
make build-release
# Copy dist/release/server to the target host and run it
```

Set environment variables directly on the host (do not ship a `.env` file). The server reads them from the process environment automatically.

`APP_ENV=production` is **required** and must be set before starting. The server will refuse to start without it. Required variables in a production deployment:

| Variable | Notes |
|---|---|
| `APP_ENV` | Must be `production` |
| `DATABASE_URL` | PostgreSQL connection string |
| `JWT_SECRET` | Long random secret — `openssl rand -hex 32` |
| `GOOGLE_CLIENT_ID` | Google OAuth credentials |
| `GOOGLE_CLIENT_SECRET` | Google OAuth credentials |
| `STRIPE_SECRET_KEY` | Required if billing is enabled |
| `STRIPE_WEBHOOK_SECRET` | Required when `STRIPE_SECRET_KEY` is set |
| `STRIPE_PRO_PRICE_ID` | Required when `STRIPE_SECRET_KEY` is set |