package session

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Session represents a user login session with a refresh token.
type Session struct {
	ID                   primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TenantID             primitive.ObjectID `bson:"tenant_id,omitempty" json:"tenant_id,omitempty"`
	UserID               primitive.ObjectID `bson:"user_id" json:"user_id"`
	RefreshTokenHash     string             `bson:"refresh_token_hash" json:"-"`
	CreatedAt            time.Time          `bson:"created_at" json:"created_at"`
	ExpiresAt            time.Time          `bson:"expires_at" json:"expires_at"`
	LastUsedAt           time.Time          `bson:"last_used_at" json:"last_used_at"`
	IP                   string             `bson:"ip,omitempty" json:"ip,omitempty"`
	UserAgent            string             `bson:"user_agent,omitempty" json:"user_agent,omitempty"`
	RevokedAt            *time.Time         `bson:"revoked_at,omitempty" json:"revoked_at,omitempty"`
	RotatedFromSessionID primitive.ObjectID `bson:"rotated_from_session_id,omitempty" json:"-"`
}

// LoginAttempt tracks login attempts for brute-force protection.
type LoginAttempt struct {
	ID       primitive.ObjectID `bson:"_id,omitempty"`
	Username string             `bson:"username"`
	IP       string             `bson:"ip"`
	Success  bool               `bson:"success"`
	At       time.Time          `bson:"at"`
}

// ResetToken stores admin-generated password reset tokens.
type ResetToken struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    primitive.ObjectID `bson:"user_id"`
	TokenHash string             `bson:"token_hash"`
	ExpiresAt time.Time          `bson:"expires_at"`
	UsedAt    *time.Time         `bson:"used_at,omitempty"`
	CreatedBy primitive.ObjectID `bson:"created_by"`
	CreatedAt time.Time          `bson:"created_at"`
}
