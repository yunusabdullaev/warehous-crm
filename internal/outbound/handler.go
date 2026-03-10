package outbound

import (
	"strconv"

	"warehouse-crm/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var req CreateOutboundRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	userID, _ := c.Locals("userID").(string)
	warehouseID := middleware.GetWarehouseID(c)

	out, err := h.service.Create(c.Context(), warehouseID, &req, userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(ToResponse(out))
}

func (h *Handler) GetByID(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	out, err := h.service.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "outbound record not found"})
	}
	return c.JSON(ToResponse(out))
}

func (h *Handler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	warehouseID := middleware.GetWarehouseID(c)
	items, total, err := h.service.List(c.Context(), warehouseID, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var resp []*OutboundResponse
	for _, item := range items {
		resp = append(resp, ToResponse(item))
	}
	return c.JSON(fiber.Map{"data": resp, "total": total, "page": page, "limit": limit})
}

func (h *Handler) Reverse(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	var req ReverseRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	userID, _ := c.Locals("userID").(string)
	userRole, _ := c.Locals("role").(string)

	out, err := h.service.Reverse(c.Context(), id, req.Reason, userID, userRole)
	if err != nil {
		switch err {
		case ErrAlreadyReversed:
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "this outbound record has already been reversed"})
		case ErrForbidden:
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "you can only reverse your own records created within the last 24 hours"})
		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
	}

	return c.JSON(ToResponse(out))
}
