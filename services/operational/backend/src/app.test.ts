// End-to-end tests driving the full Express stack (middleware + routes) with
// fake admin/Google dependencies. Mirrors the Go internal/routes/v1/routes_test.go
// coverage: health, login, google, and me across every status code.

import { describe, expect, it } from "vitest";
import request from "supertest";

import {
  AdminInvalidCredentialsError,
  AdminUnavailableError,
  AdminUserNotFoundError,
} from "./adminclient/errors";
import type { UserDTO } from "./adminclient/types";
import { createApp } from "./app";
import type { AppDeps } from "./app";
import { HttpStatus } from "./http-status";
import {
  FakeAdmin,
  FakeGoogle,
  makeConfig,
  signOpToken,
  testUuid,
  TEST_JWT_SECRET,
} from "./test/helpers";

function buildApp(overrides: Partial<AppDeps> = {}) {
  const deps: AppDeps = {
    config: makeConfig(),
    admin: new FakeAdmin(),
    google: new FakeGoogle(),
    ...overrides,
  };
  return { app: createApp(deps), deps };
}

function user(overrides: Partial<UserDTO> = {}): UserDTO {
  return {
    uuid: testUuid(),
    email: "alice@example.com",
    name: "Alice",
    mfa_enabled: false,
    ...overrides,
  };
}

describe("GET /health", () => {
  it("returns 200 with an enveloped status", async () => {
    const { app } = buildApp();
    const res = await request(app).get("/health");
    expect(res.status).toBe(HttpStatus.Ok);
    expect(res.body).toEqual({ data: { status: "ok" } });
  });
});

describe("POST /v1/auth/login", () => {
  it("returns 200 with a token and the user on success", async () => {
    const admin = new FakeAdmin();
    admin.loginFn = async (email, password) => {
      expect(email).toBe("alice@example.com");
      expect(password).toBe("pw");
      return user();
    };
    const { app } = buildApp({ admin });

    const res = await request(app)
      .post("/v1/auth/login")
      .send({ email: "alice@example.com", password: "pw" });

    expect(res.status).toBe(HttpStatus.Ok);
    expect(typeof res.body.data.token).toBe("string");
    expect(res.body.data.token.length).toBeGreaterThan(0);
    expect(res.body.data.user.email).toBe("alice@example.com");
  });

  it("returns 401 on invalid credentials", async () => {
    const admin = new FakeAdmin();
    admin.loginFn = async () => {
      throw new AdminInvalidCredentialsError();
    };
    const { app } = buildApp({ admin });

    const res = await request(app)
      .post("/v1/auth/login")
      .send({ email: "x", password: "y" });
    expect(res.status).toBe(HttpStatus.Unauthorized);
  });

  it("returns 409 when the account has MFA enabled", async () => {
    const admin = new FakeAdmin();
    admin.loginFn = async () => user({ mfa_enabled: true });
    const { app } = buildApp({ admin });

    const res = await request(app)
      .post("/v1/auth/login")
      .send({ email: "x", password: "y" });
    expect(res.status).toBe(HttpStatus.Conflict);
  });

  it("returns 502 when admin is unavailable", async () => {
    const admin = new FakeAdmin();
    admin.loginFn = async () => {
      throw new AdminUnavailableError();
    };
    const { app } = buildApp({ admin });

    const res = await request(app)
      .post("/v1/auth/login")
      .send({ email: "x", password: "y" });
    expect(res.status).toBe(HttpStatus.BadGateway);
  });

  it("returns 400 when fields are missing", async () => {
    const { app } = buildApp();
    const res = await request(app)
      .post("/v1/auth/login")
      .send({ email: "", password: "" });
    expect(res.status).toBe(HttpStatus.BadRequest);
  });

  it("returns 400 on a malformed JSON body", async () => {
    const { app } = buildApp();
    const res = await request(app)
      .post("/v1/auth/login")
      .set("Content-Type", "application/json")
      .send("{not json");
    expect(res.status).toBe(HttpStatus.BadRequest);
  });
});

