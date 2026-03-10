package history

import "go.mongodb.org/mongo-driver/bson/primitive"

type HistoryResponse struct {
	ID         string `json:"id"`
	Action     string `json:"action"`
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	UserID     string `json:"user_id"`
	Details    string `json:"details,omitempty"`
	Timestamp  string `json:"timestamp"`
}

type HistoryQuery struct {
	UserID      string             `json:"user_id,omitempty"`
	EntityType  string             `json:"entity_type,omitempty"`
	EntityID    string             `json:"entity_id,omitempty"`
	WarehouseID primitive.ObjectID `json:"-"`
}

func ToResponse(h *History) *HistoryResponse {
	return &HistoryResponse{
		ID:         h.ID.Hex(),
		Action:     h.Action,
		EntityType: h.EntityType,
		EntityID:   h.EntityID,
		UserID:     h.UserID,
		Details:    h.Details,
		Timestamp:  h.Timestamp.Format("2006-01-02T15:04:05Z"),
	}
}
