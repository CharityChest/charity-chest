// Database scaffolding. Operational has its own Postgres database with no
// entities in v1 — this Pool is reserved for future operational-owned tables.
// Like Go's gorm.Open, constructing a Pool does not eagerly connect, so the
// server still boots when the DB is unreachable and no query runs.

import { Pool } from "pg";

export function createPool(databaseUrl: string): Pool {
  const pool = new Pool({ connectionString: databaseUrl });
  // Without an "error" listener, pg emits "error" from idle clients as an
  // unhandled event (e.g. on a dropped connection), which crashes the process.
  pool.on("error", (err) => {
    console.error(`db: ${err instanceof Error ? err.message : String(err)}`);
  });
  return pool;
}
