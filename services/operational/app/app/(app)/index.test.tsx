import { act, fireEvent, render, screen, waitFor } from "@testing-library/react-native";
import * as SecureStore from "expo-secure-store";

import * as api from "@/lib/api";
import { AuthProvider } from "@/lib/auth";
import { ApiError } from "@/types/api";

import HomeScreen from "./index";

const { __router: router } = jest.requireMock("expo-router") as { __router: { replace: jest.Mock } };

jest.mock("@/lib/api", () => ({
  __esModule: true,
  login: jest.fn(),
  googleLogin: jest.fn(),
  me: jest.fn(),
}));

function renderHome() {
  return render(
    <AuthProvider>
      <HomeScreen />
    </AuthProvider>
  );
}

async function flushAsync() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe("<HomeScreen> (private area)", () => {
  beforeEach(async () => {
    jest.clearAllMocks();
    (SecureStore as unknown as { __reset: () => void }).__reset();
    router.replace.mockReset();
    // Seed a token so AuthProvider hydrates as "signed in" — HomeScreen is
    // gated on this in production via app/(app)/_layout.tsx, but the screen
    // doesn't assume it itself, so we set one for parity with reality.
    await SecureStore.setItemAsync("cc_op_token", "fake.jwt");
  });

  it("loads the user via /me and renders the welcome line", async () => {
    (api.me as jest.Mock).mockResolvedValueOnce({
      uuid: "u-1",
      email: "alice@example.com",
      name: "Alice",
      mfa_enabled: false,
    });

    renderHome();

    expect(await screen.findByText("Welcome, Alice.")).toBeTruthy();
    expect(screen.getByText("alice@example.com")).toBeTruthy();
    expect(screen.getByText("u-1")).toBeTruthy();
  });

  it("falls back to email when name is empty", async () => {
    (api.me as jest.Mock).mockResolvedValueOnce({
      uuid: "u-2",
      email: "noname@example.com",
      name: "",
      mfa_enabled: false,
    });

    renderHome();
    expect(await screen.findByText("Welcome, noname@example.com.")).toBeTruthy();
  });

  it("clears the session and redirects on 401 from /me", async () => {
    (api.me as jest.Mock).mockRejectedValueOnce(new ApiError(401, "invalid or expired token"));

    renderHome();

    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/login"));
    await waitFor(async () => {
      expect(await SecureStore.getItemAsync("cc_op_token")).toBeNull();
    });
  });

  it("shows the backend error message on non-401 failures and does not redirect", async () => {
    (api.me as jest.Mock).mockRejectedValueOnce(new ApiError(502, "identity service unavailable"));

    renderHome();

    expect(await screen.findByText("identity service unavailable")).toBeTruthy();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("logout button clears the session and redirects to /login", async () => {
    (api.me as jest.Mock).mockResolvedValueOnce({
      uuid: "u-3",
      email: "bob@example.com",
      name: "Bob",
      mfa_enabled: false,
    });

    renderHome();
    await screen.findByText("Welcome, Bob.");

    fireEvent.press(screen.getByText("Log out"));
    await flushAsync();

    await waitFor(async () => {
      expect(await SecureStore.getItemAsync("cc_op_token")).toBeNull();
    });
    expect(router.replace).toHaveBeenCalledWith("/login");
  });
});
