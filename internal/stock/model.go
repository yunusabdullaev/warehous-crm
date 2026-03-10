package stock

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Stock struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	WarehouseID primitive.ObjectID `bson:"warehouse_id" json:"warehouse_id"`
	ProductID   primitive.ObjectID `bson:"product_id" json:"product_id"`
	LocationID  primitive.ObjectID `bson:"location_id" json:"location_id"`
	LotID       primitive.ObjectID `bson:"lot_id" json:"lot_id"`
	Quantity    int                `bson:"quantity" json:"quantity"`
	LastUpdated time.Time          `bson:"last_updated" json:"last_updated"`
}
