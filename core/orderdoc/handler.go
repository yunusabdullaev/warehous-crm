package orderdoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jung-kurt/gofpdf"
	qrcode "github.com/skip2/go-qrcode"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"warehouse-crm/core/location"
	"warehouse-crm/core/order"
	"warehouse-crm/core/outbound"
	"warehouse-crm/core/picking"
	"warehouse-crm/core/product"
)

type orderQRPayload struct {
	Type    string `json:"type"`
	OrderID string `json:"orderId"`
	OrderNo string `json:"orderNo"`
}

type Handler struct {
	orderSvc    *order.Service
	pickingSvc  *picking.Service
	productSvc  *product.Service
	locationSvc *location.Service
	outboundSvc *outbound.Service
}

func NewHandler(
	orderSvc *order.Service,
	pickingSvc *picking.Service,
	productSvc *product.Service,
	locationSvc *location.Service,
	outboundSvc *outbound.Service,
) *Handler {
	return &Handler{
		orderSvc:    orderSvc,
		pickingSvc:  pickingSvc,
		productSvc:  productSvc,
		locationSvc: locationSvc,
		outboundSvc: outboundSvc,
	}
}

// PickListPDF renders an A4 PDF pick list for warehouse internal use.
// GET /orders/:id/picklist.pdf
func (h *Handler) PickListPDF(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	ord, err := h.orderSvc.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "order not found"})
	}

	tasks, err := h.pickingSvc.TasksByOrder(c.Context(), id)
	if err != nil {
		tasks = nil
	}

	// Load product & location names
	prodMap := make(map[string]string)
	locMap := make(map[string]string)
	for _, t := range tasks {
		if _, ok := prodMap[t.ProductID.Hex()]; !ok {
			p, e := h.productSvc.GetByID(c.Context(), t.ProductID)
			if e == nil {
				prodMap[t.ProductID.Hex()] = fmt.Sprintf("%s — %s", p.SKU, p.Name)
			} else {
				prodMap[t.ProductID.Hex()] = t.ProductID.Hex()
			}
		}
		if _, ok := locMap[t.LocationID.Hex()]; !ok {
			l, e := h.locationSvc.GetByID(c.Context(), t.LocationID)
			if e == nil {
				locMap[t.LocationID.Hex()] = fmt.Sprintf("%s-%s%s-%s", l.Zone, l.Aisle, l.Rack, l.Level)
			} else {
				locMap[t.LocationID.Hex()] = t.LocationID.Hex()
			}
		}
	}

	// Generate QR
	qrData, _ := json.Marshal(orderQRPayload{Type: "order", OrderID: ord.ID.Hex(), OrderNo: ord.OrderNo})
	qrPNG, _ := qrcode.Encode(string(qrData), qrcode.Medium, 200)

	// Build A4 PDF
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// QR top-right
	qrReader := bytes.NewReader(qrPNG)
	opts := gofpdf.ImageOptions{ImageType: "PNG"}
	pdf.RegisterImageOptionsReader("orderqr", opts, qrReader)
	pdf.ImageOptions("orderqr", 160, 15, 30, 30, false, opts, 0, "")

	// Title
	pdf.SetFont("Helvetica", "B", 18)
	pdf.CellFormat(130, 10, "PICK LIST", "", 1, "L", false, 0, "")
	pdf.Ln(2)

	// Order info
	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(40, 7, "Order No:", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(90, 7, ord.OrderNo, "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(40, 7, "Client:", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(90, 7, ord.ClientName, "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(40, 7, "Status:", "", 0, "L", false, 0, "")
	pdf.CellFormat(90, 7, ord.Status, "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(40, 7, "Created:", "", 0, "L", false, 0, "")
	pdf.CellFormat(90, 7, ord.CreatedAt.Format("2006-01-02 15:04"), "", 1, "L", false, 0, "")

	pdf.Ln(5)

	// Task table — grouped by location
	grouped := make(map[string][]*picking.PickTask)
	for _, t := range tasks {
		key := t.LocationID.Hex()
		grouped[key] = append(grouped[key], t)
	}

	// Table header
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetFillColor(230, 230, 230)
	pdf.CellFormat(55, 8, "Location", "1", 0, "C", true, 0, "")
	pdf.CellFormat(60, 8, "Product", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, "Planned", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, "Picked", "1", 0, "C", true, 0, "")
	pdf.CellFormat(15, 8, "Status", "1", 1, "C", true, 0, "")

	pdf.SetFont("Helvetica", "", 9)
	doneCount := 0
	totalCount := len(tasks)
	for locID, locTasks := range grouped {
		locLabel := locMap[locID]
		for i, t := range locTasks {
			loc := ""
			if i == 0 {
				loc = locLabel
			}
			status := t.Status
			if t.Status == "DONE" {
				doneCount++
			}
			pdf.CellFormat(55, 7, loc, "1", 0, "L", false, 0, "")
			pdf.CellFormat(60, 7, truncate(prodMap[t.ProductID.Hex()], 30), "1", 0, "L", false, 0, "")
			pdf.CellFormat(25, 7, fmt.Sprintf("%d", t.PlannedQty), "1", 0, "C", false, 0, "")
			pdf.CellFormat(25, 7, fmt.Sprintf("%d", t.PickedQty), "1", 0, "C", false, 0, "")
			pdf.CellFormat(15, 7, status, "1", 1, "C", false, 0, "")
		}
	}

	// Summary
	pdf.Ln(5)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(180, 8, fmt.Sprintf("Pick Completion: %d / %d tasks DONE", doneCount, totalCount), "", 1, "L", false, 0, "")

	// Footer
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "", 9)
	pdf.CellFormat(180, 6, fmt.Sprintf("Generated: %s", time.Now().Format("2006-01-02 15:04:05")), "", 1, "R", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "pdf generation failed"})
	}

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf(`inline; filename="picklist_%s.pdf"`, ord.OrderNo))
	return c.Send(buf.Bytes())
}

