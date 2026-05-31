// Locale middleware: reads the X-Locale request header, resolves it to a
// supported locale ("en" or "it"), and stores the result on res.locals.
// Mirrors the Go middleware.Locale() — only X-Locale is consulted (no
// Accept-Language), and the default is English.

import type { RequestHandler } from "express";

import { parseLocale } from "../i18n";

export function locale(): RequestHandler {
  return (req, res, next) => {
    res.locals.locale = parseLocale(req.header("X-Locale") ?? "");
    next();
  };
}
