package adjustment

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Adjustment struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	WarehouseID primitive.ObjectID  `bson:"warehouse_id" json:"warehouse_id"`
	ProductID   primitive.ObjectID  `bson:"product_id" json:"product_id"`
	LocationID  primitive.ObjectID  `bson:"location_id" json:"location_id"`
	LotID       *primitive.ObjectID `bson:"lot_id,omitempty" json:"lot_id,omitempty"`
	DeltaQty    int                 `bson:"delta_qty" json:"delta_qty"`
	Reason      string              `bson:"reason" json:"reason"`
	Note        string              `bson:"note,omitempty" json:"note,omitempty"`
	CreatedBy   string              `bson:"created_by" json:"created_by"`
	CreatedAt   time.Time           `bson:"created_at" json:"created_at"`
}

// Valid adjustment reasons
const (
	ReasonDamaged         = "DAMAGED"
	ReasonLost            = "LOST"
	ReasonFound           = "FOUND"
	ReasonCountCorrection = "COUNT_CORRECTION"
	ReasonOther           = "OTHER"
)

var ValidReasons = map[string]bool{
	ReasonDamaged:         true,
	ReasonLost:            true,
	ReasonFound:           true,
	ReasonCountCorrection: true,
	ReasonOther:           true,
}
