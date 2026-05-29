import * as Google from "expo-auth-session/providers/google";
import * as WebBrowser from "expo-web-browser";
import { useEffect } from "react";

// WebBrowser.maybeCompleteAuthSession() must run once at module load on web /
// when returning from the in-app browser. Safe to call multiple times.
WebBrowser.maybeCompleteAuthSession();

export type GoogleSignInResult =
  | { type: "success"; idToken: string }
  | { type: "cancel" }
  | { type: "error"; message: string };

// useGoogleSignIn wraps expo-auth-session's Google provider in the shape the
// login screen wants: a `prompt()` to kick off the consent flow and an `onResult`
// callback for the resolved outcome. Each platform uses its own OAuth client ID;
// the resulting ID token is verified server-side against EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID
// (which the operational backend also uses as GOOGLE_AUDIENCE).
export function useGoogleSignIn(onResult: (r: GoogleSignInResult) => void) {
  const [request, response, promptAsync] = Google.useIdTokenAuthRequest({
    iosClientId: process.env.EXPO_PUBLIC_GOOGLE_IOS_CLIENT_ID,
    androidClientId: process.env.EXPO_PUBLIC_GOOGLE_ANDROID_CLIENT_ID,
    webClientId: process.env.EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID,
  });

  useEffect(() => {
    if (!response) return;
    if (response.type === "success") {
      const idToken = response.params?.id_token;
      if (idToken) {
        onResult({ type: "success", idToken });
      } else {
        onResult({ type: "error", message: "Google did not return an ID token" });
      }
    } else if (response.type === "cancel" || response.type === "dismiss") {
      onResult({ type: "cancel" });
    } else if (response.type === "error") {
      onResult({ type: "error", message: response.error?.message ?? "google sign-in failed" });
    }
  }, [response, onResult]);

  return {
    prompt: () => promptAsync(),
    ready: request !== null,
  };
}
