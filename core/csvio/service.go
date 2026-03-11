package csvio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Service handles Excel import/export for products and locations.
type Service struct {
	products  *mongo.Collection
	locations *mongo.Collection
}

// NewService creates a new excelio service.
func NewService(products, locations *mongo.Collection) *Service {
	return &Service{products: products, locations: locations}
}

// ── Products Import ─────────────────────────────────────────────

// ImportProducts reads Excel from r, validates, and upserts products by SKU.
// Expected Header: SKU, Name, Unit, Description
func (s *Service) ImportProducts(ctx context.Context, r io.Reader) (*ImportReport, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("invalid excel file: %w", err)
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet: %w", err)
	}

	if len(rows) < 1 {
		return nil, fmt.Errorf("excel file is empty")
	}

	report := &ImportReport{Errors: []RowError{}}
	header := rows[0]
	colIdx := map[string]int{}
	for i, h := range header {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}

	reqCols := []string{"sku", "name", "unit"}
	for _, c := range reqCols {
		if _, ok := colIdx[c]; !ok {
			return nil, fmt.Errorf("missing required column: %s", c)
		}
	}

	batch := make([]mongo.WriteModel, 0, 500)
	seenSKUs := map[string]int{}

	for rowNum := 2; rowNum <= len(rows); rowNum++ {
		rowData := rows[rowNum-1]
		getCol := func(name string) string {
			if idx, ok := colIdx[name]; ok && idx < len(rowData) {
				return strings.TrimSpace(rowData[idx])
			}
			return ""
		}

		sku := getCol("sku")
		name := getCol("name")
		unit := strings.ToLower(getCol("unit"))
		desc := getCol("description")

		if sku == "" && name == "" {
			continue // Skip truly empty rows
		}

		valid := true
		if sku == "" {
			report.Failed++
			report.Errors = append(report.Errors, RowError{Row: rowNum, Field: "sku", Message: "required"})
			valid = false
		}
		if name == "" {
			if valid {
				report.Failed++
			}
			report.Errors = append(report.Errors, RowError{Row: rowNum, Field: "name", Message: "required"})
			valid = false
		}
		if unit != "kg" && unit != "pcs" && unit != "box" {
			if valid {
				report.Failed++
			}
			report.Errors = append(report.Errors, RowError{Row: rowNum, Field: "unit", Message: "must be kg, pcs, or box"})
			valid = false
		}

		if !valid {
			continue
		}

		if firstRow, exists := seenSKUs[sku]; exists {
			report.Errors = append(report.Errors, RowError{Row: rowNum, Field: "sku", Message: fmt.Sprintf("duplicate in file (first at row %d)", firstRow)})
			report.Skipped++
			continue
		}
		seenSKUs[sku] = rowNum

		doc := bson.M{
			"sku":         sku,
			"name":        name,
			"unit":        unit,
			"description": desc,
			"updated_at":  time.Now().UTC(),
		}

		filter := bson.M{"sku": sku}
		// Try to see if it exists to count Inserted vs Updated manually or just let BulkWrite handle it
		// For simplicity, we use ReplaceOne with upsert
		batch = append(batch, mongo.NewUpdateOneModel().
			SetFilter(filter).
			SetUpdate(bson.M{
				"$set": doc,
				"$setOnInsert": bson.M{
					"created_at": time.Now().UTC(),
					"_id":        primitive.NewObjectID(),
				},
			}).
			SetUpsert(true))

		if len(batch) >= 500 {
			res, err := s.products.BulkWrite(ctx, batch, options.BulkWrite().SetOrdered(false))
			if err != nil {
				return nil, err
			}
			report.Inserted += int(res.UpsertedCount)
			report.Updated += int(res.ModifiedCount)
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		res, err := s.products.BulkWrite(ctx, batch, options.BulkWrite().SetOrdered(false))
		if err != nil {
			return nil, err
		}
		report.Inserted += int(res.UpsertedCount)
		report.Updated += int(res.ModifiedCount)
	}

	return report, nil
}

// ── Locations Import ─────────────────────────────────────────────

