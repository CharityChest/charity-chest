import { useRouter } from "expo-router";
import { useCallback, useState } from "react";
import { KeyboardAvoidingView, Platform, ScrollView, StyleSheet, Text, View } from "react-native";

import { Button } from "@/components/Button";
import { ErrorBanner } from "@/components/ErrorBanner";
import { TextField } from "@/components/TextField";
import { googleLogin, login } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { useGoogleSignIn, type GoogleSignInResult } from "@/lib/google";
import { ApiError } from "@/types/api";

export default function LoginScreen() {
  const router = useRouter();
  const { signIn } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const onGoogleResult = useCallback(
    async (r: GoogleSignInResult) => {
      if (r.type === "cancel") return;
      if (r.type === "error") {
        setError(r.message);
        return;
      }
      setSubmitting(true);
      setError(null);
      try {
        const res = await googleLogin(r.idToken);
        await signIn(res.token);
        router.replace("/(app)");
      } catch (err) {
        setError(err instanceof ApiError ? err.message : "google sign-in failed");
      } finally {
        setSubmitting(false);
      }
    },
    [router, signIn]
  );

  const google = useGoogleSignIn(onGoogleResult);

  const onSubmit = useCallback(async () => {
    if (!email || !password) {
      setError("Email and password are required");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const res = await login(email, password);
      await signIn(res.token);
      router.replace("/(app)");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "login failed");
    } finally {
      setSubmitting(false);
    }
  }, [email, password, router, signIn]);

  return (
    <KeyboardAvoidingView
      style={styles.flex}
      behavior={Platform.OS === "ios" ? "padding" : undefined}
    >
      <ScrollView contentContainerStyle={styles.scroll} keyboardShouldPersistTaps="handled">
        <View style={styles.card}>
          <Text style={styles.title}>Charity Chest</Text>
          <Text style={styles.subtitle}>Sign in to continue</Text>

          <ErrorBanner message={error} />

          <TextField
            label="Email"
            value={email}
            onChangeText={setEmail}
            keyboardType="email-address"
            autoComplete="email"
            placeholder="you@example.com"
            editable={!submitting}
          />
          <TextField
            label="Password"
            value={password}
            onChangeText={setPassword}
            secureTextEntry
            autoComplete="password"
            placeholder="••••••••"
            editable={!submitting}
          />

          <Button label="Sign in" onPress={onSubmit} loading={submitting} style={styles.primaryButton} />

          <View style={styles.divider}>
            <View style={styles.dividerLine} />
            <Text style={styles.dividerText}>or</Text>
            <View style={styles.dividerLine} />
          </View>

          <Button
            label="Continue with Google"
            variant="secondary"
            onPress={() => void google.prompt()}
            disabled={!google.ready || submitting}
          />
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1, backgroundColor: "#f5f5f4" },
  scroll: {
    flexGrow: 1,
    justifyContent: "center",
    padding: 16,
  },
  card: {
    backgroundColor: "#ffffff",
    borderRadius: 12,
    padding: 24,
    shadowColor: "#000",
    shadowOpacity: 0.05,
    shadowRadius: 8,
    elevation: 2,
  },
  title: {
    fontSize: 24,
    fontWeight: "700",
    color: "#1a1a1a",
    textAlign: "center",
  },
  subtitle: {
    fontSize: 14,
    color: "#737373",
    textAlign: "center",
    marginTop: 4,
    marginBottom: 24,
  },
  primaryButton: {
    marginTop: 4,
  },
  divider: {
    flexDirection: "row",
    alignItems: "center",
    marginVertical: 16,
  },
  dividerLine: {
    flex: 1,
    height: 1,
    backgroundColor: "#e5e5e4",
  },
  dividerText: {
    marginHorizontal: 8,
    color: "#737373",
    fontSize: 13,
  },
});
