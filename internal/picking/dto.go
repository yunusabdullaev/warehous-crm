package picking

import "time"

// ── Requests ──

type ScanRequest struct {
	LocationID string `json:"location_id"`
	ProductID  string `json:"product_id"`
	LotID      string `json:"lot_id"`
	Qty        int    `json:"qty"`
	Scanner    string `json:"scanner,omitempty"`
	Client     string `json:"client,omitempty"`
}

type AssignRequest struct {
	AssignTo string `json:"assign_to"`
}

// ── Responses ──

type PickTaskResponse struct {
	ID         string  `json:"id"`
	OrderID    string  `json:"order_id"`
	ProductID  string  `json:"product_id"`
	LocationID string  `json:"location_id"`
	LotID      string  `json:"lot_id"`
	PlannedQty int     `json:"planned_qty"`
	PickedQty  int     `json:"picked_qty"`
	Status     string  `json:"status"`
	AssignedTo *string `json:"assigned_to,omitempty"`
	CreatedBy  string  `json:"created_by"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

type PickEventResponse struct {
	ID         string `json:"id"`
	OrderID    string `json:"order_id"`
	PickTaskID string `json:"pick_task_id"`
	UserID     string `json:"user_id"`
	LocationID string `json:"location_id"`
	ProductID  string `json:"product_id"`
	LotID      string `json:"lot_id"`
	Qty        int    `json:"qty"`
	ScannedAt  string `json:"scanned_at"`
	Scanner    string `json:"scanner,omitempty"`
	Client     string `json:"client,omitempty"`
}

func TaskToResponse(t *PickTask) *PickTaskResponse {
	r := &PickTaskResponse{
		ID:         t.ID.Hex(),
		OrderID:    t.OrderID.Hex(),
		ProductID:  t.ProductID.Hex(),
		LocationID: t.LocationID.Hex(),
		LotID:      t.LotID.Hex(),
		PlannedQty: t.PlannedQty,
		PickedQty:  t.PickedQty,
		Status:     t.Status,
		AssignedTo: t.AssignedTo,
		CreatedBy:  t.CreatedBy,
		CreatedAt:  t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  t.UpdatedAt.Format(time.RFC3339),
	}
	return r
}

func EventToResponse(e *PickEvent) *PickEventResponse {
	return &PickEventResponse{
		ID:         e.ID.Hex(),
		OrderID:    e.OrderID.Hex(),
		PickTaskID: e.PickTaskID.Hex(),
		UserID:     e.UserID,
		LocationID: e.LocationID.Hex(),
		ProductID:  e.ProductID.Hex(),
		LotID:      e.LotID.Hex(),
		Qty:        e.Qty,
		ScannedAt:  e.ScannedAt.Format(time.RFC3339),
		Scanner:    e.Meta.Scanner,
		Client:     e.Meta.Client,
	}
}
