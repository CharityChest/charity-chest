package model

import "time"

// PasswordResetToken records a single password-reset request. The plaintext
// token is sent only in the recovery email; the database stores only its
// SHA-256 hex digest so that read access to the table does not yield usable
// tokens. A row is consumed by stamping UsedAt — never deleted — so that an
// attacker who somehow learns a hash cannot reuse it after the legitimate
// holder has redeemed it.
type PasswordResetToken struct {
	ID        uint       `gorm:"primaryKey"                       json:"id"`
	UserID    uint       `gorm:"not null;index"                   json:"user_id"`
	TokenHash string     `gorm:"column:token_hash;uniqueIndex;not null;size:64" json:"-"`
	ExpiresAt time.Time  `gorm:"column:expires_at;not null;index" json:"expires_at"`
	UsedAt    *time.Time `gorm:"column:used_at"                   json:"used_at,omitempty"`
	CreatedAt time.Time  `                                        json:"created_at"`
}
