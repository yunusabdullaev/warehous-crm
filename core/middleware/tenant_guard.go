package middleware

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// ── Cached tenant config ──

type cachedTenant struct {
	Status    string
	Plan      string
	Limits    tenantLimits
	Features  tenantFeatures
	FetchedAt time.Time
}

type tenantLimits struct {
	MaxWarehouses  int `bson:"max_warehouses"`
	MaxUsers       int `bson:"max_users"`
	MaxProducts    int `bson:"max_products"`
	MaxDailyOrders int `bson:"max_daily_orders"`
}

type tenantFeatures struct {
	EnableReports        bool `bson:"enable_reports"`
	EnableExpiryDigest   bool `bson:"enable_expiry_digest"`
	EnableQrLabels       bool `bson:"enable_qr_labels"`
	EnableReturns        bool `bson:"enable_returns"`
	EnableLots           bool `bson:"enable_lots"`
	EnableMultiWarehouse bool `bson:"enable_multi_warehouse"`
	EnableApiExport      bool `bson:"enable_api_export"`
}

// In-memory cache with configurable TTL (default 60s, 0 = disabled)
var (
	tenantCache   = make(map[string]*cachedTenant)
	tenantCacheMu sync.RWMutex
	cacheTTL      = 60 * time.Second
)

// SetCacheTTL configures the tenant cache TTL. 0 disables caching entirely.
func SetCacheTTL(seconds int) {
	if seconds <= 0 {
		cacheTTL = 0
	} else {
		cacheTTL = time.Duration(seconds) * time.Second
	}
}

func getTenantConfig(ctx context.Context, tenantCol *mongo.Collection, tenantIDHex string) (*cachedTenant, error) {
	// Check cache (skip if cacheTTL == 0)
	if cacheTTL > 0 {
		tenantCacheMu.RLock()
		cached, ok := tenantCache[tenantIDHex]
		tenantCacheMu.RUnlock()
		if ok && time.Since(cached.FetchedAt) < cacheTTL {
			return cached, nil
		}
	}

	// Fetch from DB
	oid, err := primitive.ObjectIDFromHex(tenantIDHex)
	if err != nil {
		return nil, err
	}

	var doc struct {
		Status   string         `bson:"status"`
		Plan     string         `bson:"plan"`
		Limits   tenantLimits   `bson:"limits"`
		Features tenantFeatures `bson:"features"`
	}
	err = tenantCol.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc)
	if err != nil {
		return nil, err
	}

	entry := &cachedTenant{
		Status:    doc.Status,
		Plan:      doc.Plan,
		Limits:    doc.Limits,
		Features:  doc.Features,
		FetchedAt: time.Now(),
	}

	tenantCacheMu.Lock()
	tenantCache[tenantIDHex] = entry
	tenantCacheMu.Unlock()

	return entry, nil
}

// InvalidateTenantCache removes a tenant from the in-memory cache.
func InvalidateTenantCache(tenantIDHex string) {
	tenantCacheMu.Lock()
	delete(tenantCache, tenantIDHex)
	tenantCacheMu.Unlock()
}

// ── RequireTenantActive ──

// RequireTenantActive blocks all non-superadmin access if the tenant is SUSPENDED.
// Must be placed after AuthMiddleware.
func RequireTenantActive(tenantCol *mongo.Collection) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		if role == "superadmin" {
			return c.Next()
		}

		tenantID, _ := c.Locals("tenantID").(string)
		if tenantID == "" {
			return c.Next() // no tenant context
		}

		cfg, err := getTenantConfig(c.Context(), tenantCol, tenantID)
		if err != nil {
			slog.Error("tenant_guard: failed to fetch tenant", "error", err, "tenantID", tenantID)
			return c.Next() // fail-open to avoid locking out on DB issues
		}

		// Store in locals for downstream middleware
		c.Locals("tenantConfig", cfg)

		if cfg.Status == "SUSPENDED" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":   "TENANT_SUSPENDED",
				"message": "Tenant suspended. Contact your administrator.",
			})
		}

		return c.Next()
	}
}

// ── RequireFeature ──

// RequireFeature blocks access if the specified feature flag is disabled for the tenant.
func RequireFeature(tenantCol *mongo.Collection, featureName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		if role == "superadmin" {
			return c.Next()
		}

		tenantID, _ := c.Locals("tenantID").(string)
		if tenantID == "" {
			return c.Next()
		}

		// Try to get config from locals (set by RequireTenantActive)
		var cfg *cachedTenant
		if cached, ok := c.Locals("tenantConfig").(*cachedTenant); ok {
			cfg = cached
		} else {
			var err error
			cfg, err = getTenantConfig(c.Context(), tenantCol, tenantID)
			if err != nil {
				return c.Next() // fail-open
			}
		}

		enabled := isFeatureEnabled(cfg, featureName)
		if !enabled {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":   "FEATURE_DISABLED",
				"feature": featureName,
				"plan":    cfg.Plan,
				"message": "This feature is not available on your current plan. Upgrade to unlock.",
			})
		}

		return c.Next()
	}
}

