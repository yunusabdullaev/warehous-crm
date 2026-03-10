package dashboard

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(router fiber.Router, handler *Handler) {
	d := router.Group("/dashboard")
	d.Get("/summary", handler.Summary)
}
