import * as SecureStore from "expo-secure-store";

import { googleLogin, login, me } from "./api";
import { ApiError } from "../types/api";

// jest.setup.ts mocks expo-constants → Constants.expoConfig.extra.apiBaseUrl
// = "http://test.local:8081", and we don't set EXPO_PUBLIC_API_URL during
// tests, so every URL below should resolve against that base.
const BASE = "http://test.local:8081";

type MockResponseInit = {
  status?: number;
  body?: unknown;
  bodyText?: string;
};

function mockFetchOnce({ status = 200, body, bodyText }: MockResponseInit) {
  const text = bodyText ?? (body !== undefined ? JSON.stringify(body) : "");
  (global.fetch as jest.Mock).mockResolvedValueOnce({
    ok: status >= 200 && status < 300,
    status,
    statusText: `status ${status}`,
    text: async () => text,
  });
}

describe("api client", () => {
  beforeEach(() => {
    global.fetch = jest.fn() as unknown as typeof global.fetch;
    (SecureStore as unknown as { __reset: () => void }).__reset();
  });

  // ── login ──────────────────────────────────────────────────────────────
  describe("login", () => {
    it("POSTs JSON body and unwraps the {data} envelope", async () => {
      mockFetchOnce({
        body: { data: { token: "tk", user: { uuid: "u-1", email: "a@b.c", name: "A", mfa_enabled: false } } },
      });

      const res = await login("a@b.c", "pw");

      expect(res.token).toBe("tk");
      expect(res.user.email).toBe("a@b.c");

      const [url, init] = (global.fetch as jest.Mock).mock.calls[0] as [string, RequestInit];
      expect(url).toBe(`${BASE}/v1/auth/login`);
      expect(init.method).toBe("POST");
      expect(init.body).toBe(JSON.stringify({ email: "a@b.c", password: "pw" }));
      const headers = init.headers as Record<string, string>;
      expect(headers["Content-Type"]).toBe("application/json");
      expect(headers["X-Locale"]).toBe("en");
      expect(headers["Authorization"]).toBeUndefined();
    });

    it("throws ApiError(401) on unauthorized", async () => {
      mockFetchOnce({ status: 401, body: { message: "invalid credentials" } });
      await expect(login("x", "y")).rejects.toMatchObject({
        name: "ApiError",
        status: 401,
        message: "invalid credentials",
      });
    });

    it("throws ApiError(409) with backend message on MFA conflict", async () => {
      mockFetchOnce({ status: 409, body: { message: "MFA not supported on mobile" } });
      await expect(login("x", "y")).rejects.toMatchObject({
        status: 409,
        message: "MFA not supported on mobile",
      });
    });

    it("throws ApiError(0) on network failure", async () => {
      (global.fetch as jest.Mock).mockRejectedValueOnce(new Error("network down"));
      const err = await login("x", "y").catch((e: unknown) => e);
      expect(err).toBeInstanceOf(ApiError);
      expect((err as ApiError).status).toBe(0);
      expect((err as ApiError).message).toBe("network down");
    });

    it("falls back to statusText when error body is empty", async () => {
      mockFetchOnce({ status: 500, bodyText: "" });
      await expect(login("x", "y")).rejects.toMatchObject({
        status: 500,
        message: "status 500",
      });
    });

    it("handles non-JSON error bodies without crashing", async () => {
      mockFetchOnce({ status: 500, bodyText: "<html>nginx</html>" });
      await expect(login("x", "y")).rejects.toMatchObject({ status: 500 });
    });
  });

  // ── googleLogin ────────────────────────────────────────────────────────
  describe("googleLogin", () => {
    it("forwards the id_token to /v1/auth/google", async () => {
      mockFetchOnce({
        body: { data: { token: "tk", user: { uuid: "u-2", email: "g@x", name: "G", mfa_enabled: false } } },
      });

      const res = await googleLogin("id-token-abc");
      expect(res.token).toBe("tk");

      const [url, init] = (global.fetch as jest.Mock).mock.calls[0] as [string, RequestInit];
      expect(url).toBe(`${BASE}/v1/auth/google`);
      expect(init.body).toBe(JSON.stringify({ id_token: "id-token-abc" }));
    });

    it("surfaces 401 from Google ID-token verification failures", async () => {
      mockFetchOnce({ status: 401, body: { message: "could not verify Google sign-in" } });
      await expect(googleLogin("bad")).rejects.toMatchObject({ status: 401 });
    });
  });

  // ── me ─────────────────────────────────────────────────────────────────
  describe("me", () => {
    it("attaches Authorization: Bearer <token> from SecureStore", async () => {
      await SecureStore.setItemAsync("cc_op_token", "stored.jwt.token");
      mockFetchOnce({
        body: { data: { uuid: "u-3", email: "z@x", name: "Z", mfa_enabled: false } },
      });

      const u = await me();
      expect(u.email).toBe("z@x");

      const [, init] = (global.fetch as jest.Mock).mock.calls[0] as [string, RequestInit];
      const headers = init.headers as Record<string, string>;
      expect(headers["Authorization"]).toBe("Bearer stored.jwt.token");
    });

    it("omits Authorization when no token is stored", async () => {
      mockFetchOnce({ body: { data: { uuid: "u-4", email: "z", name: "Z", mfa_enabled: false } } });
      await me();
      const [, init] = (global.fetch as jest.Mock).mock.calls[0] as [string, RequestInit];
      const headers = init.headers as Record<string, string>;
      expect(headers["Authorization"]).toBeUndefined();
    });

    it("throws ApiError(401) so callers can branch on stale-token", async () => {
      mockFetchOnce({ status: 401, body: { message: "invalid or expired token" } });
      await expect(me()).rejects.toMatchObject({ status: 401 });
    });

    it("uses GET", async () => {
      mockFetchOnce({ body: { data: { uuid: "u", email: "e", name: "n", mfa_enabled: false } } });
      await me();
      const [, init] = (global.fetch as jest.Mock).mock.calls[0] as [string, RequestInit];
      expect(init.method).toBe("GET");
    });
  });
});
