import { API_BASE_URL } from './constants';
import { getToken } from './auth';
import type {
  AuthResponse,
  LoginResponse,
  MFASetupResponse,
  MFAStatusResponse,
  User,
  SystemStatus,
  Organization,
  OrganizationMember,
  PaginatedResult,
  UserWithOrgs,
} from '@/types/api';

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
 * Exported for testing.
 */
export function getLocale(): string {
  if (typeof window === 'undefined') return 'en';
  const [, segment] = window.location.pathname.split('/');
  return segment === 'it' ? 'it' : 'en';
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      'X-Locale': getLocale(),
      ...(options.headers as Record<string, string>),
    },
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ message: res.statusText }));
    throw new ApiError(res.status, body.message ?? 'Request failed');
  }

  if (res.status === 204) {
    return undefined as T;
  }

  const body = await res.json() as { data: T };
  return body.data;
}

async function requestPaginated<T>(path: string, options: RequestInit = {}): Promise<PaginatedResult<T>> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      'X-Locale': getLocale(),
      ...(options.headers as Record<string, string>),
    },
  });

  if (!res.ok) {
    const body = await res.json().catch(() => ({ message: res.statusText }));
    throw new ApiError(res.status, body.message ?? 'Request failed');
  }

  return await res.json() as PaginatedResult<T>;
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

  /** POST /v1/auth/login — returns a full token or an MFA challenge */
  login(email: string, password: string): Promise<LoginResponse> {
    return request('/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
  },

  /** POST /v1/auth/mfa/verify — submit TOTP code to complete login */
  mfaVerify(mfaToken: string, code: string): Promise<LoginResponse> {
    return request('/v1/auth/mfa/verify', {
      method: 'POST',
      body: JSON.stringify({ mfa_token: mfaToken, code }),
    });
  },

  /**
   * Returns the full URL to redirect the browser to for Google OAuth.
   * The server handles the consent screen and callback entirely; no fetch needed.
   * The locale is forwarded so the server can redirect back to the correct locale prefix.
   */
  googleAuthUrl(locale: string): string {
    return `${API_BASE_URL}/v1/auth/google?locale=${locale}`;
  },

  /** GET /v1/api/me  — requires a valid JWT in localStorage */
  me(): Promise<User> {
    return request('/v1/api/me', { headers: bearerHeader() });
  },

  /** GET /health */
  health(): Promise<{ status: string }> {
    return request('/health');
  },

  /** GET /v1/system/status — public, no token required */
  systemStatus(): Promise<SystemStatus> {
    return request('/v1/system/status');
  },

  // --- System management (root only) ---

  /** POST /v1/api/system/assign-role — pass role="" to remove system role */
  assignSystemRole(userId: number, role: string): Promise<User> {
    return request('/v1/api/system/assign-role', {
      method: 'POST',
      headers: bearerHeader(),
      body: JSON.stringify({ user_id: userId, role }),
    });
  },

  // --- Organisation CRUD (system/root) ---

  /** GET /v1/api/orgs */
  listOrgs(): Promise<Organization[]> {
    return request('/v1/api/orgs', { headers: bearerHeader() });
  },

  /** POST /v1/api/orgs */
  createOrg(name: string): Promise<Organization> {
    return request('/v1/api/orgs', {
      method: 'POST',
      headers: bearerHeader(),
      body: JSON.stringify({ name }),
    });
  },

  /** GET /v1/api/orgs/:orgID */
  getOrg(orgId: number): Promise<Organization> {
    return request(`/v1/api/orgs/${orgId}`, { headers: bearerHeader() });
  },

  /** PUT /v1/api/orgs/:orgID */
  updateOrg(orgId: number, name: string): Promise<Organization> {
    return request(`/v1/api/orgs/${orgId}`, {
      method: 'PUT',
      headers: bearerHeader(),
      body: JSON.stringify({ name }),
    });
  },

  /** DELETE /v1/api/orgs/:orgID */
  deleteOrg(orgId: number): Promise<void> {
    return request(`/v1/api/orgs/${orgId}`, {
      method: 'DELETE',
      headers: bearerHeader(),
    });
  },

  // --- Member management ---

  /** GET /v1/api/orgs/:orgID/members */
  listMembers(orgId: number): Promise<OrganizationMember[]> {
    return request(`/v1/api/orgs/${orgId}/members`, { headers: bearerHeader() });
  },

  /** POST /v1/api/orgs/:orgID/members */
  addMember(orgId: number, userId: number, role: string): Promise<OrganizationMember> {
    return request(`/v1/api/orgs/${orgId}/members`, {
      method: 'POST',
      headers: bearerHeader(),
      body: JSON.stringify({ user_id: userId, role }),
    });
  },

  /** PUT /v1/api/orgs/:orgID/members/:userID */
  updateMember(orgId: number, userId: number, role: string): Promise<OrganizationMember> {
    return request(`/v1/api/orgs/${orgId}/members/${userId}`, {
      method: 'PUT',
      headers: bearerHeader(),
      body: JSON.stringify({ role }),
    });
  },

  /** DELETE /v1/api/orgs/:orgID/members/:userID */
  removeMember(orgId: number, userId: number): Promise<void> {
    return request(`/v1/api/orgs/${orgId}/members/${userId}`, {
      method: 'DELETE',
      headers: bearerHeader(),
    });
  },

  // --- Admin (root only) ---

  /** GET /v1/api/admin/users — search users by email with pagination */
  searchUsers(email: string, page: number, size: number): Promise<PaginatedResult<UserWithOrgs>> {
    const params = new URLSearchParams();
    if (email) params.set('email', email);
    params.set('page', String(page));
    params.set('size', String(size));
    return requestPaginated(`/v1/api/admin/users?${params}`, { headers: bearerHeader() });
  },

  // --- MFA / Profile ---

  /** GET /v1/api/profile/mfa/setup — generate a new TOTP secret and QR URI */
  mfaSetup(): Promise<MFASetupResponse> {
    return request('/v1/api/profile/mfa/setup', { headers: bearerHeader() });
  },

  /** POST /v1/api/profile/mfa/enable — verify code and activate MFA */
  mfaEnable(code: string): Promise<MFAStatusResponse> {
    return request('/v1/api/profile/mfa/enable', {
      method: 'POST',
      headers: bearerHeader(),
      body: JSON.stringify({ code }),
    });
  },

  /** DELETE /v1/api/profile/mfa — verify code and deactivate MFA */
  mfaDisable(code: string): Promise<MFAStatusResponse> {
    return request('/v1/api/profile/mfa', {
      method: 'DELETE',
      headers: bearerHeader(),
      body: JSON.stringify({ code }),
    });
  },
};
