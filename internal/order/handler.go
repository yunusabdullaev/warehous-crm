package order

import (
	"errors"
	"strconv"
	"strings"

	"warehouse-crm/internal/middleware"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	pickingPkg "warehouse-crm/internal/picking"
	reservationPkg "warehouse-crm/internal/reservation"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(c *fiber.Ctx) error {
	var req CreateOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	userID := c.Locals("userID").(string)
	warehouseID := middleware.GetWarehouseID(c)
	order, err := h.service.Create(c.Context(), warehouseID, &req, userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(ToResponse(order))
}

func (h *Handler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	filter := bson.M{}
	warehouseID := middleware.GetWarehouseID(c)
	if !warehouseID.IsZero() {
		filter["warehouse_id"] = warehouseID
	}
	if v := c.Query("status"); v != "" {
		filter["status"] = strings.ToUpper(v)
	}
	if v := c.Query("client"); v != "" {
		filter["client_name"] = bson.M{"$regex": v, "$options": "i"}
	}

	orders, total, err := h.service.List(c.Context(), filter, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var resp []*OrderResponse
	for _, o := range orders {
		resp = append(resp, ToResponse(o))
	}
	return c.JSON(fiber.Map{"data": resp, "total": total, "page": page, "limit": limit})
}

func (h *Handler) GetByID(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	order, err := h.service.GetByID(c.Context(), id)
	if err != nil {
		if err == ErrOrderNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "order not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(ToResponse(order))
}

func (h *Handler) Update(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var req UpdateOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	userID := c.Locals("userID").(string)
	order, err := h.service.Update(c.Context(), id, &req, userID)
	if err != nil {
		if err == ErrOrderNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "order not found"})
		}
		if err == ErrNotDraft {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "can only update DRAFT orders"})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(ToResponse(order))
}

func (h *Handler) Confirm(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	userID := c.Locals("userID").(string)
	order, err := h.service.Confirm(c.Context(), id, userID)
	if err != nil {
		if err == ErrInvalidTransition {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "order is not in DRAFT status"})
		}
		if errors.Is(err, reservationPkg.ErrInsufficientStock) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(ToResponse(order))
}

func (h *Handler) Cancel(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	userID := c.Locals("userID").(string)
	role := c.Locals("role").(string)
	order, err := h.service.Cancel(c.Context(), id, userID, role)
	if err != nil {
		if err == ErrInvalidTransition {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "cannot cancel order in current status"})
		}
		if err == ErrAdminRequired {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "only admin can cancel PICKING orders"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(ToResponse(order))
}

func (h *Handler) StartPick(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	userID := c.Locals("userID").(string)
	order, err := h.service.StartPick(c.Context(), id, userID)
	if err != nil {
		if err == ErrInvalidTransition {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "order is not in CONFIRMED status"})
		}
		if errors.Is(err, pickingPkg.ErrInsufficientStock) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(ToResponse(order))
}

func (h *Handler) Ship(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	userID := c.Locals("userID").(string)
	order, err := h.service.Ship(c.Context(), id, userID)
	if err != nil {
		if err == ErrInvalidTransition {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "order is not in PICKING status"})
		}
		if err == ErrPickingNotComplete {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "all pick tasks must be completed before shipping"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(ToResponse(order))
}
