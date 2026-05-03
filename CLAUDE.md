# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Repository layout

```
charity-chest/
├── server/                         # Go HTTP API
│   ├── main.go                     # Entry point: config → migrations → routes → listen
│   ├── go.mod / go.sum             # Module: charity-chest, Go 1.26
│   ├── Makefile                    # Build, test, and utility targets
│   ├── .env.example                # Template for local secrets (never commit .env)
│   ├── .gitignore
│   ├── cmd/
│   │   └── seed-root/main.go       # CLI to create the first root user (accepts -email/-password flags or SEED_ROOT_EMAIL/SEED_ROOT_PASSWORD env vars; blocked when APP_ENV=production and a root user already exists)
│   ├── internal/
│   │   ├── cache/cache.go          # Valkey cache client: New/Disabled/Get/Set/Del/DelPattern
│   │   ├── cache/keys.go           # Cache key constants and builder functions
│   │   ├── config/config.go        # Loads env vars via godotenv; fails fast on missing required vars
│   │   ├── handler/auth.go         # Register, Login, VerifyMFA, GoogleLogin, GoogleCallback, Me
│   │   ├── handler/profile.go      # SetupMFA, EnableMFA, DisableMFA
│   │   ├── handler/system.go       # SystemStatus (public), AssignSystemRole (root only)
│   │   ├── handler/admin.go         # SearchUsers (root only) — paginated user search with org memberships
│   │   ├── handler/organization.go # Org CRUD + member management (role hierarchy + plan limits enforced)
│   │   ├── handler/billing.go      # BillingHandler: Stripe Checkout, webhook, cancel subscription, enterprise activation
│   │   ├── handler/response.go     # dataJSON + dataWithMetaJSON helpers — wrap success responses in {"data": ...} or {"data": ..., "metadata": ...}
│   │   ├── i18n/messages.go        # Message keys + EN/IT translations; T(locale, key) lookup
│   │   ├── middleware/jwt.go       # Bearer token validation; injects UserIDContextKey + EmailContextKey + RoleContextKey
│   │   ├── middleware/locale.go    # Accept-Language parser; stores resolved locale in context; defines LocaleEN/LocaleIT
│   │   ├── middleware/acl.go       # RequireSystemRole(...) and RequireOrgRole(db, ...) middleware factories
│   │   ├── model/user.go           # GORM User model (supports password + Google OAuth + Role)
│   │   ├── model/organization.go   # Organization + OrgMember GORM models (includes Plan, Stripe fields)
│   │   ├── model/plan.go           # Plan type (free/pro/enterprise), PlanLimits, LimitsFor()
│   │   ├── model/role.go           # Role constants + CanAssignOrgRole + ValidOrgRole
│   │   └── routes/
│   │       └── v1/                 # Route registration for the v1 API (one file per group)
│   │           ├── health.go       # RegisterHealth(e) — GET /health
│   │           ├── auth.go         # RegisterAuth(v1, h) — public /v1/auth/* routes
│   │           ├── api.go          # RegisterAPI(v1, h, jwtSecret) — protected /v1/api/* routes
│   │           ├── system.go       # RegisterSystem(v1, db, cache, jwtSecret) — system status + role assignment
│   │           ├── organization.go # RegisterOrgs(v1, db, cache, jwtSecret) — org CRUD + member management
│   │           ├── profile.go      # RegisterProfile(v1, db, cfg, cache, jwtSecret) — MFA management
│   │           ├── admin.go        # RegisterAdmin(v1, db, cache, jwtSecret) — root-only admin endpoints
│   │           ├── billing.go      # RegisterBilling(e, v1, db, cache, cfg, jwtSecret) — billing + plan routes
│   │           └── routes_test.go  # E2e tests for every endpoint (full stack, in-memory SQLite)
│   ├── migrations/                 # Raw SQL migrations (golang-migrate, file source)
│   │   ├── 000001_create_users_table.up.sql
│   │   ├── 000001_create_users_table.down.sql
│   │   ├── 000002_add_role_to_users.up.sql
│   │   ├── 000002_add_role_to_users.down.sql
│   │   ├── 000003_create_organizations.up.sql
│   │   ├── 000003_create_organizations.down.sql
│   │   ├── 000004_create_org_members.up.sql
│   │   ├── 000004_create_org_members.down.sql
│   │   ├── 000005_add_mfa_to_users.up.sql
│   │   ├── 000005_add_mfa_to_users.down.sql
│   │   ├── 000006_add_plan_to_organizations.up.sql
│   │   └── 000006_add_plan_to_organizations.down.sql
│   └── .docker-dev/                # Docker Compose demo environment
│       ├── Dockerfile              # Two-stage build (golang:alpine → alpine)
│       ├── docker-compose.yml      # Postgres + Valkey + server; server waits for both health checks
│       └── .env.example            # Template for Google OAuth secrets used by compose
└── webapp/                         # Next.js 15 frontend (EN + IT)
    ├── messages/                   # i18n string files
    │   ├── en.json
    │   └── it.json
    ├── vitest.config.ts            # Vitest config (jsdom, @/* alias, React plugin)
    ├── src/
    │   ├── app/
    │   │   ├── layout.tsx          # Minimal root layout
    │   │   ├── globals.css
    │   │   └── [locale]/           # All pages live here — locale prefix in URL
    │   │       ├── setup/          # "System not configured" waiting page
    │   │       ├── profile/        # User profile + MFA enable/disable
    │   │       └── auth/callback/  # Google OAuth callback — reads ?token= and stores it
    │   ├── components/
    │   │   ├── ErrorBanner.tsx     # Styled error box (border-l-4, warning icon, role=alert)
    │   │   ├── SystemGuard.tsx     # Checks system status on mount; redirects to /setup if no root user
    │   │   └── LanguageSwitcher.tsx
    │   ├── i18n/                   # next-intl wiring (routing, request, navigation)
    │   ├── middleware.ts            # Locale detection and redirect
    │   ├── lib/                    # constants.ts, api.ts (with getLocale), auth.ts
    │   ├── test/
    │   │   └── setup.ts            # Vitest global setup — loads @testing-library/jest-dom
    │   └── types/api.ts            # TypeScript types mirroring server JSON responses
    ├── .env.example                # Template: NEXT_PUBLIC_API_URL
    └── .docker-dev/                # Docker Compose dev environment for the webapp
        ├── Dockerfile              # node:20-alpine, hot reload via next dev
        ├── docker-compose.yml      # Source mount + named volumes for node_modules/.next
        └── .env.example
```

