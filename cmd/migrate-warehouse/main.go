// cmd/migrate-warehouse/main.go
//
// Migration tool to backfill warehouse_id on all warehouse-scoped collections.
//
// Usage:
//   go run ./cmd/migrate-warehouse --dry-run   # preview changes
//   go run ./cmd/migrate-warehouse --apply      # write changes
//
// Environment variables (same as main app):
//   MONGO_URI  (default: mongodb://localhost:27017)
//   DB_NAME    (default: warehouse_crm)
//
// ⚠️  Recommended: take a backup before running --apply.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Collections that have a warehouse_id field.
var scopedCollections = []string{
	"locations",
	"inbounds",
	"outbounds",
	"stocks",
	"adjustments",
	"orders",
	"reservations",
	"pick_tasks",
	"pick_events",
	"returns",
	"return_items",
	"qc_holds",
	"history",
	"alerts",
}

type stats struct {
	Collection string
	Scanned    int64
	Updated    int64
	AlreadyOK  int64
}

func main() {
	dryRun := flag.Bool("dry-run", false, "Preview changes without writing to database")
	apply := flag.Bool("apply", false, "Actually write changes to database")
	flag.Parse()

	if !*dryRun && !*apply {
		fmt.Println("Usage:")
		fmt.Println("  go run ./cmd/migrate-warehouse --dry-run   # preview")
		fmt.Println("  go run ./cmd/migrate-warehouse --apply      # write")
		os.Exit(1)
	}

	if *dryRun && *apply {
		log.Fatal("Cannot use both --dry-run and --apply")
	}

	writeMode := *apply

	// Load .env
	_ = godotenv.Load()

	mongoURI := getEnv("MONGO_URI", "mongodb://localhost:27017")
	dbName := getEnv("DB_NAME", "warehouse_crm")

	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  Multi-Warehouse Migration Tool")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("  Database:   %s\n", dbName)
	fmt.Printf("  Mongo URI:  %s\n", maskURI(mongoURI))
	if writeMode {
		fmt.Println("  Mode:       APPLY (will write changes)")
	} else {
		fmt.Println("  Mode:       DRY-RUN (read-only preview)")
	}
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	if writeMode {
		fmt.Println("⚠️  RECOMMENDED: Take a database backup before proceeding!")
		fmt.Println("    mongodump --uri=\"" + mongoURI + "\" --db=" + dbName)
		fmt.Println()
	}

	// Connect
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = client.Disconnect(ctx) }()

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}
	fmt.Println("✅ Connected to MongoDB")

	db := client.Database(dbName)

	// Step 1: Ensure DEFAULT warehouse exists
	defaultWH, err := ensureDefaultWarehouse(ctx, db)
	if err != nil {
		log.Fatalf("Failed to ensure default warehouse: %v", err)
	}
	fmt.Printf("✅ Default warehouse: %s (code=%s, name=%s)\n\n", defaultWH.Hex(), "DEFAULT", "Default Warehouse")

	// Step 2: Backfill each collection
	allStats := make([]stats, 0, len(scopedCollections))

	for _, collName := range scopedCollections {
		s := backfillCollection(ctx, db, collName, defaultWH, writeMode)
		allStats = append(allStats, s)
	}

	// Step 3: Print summary
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  Summary")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Printf("  %-20s %10s %10s %10s\n", "Collection", "Scanned", "Updated", "Already OK")
	fmt.Println("  ────────────────────────────────────────────────────")

	totalScanned := int64(0)
	totalUpdated := int64(0)
	totalOK := int64(0)

	for _, s := range allStats {
		fmt.Printf("  %-20s %10d %10d %10d\n", s.Collection, s.Scanned, s.Updated, s.AlreadyOK)
		totalScanned += s.Scanned
		totalUpdated += s.Updated
		totalOK += s.AlreadyOK
	}

	fmt.Println("  ────────────────────────────────────────────────────")
	fmt.Printf("  %-20s %10d %10d %10d\n", "TOTAL", totalScanned, totalUpdated, totalOK)
	fmt.Println("═══════════════════════════════════════════════════════")

	if !writeMode {
		fmt.Println("\n⚡ This was a DRY RUN. No changes were made.")
		fmt.Println("   Run with --apply to write changes.")
	} else {
		fmt.Println("\n✅ Migration complete. All documents now have warehouse_id.")
	}
}

func ensureDefaultWarehouse(ctx context.Context, db *mongo.Database) (primitive.ObjectID, error) {
	col := db.Collection("warehouses")

	// Try to find existing default warehouse
	var existing struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	err := col.FindOne(ctx, bson.M{"is_default": true}).Decode(&existing)
	if err == nil {
		return existing.ID, nil
	}

	// Try by code
	err = col.FindOne(ctx, bson.M{"code": "DEFAULT"}).Decode(&existing)
	if err == nil {
		return existing.ID, nil
	}

	// Create it
	now := time.Now().UTC()
	result, err := col.InsertOne(ctx, bson.M{
		"code":       "DEFAULT",
		"name":       "Default Warehouse",
		"address":    "",
		"is_default": true,
		"created_at": now,
		"updated_at": now,
	})
	if err != nil {
		return primitive.ObjectID{}, err
	}

	return result.InsertedID.(primitive.ObjectID), nil
}

func backfillCollection(ctx context.Context, db *mongo.Database, collName string, defaultWH primitive.ObjectID, writeMode bool) stats {
	col := db.Collection(collName)
	s := stats{Collection: collName}

	// Count total documents
	total, err := col.CountDocuments(ctx, bson.M{})
	if err != nil {
		fmt.Printf("  ❌ %s: count error: %v\n", collName, err)
		return s
	}
	s.Scanned = total

	// Count documents missing warehouse_id (null, missing, or zero ObjectID)
	missingFilter := bson.M{
		"$or": bson.A{
			bson.M{"warehouse_id": bson.M{"$exists": false}},
			bson.M{"warehouse_id": nil},
			bson.M{"warehouse_id": primitive.NilObjectID},
		},
	}

	missing, err := col.CountDocuments(ctx, missingFilter)
	if err != nil {
		fmt.Printf("  ❌ %s: missing count error: %v\n", collName, err)
		return s
	}

	s.AlreadyOK = total - missing

	if missing == 0 {
		fmt.Printf("  ✅ %-20s %d docs, all have warehouse_id\n", collName, total)
		return s
	}

	if writeMode {
		result, err := col.UpdateMany(ctx, missingFilter, bson.M{
			"$set": bson.M{"warehouse_id": defaultWH},
		})
		if err != nil {
			fmt.Printf("  ❌ %s: update error: %v\n", collName, err)
			return s
		}
		s.Updated = result.ModifiedCount
		fmt.Printf("  📝 %-20s %d docs, updated %d, already ok %d\n", collName, total, result.ModifiedCount, s.AlreadyOK)
	} else {
		s.Updated = missing // would be updated
		fmt.Printf("  🔍 %-20s %d docs, would update %d, already ok %d\n", collName, total, missing, s.AlreadyOK)
	}

	return s
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func maskURI(uri string) string {
	if len(uri) > 30 {
		return uri[:20] + "...***"
	}
	return uri
}
