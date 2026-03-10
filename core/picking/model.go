package picking

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ── Pick Task statuses ──

const (
	TaskStatusOpen       = "OPEN"
	TaskStatusInProgress = "IN_PROGRESS"
	TaskStatusDone       = "DONE"
	TaskStatusCancelled  = "CANCELLED"
)

// PickTask represents a single pick instruction: take plannedQty of a product from a specific location/lot.
type PickTask struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	WarehouseID primitive.ObjectID `bson:"warehouse_id" json:"warehouse_id"`
	OrderID     primitive.ObjectID `bson:"order_id" json:"order_id"`
	ProductID   primitive.ObjectID `bson:"product_id" json:"product_id"`
	LocationID  primitive.ObjectID `bson:"location_id" json:"location_id"`
	LotID       primitive.ObjectID `bson:"lot_id" json:"lot_id"`
	PlannedQty  int                `bson:"planned_qty" json:"planned_qty"`
	PickedQty   int                `bson:"picked_qty" json:"picked_qty"`
	Status      string             `bson:"status" json:"status"`
	AssignedTo  *string            `bson:"assigned_to,omitempty" json:"assigned_to,omitempty"`
	CreatedBy   string             `bson:"created_by" json:"created_by"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
}

// PickEvent is an immutable record of a scan/pick action.
type PickEvent struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	WarehouseID primitive.ObjectID `bson:"warehouse_id" json:"warehouse_id"`
	OrderID     primitive.ObjectID `bson:"order_id" json:"order_id"`
	PickTaskID  primitive.ObjectID `bson:"pick_task_id" json:"pick_task_id"`
	UserID      string             `bson:"user_id" json:"user_id"`
	LocationID  primitive.ObjectID `bson:"location_id" json:"location_id"`
	ProductID   primitive.ObjectID `bson:"product_id" json:"product_id"`
	LotID       primitive.ObjectID `bson:"lot_id" json:"lot_id"`
	Qty         int                `bson:"qty" json:"qty"`
	ScannedAt   time.Time          `bson:"scanned_at" json:"scanned_at"`
	Meta        PickEventMeta      `bson:"meta" json:"meta"`
}

type PickEventMeta struct {
	Scanner string `bson:"scanner,omitempty" json:"scanner,omitempty"`
	Client  string `bson:"client,omitempty" json:"client,omitempty"`
}
