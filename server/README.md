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

### Access a protected route

```bash
curl http://localhost:8080/v1/api/me \
  -H "Authorization: Bearer <token>"
```

---

## 6. Test Google login

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

## 7. Deploy to production

```bash
make build-release
# Copy dist/release/server to the target host and run it
```

Set environment variables directly on the host (do not ship a `.env` file). The server reads them from the process environment automatically.