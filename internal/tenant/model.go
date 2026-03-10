package tenant

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ── Plan constants ──

const (
	PlanFree       = "FREE"
	PlanPro        = "PRO"
	PlanEnterprise = "ENTERPRISE"
)

// ── Status constants ──

const (
	StatusActive    = "ACTIVE"
	StatusSuspended = "SUSPENDED"
)

// ── Billing status constants ──

const (
	BillingTrialing   = "TRIALING"
	BillingActive     = "ACTIVE"
	BillingPastDue    = "PAST_DUE"
	BillingCanceled   = "CANCELED"
	BillingIncomplete = "INCOMPLETE"
)

// ── Tenant ──

type Tenant struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Code      string             `bson:"code" json:"code"`
	Name      string             `bson:"name" json:"name"`
	Plan      string             `bson:"plan" json:"plan"`
	Status    string             `bson:"status" json:"status"`
	Limits    TenantLimits       `bson:"limits" json:"limits"`
	Features  TenantFeatures     `bson:"features" json:"features"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`

	// Stripe billing
	StripeCustomerID     string    `bson:"stripe_customer_id,omitempty" json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID string    `bson:"stripe_subscription_id,omitempty" json:"stripe_subscription_id,omitempty"`
	StripePriceID        string    `bson:"stripe_price_id,omitempty" json:"stripe_price_id,omitempty"`
	BillingStatus        string    `bson:"billing_status,omitempty" json:"billing_status,omitempty"`
	CurrentPeriodEnd     time.Time `bson:"current_period_end,omitempty" json:"current_period_end,omitempty"`
	CancelAtPeriodEnd    bool      `bson:"cancel_at_period_end" json:"cancel_at_period_end"`
}

// TenantLimits defines numeric quotas per tenant.
type TenantLimits struct {
	MaxWarehouses  int `bson:"max_warehouses" json:"max_warehouses"`
	MaxUsers       int `bson:"max_users" json:"max_users"`
	MaxProducts    int `bson:"max_products" json:"max_products"`
	MaxDailyOrders int `bson:"max_daily_orders" json:"max_daily_orders"`
	MaxStorageMB   int `bson:"max_storage_mb,omitempty" json:"max_storage_mb,omitempty"`
}

// TenantFeatures defines feature flags per tenant.
type TenantFeatures struct {
	EnableReports        bool `bson:"enable_reports" json:"enable_reports"`
	EnableExpiryDigest   bool `bson:"enable_expiry_digest" json:"enable_expiry_digest"`
	EnableQrLabels       bool `bson:"enable_qr_labels" json:"enable_qr_labels"`
	EnableReturns        bool `bson:"enable_returns" json:"enable_returns"`
	EnableLots           bool `bson:"enable_lots" json:"enable_lots"`
	EnableMultiWarehouse bool `bson:"enable_multi_warehouse" json:"enable_multi_warehouse"`
	EnableApiExport      bool `bson:"enable_api_export" json:"enable_api_export"`
}

// TenantUsage holds current usage counts for a tenant.
type TenantUsage struct {
	Warehouses  int64 `json:"warehouses"`
	Users       int64 `json:"users"`
	Products    int64 `json:"products"`
	TodayOrders int64 `json:"today_orders"`
}

// PlanDefaults returns the default limits and features for a given plan.
func PlanDefaults(plan string) (TenantLimits, TenantFeatures) {
	switch plan {
	case PlanPro:
		return TenantLimits{
				MaxWarehouses:  5,
				MaxUsers:       25,
				MaxProducts:    5000,
				MaxDailyOrders: 500,
			}, TenantFeatures{
				EnableReports:        true,
				EnableExpiryDigest:   true,
				EnableQrLabels:       true,
				EnableReturns:        true,
				EnableLots:           true,
				EnableMultiWarehouse: true,
				EnableApiExport:      false,
			}
	case PlanEnterprise:
		return TenantLimits{
				MaxWarehouses:  999,
				MaxUsers:       999,
				MaxProducts:    999999,
				MaxDailyOrders: 999999,
			}, TenantFeatures{
				EnableReports:        true,
				EnableExpiryDigest:   true,
				EnableQrLabels:       true,
				EnableReturns:        true,
				EnableLots:           true,
				EnableMultiWarehouse: true,
				EnableApiExport:      true,
			}
	default: // FREE
		return TenantLimits{
				MaxWarehouses:  1,
				MaxUsers:       5,
				MaxProducts:    500,
				MaxDailyOrders: 50,
			}, TenantFeatures{
				EnableReports:        false,
				EnableExpiryDigest:   false,
				EnableQrLabels:       false,
				EnableReturns:        false,
				EnableLots:           false,
				EnableMultiWarehouse: false,
				EnableApiExport:      false,
			}
	}
}
