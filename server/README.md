# Charity Chest — Server

Go HTTP server built with [Echo v4](https://echo.labstack.com/). Supports email/password and Google OAuth authentication with JWT-based sessions.

## Prerequisites

- Go 1.22+ **or** Docker + Docker Compose (no local Go/Postgres needed)
- A Google Cloud project with OAuth 2.0 credentials
- **For running the test suite**: a running Docker daemon (the Go tests boot a `postgres:16-alpine` container on demand via `testcontainers-go`)

---

## Quick start with Docker (recommended for demos)

```bash
# 1. Create the secrets file
cp .docker-dev/.env.example .docker-dev/.env
# Edit .docker-dev/.env — add your GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET

# 2. Start everything (Postgres + Valkey + MailHog + server, migrations run automatically)
docker compose -f .docker-dev/docker-compose.yml up --build
```

The server is available at `http://localhost:8080`. Postgres is exposed on port `5432`, Valkey on port `6379`, MailHog SMTP on port `1025`, and the MailHog inbox UI at http://localhost:8025 — recovery emails sent during local dev land there and never leave the developer's machine.

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

## Staging image

`.docker-staging/Dockerfile` builds a self-contained, production-style image intended for deployment to a managed environment (ECS, Fly.io, Kubernetes, etc.). It does not ship a `docker-compose.yml` — staging connects to managed Postgres and Valkey through environment variables supplied at deploy time.

### Build

```bash
# Build for the host architecture (build context is the server/ directory)
docker build \
  -f .docker-staging/Dockerfile \
  -t charity-chest-server:staging \
  .
```

Differences from the dev image:

- Runs as an unprivileged `app` user (not root). `/app` and everything under it is owned by root and read-only to the runtime user (the `app` user has no home directory), so a compromised process cannot tamper with the binary, migrations, or the RDS CA bundle, nor write anywhere it can later execute from.
- Built with `-trimpath` for reproducible binaries.
- Declares a Docker `HEALTHCHECK` that probes `GET /health` every 30s (`--start-period=30s` to cover migrations on cold start). Orchestrators that honour healthcheck status (Docker Swarm, ECS, Fly.io) will mark the container unhealthy and replace it; Kubernetes ignores `HEALTHCHECK` and instead expects a `livenessProbe`/`readinessProbe` pointing at the same endpoint.

### Multi-architecture builds (amd64 / arm64)

The image inherits its architecture from the `golang:alpine` and `alpine:3.23` base images, both of which are published as multi-arch manifests for `linux/amd64` and `linux/arm64`. By default `docker build` produces an image for the host's architecture; use `--platform` to target a specific one — useful when you build on an Apple Silicon / Graviton workstation and deploy on an `x86_64` host (or vice versa):

```bash
# Build for x86_64 / amd64 hosts (most cloud VMs, including default ECS Fargate)
docker build --platform linux/amd64 \
  -f .docker-staging/Dockerfile \
  -t charity-chest-server:staging-amd64 \
  .

# Build for arm64 hosts (AWS Graviton, Apple Silicon, Ampere)
docker build --platform linux/arm64 \
  -f .docker-staging/Dockerfile \
  -t charity-chest-server:staging-arm64 \
  .
```

The Go compiler in the build stage cross-compiles natively (the `Dockerfile` already pins `CGO_ENABLED=0 GOOS=linux`, so `GOARCH` is inferred from `--platform` automatically) — the heavy step is the runtime stage, which executes commands inside an `alpine:3.23` rootfs for the target architecture.

**Emulation requirement:** when the target architecture differs from the host, the `RUN` commands in the runtime stage (`apk add`, `addgroup`, `adduser`, `chmod`) run under emulation. Docker Desktop on macOS and Windows ships QEMU emulation by default, so cross-arch builds work out of the box. On Linux, install the `binfmt` handlers once per machine before attempting a cross-arch build:

```bash
# Register QEMU handlers for all supported architectures (one-time per host).
# Requires root/sudo on the host kernel; runs as a privileged throwaway container.
docker run --privileged --rm tonistiigi/binfmt --install all

# Verify amd64 and arm64 are registered
docker run --privileged --rm tonistiigi/binfmt
```

Cross-arch builds are functionally correct under QEMU but noticeably slower than native (typically 3–10× for the runtime stage). For frequent rebuilds prefer a native runner per architecture (e.g. GitHub Actions `ubuntu-latest` for amd64 + `ubuntu-24.04-arm` for arm64) and assemble a multi-arch manifest from the two native builds.

To publish a single tag that resolves to the right architecture on any host (so one ECR/Docker Hub tag works on amd64 *and* arm64 deployments), build with `buildx` and push directly to a registry:

```bash
# One-time: create and select a buildx builder that supports multi-platform
docker buildx create --name multiarch --use
docker buildx inspect --bootstrap

# Build both architectures and push a single multi-arch manifest
docker buildx build --platform linux/amd64,linux/arm64 \
  -f .docker-staging/Dockerfile \
  -t <registry>/charity-chest-server:staging \
  --push \
  .
```

`buildx` requires `--push` (or `--output=type=registry`) for multi-platform builds — the local Docker image store cannot hold a multi-arch manifest, so `--load` only works when a single `--platform` is specified.

### Run

```bash
# Run the image matching the host architecture
docker run --rm \
  -e DATABASE_URL=... \
  -e JWT_SECRET=... \
  -e GOOGLE_CLIENT_ID=... \
  -e GOOGLE_CLIENT_SECRET=... \
  -e APP_ENV=staging \
  -p 8080:8080 \
  charity-chest-server:staging

# Force-run a non-native image under QEMU (e.g. test the amd64 image on an arm64 laptop)
docker run --rm --platform linux/amd64 \
  -e DATABASE_URL=... \
  -e JWT_SECRET=... \
  -e GOOGLE_CLIENT_ID=... \
  -e GOOGLE_CLIENT_SECRET=... \
  -e APP_ENV=staging \
  -p 8080:8080 \
  charity-chest-server:staging-amd64
```

Cross-arch runtime via QEMU is fine for smoke-testing on a developer machine; never use it in production — the latency overhead dwarfs the cost of just deploying the native-arch image. Pull the correct architecture from a multi-arch tag instead, or deploy the architecture-suffixed tag that matches the host.

The entry-point seeds the first root user (best-effort) and then `exec`s the server. Set the following two variables in the deployment environment to enable seeding:

| Variable | Description |
|---|---|
| `ROOT_USER` | Email address of the root user to seed on first boot |
| `ROOT_PASSWORD` | Password for that root user |

When either is unset, seeding is skipped and only the server starts. Seeding runs on every container start; subsequent attempts fail harmlessly when the user already exists and do not block startup.

To create or replace a root user as a one-off task instead:

```bash
docker run --rm \
  -e DATABASE_URL=... \
  -e APP_ENV=staging \
  charity-chest-server:staging \
  ./seed-root -email admin@example.com -password '...'
```

---

## Staging DBMS image (CloudBeaver web UI)

`.docker-dbms-staging/Dockerfile` builds a self-contained image that ships [CloudBeaver CE](https://github.com/dbeaver/cloudbeaver) — a web UI for browsing and querying the staging Postgres database. It is a thin wrapper around the upstream `dbeaver/cloudbeaver` image: it pins a version, installs `curl` for the Docker `HEALTHCHECK`, and exposes the default port. Like the API server image, it does not ship a `docker-compose.yml` — it is meant to be deployed standalone (ECS, Fly.io, Kubernetes, etc.) and connect to the managed Postgres through a connection the operator registers in the web UI on first launch.

### Build

```bash
docker build \
  -f .docker-dbms-staging/Dockerfile \
  -t charity-chest-dbms:staging \
  .docker-dbms-staging
```

By default `docker build` produces an image for the host's architecture. The upstream `dbeaver/cloudbeaver:26.0` base is published for both `linux/amd64` and `linux/arm64`, so you can target either explicitly with `--platform` — useful when you build on one arch (e.g. an Apple Silicon laptop) and deploy on another (e.g. an `x86_64` staging VM, or an AWS Graviton/`arm64` host):

```bash
# Build for x86_64 / amd64 hosts
docker build --platform linux/amd64 \
  -f .docker-dbms-staging/Dockerfile \
  -t charity-chest-dbms:staging-amd64 \
  .docker-dbms-staging

# Build for arm64 hosts (Graviton, Apple Silicon)
docker build --platform linux/arm64 \
  -f .docker-dbms-staging/Dockerfile \
  -t charity-chest-dbms:staging-arm64 \
  .docker-dbms-staging
```

Cross-arch builds run under QEMU emulation (slow, but functionally correct). If you need both architectures under a single tag — e.g. so one ECR/Docker Hub tag works on any host — produce a multi-arch manifest with `buildx` instead:

```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  -f .docker-dbms-staging/Dockerfile \
  -t <registry>/charity-chest-dbms:staging \
  --push \
  .docker-dbms-staging
```

`buildx` requires a registry (`--push`); the local Docker image store cannot hold a multi-arch manifest, so `--load` only works for a single platform.

### Run

CloudBeaver listens on port `8978` and stores all of its state — admin user, registered connections, saved credentials — under `/opt/cloudbeaver/workspace`. **A persistent volume on that path is mandatory**: without it every container restart wipes the configuration and you have to re-run the quickstart wizard.

```bash
docker run -d --name charity-chest-dbms \
  -p 8978:8978 \
  -v charity-chest-dbms-workspace:/opt/cloudbeaver/workspace \
  charity-chest-dbms:staging
```

### Initial setup

The first time you open `http://<host>:8978` CloudBeaver runs a quickstart wizard:

1. Create the admin user (username + password). These credentials are stored in the workspace volume.
2. Register the staging Postgres connection — host = staging Postgres endpoint, port `5432`, database + user from the staging credentials.

The wizard runs only on first launch. Subsequent boots read everything from the mounted workspace.

### Environment variables

CloudBeaver is configured through the web wizard and the workspace volume rather than environment variables — there are no required env vars at the image level. The full list of variables that the image honours is:

| Variable | Required | Description |
|---|---|---|
| `CLOUDBEAVER_WORKSPACE` | no | Workspace path inside the container (default `/opt/cloudbeaver/workspace`). Only change this if you also adjust the volume mount target. |

Postgres credentials are entered into the CloudBeaver UI on first setup; they are **not** passed to this container as environment variables. Treat the workspace volume as a secret store and back it up accordingly.

### Security

CloudBeaver has no built-in TLS. Always front it with a reverse proxy that terminates TLS (ALB, Cloud Run, Nginx, Caddy) and restrict access by VPN, IP allow-list, or SSO — the admin UI holds full DB credentials.

### Updating the CloudBeaver version

The Dockerfile pins a specific upstream tag. To upgrade, check [Docker Hub](https://hub.docker.com/r/dbeaver/cloudbeaver/tags) for the latest release and bump the `FROM dbeaver/cloudbeaver:<tag>` line — keep the workspace volume across rebuilds so the existing configuration survives.

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
| `REQUEST_LOG_ENABLED` | no | Set to `false` to disable Echo's per-request access log middleware (default `true`) |
| `STRIPE_SECRET_KEY` | no | Stripe secret key. When unset, all billing endpoints return 503. |
| `STRIPE_WEBHOOK_SECRET` | if Stripe enabled | Stripe webhook signing secret. **Required** when `STRIPE_SECRET_KEY` is set — the server refuses to start without it. Also enforced at runtime: `POST /stripe/webhook` returns 503 immediately whenever this is unset, in any environment, so unsigned events can never alter plan state. |
| `STRIPE_PRO_PRICE_ID` | if Stripe enabled | Stripe Price ID for the Pro plan (e.g. `price_xxx`). **Required** when `STRIPE_SECRET_KEY` is set. |
| `SMTP_HOST` | no | SMTP relay host for the password-recovery email. Leave empty to disable email entirely — the forgot-password endpoint still returns a neutral 204 and a server-side warning is logged (no 503; that would be an enumeration signal). |
| `SMTP_PORT` | no | TCP port for the SMTP relay (default `587`). Use `1025` for MailHog in dev. |
| `SMTP_USERNAME` | if SMTP enabled & relay requires AUTH | Optional. Paired with `SMTP_PASSWORD`: set both or neither. When empty the mailer skips the AUTH step (MailHog and many internal relays reject AUTH outright). |
| `SMTP_PASSWORD` | if SMTP enabled & relay requires AUTH | Optional. See above — must be set together with `SMTP_USERNAME`. |
| `SMTP_FROM` | if SMTP enabled | Sender address. **Required** when `SMTP_HOST` is set — the server refuses to start otherwise. |
| `SMTP_FROM_NAME` | no | Sender display name (default `Charity Chest`). |

### SMTP — local dev with MailHog

The dev compose stack at `.docker-dev/docker-compose.yml` ships a MailHog container so recovery emails never leave the developer's machine.

| Service | URL |
|---|---|
| SMTP submission | `mailhog:1025` (inside the compose network), `localhost:1025` from the host |
| Captured-email UI | http://localhost:8025 |

The server is wired with `SMTP_HOST=mailhog`, `SMTP_PORT=1025`, `SMTP_FROM=no-reply@charitychest.local`, and **no credentials** (MailHog rejects AUTH).

### SMTP — staging / production

There is intentionally no `.docker-mailhog-staging/Dockerfile`. MailHog is a capture server, not an MTA — outside dev it would silently swallow every email a real user expects to receive. Point `SMTP_*` at a production relay instead:

| Relay | Host | Port | Notes |
|---|---|---|---|
| AWS SES | `email-smtp.<region>.amazonaws.com` | `587` | SMTP credentials generated under IAM (separate from the AWS access key) |
| Mailgun | `smtp.mailgun.org` | `587` | Use the SMTP credentials from the verified domain |
| Postmark | `smtp.postmarkapp.com` | `587` | Username == password == the server's API token |

Whichever relay you pick, set both `SMTP_USERNAME` and `SMTP_PASSWORD` and configure DKIM/SPF on the sending domain before any traffic hits production — most relays will refuse mail otherwise.

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
| `make build-debug` | `dist/debug/server` | Race detector on, optimisations off — debugger-friendly. Runs `templ generate` first. |
| `make build-release` | `dist/release/server` | Static binary, debug info stripped, optimised for deployment. Runs `templ generate` first. |
| `make test` | — | Runs the unit + integration test suite under `-race`. **Requires a running Docker daemon** — testcontainers-go boots a `postgres:16-alpine` instance for the duration of the run. |
| `make test-coverage` | `coverage.out`, `coverage.business.out` | Reports total coverage and "business" coverage (excludes `main.go`, `cmd/*`, and generated `*_templ.go`). Same Docker requirement as `make test`. |
| `make templ` | `*_templ.go` | Regenerates Go from every `.templ` source under `internal/templates/` |
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

### Password recovery (forgot / reset)

The flow is enumeration-safe. `POST /v1/auth/password/forgot` always returns `204 No Content` regardless of whether the email maps to an account. `POST /v1/auth/password/reset` returns `204 No Content` on success and collapses every failure mode (missing, malformed, expired, or already-used token) into a single `400 Bad Request` carrying the `password_reset_token_invalid` i18n key, so an attacker cannot distinguish which tokens ever existed.

```bash
# Step 1 — request a reset link.
# Always 204, even for unknown emails.
curl -X POST http://localhost:8080/v1/auth/password/forgot \
  -H "Content-Type: application/json" \
  -H "Accept-Language: en" \
  -H "X-Locale: en" \
  -d '{"email":"you@example.com"}'

# Step 2 — consume the token in the link (the user gets it via email).
curl -X POST http://localhost:8080/v1/auth/password/reset \
  -H "Content-Type: application/json" \
  -d '{"token":"<the-token-from-the-email>","password":"newpassword1"}'
```

Tokens are 32 random bytes (base64url), only their SHA-256 hex digest is stored, they expire after 1 hour, and they are single-use. A successful reset also invalidates every other outstanding token for the same user. The endpoint does **not** issue a JWT — the user has to log in again (and pass MFA if enabled).

In dev, recovery emails land in the MailHog UI at http://localhost:8025.

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
- `checkout.session.completed` — if the org is already on the enterprise plan, the handler persists a `BillingCleanupJob` row (with the duplicate subscription ID and payment intent ID) **before** acknowledging the webhook, then attempts to cancel the new Stripe subscription and refund the initial payment in-line. The webhook is acknowledged with 200 once the cleanup job is durable; only DB persistence errors return 500 (so Stripe retries). Stripe call failures are recorded in `last_error` on the job row for an out-of-band retry worker. The org's plan is never changed.
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