// DeliveryNotePDF renders an A4 delivery note (customer-facing).
// GET /orders/:id/deliverynote.pdf
func (h *Handler) DeliveryNotePDF(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	ord, err := h.orderSvc.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "order not found"})
	}

	if ord.Status != "SHIPPED" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "delivery note available only for shipped orders"})
	}

	// Get outbound records for this order
	outbounds, err := h.outboundSvc.ListByReference(c.Context(), fmt.Sprintf("ORDER:%s", ord.OrderNo))
	if err != nil {
		outbounds = nil
	}

	// Aggregate by product
	type shipItem struct {
		SKU  string
		Name string
		Qty  int
	}
	itemMap := make(map[string]*shipItem)
	for _, ob := range outbounds {
		pid := ob.ProductID.Hex()
		if _, ok := itemMap[pid]; !ok {
			p, e := h.productSvc.GetByID(c.Context(), ob.ProductID)
			sku, name := pid, pid
			if e == nil {
				sku = p.SKU
				name = p.Name
			}
			itemMap[pid] = &shipItem{SKU: sku, Name: name}
		}
		itemMap[pid].Qty += ob.Quantity
	}

	// QR
	qrData, _ := json.Marshal(orderQRPayload{Type: "order", OrderID: ord.ID.Hex(), OrderNo: ord.OrderNo})
	qrPNG, _ := qrcode.Encode(string(qrData), qrcode.Medium, 200)

	// Build A4 PDF
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// QR top-right
	qrReader := bytes.NewReader(qrPNG)
	opts := gofpdf.ImageOptions{ImageType: "PNG"}
	pdf.RegisterImageOptionsReader("dnqr", opts, qrReader)
	pdf.ImageOptions("dnqr", 160, 15, 30, 30, false, opts, 0, "")

	// Title
	pdf.SetFont("Helvetica", "B", 20)
	pdf.CellFormat(130, 12, "DELIVERY NOTE", "", 1, "L", false, 0, "")
	pdf.Ln(3)

	// Order info
	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(40, 7, "Order No:", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(90, 7, ord.OrderNo, "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(40, 7, "Client:", "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "B", 11)
	pdf.CellFormat(90, 7, ord.ClientName, "", 1, "L", false, 0, "")

	if ord.ShippedAt != nil {
		pdf.SetFont("Helvetica", "", 11)
		pdf.CellFormat(40, 7, "Shipped At:", "", 0, "L", false, 0, "")
		pdf.CellFormat(90, 7, ord.ShippedAt.Format("2006-01-02 15:04"), "", 1, "L", false, 0, "")
	}

	pdf.Ln(8)

	// Items table
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetFillColor(230, 230, 230)
	pdf.CellFormat(10, 8, "#", "1", 0, "C", true, 0, "")
	pdf.CellFormat(40, 8, "SKU", "1", 0, "C", true, 0, "")
	pdf.CellFormat(90, 8, "Product Name", "1", 0, "C", true, 0, "")
	pdf.CellFormat(30, 8, "Qty Shipped", "1", 1, "C", true, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	idx := 1
	totalQty := 0
	for _, item := range itemMap {
		pdf.CellFormat(10, 7, fmt.Sprintf("%d", idx), "1", 0, "C", false, 0, "")
		pdf.CellFormat(40, 7, item.SKU, "1", 0, "L", false, 0, "")
		pdf.CellFormat(90, 7, truncate(item.Name, 45), "1", 0, "L", false, 0, "")
		pdf.CellFormat(30, 7, fmt.Sprintf("%d", item.Qty), "1", 1, "C", false, 0, "")
		totalQty += item.Qty
		idx++
	}

	// Total row
	pdf.SetFont("Helvetica", "B", 10)
	pdf.CellFormat(140, 8, "TOTAL", "1", 0, "R", false, 0, "")
	pdf.CellFormat(30, 8, fmt.Sprintf("%d", totalQty), "1", 1, "C", false, 0, "")

	// Signature lines
	pdf.Ln(25)
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(80, 7, "________________________________", "", 0, "C", false, 0, "")
	pdf.CellFormat(10, 7, "", "", 0, "", false, 0, "")
	pdf.CellFormat(80, 7, "________________________________", "", 1, "C", false, 0, "")

	pdf.CellFormat(80, 7, "Delivered By", "", 0, "C", false, 0, "")
	pdf.CellFormat(10, 7, "", "", 0, "", false, 0, "")
	pdf.CellFormat(80, 7, "Received By", "", 1, "C", false, 0, "")

	// Footer
	pdf.Ln(15)
	pdf.SetFont("Helvetica", "", 8)
	pdf.CellFormat(170, 6, fmt.Sprintf("Generated: %s", time.Now().Format("2006-01-02 15:04:05")), "", 1, "R", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "pdf generation failed"})
	}

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf(`inline; filename="delivery_%s.pdf"`, ord.OrderNo))
	return c.Send(buf.Bytes())
}

