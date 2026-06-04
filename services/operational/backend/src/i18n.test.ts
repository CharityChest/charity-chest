import { describe, expect, it } from "vitest";

import { LocaleEN, LocaleIT, MessageKey, parseLocale, t } from "./i18n";

const KEYS = Object.values(MessageKey);

describe("t", () => {
  it("returns English and Italian strings", () => {
    expect(t("en", MessageKey.InvalidCredentials)).toBe("invalid credentials");
    expect(t("it", MessageKey.InvalidCredentials)).toBe("Credenziali non valide");
  });

  it("falls back to English for an unknown locale", () => {
    expect(t("fr", MessageKey.UserNotFound)).toBe(t("en", MessageKey.UserNotFound));
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
