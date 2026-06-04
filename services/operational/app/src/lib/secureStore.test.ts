import * as SecureStore from "expo-secure-store";

import { clearToken, getToken, setToken } from "./secureStore";

describe("secureStore wrapper", () => {
  beforeEach(() => {
    // The in-memory mock exposes a __reset helper for hermetic tests.
    (SecureStore as unknown as { __reset: () => void }).__reset();
    jest.clearAllMocks();
  });

  it("getToken returns null when nothing is stored", async () => {
    expect(await getToken()).toBeNull();
  });

  it("setToken persists; getToken returns the value", async () => {
    await setToken("abc.def.ghi");
    expect(await getToken()).toBe("abc.def.ghi");
    expect(SecureStore.setItemAsync).toHaveBeenCalledTimes(1);
  });

  it("clearToken removes the value", async () => {
    await setToken("xyz");
    await clearToken();
    expect(await getToken()).toBeNull();
    expect(SecureStore.deleteItemAsync).toHaveBeenCalledTimes(1);
  });

  it("uses a stable storage key for round-trips", async () => {
    await setToken("token-1");
    const calls = (SecureStore.setItemAsync as jest.Mock).mock.calls;
    const [key] = calls[0] as [string, string];
    await clearToken();
    const delKey = ((SecureStore.deleteItemAsync as jest.Mock).mock.calls[0] as [string])[0];
    expect(delKey).toBe(key);
  });
});
