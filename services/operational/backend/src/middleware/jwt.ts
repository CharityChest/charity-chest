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

import { AuthScheme, HttpHeader, JwtAlgorithm, NIL_UUID } from "../constants";
import { HttpError } from "../http";
import { HttpStatus } from "../http-status";
import { MessageKey, t } from "../i18n";

export function jwtAuth(secret: string): RequestHandler {
  return (req, res, next) => {
    const locale = res.locals.locale ?? "en";

    const authHeader = req.header(HttpHeader.Authorization) ?? "";
    if (!authHeader.startsWith(AuthScheme.Bearer)) {
      throw new HttpError(HttpStatus.Unauthorized, t(locale, MessageKey.MissingAuthHeader));
    }
    const raw = authHeader.slice(AuthScheme.Bearer.length);

    let claims: string | JwtPayload;
    try {
      // algorithms is pinned to HS256 so a token signed with a different
      // method (e.g. "none" or RS256) is rejected — mirrors the Go HMAC guard.
      claims = jwt.verify(raw, secret, { algorithms: [JwtAlgorithm.HS256] });
    } catch {
      throw new HttpError(HttpStatus.Unauthorized, t(locale, MessageKey.InvalidToken));
    }

    if (typeof claims !== "object") {
      throw new HttpError(HttpStatus.Unauthorized, t(locale, MessageKey.InvalidClaims));
    }
    const userUuid = claims.user_uuid;
    if (typeof userUuid !== "string" || userUuid === "" || userUuid === NIL_UUID) {
      throw new HttpError(HttpStatus.Unauthorized, t(locale, MessageKey.InvalidClaims));
    }

    res.locals.userUuid = userUuid;
    res.locals.email = typeof claims.email === "string" ? claims.email : undefined;
    next();
  };
}
