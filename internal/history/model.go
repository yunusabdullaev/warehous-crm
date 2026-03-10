package history

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type History struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	WarehouseID primitive.ObjectID `bson:"warehouse_id" json:"warehouse_id"`
	Action      string             `bson:"action" json:"action"`
	EntityType  string             `bson:"entity_type" json:"entity_type"`
	EntityID    string             `bson:"entity_id" json:"entity_id"`
	UserID      string             `bson:"user_id" json:"user_id"`
	Details     string             `bson:"details,omitempty" json:"details,omitempty"`
	Timestamp   time.Time          `bson:"timestamp" json:"timestamp"`
}

const (
	ActionCreate              = "create"
	ActionUpdate              = "update"
	ActionDelete              = "delete"
	ActionInbound             = "inbound"
	ActionOutbound            = "outbound"
	ActionCreateAdjustment    = "create_adjustment"
	ActionReverseInbound      = "reverse_inbound"
	ActionReverseOutbound     = "reverse_outbound"
	ActionOrderCreated        = "order_created"
	ActionOrderUpdated        = "order_updated"
	ActionOrderConfirmed      = "order_confirmed"
	ActionOrderCancelled      = "order_cancelled"
	ActionOrderShipped        = "order_shipped"
	ActionReservationCreated  = "reservation_created"
	ActionReservationReleased = "reservation_released"
	ActionPickTaskCreated     = "pick_task_created"
	ActionPickEventRecorded   = "pick_event_recorded"
	ActionPickTaskCancelled   = "pick_task_cancelled"
	ActionReturnCreated       = "return_created"
	ActionReturnItemAdded     = "return_item_added"
	ActionReturnReceived      = "return_received"
	ActionReturnCancelled     = "return_cancelled"

	EntityProduct     = "product"
	EntityLocation    = "location"
	EntityInbound     = "inbound"
	EntityOutbound    = "outbound"
	EntityStock       = "stock"
	EntityAdjustment  = "adjustment"
	EntityOrder       = "order"
	EntityReservation = "reservation"
	EntityPickTask    = "pick_task"
	EntityReturn      = "return"
	EntityReturnItem  = "return_item"
)
