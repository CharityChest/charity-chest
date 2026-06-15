import { beforeEach, describe, expect, it, vi } from "vitest";

import { AdminGrpcClient } from "./grpc-client";
import {
  AdminInvalidCredentialsError,
  AdminUnavailableError,
  AdminUserNotFoundError,
} from "./errors";
import type { UserDTO } from "./types";

// ── gRPC status codes (stable values from the gRPC spec) ─────────────────────
const UNAUTHENTICATED = 16;
const NOT_FOUND = 5;
const DEADLINE_EXCEEDED = 4;
const UNAVAILABLE = 14;

// ── Hoisted mock state ────────────────────────────────────────────────────────
// vi.hoisted() runs before any import, so these variables are accessible inside
// the vi.mock() factory functions below.

const { MockMetadata, MockAdminInternal, mockLogin, mockGoogleAuth, mockGetUser, mockClose } =
  vi.hoisted(() => {
    class MockMetadata {
      readonly entries = new Map<string, string>();
      set(k: string, v: string) {
        this.entries.set(k, v);
      }
      clone() {
        const c = new MockMetadata();
        this.entries.forEach((v, k) => c.set(k, v));
        return c;
      }
    }
    const mockLogin = vi.fn();
    const mockGoogleAuth = vi.fn();
    const mockGetUser = vi.fn();
    const mockClose = vi.fn();
    const MockAdminInternal = vi.fn().mockImplementation(() => ({
      login: mockLogin,
      googleAuth: mockGoogleAuth,
      getUser: mockGetUser,
      close: mockClose,
    }));
    return { MockMetadata, MockAdminInternal, mockLogin, mockGoogleAuth, mockGetUser, mockClose };
  });

vi.mock("@grpc/proto-loader", () => ({
  loadSync: vi.fn().mockReturnValue({}),
}));

vi.mock("@grpc/grpc-js", () => ({
  loadPackageDefinition: vi.fn().mockReturnValue({
    admin: { internal: { v1: { AdminInternal: MockAdminInternal } } },
  }),
  credentials: { createInsecure: vi.fn().mockReturnValue({}) },
  Metadata: MockMetadata,
  // Inline the numeric values — these constants are not yet initialized when the
  // vi.mock() factory runs (it is hoisted before all const declarations).
  status: { UNAUTHENTICATED: 16, NOT_FOUND: 5, DEADLINE_EXCEEDED: 4, UNAVAILABLE: 14 },
}));

// ── Fixtures ──────────────────────────────────────────────────────────────────

const GRPC_URL = "localhost:9090";
const KEY = "svc-key";

// Raw proto response: role is always a string in proto3, empty when absent.
const RAW = {
  uuid: "550e8400-e29b-41d4-a716-446655440000",
  email: "a@b.c",
  name: "A",
  role: "",
  mfa_enabled: false,
};

const USER: UserDTO = { uuid: RAW.uuid, email: RAW.email, name: RAW.name, mfa_enabled: false };

function client(): AdminGrpcClient {
  return new AdminGrpcClient(GRPC_URL, KEY, 5_000);
}

// Program the mock to resolve its gRPC callback with a success response.
function ok(mock: ReturnType<typeof vi.fn>, value: unknown): void {
  mock.mockImplementation(
    (_r: unknown, _m: unknown, _o: unknown, cb: (e: null, r: unknown) => void) => cb(null, value),
  );
}

// Program the mock to reject its gRPC callback with the given status code.
function fail(mock: ReturnType<typeof vi.fn>, code: number): void {
  mock.mockImplementation(
    (_r: unknown, _m: unknown, _o: unknown, cb: (e: { code: number; message: string }, r: null) => void) =>
      cb({ code, message: "grpc error" }, null),
  );
}

beforeEach(() => {
  mockLogin.mockReset();
  mockGoogleAuth.mockReset();
  mockGetUser.mockReset();
  mockClose.mockReset();
  MockAdminInternal.mockClear();
});

// ── Constructor ────────────────────────────────────────────────────────────────

describe("AdminGrpcClient constructor", () => {
  it("connects to the given URL", () => {
    client();
    expect(MockAdminInternal).toHaveBeenCalledWith(GRPC_URL, expect.anything());
  });
});

// ── login ─────────────────────────────────────────────────────────────────────

