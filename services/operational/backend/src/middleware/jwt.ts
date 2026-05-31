// JWT middleware: validates a Bearer token signed with the operational
// service's secret and injects the token's user UUID + email onto res.locals.
//
// Unlike admin's JWT middleware, this does NOT hit a database — operational
// holds no user table. It trusts the signature and expiry, then forwards the
// UUID to admin on the next hop (GET /v1/api/me → admin GetUser).
//
// The token carries the public identifier under the `user_uuid` claim (plus
// `email`), matching the Go middleware.Claims shape.

import type { RequestHandler } from "express";
import jwt, { type JwtPayload } from "jsonwebtoken";

import { HttpError } from "../http";
import { t } from "../i18n";

const NIL_UUID = "00000000-0000-0000-0000-000000000000";

export function jwtAuth(secret: string): RequestHandler {
  return (req, res, next) => {
    const locale = res.locals.locale ?? "en";

    const authHeader = req.header("Authorization") ?? "";
    if (!authHeader.startsWith("Bearer ")) {
      throw new HttpError(401, t(locale, "missingAuthHeader"));
    }
    const raw = authHeader.slice("Bearer ".length);

    let claims: string | JwtPayload;
    try {
      // algorithms is pinned to HS256 so a token signed with a different
      // method (e.g. "none" or RS256) is rejected — mirrors the Go HMAC guard.
      claims = jwt.verify(raw, secret, { algorithms: ["HS256"] });
    } catch {
      throw new HttpError(401, t(locale, "invalidToken"));
    }

    if (typeof claims !== "object") {
      throw new HttpError(401, t(locale, "invalidClaims"));
    }
    const userUuid = claims.user_uuid;
    if (typeof userUuid !== "string" || userUuid === "" || userUuid === NIL_UUID) {
      throw new HttpError(401, t(locale, "invalidClaims"));
    }

    res.locals.userUuid = userUuid;
    res.locals.email = typeof claims.email === "string" ? claims.email : undefined;
    next();
  };
}
