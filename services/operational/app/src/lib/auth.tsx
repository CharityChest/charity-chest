import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";

import { clearToken, getToken, setToken } from "./secureStore";

type AuthContextValue = {
  token: string | null;
  isLoading: boolean;
  signIn: (token: string) => Promise<void>;
  signOut: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

// AuthProvider hydrates the persisted token from SecureStore on mount and
// exposes signIn/signOut helpers to every screen. The mobile equivalent of
// admin webapp's localStorage-based session.
export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setTokenState] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const t = await getToken();
        if (!cancelled) {
          setTokenState(t);
        }
      } catch (err) {
        console.warn("Failed to hydrate persisted token", err);
        if (!cancelled) {
          setTokenState(null);
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const signIn = useCallback(async (newToken: string) => {
    await setToken(newToken);
    setTokenState(newToken);
  }, []);

  const signOut = useCallback(async () => {
    try {
      await clearToken();
    } catch (err) {
      // A storage failure must never strand the user in a logged-in state;
      // log it and still clear the in-memory token below.
      console.warn("Failed to clear persisted token during sign out", err);
    } finally {
      setTokenState(null);
    }
  }, []);

  return (
    <AuthContext.Provider value={{ token, isLoading, signIn, signOut }}>{children}</AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be called inside <AuthProvider>");
  }
  return ctx;
}
