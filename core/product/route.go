package product

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(router fiber.Router, handler *Handler) {
	products := router.Group("/products")
	products.Post("/", handler.Create)
	products.Get("/", handler.List)
	products.Get("/:id", handler.GetByID)
	products.Put("/:id", handler.Update)
	products.Delete("/:id", handler.Delete)
}
