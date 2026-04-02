// Mirrors the JSON-serialised User model from the Go server.
// Fields tagged `json:"-"` on the server (PasswordHash, GoogleID) are never present here.
export interface User {
  id: number;
  email: string;
  name: string;
  created_at: string;
  updated_at: string;
}

// Returned by POST /v1/auth/register and POST /v1/auth/login.
export interface AuthResponse {
  token: string;
  user: User;
}
