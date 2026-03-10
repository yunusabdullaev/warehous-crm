package csvio

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Service handles CSV import/export for products and locations.
type Service struct {
	products  *mongo.Collection
	locations *mongo.Collection
}

// NewService creates a new csvio service.
func NewService(products, locations *mongo.Collection) *Service {
	return &Service{products: products, locations: locations}
}

// ── helpers ─────────────────────────────────────────────────────

// stripBOM removes a UTF-8 BOM from the first field if present.
func stripBOM(s string) string {
	return strings.TrimPrefix(s, "\xEF\xBB\xBF")
}

func trimAll(rec []string) {
	for i := range rec {
		rec[i] = strings.TrimSpace(rec[i])
	}
}

// ── Products Import ─────────────────────────────────────────────

// ImportProducts streams CSV from r, validates, and upserts products by SKU.
// Expected CSV header: sku,name,unit,description
func (s *Service) ImportProducts(ctx context.Context, r io.Reader) (*ImportReport, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	reader.ReuseRecord = true
	reader.FieldsPerRecord = -1 // variable

	report := &ImportReport{Errors: []RowError{}}

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}
	if len(header) > 0 {
		header[0] = stripBOM(header[0])
	}
	trimAll(header)

	colIdx := map[string]int{}
	for i, h := range header {
		colIdx[strings.ToLower(h)] = i
	}
	reqCols := []string{"sku", "name", "unit"}
	for _, c := range reqCols {
		if _, ok := colIdx[c]; !ok {
			return nil, fmt.Errorf("missing required column: %s", c)
		}
	}

	type bulkItem struct {
		sku  string
		doc  bson.M
		isUp bool // true = update existing
	}

	batch := make([]bulkItem, 0, 500)
	seenSKUs := map[string]int{} // sku -> first row
	rowNum := 1                  // header is row 1

	flushBatch := func() error {
		if len(batch) == 0 {
			return nil
		}
		models := make([]mongo.WriteModel, 0, len(batch))
		for _, item := range batch {
			filter := bson.M{"sku": item.sku}
			if item.isUp {
				models = append(models, mongo.NewUpdateOneModel().
					SetFilter(filter).
					SetUpdate(bson.M{"$set": item.doc}))
			} else {
				item.doc["_id"] = primitive.NewObjectID()
				item.doc["created_at"] = time.Now().UTC()
				item.doc["updated_at"] = time.Now().UTC()
				models = append(models, mongo.NewUpdateOneModel().
					SetFilter(filter).
					SetUpdate(bson.M{"$setOnInsert": item.doc}).
					SetUpsert(true))
			}
		}
		opts := options.BulkWrite().SetOrdered(false)
		result, err := s.products.BulkWrite(ctx, models, opts)
		if err != nil {
			return err
		}
		report.Inserted += int(result.UpsertedCount)
		report.Updated += int(result.ModifiedCount)
		batch = batch[:0]
		return nil
	}

	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		rowNum++
		if err != nil {
			report.Failed++
			report.Errors = append(report.Errors, RowError{Row: rowNum, Field: "", Message: fmt.Sprintf("parse error: %v", err)})
			continue
		}

		// Make a copy since ReuseRecord is true
		row := make([]string, len(rec))
		copy(row, rec)
		trimAll(row)

		getCol := func(name string) string {
			if idx, ok := colIdx[name]; ok && idx < len(row) {
				return row[idx]
			}
			return ""
		}

		sku := getCol("sku")
		name := getCol("name")
		unit := strings.ToLower(getCol("unit"))
		desc := getCol("description")

		// Validate
		valid := true
		if sku == "" {
			report.Failed++
			report.Errors = append(report.Errors, RowError{Row: rowNum, Field: "sku", Message: "required"})
			valid = false
		} else if len(sku) > 64 {
			report.Failed++
			report.Errors = append(report.Errors, RowError{Row: rowNum, Field: "sku", Message: "must be <= 64 chars"})
			valid = false
		}
		if name == "" {
			if valid {
				report.Failed++
			}
			report.Errors = append(report.Errors, RowError{Row: rowNum, Field: "name", Message: "required"})
			valid = false
		}
		if unit != "kg" && unit != "pcs" {
			if valid {
				report.Failed++
			}
			report.Errors = append(report.Errors, RowError{Row: rowNum, Field: "unit", Message: "must be kg or pcs"})
			valid = false
		}
		if !valid {
			continue
		}

		// Duplicate SKU within the same file
		if firstRow, exists := seenSKUs[sku]; exists {
			report.Errors = append(report.Errors, RowError{Row: rowNum, Field: "sku", Message: fmt.Sprintf("duplicate in file (first at row %d)", firstRow)})
			report.Skipped++
			continue
		}
		seenSKUs[sku] = rowNum

		// Check DB for existing
		var existing bson.M
		err = s.products.FindOne(ctx, bson.M{"sku": sku}).Decode(&existing)
		isUpdate := err == nil

		doc := bson.M{
			"sku":         sku,
			"name":        name,
			"unit":        unit,
			"description": desc,
			"updated_at":  time.Now().UTC(),
		}

		if isUpdate {
			batch = append(batch, bulkItem{sku: sku, doc: doc, isUp: true})
		} else {
			batch = append(batch, bulkItem{sku: sku, doc: doc, isUp: false})
		}

		if len(batch) >= 500 {
			if err := flushBatch(); err != nil {
				return nil, fmt.Errorf("batch write error: %w", err)
			}
		}
	}

	// Flush remaining
	if err := flushBatch(); err != nil {
		return nil, fmt.Errorf("batch write error: %w", err)
	}

	return report, nil
}

