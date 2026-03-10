package middleware

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// WarehouseContext reads X-Warehouse-Id, validates user access including
// tenant isolation, and injects warehouseId into c.Locals("warehouseId").
// Requires AuthMiddleware to have run first (sets userID, role, tenantID).
func WarehouseContext(usersCol *mongo.Collection, warehousesCol *mongo.Collection) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		userID, _ := c.Locals("userID").(string)
		callerTenantID, _ := c.Locals("tenantID").(string)

		header := c.Get("X-Warehouse-Id")

		// ALL mode handling
		if header == "ALL" {
			if role == "superadmin" {
				// Superadmin: global ALL mode
				c.Locals("warehouseId", "ALL")
				return c.Next()
			}
			if role == "admin" {
				// Tenant-admin: ALL mode scoped to their tenant (handled downstream)
				c.Locals("warehouseId", "ALL")
				return c.Next()
			}
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "ALL mode is only available to admin users",
			})
		}

		// Try to parse header as ObjectID
		var warehouseID primitive.ObjectID
		if header != "" {
			oid, err := primitive.ObjectIDFromHex(header)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "invalid X-Warehouse-Id header",
				})
			}
			warehouseID = oid
		}

		// If no header, try user's default
		if warehouseID.IsZero() {
			uid, err := primitive.ObjectIDFromHex(userID)
			if err == nil {
				var user struct {
					DefaultWarehouseID primitive.ObjectID `bson:"default_warehouse_id"`
				}
				err := usersCol.FindOne(c.Context(), bson.M{"_id": uid}).Decode(&user)
				if err == nil && !user.DefaultWarehouseID.IsZero() {
					warehouseID = user.DefaultWarehouseID
				}
			}
		}

		// Superadmin without warehouse context → allow through for
		// control-plane routes (tenant mgmt, billing, user admin, etc.)
		if warehouseID.IsZero() && role == "superadmin" {
			return c.Next()
		}

		// Still empty → 400
		if warehouseID.IsZero() {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "warehouse context required — set X-Warehouse-Id header or assign a default warehouse to your user",
			})
		}

		// Superadmin = unrestricted access to any warehouse
		if role == "superadmin" {
			c.Locals("warehouseId", warehouseID)
			return c.Next()
		}

		// ── Tenant isolation: verify warehouse belongs to caller's tenant ──
		if callerTenantID != "" && warehousesCol != nil {
			callerTenantOID, _ := primitive.ObjectIDFromHex(callerTenantID)
			if !callerTenantOID.IsZero() {
				var wh struct {
					TenantID primitive.ObjectID `bson:"tenant_id"`
				}
				err := warehousesCol.FindOne(c.Context(), bson.M{"_id": warehouseID}).Decode(&wh)
				if err != nil {
					return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
						"error": "warehouse not found",
					})
				}
				if !wh.TenantID.IsZero() && wh.TenantID != callerTenantOID {
					return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
						"error":   "forbidden",
						"message": "warehouse does not belong to your tenant",
					})
				}
			}
		}

		// Admin = unrestricted access within their tenant (already validated above)
		if role == "admin" {
			c.Locals("warehouseId", warehouseID)
			return c.Next()
		}

		// Non-admin: check AllowedWarehouseIDs
		uid, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid user"})
		}

		var user struct {
			AllowedWarehouseIDs []primitive.ObjectID `bson:"allowed_warehouse_ids"`
		}
		if err := usersCol.FindOne(c.Context(), bson.M{"_id": uid}).Decode(&user); err != nil {
			slog.Error("warehouse middleware: user lookup failed", "error", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "user lookup failed"})
		}

		allowed := false
		for _, id := range user.AllowedWarehouseIDs {
			if id == warehouseID {
				allowed = true
				break
			}
		}
		if !allowed {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":   "forbidden",
				"message": "you do not have access to this warehouse",
			})
		}

		c.Locals("warehouseId", warehouseID)
		return c.Next()
	}
}

// GetWarehouseID extracts the warehouse ObjectID from Fiber context.
// Returns zero ObjectID if not set (backwards compat — runtime fallback).
func GetWarehouseID(c *fiber.Ctx) primitive.ObjectID {
	val := c.Locals("warehouseId")
	if val == nil {
		return primitive.ObjectID{}
	}
	if oid, ok := val.(primitive.ObjectID); ok {
		return oid
	}
	return primitive.ObjectID{}
}

// IsAllWarehouses checks if admin requested ALL warehouses mode.
func IsAllWarehouses(c *fiber.Ctx) bool {
	val := c.Locals("warehouseId")
	if s, ok := val.(string); ok && s == "ALL" {
		return true
	}
	return false
}
