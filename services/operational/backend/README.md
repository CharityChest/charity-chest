# Operational Backend

Node.js + TypeScript HTTP gateway for the Charity Chest mobile app. It is **stateless with respect to user identity** — every credential check and profile read is delegated to the **admin** backend's `/v1/internal/*` service-to-service API. It owns no user tables; it only signs and verifies its own short-lived JWTs for the mobile client.

## Responsibilities

- **Login** (`POST /v1/auth/login`) — forwards email/password to admin, issues an operational JWT on success.
- **Google sign-in** (`POST /v1/auth/google`) — verifies the Google ID token, delegates find-or-create to admin, issues an operational JWT.
- **Profile** (`GET /v1/api/me`) — validates the operational JWT and returns the caller's profile fetched from admin.
- **Health** (`GET /health`) — liveness probe.

MFA-enabled accounts are rejected with `409` on both login paths — the mobile app does not yet implement the TOTP step.

The backend has its own PostgreSQL + Valkey scaffolding for future operational-only entities, but v1 defines no tables and caches nothing.

## Tech stack

| Concern | Choice |
|---|---|
| Language | TypeScript (strict), Node.js 24 |
| HTTP framework | Express 5 |
| Auth | JWT (HS256) via `jsonwebtoken`; password check delegated to admin |
| Identity source | admin backend via `/v1/internal/*` (service key auth) |
| Google verify | `google-auth-library` (`OAuth2Client.verifyIdToken`) |
| Datastore (future) | PostgreSQL (`pg`), Valkey/Redis (`redis`) |
| Tests | Vitest + Supertest |

## Project layout

```
src/
├── main.ts                 # entry point: config → migrations → app → listen
├── app.ts                  # createApp(deps) — wires middleware + routes (test seam)
├── config.ts               # env loading + validation (+ Go-style duration parsing)
├── i18n.ts                 # en/it message catalog + locale parsing
├── http.ts                 # {data} envelope, HttpError, terminal error handler
├── db.ts                   # reserved Postgres pool (no entities in v1)
├── cache.ts                # reserved Valkey/Redis wrapper (disabled by default)
├── migrate.ts              # SQL migration runner (no-op when migrations/ is empty)
├── google.ts               # GoogleValidator interface + google-auth-library impl
├── adminclient/            # typed client for admin's /v1/internal/* API
├── handlers/               # auth (login/google) + me
└── middleware/             # locale, jwt
```

All non-`main` modules are unit-tested; `app.ts` is exercised end-to-end through Supertest, mirroring the previous Go `routes_test.go`.

## Local development

The operational backend talks to the admin backend, so bring admin up first, then this service. The simplest path is the unified compose stack at the repo root (`.compose/`), which wires both together.

### Option A — unified stack (recommended)

From the repo root, see [`.compose/README.md`](../../../.compose/README.md):

```bash
docker compose -f .compose/docker-compose.yml up --build
```

### Option B — this service's own compose

```bash
cp services/operational/backend/.docker-dev/.env.example services/operational/backend/.docker-dev/.env
$EDITOR services/operational/backend/.docker-dev/.env
# set SERVICE_API_KEY (match admin) and GOOGLE_AUDIENCE

docker compose -f services/operational/backend/.docker-dev/docker-compose.yml up --build -d
```

### Option C — run on the host

```bash
cd services/operational/backend
npm ci
cp .env.example .env && $EDITOR .env
npm run dev          # tsx watch — restarts on change
```

## Configuration

All via environment variables (see `.env.example`). Required: `APP_ENV`, `DATABASE_URL`, `JWT_SECRET`, `ADMIN_BASE_URL`, `SERVICE_API_KEY`, `GOOGLE_AUDIENCE` (comma-separated list of accepted Google OAuth client IDs — the iOS/Android/Web client IDs the mobile app ships with; a forwarded ID token's `aud` must match one, single value also works). Optional: `PORT` (8081), `REQUEST_LOG_ENABLED`, `ALLOWED_ORIGINS` (comma-separated CORS origins, default `http://localhost:3000`; never `*`), `ADMIN_TIMEOUT` (`10s`), `CACHE_*`. `ADMIN_TIMEOUT` and `CACHE_TTL` accept Go-style durations (`10s`, `5m`, `1h30m`) for drop-in compatibility with the existing compose files.

## Scripts

```bash
npm run dev            # watch-mode dev server (tsx)
npm run build          # tsc → dist/
npm start              # node dist/main.js
npm run typecheck      # tsc --noEmit
npm run lint           # eslint
npm test               # vitest run
npm run test:ci        # vitest run --coverage (enforces ≥ 80% via vitest.config.ts)
```

`make test` / `make test-coverage` (and friends) wrap these for parity with the rest of the monorepo. No Docker is required for the test suite — admin and Google are faked, and the migration runner's DB path is not exercised in v1.

## Architecture notes

- **No user storage.** The service holds no identity tables. A compromise of its database exposes no user data.
- **Independent JWT secret.** `JWT_SECRET` differs from admin's on purpose — an operational token can't be replayed against admin, and vice-versa. Tokens carry the public `user_uuid` claim (plus `email`), HS256, 24h expiry.
- **Service key.** Every admin call carries `X-Service-Key`; admin rejects missing/incorrect keys. See the admin backend's internal API docs.
- **Locale forwarding.** The `X-Locale` request header is propagated to admin so upstream error messages are localized.
- **Upstream failures** (transport error, timeout, or 5xx from admin) surface as `502`. Admin's `401` on login becomes `401`; a `404` on `GET /v1/api/me` becomes `404`.
