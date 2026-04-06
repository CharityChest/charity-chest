package model

import (
	"time"

	"gorm.io/gorm"
)

// Organization represents a tenant entity that groups users under a shared context.
type Organization struct {
	ID        uint           `gorm:"primaryKey"       json:"id"`
	Name      string         `gorm:"not null"         json:"name"`
	CreatedAt time.Time      `                        json:"created_at"`
	UpdatedAt time.Time      `                        json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
	Members   []OrgMember    `gorm:"foreignKey:OrgID" json:"members,omitempty"`
}

// OrgMember links a User to an Organization with an org-level role.
// Hard deletes are used — membership removal is final and the slot is reusable.
type OrgMember struct {
	ID        uint      `gorm:"primaryKey"                       json:"id"`
	OrgID     uint      `gorm:"not null;uniqueIndex:idx_org_user" json:"org_id"`
	UserID    uint      `gorm:"not null;uniqueIndex:idx_org_user" json:"user_id"`
	Role      string    `gorm:"not null"                         json:"role"`
	CreatedAt time.Time `                                         json:"created_at"`
	UpdatedAt time.Time `                                         json:"updated_at"`
	User      *User     `gorm:"foreignKey:UserID"                json:"user,omitempty"`
}
