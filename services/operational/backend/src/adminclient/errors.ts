// Typed errors returned by AdminClient, mirroring the Go adminclient sentinels.
// Handlers branch on `instanceof` to map each to the right HTTP status.

/**
 * Admin replied 401 on login/google — credentials failed for any reason. Admin
 * already returns a generic 401 to avoid user enumeration.
 */
export class AdminInvalidCredentialsError extends Error {
  constructor() {
    super("adminclient: invalid credentials");
    this.name = "AdminInvalidCredentialsError";
  }
}

/** Admin replied 404 on getUser — no user for the given UUID. */
export class AdminUserNotFoundError extends Error {
  constructor() {
    super("adminclient: user not found");
    this.name = "AdminUserNotFoundError";
  }
}

/** Transport failure, timeout, or a 5xx from admin. Surfaced as 502 upstream. */
export class AdminUnavailableError extends Error {
  constructor(message = "adminclient: admin service unavailable") {
    super(message);
    this.name = "AdminUnavailableError";
  }
}

/** Malformed or unexpected (non-200, non-mapped) response from admin. */
export class AdminBadResponseError extends Error {
  constructor(message = "adminclient: bad response from admin") {
    super(message);
    this.name = "AdminBadResponseError";
  }
}
