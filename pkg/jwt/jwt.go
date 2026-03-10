package jwt

import (
	"errors"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	TenantID string `json:"tenant_id,omitempty"`
	Purpose  string `json:"purpose,omitempty"` // "2fa" for temp tokens
	jwtlib.RegisteredClaims
}

// GenerateToken creates a JWT with expiry in hours (backward compat).
func GenerateToken(userID, username, role, tenantID, secret string, expiryHrs int) (string, error) {
	return GenerateAccessToken(userID, username, role, tenantID, secret, time.Duration(expiryHrs)*time.Hour)
}

// GenerateAccessToken creates a JWT with a specific duration.
func GenerateAccessToken(userID, username, role, tenantID, secret string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		TenantID: tenantID,
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwtlib.NewNumericDate(time.Now()),
		},
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// GenerateTempToken creates a short-lived JWT for 2FA login step.
func GenerateTempToken(userID, username, secret string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		Purpose:  "2fa",
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwtlib.NewNumericDate(time.Now()),
		},
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ValidateToken(tokenStr, secret string) (*Claims, error) {
	token, err := jwtlib.ParseWithClaims(tokenStr, &Claims{}, func(t *jwtlib.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// ParseTokenIgnoringExpiry extracts claims from a JWT without validating expiry.
// The signature is still verified. Used during token refresh to read user info
// from an expired access token.
func ParseTokenIgnoringExpiry(tokenStr, secret string) (*Claims, error) {
	token, err := jwtlib.ParseWithClaims(tokenStr, &Claims{}, func(t *jwtlib.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	}, jwtlib.WithoutClaimsValidation())
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}
