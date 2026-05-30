// Jest configuration for the operational mobile app.
//
// Uses the official `jest-expo` preset so React Native, Expo modules, and the
// babel transform pipeline all wire up correctly. Tests live alongside the
// source they cover (`*.test.ts(x)`).

// `constants.ts` reads EXPO_PUBLIC_API_URL and throws when it's unset. Babel
// inlines EXPO_PUBLIC_* at transform time, so the value must be present in the
// environment before Jest transforms any module. Setting it here (evaluated by
// Node before the transform pipeline runs) gives every suite a stable base URL;
// `src/lib/api.test.ts` asserts against this exact value.
process.env.EXPO_PUBLIC_API_URL ||= "http://test.local:8081";

/** @type {import('jest').Config} */
module.exports = {
  preset: "jest-expo",

  setupFiles: ["<rootDir>/jest.setup.ts"],

  // jest-expo's default transformIgnorePatterns already covers the Expo +
  // react-native ecosystem; we just append @testing-library helpers so their
  // ESM-only sources are transformed too.
  transformIgnorePatterns: [
    "node_modules/(?!((jest-)?react-native|@react-native(-community)?|expo(nent)?|@expo(nent)?/.*|@expo-google-fonts/.*|react-navigation|@react-navigation/.*|@unimodules/.*|unimodules|sentry-expo|native-base|react-native-svg|@testing-library/.*))",
  ],

  moduleNameMapper: {
    "^@/(.*)$": "<rootDir>/src/$1",
  },

  testMatch: [
    "<rootDir>/src/**/*.test.ts",
    "<rootDir>/src/**/*.test.tsx",
    "<rootDir>/app/**/*.test.ts",
    "<rootDir>/app/**/*.test.tsx",
  ],

  collectCoverageFrom: [
    "src/**/*.{ts,tsx}",
    "app/**/*.{ts,tsx}",
    "!**/*.d.ts",
    "!**/*.test.{ts,tsx}",
  ],

  coverageThreshold: {
    global: {
      branches: 70,
      functions: 80,
      lines: 80,
      statements: 80,
    },
  },

  // Silence the noisy native-module warnings that jest-expo emits when modules
  // are mocked away in setup.
  silent: false,
};
