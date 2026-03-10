package inbound

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Inbound struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	WarehouseID   primitive.ObjectID `bson:"warehouse_id" json:"warehouse_id"`
	ProductID     primitive.ObjectID `bson:"product_id" json:"product_id"`
	LocationID    primitive.ObjectID `bson:"location_id" json:"location_id"`
	LotID         primitive.ObjectID `bson:"lot_id" json:"lot_id"`
	Quantity      int                `bson:"quantity" json:"quantity"`
	Reference     string             `bson:"reference,omitempty" json:"reference,omitempty"`
	UserID        string             `bson:"user_id" json:"user_id"`
	Status        string             `bson:"status" json:"status"`
	ReversedAt    *time.Time         `bson:"reversed_at,omitempty" json:"reversed_at,omitempty"`
	ReversedBy    string             `bson:"reversed_by,omitempty" json:"reversed_by,omitempty"`
	ReverseReason string             `bson:"reverse_reason,omitempty" json:"reverse_reason,omitempty"`
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
}

const (
	StatusActive   = "ACTIVE"
	StatusReversed = "REVERSED"
)
