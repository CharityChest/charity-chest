// Package i18n exposes typed translation keys and a T(locale, key) lookup
// used by every handler and middleware in the operational backend.
// Mirrors the admin i18n package shape so error UX feels consistent across
// services. Only "en" and "it" are supported; everything else falls back to
// English, and a missing key falls back to its string form.
package i18n

// Key is a typed string that prevents raw strings from being passed where a
// message key is expected.
type Key string

// Message keys passed to T to look up a translated error string.
const (
	KeyInvalidBody        Key = "invalid_body"
	KeyFieldsRequired     Key = "fields_required"
	KeyInvalidCredentials Key = "invalid_credentials"
	KeyGoogleOnly         Key = "google_only"
	KeyMissingAuthHeader  Key = "missing_auth_header"
	KeyUnexpectedSigning  Key = "unexpected_signing"
	KeyInvalidToken       Key = "invalid_token"
	KeyInvalidClaims      Key = "invalid_claims"
	KeyGenerateToken      Key = "generate_token"
	KeyAdminUnavailable   Key = "admin_unavailable"
	KeyMFANotSupported    Key = "mfa_not_supported_on_mobile"
	KeyGoogleVerifyFailed Key = "google_verify_failed"
	KeyUserNotFound       Key = "user_not_found"
)

// messages maps locale → Key → translated string.
var messages = map[string]map[Key]string{
	"en": {
		KeyInvalidBody:        "invalid request body",
		KeyFieldsRequired:     "required fields are missing",
		KeyInvalidCredentials: "invalid credentials",
		KeyGoogleOnly:         "this account uses Google login",
		KeyMissingAuthHeader:  "missing or invalid authorization header",
		KeyUnexpectedSigning:  "unexpected signing method",
		KeyInvalidToken:       "invalid or expired token",
		KeyInvalidClaims:      "invalid token claims",
		KeyGenerateToken:      "failed to generate token",
		KeyAdminUnavailable:   "the identity service is temporarily unavailable",
		KeyMFANotSupported:    "MFA-enabled accounts cannot sign in from the mobile app yet — please use the admin web app",
		KeyGoogleVerifyFailed: "could not verify Google sign-in",
		KeyUserNotFound:       "user not found",
	},
	"it": {
		KeyInvalidBody:        "corpo della richiesta non valido",
		KeyFieldsRequired:     "campi obbligatori mancanti",
		KeyInvalidCredentials: "Credenziali non valide",
		KeyGoogleOnly:         "questo account utilizza l'accesso con Google",
		KeyMissingAuthHeader:  "intestazione di autorizzazione mancante o non valida",
		KeyUnexpectedSigning:  "metodo di firma non previsto",
		KeyInvalidToken:       "token non valido o scaduto",
		KeyInvalidClaims:      "claim del token non validi",
		KeyGenerateToken:      "errore nella generazione del token",
		KeyAdminUnavailable:   "il servizio identità è temporaneamente non disponibile",
		KeyMFANotSupported:    "gli account con MFA non possono ancora accedere dall'app mobile — usa l'app web di amministrazione",
		KeyGoogleVerifyFailed: "impossibile verificare l'accesso con Google",
		KeyUserNotFound:       "utente non trovato",
	},
}

// T returns the translated string for the given locale and key.
// Unknown locales fall back to "en"; missing keys fall back to the key string.
func T(locale string, key Key) string {
	if locale != "en" && locale != "it" {
		locale = "en"
	}
	if m, ok := messages[locale]; ok {
		if s, ok := m[key]; ok {
			return s
		}
	}
	return string(key)
}