// ── Locations Import ─────────────────────────────────────────────

// ImportLocations streams CSV from r, validates, and inserts locations.
// Skips if (zone,rack,level) already exists.
// Expected CSV header: zone,rack,level
func (s *Service) ImportLocations(ctx context.Context, r io.Reader) (*ImportReport, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true
	reader.ReuseRecord = true
	reader.FieldsPerRecord = -1

	report := &ImportReport{Errors: []RowError{}}

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}
	if len(header) > 0 {
		header[0] = stripBOM(header[0])
	}
	trimAll(header)

	colIdx := map[string]int{}
	for i, h := range header {
		colIdx[strings.ToLower(h)] = i
	}
	for _, c := range []string{"zone", "rack", "level"} {
		if _, ok := colIdx[c]; !ok {
			return nil, fmt.Errorf("missing required column: %s", c)
		}
	}

	type locRow struct {
		zone, rack, level string
	}

	seenKeys := map[string]int{}
	var toInsert []interface{}
	rowNum := 1

	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		rowNum++
		if err != nil {
			report.Failed++
			report.Errors = append(report.Errors, RowError{Row: rowNum, Field: "", Message: fmt.Sprintf("parse error: %v", err)})
			continue
		}

		row := make([]string, len(rec))
		copy(row, rec)
		trimAll(row)

		getCol := func(name string) string {
			if idx, ok := colIdx[name]; ok && idx < len(row) {
				return row[idx]
			}
			return ""
		}

		zone := getCol("zone")
		rack := getCol("rack")
		level := getCol("level")

		valid := true
		if zone == "" {
			report.Failed++
			report.Errors = append(report.Errors, RowError{Row: rowNum, Field: "zone", Message: "required"})
			valid = false
		}
		if rack == "" {
			if valid {
				report.Failed++
			}
			report.Errors = append(report.Errors, RowError{Row: rowNum, Field: "rack", Message: "required"})
			valid = false
		}
		if level == "" {
			if valid {
				report.Failed++
			}
			report.Errors = append(report.Errors, RowError{Row: rowNum, Field: "level", Message: "required"})
			valid = false
		}
		if !valid {
			continue
		}

		key := fmt.Sprintf("%s|%s|%s", zone, rack, level)
		if firstRow, exists := seenKeys[key]; exists {
			report.Errors = append(report.Errors, RowError{Row: rowNum, Field: "zone+rack+level", Message: fmt.Sprintf("duplicate in file (first at row %d)", firstRow)})
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

		code := fmt.Sprintf("%s-%s-%s", zone, rack, level)
		name := fmt.Sprintf("%s / Rack %s / Level %s", zone, rack, level)
		now := time.Now().UTC()

		toInsert = append(toInsert, bson.M{
			"_id":        primitive.NewObjectID(),
			"code":       code,
			"name":       name,
			"zone":       zone,
			"aisle":      "",
			"rack":       rack,
			"level":      level,
			"created_at": now,
			"updated_at": now,
		})

		// Batch insert every 500
		if len(toInsert) >= 500 {
			opts := options.InsertMany().SetOrdered(false)
			_, err := s.locations.InsertMany(ctx, toInsert, opts)
			if err != nil {
				return nil, fmt.Errorf("batch insert error: %w", err)
			}
			report.Inserted += len(toInsert)
			toInsert = toInsert[:0]
		}
	}

	if len(toInsert) > 0 {
		opts := options.InsertMany().SetOrdered(false)
		_, err := s.locations.InsertMany(ctx, toInsert, opts)
		if err != nil {
			return nil, fmt.Errorf("batch insert error: %w", err)
		}
		report.Inserted += len(toInsert)
	}

	return report, nil
}

