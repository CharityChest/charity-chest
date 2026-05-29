/**
 * Base URL of the Go API server.
 * Reads `NEXT_PUBLIC_API_URL` (required in production); falls back to `http://localhost:8080` for local dev.
 * The `NEXT_PUBLIC_` prefix is required for Next.js to inline this value into the browser bundle.
 */
export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';
