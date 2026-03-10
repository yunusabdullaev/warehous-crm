package history

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(router fiber.Router, handler *Handler) {
	h := router.Group("/history")
	h.Get("/", handler.List)
}
