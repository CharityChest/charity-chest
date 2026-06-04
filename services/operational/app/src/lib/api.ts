import { NativeModules, Platform } from "react-native";

import { API_BASE_URL } from "./constants";
import { getToken } from "./secureStore";
import { ApiError, type ApiEnvelope, type LoginResponse, type User } from "../types/api";

// Detect device locale so the backend can return localised error messages.
// Falls back to "en" when the platform can't tell us.
function deviceLocale(): string {
  const lang =
    Platform.OS === "ios"
      ? NativeModules.SettingsManager?.settings?.AppleLocale ??
        NativeModules.SettingsManager?.settings?.AppleLanguages?.[0]
      : NativeModules.I18nManager?.localeIdentifier;
  if (typeof lang === "string" && lang.toLowerCase().startsWith("it")) return "it";
  return "en";
}

type RequestOptions = {
  method: "GET" | "POST";
  path: string;
  body?: unknown;
  authToken?: string | null;
};

async function request<T>(opts: RequestOptions): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "X-Locale": deviceLocale(),
  };
  if (opts.authToken) {
    headers["Authorization"] = `Bearer ${opts.authToken}`;
  }

  let res: Response;
  try {
    res = await fetch(`${API_BASE_URL}${opts.path}`, {
      method: opts.method,
      headers,
      body: opts.body ? JSON.stringify(opts.body) : undefined,
    });
  } catch (err) {
    // Network failure — surfaces in the UI as a 0-status ApiError.
    throw new ApiError(0, err instanceof Error ? err.message : "network error");
  }

  let parsed: unknown;
  const text = await res.text();
  if (text) {
    try {
      parsed = JSON.parse(text);
    } catch {
      // non-JSON body — keep as string for the error path
      parsed = text;
    }
  }

  if (!res.ok) {
    const message =
      typeof parsed === "object" && parsed && "message" in parsed
        ? String((parsed as { message: unknown }).message)
        : res.statusText || "request failed";
    throw new ApiError(res.status, message);
  }

  // Successful responses are always wrapped as {"data": T}.
  return (parsed as ApiEnvelope<T>).data;
}

export async function login(email: string, password: string): Promise<LoginResponse> {
  return request<LoginResponse>({
    method: "POST",
    path: "/v1/auth/login",
    body: { email, password },
  });
}

export async function googleLogin(idToken: string): Promise<LoginResponse> {
  return request<LoginResponse>({
    method: "POST",
    path: "/v1/auth/google",
    body: { id_token: idToken },
  });
}

export async function me(): Promise<User> {
  const token = await getToken();
  return request<User>({
    method: "GET",
    path: "/v1/api/me",
    authToken: token,
  });
}
