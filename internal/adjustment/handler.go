package adjustment

import (
	"strconv"
	"time"

	"warehouse-crm/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var req CreateAdjustmentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	userID, _ := c.Locals("userID").(string)
	warehouseID := middleware.GetWarehouseID(c)

	adj, err := h.service.Create(c.Context(), warehouseID, &req, userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(ToResponse(adj))
}

func (h *Handler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	filter := bson.M{}
	warehouseID := middleware.GetWarehouseID(c)
	if !warehouseID.IsZero() {
		filter["warehouse_id"] = warehouseID
	}

	if pid := c.Query("productId"); pid != "" {
		filter["product_id"] = pid
	}
	if lid := c.Query("locationId"); lid != "" {
		filter["location_id"] = lid
	}

	// Date filters
	dateRange := bson.M{}
	if fromStr := c.Query("from"); fromStr != "" {
		from, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid 'from' date, use YYYY-MM-DD"})
		}
		dateRange["$gte"] = from
	}
	if toStr := c.Query("to"); toStr != "" {
		to, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid 'to' date, use YYYY-MM-DD"})
		}
		dateRange["$lte"] = to.Add(24*time.Hour - time.Nanosecond)
	}
	if len(dateRange) > 0 {
		filter["created_at"] = dateRange
	}

	items, total, err := h.service.List(c.Context(), filter, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var resp []*AdjustmentResponse
	for _, item := range items {
		resp = append(resp, ToResponse(item))
	}
	return c.JSON(fiber.Map{"data": resp, "total": total, "page": page, "limit": limit})
}
