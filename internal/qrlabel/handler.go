package qrlabel

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/jung-kurt/gofpdf"
	qrcode "github.com/skip2/go-qrcode"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"warehouse-crm/internal/location"
)

// QR payload matches the spec exactly
type qrPayload struct {
	Type       string `json:"type"`
	LocationID string `json:"locationId"`
	Zone       string `json:"zone"`
	Rack       string `json:"rack"`
	Shelf      string `json:"shelf"`
}

// Handler serves QR code PNGs and printable PDF labels.
type Handler struct {
	locationSvc *location.Service
}

func NewHandler(locationSvc *location.Service) *Handler {
	return &Handler{locationSvc: locationSvc}
}

// QRCode returns a 300×300 PNG QR code for the location.
// GET /locations/:id/qr
func (h *Handler) QRCode(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	loc, err := h.locationSvc.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "location not found"})
	}

	payload := qrPayload{
		Type:       "location",
		LocationID: loc.ID.Hex(),
		Zone:       loc.Zone,
		Rack:       loc.Rack,
		Shelf:      loc.Level, // model uses "Level", spec says "shelf"
	}

	data, _ := json.Marshal(payload)

	png, err := qrcode.Encode(string(data), qrcode.Medium, 300)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "qr generation failed"})
	}

	c.Set("Content-Type", "image/png")
	c.Set("Content-Disposition", fmt.Sprintf(`inline; filename="qr_%s.png"`, loc.Code))
	return c.Send(png)
}

// Label returns a printable A6 PDF containing the location label.
// GET /locations/:id/label
func (h *Handler) Label(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	loc, err := h.locationSvc.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "location not found"})
	}

	// Generate QR PNG for embedding
	payload := qrPayload{
		Type:       "location",
		LocationID: loc.ID.Hex(),
		Zone:       loc.Zone,
		Rack:       loc.Rack,
		Shelf:      loc.Level,
	}
	data, _ := json.Marshal(payload)
	png, err := qrcode.Encode(string(data), qrcode.Medium, 300)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "qr generation failed"})
	}

	// Build A6 PDF (105mm × 148mm)
	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		UnitStr: "mm",
		Size:    gofpdf.SizeType{Wd: 105, Ht: 148},
	})
	pdf.SetMargins(10, 10, 10)
	pdf.AddPage()

	// Title: Zone-Rack-Shelf
	label := fmt.Sprintf("%s-%s-%s", loc.Zone, loc.Rack, loc.Level)
	pdf.SetFont("Helvetica", "B", 28)
	pdf.CellFormat(85, 15, label, "", 1, "C", false, 0, "")

	pdf.Ln(5)

	// Location name
	pdf.SetFont("Helvetica", "", 12)
	pdf.CellFormat(85, 8, loc.Name, "", 1, "C", false, 0, "")

	pdf.Ln(5)

	// QR code image
	qrReader := bytes.NewReader(png)
	opts := gofpdf.ImageOptions{ImageType: "PNG"}
	pdf.RegisterImageOptionsReader("qr", opts, qrReader)
	pdf.ImageOptions("qr", 22.5, pdf.GetY(), 60, 60, false, opts, 0, "")

	pdf.SetY(pdf.GetY() + 65)

	// Human-readable ID
	pdf.SetFont("Courier", "", 8)
	pdf.CellFormat(85, 6, "ID: "+loc.ID.Hex(), "", 1, "C", false, 0, "")

	// Location code
	pdf.SetFont("Courier", "", 8)
	pdf.CellFormat(85, 6, "Code: "+loc.Code, "", 1, "C", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "pdf generation failed"})
	}

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf(`inline; filename="label_%s.pdf"`, loc.Code))
	return c.Send(buf.Bytes())
}
