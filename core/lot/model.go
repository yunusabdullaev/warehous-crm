package lot

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Lot represents a production batch for a product.
type Lot struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProductID primitive.ObjectID `bson:"product_id" json:"product_id"`
	LotNo     string             `bson:"lot_no" json:"lot_no"`
	MfgDate   *time.Time         `bson:"mfg_date,omitempty" json:"mfg_date,omitempty"`
	ExpDate   *time.Time         `bson:"exp_date,omitempty" json:"exp_date,omitempty"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}

const DefaultLotNo = "DEFAULT"
