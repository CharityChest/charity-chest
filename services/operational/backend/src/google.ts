// Google ID token validation. The handler depends only on the GoogleValidator
// interface so tests inject a fake; the real implementation wraps
// google-auth-library's OAuth2Client and is constructed in main.ts.

import { OAuth2Client } from "google-auth-library";

/** The subset of a verified Google ID token the auth handler needs. */
export interface GooglePayload {
  subject: string;
  email: string;
  name: string;
}

/**
 * Verifies a Google ID token against the expected audience(s). `audience` may
 * be a single client ID or a list — the token's `aud` must match one of them
 * (the mobile app uses a different OAuth client per platform).
 */
export interface GoogleValidator {
  validate(idToken: string, audience: string | string[]): Promise<GooglePayload>;
}

/** Production GoogleValidator backed by google-auth-library. */
export class RealGoogleValidator implements GoogleValidator {
  private readonly client = new OAuth2Client();

  async validate(idToken: string, audience: string | string[]): Promise<GooglePayload> {
    const ticket = await this.client.verifyIdToken({ idToken, audience });
    const payload = ticket.getPayload();
    if (!payload || !payload.sub) {
      throw new Error("google: empty token payload");
    }
    return {
      subject: payload.sub,
      email: payload.email ?? "",
      name: payload.name ?? "",
    };
  }
}
