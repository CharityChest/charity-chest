import { render } from "@testing-library/react-native";
import * as Google from "expo-auth-session/providers/google";

import { useGoogleSignIn, type GoogleSignInResult } from "./google";

// jest.setup.ts mocks the provider; grab the typed jest.fn so each test can
// hand the hook a specific [request, response, promptAsync] triple.
const useIdTokenAuthRequest = Google.useIdTokenAuthRequest as jest.Mock;

// Probe that mounts the hook and forwards both the resolved result and the
// returned { prompt, ready } shape to the test.
function Probe({
  onResult,
  onApi,
}: {
  onResult: (r: GoogleSignInResult) => void;
  onApi?: (api: ReturnType<typeof useGoogleSignIn>) => void;
}) {
  const api = useGoogleSignIn(onResult);
  onApi?.(api);
  return null;
}

describe("useGoogleSignIn", () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  function mockProvider(response: unknown, promptAsync = jest.fn(), request: unknown = {}) {
    useIdTokenAuthRequest.mockReturnValue([request, response, promptAsync]);
  }

  it("emits success with the id_token when the provider returns one", () => {
    mockProvider({ type: "success", params: { id_token: "tok-123" } });
    const onResult = jest.fn();
    render(<Probe onResult={onResult} />);
    expect(onResult).toHaveBeenCalledWith({ type: "success", idToken: "tok-123" });
  });

  it("emits an error when a success response carries no id_token", () => {
    mockProvider({ type: "success", params: {} });
    const onResult = jest.fn();
    render(<Probe onResult={onResult} />);
    expect(onResult).toHaveBeenCalledWith({
      type: "error",
      message: "Google did not return an ID token",
    });
  });

  it("maps a cancel response to a cancel result", () => {
    mockProvider({ type: "cancel" });
    const onResult = jest.fn();
    render(<Probe onResult={onResult} />);
    expect(onResult).toHaveBeenCalledWith({ type: "cancel" });
  });

  it("maps a dismiss response to a cancel result", () => {
    mockProvider({ type: "dismiss" });
    const onResult = jest.fn();
    render(<Probe onResult={onResult} />);
    expect(onResult).toHaveBeenCalledWith({ type: "cancel" });
  });

  it("surfaces the provider error message on an error response", () => {
    mockProvider({ type: "error", error: { message: "popup blocked" } });
    const onResult = jest.fn();
    render(<Probe onResult={onResult} />);
    expect(onResult).toHaveBeenCalledWith({ type: "error", message: "popup blocked" });
  });

  it("falls back to a generic message when the error has no message", () => {
    mockProvider({ type: "error", error: undefined });
    const onResult = jest.fn();
    render(<Probe onResult={onResult} />);
    expect(onResult).toHaveBeenCalledWith({ type: "error", message: "google sign-in failed" });
  });

  it("does nothing while there is no response yet", () => {
    mockProvider(null);
    const onResult = jest.fn();
    render(<Probe onResult={onResult} />);
    expect(onResult).not.toHaveBeenCalled();
  });

  it("reports ready=false when the request has not been created", () => {
    mockProvider(null, jest.fn(), null);
    let captured: ReturnType<typeof useGoogleSignIn> | null = null;
    render(<Probe onResult={jest.fn()} onApi={(a) => (captured = a)} />);
    expect(captured!.ready).toBe(false);
  });

  it("prompt() delegates to the provider's promptAsync", () => {
    const promptAsync = jest.fn();
    mockProvider(null, promptAsync);
    let captured: ReturnType<typeof useGoogleSignIn> | null = null;
    render(<Probe onResult={jest.fn()} onApi={(a) => (captured = a)} />);

    expect(captured!.ready).toBe(true);
    captured!.prompt();
    expect(promptAsync).toHaveBeenCalledTimes(1);
  });
});
