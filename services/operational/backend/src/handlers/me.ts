// Me handler: GET /v1/api/me. The JWT middleware has already validated the
// token and injected the user UUID onto res.locals; this handler resolves it
// to a profile via admin's service-to-service API.

import type { Request, RequestHandler, Response } from "express";

import { AdminUnavailableError, AdminUserNotFoundError } from "../adminclient/errors";
import type { AdminApi, UserDTO } from "../adminclient/types";
import { dataJson, HttpError } from "../http";
import { HttpStatus } from "../http-status";
import { MessageKey, t } from "../i18n";

export interface MeDeps {
  admin: AdminApi;
}

export function createMeHandler(deps: MeDeps): RequestHandler {
  const { admin } = deps;

  return async (_req: Request, res: Response) => {
    const locale = res.locals.locale;
    const userUuid = res.locals.userUuid;
    if (!userUuid) {
      throw new HttpError(HttpStatus.Unauthorized, t(locale, MessageKey.InvalidToken));
    }

    let user: UserDTO;
    try {
      user = await admin.getUser(userUuid, locale);
    } catch (err) {
      if (err instanceof AdminUserNotFoundError) {
        throw new HttpError(HttpStatus.NotFound, t(locale, MessageKey.UserNotFound));
      }
      if (err instanceof AdminUnavailableError) {
        throw new HttpError(HttpStatus.BadGateway, t(locale, MessageKey.AdminUnavailable));
      }
      // AdminBadResponseError or anything unexpected — upstream failure.
      throw new HttpError(HttpStatus.BadGateway, t(locale, MessageKey.AdminUnavailable));
    }

    dataJson(res, HttpStatus.Ok, user);
  };
}