---

## Server tech stack

| Concern | Library |
|---|---|
| HTTP framework | `github.com/labstack/echo/v4` |
| ORM | `gorm.io/gorm` + `gorm.io/driver/postgres` |
| Migrations | `github.com/golang-migrate/migrate/v4` (SQL files in `migrations/`) |
| Auth tokens | `github.com/golang-jwt/jwt/v5` (HS256, 24 h expiry; MFA-pending tokens expire in 5 min) |
| TOTP / MFA | `github.com/pquerna/otp/totp` (RFC 6238) |
| Password hashing | `golang.org/x/crypto/bcrypt` (DefaultCost) |
| Google OAuth | `golang.org/x/oauth2` + `golang.org/x/oauth2/google` |
| Config / secrets | `github.com/joho/godotenv` (dev only) + `os.Getenv` |
| Cache | `github.com/redis/go-redis/v9` (Valkey-compatible; disabled by default) |
| Test DB | `github.com/glebarez/sqlite` (pure Go, in-memory) |

---

## API versioning

All application routes are prefixed with `/v1/`. The health probe is intentionally unversioned — it is an infrastructure endpoint, not part of the API contract.

When a breaking change is needed, introduce a `/v2/` group in `main.go` alongside `/v1/`, add the corresponding `RegisterFoo` functions in `internal/routes/`, and keep both alive until clients have migrated.

## API surface

