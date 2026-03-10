package middleware

import (
	"github.com/gofiber/fiber/v2"
)

// RequireRoles returns middleware that restricts access to the listed roles.
// Must be placed AFTER AuthMiddleware so c.Locals("role") is set.
// Superadmin always has access to all routes.
func RequireRoles(allowed ...string) fiber.Handler {
	roleSet := make(map[string]bool, len(allowed))
	for _, r := range allowed {
		roleSet[r] = true
	}
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		// superadmin bypasses all role gates
		if role == "superadmin" {
			return c.Next()
		}
		if !roleSet[role] {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":   "forbidden",
				"message": "your role '" + role + "' does not have access to this resource",
			})
		}
		return c.Next()
	}
}
