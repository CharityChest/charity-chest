// Loads and validates operational backend configuration from environment
// variables. Mirrors the Go internal/config package: apply defaults, then
// report every missing required variable in a single error.

export type AppEnv = "local" | "testing" | "staging" | "production";

const APP_ENVS: readonly AppEnv[] = ["local", "testing", "staging", "production"];

export interface Config {
  appEnv: AppEnv;
  port: string;
  databaseUrl: string;

  jwtSecret: string;
  /** JWT lifetime in milliseconds (24h, mirrors admin). */
  jwtTtlMs: number;

  adminGrpcUrl: string;
  serviceApiKey: string;
  /** Per-request timeout for admin gRPC calls, in milliseconds (default 10s). */
  adminTimeoutMs: number;

  /**
   * Accepted `aud` values for Google ID tokens. The mobile app uses a
   * per-platform OAuth client (iOS / Android / Web), so a forwarded token's
   * audience can be any of them — list all client IDs the app ships with,
   * comma-separated, in GOOGLE_AUDIENCE.
   */
  googleAudiences: string[];

  requestLogEnabled: boolean;

  /** Browser origins allowed by CORS. Never "*" — see ALLOWED_ORIGINS. */
  allowedOrigins: string[];

  cacheEnabled: boolean;
  cacheUrl: string;
  cacheTtlMs: number;
}

type Env = Record<string, string | undefined>;

const HOUR_MS = 60 * 60 * 1000;
const MINUTE_MS = 60 * 1000;
const SECOND_MS = 1000;

/**
 * Parses a Go-style duration string (e.g. "10s", "5m", "1h30m", "500ms") into
 * milliseconds. ADMIN_TIMEOUT and CACHE_TTL carry the same syntax the Go
 * service used, so existing compose/.env values keep working unchanged. Throws
 * on an unparseable value.
 */
export function parseGoDuration(input: string): number {
  const s = input.trim();
  if (s === "") {
    throw new Error("empty duration");
  }
  const unitMs: Record<string, number> = {
    ns: 1e-6,
    us: 1e-3,
    "µs": 1e-3,
    ms: 1,
    s: SECOND_MS,
    m: MINUTE_MS,
    h: HOUR_MS,
  };
  const re = /(\d+(?:\.\d+)?)(ns|us|µs|ms|s|m|h)/g;
  let total = 0;
  let consumed = 0;
  let match: RegExpExecArray | null;
  while ((match = re.exec(s)) !== null) {
    total += Number(match[1]) * unitMs[match[2]!]!;
    consumed += match[0].length;
  }
  if (consumed !== s.length || consumed === 0) {
    throw new Error(`invalid duration: ${input}`);
  }
  return total;
}

function getenv(env: Env, key: string, def: string): string {
  const v = env[key];
  return v === undefined || v === "" ? def : v;
}

/**
 * Parses a comma-separated env value into a trimmed, empty-stripped list.
 * Returns `def` when the variable is unset or empty after stripping — used by
 * ALLOWED_ORIGINS so a misconfigured value never collapses back to "*".
 */
function parseList(value: string | undefined, def: string[]): string[] {
  if (value === undefined) return def;
  const items = value
    .split(",")
    .map((s) => s.trim())
    .filter((s) => s !== "");
  return items.length > 0 ? items : def;
}

/**
 * Reads configuration from `env` (defaults to process.env), applies defaults,
 * and validates. Throws an Error naming every missing required variable, or a
 * specific error for an invalid APP_ENV / PORT / duration.
 */
export function loadConfig(env: Env = process.env): Config {
  const cfg: Config = {
    // APP_ENV is required; default kept only so the type is satisfied before
    // validation below replaces or rejects it.
    appEnv: "local",
    port: getenv(env, "PORT", "8081"),
    databaseUrl: env.DATABASE_URL ?? "",
    jwtSecret: env.JWT_SECRET ?? "",
    jwtTtlMs: 24 * HOUR_MS,
    adminGrpcUrl: env.ADMIN_GRPC_URL ?? "",
    serviceApiKey: env.SERVICE_API_KEY ?? "",
    adminTimeoutMs: 10 * SECOND_MS,
    googleAudiences: parseList(env.GOOGLE_AUDIENCE, []),
    // Mirrors Go's exact-string semantics: enabled only when "true";
    // request logging on unless explicitly "false".
    requestLogEnabled: env.REQUEST_LOG_ENABLED !== "false",
    allowedOrigins: parseList(env.ALLOWED_ORIGINS, ["http://localhost:3000"]),
    cacheEnabled: env.CACHE_ENABLED === "true",
    cacheUrl: getenv(env, "CACHE_URL", "redis://localhost:6379"),
    cacheTtlMs: 5 * MINUTE_MS,
  };

  if (env.CACHE_TTL) {
    try {
      cfg.cacheTtlMs = parseGoDuration(env.CACHE_TTL);
    } catch {
      throw new Error(`invalid CACHE_TTL: ${env.CACHE_TTL}`);
    }
  }
  if (env.ADMIN_TIMEOUT) {
    try {
      cfg.adminTimeoutMs = parseGoDuration(env.ADMIN_TIMEOUT);
    } catch {
      throw new Error(`invalid ADMIN_TIMEOUT: ${env.ADMIN_TIMEOUT}`);
    }
  }

  const missing: string[] = [];

  const rawAppEnv = env.APP_ENV ?? "";
  if (rawAppEnv === "") {
    missing.push("APP_ENV");
  } else if (!APP_ENVS.includes(rawAppEnv as AppEnv)) {
    throw new Error(
      `invalid APP_ENV "${rawAppEnv}": must be one of local, testing, staging, production`,
    );
  } else {
    cfg.appEnv = rawAppEnv as AppEnv;
  }

  if (cfg.databaseUrl === "") missing.push("DATABASE_URL");
  if (cfg.jwtSecret === "") missing.push("JWT_SECRET");
  if (cfg.adminGrpcUrl === "") missing.push("ADMIN_GRPC_URL");
  if (cfg.serviceApiKey === "") missing.push("SERVICE_API_KEY");
  if (cfg.googleAudiences.length === 0) missing.push("GOOGLE_AUDIENCE");

  if (missing.length > 0) {
    throw new Error(`missing required environment variables: ${missing.join(", ")}`);
  }

  // Validate PORT range — an invalid value here would otherwise surface later
  // as a less actionable listen error.
  const port = Number(cfg.port);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error(`invalid PORT "${cfg.port}"`);
  }

  return cfg;
}
