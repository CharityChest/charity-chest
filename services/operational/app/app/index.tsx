import { Redirect } from "expo-router";
import { ActivityIndicator, StyleSheet, View } from "react-native";

import { useAuth } from "@/lib/auth";

// Splash: read the persisted token, then redirect into the app or login.
// The AuthProvider does the SecureStore read; this screen shows a spinner
// while that runs to avoid a flash of the login form on warm starts.
export default function Index() {
  const { token, isLoading } = useAuth();

  if (isLoading) {
    return (
      <View style={styles.center}>
        <ActivityIndicator />
      </View>
    );
  }
  return <Redirect href={token ? "/(app)" : "/login"} />;
}

const styles = StyleSheet.create({
  center: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
  },
});
