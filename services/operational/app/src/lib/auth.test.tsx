import { act, render, screen } from "@testing-library/react-native";
import * as SecureStore from "expo-secure-store";
import { useEffect } from "react";
import { Text } from "react-native";

import { AuthProvider, useAuth } from "./auth";

// Probe component that records the auth state into a marker tree so tests can
// assert on the hook output without driving navigation.
function Probe({ onMount }: { onMount?: (auth: ReturnType<typeof useAuth>) => void }) {
  const auth = useAuth();
  useEffect(() => {
    onMount?.(auth);
  }, [auth, onMount]);
  return (
    <>
      <Text testID="state">
        {auth.isLoading ? "loading" : auth.token ? `signed:${auth.token}` : "anon"}
      </Text>
    </>
  );
}

describe("AuthProvider / useAuth", () => {
  beforeEach(() => {
    (SecureStore as unknown as { __reset: () => void }).__reset();
    jest.clearAllMocks();
  });

  it("hydrates the token from SecureStore on mount", async () => {
    await SecureStore.setItemAsync("cc_op_token", "persisted-token");

    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );

    // First paint: still loading.
    expect(screen.getByTestId("state").props.children).toBe("loading");

    // After microtask drains, hydration completes.
    await act(async () => {
      await Promise.resolve();
    });
    expect(screen.getByTestId("state").props.children).toBe("signed:persisted-token");
  });

  it("starts anonymous when SecureStore is empty", async () => {
    render(
      <AuthProvider>
        <Probe />
      </AuthProvider>
    );
    await act(async () => {
      await Promise.resolve();
    });
    expect(screen.getByTestId("state").props.children).toBe("anon");
  });

  it("signIn persists the token and updates state synchronously after await", async () => {
    let capturedAuth: ReturnType<typeof useAuth> | null = null;
    render(
      <AuthProvider>
        <Probe onMount={(a) => (capturedAuth = a)} />
      </AuthProvider>
    );
    await act(async () => {
      await Promise.resolve();
    });

    if (!capturedAuth) throw new Error("auth never captured");
    await act(async () => {
      await capturedAuth!.signIn("new-token");
    });

    expect(screen.getByTestId("state").props.children).toBe("signed:new-token");
    expect(await SecureStore.getItemAsync("cc_op_token")).toBe("new-token");
  });

  it("signOut clears SecureStore and state", async () => {
    await SecureStore.setItemAsync("cc_op_token", "pre-existing");
    let captured: ReturnType<typeof useAuth> | null = null;
    render(
      <AuthProvider>
        <Probe onMount={(a) => (captured = a)} />
      </AuthProvider>
    );
    await act(async () => {
      await Promise.resolve();
    });

    if (!captured) throw new Error("auth never captured");
    await act(async () => {
      await captured!.signOut();
    });

    expect(screen.getByTestId("state").props.children).toBe("anon");
    expect(await SecureStore.getItemAsync("cc_op_token")).toBeNull();
  });

  it("throws when useAuth is called outside the provider", () => {
    // Render only the probe, with no provider wrapping it. Testing-library
    // surfaces the thrown error via `act`'s error boundary; we suppress the
    // console.error noise testing-library emits on uncaught errors.
    const spy = jest.spyOn(console, "error").mockImplementation(() => {});
    try {
      expect(() => render(<Probe />)).toThrow(/inside <AuthProvider>/);
    } finally {
      spy.mockRestore();
    }
  });
});