| Method | Path | Auth | Role | Description |
|---|---|---|---|---|
| GET | `/health` | — | — | Liveness probe (unversioned) |
| POST | `/v1/auth/register` | — | — | Create account (email + password) → JWT |
| POST | `/v1/auth/login` | — | — | Password login → JWT or MFA challenge |
| POST | `/v1/auth/mfa/verify` | MFA-pending JWT | — | Submit TOTP code to complete login → full JWT |
| GET | `/v1/auth/google?locale=<en\|it>` | — | — | Redirect to Google consent screen |
| GET | `/v1/auth/google/callback` | — | — | Exchange OAuth code → redirect to webapp with JWT |
| GET | `/v1/system/status` | — | — | Returns `{"configured": bool}` — true if a root user exists |
| GET | `/v1/api/me` | Bearer JWT | any | Return current user (includes `role` and `mfa_enabled` fields) |
| GET | `/v1/api/profile/mfa/setup` | Bearer JWT | any | Generate TOTP secret + QR URI for enrollment |
| POST | `/v1/api/profile/mfa/enable` | Bearer JWT | any | Verify TOTP code and activate MFA |
| DELETE | `/v1/api/profile/mfa` | Bearer JWT | any | Verify TOTP code and deactivate MFA |
| POST | `/v1/api/system/assign-role` | Bearer JWT | root | Assign/remove `system` role on a user |
| GET | `/v1/api/orgs` | Bearer JWT | system, root | List all organisations |
| POST | `/v1/api/orgs` | Bearer JWT | system, root | Create an organisation |
| GET | `/v1/api/orgs/:orgID` | Bearer JWT | any org member, system, root | Get an organisation |
| PUT | `/v1/api/orgs/:orgID` | Bearer JWT | system, root | Update an organisation |
| DELETE | `/v1/api/orgs/:orgID` | Bearer JWT | system, root | Delete an organisation |
| GET | `/v1/api/orgs/:orgID/members` | Bearer JWT | any org member, system, root | List members |
| POST | `/v1/api/orgs/:orgID/members` | Bearer JWT | hierarchy enforced | Add a member (role hierarchy applies) |
| PUT | `/v1/api/orgs/:orgID/members/:userID` | Bearer JWT | hierarchy enforced | Update a member's role |
| DELETE | `/v1/api/orgs/:orgID/members/:userID` | Bearer JWT | hierarchy enforced | Remove a member |
| GET | `/v1/api/admin/users?email=&page=&size=` | Bearer JWT | root | Search users by email with pagination |
| POST | `/v1/api/orgs/:orgID/billing/checkout` | Bearer JWT | org owner, system, root | Create Stripe Checkout Session → `{url}` |
| DELETE | `/v1/api/orgs/:orgID/billing/subscription` | Bearer JWT | org owner, system, root | Cancel Stripe subscription (plan reverts to free via webhook) |
| POST | `/v1/api/orgs/:orgID/plan/enterprise` | Bearer JWT | system, root | Manually activate enterprise plan |
| POST | `/stripe/webhook` | Stripe-Signature | — | Stripe lifecycle events (checkout completed → pro; subscription deleted → free) |

Protected routes live under `/v1/api/` and require a valid `Authorization: Bearer <token>` header. The JWT middleware (`internal/middleware/jwt.go`) validates the token and injects `middleware.UserIDContextKey` (uint), `middleware.EmailContextKey` (string), and `middleware.RoleContextKey` (*string, nil for roleless users) into the Echo context.

---

## Secret management rules