describe("POST /v1/auth/google", () => {
  it("returns 200 on a valid id token", async () => {
    const google = new FakeGoogle({
      subject: "sub-1",
      email: "g@example.com",
      name: "G",
    });
    const admin = new FakeAdmin();
    admin.googleFn = async (sub, email) => {
      expect(sub).toBe("sub-1");
      return user({ email });
    };
    const { app } = buildApp({ admin, google });

    const res = await request(app)
      .post("/v1/auth/google")
      .send({ id_token: "some-valid-token" });
    expect(res.status).toBe(HttpStatus.Ok);
    expect(res.body.data.token.length).toBeGreaterThan(0);
  });

  it("returns 401 on an invalid id token", async () => {
    const google = new FakeGoogle(null, new Error("bad token"));
    const { app } = buildApp({ google });

    const res = await request(app).post("/v1/auth/google").send({ id_token: "bad" });
    expect(res.status).toBe(HttpStatus.Unauthorized);
  });

  it("returns 401 when the verified payload is missing an email", async () => {
    const google = new FakeGoogle({ subject: "sub-1", email: "", name: "G" });
    const { app } = buildApp({ google });

    const res = await request(app).post("/v1/auth/google").send({ id_token: "tok" });
    expect(res.status).toBe(HttpStatus.Unauthorized);
  });

  it("returns 400 when id_token is missing", async () => {
    const { app } = buildApp();
    const res = await request(app).post("/v1/auth/google").send({ id_token: "" });
    expect(res.status).toBe(HttpStatus.BadRequest);
  });
});

describe("GET /v1/api/me", () => {
  it("returns 200 with the user profile", async () => {
    const id = testUuid();
    const admin = new FakeAdmin();
    admin.getFn = async (gotId) => {
      expect(gotId).toBe(id);
      return user({ uuid: id, email: "me@example.com", name: "Me" });
    };
    const { app } = buildApp({ admin });

    const res = await request(app)
      .get("/v1/api/me")
      .set("Authorization", `Bearer ${signOpToken(id, "me@example.com")}`);
    expect(res.status).toBe(HttpStatus.Ok);
    expect(res.body.data.email).toBe("me@example.com");
  });

  it("returns 401 without a token", async () => {
    const { app } = buildApp();
    const res = await request(app).get("/v1/api/me");
    expect(res.status).toBe(HttpStatus.Unauthorized);
  });

  it("returns 401 with a token signed by a different secret", async () => {
    const { app } = buildApp();
    const token = signOpToken(testUuid(), "x@x", { secret: "wrong-secret" });
    const res = await request(app).get("/v1/api/me").set("Authorization", `Bearer ${token}`);
    expect(res.status).toBe(HttpStatus.Unauthorized);
  });

  it("returns 404 when admin reports the user is gone", async () => {
    const admin = new FakeAdmin();
    admin.getFn = async () => {
      throw new AdminUserNotFoundError();
    };
    const { app } = buildApp({ admin });

    const res = await request(app)
      .get("/v1/api/me")
      .set("Authorization", `Bearer ${signOpToken(testUuid(), "x@x")}`);
    expect(res.status).toBe(HttpStatus.NotFound);
  });

  it("returns 502 when admin is unavailable", async () => {
    const admin = new FakeAdmin();
    admin.getFn = async () => {
      throw new AdminUnavailableError();
    };
    const { app } = buildApp({ admin });

    const res = await request(app)
      .get("/v1/api/me")
      .set("Authorization", `Bearer ${signOpToken(testUuid(), "x@x")}`);
    expect(res.status).toBe(HttpStatus.BadGateway);
  });

  it("forwards the X-Locale header through to admin", async () => {
    const admin = new FakeAdmin();
    let seenLocale: string | undefined;
    admin.getFn = async (_id, locale) => {
      seenLocale = locale;
      return user();
    };
    const { app } = buildApp({ admin });

    await request(app)
      .get("/v1/api/me")
      .set("Authorization", `Bearer ${signOpToken(testUuid(), "x@x")}`)
      .set("X-Locale", "it");
    expect(seenLocale).toBe("it");
  });
});

describe("JWT secret isolation", () => {
  it("the helper's secret matches the test config", () => {
    expect(makeConfig().jwtSecret).toBe(TEST_JWT_SECRET);
  });
});
