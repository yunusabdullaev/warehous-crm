package adjustment

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(router fiber.Router, handler *Handler) {
	a := router.Group("/adjustments")
	a.Post("/", handler.Create)
	a.Get("/", handler.List)
}
