package reservation

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	StatusActive   = "ACTIVE"
	StatusReleased = "RELEASED"
)

type Reservation struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	WarehouseID primitive.ObjectID `bson:"warehouse_id" json:"warehouse_id"`
	OrderID     primitive.ObjectID `bson:"order_id" json:"order_id"`
	ProductID   primitive.ObjectID `bson:"product_id" json:"product_id"`
	Qty         int                `bson:"qty" json:"qty"`
	Status      string             `bson:"status" json:"status"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	CreatedBy   string             `bson:"created_by" json:"created_by"`
	ReleasedAt  *time.Time         `bson:"released_at,omitempty" json:"released_at,omitempty"`
	ReleasedBy  string             `bson:"released_by,omitempty" json:"released_by,omitempty"`
	Reason      string             `bson:"reason,omitempty" json:"reason,omitempty"`
}
