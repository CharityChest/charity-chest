// Builds the Express application from injected dependencies. Keeping the wiring
// in a factory (separate from main.ts) lets tests drive the full middleware +
// route stack against fake admin/Google implementations — the equivalent of
// the Go routes_test.go newServer() harness.

import cors from "cors";
import express, { type Express } from "express";

import type { AdminApi } from "./adminclient/types";
import type { Config } from "./config";
import { HttpHeader } from "./constants";
import type { GoogleValidator } from "./google";
import { createAuthHandlers } from "./handlers/auth";
import { createMeHandler } from "./handlers/me";
import { errorHandler } from "./http";
import { HttpStatus } from "./http-status";
import { jwtAuth } from "./middleware/jwt";
import { locale } from "./middleware/locale";

export interface AppDeps {
  config: Config;
  admin: AdminApi;
  google: GoogleValidator;
}

/** Minimal request logger (used when REQUEST_LOG_ENABLED). */
function requestLogger(): express.RequestHandler {
  return (req, res, next) => {
    const start = Date.now();
    res.on("finish", () => {
      console.log(
        `${req.method} ${req.originalUrl} ${res.statusCode} ${Date.now() - start}ms`,
      );
    });
    next();
  };
}

export function createApp(deps: AppDeps): Express {
  const app = express();
  app.disable("x-powered-by");

  if (deps.config.requestLogEnabled) {
    app.use(requestLogger());
  }
  app.use(locale());
  app.use(
    cors({
      origin: "*",
      allowedHeaders: [
        HttpHeader.Origin,
        HttpHeader.ContentType,
        HttpHeader.Authorization,
        HttpHeader.XLocale,
      ],
    }),
  );
  app.use(express.json());

  // Unversioned liveness probe. Enveloped to match the rest of the API.
  app.get("/health", (_req, res) => {
    res.status(HttpStatus.Ok).json({ data: { status: "ok" } });
  });

  const auth = createAuthHandlers(deps);
  const me = createMeHandler(deps);

  const v1 = express.Router();
  v1.post("/auth/login", auth.login);
  v1.post("/auth/google", auth.google);

  const api = express.Router();
  api.use(jwtAuth(deps.config.jwtSecret));
  api.get("/me", me);
  v1.use("/api", api);

  app.use("/v1", v1);

  app.use(errorHandler);
  return app;
}
