package csvio

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Handler handles CSV import/export HTTP endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a new csvio handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ImportProducts handles POST /import/products (multipart/form-data, field="file").
func (h *Handler) ImportProducts(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "file field is required"})
	}

	f, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to open uploaded file"})
	}
	defer f.Close()

	report, err := h.service.ImportProducts(c.Context(), f)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(report)
}

// ImportLocations handles POST /import/locations (multipart/form-data, field="file").
func (h *Handler) ImportLocations(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "file field is required"})
	}

	f, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to open uploaded file"})
	}
	defer f.Close()

	report, err := h.service.ImportLocations(c.Context(), f)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(report)
}

// ExportProducts handles GET /export/products → text/csv.
func (h *Handler) ExportProducts(c *fiber.Ctx) error {
	data, err := h.service.ExportProductsToBytes(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	filename := fmt.Sprintf("products_%s.csv", time.Now().Format("20060102_150405"))
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	return c.Send(data)
}

// ExportLocations handles GET /export/locations → text/csv.
func (h *Handler) ExportLocations(c *fiber.Ctx) error {
	data, err := h.service.ExportLocationsToBytes(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	filename := fmt.Sprintf("locations_%s.csv", time.Now().Format("20060102_150405"))
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	return c.Send(data)
}
