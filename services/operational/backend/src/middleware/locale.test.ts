import express from "express";
import request from "supertest";
import { describe, expect, it } from "vitest";

import { locale } from "./locale";

function appWithLocale() {
  const app = express();
  app.use(locale());
  app.get("/probe", (_req, res) => {
    res.json({ locale: res.locals.locale });
  });
  return app;
}

describe("locale middleware", () => {
  it("defaults to English with no header", async () => {
    const res = await request(appWithLocale()).get("/probe");
    expect(res.body.locale).toBe("en");
  });

  it("resolves X-Locale: it to Italian", async () => {
    const res = await request(appWithLocale()).get("/probe").set("X-Locale", "it");
    expect(res.body.locale).toBe("it");
  });

  it("ignores Accept-Language (only X-Locale is consulted)", async () => {
    const res = await request(appWithLocale())
      .get("/probe")
      .set("Accept-Language", "it-IT,it;q=0.9");
    expect(res.body.locale).toBe("en");
  });
});
