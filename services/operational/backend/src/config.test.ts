import { describe, expect, it } from "vitest";

import { loadConfig, parseGoDuration } from "./config";

const REQUIRED: Record<string, string> = {
  APP_ENV: "local",
  DATABASE_URL: "postgres://u:p@localhost:5432/db?sslmode=disable",
  JWT_SECRET: "secret",
  ADMIN_BASE_URL: "http://localhost:8080",
  SERVICE_API_KEY: "svc-key",
  GOOGLE_AUDIENCE: "aud.apps.googleusercontent.com",
};

describe("parseGoDuration", () => {
  it("parses single units", () => {
    expect(parseGoDuration("10s")).toBe(10_000);
    expect(parseGoDuration("5m")).toBe(5 * 60_000);
    expect(parseGoDuration("1h")).toBe(60 * 60_000);
    expect(parseGoDuration("500ms")).toBe(500);
  });

  it("parses compound durations", () => {
    expect(parseGoDuration("1h30m")).toBe(90 * 60_000);
  });

  it("throws on invalid input", () => {
    expect(() => parseGoDuration("")).toThrow();
    expect(() => parseGoDuration("abc")).toThrow();
    expect(() => parseGoDuration("10")).toThrow();
    expect(() => parseGoDuration("10s5")).toThrow();
  });
});

describe("loadConfig", () => {
  it("loads a valid environment with defaults applied", () => {
    const cfg = loadConfig({ ...REQUIRED });
    expect(cfg.appEnv).toBe("local");
    expect(cfg.port).toBe("8081");
    expect(cfg.adminTimeoutMs).toBe(10_000);
    expect(cfg.cacheTtlMs).toBe(5 * 60_000);
    expect(cfg.requestLogEnabled).toBe(true);
    expect(cfg.cacheEnabled).toBe(false);
    expect(cfg.cacheUrl).toBe("redis://localhost:6379");
    expect(cfg.jwtTtlMs).toBe(24 * 60 * 60 * 1000);
  });

  it("honours overrides for optional vars", () => {
    const cfg = loadConfig({
      ...REQUIRED,
      PORT: "9000",
      ADMIN_TIMEOUT: "3s",
      CACHE_TTL: "2m",
      REQUEST_LOG_ENABLED: "false",
      CACHE_ENABLED: "true",
      CACHE_URL: "redis://valkey:6379",
    });
    expect(cfg.port).toBe("9000");
    expect(cfg.adminTimeoutMs).toBe(3_000);
    expect(cfg.cacheTtlMs).toBe(2 * 60_000);
    expect(cfg.requestLogEnabled).toBe(false);
    expect(cfg.cacheEnabled).toBe(true);
    expect(cfg.cacheUrl).toBe("redis://valkey:6379");
  });

  it("defaults allowedOrigins to localhost and never wildcards", () => {
    const cfg = loadConfig({ ...REQUIRED });
    expect(cfg.allowedOrigins).toEqual(["http://localhost:3000"]);
  });

  it("parses ALLOWED_ORIGINS as a trimmed, empty-stripped list", () => {
    const cfg = loadConfig({
      ...REQUIRED,
      ALLOWED_ORIGINS: " https://a.example , https://b.example ,",
    });
    expect(cfg.allowedOrigins).toEqual(["https://a.example", "https://b.example"]);
  });

  it("falls back to the default when ALLOWED_ORIGINS is empty", () => {
    expect(loadConfig({ ...REQUIRED, ALLOWED_ORIGINS: "" }).allowedOrigins).toEqual([
      "http://localhost:3000",
    ]);
    expect(loadConfig({ ...REQUIRED, ALLOWED_ORIGINS: " , " }).allowedOrigins).toEqual([
      "http://localhost:3000",
    ]);
  });

  it("uses exact-string boolean semantics (only 'true' enables cache)", () => {
    expect(loadConfig({ ...REQUIRED, CACHE_ENABLED: "1" }).cacheEnabled).toBe(false);
    expect(loadConfig({ ...REQUIRED, CACHE_ENABLED: "TRUE" }).cacheEnabled).toBe(false);
    expect(loadConfig({ ...REQUIRED, REQUIRED_LOG: "x" }).requestLogEnabled).toBe(true);
  });

  it("lists every missing required variable in one error", () => {
    let message = "";
    try {
      loadConfig({});
    } catch (err) {
      message = (err as Error).message;
    }
    expect(message).toContain("APP_ENV");
    expect(message).toContain("DATABASE_URL");
    expect(message).toContain("JWT_SECRET");
    expect(message).toContain("ADMIN_BASE_URL");
    expect(message).toContain("SERVICE_API_KEY");
    expect(message).toContain("GOOGLE_AUDIENCE");
  });

  it("rejects an invalid APP_ENV", () => {
    expect(() => loadConfig({ ...REQUIRED, APP_ENV: "prod" })).toThrow(/invalid APP_ENV/);
  });

  it("rejects an out-of-range PORT", () => {
    expect(() => loadConfig({ ...REQUIRED, PORT: "0" })).toThrow(/invalid PORT/);
    expect(() => loadConfig({ ...REQUIRED, PORT: "70000" })).toThrow(/invalid PORT/);
    expect(() => loadConfig({ ...REQUIRED, PORT: "abc" })).toThrow(/invalid PORT/);
  });

  it("rejects an invalid duration", () => {
    expect(() => loadConfig({ ...REQUIRED, ADMIN_TIMEOUT: "soon" })).toThrow(
      /invalid ADMIN_TIMEOUT/,
    );
    expect(() => loadConfig({ ...REQUIRED, CACHE_TTL: "lots" })).toThrow(/invalid CACHE_TTL/);
  });
});
