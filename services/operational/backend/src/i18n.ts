// Typed message translation for the operational backend. Mirrors the Go
// internal/i18n package: the same keys and the same EN/IT strings, so error UX
// stays identical across the rewrite. Only "en" and "it" are supported;
// unknown locales fall back to English and a missing key falls back to its own
// identifier.

export type Locale = "en" | "it";

export const LocaleEN: Locale = "en";
export const LocaleIT: Locale = "it";

/**
 * Enum-like message keys. Call sites reference `MessageKey.InvalidBody` rather
 * than the bare string, so typos are compile errors and every translatable
 * message is discoverable from one place. Exported both as the value map and
 * as the union type of its values.
 */
export const MessageKey = {
  InvalidBody: "invalidBody",
  FieldsRequired: "fieldsRequired",
  InvalidCredentials: "invalidCredentials",
  GoogleOnly: "googleOnly",
  MissingAuthHeader: "missingAuthHeader",
  UnexpectedSigning: "unexpectedSigning",
  InvalidToken: "invalidToken",
  InvalidClaims: "invalidClaims",
  GenerateToken: "generateToken",
  AdminUnavailable: "adminUnavailable",
  MfaNotSupported: "mfaNotSupported",
  GoogleVerifyFailed: "googleVerifyFailed",
  UserNotFound: "userNotFound",
  ServerError: "serverError",
} as const;

export type MessageKey = (typeof MessageKey)[keyof typeof MessageKey];

const messages: Record<Locale, Record<MessageKey, string>> = {
  en: {
    [MessageKey.InvalidBody]: "invalid request body",
    [MessageKey.FieldsRequired]: "required fields are missing",
    [MessageKey.InvalidCredentials]: "invalid credentials",
    [MessageKey.GoogleOnly]: "this account uses Google login",
    [MessageKey.MissingAuthHeader]: "missing or invalid authorization header",
    [MessageKey.UnexpectedSigning]: "unexpected signing method",
    [MessageKey.InvalidToken]: "invalid or expired token",
    [MessageKey.InvalidClaims]: "invalid token claims",
    [MessageKey.GenerateToken]: "failed to generate token",
    [MessageKey.AdminUnavailable]: "the identity service is temporarily unavailable",
    [MessageKey.MfaNotSupported]:
      "MFA-enabled accounts cannot sign in from the mobile app yet — please use the admin web app",
    [MessageKey.GoogleVerifyFailed]: "could not verify Google sign-in",
    [MessageKey.UserNotFound]: "user not found",
    [MessageKey.ServerError]: "internal server error",
  },
  it: {
    [MessageKey.InvalidBody]: "corpo della richiesta non valido",
    [MessageKey.FieldsRequired]: "campi obbligatori mancanti",
    [MessageKey.InvalidCredentials]: "Credenziali non valide",
    [MessageKey.GoogleOnly]: "questo account utilizza l'accesso con Google",
    [MessageKey.MissingAuthHeader]: "intestazione di autorizzazione mancante o non valida",
    [MessageKey.UnexpectedSigning]: "metodo di firma non previsto",
    [MessageKey.InvalidToken]: "token non valido o scaduto",
    [MessageKey.InvalidClaims]: "claim del token non validi",
    [MessageKey.GenerateToken]: "errore nella generazione del token",
    [MessageKey.AdminUnavailable]: "il servizio identità è temporaneamente non disponibile",
    [MessageKey.MfaNotSupported]:
      "gli account con MFA non possono ancora accedere dall'app mobile — usa l'app web di amministrazione",
    [MessageKey.GoogleVerifyFailed]: "impossibile verificare l'accesso con Google",
    [MessageKey.UserNotFound]: "utente non trovato",
    [MessageKey.ServerError]: "errore interno del server",
  },
};

/**
 * Returns the translated string for `locale` and `key`. Unknown locales fall
 * back to English; a missing key falls back to the key identifier.
 */
export function t(locale: string, key: MessageKey): string {
  const loc: Locale = locale === LocaleEN || locale === LocaleIT ? locale : LocaleEN;
  const table = messages[loc];
  return table[key] ?? key;
}

/**
 * Resolves an X-Locale header value to a supported locale. Mirrors the Go
 * middleware's detectLocale: only an exact (trimmed, case-insensitive) "it"
 * selects Italian; everything else — including "it-IT" — falls back to English.
 */
export function parseLocale(value: string): Locale {
  if (value.trim().toLowerCase() === LocaleIT) {
    return LocaleIT;
  }
  return LocaleEN;
}
