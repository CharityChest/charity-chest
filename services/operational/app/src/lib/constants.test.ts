import * as babel from "@babel/core";
import { readFileSync } from "fs";
import { join } from "path";

describe("API_BASE_URL", () => {
  // NOTE: `babel-preset-expo` inlines every `process.env.EXPO_PUBLIC_*`
  // reference into a literal at *transform* time (verified below). That means
  // the env-var-preference branch in constants.ts is a build-time decision —
  // it can't be flipped by mutating `process.env` at runtime, and Jest caches
  // the single transformed copy for the whole run. So we test the two layers
  // where each branch is actually observable:
  //   • the runtime fallback (env unset at build → Constants.extra), and
  //   • the build-time inlining that makes EXPO_PUBLIC_API_URL win when set.

  it("falls back to expoConfig.extra.apiBaseUrl when the env var is unset at build time", () => {
    // EXPO_PUBLIC_API_URL is unset when Jest transforms this suite, so the
    // inlined value is `undefined` and resolution falls through to the mocked
    // expo-constants value from jest.setup.ts.
    expect(process.env.EXPO_PUBLIC_API_URL).toBeUndefined();
    jest.isolateModules(() => {
      const { API_BASE_URL } = require("./constants") as typeof import("./constants");
      expect(API_BASE_URL).toBe("http://test.local:8081");
    });
  });

  it("inlines EXPO_PUBLIC_API_URL ahead of the fallback when set at build time", () => {
    const source = readFileSync(join(__dirname, "constants.ts"), "utf8");

    const prev = process.env.EXPO_PUBLIC_API_URL;
    process.env.EXPO_PUBLIC_API_URL = "https://api.example.com";
    let code: string;
    try {
      code =
        babel.transformSync(source, {
          filename: "constants.ts",
          presets: ["babel-preset-expo"],
          configFile: false,
          babelrc: false,
        })?.code ?? "";
    } finally {
      if (prev === undefined) delete process.env.EXPO_PUBLIC_API_URL;
      else process.env.EXPO_PUBLIC_API_URL = prev;
    }

    // The env var is inlined as a literal to the LEFT of the `||`, so it wins
    // over the Constants fallback. (When unset at build time the same position
    // collapses to `undefined` and the fallback is reached — that runtime path
    // is exercised by the test above.)
    expect(code).toContain('"https://api.example.com"||');
  });
});
