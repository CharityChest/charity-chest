import { API_BASE_URL, resolveApiBaseUrl } from "./constants";

describe("API_BASE_URL", () => {
  // `babel-preset-expo` inlines `process.env.EXPO_PUBLIC_*` into literals at
  // *transform* time, so the module-level `API_BASE_URL` is a build-time value
  // that can't be flipped by mutating `process.env` at runtime. We assert the
  // resolved value here and exercise both guard branches via `resolveApiBaseUrl`.

  it("resolves to the build-time EXPO_PUBLIC_API_URL", () => {
    // jest.config.js sets EXPO_PUBLIC_API_URL before the transform pipeline runs.
    expect(process.env.EXPO_PUBLIC_API_URL).toBe("http://test.local:8081");
    expect(API_BASE_URL).toBe("http://test.local:8081");
  });

  it("returns the value when EXPO_PUBLIC_API_URL is set", () => {
    expect(resolveApiBaseUrl("https://api.example.com")).toBe("https://api.example.com");
  });

  it.each([undefined, ""])("throws a clear error when the value is %p", (value) => {
    expect(() => resolveApiBaseUrl(value)).toThrow(/EXPO_PUBLIC_API_URL/);
  });
});