// LabelPDF renders an A6 order label with QR.
// GET /orders/:id/label
func (h *Handler) LabelPDF(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	ord, err := h.orderSvc.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "order not found"})
	}

	// QR
	qrData, _ := json.Marshal(orderQRPayload{Type: "order", OrderID: ord.ID.Hex(), OrderNo: ord.OrderNo})
	qrPNG, _ := qrcode.Encode(string(qrData), qrcode.Medium, 300)

	// A6 PDF
	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		UnitStr: "mm",
		Size:    gofpdf.SizeType{Wd: 105, Ht: 148},
	})
	pdf.SetMargins(10, 10, 10)
	pdf.AddPage()

	// Order No big
	pdf.SetFont("Helvetica", "B", 28)
	pdf.CellFormat(85, 15, ord.OrderNo, "", 1, "C", false, 0, "")
	pdf.Ln(3)

	// Client name
	pdf.SetFont("Helvetica", "", 12)
	pdf.CellFormat(85, 8, ord.ClientName, "", 1, "C", false, 0, "")
	pdf.Ln(3)

	// Status
	pdf.SetFont("Helvetica", "B", 14)
	pdf.CellFormat(85, 8, ord.Status, "", 1, "C", false, 0, "")
	pdf.Ln(5)

	// QR
	qrReader := bytes.NewReader(qrPNG)
	opts := gofpdf.ImageOptions{ImageType: "PNG"}
	pdf.RegisterImageOptionsReader("olqr", opts, qrReader)
	pdf.ImageOptions("olqr", 22.5, pdf.GetY(), 60, 60, false, opts, 0, "")

	pdf.SetY(pdf.GetY() + 65)

	// ID
	pdf.SetFont("Courier", "", 7)
	pdf.CellFormat(85, 5, "ID: "+ord.ID.Hex(), "", 1, "C", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "pdf generation failed"})
	}

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf(`inline; filename="label_%s.pdf"`, ord.OrderNo))
	return c.Send(buf.Bytes())
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "."
}
