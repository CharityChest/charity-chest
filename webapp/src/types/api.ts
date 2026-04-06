// Mirrors the JSON-serialised User model from the Go server.
// Fields tagged `json:"-"` on the server (PasswordHash, GoogleID) are never present here.
export interface User {
  id: number;
  email: string;
  name: string;
  role?: string | null;
  created_at: string;
  updated_at: string;
}

// Returned by POST /v1/auth/register and POST /v1/auth/login.
export interface AuthResponse {
  token: string;
  user: User;
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
