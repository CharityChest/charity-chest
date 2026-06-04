// Auth handlers: POST /v1/auth/login and POST /v1/auth/google.
//
// Neither stores credentials — every check is delegated to admin's
// service-to-service API. After admin confirms identity, the handler signs an
// operational-owned JWT (HS256, 24h, carrying the user's public UUID + email)
// and returns it alongside the slim user DTO.
//
// MFA-enabled accounts are rejected with 409: the mobile app does not yet
// implement the TOTP step.

import type { Request, RequestHandler, Response } from "express";
import jwt from "jsonwebtoken";

import {
  AdminInvalidCredentialsError,
  AdminUnavailableError,
  AdminUserNotFoundError,
} from "../adminclient/errors";
import type { AdminApi, UserDTO } from "../adminclient/types";
import { JwtAlgorithm } from "../constants";
import type { Config } from "../config";
import type { GoogleValidator } from "../google";
import { dataJson, HttpError } from "../http";
import { HttpStatus } from "../http-status";
import { MessageKey, t } from "../i18n";

export interface AuthDeps {
  config: Config;
  admin: AdminApi;
  google: GoogleValidator;
}

export interface AuthHandlers {
  login: RequestHandler;
  google: RequestHandler;
}

/** Maps an adminclient error to the matching localized HttpError. */
export function mapAdminError(locale: string, err: unknown): HttpError {
  if (err instanceof AdminInvalidCredentialsError) {
    return new HttpError(HttpStatus.Unauthorized, t(locale, MessageKey.InvalidCredentials));
  }
  if (err instanceof AdminUserNotFoundError) {
    return new HttpError(HttpStatus.NotFound, t(locale, MessageKey.UserNotFound));
  }
  if (err instanceof AdminUnavailableError) {
    return new HttpError(HttpStatus.BadGateway, t(locale, MessageKey.AdminUnavailable));
  }
  // AdminBadResponseError and anything unexpected — treat as upstream failure.
  return new HttpError(HttpStatus.BadGateway, t(locale, MessageKey.AdminUnavailable));
}

export function createAuthHandlers(deps: AuthDeps): AuthHandlers {
  const { config, admin, google } = deps;

  function issueToken(res: Response, locale: string, user: UserDTO): void {
    const nowSec = Math.floor(Date.now() / 1000);
    const claims = {
      user_uuid: user.uuid,
      email: user.email,
      exp: nowSec + Math.floor(config.jwtTtlMs / 1000),
      iat: nowSec,
    };
    let token: string;
    try {
      token = jwt.sign(claims, config.jwtSecret, { algorithm: JwtAlgorithm.HS256 });
    } catch {
      throw new HttpError(HttpStatus.InternalServerError, t(locale, MessageKey.GenerateToken));
    }
    dataJson(res, HttpStatus.Ok, { token, user });
  }

  const login: RequestHandler = async (req: Request, res: Response) => {
    const locale = res.locals.locale;
    const body = (req.body ?? {}) as { email?: unknown; password?: unknown };
    const email = typeof body.email === "string" ? body.email : "";
    const password = typeof body.password === "string" ? body.password : "";
    if (email === "" || password === "") {
      throw new HttpError(HttpStatus.BadRequest, t(locale, MessageKey.FieldsRequired));
    }

    let user: UserDTO;
    try {
      user = await admin.login(email, password, locale);
    } catch (err) {
      throw mapAdminError(locale, err);
    }

    if (user.mfa_enabled) {
      throw new HttpError(HttpStatus.Conflict, t(locale, MessageKey.MfaNotSupported));
    }

    issueToken(res, locale, user);
  };

  const googleHandler: RequestHandler = async (req: Request, res: Response) => {
    const locale = res.locals.locale;
    const body = (req.body ?? {}) as { id_token?: unknown };
    const idToken = typeof body.id_token === "string" ? body.id_token : "";
    if (idToken === "") {
      throw new HttpError(HttpStatus.BadRequest, t(locale, MessageKey.FieldsRequired));
    }

    let payload;
    try {
      payload = await google.validate(idToken, config.googleAudience);
    } catch {
      throw new HttpError(HttpStatus.Unauthorized, t(locale, MessageKey.GoogleVerifyFailed));
    }
    if (payload.subject === "" || payload.email === "") {
      throw new HttpError(HttpStatus.Unauthorized, t(locale, MessageKey.GoogleVerifyFailed));
    }

    let user: UserDTO;
    try {
      user = await admin.googleAuth(payload.subject, payload.email, payload.name, locale);
    } catch (err) {
      throw mapAdminError(locale, err);
    }

    if (user.mfa_enabled) {
      throw new HttpError(HttpStatus.Conflict, t(locale, MessageKey.MfaNotSupported));
    }

    issueToken(res, locale, user);
  };

  return { login, google: googleHandler };
}
