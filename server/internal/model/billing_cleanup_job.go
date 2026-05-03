package model

import "time"

// BillingCleanupReason categorises why a cleanup job was created.
const (
	// BillingCleanupReasonDuplicateEnterprise marks a Stripe checkout that
	// completed against an org already on the enterprise plan; the resulting
	// subscription must be cancelled and the payment refunded.
	BillingCleanupReasonDuplicateEnterprise = "duplicate_enterprise_checkout"
)

// BillingCleanupJob records a Stripe-side cleanup the server still owes —
// e.g. cancelling a duplicate subscription or refunding a payment — after a
// webhook has already been acknowledged. The webhook persists the job before
// returning 200 so the operation survives a Stripe API outage and can be
// retried out-of-band.
type BillingCleanupJob struct {
	ID                      uint       `gorm:"primaryKey"                       json:"id"`
	OrgID                   uint       `gorm:"not null;index"                   json:"org_id"`
	Reason                  string     `gorm:"not null"                         json:"reason"`
	StripeSubscriptionID    *string    `gorm:"column:stripe_subscription_id"    json:"stripe_subscription_id,omitempty"`
	StripePaymentIntentID   *string    `gorm:"column:stripe_payment_intent_id"  json:"stripe_payment_intent_id,omitempty"`
	SubscriptionCancelledAt *time.Time `gorm:"column:subscription_cancelled_at" json:"subscription_cancelled_at,omitempty"`
	PaymentRefundedAt       *time.Time `gorm:"column:payment_refunded_at"       json:"payment_refunded_at,omitempty"`
	LastError               *string    `gorm:"column:last_error"                json:"last_error,omitempty"`
	AttemptCount            int        `gorm:"not null;default:0"               json:"attempt_count"`
	CreatedAt               time.Time  `                                        json:"created_at"`
	UpdatedAt               time.Time  `                                        json:"updated_at"`
}
