// NEXT_PUBLIC_ prefix is required for this value to be available in the browser bundle.
// Set NEXT_PUBLIC_API_URL in .env.local (dev) or as a real environment variable (prod).
export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080';
