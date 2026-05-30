# Charity Chest — Operational mobile app

React Native + TypeScript + Expo SDK 52 app for iOS and Android. Pairs with `services/operational/backend`.

v1 does exactly two things: let a user sign in (email/password or Google) and show a private screen with their profile fetched from `/v1/api/me`.

## Prerequisites

- Node 18+ and npm.
- Xcode (for iOS Simulator) and/or Android Studio (for an emulator).
- A running `services/operational/backend` and `services/admin/backend` (see those READMEs).
- Three Google Cloud OAuth 2.0 client IDs (iOS, Android, Web) provisioned under the same project. The **Web** client ID is sent to the backend as `GOOGLE_AUDIENCE`.

## Set up

```bash
cd services/operational/app
npm install
cp .env.example .env.local
$EDITOR .env.local       # fill EXPO_PUBLIC_API_URL + the three Google client IDs
```

> **Android emulator note:** `localhost` inside the emulator does **not** mean the host machine. Use `http://10.0.2.2:8081` for `EXPO_PUBLIC_API_URL` when running on the standard AVD.

## Run

```bash
npx expo run:ios
# or
npx expo run:android
```

These commands run a native dev build (required because `expo-secure-store` and `expo-auth-session` ship native code that Expo Go does not include).

## Project layout

```
app/
  _layout.tsx          root Stack + <AuthProvider>
  index.tsx            splash: redirects to /login or /(app)
  login.tsx            email/password form + "Continue with Google"
  (app)/_layout.tsx    auth-gate; redirects to /login when no token
  (app)/index.tsx      private area: welcome + user info + logout
src/
  lib/api.ts           typed fetch wrapper; throws ApiError
  lib/auth.tsx         <AuthProvider> + useAuth() (SecureStore-backed token)
  lib/google.ts        useGoogleSignIn() — wraps expo-auth-session/providers/google
  lib/secureStore.ts   typed expo-secure-store wrapper
  lib/constants.ts     resolved EXPO_PUBLIC_API_URL (throws if unset)
  types/api.ts         User, LoginResponse, ApiError
  components/          Button, TextField, ErrorBanner — minimal RN primitives
```

## Tests

The app ships with a Jest + `jest-expo` + `@testing-library/react-native` test suite. Test files live next to their source as `*.test.ts(x)`.

```bash
npm test             # one-shot run
npm run test:watch   # re-runs on save
npm run test:ci      # what CI runs: --coverage with thresholds (80% lines/functions/statements, 70% branches)
```

The suite mocks Expo's native modules (`expo-secure-store`, `expo-constants`, `expo-web-browser`, `expo-auth-session/providers/google`) and `expo-router` in `jest.setup.ts`, so tests run in pure Node — no simulator or device required.

Covered today:
- **`src/lib`** — `api` (login, googleLogin, me) with mocked fetch and SecureStore; `secureStore` round-trips; `auth` provider hydration and signIn/signOut; `constants` env precedence; `google` (`useGoogleSignIn` success/cancel/dismiss/error/missing-token/ready/prompt branches, driving the mocked `useIdTokenAuthRequest`); `ApiError`.
- **`src/components`** — `Button`, `TextField`, `ErrorBanner` render + interaction.
- **`app/`** — `login` (form submit, Google flow, error surfaces), `(app)/index` (`/me` load, logout, 401 handling), `index` splash redirect, `(app)/_layout` auth gate.

> **`constants` env note:** `babel-preset-expo` inlines `process.env.EXPO_PUBLIC_*` into literals at *transform* time, so `EXPO_PUBLIC_API_URL` is a build-time value — there's no committed default and a missing value makes `constants.ts` throw at module load. `jest.config.js` sets it before the transform pipeline runs; `constants.test.ts` covers the happy path and verifies the fail-fast throw by re-transforming the source with `@babel/core` with the env var unset.

## Known v1 limitations

- **English-only.** Device locale is forwarded to the backend (so server-side error messages are translated), but UI strings are hard-coded English.
- **No MFA support.** Accounts with MFA enabled get a localised "use the admin web app" message from the backend; the app surfaces it as-is.
- **No password recovery.** Users still recover via the admin webapp.
- **No biometric unlock.** Tokens live in `expo-secure-store`; we don't gate reads behind FaceID/TouchID yet.
- **Client-only logout.** Tapping logout clears the local token; operational doesn't keep a server-side revocation list.
