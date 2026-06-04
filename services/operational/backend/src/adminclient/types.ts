// UserDTO mirrors the slim DTO admin returns from /v1/internal/*. Sensitive
// columns (id, password_hash, totp_secret, google_id) are intentionally
// absent. `role` is optional/nullable, matching admin's `omitempty` encoding.
export interface UserDTO {
  uuid: string;
  email: string;
  name: string;
  role?: string | null;
  mfa_enabled: boolean;
}

/**
 * The subset of admin's service-to-service API the operational backend needs.
 * AdminClient implements it against the real HTTP API; tests inject fakes.
 *
 * Each method takes an optional `locale` that is forwarded to admin as the
 * X-Locale header so admin can localize its error messages.
 */
export interface AdminApi {
  login(email: string, password: string, locale?: string): Promise<UserDTO>;
  googleAuth(
    googleSub: string,
    email: string,
    name: string,
    locale?: string,
  ): Promise<UserDTO>;
  getUser(userUuid: string, locale?: string): Promise<UserDTO>;
}
