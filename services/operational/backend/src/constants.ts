// Enum-like constants for the string literals shared across the backend —
// header names, HTTP methods, content types, the auth scheme, the JWT signing
// algorithm, and the nil UUID. Centralising them avoids magic strings and
// keeps spelling consistent between producers and consumers.

/** HTTP header names. */
export const HttpHeader = {
  Authorization: "Authorization",
  ContentType: "Content-Type",
  Origin: "Origin",
  XLocale: "X-Locale",
  /** Shared service-to-service secret header (matches admin's expectation). */
  XServiceKey: "X-Service-Key",
} as const;

/** HTTP methods used by the admin client. */
export const HttpMethod = {
  Get: "GET",
  Post: "POST",
} as const;

export type HttpMethod = (typeof HttpMethod)[keyof typeof HttpMethod];

/** Content-Type values. */
export const ContentType = {
  Json: "application/json",
} as const;

/** Authorization scheme prefix (note the trailing space). */
export const AuthScheme = {
  Bearer: "Bearer ",
} as const;

/** JWT signing algorithms in use (HS256 only). */
export const JwtAlgorithm = {
  HS256: "HS256",
} as const;

/** body-parser's error `type` for a malformed JSON request body. */
export const BODY_PARSE_ERROR_TYPE = "entity.parse.failed";

/** The all-zero UUID — an invalid subject for an authenticated token. */
export const NIL_UUID = "00000000-0000-0000-0000-000000000000";
