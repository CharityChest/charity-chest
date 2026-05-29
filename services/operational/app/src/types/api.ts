// Shape of every successful operational backend response.
export type ApiEnvelope<T> = { data: T };

// Slim user DTO returned by the operational backend's /v1/api/me and the
// login responses. Mirrors the JSON shape the Go handlers encode.
export type User = {
  uuid: string;
  email: string;
  name: string;
  role?: string | null;
  mfa_enabled: boolean;
};

// Shape returned by POST /v1/auth/login and /v1/auth/google.
export type LoginResponse = {
  token: string;
  user: User;
};

// Thrown by the API client on any non-2xx response.
export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}
