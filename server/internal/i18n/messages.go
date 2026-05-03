package i18n

// Key is a typed string that prevents raw strings from being passed where a
// message key is expected.
type Key string

// Message key constants passed to T to look up a translated error string.
const (
	KeyInvalidBody        Key = "invalid_body"
	KeyFieldsRequired     Key = "fields_required"
	KeyPasswordTooShort   Key = "password_too_short"
	KeyEmailTaken         Key = "email_taken"
	KeyProcessPassword    Key = "process_password"
	KeyCreateUser         Key = "create_user"
	KeyGenerateToken      Key = "generate_token"
	KeyGenerateState      Key = "generate_state"
	KeyInvalidOAuthState  Key = "invalid_oauth_state"
	KeyMissingOAuthCode   Key = "missing_oauth_code"
	KeyExchangeOAuthCode  Key = "exchange_oauth_code"
	KeyFetchUserInfo      Key = "fetch_user_info"
	KeyResolveUser        Key = "resolve_user"
	KeyInvalidCredentials Key = "invalid_credentials"
	KeyGoogleOnly         Key = "google_only"
	KeyUserNotFound       Key = "user_not_found"
	KeyMissingAuthHeader  Key = "missing_auth_header"
	KeyUnexpectedSigning  Key = "unexpected_signing"
	KeyInvalidToken       Key = "invalid_token"
	KeyInvalidClaims      Key = "invalid_claims"

	KeyForbidden           Key = "forbidden"
	KeyOrgNotFound         Key = "org_not_found"
	KeyMemberExists        Key = "member_exists"
	KeyMemberNotFound      Key = "member_not_found"
	KeyInvalidRole         Key = "invalid_role"
	KeyCannotManageRole    Key = "cannot_manage_role"
	KeySystemNotConfigured      Key = "system_not_configured"
	KeySystemStatusQueryFailed Key = "system_status_query_failed"

	KeyMFACodeRequired        Key = "mfa_code_required"
	KeyMFAInvalidCode         Key = "mfa_invalid_code"
	KeyMFANotEnabled          Key = "mfa_not_enabled"
	KeyMFAAlreadyEnabled      Key = "mfa_already_enabled"
	KeyMFASetupRequired       Key = "mfa_setup_required"
	KeyMFAGenerateSecret      Key = "mfa_generate_secret"
	KeyMFAInvalidPendingToken Key = "mfa_invalid_pending_token"

	KeyRoleNotAllowedOnPlan    Key = "role_not_allowed_on_plan"
	KeyPlanMemberLimitReached  Key = "plan_member_limit_reached"
	KeyPlanAlreadyActive       Key = "plan_already_active"
	KeyStripeNotConfigured     Key = "stripe_not_configured"
	KeyBillingCheckoutFailed   Key = "billing_checkout_failed"
	KeyInvalidWebhookSignature Key = "invalid_webhook_signature"
	KeySubscriptionNotFound    Key = "subscription_not_found"
)

