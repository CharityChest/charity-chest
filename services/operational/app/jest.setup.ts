// Global Jest setup: mocks for Expo and native modules that won't run in a
// pure Node test environment.
//
// Keep this file small — per-test overrides live in the test files themselves
// via `jest.mocked(...)` so the global behaviour stays predictable.

// ── expo-secure-store ────────────────────────────────────────────────────
// Real native module won't load under Node. We replace it with an in-memory
// store so AuthProvider's hydration actually works end-to-end in tests.
jest.mock("expo-secure-store", () => {
  const store = new Map<string, string>();
  return {
    __esModule: true,
    getItemAsync: jest.fn(async (key: string) => store.get(key) ?? null),
    setItemAsync: jest.fn(async (key: string, value: string) => {
      store.set(key, value);
    }),
    deleteItemAsync: jest.fn(async (key: string) => {
      store.delete(key);
    }),
    __reset: () => store.clear(),
  };
});

// ── expo-constants ────────────────────────────────────────────────────────
// constants.ts reads `Constants.expoConfig.extra.apiBaseUrl` as a fallback
// when EXPO_PUBLIC_API_URL isn't set.
jest.mock("expo-constants", () => ({
  __esModule: true,
  default: {
    expoConfig: {
      extra: {
        apiBaseUrl: "http://test.local:8081",
      },
    },
  },
}));

// ── expo-web-browser ──────────────────────────────────────────────────────
// `maybeCompleteAuthSession` runs at module load on the real implementation.
jest.mock("expo-web-browser", () => ({
  __esModule: true,
  maybeCompleteAuthSession: jest.fn(),
}));

// ── expo-auth-session / Google provider ───────────────────────────────────
// Google.useIdTokenAuthRequest returns the [request, response, promptAsync]
// triple. Tests can override this via jest.mocked(...).
jest.mock("expo-auth-session/providers/google", () => ({
  __esModule: true,
  useIdTokenAuthRequest: jest.fn(() => [{}, null, jest.fn()]),
}));

// ── expo-router ───────────────────────────────────────────────────────────
// Replace Stack with a transparent fragment; Redirect renders a marker so
// tests can assert on the chosen path. useRouter returns a stable mock object
// reused across tests — re-imported with jest.requireMock when assertions
// need access to its jest.fn() spies.
jest.mock("expo-router", () => {
  const React = require("react");
  const router = {
    replace: jest.fn(),
    push: jest.fn(),
    back: jest.fn(),
  };
  return {
    __esModule: true,
    Stack: ({ children }: { children?: React.ReactNode }) =>
      React.createElement(React.Fragment, null, children),
    Redirect: ({ href }: { href: string }) =>
      React.createElement("Redirect", { testID: "redirect", href }),
    useRouter: () => router,
    __router: router,
  };
});

// ── React Native NativeModules used by lib/api ────────────────────────────
// api.ts reads SettingsManager.settings.AppleLocale on iOS or
// I18nManager.localeIdentifier on Android to derive the X-Locale header.
// The default RN mock leaves these undefined, which is fine (falls back to
// "en") — but we set them explicitly so tests don't rely on undefined order.
import { NativeModules, Platform } from "react-native";

Object.assign(NativeModules, {
  SettingsManager: {
    settings: {
      AppleLocale: "en_US",
      AppleLanguages: ["en"],
    },
  },
  I18nManager: {
    localeIdentifier: "en_US",
  },
});

// Default to iOS so tests have a stable platform. Tests that need Android can
// override Platform.OS via `(Platform as any).OS = "android"` then restore in
// afterEach.
(Platform as { OS: string }).OS = "ios";

// ── Silence common RN warnings during tests ───────────────────────────────
// Animation warnings from the testing-library / RN combo are noise.
const origWarn = console.warn;
console.warn = (...args: unknown[]) => {
  const first = typeof args[0] === "string" ? args[0] : "";
  if (
    first.includes("useNativeDriver") ||
    first.includes("Animated") ||
    first.includes("AsyncStorage has been extracted")
  ) {
    return;
  }
  origWarn(...args);
};
