// A thin wrapper around Redis/Valkey used by the operational backend. Mirrors
// services/admin/backend/internal/cache: a Cache type with a disabled mode and
// JSON (de)serialization. Disabled by default; reserved for future cached
// endpoints (no operational endpoint caches yet).

import { createClient, type RedisClientType } from "redis";

export class Cache {
  private constructor(
    private readonly client: RedisClientType | null,
    private readonly ttlMs: number,
    private readonly isDisabled: boolean,
  ) {}

  /** Returns a Cache whose operations are all no-ops (get always misses). */
  static disabled(): Cache {
    return new Cache(null, 0, true);
  }

  /** Connects to Redis/Valkey at `url` and returns an enabled Cache. */
  static async connect(url: string, ttlMs: number): Promise<Cache> {
    const client: RedisClientType = createClient({ url });
    // Without an "error" listener, node-redis emits "error" as an unhandled
    // event (e.g. on a dropped connection), which crashes the process.
    client.on("error", (err) => {
      console.error(`cache: ${err instanceof Error ? err.message : String(err)}`);
    });
    await client.connect();
    return new Cache(client, ttlMs, false);
  }

  /** Fetches and JSON-parses `key`. Returns null on miss. */
  async get<T>(key: string): Promise<T | null> {
    if (this.isDisabled || this.client === null) {
      return null;
    }
    const raw = await this.client.get(key);
    if (raw === null) {
      return null;
    }
    return JSON.parse(raw) as T;
  }

  /** JSON-serializes and stores `value` at `key` with the configured TTL. */
  async set(key: string, value: unknown): Promise<void> {
    if (this.isDisabled || this.client === null) {
      return;
    }
    await this.client.set(key, JSON.stringify(value), { PX: this.ttlMs });
  }

  /** Removes keys. */
  async del(...keys: string[]): Promise<void> {
    if (this.isDisabled || this.client === null || keys.length === 0) {
      return;
    }
    await this.client.del(keys);
  }

  async close(): Promise<void> {
    if (this.client !== null) {
      await this.client.quit();
    }
  }
}
