// Operational backend base URL.
//
// Resolved at build time: `babel-preset-expo` inlines every `process.env.EXPO_PUBLIC_*`
// reference into a literal. There is no committed default — set EXPO_PUBLIC_API_URL
// via `.env` / `.env.local` (see `.env.example`). A missing value fails fast at
// module load so a misconfigured build can't silently point at the wrong host.

export function resolveApiBaseUrl(value: string | undefined): string {
  if (!value) {
    throw new Error(
      "EXPO_PUBLIC_API_URL is not set. Copy .env.example to .env.local and set it " +
        "before building (e.g. http://localhost:8081 from the iOS Simulator, " +
        "http://10.0.2.2:8081 from the Android emulator).",
    );
  }
  return value;
}

export const API_BASE_URL: string = resolveApiBaseUrl(process.env.EXPO_PUBLIC_API_URL);
