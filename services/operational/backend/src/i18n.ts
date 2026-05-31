// Typed message translation for the operational backend. Mirrors the Go
// internal/i18n package: the same keys and the same EN/IT strings, so error UX
// stays identical across the rewrite. Only "en" and "it" are supported;
// unknown locales fall back to English and a missing key falls back to its own
// identifier.

export type Locale = "en" | "it";

export const LocaleEN: Locale = "en";
export const LocaleIT: Locale = "it";

export type MessageKey =
  | "invalidBody"
  | "fieldsRequired"
  | "invalidCredentials"
  | "googleOnly"
  | "missingAuthHeader"
  | "unexpectedSigning"
  | "invalidToken"
  | "invalidClaims"
  | "generateToken"
  | "adminUnavailable"
  | "mfaNotSupported"
  | "googleVerifyFailed"
  | "userNotFound";

const messages: Record<Locale, Record<MessageKey, string>> = {
  en: {
    invalidBody: "invalid request body",
    fieldsRequired: "required fields are missing",
    invalidCredentials: "invalid credentials",
    googleOnly: "this account uses Google login",
    missingAuthHeader: "missing or invalid authorization header",
    unexpectedSigning: "unexpected signing method",
    invalidToken: "invalid or expired token",
    invalidClaims: "invalid token claims",
    generateToken: "failed to generate token",
    adminUnavailable: "the identity service is temporarily unavailable",
    mfaNotSupported:
      "MFA-enabled accounts cannot sign in from the mobile app yet — please use the admin web app",
    googleVerifyFailed: "could not verify Google sign-in",
    userNotFound: "user not found",
  },
  it: {
    invalidBody: "corpo della richiesta non valido",
    fieldsRequired: "campi obbligatori mancanti",
    invalidCredentials: "Credenziali non valide",
    googleOnly: "questo account utilizza l'accesso con Google",
    missingAuthHeader: "intestazione di autorizzazione mancante o non valida",
    unexpectedSigning: "metodo di firma non previsto",
    invalidToken: "token non valido o scaduto",
    invalidClaims: "claim del token non validi",
    generateToken: "errore nella generazione del token",
    adminUnavailable: "il servizio identità è temporaneamente non disponibile",
    mfaNotSupported:
      "gli account con MFA non possono ancora accedere dall'app mobile — usa l'app web di amministrazione",
    googleVerifyFailed: "impossibile verificare l'accesso con Google",
    userNotFound: "utente non trovato",
  },
};

/**
 * Returns the translated string for `locale` and `key`. Unknown locales fall
 * back to English; a missing key falls back to the key identifier.
 */
export function t(locale: string, key: MessageKey): string {
  const loc: Locale = locale === "en" || locale === "it" ? locale : "en";
  const table = messages[loc];
  return table[key] ?? key;
}

/**
 * Resolves an X-Locale header value to a supported locale. Mirrors the Go
 * middleware's detectLocale: only an exact (trimmed, case-insensitive) "it"
 * selects Italian; everything else — including "it-IT" — falls back to English.
 */
export function parseLocale(value: string): Locale {
  if (value.trim().toLowerCase() === "it") {
    return LocaleIT;
  }
  return LocaleEN;
}
