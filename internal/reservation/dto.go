package reservation

import "time"

type ReservationResponse struct {
	ID         string  `json:"id"`
	OrderID    string  `json:"order_id"`
	ProductID  string  `json:"product_id"`
	Qty        int     `json:"qty"`
	Status     string  `json:"status"`
	CreatedAt  string  `json:"created_at"`
	CreatedBy  string  `json:"created_by"`
	ReleasedAt *string `json:"released_at,omitempty"`
	ReleasedBy string  `json:"released_by,omitempty"`
	Reason     string  `json:"reason,omitempty"`
}

type ReleaseRequest struct {
	ReservationID string `json:"reservation_id"`
	Reason        string `json:"reason"`
}

func ToResponse(r *Reservation) *ReservationResponse {
	resp := &ReservationResponse{
		ID:         r.ID.Hex(),
		OrderID:    r.OrderID.Hex(),
		ProductID:  r.ProductID.Hex(),
		Qty:        r.Qty,
		Status:     r.Status,
		CreatedAt:  r.CreatedAt.Format(time.RFC3339),
		CreatedBy:  r.CreatedBy,
		ReleasedBy: r.ReleasedBy,
		Reason:     r.Reason,
	}
	if r.ReleasedAt != nil {
		s := r.ReleasedAt.Format(time.RFC3339)
		resp.ReleasedAt = &s
	}
	return resp
}
