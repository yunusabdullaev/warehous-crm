package location

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(router fiber.Router, handler *Handler) {
	locations := router.Group("/locations")
	locations.Post("/", handler.Create)
	locations.Get("/", handler.List)
	locations.Get("/:id", handler.GetByID)
	locations.Put("/:id", handler.Update)
	locations.Delete("/:id", handler.Delete)
}
