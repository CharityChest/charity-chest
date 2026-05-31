// Shared test helpers: a config factory, in-memory fakes for the admin client
// and Google validator, and a token signer. Mirrors the fakes in the Go
// routes_test.go so the e2e tests exercise the full middleware + route stack.

import jwt from "jsonwebtoken";

import type { AdminApi, UserDTO } from "../adminclient/types";
import type { Config } from "../config";
import { JwtAlgorithm } from "../constants";
import type { GooglePayload, GoogleValidator } from "../google";

export const TEST_JWT_SECRET = "op-routes-test-secret";
export const TEST_AUDIENCE = "test-audience";

export function makeConfig(overrides: Partial<Config> = {}): Config {
  return {
    appEnv: "testing",
    port: "8081",
    databaseUrl: "postgres://localhost:5432/test?sslmode=disable",
    jwtSecret: TEST_JWT_SECRET,
    jwtTtlMs: 24 * 60 * 60 * 1000,
    adminBaseUrl: "http://admin.test",
    serviceApiKey: "test-service-key",
    adminTimeoutMs: 10_000,
    googleAudience: TEST_AUDIENCE,
    requestLogEnabled: false,
    cacheEnabled: false,
    cacheUrl: "redis://localhost:6379",
    cacheTtlMs: 5 * 60 * 1000,
    ...overrides,
  };
}

type LoginFn = (email: string, password: string, locale?: string) => Promise<UserDTO>;
type GoogleFn = (
  sub: string,
  email: string,
  name: string,
  locale?: string,
) => Promise<UserDTO>;
type GetFn = (userUuid: string, locale?: string) => Promise<UserDTO>;

/** Configurable in-memory AdminApi. Unset methods throw if called. */
export class FakeAdmin implements AdminApi {
  loginFn: LoginFn = () => {
    throw new Error("loginFn not set");
  };
  googleFn: GoogleFn = () => {
    throw new Error("googleFn not set");
  };
  getFn: GetFn = () => {
    throw new Error("getFn not set");
  };

  login(email: string, password: string, locale?: string): Promise<UserDTO> {
    return this.loginFn(email, password, locale);
  }
  googleAuth(sub: string, email: string, name: string, locale?: string): Promise<UserDTO> {
    return this.googleFn(sub, email, name, locale);
  }
  getUser(userUuid: string, locale?: string): Promise<UserDTO> {
    return this.getFn(userUuid, locale);
  }
}

/** Google validator fake — returns `payload` unless `err` is set. */
export class FakeGoogle implements GoogleValidator {
  constructor(
    private readonly payload: GooglePayload | null = null,
    private readonly err: Error | null = null,
  ) {}

  validate(_idToken: string, audience: string): Promise<GooglePayload> {
    if (this.err) {
      return Promise.reject(this.err);
    }
    if (audience !== TEST_AUDIENCE) {
      return Promise.reject(new Error("audience mismatch"));
    }
    return Promise.resolve(this.payload as GooglePayload);
  }
}

/** Signs an operational JWT the way the auth handler does (user_uuid + email). */
export function signOpToken(
  userUuid: string,
  email: string,
  opts: { secret?: string; expiresInSec?: number } = {},
): string {
  const secret = opts.secret ?? TEST_JWT_SECRET;
  const nowSec = Math.floor(Date.now() / 1000);
  const exp = nowSec + (opts.expiresInSec ?? 3600);
  return jwt.sign({ user_uuid: userUuid, email, iat: nowSec, exp }, secret, {
    algorithm: JwtAlgorithm.HS256,
  });
}

/** A valid-looking UUID for tests. */
export function testUuid(): string {
  return "550e8400-e29b-41d4-a716-446655440000";
}
