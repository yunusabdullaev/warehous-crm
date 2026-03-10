package lot

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Handler exposes HTTP endpoints for lot management.
type Handler struct {
	svc *Service
}

// NewHandler creates a new lot handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Create handles POST /lots
func (h *Handler) Create(c *fiber.Ctx) error {
	var req struct {
		ProductID string  `json:"product_id"`
		LotNo     string  `json:"lot_no"`
		ExpDate   *string `json:"exp_date,omitempty"`
		MfgDate   *string `json:"mfg_date,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	productID, err := primitive.ObjectIDFromHex(req.ProductID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid product_id"})
	}

	var expDate, mfgDate *time.Time
	if req.ExpDate != nil && *req.ExpDate != "" {
		t, err := time.Parse("2006-01-02", *req.ExpDate)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid exp_date format, use YYYY-MM-DD"})
		}
		expDate = &t
	}
	if req.MfgDate != nil && *req.MfgDate != "" {
		t, err := time.Parse("2006-01-02", *req.MfgDate)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid mfg_date format, use YYYY-MM-DD"})
		}
		mfgDate = &t
	}

	lot, err := h.svc.Create(c.Context(), productID, req.LotNo, expDate, mfgDate)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(toLotResponse(lot))
}

// GetByID handles GET /lots/:id
func (h *Handler) GetByID(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	lot, err := h.svc.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "lot not found"})
	}
	return c.JSON(toLotResponse(lot))
}

// List handles GET /lots?productId=&page=&limit=
func (h *Handler) List(c *fiber.Ctx) error {
	productIDStr := c.Query("productId")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	if productIDStr != "" {
		productID, err := primitive.ObjectIDFromHex(productIDStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid productId"})
		}
		lots, err := h.svc.ListByProduct(c.Context(), productID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		var resp []fiber.Map
		for _, l := range lots {
			resp = append(resp, toLotResponse(l))
		}
		return c.JSON(fiber.Map{"data": resp, "total": len(lots)})
	}

	lots, total, err := h.svc.ListAll(c.Context(), page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	var resp []fiber.Map
	for _, l := range lots {
		resp = append(resp, toLotResponse(l))
	}
	return c.JSON(fiber.Map{"data": resp, "total": total, "page": page, "limit": limit})
}

func toLotResponse(l *Lot) fiber.Map {
	m := fiber.Map{
		"id":         l.ID.Hex(),
		"product_id": l.ProductID.Hex(),
		"lot_no":     l.LotNo,
		"created_at": l.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if l.ExpDate != nil {
		m["exp_date"] = l.ExpDate.Format("2006-01-02")
	}
	if l.MfgDate != nil {
		m["mfg_date"] = l.MfgDate.Format("2006-01-02")
	}
	return m
}
