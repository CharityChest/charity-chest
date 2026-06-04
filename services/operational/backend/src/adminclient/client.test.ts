import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { HttpStatus } from "../http-status";
import { AdminClient } from "./client";
import {
  AdminBadResponseError,
  AdminInvalidCredentialsError,
  AdminUnavailableError,
  AdminUserNotFoundError,
} from "./errors";
import type { UserDTO } from "./types";

const BASE = "http://admin.test";
const KEY = "svc-key";

const USER: UserDTO = {
  uuid: "550e8400-e29b-41d4-a716-446655440000",
  email: "a@b.c",
  name: "A",
  mfa_enabled: false,
};

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function client(): AdminClient {
  return new AdminClient(BASE + "/", KEY, 5_000);
}

describe("AdminClient.login", () => {
  it("sends the service key + content type and decodes the envelope", async () => {
    fetchMock.mockResolvedValue(jsonResponse(HttpStatus.Ok, { data: USER }));
    const out = await client().login("a@b.c", "pw", "it");

    expect(out).toEqual(USER);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("http://admin.test/v1/internal/auth/login");
    expect(init.method).toBe("POST");
    expect(init.headers["X-Service-Key"]).toBe(KEY);
    expect(init.headers["X-Locale"]).toBe("it");
    expect(init.headers["Content-Type"]).toBe("application/json");
    expect(JSON.parse(init.body)).toEqual({ email: "a@b.c", password: "pw" });
  });

  it("omits X-Locale when no locale is given", async () => {
    fetchMock.mockResolvedValue(jsonResponse(HttpStatus.Ok, { data: USER }));
    await client().login("a@b.c", "pw");
    expect(fetchMock.mock.calls[0][1].headers["X-Locale"]).toBeUndefined();
  });

  it("maps 401 to AdminInvalidCredentialsError", async () => {
    fetchMock.mockResolvedValue(jsonResponse(HttpStatus.Unauthorized, { message: "no" }));
    await expect(client().login("a", "b")).rejects.toBeInstanceOf(
      AdminInvalidCredentialsError,
    );
  });

  it("maps 5xx to AdminUnavailableError", async () => {
    fetchMock.mockResolvedValue(jsonResponse(HttpStatus.ServiceUnavailable, { message: "down" }));
    await expect(client().login("a", "b")).rejects.toBeInstanceOf(AdminUnavailableError);
  });

  it("maps other non-200 statuses to AdminBadResponseError", async () => {
    fetchMock.mockResolvedValue(jsonResponse(HttpStatus.Forbidden, {}));
    await expect(client().login("a", "b")).rejects.toBeInstanceOf(AdminBadResponseError);
  });

  it("maps transport failures to AdminUnavailableError", async () => {
    fetchMock.mockRejectedValue(new Error("ECONNREFUSED"));
    await expect(client().login("a", "b")).rejects.toBeInstanceOf(AdminUnavailableError);
  });
});

describe("AdminClient.googleAuth", () => {
  it("posts the google fields", async () => {
    fetchMock.mockResolvedValue(jsonResponse(HttpStatus.Ok, { data: USER }));
    await client().googleAuth("sub-1", "g@x", "G");
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("http://admin.test/v1/internal/auth/google");
    expect(JSON.parse(init.body)).toEqual({
      google_sub: "sub-1",
      email: "g@x",
      name: "G",
    });
  });
});

describe("AdminClient.getUser", () => {
  it("GETs the user and decodes the envelope", async () => {
    fetchMock.mockResolvedValue(jsonResponse(HttpStatus.Ok, { data: USER }));
    const out = await client().getUser(USER.uuid);
    expect(out).toEqual(USER);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`http://admin.test/v1/internal/users/${USER.uuid}`);
    expect(init.method).toBe("GET");
    expect(init.body).toBeUndefined();
  });

  it("maps 404 to AdminUserNotFoundError", async () => {
    fetchMock.mockResolvedValue(jsonResponse(HttpStatus.NotFound, { message: "gone" }));
    await expect(client().getUser(USER.uuid)).rejects.toBeInstanceOf(
      AdminUserNotFoundError,
    );
  });

  it("maps 5xx to AdminUnavailableError", async () => {
    fetchMock.mockResolvedValue(jsonResponse(HttpStatus.InternalServerError, {}));
    await expect(client().getUser(USER.uuid)).rejects.toBeInstanceOf(AdminUnavailableError);
  });

  it("maps other non-200 statuses to AdminBadResponseError", async () => {
    fetchMock.mockResolvedValue(jsonResponse(HttpStatus.Forbidden, {}));
    await expect(client().getUser(USER.uuid)).rejects.toBeInstanceOf(AdminBadResponseError);
  });

  it("treats a missing data envelope as a bad response", async () => {
    fetchMock.mockResolvedValue(jsonResponse(HttpStatus.Ok, { nope: true }));
    await expect(client().getUser(USER.uuid)).rejects.toBeInstanceOf(AdminBadResponseError);
  });
});
