package handler_test

// Black-box tests for the handler internals that used to live in
// auth_internal_test.go. Each group exercises an unexported helper through the
// nearest public entry point:
//
//   - Google OAuth helpers (fetchGoogleUserInfo, findOrCreateGoogleUser)
//     are driven through GoogleCallback by stubbing the upstream Google
//     endpoints on http.DefaultClient.Transport. A single transport shim
//     covers both the token exchange and the userinfo lookup because the
//     oauth2 library and the handler both reach into the default client.
//
//   - buildResetURL is exercised through ForgotPassword by capturing the
//     outbound email with a fakeMailer and reading the link out of the body.
//
//   - newGoMailMailer's AUTH wiring is exercised by pointing the production
//     mailer at an in-process SMTP capture server and observing whether the
//     AUTH command appears on the wire.
//
//   - disabledMailer.Send is exercised by constructing NewAuthHandler with an
//     empty SMTPHost (so the disabled mailer is wired in) and confirming the
//     forgot-password endpoint still returns its neutral 204.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"charity-chest/internal/cache"
	"charity-chest/internal/handler"
	ccmiddleware "charity-chest/internal/middleware"
	"charity-chest/internal/model"

	"github.com/labstack/echo/v4"
)

// roundTripperFunc adapts a function into an http.RoundTripper so tests can
// hijack http.DefaultClient.Transport without spinning up a real server.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// stubGoogleTransport installs a transport on http.DefaultClient that answers
// the token-exchange call with a fixed access token and routes the userinfo
// call to the supplied function. The previous transport is restored on test
// cleanup so parallel-friendly tests stay isolated.
func stubGoogleTransport(t *testing.T, userinfo func(*http.Request) (*http.Response, error)) {
	t.Helper()
	old := http.DefaultClient.Transport
	t.Cleanup(func() { http.DefaultClient.Transport = old })

	http.DefaultClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "oauth2.googleapis.com":
			body := `{"access_token":"the-token","token_type":"Bearer","expires_in":3600}`
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "www.googleapis.com":
			return userinfo(r)
		}
		return nil, errors.New("stubGoogleTransport: unexpected URL " + r.URL.String())
	})
}

// doGoogleCallback drives the callback endpoint with a matching state cookie
// so the only variables under test are the stubbed upstream responses.
func doGoogleCallback(e *echo.Echo) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/google/callback?state=abc&code=auth-code", nil)
	req.AddCookie(&http.Cookie{Name: handler.CookieOAuthState, Value: "abc"})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// --- fetchGoogleUserInfo behaviours, exercised through GoogleCallback ---

func TestGoogleCallback_FetchUserInfo_Success(t *testing.T) {
	var sawAuthHeader string
	stubGoogleTransport(t, func(r *http.Request) (*http.Response, error) {
		sawAuthHeader = r.Header.Get("Authorization")
		return jsonResponse(`{"id":"g-1","email":"u@example.com","name":"User"}`), nil
	})

	e, _, db := newServer(t)
	rec := doGoogleCallback(e)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307; body: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "?token=") {
		t.Errorf("Location %q missing token query", loc)
	}
	if sawAuthHeader != "Bearer the-token" {
		t.Errorf("Authorization header forwarded to Google = %q", sawAuthHeader)
	}

	var user model.User
	if err := db.Where("email = ?", "u@example.com").First(&user).Error; err != nil {
		t.Fatalf("user not persisted: %v", err)
	}
	if user.GoogleID == nil || *user.GoogleID != "g-1" {
		t.Errorf("GoogleID = %v, want %q", user.GoogleID, "g-1")
	}
}

func TestGoogleCallback_FetchUserInfo_TransportError(t *testing.T) {
	stubGoogleTransport(t, func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network down")
	})

	e, _, _ := newServer(t)
	rec := doGoogleCallback(e)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "error=sign_in_failed") {
		t.Errorf("Location %q does not contain error=sign_in_failed", loc)
	}
}

func TestGoogleCallback_FetchUserInfo_InvalidJSON(t *testing.T) {
	stubGoogleTransport(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(`not json`), nil
	})

	e, _, _ := newServer(t)
	rec := doGoogleCallback(e)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307", rec.Code)
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "error=sign_in_failed") {
		t.Errorf("Location %q does not contain error=sign_in_failed", loc)
	}
}

