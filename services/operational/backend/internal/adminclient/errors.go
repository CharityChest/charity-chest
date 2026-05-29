package adminclient

import "errors"

// ErrInvalidCredentials maps to admin's 401 on Login. Returned by Login when
// the email/password pair fails for any reason (no user, no password set,
// wrong password) — admin already returns a generic 401 to avoid enumeration.
var ErrInvalidCredentials = errors.New("adminclient: invalid credentials")

// ErrUserNotFound maps to admin's 404 on GetUser.
var ErrUserNotFound = errors.New("adminclient: user not found")

// ErrAdminUnavailable wraps any transport or 5xx failure when talking to admin.
// Handlers surface this as a 502 so callers learn the failure is upstream.
var ErrAdminUnavailable = errors.New("adminclient: admin service unavailable")

// ErrBadResponse covers malformed or unexpected admin responses.
var ErrBadResponse = errors.New("adminclient: bad response from admin")