- Secrets live **only in environment variables** — never hardcoded, never committed.
- `server/.env` is git-ignored. Copy `server/.env.example` to create it locally.
- `server/.docker-dev/.env` is also git-ignored. Copy `server/.docker-dev/.env.example`.
- `config.Load()` (`internal/config/config.go`) calls `godotenv.Load()` silently (ignored in production) then validates all required vars, returning an error that names every missing variable.
- Required vars: `DATABASE_URL`, `JWT_SECRET`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `APP_ENV`.
- `APP_ENV` is **required** and must be one of `local`, `testing`, `staging`, `production`. An absent or unrecognised value causes `Load()` to fail with a clear error. Use the typed constants `config.AppEnvLocal`, `config.AppEnvTesting`, `config.AppEnvStaging`, `config.AppEnvProduction` — never compare against bare string literals.
- Optional vars with defaults: `GOOGLE_REDIRECT_URL` (default `http://localhost:8080/v1/auth/google/callback`), `FRONTEND_URL` (default `http://localhost:3000`), `PORT` (default `8080`).
- Cache vars (all optional): `CACHE_ENABLED` (default `false`), `CACHE_URL` (default `redis://localhost:6379`), `CACHE_TTL` (default `5m` — any `time.ParseDuration` string).
- Stripe vars (all optional — billing endpoints return 503 when `STRIPE_SECRET_KEY` is unset): `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET`, `STRIPE_PRO_PRICE_ID`. When `STRIPE_SECRET_KEY` is set, **both** `STRIPE_WEBHOOK_SECRET` and `STRIPE_PRO_PRICE_ID` must also be set; `Load()` treats them as a group and names every missing companion var in the error.
- `FRONTEND_URL` is used by `GoogleCallback` to redirect the browser back to the webapp after the OAuth exchange.

---

## Database migrations

- Migrations are plain SQL files in `server/migrations/`, named `NNNNNN_<description>.{up,down}.sql`.
- They run automatically when the server starts (`migrate.Up()` in `main.go`).
- `ErrNoChange` is silently ignored; any other error is fatal.
- Never modify an already-applied migration file — add a new one instead.

---

## Development workflow

```bash
# Run locally (requires .env and a running Postgres)
cd server
go run .

# Run with Docker (no local Go/Postgres needed)
docker compose -f server/.docker-dev/docker-compose.yml up --build

# Build
make build-debug    # dist/debug/server   — race detector, no optimisations
make build-release  # dist/release/server — static, stripped

# Test (no external services needed — uses SQLite in-memory)
make test                                          # all tests
go test -race -run TestFunctionName ./internal/... # single test by name

# Seed the first root user (requires DATABASE_URL in .env or environment)
make seed-root EMAIL=admin@example.com PASSWORD=secret

# Tidy dependencies
make tidy

# Clean build artifacts
make clean
```

---

## Code conventions

- **Package layout**: all non-main code lives under `internal/`. No `pkg/` directory.
- **Response envelope**: all successful JSON responses are wrapped as `{"data": <payload>}` via the `dataJSON` helper in `handler/response.go`. Error responses (`{"message": "..."}`) from Echo's HTTPError are not wrapped.
- **Paginated responses**: endpoints that page results accept `page` (default 1) and `size` (default 20, max 100) query params and return `{"data": [...], "metadata": {"page", "size", "total", "total_pages"}}` via `dataWithMetaJSON` in `handler/response.go`. The webapp uses `requestPaginated<T>` in `api.ts` which returns the full `PaginatedResult<T>` object (both `data` and `metadata`) without unwrapping.
- **Error handling**: handlers return `echo.NewHTTPError(statusCode, message)`. Errors are never swallowed silently.
- **No user enumeration**: login returns a generic 401 for both "user not found" and "wrong password".
- **i18n**: all error messages are translated via `internal/i18n`. The `Locale` middleware (global) reads `Accept-Language`, resolves it to `middleware.LocaleEN` or `middleware.LocaleIT` (default `LocaleEN`), and stores it under `middleware.LocaleContextKey` in the Echo context. Handlers call `i18n.T(locale(c), i18n.KeyXxx)`. When adding a new error message, add its key constant to `internal/i18n/messages.go` and provide both EN and IT translations.
- **Named constants over magic strings**: use `middleware.UserIDContextKey` / `middleware.EmailContextKey` / `middleware.RoleContextKey` when reading JWT context values; `middleware.LocaleContextKey` / `middleware.LocaleEN` / `middleware.LocaleIT` for locale keys; `handler.CookieOAuthState` / `handler.CookieOAuthLocale` for OAuth cookie names; `model.RoleRoot` / `model.RoleSystem` / `model.OrgRoleOwner` etc. for role strings; `config.AppEnvLocal` / `config.AppEnvTesting` / `config.AppEnvStaging` / `config.AppEnvProduction` for environment comparisons. Never repeat bare string literals for these values.
- **Sensitive fields**: `PasswordHash` and `GoogleID` are tagged `json:"-"` — they must never appear in API responses.
- **Nullable columns**: `PasswordHash`, `GoogleID`, and `Role` are `*string`; nil means that field is not set for that user.
- **Unit tests**: one `_test.go` file per source file, in `package foo_test` (black-box). Each test gets a fresh in-memory SQLite DB via `newTestDB(t)`. No external services, no global state.
- **E2e tests**: `internal/routes/v1/routes_test.go` exercises every endpoint through the full Echo stack (all middleware in the chain). `newServer(t)` wires `RegisterHealth` + `RegisterAuth` + `RegisterAPI` + `RegisterSystem` + `RegisterOrgs` against an in-memory SQLite DB — no external services required.
- **No testify**: tests use only the standard `testing` package.
- **Migrations**: always add a matching `.down.sql` for every `.up.sql`.
- **Cache**: selected handler groups (`AuthHandler`, `OrgHandler`, `AdminHandler`, `SystemHandler`, `ProfileHandler`) receive a `*cache.Cache`. For basic functional handler tests pass `cache.Disabled()`. For tests that specifically exercise cache hit/miss/invalidation/error paths, use an in-process `miniredis` instance via `newMiniRedisCache(t)` (defined in `auth_test.go`); miniredis is in-memory and satisfies the "no external services" rule. Cache errors are non-fatal — log with `log.Printf` and fall through to the database. Never skip cache invalidation on a successful write.

