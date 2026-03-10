package session

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository struct {
	sessions      *mongo.Collection
	loginAttempts *mongo.Collection
	resetTokens   *mongo.Collection
}

func NewRepository(sessions, loginAttempts, resetTokens *mongo.Collection) *Repository {
	return &Repository{
		sessions:      sessions,
		loginAttempts: loginAttempts,
		resetTokens:   resetTokens,
	}
}

// ── Sessions ──

func (r *Repository) CreateSession(ctx context.Context, s *Session) error {
	s.ID = primitive.NewObjectID()
	s.CreatedAt = time.Now().UTC()
	s.LastUsedAt = s.CreatedAt
	_, err := r.sessions.InsertOne(ctx, s)
	return err
}

func (r *Repository) FindSessionByID(ctx context.Context, id primitive.ObjectID) (*Session, error) {
	var s Session
	err := r.sessions.FindOne(ctx, bson.M{"_id": id}).Decode(&s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) FindActiveSessionByHash(ctx context.Context, hash string) (*Session, error) {
	var s Session
	err := r.sessions.FindOne(ctx, bson.M{
		"refresh_token_hash": hash,
		"revoked_at":         nil,
		"expires_at":         bson.M{"$gt": time.Now().UTC()},
	}).Decode(&s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repository) UpdateSession(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	_, err := r.sessions.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func (r *Repository) RevokeSession(ctx context.Context, id primitive.ObjectID) error {
	now := time.Now().UTC()
	_, err := r.sessions.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{"revoked_at": now},
	})
	return err
}

func (r *Repository) RevokeAllUserSessions(ctx context.Context, userID primitive.ObjectID) (int64, error) {
	now := time.Now().UTC()
	res, err := r.sessions.UpdateMany(ctx,
		bson.M{"user_id": userID, "revoked_at": nil},
		bson.M{"$set": bson.M{"revoked_at": now}},
	)
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

func (r *Repository) ListActiveSessions(ctx context.Context, userID primitive.ObjectID) ([]Session, error) {
	cursor, err := r.sessions.Find(ctx, bson.M{
		"user_id":    userID,
		"revoked_at": nil,
		"expires_at": bson.M{"$gt": time.Now().UTC()},
	}, options.Find().SetSort(bson.D{{Key: "last_used_at", Value: -1}}).SetLimit(50))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var sessions []Session
	if err := cursor.All(ctx, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// ── Login Attempts ──

func (r *Repository) RecordAttempt(ctx context.Context, a *LoginAttempt) error {
	a.ID = primitive.NewObjectID()
	a.At = time.Now().UTC()
	_, err := r.loginAttempts.InsertOne(ctx, a)
	return err
}

func (r *Repository) CountRecentFailures(ctx context.Context, username, ip string, window time.Duration) (int64, error) {
	since := time.Now().UTC().Add(-window)
	return r.loginAttempts.CountDocuments(ctx, bson.M{
		"username": username,
		"ip":       ip,
		"success":  false,
		"at":       bson.M{"$gte": since},
	})
}

// ── Reset Tokens ──

func (r *Repository) CreateResetToken(ctx context.Context, t *ResetToken) error {
	t.ID = primitive.NewObjectID()
	t.CreatedAt = time.Now().UTC()
	_, err := r.resetTokens.InsertOne(ctx, t)
	return err
}

func (r *Repository) FindValidResetToken(ctx context.Context, hash string) (*ResetToken, error) {
	var t ResetToken
	err := r.resetTokens.FindOne(ctx, bson.M{
		"token_hash": hash,
		"used_at":    nil,
		"expires_at": bson.M{"$gt": time.Now().UTC()},
	}).Decode(&t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *Repository) MarkResetTokenUsed(ctx context.Context, id primitive.ObjectID) error {
	now := time.Now().UTC()
	_, err := r.resetTokens.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{"used_at": now},
	})
	return err
}
