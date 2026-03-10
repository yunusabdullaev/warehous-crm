package billing

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// BillingEvent tracks processed Stripe webhook events for idempotency + audit.
type BillingEvent struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TenantID      primitive.ObjectID `bson:"tenant_id,omitempty" json:"tenant_id"`
	StripeEventID string             `bson:"stripe_event_id" json:"stripe_event_id"`
	Type          string             `bson:"type" json:"type"`
	ReceivedAt    time.Time          `bson:"received_at" json:"received_at"`
	PayloadHash   string             `bson:"payload_hash,omitempty" json:"payload_hash,omitempty"`
}
