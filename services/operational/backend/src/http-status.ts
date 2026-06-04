// Enum-like HTTP status codes. Using a frozen const object (rather than bare
// numeric literals) keeps call sites self-documenting and lets the compiler
// flag typos. `HttpStatus` is exported both as the value map and as a type.

export const HttpStatus = {
  Ok: 200,
  BadRequest: 400,
  Unauthorized: 401,
  Forbidden: 403,
  NotFound: 404,
  Conflict: 409,
  InternalServerError: 500,
  BadGateway: 502,
  ServiceUnavailable: 503,
} as const;

export type HttpStatus = (typeof HttpStatus)[keyof typeof HttpStatus];
