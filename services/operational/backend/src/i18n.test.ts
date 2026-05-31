import { describe, expect, it } from "vitest";

import { LocaleEN, LocaleIT, parseLocale, t, type MessageKey } from "./i18n";

const KEYS: MessageKey[] = [
  "invalidBody",
  "fieldsRequired",
  "invalidCredentials",
  "googleOnly",
  "missingAuthHeader",
  "unexpectedSigning",
  "invalidToken",
  "invalidClaims",
  "generateToken",
  "adminUnavailable",
  "mfaNotSupported",
  "googleVerifyFailed",
  "userNotFound",
];

describe("t", () => {
  it("returns English and Italian strings", () => {
    expect(t("en", "invalidCredentials")).toBe("invalid credentials");
    expect(t("it", "invalidCredentials")).toBe("Credenziali non valide");
  });

  it("falls back to English for an unknown locale", () => {
    expect(t("fr", "userNotFound")).toBe(t("en", "userNotFound"));
  });

  it("translates every key in both locales (no fallthrough to the identifier)", () => {
    for (const key of KEYS) {
      expect(t("en", key)).not.toBe(key);
      expect(t("it", key)).not.toBe(key);
    }
  });
});

describe("parseLocale", () => {
  it("matches an exact 'it' case-insensitively", () => {
    expect(parseLocale("it")).toBe(LocaleIT);
    expect(parseLocale("IT")).toBe(LocaleIT);
    expect(parseLocale("  it  ")).toBe(LocaleIT);
  });

  it("falls back to English for anything else", () => {
    expect(parseLocale("en")).toBe(LocaleEN);
    expect(parseLocale("it-IT")).toBe(LocaleEN);
    expect(parseLocale("")).toBe(LocaleEN);
    expect(parseLocale("fr")).toBe(LocaleEN);
  });
});
