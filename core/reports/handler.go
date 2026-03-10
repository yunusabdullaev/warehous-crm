package reports

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

func (h *Handler) Movements(c *fiber.Ctx) error {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	groupBy := c.Query("groupBy", "day")
	includeAdjustments := c.Query("includeAdjustments", "false") == "true"

	if groupBy != "day" && groupBy != "week" && groupBy != "month" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "groupBy must be 'day', 'week', or 'month'",
		})
	}

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
		to = to.Add(24*time.Hour - time.Nanosecond)
	}

	warehouseID := middleware.GetWarehouseID(c)
	report, err := h.service.GetMovements(c.Context(), warehouseID, from, to, groupBy, includeAdjustments)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(report)
}

func (h *Handler) StockReport(c *fiber.Ctx) error {
	groupBy := c.Query("groupBy", "product")

	if groupBy != "zone" && groupBy != "rack" && groupBy != "product" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "groupBy must be 'zone', 'rack', or 'product'",
		})
	}

	warehouseID := middleware.GetWarehouseID(c)
	report, err := h.service.GetStockReport(c.Context(), warehouseID, groupBy)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(report)
}

func (h *Handler) OrderReport(c *fiber.Ctx) error {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	groupBy := c.Query("groupBy", "day")

	if groupBy != "day" && groupBy != "week" && groupBy != "month" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "groupBy must be 'day', 'week', or 'month'",
		})
	}

	var from, to time.Time
	var err error
	if fromStr != "" {
		from, err = time.Parse("2006-01-02", fromStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid 'from' date"})
		}
	}
	if toStr != "" {
		to, err = time.Parse("2006-01-02", toStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid 'to' date"})
		}
		to = to.Add(24*time.Hour - time.Nanosecond)
	}

	warehouseID := middleware.GetWarehouseID(c)
	report, err := h.service.GetOrderReport(c.Context(), warehouseID, from, to, groupBy)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(report)
}

func (h *Handler) PickingReport(c *fiber.Ctx) error {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	groupBy := c.Query("groupBy", "day")

	if groupBy != "day" && groupBy != "week" && groupBy != "month" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "groupBy must be 'day', 'week', or 'month'",
		})
	}

	var from, to time.Time
	var err error
	if fromStr != "" {
		from, err = time.Parse("2006-01-02", fromStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid 'from' date"})
		}
	}
	if toStr != "" {
		to, err = time.Parse("2006-01-02", toStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid 'to' date"})
		}
		to = to.Add(24*time.Hour - time.Nanosecond)
	}

	warehouseID := middleware.GetWarehouseID(c)
	report, err := h.service.GetPickingReport(c.Context(), warehouseID, from, to, groupBy)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(report)
}

func (h *Handler) ReturnsReport(c *fiber.Ctx) error {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	groupBy := c.Query("groupBy", "day")

	if groupBy != "day" && groupBy != "week" && groupBy != "month" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "groupBy must be 'day', 'week', or 'month'",
		})
	}

	var from, to time.Time
	var err error
	if fromStr != "" {
		from, err = time.Parse("2006-01-02", fromStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid 'from' date"})
		}
	}
	if toStr != "" {
		to, err = time.Parse("2006-01-02", toStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid 'to' date"})
		}
		to = to.Add(24*time.Hour - time.Nanosecond)
	}

	warehouseID := middleware.GetWarehouseID(c)
	report, err := h.service.GetReturnsReport(c.Context(), warehouseID, from, to, groupBy)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(report)
}

func (h *Handler) ExpiryReport(c *fiber.Ctx) error {
	daysStr := c.Query("days", "30")
	days := 30
	if d, err := time.ParseDuration(daysStr + "h"); err == nil {
		days = int(d.Hours() / 24)
	} else {
		for _, ch := range daysStr {
			if ch < '0' || ch > '9' {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "days must be a positive integer"})
			}
		}
		d := 0
		for _, ch := range daysStr {
			d = d*10 + int(ch-'0')
		}
		if d > 0 {
			days = d
		}
	}

	warehouseID := middleware.GetWarehouseID(c)
	report, err := h.service.GetExpiryReport(c.Context(), warehouseID, days)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(report)
}
