// Me handler: GET /v1/api/me. The JWT middleware has already validated the
// token and injected the user UUID onto res.locals; this handler resolves it
// to a profile via admin's service-to-service API.

import type { Request, RequestHandler, Response } from "express";

import { AdminUnavailableError, AdminUserNotFoundError } from "../adminclient/errors";
import type { AdminApi, UserDTO } from "../adminclient/types";
import { dataJson, HttpError } from "../http";
import { t } from "../i18n";

export interface MeDeps {
  admin: AdminApi;
}

export function createMeHandler(deps: MeDeps): RequestHandler {
  const { admin } = deps;

  return async (_req: Request, res: Response) => {
    const locale = res.locals.locale;
    const userUuid = res.locals.userUuid;
    if (!userUuid) {
      throw new HttpError(401, t(locale, "invalidToken"));
    }

    let user: UserDTO;
    try {
      user = await admin.getUser(userUuid, locale);
    } catch (err) {
      if (err instanceof AdminUserNotFoundError) {
        throw new HttpError(404, t(locale, "userNotFound"));
      }
      if (err instanceof AdminUnavailableError) {
        throw new HttpError(502, t(locale, "adminUnavailable"));
      }
      // AdminBadResponseError or anything unexpected — upstream failure.
      throw new HttpError(502, t(locale, "adminUnavailable"));
    }

    dataJson(res, 200, user);
  };
}
