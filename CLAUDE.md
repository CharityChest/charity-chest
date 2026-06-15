# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Repository layout

This repo is a microservices monorepo. Each service lives under `services/<name>/` with a `backend/` and either a `frontend/` (web) or `app/` (mobile). There are two services today:

- `services/admin/backend/` — Go HTTP API (owns user identity, billing, orgs).
- `services/admin/frontend/` — Next.js 15 webapp targeting admin users.
- `services/operational/backend/` — Node.js + TypeScript (Express 5) HTTP gateway for the mobile app. Stateless re: user data — delegates every identity read/write to admin via gRPC (`AdminInternal` service, see "Service-to-service gRPC API"). Has its own DB scaffolding for future operational-only entities; today the DB has no entities. Code lives under `src/`; tests are co-located `*.test.ts` (Vitest + Supertest). See `services/operational/backend/README.md`.
- `services/operational/app/` — Expo + React Native + TypeScript mobile app (iOS + Android).

At the repo root, `.compose/` holds the **unified development stack** (`docker-compose.yml` + `.env.example` + `README.md`) — one compose that brings up admin (Postgres + Valkey + Mailpit + Go API + Next.js webapp) and operational (its own Postgres + Valkey + Node.js API) on a single docker network named `charitychest`. Services are renamed `admin-backend` / `op-backend` / `admin-frontend` / `admin-postgres` / `op-postgres` / `admin-valkey` / `op-valkey` / `mailpit` so they coexist; operational reaches admin's gRPC server in-cluster via `admin-backend:9090`. `SERVICE_API_KEY` is declared once in `.compose/.env` and passed to both backends. The per-service `.docker-dev/docker-compose.yml` files still work in isolation but expose the same host ports — never run both modes at once.

The admin backend is a Go module (`charity-chest/services/admin/backend`); its imports are `<module>/internal/...`. The operational backend is a TypeScript package (`charity-chest-operational-backend`) with sources under `src/` and relative imports.

Inside `services/admin/backend/`:
- `main.go` — entry point: config → migrations → routes → listen.
- `cmd/seed-root/` — CLI that creates the first root user. Accepts `-email`/`-password` flags or `SEED_ROOT_EMAIL`/`SEED_ROOT_PASSWORD` env vars; refuses to run when `APP_ENV=production` and a root user already exists.
- `internal/` — all non-`main` code lives here (no `pkg/`).
  - `cache/`, `config/`, `grpc/adminpb/` (generated gRPC bindings), `i18n/`, `middleware/`, `model/`, `handler/`, `routes/v1/`, `templates/<category>/`, `testdb/` (shared per-test Postgres harness; not imported from production).
- `migrations/` — `golang-migrate` SQL files (`NNNNNN_<description>.{up,down}.sql`).
- `.docker-dev/` — Compose stack (Postgres + Valkey + Mailpit + server).
- `.docker-staging/` — standalone server image (no compose).
- `.docker-dbms-staging/` — standalone CloudBeaver image for the staging DB web UI.

Inside `services/admin/frontend/`:
- `src/app/[locale]/` — every page is under the locale prefix.
- `src/components/`, `src/i18n/`, `src/lib/`, `src/types/`, `src/test/`.
- `messages/{en,it}.json` — i18n strings.
- `.docker-dev/` and `.docker-staging/` — Compose dev stack and standalone staging image.

When you need a specific file, list the directory rather than relying on this section staying current.

---

## Server tech stack

| Concern | Library |
|---|---|
| HTTP framework | `github.com/labstack/echo/v4` |
| ORM | `gorm.io/gorm` + `gorm.io/driver/postgres` |
| Migrations | `github.com/golang-migrate/migrate/v4` (file source) |
| Auth tokens | `github.com/golang-jwt/jwt/v5` (HS256, 24h expiry; MFA-pending 5min) |
| TOTP / MFA | `github.com/pquerna/otp/totp` |
| Password hashing | `golang.org/x/crypto/bcrypt` (DefaultCost) |
| Google OAuth | `golang.org/x/oauth2` + `.../google` |
| Config | `github.com/joho/godotenv` (dev only) + `os.Getenv` |
| Cache | `github.com/redis/go-redis/v9` (Valkey-compatible; disabled by default) |
| HTML templates | `github.com/a-h/templ` (`.templ` → `*_templ.go`) |
| Email | `github.com/wneessen/go-mail` (SMTP) |
| Test DB | Real Postgres via `testcontainers-go`; see `internal/testdb` |

