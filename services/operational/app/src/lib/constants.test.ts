describe("API_BASE_URL", () => {
  // constants.ts caches the resolved URL at module load. We use jest.isolateModules
  // to get a fresh evaluation per test so env-var changes actually take effect.
  beforeEach(() => {
    delete process.env.EXPO_PUBLIC_API_URL;
  });

  it("falls back to expoConfig.extra.apiBaseUrl when env is unset", () => {
    jest.isolateModules(() => {
      const { API_BASE_URL } = require("./constants") as typeof import("./constants");
      // jest.setup.ts mocks expo-constants → "http://test.local:8081".
      expect(API_BASE_URL).toBe("http://test.local:8081");
    });
  });

  it("prefers EXPO_PUBLIC_API_URL when set", () => {
    process.env.EXPO_PUBLIC_API_URL = "https://api.example.com";
    jest.isolateModules(() => {
      const { API_BASE_URL } = require("./constants") as typeof import("./constants");
      expect(API_BASE_URL).toBe("https://api.example.com");
    });
  });
});
