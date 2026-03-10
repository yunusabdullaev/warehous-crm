// cmd/migrate-tenant/main.go
//
// Migration tool to set up multi-tenant data isolation.
//
// Usage:
//   go run ./cmd/migrate-tenant --dry-run   # preview changes
//   go run ./cmd/migrate-tenant --apply      # write changes
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
		fmt.Println("  go run ./cmd/migrate-tenant --dry-run   # preview")
		fmt.Println("  go run ./cmd/migrate-tenant --apply      # write")
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
	fmt.Println("  Multi-Tenant Migration Tool")
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

	// Step 1: Ensure DEFAULT tenant exists
	defaultTenantID, err := ensureDefaultTenant(ctx, db, writeMode)
	if err != nil {
		log.Fatalf("Failed to ensure default tenant: %v", err)
	}
	fmt.Printf("✅ Default tenant: %s (code=TEN-DEFAULT)\n\n", defaultTenantID.Hex())

	// Step 2: Backfill users with tenant_id
	allStats := make([]stats, 0, 2)

	userStats := backfillCollection(ctx, db, "users", defaultTenantID, writeMode)
	allStats = append(allStats, userStats)

	// Step 3: Backfill warehouses with tenant_id
	whStats := backfillCollection(ctx, db, "warehouses", defaultTenantID, writeMode)
	allStats = append(allStats, whStats)

	// Step 4: Print summary
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
		fmt.Println("\n✅ Migration complete. All users and warehouses now have tenant_id.")
	}
}

func ensureDefaultTenant(ctx context.Context, db *mongo.Database, writeMode bool) (primitive.ObjectID, error) {
	col := db.Collection("tenants")

	// Try to find existing default tenant by code
	var existing struct {
		ID primitive.ObjectID `bson:"_id"`
	}

	err := col.FindOne(ctx, bson.M{"code": "TEN-DEFAULT"}).Decode(&existing)
	if err == nil {
		return existing.ID, nil
	}

	if !writeMode {
		// In dry-run mode, return a placeholder ID
		fmt.Println("  🔍 Would create DEFAULT tenant (code=TEN-DEFAULT)")
		return primitive.NewObjectID(), nil
	}

	// Create default tenant
	now := time.Now().UTC()
	result, err := col.InsertOne(ctx, bson.M{
		"code":       "TEN-DEFAULT",
		"name":       "Default Tenant",
		"plan":       "FREE",
		"created_at": now,
	})
	if err != nil {
		return primitive.ObjectID{}, err
	}

	fmt.Println("  📝 Created DEFAULT tenant (code=TEN-DEFAULT)")
	return result.InsertedID.(primitive.ObjectID), nil
}

func backfillCollection(ctx context.Context, db *mongo.Database, collName string, defaultTenantID primitive.ObjectID, writeMode bool) stats {
	col := db.Collection(collName)
	s := stats{Collection: collName}

	// Count total documents
	total, err := col.CountDocuments(ctx, bson.M{})
	if err != nil {
		fmt.Printf("  ❌ %s: count error: %v\n", collName, err)
		return s
	}
	s.Scanned = total

	// Count documents missing tenant_id (null, missing, or zero ObjectID)
	missingFilter := bson.M{
		"$or": bson.A{
			bson.M{"tenant_id": bson.M{"$exists": false}},
			bson.M{"tenant_id": nil},
			bson.M{"tenant_id": primitive.NilObjectID},
		},
	}

	missing, err := col.CountDocuments(ctx, missingFilter)
	if err != nil {
		fmt.Printf("  ❌ %s: missing count error: %v\n", collName, err)
		return s
	}

	s.AlreadyOK = total - missing

	if missing == 0 {
		fmt.Printf("  ✅ %-20s %d docs, all have tenant_id\n", collName, total)
		return s
	}

	if writeMode {
		result, err := col.UpdateMany(ctx, missingFilter, bson.M{
			"$set": bson.M{"tenant_id": defaultTenantID},
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