---

## Cache system

The cache layer lives in `internal/cache/`. It wraps Valkey (Redis-compatible) via `go-redis/v9` and is fully transparent to the API surface.

### Key scheme

| Key | Endpoint | Invalidated by |
|---|---|---|
| `system:status` | `GET /v1/system/status` | Only `configured=true` is cached; `configured=false` is never stored |
| `user:{id}` | `GET /v1/api/me` | EnableMFA, DisableMFA, AssignSystemRole, Google link |
| `orgs:list` | `GET /v1/api/orgs` | CreateOrg, UpdateOrg, DeleteOrg |
| `org:{id}` | `GET /v1/api/orgs/:orgID` | UpdateOrg, DeleteOrg |
| `org:{id}:members` | `GET /v1/api/orgs/:orgID/members` | AddMember, UpdateMember, RemoveMember, DeleteOrg |
| `admin:users:{email}:{page}:{size}` | `GET /v1/api/admin/users` | Register, AssignSystemRole, any member change, Google create/link |

Use `cache.KeyUser(id)`, `cache.KeyOrg(id)`, etc. from `internal/cache/keys.go` — never hardcode key strings in handlers.

### Pattern invalidation

`cache.DelPattern(ctx, cache.KeyAdminUsersGlob)` uses SCAN + DEL to clear all `admin:users:*` entries. Use this on any write that changes user or membership data visible to the admin search.

### Adding a new cached endpoint

1. Call `h.cache.Get(ctx, key, &dest)` before the DB query. On hit, return immediately.
2. Call `h.cache.Set(ctx, key, value)` after a successful DB query.
3. On any write that affects this key, call `h.cache.Del(ctx, key)` (or `DelPattern` for wildcard keys).
4. Use a key builder from `internal/cache/keys.go` or add one there.

---

## Billing & plans

Organisations have one of three subscription plans stored in `organizations.plan`:

| Plan | Owners | Admins | Operationals | Activation |
|---|---|---|---|---|
| `free` | 1 | 0 (not allowed) | 5 | default |
| `pro` | 1 | 3 | 15 | Stripe Checkout (webhook flips plan) |
| `enterprise` | unlimited | unlimited | unlimited | `POST /v1/api/orgs/:orgID/plan/enterprise` by root/system |

