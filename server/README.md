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

# 2. Start everything (Postgres + server, migrations run automatically)
docker compose -f .docker-dev/docker-compose.yml up --build
```

The server is available at `http://localhost:8080`. Postgres is exposed on port `5432` for local inspection.

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
| `APP_ENV` | no | Set to `production` to signal a production environment. The `seed-root` command refuses to run when this is set to `production`. |

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

## 5. API reference

All application endpoints are versioned under `/v1/`. The health probe is unversioned.

### Health check

```bash
curl http://localhost:8080/health
```

### System status (public)

Returns whether the system has been bootstrapped (at least one `root` user exists in the database).

```bash
curl http://localhost:8080/v1/system/status
# {"configured":false}
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

## 6. Roles and initial setup

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

> **Production guard**: when `APP_ENV=production` is set the command is allowed only while no root user exists (bootstrap path). Once a root user is present it exits with an error, preventing accidental creation of additional root users on a live deployment.

After the root user exists, `GET /v1/system/status` returns `{"configured":true}` and the webapp allows normal access.

---

## 7. Test Google login

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

## 8. Deploy to production

```bash
make build-release
# Copy dist/release/server to the target host and run it
```

Set environment variables directly on the host (do not ship a `.env` file). The server reads them from the process environment automatically.