// messages maps locale → Key → translated string.
// Only "en" and "it" are supported; everything else falls back to "en".
var messages = map[string]map[Key]string{
	"en": {
		KeyInvalidBody:        "invalid request body",
		KeyFieldsRequired:     "email, password, and name are required",
		KeyPasswordTooShort:   "password must be at least 8 characters",
		KeyEmailTaken:         "email already registered",
		KeyProcessPassword:    "failed to process password",
		KeyCreateUser:         "failed to create user",
		KeyGenerateToken:      "failed to generate token",
		KeyGenerateState:      "failed to generate state",
		KeyInvalidOAuthState:  "invalid oauth state",
		KeyMissingOAuthCode:   "missing oauth code",
		KeyExchangeOAuthCode:  "failed to exchange oauth code",
		KeyFetchUserInfo:      "failed to fetch google user info",
		KeyResolveUser:        "failed to resolve user",
		KeyInvalidCredentials: "invalid credentials",
		KeyGoogleOnly:         "this account uses Google login",
		KeyUserNotFound:       "user not found",
		KeyMissingAuthHeader:  "missing or invalid authorization header",
		KeyUnexpectedSigning:  "unexpected signing method",
		KeyInvalidToken:       "invalid or expired token",
		KeyInvalidClaims:      "invalid token claims",

		KeyForbidden:           "forbidden",
		KeyOrgNotFound:         "organization not found",
		KeyMemberExists:        "user is already a member",
		KeyMemberNotFound:      "member not found",
		KeyInvalidRole:         "invalid role",
		KeyCannotManageRole:    "you do not have permission to assign this role",
		KeySystemNotConfigured:      "system not yet configured",
		KeySystemStatusQueryFailed: "failed to query system status",

		KeyMFACodeRequired:        "mfa code is required",
		KeyMFAInvalidCode:         "invalid mfa code",
		KeyMFANotEnabled:          "mfa is not enabled",
		KeyMFAAlreadyEnabled:      "mfa is already enabled",
		KeyMFASetupRequired:       "complete mfa setup first",
		KeyMFAGenerateSecret:      "failed to generate mfa secret",
		KeyMFAInvalidPendingToken: "invalid or expired mfa session",

		KeyRoleNotAllowedOnPlan:    "this role is not available on your current plan",
		KeyPlanMemberLimitReached:  "member limit for this role has been reached on your current plan",
		KeyPlanAlreadyActive:       "this plan is already active",
		KeyStripeNotConfigured:     "payment processing is not configured",
		KeyBillingCheckoutFailed:   "failed to create checkout session",
		KeyInvalidWebhookSignature: "invalid webhook signature",
		KeySubscriptionNotFound:    "no active subscription found",
	},
	"it": {
		KeyInvalidBody:        "corpo della richiesta non valido",
		KeyFieldsRequired:     "email, password e nome sono obbligatori",
		KeyPasswordTooShort:   "la password deve essere di almeno 8 caratteri",
		KeyEmailTaken:         "email già registrata",
		KeyProcessPassword:    "errore nell'elaborazione della password",
		KeyCreateUser:         "errore nella creazione dell'utente",
		KeyGenerateToken:      "errore nella generazione del token",
		KeyGenerateState:      "errore nella generazione dello stato",
		KeyInvalidOAuthState:  "stato oauth non valido",
		KeyMissingOAuthCode:   "codice oauth mancante",
		KeyExchangeOAuthCode:  "errore nello scambio del codice oauth",
		KeyFetchUserInfo:      "errore nel recupero delle informazioni utente da Google",
		KeyResolveUser:        "errore nella risoluzione dell'utente",
		KeyInvalidCredentials: "Credenziali non valide",
		KeyGoogleOnly:         "questo account utilizza l'accesso con Google",
		KeyUserNotFound:       "utente non trovato",
		KeyMissingAuthHeader:  "intestazione di autorizzazione mancante o non valida",
		KeyUnexpectedSigning:  "metodo di firma non previsto",
		KeyInvalidToken:       "token non valido o scaduto",
		KeyInvalidClaims:      "claim del token non validi",

		KeyForbidden:           "accesso negato",
		KeyOrgNotFound:         "organizzazione non trovata",
		KeyMemberExists:        "l'utente è già membro",
		KeyMemberNotFound:      "membro non trovato",
		KeyInvalidRole:         "ruolo non valido",
		KeyCannotManageRole:    "non hai i permessi per assegnare questo ruolo",
		KeySystemNotConfigured:      "sistema non ancora configurato",
		KeySystemStatusQueryFailed: "errore nella verifica dello stato del sistema",

		KeyMFACodeRequired:        "il codice mfa è obbligatorio",
		KeyMFAInvalidCode:         "codice mfa non valido",
		KeyMFANotEnabled:          "mfa non è abilitato",
		KeyMFAAlreadyEnabled:      "mfa è già abilitato",
		KeyMFASetupRequired:       "completa prima la configurazione mfa",
		KeyMFAGenerateSecret:      "errore nella generazione del segreto mfa",
		KeyMFAInvalidPendingToken: "sessione mfa non valida o scaduta",

		KeyRoleNotAllowedOnPlan:    "questo ruolo non è disponibile nel piano attuale",
		KeyPlanMemberLimitReached:  "il limite di membri per questo ruolo è stato raggiunto nel piano attuale",
		KeyPlanAlreadyActive:       "questo piano è già attivo",
		KeyStripeNotConfigured:     "il sistema di pagamento non è configurato",
		KeyBillingCheckoutFailed:   "errore nella creazione della sessione di pagamento",
		KeyInvalidWebhookSignature: "firma del webhook non valida",
		KeySubscriptionNotFound:    "nessun abbonamento attivo trovato",
	},
}

// T returns the translated string for the given locale and key.
// Unknown locales fall back to "en"; missing keys fall back to the key string itself.
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
