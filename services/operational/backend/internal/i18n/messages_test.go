package i18n_test

import (
	"testing"

	"charity-chest/services/operational/backend/internal/i18n"
)

func TestT_EnglishAndItalian(t *testing.T) {
	if got := i18n.T("en", i18n.KeyInvalidCredentials); got != "invalid credentials" {
		t.Errorf("en = %q", got)
	}
	if got := i18n.T("it", i18n.KeyInvalidCredentials); got != "Credenziali non valide" {
		t.Errorf("it = %q", got)
	}
}

func TestT_UnknownLocaleFallsBackToEnglish(t *testing.T) {
	en := i18n.T("en", i18n.KeyUserNotFound)
	if got := i18n.T("fr", i18n.KeyUserNotFound); got != en {
		t.Errorf("fr fallback = %q, want %q", got, en)
	}
	if got := i18n.T("", i18n.KeyUserNotFound); got != en {
		t.Errorf("empty-locale fallback = %q, want %q", got, en)
	}
}

func TestT_MissingKeyFallsBackToKeyString(t *testing.T) {
	const unknown = i18n.Key("definitely_not_a_real_key")
	if got := i18n.T("en", unknown); got != string(unknown) {
		t.Errorf("missing-key fallback = %q, want %q", got, string(unknown))
	}
}

// Every key defined in the package must have both an EN and IT translation —
// guards against adding a key and forgetting one locale.
func TestT_AllKeysTranslatedInBothLocales(t *testing.T) {
	keys := []i18n.Key{
		i18n.KeyInvalidBody, i18n.KeyFieldsRequired, i18n.KeyInvalidCredentials,
		i18n.KeyGoogleOnly, i18n.KeyMissingAuthHeader, i18n.KeyUnexpectedSigning,
		i18n.KeyInvalidToken, i18n.KeyInvalidClaims, i18n.KeyGenerateToken,
		i18n.KeyAdminUnavailable, i18n.KeyMFANotSupported, i18n.KeyGoogleVerifyFailed,
		i18n.KeyUserNotFound,
	}
	for _, k := range keys {
		for _, loc := range []string{"en", "it"} {
			if got := i18n.T(loc, k); got == string(k) {
				t.Errorf("key %q has no %s translation (fell back to key string)", k, loc)
			}
		}
	}
}
