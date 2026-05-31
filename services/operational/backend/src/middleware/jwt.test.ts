import express from "express";
import jwt from "jsonwebtoken";
import request from "supertest";
import { describe, expect, it } from "vitest";

import { JwtAlgorithm } from "../constants";
import { errorHandler } from "../http";
import { HttpStatus } from "../http-status";
import { locale } from "./locale";
import { jwtAuth } from "./jwt";

const SECRET = "jwt-mw-test-secret";
const UUID = "550e8400-e29b-41d4-a716-446655440000";

function protectedApp() {
  const app = express();
  app.use(locale());
  app.use(jwtAuth(SECRET));
  app.get("/probe", (_req, res) => {
    res.json({ userUuid: res.locals.userUuid, email: res.locals.email });
  });
  app.use(errorHandler);
  return app;
}

function sign(payload: object, secret = SECRET, options?: jwt.SignOptions): string {
  return jwt.sign(payload, secret, { algorithm: JwtAlgorithm.HS256, ...options });
}

describe("jwtAuth middleware", () => {
  it("accepts a valid token and exposes the claims", async () => {
    const token = sign({ user_uuid: UUID, email: "a@b.c" });
    const res = await request(protectedApp())
      .get("/probe")
      .set("Authorization", `Bearer ${token}`);
    expect(res.status).toBe(HttpStatus.Ok);
    expect(res.body.userUuid).toBe(UUID);
    expect(res.body.email).toBe("a@b.c");
  });

  it("rejects a missing Authorization header", async () => {
    const res = await request(protectedApp()).get("/probe");
    expect(res.status).toBe(HttpStatus.Unauthorized);
  });

  it("rejects a non-Bearer scheme", async () => {
    const res = await request(protectedApp()).get("/probe").set("Authorization", "Basic abc");
    expect(res.status).toBe(HttpStatus.Unauthorized);
  });

  it("rejects a bad signature", async () => {
    const token = sign({ user_uuid: UUID }, "other-secret");
    const res = await request(protectedApp())
      .get("/probe")
      .set("Authorization", `Bearer ${token}`);
    expect(res.status).toBe(HttpStatus.Unauthorized);
  });

  it("rejects an expired token", async () => {
    const token = sign({ user_uuid: UUID }, SECRET, { expiresIn: -10 });
    const res = await request(protectedApp())
      .get("/probe")
      .set("Authorization", `Bearer ${token}`);
    expect(res.status).toBe(HttpStatus.Unauthorized);
  });

  it("rejects a token signed with a non-HMAC alg claim (none)", async () => {
    // "alg: none" tokens have an empty signature; jsonwebtoken rejects them
    // because we pin algorithms to HS256.
    const unsigned = jwt.sign({ user_uuid: UUID }, "", { algorithm: "none" });
    const res = await request(protectedApp())
      .get("/probe")
      .set("Authorization", `Bearer ${unsigned}`);
    expect(res.status).toBe(HttpStatus.Unauthorized);
  });

  it("rejects a token with no user_uuid claim", async () => {
    const token = sign({ email: "a@b.c" });
    const res = await request(protectedApp())
      .get("/probe")
      .set("Authorization", `Bearer ${token}`);
    expect(res.status).toBe(HttpStatus.Unauthorized);
  });

  it("rejects the nil UUID", async () => {
    const token = sign({ user_uuid: "00000000-0000-0000-0000-000000000000" });
    const res = await request(protectedApp())
      .get("/probe")
      .set("Authorization", `Bearer ${token}`);
    expect(res.status).toBe(HttpStatus.Unauthorized);
  });
});
