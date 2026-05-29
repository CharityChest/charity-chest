/** localStorage key used to persist the JWT across page loads. */
const TOKEN_KEY = 'cc_token';

/** Returns the stored JWT, or null if none exists or running on the server. */
export function getToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem(TOKEN_KEY);
}

/** Persists a JWT to localStorage after a successful login or OAuth callback. */
export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

/** Removes the stored JWT — call on logout or on a 401 response. */
export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY);
}

/** Returns true if a JWT is present in localStorage (does not validate the token). */
export function isAuthenticated(): boolean {
  return getToken() !== null;
}

/**
 * Reads the system-level role from the stored JWT payload without a network call.
 * Returns "root", "system", or null (for roleless / org-only users).
 * Never call this for security decisions — the server enforces actual access control.
 */
export function getRole(): string | null {
  const token = getToken();
  if (!token) return null;
  try {
    const payload = JSON.parse(atob(token.split('.')[1]));
    return typeof payload.role === 'string' ? payload.role : null;
  } catch {
    return null;
  }
}