// ── Products Export ─────────────────────────────────────────────

// ExportProducts writes all products as CSV to w.
func (s *Service) ExportProducts(ctx context.Context, w io.Writer) error {
	cursor, err := s.products.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "sku", Value: 1}}))
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	writer := csv.NewWriter(w)
	defer writer.Flush()

	writer.Write([]string{"sku", "name", "unit", "description"})

	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		writer.Write([]string{
			fmt.Sprint(doc["sku"]),
			fmt.Sprint(doc["name"]),
			fmt.Sprint(doc["unit"]),
			fmt.Sprint(doc["description"]),
		})
	}
	return cursor.Err()
}

// ExportLocations writes all locations as CSV to w.
func (s *Service) ExportLocations(ctx context.Context, w io.Writer) error {
	cursor, err := s.locations.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "zone", Value: 1}, {Key: "rack", Value: 1}, {Key: "level", Value: 1}}))
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	writer := csv.NewWriter(&bomWriter{w: w, first: true})
	defer writer.Flush()

	writer.Write([]string{"zone", "rack", "level"})

	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		writer.Write([]string{
			fmt.Sprint(doc["zone"]),
			fmt.Sprint(doc["rack"]),
			fmt.Sprint(doc["level"]),
		})
	}
	return cursor.Err()
}

// bomWriter writes a UTF-8 BOM before the first write for Excel compatibility.
type bomWriter struct {
	w     io.Writer
	first bool
}

func (bw *bomWriter) Write(p []byte) (int, error) {
	if bw.first {
		bw.first = false
		bom := []byte{0xEF, 0xBB, 0xBF}
		combined := make([]byte, 0, len(bom)+len(p))
		combined = append(combined, bom...)
		combined = append(combined, p...)
		n, err := bw.w.Write(combined)
		if n >= len(bom) {
			return n - len(bom), err
		}
		return 0, err
	}
	return bw.w.Write(p)
}

// ExportProductsToBytes is a convenience wrapper.
func (s *Service) ExportProductsToBytes(ctx context.Context) ([]byte, error) {
	var buf bytes.Buffer
	if err := s.ExportProducts(ctx, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ExportLocationsToBytes is a convenience wrapper.
func (s *Service) ExportLocationsToBytes(ctx context.Context) ([]byte, error) {
	var buf bytes.Buffer
	if err := s.ExportLocations(ctx, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