// --- findOrCreateGoogleUser behaviours, exercised through GoogleCallback ---

func TestGoogleCallback_NewUser_Created(t *testing.T) {
	stubGoogleTransport(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"id":"g-new","email":"new@example.com","name":"New"}`), nil
	})

	e, _, db := newServer(t)
	if rec := doGoogleCallback(e); rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307", rec.Code)
	}

	var count int64
	db.Model(&model.User{}).Where("email = ?", "new@example.com").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 row in users; got %d", count)
	}

	var user model.User
	if err := db.Where("email = ?", "new@example.com").First(&user).Error; err != nil {
		t.Fatalf("look up created user: %v", err)
	}
	if user.GoogleID == nil || *user.GoogleID != "g-new" {
		t.Errorf("GoogleID = %v, want %q", user.GoogleID, "g-new")
	}
}

func TestGoogleCallback_ExistingByGoogleID_Found(t *testing.T) {
	stubGoogleTransport(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"id":"g-existing","email":"pre@example.com","name":"Pre"}`), nil
	})

	e, _, db := newServer(t)
	gid := "g-existing"
	pre := &model.User{Email: "pre@example.com", Name: "Pre", GoogleID: &gid}
	db.Create(pre)

	if rec := doGoogleCallback(e); rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307", rec.Code)
	}

	var count int64
	db.Model(&model.User{}).Where("email = ?", "pre@example.com").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 row (existing user reused); got %d", count)
	}
}

