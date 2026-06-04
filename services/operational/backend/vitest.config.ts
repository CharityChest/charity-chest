import { defineConfig } from "vitest/config";

// Test configuration for the operational backend.
//
// Coverage gate mirrors the admin backend's "80% business coverage" rule: the
// thin real-impl wrappers that only matter at runtime (the entry point, the
// Postgres pool, the Redis cache client, the live Google validator, and the
// SQL migration runner) are excluded so the threshold reflects the business
// logic — config parsing, i18n, middleware, the admin client, and handlers.
export default defineConfig({
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
    coverage: {
      provider: "v8",
      reporter: ["text", "html", "lcov"],
      include: ["src/**/*.ts"],
      exclude: [
        "src/**/*.test.ts",
        "src/main.ts",
        "src/db.ts",
        "src/cache.ts",
        "src/migrate.ts",
        "src/google.ts",
        "src/types/**",
      ],
      thresholds: {
        lines: 80,
        functions: 80,
        statements: 80,
        branches: 80,
      },
    },
  },
});
