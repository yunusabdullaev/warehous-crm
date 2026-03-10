package picking

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetTasksByOrder returns all pick tasks for an order.
// GET /orders/:id/pick-tasks
func (h *Handler) GetTasksByOrder(c *fiber.Ctx) error {
	orderID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid order id"})
	}

	tasks, err := h.service.TasksByOrder(c.Context(), orderID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var resp []*PickTaskResponse
	for _, t := range tasks {
		resp = append(resp, TaskToResponse(t))
	}
	if resp == nil {
		resp = []*PickTaskResponse{}
	}
	return c.JSON(fiber.Map{"data": resp})
}

// MyTasks returns tasks assigned to the current user.
// GET /pick-tasks/my
func (h *Handler) MyTasks(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	tasks, err := h.service.TasksByAssignee(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var resp []*PickTaskResponse
	for _, t := range tasks {
		resp = append(resp, TaskToResponse(t))
	}
	if resp == nil {
		resp = []*PickTaskResponse{}
	}
	return c.JSON(fiber.Map{"data": resp})
}

// Assign sets the assignee on a task.
// POST /pick-tasks/:id/assign
func (h *Handler) Assign(c *fiber.Ctx) error {
	taskID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid task id"})
	}

	var req AssignRequest
	if err := c.BodyParser(&req); err != nil || req.AssignTo == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "assign_to is required"})
	}

	if err := h.service.Assign(c.Context(), taskID, req.AssignTo); err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "task not found or already completed"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"ok": true})
}

// Scan submits a pick scan event.
// POST /pick-tasks/:id/scan
func (h *Handler) Scan(c *fiber.Ctx) error {
	taskID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid task id"})
	}

	var req ScanRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}
	if req.Qty <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "qty must be positive"})
	}

	locationID, err := primitive.ObjectIDFromHex(req.LocationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid location_id"})
	}
	productID, err := primitive.ObjectIDFromHex(req.ProductID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid product_id"})
	}
	lotID, err := primitive.ObjectIDFromHex(req.LotID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid lot_id"})
	}

	userID := c.Locals("userID").(string)
	meta := PickEventMeta{Scanner: req.Scanner, Client: req.Client}

	task, err := h.service.Scan(c.Context(), taskID, locationID, productID, lotID, req.Qty, userID, meta)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "task not found"})
		}
		if errors.Is(err, ErrLocationMismatch) ||
			errors.Is(err, ErrProductMismatch) ||
			errors.Is(err, ErrLotMismatch) ||
			errors.Is(err, ErrQtyExceedsPlanned) ||
			errors.Is(err, ErrTaskNotPickable) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(TaskToResponse(task))
}

// GetTask returns a single task by ID.
// GET /pick-tasks/:id
func (h *Handler) GetTask(c *fiber.Ctx) error {
	taskID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid task id"})
	}

	task, err := h.service.GetTask(c.Context(), taskID)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "task not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(TaskToResponse(task))
}
