/**
 * Mirrors the JSON-serialised User model from the Go server.
 * Fields tagged `json:"-"` on the server (PasswordHash, GoogleID, TOTPSecret) are never present here.
 */
export interface User {
  uuid: string;
  email: string;
  name: string;
  role?: string | null;
  mfa_enabled: boolean;
  created_at: string;
  updated_at: string;
}

/** Returned by POST /v1/auth/register. */
export interface AuthResponse {
  token: string;
  user: User;
}

/** Returned by POST /v1/auth/login — either a full token or an MFA challenge. */
export interface LoginResponse {
  token?: string;
  user?: User;
  mfa_required?: boolean;
  mfa_token?: string;
}

/** Returned by GET /v1/api/profile/mfa/setup. */
export interface MFASetupResponse {
  uri: string;
  secret: string;
}

/** Returned by POST /v1/api/profile/mfa/enable and DELETE /v1/api/profile/mfa. */
export interface MFAStatusResponse {
  mfa_enabled: boolean;
}

/** Returned by GET /v1/system/status. */
export interface SystemStatus {
  configured: boolean;
}

/** Subscription plan for an organisation. */
export type Plan = 'free' | 'pro' | 'enterprise';

/** Returned by GET /v1/api/orgs and GET /v1/api/orgs/:orgUUID. */
export interface Organization {
  uuid: string;
  name: string;
  plan: Plan;
  created_at: string;
  updated_at: string;
  members?: OrganizationMember[];
}

/** Returned by POST /v1/api/orgs/:orgUUID/billing/checkout. */
export interface BillingCheckoutResponse {
  url: string;
}

/** A single row from GET /v1/api/orgs/:orgUUID/members. */
export interface OrganizationMember {
  uuid: string;
  role: string;
  created_at: string;
  updated_at: string;
  user?: User;
}

/** Pagination metadata included in every paginated list response. */
export interface PaginationMeta {
  page: number;
  size: number;
  total: number;
  total_pages: number;
}

/** Envelope for paginated list responses: `{ data: T[], metadata: PaginationMeta }`. */
export interface PaginatedResult<T> {
  data: T[];
  metadata: PaginationMeta;
}

/** Organisation summary embedded in admin user-search results. */
export interface OrgSummary {
  uuid: string;
  name: string;
  role: string;
}

/** Returned by GET /v1/api/admin/users — a user record enriched with org memberships. */
export interface UserWithOrgs extends User {
  organizations: OrgSummary[];
}
