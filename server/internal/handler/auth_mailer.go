package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"charity-chest/internal/config"
	"charity-chest/internal/i18n"
	"charity-chest/internal/templates/email"

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
	host      string
	port      int
	username  string
	password  string
	from      string
	fromName  string
	authSet   bool
	forceIPv4 bool
}

// newGoMailMailer returns a configured production mailer.
// authSet records whether SMTP authentication was supplied so the mailer can
// skip the AUTH step when talking to relays that reject it (e.g. Mailpit).
func newGoMailMailer(cfg *config.Config) *goMailMailer {
	return &goMailMailer{
		host:      cfg.SMTPHost,
		port:      cfg.SMTPPort,
		username:  cfg.SMTPUsername,
		password:  cfg.SMTPPassword,
		from:      cfg.SMTPFrom,
		fromName:  cfg.SMTPFromName,
		authSet:   cfg.SMTPUsername != "" && cfg.SMTPPassword != "",
		forceIPv4: cfg.SMTPForceIPv4,
	}
}

// ipv4DialContext is a go-mail DialContextFunc that pins the SMTP connection
// to IPv4. The network argument supplied by go-mail (always "tcp", including
// on its internal fallback in client.go) is intentionally ignored so the
// resolver never returns an AAAA record the egress path can't reach.
func ipv4DialContext(ctx context.Context, _, address string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "tcp4", address)
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
		// Mailpit and similar capture relays reject the AUTH command outright.
		opts = append(opts, gomail.WithSMTPAuth(gomail.SMTPAuthNoAuth))
	}
	// Avoid TLS on plaintext capture servers (port 1025 is the Mailpit default).
	if m.port == 1025 {
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	}
	if m.forceIPv4 {
		opts = append(opts, gomail.WithDialContextFunc(ipv4DialContext))
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

// --- Email rendering ---

// renderPasswordResetEmail produces the localized HTML and plaintext bodies
// for a password reset email. The HTML body is rendered by the templ component
// in `internal/templates/email/password_reset.templ`, which auto-escapes every
// interpolation so a malicious display name cannot inject markup. The plaintext
// body is assembled with a simple string builder because HTML-escaping is the
// wrong policy for plaintext (it would turn `<` into `&lt;` in the user's
// inbox).
func renderPasswordResetEmail(locale, name, resetURL string) (htmlBody, textBody string, err error) {
	greeting := i18n.T(locale, i18n.KeyPasswordResetEmailGreeting)
	greetingLine := greeting + ","
	if name != "" {
		greetingLine = greeting + " " + name + ","
	}

	data := email.PasswordResetData{
		GreetingLine: greetingLine,
		Intro:        i18n.T(locale, i18n.KeyPasswordResetEmailIntro),
		CTA:          i18n.T(locale, i18n.KeyPasswordResetEmailCTA),
		URL:          resetURL,
		Expiry:       i18n.T(locale, i18n.KeyPasswordResetEmailExpiry),
		Ignore:       i18n.T(locale, i18n.KeyPasswordResetEmailIgnore),
		Footer:       i18n.T(locale, i18n.KeyPasswordResetEmailFooter),
	}

	var htmlBuf bytes.Buffer
	if err := email.PasswordReset(data).Render(context.Background(), &htmlBuf); err != nil {
		return "", "", fmt.Errorf("mailer: render html template: %w", err)
	}

	var textBuf strings.Builder
	fmt.Fprintf(&textBuf, "%s\n\n%s\n\n%s: %s\n\n%s\n\n%s\n\n—\n%s\n",
		data.GreetingLine,
		data.Intro,
		data.CTA, data.URL,
		data.Expiry,
		data.Ignore,
		data.Footer,
	)

	return htmlBuf.String(), textBuf.String(), nil
}
