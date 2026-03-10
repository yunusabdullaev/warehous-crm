package outbound

type CreateOutboundRequest struct {
	ProductID  string `json:"product_id"`
	LocationID string `json:"location_id"`
	LotID      string `json:"lot_id"`
	Quantity   int    `json:"quantity"`
	Reference  string `json:"reference,omitempty"`
}

type ReverseRequest struct {
	Reason string `json:"reason"`
}

type OutboundResponse struct {
	ID            string  `json:"id"`
	ProductID     string  `json:"product_id"`
	LocationID    string  `json:"location_id"`
	LotID         string  `json:"lot_id"`
	Quantity      int     `json:"quantity"`
	Reference     string  `json:"reference,omitempty"`
	UserID        string  `json:"user_id"`
	Status        string  `json:"status"`
	ReversedAt    *string `json:"reversed_at,omitempty"`
	ReversedBy    string  `json:"reversed_by,omitempty"`
	ReverseReason string  `json:"reverse_reason,omitempty"`
	CreatedAt     string  `json:"created_at"`
}

func ToResponse(o *Outbound) *OutboundResponse {
	resp := &OutboundResponse{
		ID:            o.ID.Hex(),
		ProductID:     o.ProductID.Hex(),
		LocationID:    o.LocationID.Hex(),
		LotID:         o.LotID.Hex(),
		Quantity:      o.Quantity,
		Reference:     o.Reference,
		UserID:        o.UserID,
		Status:        o.Status,
		ReversedBy:    o.ReversedBy,
		ReverseReason: o.ReverseReason,
		CreatedAt:     o.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if resp.Status == "" {
		resp.Status = StatusActive
	}
	if o.ReversedAt != nil {
		s := o.ReversedAt.Format("2006-01-02T15:04:05Z")
		resp.ReversedAt = &s
	}
	return resp
}