func isFeatureEnabled(cfg *cachedTenant, name string) bool {
	switch name {
	case "enableReports":
		return cfg.Features.EnableReports
	case "enableExpiryDigest":
		return cfg.Features.EnableExpiryDigest
	case "enableQrLabels":
		return cfg.Features.EnableQrLabels
	case "enableReturns":
		return cfg.Features.EnableReturns
	case "enableLots":
		return cfg.Features.EnableLots
	case "enableMultiWarehouse":
		return cfg.Features.EnableMultiWarehouse
	case "enableApiExport":
		return cfg.Features.EnableApiExport
	default:
		return true // unknown feature → allow
	}
}

// ── EnforceLimit ──

// EnforceLimit blocks write operations if the tenant has reached their plan quota.
// Returns HTTP 402 with structured JSON on limit exceeded.
func EnforceLimit(tenantCol *mongo.Collection, db *mongo.Database, limitName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		if role == "superadmin" {
			return c.Next()
		}

		tenantID, _ := c.Locals("tenantID").(string)
		if tenantID == "" {
			return c.Next()
		}

		// Get tenant config
		var cfg *cachedTenant
		if cached, ok := c.Locals("tenantConfig").(*cachedTenant); ok {
			cfg = cached
		} else {
			var err error
			cfg, err = getTenantConfig(c.Context(), tenantCol, tenantID)
			if err != nil {
				return c.Next() // fail-open
			}
		}

		tenantOID, err := primitive.ObjectIDFromHex(tenantID)
		if err != nil {
			return c.Next()
		}

		allowed, current, err := checkLimit(c.Context(), db, cfg, tenantOID, limitName)
		if err != nil {
			slog.Error("tenant_guard: limit check failed", "error", err, "limit", limitName)
			return c.Next() // fail-open
		}

		if current >= int64(allowed) {
			return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{
				"error":   "PLAN_LIMIT",
				"limit":   limitName,
				"allowed": allowed,
				"current": current,
				"plan":    cfg.Plan,
				"message": "Plan limit reached. Upgrade your plan to increase this limit.",
			})
		}

		return c.Next()
	}
}

func checkLimit(ctx context.Context, db *mongo.Database, cfg *cachedTenant, tenantID primitive.ObjectID, limitName string) (int, int64, error) {
	tenantFilter := bson.M{"tenant_id": tenantID}

	switch limitName {
	case "maxWarehouses":
		count, err := db.Collection("warehouses").CountDocuments(ctx, tenantFilter)
		return cfg.Limits.MaxWarehouses, count, err

	case "maxUsers":
		count, err := db.Collection("users").CountDocuments(ctx, tenantFilter)
		return cfg.Limits.MaxUsers, count, err

	case "maxProducts":
		// Products are warehouse-scoped, need to find tenant's warehouses first
		whIDs, err := getTenantWarehouseIDs(ctx, db, tenantID)
		if err != nil {
			return cfg.Limits.MaxProducts, 0, err
		}
		if len(whIDs) == 0 {
			return cfg.Limits.MaxProducts, 0, nil
		}
		count, err := db.Collection("products").CountDocuments(ctx, bson.M{
			"warehouse_id": bson.M{"$in": whIDs},
		})
		return cfg.Limits.MaxProducts, count, err

	case "maxDailyOrders":
		whIDs, err := getTenantWarehouseIDs(ctx, db, tenantID)
		if err != nil {
			return cfg.Limits.MaxDailyOrders, 0, err
		}
		if len(whIDs) == 0 {
			return cfg.Limits.MaxDailyOrders, 0, nil
		}
		now := time.Now().UTC()
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		count, err := db.Collection("orders").CountDocuments(ctx, bson.M{
			"warehouse_id": bson.M{"$in": whIDs},
			"created_at":   bson.M{"$gte": startOfDay},
		})
		return cfg.Limits.MaxDailyOrders, count, err

	default:
		return 0, 0, nil
	}
}

func getTenantWarehouseIDs(ctx context.Context, db *mongo.Database, tenantID primitive.ObjectID) ([]primitive.ObjectID, error) {
	cursor, err := db.Collection("warehouses").Find(ctx, bson.M{"tenant_id": tenantID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var ids []primitive.ObjectID
	for cursor.Next(ctx) {
		var wh struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if cursor.Decode(&wh) == nil {
			ids = append(ids, wh.ID)
		}
	}
	return ids, nil
}
