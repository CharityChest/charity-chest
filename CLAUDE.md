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
│   ├── internal/
│   │   ├── config/config.go        # Loads env vars via godotenv; fails fast on missing required vars
│   │   ├── handler/auth.go         # Register, Login, GoogleLogin, GoogleCallback, Me
│   │   ├── i18n/messages.go        # Message keys + EN/IT translations; T(locale, key) lookup
│   │   ├── middleware/jwt.go       # Bearer token validation; injects user_id + email into context
│   │   ├── middleware/locale.go    # Accept-Language parser; stores resolved locale in context
│   │   ├── model/user.go           # GORM User model (supports password + Google OAuth)
│   │   └── routes/
│   │       └── v1/                 # Route registration for the v1 API (one file per group)
│   │           ├── health.go       # RegisterHealth(e) — GET /health
│   │           ├── auth.go         # RegisterAuth(v1, h) — public /v1/auth/* routes
│   │           ├── api.go          # RegisterAPI(v1, h, jwtSecret) — protected /v1/api/* routes
│   │           └── routes_test.go  # E2e tests for every endpoint (full stack, in-memory SQLite)
│   ├── migrations/                 # Raw SQL migrations (golang-migrate, file source)
│   │   ├── 000001_create_users_table.up.sql
│   │   └── 000001_create_users_table.down.sql
│   └── .docker-dev/                # Docker Compose demo environment
│       ├── Dockerfile              # Two-stage build (golang:alpine → alpine)
│       ├── docker-compose.yml      # Postgres + server; server waits for DB health check
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
    │   │       └── auth/callback/  # Google OAuth callback — reads ?token= and stores it
    │   ├── components/
    │   │   ├── ErrorBanner.tsx     # Styled error box (border-l-4, warning icon, role=alert)
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
| Auth tokens | `github.com/golang-jwt/jwt/v5` (HS256, 24 h expiry) |
| Password hashing | `golang.org/x/crypto/bcrypt` (DefaultCost) |
| Google OAuth | `golang.org/x/oauth2` + `golang.org/x/oauth2/google` |
| Config / secrets | `github.com/joho/godotenv` (dev only) + `os.Getenv` |
| Test DB | `github.com/glebarez/sqlite` (pure Go, in-memory) |

---

## API versioning

All application routes are prefixed with `/v1/`. The health probe is intentionally unversioned — it is an infrastructure endpoint, not part of the API contract.

When a breaking change is needed, introduce a `/v2/` group in `main.go` alongside `/v1/`, add the corresponding `RegisterFoo` functions in `internal/routes/`, and keep both alive until clients have migrated.

## API surface

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/health` | — | Liveness probe (unversioned) |
| POST | `/v1/auth/register` | — | Create account (email + password) → JWT |
| POST | `/v1/auth/login` | — | Password login → JWT |
| GET | `/v1/auth/google` | — | Redirect to Google consent screen |
| GET | `/v1/auth/google/callback` | — | Exchange OAuth code → redirect to webapp `/en/auth/callback?token=<jwt>` |
| GET | `/v1/api/me` | Bearer JWT | Return current user |

Protected routes live under `/v1/api/` and require a valid `Authorization: Bearer <token>` header. The JWT middleware (`internal/middleware/jwt.go`) validates the token and injects `user_id` (uint) and `email` (string) into the Echo context.

---

## Secret management rules

- Secrets live **only in environment variables** — never hardcoded, never committed.
- `server/.env` is git-ignored. Copy `server/.env.example` to create it locally.
- `server/.docker-dev/.env` is also git-ignored. Copy `server/.docker-dev/.env.example`.
- `config.Load()` (`internal/config/config.go`) calls `godotenv.Load()` silently (ignored in production) then validates all required vars, returning an error that names every missing variable.
- Required vars: `DATABASE_URL`, `JWT_SECRET`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`.
- Optional vars with defaults: `GOOGLE_REDIRECT_URL` (default `http://localhost:8080/v1/auth/google/callback`), `FRONTEND_URL` (default `http://localhost:3000`), `PORT` (default `8080`).
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

# Tidy dependencies
make tidy

# Clean build artifacts
make clean
```

---

## Code conventions

- **Package layout**: all non-main code lives under `internal/`. No `pkg/` directory.
- **Error handling**: handlers return `echo.NewHTTPError(statusCode, message)`. Errors are never swallowed silently.
- **No user enumeration**: login returns a generic 401 for both "user not found" and "wrong password".
- **i18n**: all error messages are translated via `internal/i18n`. The `Locale` middleware (global) reads `Accept-Language`, resolves it to `"en"` or `"it"` (default `"en"`), and stores it under `"locale"` in the Echo context. Handlers call `i18n.T(locale(c), i18n.KeyXxx)`. When adding a new error message, add its key constant to `internal/i18n/messages.go` and provide both EN and IT translations.
- **Sensitive fields**: `PasswordHash` and `GoogleID` are tagged `json:"-"` — they must never appear in API responses.
- **Nullable columns**: `PasswordHash` and `GoogleID` are `*string`; nil means that auth method is not configured for that user.
- **Unit tests**: one `_test.go` file per source file, in `package foo_test` (black-box). Each test gets a fresh in-memory SQLite DB via `newTestDB(t)`. No external services, no global state.
- **E2e tests**: `internal/routes/v1/routes_test.go` exercises every endpoint through the full Echo stack (all middleware in the chain). `newServer(t)` wires `RegisterHealth` + `RegisterAuth` + `RegisterAPI` against an in-memory SQLite DB — no external services required.
- **No testify**: tests use only the standard `testing` package.
- **Migrations**: always add a matching `.down.sql` for every `.up.sql`.

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
- **No secrets in the browser**: JWT signing keys and OAuth credentials live exclusively on the server.
- **Navigation**: always import `Link`, `useRouter`, and `usePathname` from `@/i18n/navigation` — never from `next/link` or `next/navigation`. This ensures the current locale is preserved on every navigation.
- **Translations**: call `useTranslations()` (or `useTranslations('namespace')`) inside components. Never hardcode UI strings directly in JSX.

---

## Webapp i18n conventions

- Supported locales: `en` (default), `it`. Defined in `webapp/src/i18n/routing.ts`.
- All UI strings live in `webapp/messages/en.json` and `webapp/messages/it.json`. Both files must be kept in sync — every key present in one must exist in the other.
- Namespaces: `common`, `home`, `login`, `register`, `dashboard`, `authCallback`. Add new namespaces as the app grows.
- To add a new language: add the locale to `routing.ts`, create `messages/<code>.json`, add its label to `LanguageSwitcher.tsx`, and extend the middleware matcher regex.

---

## Adding a new webapp page

1. Create `webapp/src/app/[locale]/<route>/page.tsx`. Add `'use client'` if the page needs browser APIs or React state.
2. Import `Link` and `useRouter` from `@/i18n/navigation`.
3. Add translations for every new string to both `messages/en.json` and `messages/it.json`.
4. If the page calls a new API endpoint, add a typed wrapper to `webapp/src/lib/api.ts` and the corresponding TypeScript types to `webapp/src/types/api.ts`.
5. Protected pages must redirect to `/login` when `isAuthenticated()` returns false.