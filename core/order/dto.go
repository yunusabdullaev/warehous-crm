package order

import "time"

// ── Requests ──

type OrderItemRequest struct {
	ProductID    string `json:"product_id"`
	RequestedQty int    `json:"requested_qty"`
}

type CreateOrderRequest struct {
	ClientName string             `json:"client_name"`
	Notes      string             `json:"notes,omitempty"`
	Items      []OrderItemRequest `json:"items"`
}

type UpdateOrderRequest struct {
	ClientName string             `json:"client_name,omitempty"`
	Notes      string             `json:"notes,omitempty"`
	Items      []OrderItemRequest `json:"items,omitempty"`
}

// ── Responses ──

type OrderItemResponse struct {
	ProductID    string `json:"product_id"`
	RequestedQty int    `json:"requested_qty"`
	ReservedQty  int    `json:"reserved_qty"`
	ShippedQty   int    `json:"shipped_qty"`
}

type OrderResponse struct {
	ID          string              `json:"id"`
	OrderNo     string              `json:"order_no"`
	ClientName  string              `json:"client_name"`
	Status      string              `json:"status"`
	Notes       string              `json:"notes,omitempty"`
	Items       []OrderItemResponse `json:"items"`
	CreatedBy   string              `json:"created_by"`
	CreatedAt   string              `json:"created_at"`
	ConfirmedAt *string             `json:"confirmed_at,omitempty"`
	ShippedAt   *string             `json:"shipped_at,omitempty"`
	CancelledAt *string             `json:"cancelled_at,omitempty"`
}

func ToResponse(o *Order) *OrderResponse {
	items := make([]OrderItemResponse, len(o.Items))
	for i, it := range o.Items {
		items[i] = OrderItemResponse{
			ProductID:    it.ProductID.Hex(),
			RequestedQty: it.RequestedQty,
			ReservedQty:  it.ReservedQty,
			ShippedQty:   it.ShippedQty,
		}
	}
	resp := &OrderResponse{
		ID:         o.ID.Hex(),
		OrderNo:    o.OrderNo,
		ClientName: o.ClientName,
		Status:     o.Status,
		Notes:      o.Notes,
		Items:      items,
		CreatedBy:  o.CreatedBy,
		CreatedAt:  o.CreatedAt.Format(time.RFC3339),
	}
	if o.ConfirmedAt != nil {
		s := o.ConfirmedAt.Format(time.RFC3339)
		resp.ConfirmedAt = &s
	}
	if o.ShippedAt != nil {
		s := o.ShippedAt.Format(time.RFC3339)
		resp.ShippedAt = &s
	}
	if o.CancelledAt != nil {
		s := o.CancelledAt.Format(time.RFC3339)
		resp.CancelledAt = &s
	}
	return resp
}