func (s *Service) ImportLocations(ctx context.Context, r io.Reader) (*ImportReport, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("invalid excel file: %w", err)
	}
	defer f.Close()

	rows, err := f.GetRows(f.GetSheetName(0))
	if err != nil {
		return nil, err
	}
	if len(rows) < 1 {
		return nil, fmt.Errorf("empty file")
	}

	report := &ImportReport{Errors: []RowError{}}
	header := rows[0]
	colIdx := map[string]int{}
	for i, h := range header {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}

	for _, c := range []string{"zone", "rack", "level"} {
		if _, ok := colIdx[c]; !ok {
			return nil, fmt.Errorf("missing required column: %s", c)
		}
	}

	seenKeys := map[string]int{}
	var toInsert []interface{}

	for rowNum := 2; rowNum <= len(rows); rowNum++ {
		rowData := rows[rowNum-1]
		getCol := func(name string) string {
			if idx, ok := colIdx[name]; ok && idx < len(rowData) {
				return strings.TrimSpace(rowData[idx])
			}
			return ""
		}

		zone := getCol("zone")
		rack := getCol("rack")
		level := getCol("level")

		if zone == "" && rack == "" && level == "" {
			continue
		}

		valid := true
		if zone == "" || rack == "" || level == "" {
			report.Failed++
			report.Errors = append(report.Errors, RowError{Row: rowNum, Message: "zone, rack, level are all required"})
			valid = false
		}
		if !valid {
			continue
		}

		key := fmt.Sprintf("%s|%s|%s", zone, rack, level)
		if _, exists := seenKeys[key]; exists {
			report.Skipped++
			continue
		}
		seenKeys[key] = rowNum

		// Check DB
		count, _ := s.locations.CountDocuments(ctx, bson.M{"zone": zone, "rack": rack, "level": level})
		if count > 0 {
			report.Skipped++
			continue
		}

		now := time.Now().UTC()
		toInsert = append(toInsert, bson.M{
			"_id":        primitive.NewObjectID(),
			"code":       fmt.Sprintf("%s-%s-%s", zone, rack, level),
			"name":       fmt.Sprintf("%s / Rack %s / Level %s", zone, rack, level),
			"zone":       zone,
			"rack":       rack,
			"level":      level,
			"created_at": now,
			"updated_at": now,
		})

		if len(toInsert) >= 500 {
			res, err := s.locations.InsertMany(ctx, toInsert, options.InsertMany().SetOrdered(false))
			if err != nil {
				return nil, err
			}
			report.Inserted += len(res.InsertedIDs)
			toInsert = toInsert[:0]
		}
	}

	if len(toInsert) > 0 {
		res, err := s.locations.InsertMany(ctx, toInsert, options.InsertMany().SetOrdered(false))
		if err != nil {
			return nil, err
		}
		report.Inserted += len(res.InsertedIDs)
	}

	return report, nil
}

// ── Exports ──────────────────────────────────────────────────────

func (s *Service) ExportProductsToBytes(ctx context.Context) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Products"
	f.SetSheetName("Sheet1", sheet)

	f.SetCellValue(sheet, "A1", "SKU")
	f.SetCellValue(sheet, "B1", "Name")
	f.SetCellValue(sheet, "C1", "Unit")
	f.SetCellValue(sheet, "D1", "Description")

	cursor, err := s.products.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "sku", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	row := 2
	for cursor.Next(ctx) {
		var p struct {
			SKU, Name, Unit, Description string
		}
		if err := cursor.Decode(&p); err != nil {
			continue
		}
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), p.SKU)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), p.Name)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), p.Unit)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), p.Description)
		row++
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *Service) ExportLocationsToBytes(ctx context.Context) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Locations"
	f.SetSheetName("Sheet1", sheet)

	f.SetCellValue(sheet, "A1", "Zone")
	f.SetCellValue(sheet, "B1", "Rack")
	f.SetCellValue(sheet, "C1", "Level")

	cursor, err := s.locations.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "zone", Value: 1}, {Key: "rack", Value: 1}, {Key: "level", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	row := 2
	for cursor.Next(ctx) {
		var l struct {
			Zone, Rack, Level string
		}
		if err := cursor.Decode(&l); err != nil {
			continue
		}
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), l.Zone)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), l.Rack)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), l.Level)
		row++
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
