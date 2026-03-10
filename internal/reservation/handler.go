package reservation

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

	filter := bson.M{}
	if v := c.Query("orderId"); v != "" {
		oid, err := primitive.ObjectIDFromHex(v)
		if err == nil {
			filter["order_id"] = oid
		}
	}
	if v := c.Query("productId"); v != "" {
		pid, err := primitive.ObjectIDFromHex(v)
		if err == nil {
			filter["product_id"] = pid
		}
	}
	if v := c.Query("status"); v != "" {
		filter["status"] = v
	}

	reservations, total, err := h.service.List(c.Context(), filter, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var resp []*ReservationResponse
	for _, r := range reservations {
		resp = append(resp, ToResponse(r))
	}
	return c.JSON(fiber.Map{"data": resp, "total": total, "page": page, "limit": limit})
}

func (h *Handler) Release(c *fiber.Ctx) error {
	var req ReleaseRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if req.ReservationID == "" || req.Reason == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "reservation_id and reason are required"})
	}

	rid, err := primitive.ObjectIDFromHex(req.ReservationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid reservation_id"})
	}

	userID := c.Locals("userID").(string)
	if err := h.service.ReleaseOne(c.Context(), rid, userID, req.Reason); err != nil {
		if err == ErrAlreadyReleased {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "reservation already released"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"message": "reservation released"})
}
