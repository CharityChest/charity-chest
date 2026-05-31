import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { runMigrations } from "./migrate";

// These tests cover the no-op paths only — they must not touch a database.
// The DATABASE_URL below is deliberately unreachable; if any test tried to
// connect, it would hang/fail, proving the no-op branches never connect.
const UNREACHABLE_DB = "postgres://nobody@127.0.0.1:1/none?sslmode=disable";

let dirs: string[] = [];

beforeEach(() => {
  vi.spyOn(console, "log").mockImplementation(() => {});
});

afterEach(async () => {
  vi.restoreAllMocks();
  await Promise.all(dirs.map((d) => rm(d, { recursive: true, force: true })));
  dirs = [];
});

async function tempDir(): Promise<string> {
  const d = await mkdtemp(path.join(tmpdir(), "op-migrate-"));
  dirs.push(d);
  return d;
}

describe("runMigrations", () => {
  it("is a no-op when the migrations directory does not exist", async () => {
    await expect(
      runMigrations(UNREACHABLE_DB, path.join(tmpdir(), "does-not-exist-xyz")),
    ).resolves.toBeUndefined();
  });

  it("is a no-op when the directory has no .up.sql files", async () => {
    const dir = await tempDir();
    await writeFile(path.join(dir, "README.md"), "not a migration");
    await expect(runMigrations(UNREACHABLE_DB, dir)).resolves.toBeUndefined();
  });
});
