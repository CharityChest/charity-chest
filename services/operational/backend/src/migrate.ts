// SQL migration runner. Mirrors the Go main.go behaviour: run ordered
// `*.up.sql` files from the migrations directory on startup, and treat an
// absent directory or an empty one as a no-op (v1 has no operational entities).
//
// Applied versions are tracked in a `schema_migrations` table so reruns are
// idempotent — the same contract golang-migrate provided.

import { readFile, readdir } from "node:fs/promises";
import path from "node:path";

import { Client } from "pg";

const DEFAULT_DIR = path.resolve(process.cwd(), "migrations");

// Session-level advisory lock key that serializes the migration run across
// replicas: only one process at a time may execute the check+apply+insert
// sequence, so concurrent startups can't both apply the same migration.
const MIGRATION_LOCK_KEY = 4927510384172639;

export async function runMigrations(
  databaseUrl: string,
  dir: string = DEFAULT_DIR,
): Promise<void> {
  let files: string[];
  try {
    files = (await readdir(dir)).filter((f) => f.endsWith(".up.sql")).sort();
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      console.log("migrate: skipped (no migrations directory)");
      return;
    }
    throw err;
  }

  if (files.length === 0) {
    console.log("migrate: skipped (no migration files)");
    return;
  }

  const client = new Client({ connectionString: databaseUrl });
  await client.connect();
  try {
    await client.query(
      `CREATE TABLE IF NOT EXISTS schema_migrations (
         version    TEXT PRIMARY KEY,
         applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
       )`,
    );

    await client.query("SELECT pg_advisory_lock($1)", [MIGRATION_LOCK_KEY]);
    try {
      for (const file of files) {
        const version = file.replace(/\.up\.sql$/, "");
        const existing = await client.query(
          "SELECT 1 FROM schema_migrations WHERE version = $1",
          [version],
        );
        if (existing.rowCount && existing.rowCount > 0) {
          continue;
        }

        const sql = await readFile(path.join(dir, file), "utf8");
        await client.query("BEGIN");
        try {
          await client.query(sql);
          await client.query("INSERT INTO schema_migrations (version) VALUES ($1)", [
            version,
          ]);
          await client.query("COMMIT");
        } catch (err) {
          await client.query("ROLLBACK");
          throw err;
        }
        console.log(`migrate: applied ${version}`);
      }
    } finally {
      await client.query("SELECT pg_advisory_unlock($1)", [MIGRATION_LOCK_KEY]);
    }
  } finally {
    await client.end();
  }
}
