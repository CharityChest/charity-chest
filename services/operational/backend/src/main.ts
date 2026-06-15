// Operational backend entry point.
//
// Bootstrap order mirrors the Go main.go and services/admin/backend/main.go:
// config → migrations → DB pool → cache → admin client → Google validator →
// Express app → listen. Migrations are a no-op when the migrations/ directory
// is empty (v1).

import "dotenv/config";

import { AdminGrpcClient } from "./adminclient/grpc-client";
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

  const admin = new AdminGrpcClient(
    config.adminGrpcUrl,
    config.serviceApiKey,
    config.adminTimeoutMs,
  );
  const google = new RealGoogleValidator();

  const app = createApp({ config, admin, google });
  const server = app.listen(Number(config.port), () => {
    console.log(
      `starting operational server on :${config.port} (admin-grpc=${config.adminGrpcUrl})`,
    );
  });

  // Graceful shutdown: stop accepting connections, then drain the cache and DB
  // pools so the container stop doesn't leak Redis/Postgres connections.
  let shuttingDown = false;
  const shutdown = (signal: string): void => {
    if (shuttingDown) {
      return;
    }
    shuttingDown = true;
    console.log(`${signal}: shutting down`);

    // Force-exit if shutdown hangs (e.g. a stuck connection close).
    const forceExit = setTimeout(() => {
      console.error("shutdown: timed out, forcing exit");
      process.exit(1);
    }, 10_000);
    forceExit.unref();

    server.close(async () => {
      try {
        admin.close();
        await cache.close();
        await db.end();
      } catch (err) {
        console.error(
          `shutdown: ${err instanceof Error ? err.message : String(err)}`,
        );
      }
      clearTimeout(forceExit);
      process.exit(0);
    });
  };

  process.on("SIGINT", () => shutdown("SIGINT"));
  process.on("SIGTERM", () => shutdown("SIGTERM"));
}

void main();
