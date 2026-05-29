# Charity Chest — Operational backend

Go HTTP gateway for the operational mobile app (`services/operational/app`). Targets end users on iOS and Android.

It is **stateless re: user data** — every credential check and user lookup is delegated to the admin backend via `/v1/internal/*`, authenticated with a shared `X-Service-Key` header. Operational signs its own short-lived JWTs for the mobile app; it does not reuse admin's JWT secret.

The service still ships with its own Postgres + GORM + cache scaffolding so future operational-owned entities (per-user operational state, etc.) can be added without re-bootstrapping. In v1 the schema is empty.

## What's in v1

- `POST /v1/auth/login` — email + password → operational JWT.
- `POST /v1/auth/google` — Google ID token (verified server-side with `google.golang.org/api/idtoken`) → operational JWT.
- `GET  /v1/api/me` — operational JWT in → user profile from admin out.
- `GET  /health` — liveness probe (unversioned).

## Known v1 limitations

- **MFA is not supported on mobile.** Admin reports `mfa_enabled=true` → operational returns HTTP 409 with a localised "use the admin web app" message.
- **No password recovery on mobile.** Users still recover via the admin webapp.
- **Logout is client-only.** The mobile app drops its token; operational does not maintain a revocation list.

## Running locally

The operational compose joins the admin compose's docker network as `external: true`, so bring admin up first.

```bash
# 1. Generate the shared service-to-service secret and put it in BOTH
#    admin's and operational's .docker-dev/.env (same value).
openssl rand -hex 32

# 2. Bring up admin (Postgres + Valkey + Mailpit + server)
docker compose -f services/admin/backend/.docker-dev/docker-compose.yml up --build -d

# 3. Seed an admin root user (needed to test password login from the app)
make -C services/admin/backend seed-root EMAIL=root@example.com PASSWORD=changeme

# 4. Fill .docker-dev/.env with the shared SERVICE_API_KEY and GOOGLE_AUDIENCE
cp services/operational/backend/.docker-dev/.env.example services/operational/backend/.docker-dev/.env
$EDITOR services/operational/backend/.docker-dev/.env

# 5. Bring up operational (Postgres + Valkey + server). Reaches admin at server:8080
#    via the shared `charitychest_admin` docker network.
docker compose -f services/operational/backend/.docker-dev/docker-compose.yml up --build -d

# 6. Smoke test
curl localhost:8081/health
curl -X POST localhost:8081/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"root@example.com","password":"changeme"}'
```

## Environment variables

| Variable | Required | Description |
|---|---|---|
| `APP_ENV` | **yes** | One of `local`, `testing`, `staging`, `production`. |
| `DATABASE_URL` | **yes** | PostgreSQL DSN. Used even though v1 has no entities — present so future migrations land cleanly. |
| `JWT_SECRET` | **yes** | HS256 signing secret for operational's JWTs. Independent from admin's `JWT_SECRET`. |
| `ADMIN_BASE_URL` | **yes** | Admin backend base URL, e.g. `http://localhost:8080`. |
| `SERVICE_API_KEY` | **yes** | Shared secret matching admin's `SERVICE_API_KEY`. Sent as `X-Service-Key` on every admin call. |
| `GOOGLE_AUDIENCE` | **yes** | Google OAuth Web Client ID validated as the `aud` claim on Google ID tokens forwarded by the mobile app. |
| `PORT` | no | HTTP listen port (default `8081`). |
| `REQUEST_LOG_ENABLED` | no | Echo's RequestLogger middleware (default `true`). |
| `ADMIN_TIMEOUT` | no | Per-request timeout for admin calls (default `10s`). |
| `CACHE_ENABLED` | no | Enable Valkey caching (default `false`). |
| `CACHE_URL` | no | Valkey URL (default `redis://localhost:6379`). |
| `CACHE_TTL` | no | TTL for cached entries (default `5m`). |

## Tests

Unit tests use stdlib `testing`. Handler integration tests use stub `AdminAPI` and `GoogleValidator` implementations defined inside the test files; `internal/testdb` is wired up but not yet used because there are no entities.

```bash
make -C services/operational/backend test
make -C services/operational/backend test-coverage
```

Docker must be running for tests (`internal/testdb` boots a Postgres container via testcontainers).