- Plan limits are enforced in `AddMember` and `UpdateMember` by the `checkPlanLimit` helper (`handler/organization.go`). The org row is locked with `SELECT … FOR UPDATE` inside a transaction so the count check and the write are atomic — concurrent requests cannot race past the cap.
- Downgrades do **not** remove existing over-limit members ("grandfathering") — only new additions are blocked.
- Plan type, constants, and `LimitsFor()` live in `model/plan.go`.
- Stripe integration is optional: set `STRIPE_SECRET_KEY` to enable. Billing endpoints return 503 when unset.
- `HandleWebhook` skips signature verification only outside production (`APP_ENV != production`), enabling local dev and automated tests to send raw payloads. In production with `STRIPE_WEBHOOK_SECRET` unset the endpoint returns 503 immediately — unsigned events are never processed.
- The `StripeGateway` interface in `handler/billing.go` is exported so tests can inject a mock via `NewBillingHandlerWithGateway`. The real gateway (`stripeGoGateway`) is constructed once with a per-client `*stripeclient.API` — the global `stripe.Key` is never mutated.

---

## ACL — roles and access control

### Role model

Two tiers of roles are in use:

**System-level roles** (stored on `users.role`, embedded in JWT at login):
- `root` — set **only** via direct database write. No API endpoint can create or promote a root user. Manages system-level users.
- `system` — assigned by root via `POST /v1/api/system/assign-role`. Manages organisations and owners.

**Org-level roles** (stored in `org_members.role`, looked up from DB on org-scoped requests):
- `owner` — can add/remove admins and operationals within their org.
- `admin` — can add/remove operationals within their org.
- `operational` — no member management; handles day-to-day operations.

A user with a system-level role (`root`/`system`) can also be an org member — the two tiers are not mutually exclusive.

### Adding a new role

1. Add a constant to `server/internal/model/role.go`.
2. If it is an org-level role, update `CanAssignOrgRole` and `ValidOrgRole` in the same file.
3. No migration needed — roles are stored as `VARCHAR(50)`.

### Middleware

- `middleware.RequireSystemRole(roles ...string)` — reads `RoleContextKey` from the JWT context. Returns 403 if the caller's role is not in the allowed set. Use on routes that require a system-level role.
- `middleware.RequireOrgRole(db, roles ...string)` — reads `RoleContextKey`; root/system bypass automatically. For all other callers, queries `org_members` by `:orgID` path parameter. Returns 403 if the caller is not a member with an allowed role. Injects `"org_member_role"` into the Echo context so handlers can reuse it for hierarchy checks without a second DB query.

### Hierarchy enforcement

`model.CanAssignOrgRole(actorRole, targetRole)` is the single source of truth for which org role may assign another. Handlers call this (via `enforceCanAssign`) for `AddMember`, `UpdateMember`, and `RemoveMember`. Root and system users bypass the check entirely.

### System configuration check

`GET /v1/system/status` returns `{"configured": bool}`. The webapp `SystemGuard` component calls this on every page mount and redirects to `/setup` if `configured` is false. The `/setup` page shows a static notice and a "Check Again" button.

---

## Adding a new API endpoint

1. Add the handler method to the appropriate file in `internal/handler/` (or create a new file for a new domain).
2. Register the route in `internal/routes/v1/`: add it to the relevant `Register*` function (`auth.go` for public routes, `api.go` for protected ones), or create a new file with a new `Register*` function and call it from `main.go`.
3. If the endpoint needs a new table or column, create `migrations/NNNNNN_<description>.{up,down}.sql`.
4. For any new error messages, add the key to `internal/i18n/messages.go` with both `"en"` and `"it"` translations.
5. Add unit tests in the corresponding `_test.go` file inside `internal/handler/`.
6. Add e2e tests in `internal/routes/v1/routes_test.go` — the `newServer` helper already wires the full stack.

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

---

## Webapp environment variables

| Variable | Description |
|---|---|
| `NEXT_PUBLIC_API_URL` | Base URL of the API server. The `NEXT_PUBLIC_` prefix is required — Next.js inlines it into the browser bundle. No other secrets belong in the webapp. |

- `webapp/.env.local` is git-ignored. Copy `webapp/.env.example` to create it locally.
- `webapp/.docker-dev/.env` is also git-ignored. Copy `webapp/.docker-dev/.env.example`.

---

