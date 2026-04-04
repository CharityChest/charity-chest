import { API_BASE_URL } from './constants';
import { getToken } from './auth';
import type { AuthResponse, User } from '@/types/api';

// Carries the HTTP status code so callers can branch on it (e.g. 401 → redirect to login).
export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

/**
 * Reads the current locale from the URL path prefix (/en/... or /it/...).
 * Falls back to "en" in non-browser environments (SSR) or unrecognised paths.
 */
function getLocale(): string {
  if (typeof window === 'undefined') return 'en';
  const [, segment] = window.location.pathname.split('/');
  return segment === 'it' ? 'it' : 'en';
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      'Accept-Language': getLocale(),
      ...(options.headers as Record<string, string>),
    },
    ...options,
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ message: res.statusText }));
    throw new ApiError(res.status, body.message ?? 'Request failed');
  }

  return res.json() as Promise<T>;
}

function bearerHeader(): Record<string, string> {
  const token = getToken();
  return token ? { Authorization: `Bearer ${token}` } : {};
}

export const api = {
  /** POST /v1/auth/register */
  register(email: string, password: string, name: string): Promise<AuthResponse> {
    return request('/v1/auth/register', {
      method: 'POST',
      body: JSON.stringify({ email, password, name }),
    });
  },

  /** POST /v1/auth/login */
  login(email: string, password: string): Promise<AuthResponse> {
    return request('/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
  },

  /**
   * Returns the full URL to redirect the browser to for Google OAuth.
   * The server handles the consent screen and callback entirely; no fetch needed.
   */
  googleAuthUrl(): string {
    return `${API_BASE_URL}/v1/auth/google`;
  },

  /** GET /v1/api/me  — requires a valid JWT in localStorage */
  me(): Promise<User> {
    return request('/v1/api/me', { headers: bearerHeader() });
  },

  /** GET /health */
  health(): Promise<{ status: string }> {
    return request('/health');
  },
};
