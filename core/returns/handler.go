package returns

import (
	"strconv"
	"time"

	"warehouse-crm/core/middleware"

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

// Create — POST /returns
func (h *Handler) Create(c *fiber.Ctx) error {
	var req CreateReturnRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.OrderID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "order_id is required"})
	}

	userID := c.Locals("userID").(string)
	warehouseID := middleware.GetWarehouseID(c)
	ret, err := h.service.Create(c.Context(), warehouseID, &req, userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(ret)
}

// List — GET /returns
func (h *Handler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	filter := bson.M{}
	warehouseID := middleware.GetWarehouseID(c)
	if !warehouseID.IsZero() {
		filter["warehouse_id"] = warehouseID
	}
	if status := c.Query("status"); status != "" {
		filter["status"] = status
	}
	if orderID := c.Query("orderId"); orderID != "" {
		oid, err := primitive.ObjectIDFromHex(orderID)
		if err == nil {
			filter["order_id"] = oid
		}
	}
	if from := c.Query("from"); from != "" {
		t, err := time.Parse("2006-01-02", from)
		if err == nil {
			if filter["created_at"] == nil {
				filter["created_at"] = bson.M{}
			}
			filter["created_at"].(bson.M)["$gte"] = t
		}
	}
	if to := c.Query("to"); to != "" {
		t, err := time.Parse("2006-01-02", to)
		if err == nil {
			t = t.Add(24*time.Hour - time.Nanosecond)
			if filter["created_at"] == nil {
				filter["created_at"] = bson.M{}
			}
			filter["created_at"].(bson.M)["$lte"] = t
		}
	}

	returns, total, err := h.service.List(c.Context(), filter, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if returns == nil {
		returns = []*Return{}
	}
	return c.JSON(fiber.Map{
		"data":  returns,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GetByID — GET /returns/:id
func (h *Handler) GetByID(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	result, err := h.service.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "return not found"})
	}
	return c.JSON(result)
}

// AddItem — POST /returns/:id/items
func (h *Handler) AddItem(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	var req AddItemRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	userID := c.Locals("userID").(string)
	item, err := h.service.AddItem(c.Context(), id, &req, userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(item)
}

// Receive — POST /returns/:id/receive
func (h *Handler) Receive(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	userID := c.Locals("userID").(string)
	ret, err := h.service.Receive(c.Context(), id, userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(ret)
}

// Cancel — POST /returns/:id/cancel
func (h *Handler) Cancel(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	userID := c.Locals("userID").(string)
	ret, err := h.service.Cancel(c.Context(), id, userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(ret)
}
