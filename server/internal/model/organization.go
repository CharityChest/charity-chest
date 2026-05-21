package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Organization represents a tenant entity that groups users under a shared context.
// The integer ID is the internal primary key (foreign keys, cache keys); UUID is the
// public identifier exposed in API responses and URL params.
type Organization struct {
	ID                   uint           `gorm:"primaryKey"                                               json:"-"`
	UUID                 uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex;default:gen_random_uuid()" json:"uuid"`
	Name                 string         `gorm:"not null"                                                 json:"name"`
	Plan                 Plan           `gorm:"not null;default:free"                                    json:"plan"`
	StripeCustomerID     *string        `gorm:"column:stripe_customer_id"                                json:"-"`
	StripeSubscriptionID *string        `gorm:"column:stripe_subscription_id"                            json:"-"`
	CreatedAt            time.Time      `                                                                 json:"created_at"`
	UpdatedAt            time.Time      `                                                                 json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index"                                                    json:"-"`
	Members              []OrgMember    `gorm:"foreignKey:OrgID"                                         json:"members,omitempty"`
}

// OrgMember links a User to an Organization with an org-level role.
// Hard deletes are used — membership removal is final and the slot is reusable.
// Int OrgID/UserID are internal foreign keys (hidden from JSON); the embedded
// User (when preloaded) carries the public UUID for the user.
type OrgMember struct {
	ID        uint       `gorm:"primaryKey"                                               json:"-"`
	UUID      uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex;default:gen_random_uuid()" json:"uuid"`
	OrgID     uint       `gorm:"not null;uniqueIndex:idx_org_user"                        json:"-"`
	UserID    uint       `gorm:"not null;uniqueIndex:idx_org_user"                        json:"-"`
	Role      MemberRole `gorm:"not null"                                                 json:"role"`
	CreatedAt time.Time  `                                                                 json:"created_at"`
	UpdatedAt time.Time  `                                                                 json:"updated_at"`
	User      *User      `gorm:"foreignKey:UserID"                                        json:"user,omitempty"`
}