## Webapp development workflow

```bash
# Run locally (requires the server to be running)
cd webapp
npm install
npm run dev          # http://localhost:3000

# Build
npm run build

# Test
npm test             # run all tests once
npm run test:watch   # watch mode

# Lint
npm run lint

# Run with Docker
docker compose -f webapp/.docker-dev/docker-compose.yml up --build
```

---

## Webapp code conventions

- **API client**: all server calls go through `webapp/src/lib/api.ts`. No raw `fetch` calls in components. Every request automatically includes `Accept-Language` derived from the URL locale via `getLocale()`.
- **Tests**: co-locate test files alongside the source (`*.test.ts` / `*.test.tsx`). Test setup lives in `src/test/setup.ts`. Config is `vitest.config.ts` at the webapp root.
- **Error display**: use `<ErrorBanner message={error} />` (`src/components/ErrorBanner.tsx`) for all API error messages. Never use a bare `<p>` for server errors.
- **Auth helpers**: token read/write/clear live in `webapp/src/lib/auth.ts`. No other file touches `localStorage` directly.
- **Constants**: `NEXT_PUBLIC_API_URL` is accessed only via `webapp/src/lib/constants.ts#API_BASE_URL`.
- **Error handling**: `api.ts` throws `ApiError` (carries HTTP status). Components catch it and branch on `err.status` (e.g. 401 → clear token + redirect to `/login`).
- **Protected pages**: check `isAuthenticated()` in a `useEffect`, then call `router.replace('/login')` if false. Do not rely on server-side session checks.
- **System configuration gate**: `SystemGuard` (in the locale layout) calls `api.systemStatus()` on every page mount. If `configured` is false and the user is not already on `/setup`, it redirects to `/setup`. The `/setup` page has a "Check Again" button. No page needs to implement this check individually.
- **No secrets in the browser**: JWT signing keys and OAuth credentials live exclusively on the server.
- **Navigation**: always import `Link`, `useRouter`, and `usePathname` from `@/i18n/navigation` — never from `next/link` or `next/navigation`. This ensures the current locale is preserved on every navigation.
- **Translations**: call `useTranslations()` (or `useTranslations('namespace')`) inside components. Never hardcode UI strings directly in JSX.
- **Responsive design**: all pages are mobile-first. Use `px-4 py-12 sm:px-6 lg:px-8` as the standard `<main>` padding. Form inputs must use `text-base sm:text-sm` (prevents iOS auto-zoom). Buttons and interactive elements use `py-3 sm:py-2` for adequate touch targets on mobile. The `<body>` has `pt-14` (set in `[locale]/layout.tsx`) to prevent content from sitting under the fixed `LanguageSwitcher`. Never add fixed pixel widths that break on narrow screens.

---

## Webapp i18n conventions

- Supported locales: `en` (default), `it`. Defined in `webapp/src/i18n/routing.ts`.
- All UI strings live in `webapp/messages/en.json` and `webapp/messages/it.json`. Both files must be kept in sync — every key present in one must exist in the other.
- Namespaces: `common`, `home`, `login`, `register`, `dashboard`, `authCallback`, `setup`. Add new namespaces as the app grows.
- To add a new language: add the locale to `routing.ts`, create `messages/<code>.json`, add its label to `LanguageSwitcher.tsx`, and extend the middleware matcher regex.

---

## Adding a new webapp page

1. Create `webapp/src/app/[locale]/<route>/page.tsx`. Add `'use client'` if the page needs browser APIs or React state.
2. Import `Link` and `useRouter` from `@/i18n/navigation`.
3. Add translations for every new string to both `messages/en.json` and `messages/it.json`.
4. If the page calls a new API endpoint, add a typed wrapper to `webapp/src/lib/api.ts` and the corresponding TypeScript types to `webapp/src/types/api.ts`.
5. Protected pages must redirect to `/login` when `isAuthenticated()` returns false.
6. Wrap page content in `<main className="flex min-h-screen items-center justify-center px-4 py-12 sm:px-6 lg:px-8">` (or the equivalent column variant for the home page) to stay consistent with the responsive layout.