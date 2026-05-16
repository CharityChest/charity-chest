package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	texttemplate "text/template"
	"time"

	"charity-chest/internal/config"
	"charity-chest/internal/i18n"

	gomail "github.com/wneessen/go-mail"
)

// ErrMailerDisabled is returned by the disabled mailer to signal that no SMTP
// configuration was supplied. The forgot-password handler logs it and still
// returns the neutral 2xx response so the disabled state is not observable to
// clients (which would otherwise leak the operator's email configuration).
var ErrMailerDisabled = errors.New("mailer: SMTP is not configured")

// MailerGateway abstracts outbound email so tests can capture sends without
// touching the network. It mirrors handler.StripeGateway in shape, providing a
// stable seam for production (goMailMailer) and tests (fakeMailer).
type MailerGateway interface {
	// Send delivers a single message with an HTML body and a plaintext
	// alternative. Implementations are responsible for their own timeouts;
	// callers should pass a derived context with a deadline.
	Send(ctx context.Context, to, subject, htmlBody, textBody string) error
}

// goMailMailer is the production MailerGateway implementation backed by
// github.com/wneessen/go-mail. It is constructed once at server startup with
// the resolved SMTP configuration; the client is reused across requests so
// each Send opens a fresh connection but reuses the same TLS settings.
type goMailMailer struct {
	host     string
	port     int
	username string
	password string
	from     string
	fromName string
	authSet  bool
}

// newGoMailMailer returns a configured production mailer.
// authSet records whether SMTP authentication was supplied so the mailer can
// skip the AUTH step when talking to relays that reject it (e.g. MailHog).
func newGoMailMailer(cfg *config.Config) *goMailMailer {
	return &goMailMailer{
		host:     cfg.SMTPHost,
		port:     cfg.SMTPPort,
		username: cfg.SMTPUsername,
		password: cfg.SMTPPassword,
		from:     cfg.SMTPFrom,
		fromName: cfg.SMTPFromName,
		authSet:  cfg.SMTPUsername != "" && cfg.SMTPPassword != "",
	}
}

// Send delivers the message synchronously. The caller is expected to invoke
// this inside a background goroutine — the password-reset handler does not
// block its HTTP response on SMTP latency.
func (m *goMailMailer) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	msg := gomail.NewMsg()
	if err := msg.FromFormat(m.fromName, m.from); err != nil {
		return fmt.Errorf("mailer: set From: %w", err)
	}
	if err := msg.To(to); err != nil {
		return fmt.Errorf("mailer: set To: %w", err)
	}
	msg.Subject(subject)
	msg.SetBodyString(gomail.TypeTextPlain, textBody)
	msg.AddAlternativeString(gomail.TypeTextHTML, htmlBody)

	opts := []gomail.Option{
		gomail.WithPort(m.port),
		gomail.WithTimeout(30 * time.Second),
	}
	if m.authSet {
		opts = append(opts,
			gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
			gomail.WithUsername(m.username),
			gomail.WithPassword(m.password),
		)
	} else {
		// MailHog and similar capture relays reject the AUTH command outright.
		opts = append(opts, gomail.WithSMTPAuth(gomail.SMTPAuthNoAuth))
	}
	// Avoid TLS on plaintext capture servers (port 1025 is the MailHog default).
	if m.port == 1025 {
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	}

	client, err := gomail.NewClient(m.host, opts...)
	if err != nil {
		return fmt.Errorf("mailer: new client: %w", err)
	}
	return client.DialAndSendWithContext(ctx, msg)
}

// disabledMailer is wired in when SMTP is unconfigured. It records nothing and
// always returns ErrMailerDisabled so the handler can log a clear "feature is
// off" warning without exposing the state to clients.
type disabledMailer struct{}

// Send always fails with ErrMailerDisabled.
func (disabledMailer) Send(context.Context, string, string, string, string) error {
	return ErrMailerDisabled
}

// --- Email templates ---

// passwordResetTemplateData is the data passed to the HTML and text templates.
// Every user-controllable value (name) is rendered through the template engine,
// never concatenated with the localized strings, so a malicious display name
// cannot escape into HTML markup.
type passwordResetTemplateData struct {
	Greeting string
	Name     string
	Intro    string
	CTA      string
	URL      string
	Expiry   string
	Ignore   string
	Footer   string
}

// passwordResetHTMLTemplate renders the HTML alternative of the recovery email.
// Width is constrained for readability in narrow email clients; styling is
// intentionally inline because Gmail and friends strip <style> blocks.
const passwordResetHTMLTemplate = `<!DOCTYPE html>
<html>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 560px; margin: 0 auto; padding: 24px; color: #1f2937;">
  <p style="font-size: 16px;">{{.Greeting}}{{if .Name}} {{.Name}}{{end}},</p>
  <p style="font-size: 16px; line-height: 1.5;">{{.Intro}}</p>
  <p style="margin: 32px 0;">
    <a href="{{.URL}}" style="background: #2563eb; color: #ffffff; padding: 12px 24px; border-radius: 6px; text-decoration: none; font-weight: 600;">{{.CTA}}</a>
  </p>
  <p style="font-size: 14px; color: #6b7280;">{{.Expiry}}</p>
  <p style="font-size: 14px; color: #6b7280; line-height: 1.5;">{{.Ignore}}</p>
  <hr style="margin: 32px 0; border: none; border-top: 1px solid #e5e7eb;">
  <p style="font-size: 14px; color: #6b7280;">{{.Footer}}</p>
</body>
</html>`

// passwordResetTextTemplate renders the plaintext alternative for clients that
// do not display HTML.
const passwordResetTextTemplate = `{{.Greeting}}{{if .Name}} {{.Name}}{{end}},

{{.Intro}}

{{.CTA}}: {{.URL}}

{{.Expiry}}

{{.Ignore}}

—
{{.Footer}}
`

// renderPasswordResetEmail produces the localized HTML and plaintext bodies
// for a password reset email. The locale is resolved via i18n.T; user-supplied
// values flow through html/template so they cannot inject markup.
func renderPasswordResetEmail(locale, name, resetURL string) (htmlBody, textBody string, err error) {
	data := passwordResetTemplateData{
		Greeting: i18n.T(locale, i18n.KeyPasswordResetEmailGreeting),
		Name:     name,
		Intro:    i18n.T(locale, i18n.KeyPasswordResetEmailIntro),
		CTA:      i18n.T(locale, i18n.KeyPasswordResetEmailCTA),
		URL:      resetURL,
		Expiry:   i18n.T(locale, i18n.KeyPasswordResetEmailExpiry),
		Ignore:   i18n.T(locale, i18n.KeyPasswordResetEmailIgnore),
		Footer:   i18n.T(locale, i18n.KeyPasswordResetEmailFooter),
	}

	htmlTpl, err := template.New("reset-html").Parse(passwordResetHTMLTemplate)
	if err != nil {
		return "", "", fmt.Errorf("mailer: parse html template: %w", err)
	}
	var htmlBuf bytes.Buffer
	if err := htmlTpl.Execute(&htmlBuf, data); err != nil {
		return "", "", fmt.Errorf("mailer: execute html template: %w", err)
	}

	textTpl, err := texttemplate.New("reset-text").Parse(passwordResetTextTemplate)
	if err != nil {
		return "", "", fmt.Errorf("mailer: parse text template: %w", err)
	}
	var textBuf bytes.Buffer
	if err := textTpl.Execute(&textBuf, data); err != nil {
		return "", "", fmt.Errorf("mailer: execute text template: %w", err)
	}

	return htmlBuf.String(), textBuf.String(), nil
}
