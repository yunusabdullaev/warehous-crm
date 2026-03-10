package auth

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID                  primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	Username            string               `bson:"username" json:"username"`
	Password            string               `bson:"password" json:"-"`
	Role                string               `bson:"role" json:"role"`
	TenantID            primitive.ObjectID   `bson:"tenant_id,omitempty" json:"tenant_id,omitempty"`
	AllowedWarehouseIDs []primitive.ObjectID `bson:"allowed_warehouse_ids,omitempty" json:"allowed_warehouse_ids,omitempty"`
	DefaultWarehouseID  primitive.ObjectID   `bson:"default_warehouse_id,omitempty" json:"default_warehouse_id,omitempty"`
	CreatedAt           time.Time            `bson:"created_at" json:"created_at"`
	UpdatedAt           time.Time            `bson:"updated_at" json:"updated_at"`

	// 2FA
	TwoFactorSecret  string `bson:"two_factor_secret,omitempty" json:"-"`
	TwoFactorEnabled bool   `bson:"two_factor_enabled" json:"two_factor_enabled"`
}

const (
	RoleSuperAdmin = "superadmin"
	RoleAdmin      = "admin"
	RoleOperator   = "operator"
	RoleViewer     = "viewer"
	RoleLoader     = "loader"
)
