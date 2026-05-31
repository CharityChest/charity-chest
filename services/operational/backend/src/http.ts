// Shared HTTP helpers: the success envelope, a typed error, and the terminal
// error-handling middleware. Mirrors the admin/Go conventions — successful
// JSON is wrapped as {"data": ...} while errors are bare {"message": ...}.

import type { ErrorRequestHandler, Response } from "express";

import { BODY_PARSE_ERROR_TYPE } from "./constants";
import { HttpStatus } from "./http-status";
import { MessageKey, t } from "./i18n";

/**
 * An HTTP error carrying a status code and an already-localized message.
 * Handlers and middleware throw this; the terminal errorHandler renders it as
 * {"message": ...}, matching Echo's HTTPError shape.
 */
export class HttpError extends Error {
  constructor(
    public readonly status: HttpStatus,
    message: string,
  ) {
    super(message);
    this.name = "HttpError";
  }
}

/** Wraps a successful payload in the {"data": ...} envelope. */
export function dataJson(res: Response, status: HttpStatus, payload: unknown): void {
  res.status(status).json({ data: payload });
}

/** True when an error is express.json()'s malformed-body SyntaxError. */
function isBodyParseError(err: unknown): boolean {
  return (
    typeof err === "object" &&
    err !== null &&
    "type" in err &&
    (err as { type?: unknown }).type === BODY_PARSE_ERROR_TYPE
  );
}

/**
 * Terminal Express error handler. Renders HttpError with its status/message,
 * maps malformed JSON bodies to a localized 400 (mirroring Go's c.Bind → 400),
 * and collapses anything else into a localized 500.
 */
export const errorHandler: ErrorRequestHandler = (err, _req, res, next) => {
  if (res.headersSent) {
    next(err);
    return;
  }

  const locale = (res.locals.locale as string | undefined) ?? "en";

  if (err instanceof HttpError) {
    res.status(err.status).json({ message: err.message });
    return;
  }

  if (isBodyParseError(err)) {
    res.status(HttpStatus.BadRequest).json({ message: t(locale, MessageKey.InvalidBody) });
    return;
  }

  console.error("unhandled error:", err);
  res
    .status(HttpStatus.InternalServerError)
    .json({ message: t(locale, MessageKey.ServerError) });
};
