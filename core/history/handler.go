package history

import (
	"strconv"

	"warehouse-crm/core/middleware"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	query := &HistoryQuery{
		UserID:      c.Query("user_id"),
		EntityType:  c.Query("entity_type"),
		EntityID:    c.Query("entity_id"),
		WarehouseID: middleware.GetWarehouseID(c),
	}

	histories, total, err := h.service.GetHistory(c.Context(), query, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var resp []*HistoryResponse
	for _, hist := range histories {
		resp = append(resp, ToResponse(hist))
	}
	return c.JSON(fiber.Map{"data": resp, "total": total, "page": page, "limit": limit})
}
