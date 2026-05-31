// A thin HTTP client for admin's service-to-service internal API
// (/v1/internal/*). It holds the shared service key, applies a per-request
// timeout, forwards the caller's locale, and maps non-2xx responses to typed
// errors. Mirrors the Go adminclient package, including its status-code
// mapping (which differs between POST auth calls and GET user lookups).

import {
  AdminBadResponseError,
  AdminInvalidCredentialsError,
  AdminUnavailableError,
  AdminUserNotFoundError,
} from "./errors";
import type { AdminApi, UserDTO } from "./types";

/** Shared service-secret header. Must match admin's middleware.ServiceKeyHeader. */
const SERVICE_KEY_HEADER = "X-Service-Key";

export class AdminClient implements AdminApi {
  private readonly baseUrl: string;

  constructor(
    baseUrl: string,
    private readonly serviceKey: string,
    private readonly timeoutMs: number,
  ) {
    this.baseUrl = baseUrl.replace(/\/+$/, "");
  }

  /** POST /v1/internal/auth/login — 401 → invalid credentials. */
  login(email: string, password: string, locale?: string): Promise<UserDTO> {
    return this.postUser("/v1/internal/auth/login", { email, password }, locale);
  }

  /** POST /v1/internal/auth/google — caller has already verified the ID token. */
  googleAuth(
    googleSub: string,
    email: string,
    name: string,
    locale?: string,
  ): Promise<UserDTO> {
    return this.postUser(
      "/v1/internal/auth/google",
      { google_sub: googleSub, email, name },
      locale,
    );
  }

  /** GET /v1/internal/users/:userUUID — 404 → user not found. */
  async getUser(userUuid: string, locale?: string): Promise<UserDTO> {
    const resp = await this.fetchWithTimeout(
      "GET",
      `/v1/internal/users/${encodeURIComponent(userUuid)}`,
      undefined,
      locale,
    );
    if (resp.status === 404) {
      throw new AdminUserNotFoundError();
    }
    if (resp.status >= 500) {
      throw new AdminUnavailableError(`adminclient: admin returned ${resp.status}`);
    }
    if (resp.status !== 200) {
      throw new AdminBadResponseError(`adminclient: unexpected status ${resp.status}`);
    }
    return decodeUser(resp);
  }

  private async postUser(
    path: string,
    body: Record<string, string>,
    locale?: string,
  ): Promise<UserDTO> {
    const resp = await this.fetchWithTimeout("POST", path, body, locale);
    if (resp.status === 401) {
      throw new AdminInvalidCredentialsError();
    }
    if (resp.status >= 500) {
      throw new AdminUnavailableError(`adminclient: admin returned ${resp.status}`);
    }
    if (resp.status !== 200) {
      throw new AdminBadResponseError(`adminclient: unexpected status ${resp.status}`);
    }
    return decodeUser(resp);
  }

  private async fetchWithTimeout(
    method: "GET" | "POST",
    path: string,
    body: Record<string, string> | undefined,
    locale: string | undefined,
  ): Promise<Response> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeoutMs);

    const headers: Record<string, string> = { [SERVICE_KEY_HEADER]: this.serviceKey };
    if (locale) {
      headers["X-Locale"] = locale;
    }
    if (body !== undefined) {
      headers["Content-Type"] = "application/json";
    }

    try {
      return await fetch(this.baseUrl + path, {
        method,
        headers,
        body: body !== undefined ? JSON.stringify(body) : undefined,
        signal: controller.signal,
      });
    } catch (err) {
      // Transport failure or abort (timeout) — surfaced as 502 by handlers.
      throw new AdminUnavailableError(
        `adminclient: ${err instanceof Error ? err.message : String(err)}`,
      );
    } finally {
      clearTimeout(timer);
    }
  }
}

async function decodeUser(resp: Response): Promise<UserDTO> {
  let wrapper: { data?: UserDTO };
  try {
    wrapper = (await resp.json()) as { data?: UserDTO };
  } catch {
    throw new AdminBadResponseError("adminclient: decode body failed");
  }
  if (!wrapper || typeof wrapper.data !== "object" || wrapper.data === null) {
    throw new AdminBadResponseError("adminclient: missing data envelope");
  }
  return wrapper.data;
}
