package adminclient

import "github.com/google/uuid"

// User is the slim DTO returned by admin's /v1/internal/* endpoints. It mirrors
// the shape admin's handler/internal_api.go encodes. Sensitive columns (id,
// password_hash, totp_secret, google_id) are intentionally absent.
type User struct {
	UUID       uuid.UUID `json:"uuid"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	Role       *string   `json:"role,omitempty"`
	MFAEnabled bool      `json:"mfa_enabled"`
}
