package product

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Product struct {
	ID                primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	SKU               string             `bson:"sku" json:"sku"`
	Name              string             `bson:"name" json:"name"`
	Description       string             `bson:"description,omitempty" json:"description,omitempty"`
	Unit              string             `bson:"unit" json:"unit"`
	LowStockThreshold *int               `bson:"low_stock_threshold,omitempty" json:"low_stock_threshold,omitempty"`
	CreatedAt         time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt         time.Time          `bson:"updated_at" json:"updated_at"`
}
