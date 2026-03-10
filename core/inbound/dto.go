package inbound

type CreateInboundRequest struct {
	ProductID  string  `json:"product_id"`
	LocationID string  `json:"location_id"`
	Quantity   int     `json:"quantity"`
	Reference  string  `json:"reference,omitempty"`
	LotNo      string  `json:"lot_no"`
	LotID      string  `json:"lot_id,omitempty"`
	ExpDate    *string `json:"exp_date,omitempty"`
	MfgDate    *string `json:"mfg_date,omitempty"`
}

type ReverseRequest struct {
	Reason string `json:"reason"`
}

type InboundResponse struct {
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

func ToResponse(i *Inbound) *InboundResponse {
	resp := &InboundResponse{
		ID:            i.ID.Hex(),
		ProductID:     i.ProductID.Hex(),
		LocationID:    i.LocationID.Hex(),
		LotID:         i.LotID.Hex(),
		Quantity:      i.Quantity,
		Reference:     i.Reference,
		UserID:        i.UserID,
		Status:        i.Status,
		ReversedBy:    i.ReversedBy,
		ReverseReason: i.ReverseReason,
		CreatedAt:     i.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if resp.Status == "" {
		resp.Status = StatusActive
	}
	if i.ReversedAt != nil {
		s := i.ReversedAt.Format("2006-01-02T15:04:05Z")
		resp.ReversedAt = &s
	}
	return resp
}
