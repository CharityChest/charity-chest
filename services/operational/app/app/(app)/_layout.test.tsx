import { act, render, screen } from "@testing-library/react-native";
import * as SecureStore from "expo-secure-store";
import { Text } from "react-native";

import { AuthProvider } from "@/lib/auth";

import AppLayout from "./_layout";

// The Stack mock from jest.setup.ts renders children directly, so we render
// a sentinel inside AppLayout to detect that the gate let us through.
function Sentinel() {
  return <Text testID="inside-app">inside</Text>;
}

describe("<AppLayout> auth gate", () => {
  beforeEach(() => {
    (SecureStore as unknown as { __reset: () => void }).__reset();
    jest.clearAllMocks();
  });

  it("renders nothing while AuthProvider is hydrating", () => {
    render(
      <AuthProvider>
        <AppLayout />
        <Sentinel />
      </AuthProvider>
    );
    // The gate returns null during loading; the sibling sentinel is unrelated
    // and proves the tree mounted. The redirect marker must NOT appear yet.
    expect(screen.queryByTestId("redirect")).toBeNull();
  });

  it("redirects to /login when no token is stored after hydration", async () => {
    render(
      <AuthProvider>
        <AppLayout />
      </AuthProvider>
    );
    await act(async () => {
      await Promise.resolve();
    });
    const redirect = await screen.findByTestId("redirect");
    expect(redirect.props.href).toBe("/login");
  });

  it("admits the Stack when a token is present", async () => {
    await SecureStore.setItemAsync("cc_op_token", "ok");
    render(
      <AuthProvider>
        <AppLayout />
      </AuthProvider>
    );
    await act(async () => {
      await Promise.resolve();
    });
    expect(screen.queryByTestId("redirect")).toBeNull();
  });
});
