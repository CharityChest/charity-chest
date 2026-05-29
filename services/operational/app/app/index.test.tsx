import { act, render, screen } from "@testing-library/react-native";
import * as SecureStore from "expo-secure-store";

import { AuthProvider } from "@/lib/auth";

import Index from "./index";

function renderSplash() {
  return render(
    <AuthProvider>
      <Index />
    </AuthProvider>
  );
}

describe("<Index> (splash)", () => {
  beforeEach(() => {
    (SecureStore as unknown as { __reset: () => void }).__reset();
    jest.clearAllMocks();
  });

  it("shows a spinner while the AuthProvider hydrates", () => {
    renderSplash();
    expect(screen.UNSAFE_queryByType(
      // eslint-disable-next-line @typescript-eslint/no-var-requires
      require("react-native").ActivityIndicator
    )).not.toBeNull();
  });

  it("redirects to /login when no token is stored", async () => {
    renderSplash();
    await act(async () => {
      await Promise.resolve();
    });
    const redirect = await screen.findByTestId("redirect");
    expect(redirect.props.href).toBe("/login");
  });

  it("redirects to /(app) when a token is stored", async () => {
    await SecureStore.setItemAsync("cc_op_token", "fake.token");
    renderSplash();
    await act(async () => {
      await Promise.resolve();
    });
    const redirect = await screen.findByTestId("redirect");
    expect(redirect.props.href).toBe("/(app)");
  });
});
