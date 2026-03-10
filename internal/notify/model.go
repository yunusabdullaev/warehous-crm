package notify

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// NotifySettings stored as single doc in "settings" collection (key="notifications").
type NotifySettings struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	Key             string             `bson:"key" json:"key"`
	TelegramEnabled bool               `bson:"telegram_enabled" json:"telegram_enabled"`
	TelegramToken   string             `bson:"telegram_bot_token" json:"telegram_bot_token"`
	TelegramChatIDs string             `bson:"telegram_chat_ids" json:"telegram_chat_ids"`

	// ── Expiry Digest ──
	ExpiryDigestEnabled bool   `bson:"expiry_digest_enabled" json:"expiry_digest_enabled"`
	ExpiryDigestDays    int    `bson:"expiry_digest_days" json:"expiry_digest_days"`
	ExpiryDigestTime    string `bson:"expiry_digest_time" json:"expiry_digest_time"`
	ExpiryDigestChatIDs string `bson:"expiry_digest_chat_ids" json:"expiry_digest_chat_ids"`

	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// DigestResult returned by RunExpiryDigest.
type DigestResult struct {
	Sent    bool   `json:"sent"`
	Skipped bool   `json:"skipped"`
	Reason  string `json:"reason,omitempty"`
	Total   int    `json:"total"`
	Urgent  int    `json:"urgent"`
	Warning int    `json:"warning"`
	Notice  int    `json:"notice"`
}

// AlertDedup tracks per-key alert de-duplication.
type AlertDedup struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Key        string             `bson:"key" json:"key"`
	LastSentAt time.Time          `bson:"last_sent_at" json:"last_sent_at"`
}
