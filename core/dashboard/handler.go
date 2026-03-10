package dashboard

import (
	"time"

	"warehouse-crm/core/middleware"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Summary(c *fiber.Ctx) error {
	fromStr := c.Query("from")
	toStr := c.Query("to")

	var from, to time.Time
	var err error
	if fromStr != "" {
		from, err = time.Parse("2006-01-02", fromStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid 'from' date, use YYYY-MM-DD"})
		}
	}
	if toStr != "" {
		to, err = time.Parse("2006-01-02", toStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid 'to' date, use YYYY-MM-DD"})
		}
		// Include the entire "to" day
		to = to.Add(24*time.Hour - time.Nanosecond)
	}

	// ALL mode → zero ObjectID → no warehouse filter
	warehouseID := middleware.GetWarehouseID(c)

	summary, err := h.service.GetSummary(c.Context(), warehouseID, from, to)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(summary)
}
