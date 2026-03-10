package order

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	StatusDraft     = "DRAFT"
	StatusConfirmed = "CONFIRMED"
	StatusPicking   = "PICKING"
	StatusShipped   = "SHIPPED"
	StatusCancelled = "CANCELLED"
)

type Order struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	WarehouseID primitive.ObjectID `bson:"warehouse_id" json:"warehouse_id"`
	OrderNo     string             `bson:"order_no" json:"order_no"`
	ClientName  string             `bson:"client_name" json:"client_name"`
	Status      string             `bson:"status" json:"status"`
	Items       []OrderItem        `bson:"items" json:"items"`
	Notes       string             `bson:"notes,omitempty" json:"notes,omitempty"`
	CreatedBy   string             `bson:"created_by" json:"created_by"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	ConfirmedAt *time.Time         `bson:"confirmed_at,omitempty" json:"confirmed_at,omitempty"`
	ShippedAt   *time.Time         `bson:"shipped_at,omitempty" json:"shipped_at,omitempty"`
	CancelledAt *time.Time         `bson:"cancelled_at,omitempty" json:"cancelled_at,omitempty"`
}

type OrderItem struct {
	ProductID    primitive.ObjectID `bson:"product_id" json:"product_id"`
	RequestedQty int                `bson:"requested_qty" json:"requested_qty"`
	ReservedQty  int                `bson:"reserved_qty" json:"reserved_qty"`
	ShippedQty   int                `bson:"shipped_qty" json:"shipped_qty"`
}
