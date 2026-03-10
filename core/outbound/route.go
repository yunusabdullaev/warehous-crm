package outbound

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(router fiber.Router, handler *Handler) {
	o := router.Group("/outbound")
	o.Post("/", handler.Create)
	o.Get("/", handler.List)
	o.Get("/:id", handler.GetByID)
	o.Post("/:id/reverse", handler.Reverse)
}
