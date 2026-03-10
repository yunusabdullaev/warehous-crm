package auth

import "github.com/gofiber/fiber/v2"

// RegisterPublicRoutes registers unauthenticated auth endpoints.
func RegisterPublicRoutes(router fiber.Router, handler *Handler) {
	auth := router.Group("/auth")
	auth.Post("/register", handler.Register)
	auth.Post("/login", handler.Login)
	auth.Post("/login-2fa", handler.Login2FA)
	auth.Post("/refresh", handler.Refresh)
	auth.Post("/logout", handler.Logout)
	auth.Post("/reset-password", handler.ResetPassword)
}

// RegisterProtectedRoutes registers authenticated auth endpoints.
func RegisterProtectedRoutes(router fiber.Router, handler *Handler) {
	auth := router.Group("/auth")
	auth.Post("/logout-all", handler.LogoutAll)
	auth.Get("/sessions", handler.ListSessions)
	auth.Delete("/sessions/:id", handler.RevokeSession)

	// 2FA
	auth.Post("/2fa/setup", handler.Setup2FA)
	auth.Post("/2fa/verify", handler.Verify2FA)
	auth.Delete("/2fa", handler.Disable2FA)
}
