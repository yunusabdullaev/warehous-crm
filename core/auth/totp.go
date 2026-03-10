package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image/png"

	"github.com/pquerna/otp/totp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Setup2FA generates a TOTP secret for the user (does not enable yet).
func (s *Service) Setup2FA(ctx context.Context, userID primitive.ObjectID) (*Setup2FAResponse, error) {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Check feature flag on tenant
	if !s.is2FAAllowed(ctx, user.TenantID) {
		return nil, errors.New("2FA requires ENTERPRISE plan")
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "WarehouseCRM",
		AccountName: user.Username,
	})
	if err != nil {
		return nil, err
	}

	// Store secret (not yet enabled)
	_ = s.repo.Update(ctx, userID, bson.M{
		"two_factor_secret": key.Secret(),
	})

	// Generate QR PNG as base64
	img, err := key.Image(200, 200)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	qrB64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	return &Setup2FAResponse{
		Secret: key.Secret(),
		URI:    key.URL(),
		QR:     qrB64,
	}, nil
}

// Verify2FA validates a TOTP code and enables 2FA on the user account.
func (s *Service) Verify2FA(ctx context.Context, userID primitive.ObjectID, code string) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	if user.TwoFactorSecret == "" {
		return errors.New("2FA not set up — call setup first")
	}

	if !totp.Validate(code, user.TwoFactorSecret) {
		return errors.New("invalid 2FA code")
	}

	return s.repo.Update(ctx, userID, bson.M{
		"two_factor_enabled": true,
	})
}

// Disable2FA disables 2FA on the user account.
func (s *Service) Disable2FA(ctx context.Context, userID primitive.ObjectID) error {
	return s.repo.Update(ctx, userID, bson.M{
		"two_factor_enabled": false,
		"two_factor_secret":  "",
	})
}

// validate2FACode checks a TOTP code without changing state.
func (s *Service) validate2FACode(ctx context.Context, userID primitive.ObjectID, code string) error {
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return errors.New("user not found")
	}

	if !user.TwoFactorEnabled || user.TwoFactorSecret == "" {
		return errors.New("2FA not enabled")
	}

	if !totp.Validate(code, user.TwoFactorSecret) {
		return errors.New("invalid 2FA code")
	}

	return nil
}

// is2FAAllowed checks if the tenant's plan supports 2FA.
func (s *Service) is2FAAllowed(ctx context.Context, tenantID primitive.ObjectID) bool {
	if tenantID.IsZero() {
		return true // superadmin without tenant
	}
	if s.tenantCol == nil {
		return false
	}

	var tenant struct {
		Features struct {
			Enable2FA bool `bson:"enable_2fa"`
		} `bson:"features"`
	}
	err := s.tenantCol.FindOne(ctx, bson.M{"_id": tenantID}).Decode(&tenant)
	if err != nil {
		return false
	}
	return tenant.Features.Enable2FA
}