func TestGoogleCallback_ExistingByEmail_LinksGoogleID(t *testing.T) {
	stubGoogleTransport(t, func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"id":"g-link","email":"link@example.com","name":"Link"}`), nil
	})

	e, _, db := newServer(t)
	pre := &model.User{Email: "link@example.com", Name: "Link"}
	db.Create(pre)

	if rec := doGoogleCallback(e); rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want 307", rec.Code)
	}

	var reloaded model.User
	if err := db.First(&reloaded, pre.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloaded.GoogleID == nil || *reloaded.GoogleID != "g-link" {
		t.Errorf("GoogleID = %v, want %q linked on existing email account", reloaded.GoogleID, "g-link")
	}
}

// --- buildResetURL — exercised through ForgotPassword + captured email body ---
//
// The reset URL the handler bakes into the email is the only externally
// observable side effect of buildResetURL. By capturing the message with the
// fakeMailer defined in auth_password_reset_test.go we can assert the locale
// segment without reaching into the AuthHandler's unexported method.

// newForgotPasswordServerWithLocale wires the minimum Echo stack needed to
// drive ForgotPassword end-to-end with the locale middleware in place so an
// inbound X-Locale header reaches the handler.
func newForgotPasswordServerWithLocale(t *testing.T) (*echo.Echo, *fakeMailer) {
	t.Helper()
	db := newTestDB(t)
	cfg := testCfg()
	cfg.FrontendURL = "https://app.example.test"
	mailer := &fakeMailer{}
	h := handler.NewAuthHandlerWithMailer(db, cfg, cache.Disabled(), mailer)

	e := echo.New()
	e.Use(ccmiddleware.Locale())
	v1 := e.Group("/v1")
	auth := v1.Group("/auth")
	auth.POST("/register", h.Register)
	auth.POST("/password/forgot", h.ForgotPassword)

	return e, mailer
}

func TestForgotPassword_BuildsEnglishResetURL(t *testing.T) {
	e, mailer := newForgotPasswordServerWithLocale(t)
	postJSON(e, "/v1/auth/register", `{"email":"en@example.com","password":"password123","name":"EN"}`)

	rec := postJSONWithHeader(e, "/v1/auth/password/forgot", `{"email":"en@example.com"}`, map[string]string{"X-Locale": "en"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	calls := mailer.waitForCalls(t, 1)
	if !strings.Contains(calls[0].HTMLBody, "https://app.example.test/en/reset-password?token=") {
		t.Errorf("HTMLBody missing English reset URL: %s", calls[0].HTMLBody)
	}
}

func TestForgotPassword_BuildsItalianResetURL(t *testing.T) {
	e, mailer := newForgotPasswordServerWithLocale(t)
	postJSON(e, "/v1/auth/register", `{"email":"it@example.com","password":"password123","name":"IT"}`)

	rec := postJSONWithHeader(e, "/v1/auth/password/forgot", `{"email":"it@example.com"}`, map[string]string{"X-Locale": "it"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	calls := mailer.waitForCalls(t, 1)
	if !strings.Contains(calls[0].HTMLBody, "https://app.example.test/it/reset-password?token=") {
		t.Errorf("HTMLBody missing Italian reset URL: %s", calls[0].HTMLBody)
	}
}

func TestForgotPassword_ResetURL_FallsBackToEN_OnUnknownLocale(t *testing.T) {
	e, mailer := newForgotPasswordServerWithLocale(t)
	postJSON(e, "/v1/auth/register", `{"email":"zz@example.com","password":"password123","name":"ZZ"}`)

	rec := postJSONWithHeader(e, "/v1/auth/password/forgot", `{"email":"zz@example.com"}`, map[string]string{"X-Locale": "zz"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	calls := mailer.waitForCalls(t, 1)
	if !strings.Contains(calls[0].HTMLBody, "https://app.example.test/en/reset-password?token=") {
		t.Errorf("HTMLBody should fall back to /en/ URL: %s", calls[0].HTMLBody)
	}
}

// --- newGoMailMailer AUTH wiring — exercised through a fake SMTP server ---
//
// The unexported newGoMailMailer constructor builds a goMailMailer that
// decides at Send-time whether to include an AUTH command based on whether
// SMTP credentials were supplied in the config. The only externally visible
// difference between the two branches is the SMTP wire conversation, so we
// stand up a minimal in-process listener that speaks just enough SMTP to
// satisfy go-mail and record whether AUTH ever appeared.

// smtpCapture is a tiny in-process SMTP listener used by the goMailMailer
// tests. It accepts connections, runs through the SMTP commands go-mail
// issues, and records whether the client attempted AUTH. We bind on port
// 1025 specifically because goMailMailer.Send hard-codes TLSPolicy=NoTLS for
// that port (matching the MailHog dev convention); on any other port the
// default TLSMandatory policy would require STARTTLS support that this
// minimal listener cannot provide.
type smtpCapture struct {
	ln        net.Listener
	host      string
	port      int
	authSeen  int32
	delivered int32
}

// startSMTPCapture stands up the listener on port 1025. If that port is in
// use (e.g. a local MailHog is running on the dev box) the test is skipped —
// the goMailMailer's TLS contract makes any other port unworkable for a
// plaintext fake server.
func startSMTPCapture(t *testing.T) *smtpCapture {
	t.Helper()
	const smtpPort = 1025
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", smtpPort))
	if err != nil {
		t.Skipf("smtpCapture: port %d unavailable (%v) — skipping wire-level SMTP test", smtpPort, err)
	}
	s := &smtpCapture{ln: ln, host: "127.0.0.1", port: smtpPort}
	go s.acceptLoop()
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

func (s *smtpCapture) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *smtpCapture) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	fmt.Fprintf(conn, "220 fake-smtp ESMTP ready\r\n")

	r := bufio.NewReader(conn)
	inData := false
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if inData {
			if trimmed == "." {
				fmt.Fprintf(conn, "250 OK message accepted\r\n")
				atomic.AddInt32(&s.delivered, 1)
				inData = false
			}
			continue
		}
		upper := strings.ToUpper(trimmed)
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			// Advertise AUTH but NOT STARTTLS so TLSOpportunistic stays plaintext.
			fmt.Fprintf(conn, "250-fake-smtp\r\n250-AUTH PLAIN LOGIN\r\n250 SIZE 10240000\r\n")
		case strings.HasPrefix(upper, "AUTH"):
			atomic.StoreInt32(&s.authSeen, 1)
			fmt.Fprintf(conn, "235 2.7.0 Authentication successful\r\n")
		case strings.HasPrefix(upper, "MAIL FROM:"):
			fmt.Fprintf(conn, "250 OK\r\n")
		case strings.HasPrefix(upper, "RCPT TO:"):
			fmt.Fprintf(conn, "250 OK\r\n")
		case upper == "DATA":
			fmt.Fprintf(conn, "354 End data with <CR><LF>.<CR><LF>\r\n")
			inData = true
		case upper == "QUIT":
			fmt.Fprintf(conn, "221 bye\r\n")
			return
		case upper == "RSET", upper == "NOOP":
			fmt.Fprintf(conn, "250 OK\r\n")
		default:
			fmt.Fprintf(conn, "250 OK\r\n")
		}
	}
}

func (s *smtpCapture) waitForDelivery(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&s.delivered) > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("smtpCapture: no delivery within timeout")
}

// newForgotPasswordServerWithSMTP wires NewAuthHandler (not the WithMailer
// variant) so the production goMailMailer is constructed from the config.
// Tests pass an smtpCapture address + optional credentials to exercise the
// two branches of newGoMailMailer.
func newForgotPasswordServerWithSMTP(t *testing.T, host string, port int, user, pass string) *echo.Echo {
	t.Helper()
	db := newTestDB(t)
	cfg := testCfg()
	cfg.FrontendURL = "https://app.example.test"
	cfg.SMTPHost = host
	cfg.SMTPPort = port
	cfg.SMTPUsername = user
	cfg.SMTPPassword = pass
	cfg.SMTPFrom = "no-reply@example.test"
	cfg.SMTPFromName = "Example"

	h := handler.NewAuthHandler(db, cfg, cache.Disabled())

	e := echo.New()
	v1 := e.Group("/v1")
	auth := v1.Group("/auth")
	auth.POST("/register", h.Register)
	auth.POST("/password/forgot", h.ForgotPassword)
	return e
}

func TestForgotPassword_GoMailMailer_SkipsAuthWhenNoCreds(t *testing.T) {
	srv := startSMTPCapture(t)
	e := newForgotPasswordServerWithSMTP(t, srv.host, srv.port, "", "")
	postJSON(e, "/v1/auth/register", `{"email":"noauth@example.com","password":"password123","name":"NoAuth"}`)

	rec := postJSON(e, "/v1/auth/password/forgot", `{"email":"noauth@example.com"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	srv.waitForDelivery(t)
	if atomic.LoadInt32(&srv.authSeen) != 0 {
		t.Error("AUTH command should not be sent when SMTP credentials are absent")
	}
}

func TestForgotPassword_GoMailMailer_SendsAuthWhenCredsPresent(t *testing.T) {
	srv := startSMTPCapture(t)
	e := newForgotPasswordServerWithSMTP(t, srv.host, srv.port, "user", "pass")
	postJSON(e, "/v1/auth/register", `{"email":"auth@example.com","password":"password123","name":"WithAuth"}`)

	rec := postJSON(e, "/v1/auth/password/forgot", `{"email":"auth@example.com"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	srv.waitForDelivery(t)
	if atomic.LoadInt32(&srv.authSeen) != 1 {
		t.Error("AUTH command should be sent when SMTP credentials are present")
	}
}

// --- disabledMailer.Send — exercised through ForgotPassword with no SMTP ---
//
// When cfg.SMTPHost is empty, NewAuthHandler wires the disabledMailer. The
// only externally observable behaviour is that the recovery email is dropped
// silently while the endpoint keeps its enumeration-safe 204 contract.

func TestForgotPassword_DisabledMailer_StillReturns204(t *testing.T) {
	db := newTestDB(t)
	cfg := testCfg()
	cfg.FrontendURL = "https://app.example.test"
	// cfg.SMTPHost intentionally left empty so NewAuthHandler picks disabledMailer.
	h := handler.NewAuthHandler(db, cfg, cache.Disabled())

	e := echo.New()
	v1 := e.Group("/v1")
	v1.Group("/auth").POST("/register", h.Register)
	v1.Group("/auth").POST("/password/forgot", h.ForgotPassword)

	postJSON(e, "/v1/auth/register", `{"email":"off@example.com","password":"password123","name":"Off"}`)

	rec := postJSON(e, "/v1/auth/password/forgot", `{"email":"off@example.com"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}
