package notify

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository struct {
	settings *mongo.Collection
	alerts   *mongo.Collection
}

func NewRepository(settings, alerts *mongo.Collection) *Repository {
	return &Repository{settings: settings, alerts: alerts}
}

// GetSettings returns current notification settings (or defaults).
func (r *Repository) GetSettings(ctx context.Context) (*NotifySettings, error) {
	var s NotifySettings
	err := r.settings.FindOne(ctx, bson.M{"key": "notifications"}).Decode(&s)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &NotifySettings{
				Key:              "notifications",
				ExpiryDigestDays: 14,
				ExpiryDigestTime: "08:30",
			}, nil
		}
		return nil, err
	}
	// Apply defaults for zero values
	if s.ExpiryDigestDays == 0 {
		s.ExpiryDigestDays = 14
	}
	if s.ExpiryDigestTime == "" {
		s.ExpiryDigestTime = "08:30"
	}
	return &s, nil
}

// UpsertSettings creates or updates notification settings.
func (r *Repository) UpsertSettings(ctx context.Context, s *NotifySettings) error {
	s.Key = "notifications"
	s.UpdatedAt = time.Now().UTC()
	opts := options.Update().SetUpsert(true)
	_, err := r.settings.UpdateOne(ctx, bson.M{"key": "notifications"}, bson.M{
		"$set": bson.M{
			"telegram_enabled":       s.TelegramEnabled,
			"telegram_bot_token":     s.TelegramToken,
			"telegram_chat_ids":      s.TelegramChatIDs,
			"expiry_digest_enabled":  s.ExpiryDigestEnabled,
			"expiry_digest_days":     s.ExpiryDigestDays,
			"expiry_digest_time":     s.ExpiryDigestTime,
			"expiry_digest_chat_ids": s.ExpiryDigestChatIDs,
			"updated_at":             s.UpdatedAt,
		},
		"$setOnInsert": bson.M{
			"_id": primitive.NewObjectID(),
			"key": "notifications",
		},
	}, opts)
	return err
}

// GetAlertDedup returns last send time for a dedup key.
func (r *Repository) GetAlertDedup(ctx context.Context, key string) (*AlertDedup, error) {
	var a AlertDedup
	err := r.alerts.FindOne(ctx, bson.M{"key": key}).Decode(&a)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// UpsertAlertDedup marks the current time for a dedup key.
func (r *Repository) UpsertAlertDedup(ctx context.Context, key string) error {
	opts := options.Update().SetUpsert(true)
	_, err := r.alerts.UpdateOne(ctx, bson.M{"key": key}, bson.M{
		"$set": bson.M{
			"last_sent_at": time.Now().UTC(),
		},
		"$setOnInsert": bson.M{
			"_id": primitive.NewObjectID(),
			"key": key,
		},
	}, opts)
	return err
}
