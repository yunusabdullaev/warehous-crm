package billing

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Repository struct {
	col *mongo.Collection
}

func NewRepository(col *mongo.Collection) *Repository {
	return &Repository{col: col}
}

// EventExists checks if a Stripe event has already been processed.
func (r *Repository) EventExists(ctx context.Context, stripeEventID string) (bool, error) {
	count, err := r.col.CountDocuments(ctx, bson.M{"stripe_event_id": stripeEventID})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// InsertEvent records a processed webhook event.
func (r *Repository) InsertEvent(ctx context.Context, evt *BillingEvent) error {
	evt.ID = primitive.NewObjectID()
	evt.ReceivedAt = time.Now().UTC()
	_, err := r.col.InsertOne(ctx, evt)
	return err
}