describe("AdminGrpcClient.login", () => {
  it("returns the decoded UserDTO on success", async () => {
    ok(mockLogin, RAW);
    expect(await client().login("a@b.c", "pw")).toEqual(USER);
  });

  it("sends email and password in the request", async () => {
    ok(mockLogin, RAW);
    await client().login("a@b.c", "pw");
    expect(mockLogin.mock.calls[0][0]).toEqual({ email: "a@b.c", password: "pw" });
  });

  it("includes the service key in metadata", async () => {
    ok(mockLogin, RAW);
    await client().login("a@b.c", "pw");
    const meta = mockLogin.mock.calls[0][1] as MockMetadata;
    expect(meta.entries.get("x-service-key")).toBe(KEY);
  });

  it("sets x-locale when locale is provided", async () => {
    ok(mockLogin, RAW);
    await client().login("a@b.c", "pw", "it");
    const meta = mockLogin.mock.calls[0][1] as MockMetadata;
    expect(meta.entries.get("x-locale")).toBe("it");
  });

  it("omits x-locale when no locale is given", async () => {
    ok(mockLogin, RAW);
    await client().login("a@b.c", "pw");
    const meta = mockLogin.mock.calls[0][1] as MockMetadata;
    expect(meta.entries.has("x-locale")).toBe(false);
  });

  it("maps UNAUTHENTICATED to AdminInvalidCredentialsError", async () => {
    fail(mockLogin, UNAUTHENTICATED);
    await expect(client().login("a", "b")).rejects.toBeInstanceOf(AdminInvalidCredentialsError);
  });

  it("maps DEADLINE_EXCEEDED to AdminUnavailableError", async () => {
    fail(mockLogin, DEADLINE_EXCEEDED);
    await expect(client().login("a", "b")).rejects.toBeInstanceOf(AdminUnavailableError);
  });

  it("maps UNAVAILABLE to AdminUnavailableError", async () => {
    fail(mockLogin, UNAVAILABLE);
    await expect(client().login("a", "b")).rejects.toBeInstanceOf(AdminUnavailableError);
  });

  it("maps unknown gRPC error codes to AdminUnavailableError", async () => {
    fail(mockLogin, 2 /* UNKNOWN */);
    await expect(client().login("a", "b")).rejects.toBeInstanceOf(AdminUnavailableError);
  });
});

// ── googleAuth ────────────────────────────────────────────────────────────────

describe("AdminGrpcClient.googleAuth", () => {
  it("returns the decoded UserDTO on success", async () => {
    ok(mockGoogleAuth, RAW);
    expect(await client().googleAuth("sub-1", "g@x", "G")).toEqual(USER);
  });

  it("sends google_sub, email, and name in the request", async () => {
    ok(mockGoogleAuth, RAW);
    await client().googleAuth("sub-1", "g@x", "G");
    expect(mockGoogleAuth.mock.calls[0][0]).toEqual({ google_sub: "sub-1", email: "g@x", name: "G" });
  });

  it("maps UNAUTHENTICATED to AdminInvalidCredentialsError", async () => {
    fail(mockGoogleAuth, UNAUTHENTICATED);
    await expect(client().googleAuth("s", "e", "n")).rejects.toBeInstanceOf(
      AdminInvalidCredentialsError,
    );
  });
});

// ── getUser ───────────────────────────────────────────────────────────────────

describe("AdminGrpcClient.getUser", () => {
  it("returns the decoded UserDTO on success", async () => {
    ok(mockGetUser, RAW);
    expect(await client().getUser(RAW.uuid)).toEqual(USER);
  });

  it("sends user_uuid in the request", async () => {
    ok(mockGetUser, RAW);
    await client().getUser(RAW.uuid);
    expect(mockGetUser.mock.calls[0][0]).toEqual({ user_uuid: RAW.uuid });
  });

  it("maps NOT_FOUND to AdminUserNotFoundError", async () => {
    fail(mockGetUser, NOT_FOUND);
    await expect(client().getUser(RAW.uuid)).rejects.toBeInstanceOf(AdminUserNotFoundError);
  });

  it("does not map UNAUTHENTICATED to AdminInvalidCredentialsError (opts.notFound path)", async () => {
    fail(mockGetUser, UNAUTHENTICATED);
    await expect(client().getUser(RAW.uuid)).rejects.toBeInstanceOf(AdminUnavailableError);
  });

  it("maps DEADLINE_EXCEEDED to AdminUnavailableError", async () => {
    fail(mockGetUser, DEADLINE_EXCEEDED);
    await expect(client().getUser(RAW.uuid)).rejects.toBeInstanceOf(AdminUnavailableError);
  });
});

// ── role mapping (proto3 empty-string → undefined) ────────────────────────────

describe("role field mapping", () => {
  it("converts empty role string to undefined", async () => {
    ok(mockLogin, { ...RAW, role: "" });
    const dto = await client().login("a", "b");
    expect(dto.role).toBeUndefined();
  });

  it("preserves non-empty role strings", async () => {
    ok(mockLogin, { ...RAW, role: "system" });
    const dto = await client().login("a", "b");
    expect(dto.role).toBe("system");
  });
});

// ── close ─────────────────────────────────────────────────────────────────────

describe("AdminGrpcClient.close", () => {
  it("delegates to the underlying gRPC channel", () => {
    client().close();
    expect(mockClose).toHaveBeenCalledOnce();
  });
});
