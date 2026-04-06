package model

import (
	"time"

	"gorm.io/gorm"
)

// User represents an account that can authenticate via password or Google OAuth.
// PasswordHash is nil for Google-only accounts; GoogleID is nil for password-only accounts.
type User struct {
	ID           uint           `gorm:"primaryKey"            json:"id"`
	Email        string         `gorm:"uniqueIndex;not null"  json:"email"`
	PasswordHash *string        `gorm:"column:password_hash"  json:"-"`
	GoogleID     *string        `gorm:"uniqueIndex"           json:"-"`
	Name         string         `gorm:"not null;default:''"   json:"name"`
	Role         *string        `gorm:"column:role"           json:"role,omitempty"`
	CreatedAt    time.Time      `                             json:"created_at"`
	UpdatedAt    time.Time      `                             json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index"                 json:"-"`
}