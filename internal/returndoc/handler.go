package returndoc

import (
	"bytes"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jung-kurt/gofpdf"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"warehouse-crm/internal/product"
	"warehouse-crm/internal/returns"
)

type Handler struct {
	returnsSvc *returns.Service
	productSvc *product.Service
}

func NewHandler(returnsSvc *returns.Service, productSvc *product.Service) *Handler {
	return &Handler{returnsSvc: returnsSvc, productSvc: productSvc}
}

// NotePDF renders an A4 return note PDF.
// GET /returns/:id/note.pdf
func (h *Handler) NotePDF(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	result, err := h.returnsSvc.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "return not found"})
	}

	ret := result.Return
	items := result.Items

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// ── Header ──
	pdf.SetFont("Arial", "B", 18)
	pdf.CellFormat(0, 10, "RETURN NOTE", "", 1, "C", false, 0, "")
	pdf.Ln(4)

	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(100, 100, 100)
	pdf.CellFormat(0, 5, fmt.Sprintf("Generated: %s", time.Now().UTC().Format("2006-01-02 15:04 UTC")), "", 1, "C", false, 0, "")
	pdf.Ln(8)

	// ── Info table ──
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(40, 7, "RMA No:", "", 0, "", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(0, 7, ret.RMANo, "", 1, "", false, 0, "")

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(40, 7, "Order No:", "", 0, "", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(0, 7, ret.OrderNo, "", 1, "", false, 0, "")

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(40, 7, "Client:", "", 0, "", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(0, 7, ret.ClientName, "", 1, "", false, 0, "")

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(40, 7, "Status:", "", 0, "", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(0, 7, ret.Status, "", 1, "", false, 0, "")

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(40, 7, "Created:", "", 0, "", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(0, 7, ret.CreatedAt.Format("2006-01-02 15:04"), "", 1, "", false, 0, "")

	if ret.Notes != "" {
		pdf.SetFont("Arial", "B", 11)
		pdf.CellFormat(40, 7, "Notes:", "", 0, "", false, 0, "")
		pdf.SetFont("Arial", "", 11)
		pdf.CellFormat(0, 7, truncate(ret.Notes, 80), "", 1, "", false, 0, "")
	}
	pdf.Ln(8)

	// ── Items table ──
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 7, "Return Items", "", 1, "", false, 0, "")
	pdf.Ln(2)

	// Header row
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(240, 240, 240)
	pdf.CellFormat(10, 7, "#", "1", 0, "C", true, 0, "")
	pdf.CellFormat(30, 7, "SKU", "1", 0, "", true, 0, "")
	pdf.CellFormat(55, 7, "Product", "1", 0, "", true, 0, "")
	pdf.CellFormat(15, 7, "Qty", "1", 0, "C", true, 0, "")
	pdf.CellFormat(30, 7, "Disposition", "1", 0, "C", true, 0, "")
	pdf.CellFormat(40, 7, "Note", "1", 1, "", true, 0, "")

	// Data rows
	pdf.SetFont("Arial", "", 9)
	for i, item := range items {
		sku := item.ProductID.Hex()[:8]
		prodName := "Unknown"

		prod, err := h.productSvc.GetByID(c.Context(), item.ProductID)
		if err == nil {
			sku = prod.SKU
			prodName = prod.Name
		}

		pdf.CellFormat(10, 7, fmt.Sprintf("%d", i+1), "1", 0, "C", false, 0, "")
		pdf.CellFormat(30, 7, truncate(sku, 15), "1", 0, "", false, 0, "")
		pdf.CellFormat(55, 7, truncate(prodName, 30), "1", 0, "", false, 0, "")
		pdf.CellFormat(15, 7, fmt.Sprintf("%d", item.Qty), "1", 0, "C", false, 0, "")
		pdf.CellFormat(30, 7, item.Disposition, "1", 0, "C", false, 0, "")
		pdf.CellFormat(40, 7, truncate(item.Note, 20), "1", 1, "", false, 0, "")
	}
	pdf.Ln(20)

	// ── Signature lines ──
	pdf.SetFont("Arial", "", 10)
	y := pdf.GetY()
	pdf.Line(15, y, 90, y)
	pdf.Line(110, y, 185, y)
	pdf.Ln(2)
	pdf.CellFormat(90, 5, "Warehouse Representative", "", 0, "C", false, 0, "")
	pdf.CellFormat(0, 5, "Customer", "", 1, "C", false, 0, "")
	pdf.Ln(5)
	pdf.SetFont("Arial", "", 8)
	pdf.CellFormat(90, 5, "Name: ___________________  Date: ________", "", 0, "C", false, 0, "")
	pdf.CellFormat(0, 5, "Name: ___________________  Date: ________", "", 1, "C", false, 0, "")

	// Output
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "pdf generation failed"})
	}

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf("inline; filename=%s.pdf", ret.RMANo))
	return c.Send(buf.Bytes())
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
