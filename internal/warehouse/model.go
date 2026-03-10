package warehouse

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Warehouse struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Code      string             `bson:"code" json:"code"`
	Name      string             `bson:"name" json:"name"`
	Address   string             `bson:"address,omitempty" json:"address,omitempty"`
	TenantID  primitive.ObjectID `bson:"tenant_id,omitempty" json:"tenant_id,omitempty"`
	IsDefault bool               `bson:"is_default" json:"is_default"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
}