---

## API versioning

All application routes are prefixed `/v1/`. The `/health` probe is intentionally unversioned — it's an infrastructure endpoint, not part of the API contract.

When a breaking change is needed, introduce `/v2/` alongside `/v1/` in `main.go`, add `RegisterFoo` functions under `internal/routes/v2/`, and keep both alive until clients migrate.

The authoritative list of routes lives in `internal/routes/v1/*.go` — read those files rather than maintaining a duplicate table here.

---

## Service-to-service gRPC API

Sibling backends in this monorepo (today: `services/operational/backend/`) call admin via the **`AdminInternal` gRPC service**. It exists so other services can validate credentials and look up users without holding their own copy of the identity tables.

- **Proto**: canonical definition at `services/admin/backend/proto/admin/internal/v1/internal.proto` (within the admin backend's Docker context). Generated Go bindings in `services/admin/backend/internal/grpc/adminpb/` (checked in — `go build` works without a separate codegen step). The operational backend loads the proto dynamically at runtime from `src/adminclient/internal.proto` (a copy, kept in sync by `make proto` in the admin backend).
- **Auth**: every gRPC call must include `x-service-key: <shared-secret>` in metadata. The `handler.ServiceKeyInterceptor` validates it in constant time before any method runs. Missing → `UNAUTHENTICATED`; mismatch → `PERMISSION_DENIED`.
- **Env policy**: `SERVICE_API_KEY` is **optional**. When unset, `main.go` skips starting the gRPC server (callers get connection-refused). `GRPC_PORT` defaults to `9090`.
- **No JWTs issued**: callers sign their own tokens. Admin returns only a slim `UserDTO` (`uuid`, `email`, `name`, `role`, `mfa_enabled`) — never `id`, `password_hash`, `totp_secret`, `google_id`.
- **Methods** (`handler/grpc_server.go`):
  - `Login(email, password)` → `UserDTO`; `UNAUTHENTICATED` on every credential failure mode (no enumeration).
  - `GoogleAuth(google_sub, email, name)` → `UserDTO`; find-or-create via the same `findOrCreateGoogleUser` helper used by the browser OAuth callback.
  - `GetUser(user_uuid)` → `UserDTO`; `NOT_FOUND` if no row.

When you add a new method, add it to the proto, run `make proto`, implement it in `handler/grpc_server.go`, and return only `UserDTO` (or a similarly restricted message) — never `model.User` directly. Regenerating also updates the operational backend's proto copy.

---

## Entity identifiers (UUID v4)

Every domain entity (`users`, `organizations`, `org_members`, `billing_cleanup_jobs`, `password_reset_tokens`) carries two identifiers:

- **Integer `id`** (`SERIAL PRIMARY KEY`) — internal. Used for foreign keys, cache keys, log lines, and DB joins. **Never appears in API responses, URLs, request bodies, or JWT claims.** Tagged `json:"-"`.
- **`uuid` (UUID v4)** (`UUID NOT NULL UNIQUE DEFAULT gen_random_uuid()`) — public. Used in URL path params (`:orgUUID`, `:userUUID`), in JSON responses as `"uuid"`, and in request bodies referencing another entity (`{"user_uuid": "..."}`). Generated by Postgres' `gen_random_uuid()` (`pgcrypto`); GORM reads it back after `Create`.

Request-time flow:
1. Path param arrives as `:orgUUID` / `:userUUID`.
2. Handler (or middleware) parses with `uuid.Parse(...)` and looks up the row by `uuid` to get the int id.
3. All downstream code — DB, cache, transactions, logs — uses the int id.
4. JSON output exposes only `"uuid"`.

JWT claims embed the user's public **`user_uuid`** (not the int id). The `JWT(db, secret)` auth middleware (`middleware/jwt.go`) validates the token, then resolves `user_uuid` → int id with a single `SELECT id FROM users WHERE uuid = ?` and injects the **int** id under `middleware.UserIDContextKey` — so every handler, cache key, and FK query downstream keeps using the int id unchanged. A valid signature over a UUID that maps to no user (e.g. a deleted account) is rejected as `KeyInvalidToken` (401). Trade-off: this costs one DB lookup per authenticated request (the previous int-in-JWT design avoided it), in exchange for the token carrying only the public identifier — consistent with URLs and request bodies.

For org-scoped routes, `RequireOrgRole` resolves `:orgUUID` to the int id once and injects it under `middleware.OrgIDContextKey`; handlers read it with `orgIDFromContext(c)`. Handlers behind `RequireSystemRole` only (`UpdateOrg`, `DeleteOrg`, `AssignEnterprisePlan`) resolve the UUID themselves via helpers in `handler/organization.go` (`loadOrgByUUID`, `resolveUserIDByUUID`, `resolveUserIDFromUUIDParam`).

New entities follow the same pattern: `ID uint` with `json:"-"`, `UUID uuid.UUID` with `gorm:"type:uuid;not null;uniqueIndex;default:gen_random_uuid()" json:"uuid"`, and `uuid UUID NOT NULL UNIQUE DEFAULT gen_random_uuid()` in the migration.

---

## Secret management rules

- Secrets live **only** in environment variables — never hardcoded, never committed.
- `services/admin/backend/.env` and `services/admin/backend/.docker-dev/.env` are git-ignored. Copy from the matching `.env.example`.
- `config.Load()` calls `godotenv.Load()` silently (ignored in production) then validates required vars, returning an error that names every missing variable.
- **Required**: `DATABASE_URL`, `JWT_SECRET`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `APP_ENV`.
- `APP_ENV` must be one of `local`, `testing`, `staging`, `production`. Compare against the typed constants `config.AppEnv*` — never bare string literals.
- **Optional with defaults**: `GOOGLE_REDIRECT_URL` (`http://localhost:8080/v1/auth/google/callback`), `FRONTEND_URL` (`http://localhost:3000`), `PORT` (`8080`), `GRPC_PORT` (`9090` — the port the gRPC server listens on when `SERVICE_API_KEY` is set).
- **Cache**: `CACHE_ENABLED` (`false`), `CACHE_URL` (`redis://localhost:6379`), `CACHE_TTL` (`5m`).
- `REQUEST_LOG_ENABLED` (`true`): when `false`, Echo's access log middleware isn't mounted.
- **Stripe** (companion group — billing endpoints return 503 when `STRIPE_SECRET_KEY` is unset): `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `STRIPE_PRO_PRICE_ID`. When `STRIPE_SECRET_KEY` is set, the other two are required; `Load()` reports every missing companion.
- **SMTP** (companion group — when `SMTP_HOST` is unset, the recovery email is silently skipped, the forgot-password endpoint still returns the neutral 204, and a server-side warning is logged; **no 503** because that would be an enumeration signal): `SMTP_HOST`, `SMTP_PORT` (`587`), `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM`, `SMTP_FROM_NAME` (`Charity Chest`), `SMTP_FORCE_IPV4` (`true`).
  - When `SMTP_HOST` is set, `SMTP_FROM` is required.
  - `SMTP_USERNAME` / `SMTP_PASSWORD` are an **optional pair** (both or neither) so capture servers like Mailpit (which rejects AUTH) and internal relays work; the mailer skips AUTH when both are empty.
  - `SMTP_FORCE_IPV4` pins the dial to `tcp4`. Many self-hosted relays publish unreachable AAAA records; the default dual-stack dial then times out.
- `FRONTEND_URL` is used by `GoogleCallback` to redirect back to the webapp with the JWT.
- **gRPC** (optional group — `SERVICE_API_KEY` enables the `AdminInternal` gRPC server; when unset, `main.go` skips starting it entirely): `SERVICE_API_KEY`, `GRPC_PORT` (`9090`). `GRPC_PORT` is only read when `SERVICE_API_KEY` is non-empty.
- `ROOT_USER` / `ROOT_PASSWORD` are **container-level** vars consumed by the entry-point scripts, not by `config.Load()`. Required in dev compose; optional in staging (entry-point seeds best-effort and continues even if seeding fails).

---

## Database migrations

- Plain SQL in `services/admin/backend/migrations/`, named `NNNNNN_<description>.{up,down}.sql`.
- Run automatically on server start (`migrate.Up()` in `main.go`). `ErrNoChange` is ignored; any other error is fatal.
- Never modify an already-applied migration — add a new one.
- Always add a matching `.down.sql` for every `.up.sql`.

---

## Server code conventions

- **Package layout**: all non-main code under `internal/`. No `pkg/`.
- **Response envelope**: successful JSON responses are wrapped as `{"data": <payload>}` via `dataJSON` in `handler/response.go`. Paginated responses use `dataWithMetaJSON` → `{"data": [...], "metadata": {"page","size","total","total_pages"}}` with `page` (default 1) and `size` (default 20, max 100) query params. Echo's `HTTPError` (`{"message": "..."}`) is not wrapped.
- **Errors**: handlers return `echo.NewHTTPError(statusCode, message)`. Never swallow errors silently.
- **No user enumeration**: login returns a generic 401 for both "user not found" and "wrong password".
- **i18n**: all error messages translate through `internal/i18n`. The global `Locale` middleware parses `Accept-Language` into `middleware.LocaleEN` / `LocaleIT` (default EN). Handlers call `i18n.T(locale(c), i18n.KeyXxx)`. New messages need both EN and IT translations.
- **Named constants over magic strings**: use `middleware.UserIDContextKey` / `EmailContextKey` / `RoleContextKey` / `LocaleContextKey` / `LocaleEN` / `LocaleIT`; `handler.CookieOAuthState` / `CookieOAuthLocale`; `model.RoleRoot` / `RoleSystem` / `OrgRoleOwner` etc.; `config.AppEnv*`. Never repeat bare string literals for these.
- **Sensitive fields**: `PasswordHash` and `GoogleID` are `json:"-"` — must never appear in API responses.
- **Nullable columns**: `PasswordHash`, `GoogleID`, `Role` are `*string`; nil means unset.
- **Unit tests**: one `_test.go` per source file, `package foo_test` (black-box). Each test gets a fresh per-test Postgres DB via `newTestDB(t)` (wraps `testdb.Open(t)`). The first call boots a `postgres:16-alpine` container and applies migrations to a template; subsequent calls clone it with `CREATE DATABASE ... TEMPLATE` (fast) and drop on cleanup. **Docker must be running.**
- **E2e tests**: `internal/routes/v1/routes_test.go` exercises every endpoint through the full Echo stack. `newServer(t)` wires all `Register*` against the same per-test Postgres.
- **No testify**: standard `testing` package only.
- **Cache in tests**: pass `cache.Disabled()` for basic handler tests. For cache hit/miss/invalidation tests use an in-process `miniredis` via `newMiniRedisCache(t)` (defined in `auth_test.go`).
- **Cache errors are non-fatal**: log with `log.Printf` and fall through to the database. Never skip invalidation on a successful write.
- **Templates**: templ sources live under `internal/templates/<category>/`, one file per template. The subdirectory is the Go package — keep it tightly scoped (`email`, `page`, …). Per-template data structs go in `types.go` alongside the `.templ`. Run `make templ` after editing (`make build-*` invokes it automatically). Generated `*_templ.go` is checked in so plain `go build` works.
- **gRPC bindings**: proto source at `proto/admin/internal/v1/internal.proto`. Run `make proto` after editing (`make build-*` invokes it automatically; requires `protoc` in `PATH`). Generated `*pb.go` files are checked in.

---

## Cache system

Wrapper around Valkey (Redis-compatible) via `go-redis/v9`, fully transparent to the API.

### Key scheme

| Key | Endpoint | Invalidated by |
|---|---|---|
| `system:status` | `GET /v1/system/status` | Only `configured=true` is cached; `false` is never stored |
| `user:{id}` | `GET /v1/api/me` | EnableMFA, DisableMFA, AssignSystemRole, Google link, ResetPassword |
| `orgs:list` | `GET /v1/api/orgs` | CreateOrg, UpdateOrg, DeleteOrg |
| `org:{id}` | `GET /v1/api/orgs/:orgUUID` | UpdateOrg, DeleteOrg |
| `org:{id}:members` | `GET /v1/api/orgs/:orgUUID/members` | AddMember, UpdateMember, RemoveMember, DeleteOrg |
| `admin:users:{email}:{page}:{size}` | `GET /v1/api/admin/users` | Register, AssignSystemRole, any member change, Google create/link, ResetPassword |

Use the builders in `internal/cache/keys.go` (`cache.KeyUser(id)`, `cache.KeyOrg(id)`, …) — never hardcode key strings.

`cache.DelPattern(ctx, cache.KeyAdminUsersGlob)` uses SCAN + DEL to clear wildcard keys. Use it on any write that affects user or membership data visible to admin search.

### Adding a cached endpoint

1. `h.cache.Get(ctx, key, &dest)` before the DB query — return on hit.
2. `h.cache.Set(ctx, key, value)` after a successful query.
3. `h.cache.Del(ctx, key)` (or `DelPattern`) on any write affecting that key.
4. Add a builder to `cache/keys.go` rather than hardcoding.

---

## Billing & plans

Each org has one of three plans on `organizations.plan`:

| Plan | Owners | Admins | Operationals | Activation |
|---|---|---|---|---|
| `free` | 1 | 0 (not allowed) | 5 | default |
| `pro` | 1 | 3 | 15 | Stripe Checkout (webhook flips plan) |
| `enterprise` | unlimited | unlimited | unlimited | `POST /v1/api/orgs/:orgUUID/plan/enterprise` by root/system |

- Limits enforced in `AddMember` / `UpdateMember` by `checkPlanLimit` (`handler/organization.go`). The org row is locked with `SELECT … FOR UPDATE` inside a transaction so the count check and write are atomic — concurrent requests can't race past the cap.
- Downgrades **don't** remove existing over-limit members ("grandfathering") — only new additions are blocked.
- Plan type, constants, and `LimitsFor()` live in `model/plan.go`.
- Stripe is optional: set `STRIPE_SECRET_KEY` to enable; billing endpoints return 503 when unset.
- `HandleWebhook` returns 503 immediately when `STRIPE_WEBHOOK_SECRET` is unset, in **any** environment — unsigned events are never processed. When the secret is set and `APP_ENV != production`, signature verification is skipped so dev and tests can send raw payloads. In production the `Stripe-Signature` header is always validated.
- Checkout session metadata carries the org's **public `org_uuid`**, never the internal int id (Stripe persists/surfaces metadata, so it stays on the public identifier — see "Entity identifiers"). `HandleWebhook` resolves `org_uuid` → the org row once, then uses the int id downstream (FK, cache keys, plan update). A missing/malformed/unknown `org_uuid` is acknowledged with 200 and changes nothing.
- On `checkout.session.completed` for an org already on enterprise, the handler persists a `BillingCleanupJob` **before** acknowledging the webhook. Only DB errors return 500 (so Stripe retries); once durable, the webhook returns 200 and the cancel + refund Stripe calls run in-line, stamping success timestamps or `last_error`. The org's plan is never altered. Cancel and refund are independent. Pending rows (`subscription_cancelled_at`/`payment_refunded_at` NULL) are the source of truth for an out-of-band retry worker.
- `AssignEnterprisePlan` cancels an existing Stripe subscription before promoting the org. If cancellation fails the handler returns 500 — the org is not promoted, and `stripe_subscription_id` is preserved for later reconciliation.
- `StripeGateway` in `handler/billing.go` is exported so tests inject a mock via `NewBillingHandlerWithGateway`. Methods: `CreateCheckoutSession`, `CancelSubscription`, `RefundPayment`. The real gateway (`stripeGoGateway`) is constructed once with a per-client `*stripeclient.API` — the global `stripe.Key` is never mutated.

---

## Password recovery

Self-service flow in `handler/auth_password_reset.go`:

- **Endpoints**: `POST /v1/auth/password/forgot`, `POST /v1/auth/password/reset`. Both public, both enumeration-safe.
- **Tokens**: 32 random bytes (`crypto/rand`), base64url for the URL; **SHA-256 hex digest** stored in `password_reset_tokens`. SHA-256 is appropriate because tokens are high-entropy random, not low-entropy passwords. Expire after **1 hour**, single-use.
- **`ForgotPassword`**: always `204 No Content`. When the email matches a user, a token is persisted and `mailer.Send(...)` runs in a background goroutine on a `context.Background()` with a 30s timeout — do **not** use `c.Request().Context()`, Echo cancels it as soon as the handler returns and aborts the SMTP dial. A per-email 60s throttle (`passwordResetThrottleWindow`) silently suppresses repeats so the endpoint can't be turned into an email bomb. Google-only accounts (no `password_hash`) are allowed to request a reset — this is the only way they can set an initial password.
- **`ResetPassword`**: every step runs inside one `db.Transaction(...)` so two concurrent requests with the same token can't both succeed. Updates `password_hash`, stamps `used_at` on the consumed token, and stamps `used_at` on every *other* outstanding token for that user (defence in depth). Does **not** issue a JWT — user must log in again, MFA still applies. Every failure mode returns `KeyPasswordResetTokenInvalid` so an attacker can't probe which tokens existed.
- **Mailer**: `MailerGateway` mirrors `StripeGateway` — exported interface, `goMailMailer` real impl, `disabledMailer` fallback when `cfg.SMTPHost == ""`, `NewAuthHandlerWithMailer` test seam. HTML body is rendered by `email.PasswordReset` (templ component) so user-controllable data is auto-escaped; plaintext alternative is assembled with a `strings.Builder` because HTML-escaping would garble the user's inbox. The greeting line is composed in Go before being passed to the template.
- **Localization**: subject and body come from `internal/i18n` (`KeyPasswordResetEmail*`). The reset URL embeds the locale (`/en/reset-password?token=...` or `/it/...`).
- **Known limitation**: existing JWTs remain valid after a password reset (no `password_changed_at` + `iat` check). Add it if/when stolen-token risk warrants.
- **Dev SMTP**: Mailpit at `http://localhost:8025`. Staging/production should point `SMTP_*` at a real relay.

---

## ACL — roles and access control

### Role model

**System-level** (stored on `users.role`, embedded in the JWT at login):
- `root` — set **only** via direct DB write. No API endpoint creates or promotes a root user.
- `system` — assigned by root via `POST /v1/api/system/assign-role`.

**Org-level** (stored on `org_members.role`, looked up per request):
- `owner`, `admin`, `operational`.

A user with a system role can also be an org member — the tiers are not mutually exclusive.

### Adding a new role

1. Add a constant to `internal/model/role.go`.
2. If org-level, update `CanAssignOrgRole` and `ValidOrgRole`.
3. No migration needed — roles are `VARCHAR(50)`.

### Middleware

- `JWT(db, secret)` — validates the Bearer token, rejects MFA-pending tokens, resolves the token's `user_uuid` → int id (one DB lookup) and injects the int id under `UserIDContextKey` plus `EmailContextKey` / `RoleContextKey`. 401 on a missing/invalid token or a UUID that maps to no user.
- `RequireSystemRole(roles...)` — reads `RoleContextKey` from JWT; 403 if not allowed.
- `RequireOrgRole(db, roles...)` — root/system bypass; otherwise queries `org_members` by `:orgUUID`. Injects `"org_member_role"` into the Echo context so handlers reuse it for hierarchy checks without a second DB query.

### Hierarchy enforcement

`model.CanAssignOrgRole(actorRole, targetRole)` is the single source of truth for which org role may assign another. Handlers call it via `enforceCanAssign` for `AddMember` / `UpdateMember` / `RemoveMember`. Root and system bypass.

### System configuration check

`GET /v1/system/status` returns `{"configured": bool}`. The webapp `SystemGuard` calls this on every page mount and redirects to `/setup` if `configured` is false.

---

## Adding a new API endpoint

1. Add the handler to the appropriate file in `internal/handler/` (or create a new domain file).
2. Register the route in `internal/routes/v1/` — `auth.go` for public, `api.go` for protected — or add a new `Register*` and call it from `main.go`.
3. If new schema is needed, add `migrations/NNNNNN_<description>.{up,down}.sql`.
4. For new error messages, add the key to `internal/i18n/messages.go` with EN + IT.
5. Add unit tests in the matching `_test.go`.
6. Add e2e tests in `internal/routes/v1/routes_test.go`.

---

## Development workflow

See `services/admin/backend/Makefile` for the full set of targets. The non-obvious bits:

- `make test` and `make test-coverage` require a running Docker daemon (testcontainers-go boots Postgres).
- `make test-coverage` enforces ≥ 80% on the **business** coverage (excludes `main.go`/`cmd/`/generated). Full coverage is reported but not gated.
- `make templ` regenerates `*_templ.go` from `.templ` sources. `make build-*` invokes it automatically; both source and generated files are checked in so plain `go build` works.
- `make proto` regenerates `internal/grpc/adminpb/*.pb.go` from `proto/admin/internal/v1/internal.proto` and syncs the copy in `services/operational/backend/src/adminclient/internal.proto`. `make build-*` invokes it automatically; requires `protoc` in `PATH` (installed separately — see the target's comment). Both the `.proto` source and generated files are checked in.
- `make seed-root EMAIL=... PASSWORD=...` seeds the first root user (needs `DATABASE_URL`).
- **Unified compose stack (preferred)**: `docker compose -f .compose/docker-compose.yml up --build` brings up the whole topology — admin (Postgres + Valkey + Mailpit + Go API + Next.js webapp) and operational (Postgres + Valkey + Go API) on the shared `charitychest` network. Env lives in `.compose/.env` (copy from `.compose/.env.example`). The root user is seeded automatically from `ROOT_USER` / `ROOT_PASSWORD` on first boot — no separate `seed-root` step. See `.compose/README.md` for the full env reference and the synthesis caveats.
- **Per-service compose (isolation)**: each service still has its own `.docker-dev/docker-compose.yml` for when you only want one service up. Don't mix the two modes — host ports collide.

---

## Webapp tech stack

| Concern | Choice |
|---|---|
| Framework | Next.js 15 (App Router) |
| Language | TypeScript (strict) |
| Styling | Tailwind CSS v3 |
| Auth storage | `localStorage` (`cc_token`) |
| QR code | `react-qr-code` (TOTP enrollment) |
| Testing | Vitest + React Testing Library + jsdom |

`NEXT_PUBLIC_API_URL` is the only env var. Copy `services/admin/frontend/.env.example` to `.env.local` (git-ignored). No other secrets belong in the webapp.

---

## Webapp code conventions

- **API client**: all server calls go through `services/admin/frontend/src/lib/api.ts`. No raw `fetch` in components. Every request includes `Accept-Language` derived from the URL locale via `getLocale()`.
- **Paginated calls**: use `requestPaginated<T>` in `api.ts` — returns the full `PaginatedResult<T>` (both `data` and `metadata`) without unwrapping.
- **Tests**: co-located `*.test.ts(x)`. Setup at `src/test/setup.ts`, config at `vitest.config.ts`.
- **Error display**: `<ErrorBanner message={error} />` for all API error messages. Never a bare `<p>`.
- **Auth helpers**: token read/write/clear live in `src/lib/auth.ts`. No other file touches `localStorage` directly.
- **Constants**: `NEXT_PUBLIC_API_URL` is accessed only via `src/lib/constants.ts#API_BASE_URL`.
- **Errors**: `api.ts` throws `ApiError` (carries HTTP status). Components branch on `err.status` (e.g. 401 → clear token + redirect to `/login`).
- **Protected pages**: check `isAuthenticated()` in a `useEffect`, then `router.replace('/login')` if false. No SSR session checks.
- **System configuration gate**: `SystemGuard` (in the locale layout) handles the `/setup` redirect — pages don't need to implement it.
- **No secrets in the browser**: JWT signing keys and OAuth credentials stay server-side.
- **Navigation**: always import `Link`, `useRouter`, `usePathname` from `@/i18n/navigation` — never `next/link` or `next/navigation`. This preserves the current locale on every navigation.
- **Translations**: `useTranslations()` inside components. Never hardcode UI strings in JSX.
- **Responsive design** (mobile-first):
  - Standard `<main>` padding: `px-4 py-12 sm:px-6 lg:px-8`.
  - Form inputs: `text-base sm:text-sm` (prevents iOS auto-zoom).
  - Buttons / interactive: `py-3 sm:py-2` for touch targets.
  - `<body>` has `pt-14` (in `[locale]/layout.tsx`) so content doesn't sit under the fixed `LanguageSwitcher`.
  - Never fixed pixel widths that break narrow screens.

---

## Webapp i18n

- Supported locales: `en` (default), `it`. Defined in `services/admin/frontend/src/i18n/routing.ts`.
- All UI strings in `messages/en.json` and `messages/it.json`. Both must stay in sync — every key in one exists in the other.
- Namespaces: `common`, `home`, `login`, `register`, `dashboard`, `authCallback`, `setup`, `forgotPassword`, `resetPassword`. Add new namespaces as the app grows.
- Adding a language: add the locale to `routing.ts`, create `messages/<code>.json`, add its label to `LanguageSwitcher.tsx`, extend the middleware matcher regex.

---

## Adding a new webapp page

1. Create `services/admin/frontend/src/app/[locale]/<route>/page.tsx`. Add `'use client'` if browser APIs / state are needed.
2. Import `Link` and `useRouter` from `@/i18n/navigation`.
3. Add translations to both `messages/en.json` and `messages/it.json`.
4. New API endpoint → add a typed wrapper in `src/lib/api.ts` and matching types in `src/types/api.ts`.
5. Protected pages: redirect to `/login` when `isAuthenticated()` returns false.
6. Wrap content in `<main className="flex min-h-screen items-center justify-center px-4 py-12 sm:px-6 lg:px-8">` (or the column variant for the home page) for layout consistency.
