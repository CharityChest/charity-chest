import { Redirect, Stack } from "expo-router";

import { useAuth } from "@/lib/auth";

// Auth gate. Any route under app/(app)/ requires a token; otherwise we
// redirect to /login. The /index splash already does this on cold start,
// but this layout also covers warm starts (e.g. someone deep-links here
// without going through the splash).
export default function AppLayout() {
  const { token, isLoading } = useAuth();

  if (isLoading) return null;
  if (!token) return <Redirect href="/login" />;

  return <Stack screenOptions={{ headerShown: false }} />;
}
