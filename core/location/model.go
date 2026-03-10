package location

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Location struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	WarehouseID primitive.ObjectID `bson:"warehouse_id" json:"warehouse_id"`
	Code        string             `bson:"code" json:"code"`
	Name        string             `bson:"name" json:"name"`
	Zone        string             `bson:"zone,omitempty" json:"zone,omitempty"`
	Aisle       string             `bson:"aisle,omitempty" json:"aisle,omitempty"`
	Rack        string             `bson:"rack,omitempty" json:"rack,omitempty"`
	Level       string             `bson:"level,omitempty" json:"level,omitempty"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
}
