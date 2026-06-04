import { useRouter } from "expo-router";
import { useEffect, useState } from "react";
import { ActivityIndicator, ScrollView, StyleSheet, Text, View } from "react-native";

import { Button } from "@/components/Button";
import { ErrorBanner } from "@/components/ErrorBanner";
import { me } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { ApiError, type User } from "@/types/api";

// Private area: fetches /v1/api/me on mount, renders the user's name + email,
// and offers logout. Any 401 (e.g. stale token) clears the session and bounces
// to /login.
export default function HomeScreen() {
  const router = useRouter();
  const { signOut } = useAuth();
  const [user, setUser] = useState<User | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const u = await me();
        if (!cancelled) setUser(u);
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
          await signOut();
          router.replace("/login");
          return;
        }
        if (!cancelled) setError(err instanceof ApiError ? err.message : "failed to load profile");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [router, signOut]);

  const handleLogout = async () => {
    await signOut();
    router.replace("/login");
  };

  return (
    <ScrollView contentContainerStyle={styles.scroll}>
      <View style={styles.card}>
        <Text style={styles.title}>Charity Chest</Text>
        <ErrorBanner message={error} />

        {!user && !error ? (
          <ActivityIndicator style={styles.spinner} />
        ) : null}

        {user ? (
          <>
            <Text style={styles.welcome}>Welcome, {user.name || user.email}.</Text>
            <View style={styles.row}>
              <Text style={styles.field}>Email</Text>
              <Text style={styles.value}>{user.email}</Text>
            </View>
            <View style={styles.row}>
              <Text style={styles.field}>User ID</Text>
              <Text style={styles.value}>{user.uuid}</Text>
            </View>
          </>
        ) : null}

        <Button label="Log out" variant="secondary" onPress={handleLogout} style={styles.logout} />
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  scroll: {
    flexGrow: 1,
    justifyContent: "center",
    padding: 16,
    backgroundColor: "#f5f5f4",
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
    fontSize: 22,
    fontWeight: "700",
    color: "#1a1a1a",
    marginBottom: 16,
    textAlign: "center",
  },
  spinner: {
    marginVertical: 24,
  },
  welcome: {
    fontSize: 18,
    fontWeight: "600",
    color: "#1a1a1a",
    marginBottom: 16,
  },
  row: {
    marginBottom: 12,
  },
  field: {
    fontSize: 12,
    fontWeight: "600",
    color: "#737373",
    textTransform: "uppercase",
    letterSpacing: 0.5,
  },
  value: {
    fontSize: 15,
    color: "#1a1a1a",
    marginTop: 2,
  },
  logout: {
    marginTop: 24,
  },
});
