package stock

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(router fiber.Router, handler *Handler) {
	s := router.Group("/stock")
	s.Get("/", handler.ListAll)
	s.Get("/product/:id", handler.GetByProduct)
	s.Get("/location/:id", handler.GetByLocation)
}
