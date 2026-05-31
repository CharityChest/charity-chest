// Shared HTTP helpers: the success envelope, a typed error, and the terminal
// error-handling middleware. Mirrors the admin/Go conventions — successful
// JSON is wrapped as {"data": ...} while errors are bare {"message": ...}.

import type { ErrorRequestHandler, Response } from "express";

import { t } from "./i18n";

/**
 * An HTTP error carrying a status code and an already-localized message.
 * Handlers and middleware throw this; the terminal errorHandler renders it as
 * {"message": ...}, matching Echo's HTTPError shape.
 */
export class HttpError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = "HttpError";
  }
}

/** Wraps a successful payload in the {"data": ...} envelope. */
export function dataJson(res: Response, status: number, payload: unknown): void {
  res.status(status).json({ data: payload });
}

/** True when an error is express.json()'s malformed-body SyntaxError. */
function isBodyParseError(err: unknown): boolean {
  return (
    typeof err === "object" &&
    err !== null &&
    "type" in err &&
    (err as { type?: unknown }).type === "entity.parse.failed"
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
    res.status(400).json({ message: t(locale, "invalidBody") });
    return;
  }

  // eslint-disable-next-line no-console
  console.error("unhandled error:", err);
  res.status(500).json({ message: "internal server error" });
};
