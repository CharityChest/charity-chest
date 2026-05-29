import Constants from "expo-constants";

// Resolved at runtime. EXPO_PUBLIC_API_URL is preferred (inlined into the
// binary at build time); fall back to the value in app.json's `extra` block.
export const API_BASE_URL: string =
  process.env.EXPO_PUBLIC_API_URL ||
  ((Constants.expoConfig?.extra as { apiBaseUrl?: string } | undefined)?.apiBaseUrl ?? "http://localhost:8081");
