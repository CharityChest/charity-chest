// Package email holds the templ-rendered HTML bodies for outbound transactional
// emails (password recovery, future invitations, etc.). One file per email
// keeps the directory browsable; the package only ever produces HTML — the
// plaintext alternative is assembled in the caller because HTML-escaping is
// the wrong policy for plaintext.
package email

// PasswordResetData feeds the PasswordReset template. The caller pre-composes
// GreetingLine ("Hello[, Name],") in Go so the templ source can render the
// whole salutation as a single auto-escaped expression — keeping the conditional
// name suffix out of the template entirely.
type PasswordResetData struct {
	GreetingLine string
	Intro        string
	CTA          string
	URL          string
	Expiry       string
	Ignore       string
	Footer       string
}
