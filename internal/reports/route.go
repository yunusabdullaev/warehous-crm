package reports

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(router fiber.Router, handler *Handler) {
	r := router.Group("/reports")
	r.Get("/movements", handler.Movements)
	r.Get("/stock", handler.StockReport)
}
