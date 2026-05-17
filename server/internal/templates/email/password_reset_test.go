package email_test

import (
	"bytes"
	"strings"
	"testing"

	"charity-chest/internal/templates/email"
)

// TestPasswordReset_RendersAllFields exercises the templ-generated component
// end to end. It is intentionally minimal — the goal is to detect regressions
// in the data → output wiring (a renamed field, a forgotten interpolation),
// not to lock the markup down to the byte. We only assert on the values we
// actually feed in, never on the surrounding HTML, so a future restyle of the
// email body does not require touching this test.
func TestPasswordReset_RendersAllFields(t *testing.T) {
	t.Parallel()

	data := email.PasswordResetData{
		GreetingLine: "Hello Alice,",
		Intro:        "intro-marker",
		CTA:          "cta-marker",
		URL:          "https://app.example.test/en/reset-password?token=abc",
		Expiry:       "expiry-marker",
		Ignore:       "ignore-marker",
		Footer:       "footer-marker",
	}

	var buf bytes.Buffer
	if err := email.PasswordReset(data).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		data.GreetingLine,
		data.Intro,
		data.CTA,
		data.URL,
		data.Expiry,
		data.Ignore,
		data.Footer,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered HTML missing %q\n---\n%s\n---", want, out)
		}
	}
}

// TestPasswordReset_EscapesHTMLInData verifies the auto-escaping contract we
// rely on for security: every interpolated string flows through templ's
// HTML escaper, so a hostile display name cannot inject markup into the
// rendered email. This is the load-bearing reason we use templ here rather
// than concatenating into raw HTML.
func TestPasswordReset_EscapesHTMLInData(t *testing.T) {
	t.Parallel()

	data := email.PasswordResetData{
		GreetingLine: "Hello <script>alert('xss')</script>,",
		Intro:        "intro",
		CTA:          "cta",
		URL:          "https://example.test/reset",
		Expiry:       "expiry",
		Ignore:       "ignore",
		Footer:       "footer",
	}

	var buf bytes.Buffer
	if err := email.PasswordReset(data).Render(t.Context(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := buf.String()

	if strings.Contains(out, "<script>") {
		t.Fatalf("raw <script> tag survived rendering — auto-escape broken:\n%s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Errorf("expected escaped <script> in output, got:\n%s", out)
	}
}
