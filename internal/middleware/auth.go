package middleware

import (
	"strings"

	jwtPkg "warehouse-crm/pkg/jwt"

	"github.com/gofiber/fiber/v2"
)

func AuthMiddleware(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var tokenStr string

		// 1. Try Authorization header first
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				tokenStr = parts[1]
			}
		}

		// 2. Fall back to HttpOnly cookie
		if tokenStr == "" {
			tokenStr = c.Cookies("wms_access")
		}

		if tokenStr == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing authorization",
			})
		}

		claims, err := jwtPkg.ValidateToken(tokenStr, secret)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid or expired token",
			})
		}

		// Block temp tokens from being used as access tokens
		if claims.Purpose == "2fa" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "temporary token cannot be used for API access",
			})
		}

		c.Locals("userID", claims.UserID)
		c.Locals("username", claims.Username)
		c.Locals("role", claims.Role)
		c.Locals("tenantID", claims.TenantID)

		return c.Next()
	}
}
