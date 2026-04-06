# Charity Chest — Web App

Next.js 15 frontend for the Charity Chest API, with full English/Italian localisation via `next-intl`.

## Pages

All routes are prefixed with the active locale. Bare `/` redirects to `/en/` by the middleware.

| Route | Auth required | Description |
|---|---|---|
| `/:locale/` | — | Landing page |
| `/:locale/login` | — | Email/password login + Google OAuth button |
| `/:locale/register` | — | Account creation |
| `/:locale/dashboard` | JWT | Current user profile (calls `GET /v1/api/me`) |
| `/:locale/setup` | — | "System not configured" waiting page — shown when no root user exists |

> **System configuration gate**: `SystemGuard` (mounted in the locale layout) calls `GET /v1/system/status` on every page mount. If the server reports `configured: false`, all pages are redirected to `/setup` until a root user is created directly in the database.

## Tech stack

| Concern | Choice |
|---|---|
| Framework | Next.js 15 (App Router) |
| Language | TypeScript (strict) |
| Styling | Tailwind CSS v3 |
| i18n | `next-intl` v3 |
| Auth storage | `localStorage` (`cc_token`) |
| Testing | Vitest + React Testing Library + jsdom |

## Project layout

```
webapp/
├── messages/
│   ├── en.json                 # English strings
│   └── it.json                 # Italian strings
├── vitest.config.ts            # Vitest config (jsdom environment, @/* alias, React plugin)
├── src/
│   ├── app/
│   │   ├── layout.tsx          # Minimal root layout (delegates html/body to [locale])
│   │   ├── globals.css
│   │   └── [locale]/           # Locale-prefixed App Router segment
│   │       ├── layout.tsx      # Sets <html lang>, mounts NextIntlClientProvider
│   │       ├── page.tsx        # Landing
│   │       ├── login/page.tsx
│   │       ├── register/page.tsx
│   │       ├── dashboard/page.tsx  # Protected — redirects to /login if no token
│   │       └── setup/page.tsx      # Shown when system is unconfigured; "Check Again" button
│   ├── components/
│   │   ├── ErrorBanner.tsx         # Styled API error box — border-l-4, icon, role=alert
│   │   ├── SystemGuard.tsx         # Polls /v1/system/status; redirects to /setup if unconfigured
│   │   └── LanguageSwitcher.tsx    # EN / IT toggle, rendered on every page
│   ├── i18n/
│   │   ├── routing.ts          # defineRouting — locales + defaultLocale
│   │   ├── request.ts          # getRequestConfig — loads messages per request
│   │   └── navigation.ts       # createNavigation — locale-aware Link, useRouter, etc.
│   ├── middleware.ts            # next-intl middleware — locale detection and redirect
│   ├── lib/
│   │   ├── constants.ts        # API_BASE_URL from NEXT_PUBLIC_API_URL
│   │   ├── api.ts              # Typed fetch wrappers; sends Accept-Language on every call
│   │   └── auth.ts             # Token get / set / clear helpers (cc_token in localStorage)
│   ├── test/
│   │   └── setup.ts            # Vitest setup — imports @testing-library/jest-dom matchers
│   └── types/
│       └── api.ts              # TypeScript types mirroring the server's JSON
├── .env.example                # Template — copy to .env.local
└── .docker-dev/
    ├── Dockerfile              # node:20-alpine, hot reload via next dev
    ├── docker-compose.yml      # Mounts source; named volumes for node_modules/.next
    └── .env.example
```

## Internationalisation

Supported locales: **`en`** (default), **`it`**.

| File | Purpose |
|---|---|
| `messages/en.json` | All English UI strings |
| `messages/it.json` | All Italian UI strings |
| `src/i18n/routing.ts` | Locale list and default locale |
| `src/i18n/navigation.ts` | Re-exports locale-aware `Link`, `useRouter`, `usePathname` |
| `src/middleware.ts` | Redirects `/` → `/en/`, validates locale prefix |

**Adding a new language:**
1. Add the locale code to `routing.ts` → `locales` array.
2. Create `messages/<code>.json` mirroring the structure of `en.json`.
3. Add its label to `LanguageSwitcher.tsx` → `LABELS`.
4. Update the middleware matcher regex to include the new code.

**Adding a new string:**
1. Add the key under the appropriate namespace in both `en.json` and `it.json`.
2. Call `t('namespace.key')` in the component.

## Environment variables

| Variable | Where | Description |
|---|---|---|
| `NEXT_PUBLIC_API_URL` | `.env.local` | Base URL of the API server. `NEXT_PUBLIC_` prefix is required — Next.js inlines it into the browser bundle. |

No other secrets are needed in the webapp. JWT signing keys and OAuth credentials live exclusively on the server.

## Testing

Unit tests run entirely in-process — no server, no browser needed.

```bash
npm test             # run all tests once (CI mode)
npm run test:watch   # watch mode for development
```

| File | What it covers |
|---|---|
| `src/lib/auth.test.ts` | `getToken`, `setToken`, `clearToken`, `isAuthenticated` via jsdom `localStorage` |
| `src/lib/api.test.ts` | `ApiError` shape; `getLocale` for all URL prefixes; `Accept-Language` header sent correctly; error body parsed into `ApiError` |
| `src/components/ErrorBanner.test.tsx` | Renders null on empty message; message text; `role="alert"`; warning icon; border classes |
| `src/components/SystemGuard.test.tsx` | Redirects to `/setup` when unconfigured; redirects away from `/setup` when configured; no redirect on network error |

Test files live alongside their source (`*.test.ts` / `*.test.tsx`). The Vitest config is `vitest.config.ts` at the repo root; global test setup is `src/test/setup.ts`.

---

## Running locally

```bash
# 1. Install dependencies
npm install

# 2. Configure environment
cp .env.example .env.local
# Edit .env.local — set NEXT_PUBLIC_API_URL if the server runs on a non-default port

# 3. Start the dev server (requires the API server to be running)
npm run dev          # http://localhost:3000  →  redirects to /en/
```

## Running with Docker

```bash
cd .docker-dev
cp .env.example .env
docker compose up --build   # http://localhost:3000
```

The compose file mounts the source directory for hot reload. Named volumes (`node_modules`, `next_cache`) prevent the host filesystem from overwriting the container's installed packages.

The API server is **not** included in this compose file. Start it separately via `docker compose -f server/.docker-dev/docker-compose.yml up --build` or `go run .` from the `server/` directory.

## Google OAuth note

The "Continue with Google" button navigates the browser to `GET /v1/auth/google` on the API server. The server handles the full OAuth flow and returns `{ token, user }` as JSON at its callback URL. A full webapp integration would require the server callback to redirect back to the webapp (e.g. `/:locale/login?token=...`) so the token can be stored automatically.
