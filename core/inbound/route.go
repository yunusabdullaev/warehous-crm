package inbound

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(router fiber.Router, handler *Handler) {
	i := router.Group("/inbound")
	i.Post("/", handler.Create)
	i.Get("/", handler.List)
	i.Get("/:id", handler.GetByID)
	i.Post("/:id/reverse", handler.Reverse)
}
