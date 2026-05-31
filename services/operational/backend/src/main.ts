// Operational backend entry point.
//
// Bootstrap order mirrors the Go main.go and services/admin/backend/main.go:
// config → migrations → DB pool → cache → admin client → Google validator →
// Express app → listen. Migrations are a no-op when the migrations/ directory
// is empty (v1).

import "dotenv/config";

import { AdminClient } from "./adminclient/client";
import { createApp } from "./app";
import { Cache } from "./cache";
import { loadConfig } from "./config";
import { createPool } from "./db";
import { RealGoogleValidator } from "./google";
import { runMigrations } from "./migrate";

async function main(): Promise<void> {
  let config;
  try {
    config = loadConfig();
  } catch (err) {
    console.error(`config: ${err instanceof Error ? err.message : String(err)}`);
    process.exit(1);
  }

  try {
    await runMigrations(config.databaseUrl);
  } catch (err) {
    console.error(`migrate: ${err instanceof Error ? err.message : String(err)}`);
    process.exit(1);
  }

  // Reserved for future operational-owned entities (no tables in v1).
  const db = createPool(config.databaseUrl);
  void db;

  // Reserved for future cached endpoints (none in v1).
  let cache: Cache;
  if (config.cacheEnabled) {
    try {
      cache = await Cache.connect(config.cacheUrl, config.cacheTtlMs);
      console.log(`cache: enabled (TTL=${config.cacheTtlMs}ms)`);
    } catch (err) {
      console.error(`cache: ${err instanceof Error ? err.message : String(err)}`);
      process.exit(1);
    }
  } else {
    cache = Cache.disabled();
    console.log("cache: disabled");
  }
  void cache;

  const admin = new AdminClient(
    config.adminBaseUrl,
    config.serviceApiKey,
    config.adminTimeoutMs,
  );
  const google = new RealGoogleValidator();

  const app = createApp({ config, admin, google });
  app.listen(Number(config.port), () => {
    console.log(
      `starting operational server on :${config.port} (admin=${config.adminBaseUrl})`,
    );
  });
}

void main();
