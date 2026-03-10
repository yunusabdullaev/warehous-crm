package stock

import (
	"context"
	"strconv"

	"warehouse-crm/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ReservationQuerier is a local interface to avoid import cycle with reservation package.
type ReservationQuerier interface {
	SumActiveByProduct(ctx context.Context, productID primitive.ObjectID) (int, error)
}

type Handler struct {
	service      *Service
	reservations ReservationQuerier
}

func NewHandler(service *Service, reservations ReservationQuerier) *Handler {
	return &Handler{service: service, reservations: reservations}
}

func (h *Handler) ListAll(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	warehouseID := middleware.GetWarehouseID(c)
	stocks, total, err := h.service.ListAll(c.Context(), warehouseID, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var resp []*StockResponse
	for _, s := range stocks {
		reserved, _ := h.reservations.SumActiveByProduct(c.Context(), s.ProductID)
		resp = append(resp, ToResponseWithReservation(s, reserved))
	}
	return c.JSON(fiber.Map{"data": resp, "total": total, "page": page, "limit": limit})
}

func (h *Handler) GetByProduct(c *fiber.Ctx) error {
	productID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid product id"})
	}

	stocks, err := h.service.ListByProduct(c.Context(), productID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	reserved, _ := h.reservations.SumActiveByProduct(c.Context(), productID)

	var resp []*StockResponse
	for _, s := range stocks {
		resp = append(resp, ToResponseWithReservation(s, reserved))
	}
	return c.JSON(resp)
}

func (h *Handler) GetByLocation(c *fiber.Ctx) error {
	locationID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid location id"})
	}

	stocks, err := h.service.ListByLocation(c.Context(), locationID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var resp []*StockResponse
	for _, s := range stocks {
		reserved, _ := h.reservations.SumActiveByProduct(c.Context(), s.ProductID)
		resp = append(resp, ToResponseWithReservation(s, reserved))
	}
	return c.JSON(resp)
}
