// Database scaffolding. Operational has its own Postgres database with no
// entities in v1 — this Pool is reserved for future operational-owned tables.
// Like Go's gorm.Open, constructing a Pool does not eagerly connect, so the
// server still boots when the DB is unreachable and no query runs.

import { Pool } from "pg";

export function createPool(databaseUrl: string): Pool {
  return new Pool({ connectionString: databaseUrl });
}
