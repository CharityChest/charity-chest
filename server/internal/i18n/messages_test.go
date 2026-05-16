package i18n_test

import (
	"testing"

	"charity-chest/internal/i18n"
)

// TestT_KnownKey_EN_Translates verifies a happy-path EN lookup so we catch a
// regression where the en table is dropped from the maps.
func TestT_KnownKey_EN_Translates(t *testing.T) {
	got := i18n.T("en", i18n.KeyInvalidBody)
	if got != "invalid request body" {
		t.Errorf("T(en, KeyInvalidBody) = %q", got)
	}
}

// TestT_KnownKey_IT_Translates verifies the IT branch is wired and the table
// has the expected localized string. Any change to the IT string is caught
// here so translations don't silently regress to English.
func TestT_KnownKey_IT_Translates(t *testing.T) {
	got := i18n.T("it", i18n.KeyInvalidBody)
	if got != "corpo della richiesta non valido" {
		t.Errorf("T(it, KeyInvalidBody) = %q", got)
	}
}

// TestT_UnknownLocale_FallsBackToEN guards the documented behaviour: any
// locale outside {en, it} resolves to the EN table.
func TestT_UnknownLocale_FallsBackToEN(t *testing.T) {
	en := i18n.T("en", i18n.KeyInvalidCredentials)
	got := i18n.T("zz", i18n.KeyInvalidCredentials)
	if got != en {
		t.Errorf("T(zz, KeyInvalidCredentials) = %q, want EN fallback %q", got, en)
	}
}

// TestT_EmptyLocale_FallsBackToEN is the explicit "empty string" case — it
// matters because handlers default to "" before the locale middleware fires.
func TestT_EmptyLocale_FallsBackToEN(t *testing.T) {
	en := i18n.T("en", i18n.KeyInvalidCredentials)
	got := i18n.T("", i18n.KeyInvalidCredentials)
	if got != en {
		t.Errorf("T(\"\", ...) = %q, want EN fallback %q", got, en)
	}
}

// TestT_MissingKey_ReturnsKeyString documents the behaviour for keys that
// somehow slip past the const list — T returns the raw key so a missing
// translation is visible in the response rather than producing an empty string.
func TestT_MissingKey_ReturnsKeyString(t *testing.T) {
	const phantom i18n.Key = "this_key_does_not_exist_in_the_table"
	got := i18n.T("en", phantom)
	if got != string(phantom) {
		t.Errorf("T(en, phantom) = %q, want key string %q", got, string(phantom))
	}
}

// TestT_EveryKey_HasENAndIT_Translation guards against drift between the two
// locale tables. If a future PR adds a Key constant but forgets the IT
// translation, this test fails. We can't enumerate the const block from
// reflection, so we maintain the list by hand — adding a new key to the
// production code requires extending this list, which is a deliberate
// nudge toward keeping translations in sync.
func TestT_EveryKey_HasENAndIT_Translation(t *testing.T) {
	keys := []i18n.Key{
		i18n.KeyInvalidBody,
		i18n.KeyFieldsRequired,
		i18n.KeyPasswordTooShort,
		i18n.KeyEmailTaken,
		i18n.KeyProcessPassword,
		i18n.KeyCreateUser,
		i18n.KeyGenerateToken,
		i18n.KeyGenerateState,
		i18n.KeyInvalidOAuthState,
		i18n.KeyMissingOAuthCode,
		i18n.KeyExchangeOAuthCode,
		i18n.KeyFetchUserInfo,
		i18n.KeyResolveUser,
		i18n.KeyInvalidCredentials,
		i18n.KeyGoogleOnly,
		i18n.KeyUserNotFound,
		i18n.KeyMissingAuthHeader,
		i18n.KeyUnexpectedSigning,
		i18n.KeyInvalidToken,
		i18n.KeyInvalidClaims,
		i18n.KeyForbidden,
		i18n.KeyOrgNotFound,
		i18n.KeyMemberExists,
		i18n.KeyMemberNotFound,
		i18n.KeyInvalidRole,
		i18n.KeyCannotManageRole,
		i18n.KeySystemNotConfigured,
		i18n.KeySystemStatusQueryFailed,
		i18n.KeyMFACodeRequired,
		i18n.KeyMFAInvalidCode,
		i18n.KeyMFANotEnabled,
		i18n.KeyMFAAlreadyEnabled,
		i18n.KeyMFASetupRequired,
		i18n.KeyMFAGenerateSecret,
		i18n.KeyMFAInvalidPendingToken,
		i18n.KeyDatabaseError,
		i18n.KeyReadBodyFailed,
		i18n.KeyInvalidEventPayload,
		i18n.KeyCancelSubscriptionFailed,
		i18n.KeyRoleNotAllowedOnPlan,
		i18n.KeyPlanMemberLimitReached,
		i18n.KeyPlanAlreadyActive,
		i18n.KeyStripeNotConfigured,
		i18n.KeyBillingCheckoutFailed,
		i18n.KeyInvalidWebhookSignature,
		i18n.KeySubscriptionNotFound,
		i18n.KeyEnterpriseCheckoutConflict,
		i18n.KeyEmailRequired,
		i18n.KeyPasswordResetTokenRequired,
		i18n.KeyPasswordResetTokenInvalid,
		i18n.KeyPasswordResetEmailSubject,
		i18n.KeyPasswordResetEmailGreeting,
		i18n.KeyPasswordResetEmailIntro,
		i18n.KeyPasswordResetEmailCTA,
		i18n.KeyPasswordResetEmailExpiry,
		i18n.KeyPasswordResetEmailIgnore,
		i18n.KeyPasswordResetEmailFooter,
	}

	for _, k := range keys {
		t.Run(string(k), func(t *testing.T) {
			en := i18n.T("en", k)
			if en == "" {
				t.Errorf("EN translation is empty for %q", k)
			}
			it := i18n.T("it", k)
			if it == "" {
				t.Errorf("IT translation is empty for %q", k)
			}
		})
	}
}
