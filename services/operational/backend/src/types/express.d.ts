// Augments Express's per-request `res.locals` with the values our middleware
// injects, so handlers read them with full type safety (no `any` casts).

import "express";

declare global {
  namespace Express {
    interface Locals {
      /** Resolved request locale ("en" | "it"), set by the locale middleware. */
      locale: string;
      /** Authenticated user's public UUID, set by the JWT middleware. */
      userUuid?: string;
      /** Authenticated user's email, set by the JWT middleware. */
      email?: string;
    }
  }
}
