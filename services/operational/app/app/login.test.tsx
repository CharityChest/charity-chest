import { act, fireEvent, render, screen, waitFor } from "@testing-library/react-native";

import * as api from "@/lib/api";
import { AuthProvider } from "@/lib/auth";
import * as google from "@/lib/google";
import { ApiError } from "@/types/api";

import LoginScreen from "./login";

// Reach into the expo-router mock for the shared router stub.
const { __router: router } = jest.requireMock("expo-router") as { __router: { replace: jest.Mock } };

// Mock api functions; tests reset their behaviour per case.
jest.mock("@/lib/api", () => ({
  __esModule: true,
  login: jest.fn(),
  googleLogin: jest.fn(),
  me: jest.fn(),
}));

// Capture the onResult callback that login.tsx passes to useGoogleSignIn so
// tests can simulate Google's response without going near the real provider.
let capturedOnResult: ((r: google.GoogleSignInResult) => void) | null = null;
const mockPrompt = jest.fn();

jest.mock("@/lib/google", () => ({
  __esModule: true,
  useGoogleSignIn: (onResult: (r: google.GoogleSignInResult) => void) => {
    capturedOnResult = onResult;
    return { prompt: mockPrompt, ready: true };
  },
}));

function renderLogin() {
  return render(
    <AuthProvider>
      <LoginScreen />
    </AuthProvider>
  );
}

async function flushAuthHydration() {
  // AuthProvider's SecureStore hydration completes after a microtask drain.
  await act(async () => {
    await Promise.resolve();
  });
}

describe("<LoginScreen>", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    capturedOnResult = null;
    router.replace.mockReset();
  });

  it("rejects empty fields without calling the API", async () => {
    renderLogin();
    await flushAuthHydration();

    fireEvent.press(screen.getByText("Sign in"));
    expect(await screen.findByText("Email and password are required")).toBeTruthy();
    expect(api.login).not.toHaveBeenCalled();
  });

  it("logs in successfully and replaces route with /(app)", async () => {
    (api.login as jest.Mock).mockResolvedValueOnce({
      token: "tk-1",
      user: { uuid: "u-1", email: "alice@example.com", name: "Alice", mfa_enabled: false },
    });

    renderLogin();
    await flushAuthHydration();

    fireEvent.changeText(screen.getByPlaceholderText("you@example.com"), "alice@example.com");
    fireEvent.changeText(screen.getByPlaceholderText("••••••••"), "password123");
    fireEvent.press(screen.getByText("Sign in"));

    await waitFor(() => expect(api.login).toHaveBeenCalledWith("alice@example.com", "password123"));
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/(app)"));
  });

  it("shows the backend error message when login fails", async () => {
    (api.login as jest.Mock).mockRejectedValueOnce(new ApiError(401, "invalid credentials"));

    renderLogin();
    await flushAuthHydration();
    fireEvent.changeText(screen.getByPlaceholderText("you@example.com"), "x@y.z");
    fireEvent.changeText(screen.getByPlaceholderText("••••••••"), "bad");
    fireEvent.press(screen.getByText("Sign in"));

    expect(await screen.findByText("invalid credentials")).toBeTruthy();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("falls back to a generic message on non-ApiError throws", async () => {
    (api.login as jest.Mock).mockRejectedValueOnce(new Error("boom"));

    renderLogin();
    await flushAuthHydration();
    fireEvent.changeText(screen.getByPlaceholderText("you@example.com"), "x@y.z");
    fireEvent.changeText(screen.getByPlaceholderText("••••••••"), "any");
    fireEvent.press(screen.getByText("Sign in"));

    expect(await screen.findByText("login failed")).toBeTruthy();
  });

  it("surfaces the MFA-not-supported 409 verbatim", async () => {
    (api.login as jest.Mock).mockRejectedValueOnce(
      new ApiError(409, "MFA-enabled accounts cannot sign in from the mobile app yet — please use the admin web app")
    );

    renderLogin();
    await flushAuthHydration();
    fireEvent.changeText(screen.getByPlaceholderText("you@example.com"), "mfa@x.y");
    fireEvent.changeText(screen.getByPlaceholderText("••••••••"), "right-password");
    fireEvent.press(screen.getByText("Sign in"));

    expect(
      await screen.findByText(/MFA-enabled accounts cannot sign in from the mobile app/)
    ).toBeTruthy();
  });

  it('"Continue with Google" calls promptAsync, then api.googleLogin on success', async () => {
    (api.googleLogin as jest.Mock).mockResolvedValueOnce({
      token: "tk-google",
      user: { uuid: "u-g", email: "g@x.y", name: "G", mfa_enabled: false },
    });

    renderLogin();
    await flushAuthHydration();

    fireEvent.press(screen.getByText("Continue with Google"));
    expect(mockPrompt).toHaveBeenCalledTimes(1);

    // Simulate Google returning successfully.
    expect(capturedOnResult).not.toBeNull();
    await act(async () => {
      capturedOnResult!({ type: "success", idToken: "id-token-xyz" });
    });

    await waitFor(() => expect(api.googleLogin).toHaveBeenCalledWith("id-token-xyz"));
    await waitFor(() => expect(router.replace).toHaveBeenCalledWith("/(app)"));
  });

  it("ignores a Google cancel without showing an error", async () => {
    renderLogin();
    await flushAuthHydration();

    fireEvent.press(screen.getByText("Continue with Google"));
    expect(capturedOnResult).not.toBeNull();

    await act(async () => {
      capturedOnResult!({ type: "cancel" });
    });

    expect(api.googleLogin).not.toHaveBeenCalled();
    expect(screen.queryByText(/failed|required/)).toBeNull();
  });

  it("shows a Google error message and does not navigate", async () => {
    renderLogin();
    await flushAuthHydration();

    fireEvent.press(screen.getByText("Continue with Google"));
    await act(async () => {
      capturedOnResult!({ type: "error", message: "consent screen failed" });
    });

    expect(await screen.findByText("consent screen failed")).toBeTruthy();
    expect(router.replace).not.toHaveBeenCalled();
  });

  it("surfaces ApiError messages from googleLogin", async () => {
    (api.googleLogin as jest.Mock).mockRejectedValueOnce(
      new ApiError(401, "could not verify Google sign-in")
    );

    renderLogin();
    await flushAuthHydration();

    fireEvent.press(screen.getByText("Continue with Google"));
    await act(async () => {
      capturedOnResult!({ type: "success", idToken: "tok" });
    });

    expect(await screen.findByText("could not verify Google sign-in")).toBeTruthy();
  });
});
