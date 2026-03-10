package returns

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ── Return statuses ──
const (
	StatusOpen      = "OPEN"
	StatusReceived  = "RECEIVED"
	StatusClosed    = "CLOSED"
	StatusCancelled = "CANCELLED"
)

// ── Dispositions ──
const (
	DispositionRestock = "RESTOCK"
	DispositionDamaged = "DAMAGED"
	DispositionQCHold  = "QC_HOLD"
)

// ── QC Hold statuses ──
const (
	QCStatusHeld     = "HELD"
	QCStatusReleased = "RELEASED"
)

// Return is the RMA header stored in the "returns" collection.
type Return struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	WarehouseID primitive.ObjectID `bson:"warehouse_id" json:"warehouse_id"`
	RMANo       string             `bson:"rma_no" json:"rma_no"`
	OrderID     primitive.ObjectID `bson:"order_id" json:"order_id"`
	OrderNo     string             `bson:"order_no" json:"order_no"`
	ClientName  string             `bson:"client_name" json:"client_name"`
	Status      string             `bson:"status" json:"status"`
	Notes       string             `bson:"notes,omitempty" json:"notes,omitempty"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	CreatedBy   string             `bson:"created_by" json:"created_by"`
	ReceivedAt  *time.Time         `bson:"received_at,omitempty" json:"received_at,omitempty"`
	ReceivedBy  string             `bson:"received_by,omitempty" json:"received_by,omitempty"`
}

// ReturnItem is a line item stored in the "return_items" collection.
type ReturnItem struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	WarehouseID primitive.ObjectID  `bson:"warehouse_id" json:"warehouse_id"`
	ReturnID    primitive.ObjectID  `bson:"return_id" json:"return_id"`
	ProductID   primitive.ObjectID  `bson:"product_id" json:"product_id"`
	LocationID  *primitive.ObjectID `bson:"location_id,omitempty" json:"location_id,omitempty"`
	LotID       *primitive.ObjectID `bson:"lot_id,omitempty" json:"lot_id,omitempty"`
	Qty         int                 `bson:"qty" json:"qty"`
	Disposition string              `bson:"disposition" json:"disposition"`
	Note        string              `bson:"note,omitempty" json:"note,omitempty"`
	CreatedAt   time.Time           `bson:"created_at" json:"created_at"`
	CreatedBy   string              `bson:"created_by" json:"created_by"`
}

// QCHold tracks items held for quality control in the "qc_holds" collection.
type QCHold struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	WarehouseID primitive.ObjectID  `bson:"warehouse_id" json:"warehouse_id"`
	ReturnID    primitive.ObjectID  `bson:"return_id" json:"return_id"`
	ProductID   primitive.ObjectID  `bson:"product_id" json:"product_id"`
	LotID       *primitive.ObjectID `bson:"lot_id,omitempty" json:"lot_id,omitempty"`
	Qty         int                 `bson:"qty" json:"qty"`
	Status      string              `bson:"status" json:"status"`
	CreatedAt   time.Time           `bson:"created_at" json:"created_at"`
	ReleasedAt  *time.Time          `bson:"released_at,omitempty" json:"released_at,omitempty"`
}
