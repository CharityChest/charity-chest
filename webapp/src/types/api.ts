// Mirrors the JSON-serialised User model from the Go server.
// Fields tagged `json:"-"` on the server (PasswordHash, GoogleID, TOTPSecret) are never present here.
export interface User {
  id: number;
  email: string;
  name: string;
  role?: string | null;
  mfa_enabled: boolean;
  created_at: string;
  updated_at: string;
}

// Returned by POST /v1/auth/register.
export interface AuthResponse {
  token: string;
  user: User;
}

// Returned by POST /v1/auth/login — either a full token or an MFA challenge.
export interface LoginResponse {
  token?: string;
  user?: User;
  mfa_required?: boolean;
  mfa_token?: string;
}

// Returned by GET /v1/api/profile/mfa/setup.
export interface MFASetupResponse {
  uri: string;
  secret: string;
}

// Returned by POST /v1/api/profile/mfa/enable and DELETE /v1/api/profile/mfa.
export interface MFAStatusResponse {
  mfa_enabled: boolean;
}

// Returned by GET /v1/system/status.
export interface SystemStatus {
  configured: boolean;
}

export interface Organization {
  id: number;
  name: string;
  created_at: string;
  updated_at: string;
  members?: OrganizationMember[];
}

export interface OrganizationMember {
  id: number;
  org_id: number;
  user_id: number;
  role: string;
  created_at: string;
  updated_at: string;
  user?: User;
}

export interface PaginationMeta {
  page: number;
  size: number;
  total: number;
  total_pages: number;
}

export interface PaginatedResult<T> {
  data: T[];
  metadata: PaginationMeta;
}

export interface OrgSummary {
  id: number;
  name: string;
  role: string;
}

export interface UserWithOrgs extends User {
  organizations: OrgSummary[];
}
