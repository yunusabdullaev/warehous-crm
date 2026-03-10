package stock

type StockResponse struct {
	ID           string `json:"id"`
	ProductID    string `json:"product_id"`
	LocationID   string `json:"location_id"`
	LotID        string `json:"lot_id"`
	Quantity     int    `json:"quantity"`
	ReservedQty  int    `json:"reserved_qty"`
	AvailableQty int    `json:"available_qty"`
	LastUpdated  string `json:"last_updated"`
}

type StockQuery struct {
	ProductID  string `json:"product_id,omitempty"`
	LocationID string `json:"location_id,omitempty"`
	LotID      string `json:"lot_id,omitempty"`
}

func ToResponse(s *Stock) *StockResponse {
	return &StockResponse{
		ID:           s.ID.Hex(),
		ProductID:    s.ProductID.Hex(),
		LocationID:   s.LocationID.Hex(),
		LotID:        s.LotID.Hex(),
		Quantity:     s.Quantity,
		ReservedQty:  0,
		AvailableQty: s.Quantity,
		LastUpdated:  s.LastUpdated.Format("2006-01-02T15:04:05Z"),
	}
}

// ToResponseWithReservation creates a response enriched with reservation data.
func ToResponseWithReservation(s *Stock, reservedQty int) *StockResponse {
	available := s.Quantity - reservedQty
	if available < 0 {
		available = 0
	}
	return &StockResponse{
		ID:           s.ID.Hex(),
		ProductID:    s.ProductID.Hex(),
		LocationID:   s.LocationID.Hex(),
		LotID:        s.LotID.Hex(),
		Quantity:     s.Quantity,
		ReservedQty:  reservedQty,
		AvailableQty: available,
		LastUpdated:  s.LastUpdated.Format("2006-01-02T15:04:05Z"),
	}
}
