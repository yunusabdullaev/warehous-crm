package adjustment

type CreateAdjustmentRequest struct {
	ProductID     string `json:"product_id"`
	LocationID    string `json:"location_id"`
	LotID         string `json:"lot_id,omitempty"`
	DeltaQty      int    `json:"delta_qty"`
	Reason        string `json:"reason"`
	Note          string `json:"note,omitempty"`
	AllowNegative bool   `json:"allow_negative"` // default false; only admin can set true
}

type AdjustmentResponse struct {
	ID         string `json:"id"`
	ProductID  string `json:"product_id"`
	LocationID string `json:"location_id"`
	LotID      string `json:"lot_id,omitempty"`
	DeltaQty   int    `json:"delta_qty"`
	Reason     string `json:"reason"`
	Note       string `json:"note,omitempty"`
	CreatedBy  string `json:"created_by"`
	CreatedAt  string `json:"created_at"`
}

func ToResponse(a *Adjustment) *AdjustmentResponse {
	resp := &AdjustmentResponse{
		ID:         a.ID.Hex(),
		ProductID:  a.ProductID.Hex(),
		LocationID: a.LocationID.Hex(),
		DeltaQty:   a.DeltaQty,
		Reason:     a.Reason,
		Note:       a.Note,
		CreatedBy:  a.CreatedBy,
		CreatedAt:  a.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if a.LotID != nil {
		resp.LotID = a.LotID.Hex()
	}
	return resp
}
