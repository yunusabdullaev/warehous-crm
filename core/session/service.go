package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	jwtPkg "warehouse-crm/pkg/jwt"
)

const (
	BruteForceMaxAttempts = 10
	BruteForceWindow      = 15 * time.Minute
)

type Service struct {
	repo           *Repository
	jwtSecret      string
	accessTTLMin   int
	refreshTTLDays int
}

func NewService(repo *Repository, jwtSecret string, accessTTLMin, refreshTTLDays int) *Service {
	return &Service{
		repo:           repo,
		jwtSecret:      jwtSecret,
		accessTTLMin:   accessTTLMin,
		refreshTTLDays: refreshTTLDays,
	}
}

// ── Session lifecycle ──

// CreateSession creates a new session and returns the session ID and raw refresh token.
func (s *Service) CreateSession(ctx context.Context, userID, tenantID primitive.ObjectID, ip, userAgent string) (primitive.ObjectID, string, error) {
	rawToken, err := generateSecureToken(48)
	if err != nil {
		return primitive.NilObjectID, "", err
	}
	hash := hashToken(rawToken)

	sess := &Session{
		UserID:           userID,
		TenantID:         tenantID,
		RefreshTokenHash: hash,
		ExpiresAt:        time.Now().UTC().Add(time.Duration(s.refreshTTLDays) * 24 * time.Hour),
		IP:               ip,
		UserAgent:        truncate(userAgent, 256),
	}

	if err := s.repo.CreateSession(ctx, sess); err != nil {
		return primitive.NilObjectID, "", err
	}

	return sess.ID, rawToken, nil
}

// RefreshSession validates the refresh token, rotates it, and issues a new access token.
func (s *Service) RefreshSession(ctx context.Context, rawToken string, userID, username, role, tenantID string, ip string) (accessToken, newRefreshToken string, err error) {
	hash := hashToken(rawToken)

	sess, err := s.repo.FindActiveSessionByHash(ctx, hash)
	if err != nil {
		return "", "", errors.New("invalid or expired refresh token")
	}

	// Verify session belongs to claimed user
	if sess.UserID.Hex() != userID {
		// Possible token theft — revoke the session
		s.repo.RevokeSession(ctx, sess.ID)
		slog.Warn("session: possible token theft detected",
			"session_id", sess.ID.Hex(), "claimed_user", userID, "session_user", sess.UserID.Hex())
		return "", "", errors.New("invalid refresh token")
	}

	// Revoke old session
	_ = s.repo.RevokeSession(ctx, sess.ID)

	// Create rotated session
	newRaw, err := generateSecureToken(48)
	if err != nil {
		return "", "", err
	}
	newHash := hashToken(newRaw)

	newSess := &Session{
		UserID:               sess.UserID,
		TenantID:             sess.TenantID,
		RefreshTokenHash:     newHash,
		ExpiresAt:            time.Now().UTC().Add(time.Duration(s.refreshTTLDays) * 24 * time.Hour),
		IP:                   ip,
		UserAgent:            sess.UserAgent,
		RotatedFromSessionID: sess.ID,
	}

	if err := s.repo.CreateSession(ctx, newSess); err != nil {
		return "", "", err
	}

	// Issue new access token
	ttl := time.Duration(s.accessTTLMin) * time.Minute
	access, err := jwtPkg.GenerateAccessToken(userID, username, role, tenantID, s.jwtSecret, ttl)
	if err != nil {
		return "", "", err
	}

	return access, newRaw, nil
}

// RevokeSession revokes a single session.
func (s *Service) RevokeSession(ctx context.Context, sessionID primitive.ObjectID) error {
	return s.repo.RevokeSession(ctx, sessionID)
}

// RevokeByRefreshToken revokes the session matching the given raw refresh token.
func (s *Service) RevokeByRefreshToken(ctx context.Context, rawToken string) error {
	hash := hashToken(rawToken)
	sess, err := s.repo.FindActiveSessionByHash(ctx, hash)
	if err != nil {
		return nil // already revoked or expired — idempotent
	}
	return s.repo.RevokeSession(ctx, sess.ID)
}

// RevokeAllUserSessions revokes all active sessions for a user.
func (s *Service) RevokeAllUserSessions(ctx context.Context, userID primitive.ObjectID) (int64, error) {
	return s.repo.RevokeAllUserSessions(ctx, userID)
}

// ListActiveSessions lists all active sessions for a user.
func (s *Service) ListActiveSessions(ctx context.Context, userID primitive.ObjectID) ([]Session, error) {
	return s.repo.ListActiveSessions(ctx, userID)
}

// ── Brute-force protection ──

// CheckBruteForce returns true if the account is locked.
func (s *Service) CheckBruteForce(ctx context.Context, username, ip string) (bool, int64) {
	count, err := s.repo.CountRecentFailures(ctx, username, ip, BruteForceWindow)
	if err != nil {
		return false, 0
	}
	return count >= BruteForceMaxAttempts, count
}

// RecordLoginAttempt records a login attempt.
func (s *Service) RecordLoginAttempt(ctx context.Context, username, ip string, success bool) {
	_ = s.repo.RecordAttempt(ctx, &LoginAttempt{
		Username: username,
		IP:       ip,
		Success:  success,
	})
}

// ── Reset tokens ──

// GenerateResetToken creates a one-time password reset token (30min TTL).
func (s *Service) GenerateResetToken(ctx context.Context, userID, createdBy primitive.ObjectID) (string, error) {
	rawToken, err := generateSecureToken(32)
	if err != nil {
		return "", err
	}
	hash := hashToken(rawToken)

	rt := &ResetToken{
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
		CreatedBy: createdBy,
	}

	if err := s.repo.CreateResetToken(ctx, rt); err != nil {
		return "", err
	}

	return rawToken, nil
}

// ValidateResetToken checks if a reset token is valid, returns the user ID.
func (s *Service) ValidateResetToken(ctx context.Context, rawToken string) (primitive.ObjectID, error) {
	hash := hashToken(rawToken)
	rt, err := s.repo.FindValidResetToken(ctx, hash)
	if err != nil {
		return primitive.NilObjectID, errors.New("invalid or expired reset token")
	}
	return rt.UserID, nil
}

// ConsumeResetToken marks the reset token as used.
func (s *Service) ConsumeResetToken(ctx context.Context, rawToken string) error {
	hash := hashToken(rawToken)
	rt, err := s.repo.FindValidResetToken(ctx, hash)
	if err != nil {
		return errors.New("invalid or expired reset token")
	}
	return s.repo.MarkResetTokenUsed(ctx, rt.ID)
}

// ── Access Token ──

// IssueAccessToken generates a new access JWT.
func (s *Service) IssueAccessToken(userID, username, role, tenantID string) (string, error) {
	ttl := time.Duration(s.accessTTLMin) * time.Minute
	return jwtPkg.GenerateAccessToken(userID, username, role, tenantID, s.jwtSecret, ttl)
}

// IssueTempToken generates a temp JWT for 2FA.
func (s *Service) IssueTempToken(userID, username string) (string, error) {
	return jwtPkg.GenerateTempToken(userID, username, s.jwtSecret, 5*time.Minute)
}

// ValidateTempToken validates a temp token for 2FA.
func (s *Service) ValidateTempToken(tokenStr string) (*jwtPkg.Claims, error) {
	claims, err := jwtPkg.ValidateToken(tokenStr, s.jwtSecret)
	if err != nil {
		return nil, err
	}
	if claims.Purpose != "2fa" {
		return nil, errors.New("not a 2FA temporary token")
	}
	return claims, nil
}

// ── Helpers ──

func generateSecureToken(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}
