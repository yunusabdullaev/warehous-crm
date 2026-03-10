package history

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) RecordAction(ctx context.Context, action, entityType, entityID, userID, details string) error {
	h := &History{
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		UserID:     userID,
		Details:    details,
	}
	return s.repo.Create(ctx, h)
}

func (s *Service) GetHistory(ctx context.Context, query *HistoryQuery, page, limit int) ([]*History, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	filter := bson.M{}
	if query.UserID != "" {
		filter["user_id"] = query.UserID
	}
	if query.EntityType != "" {
		filter["entity_type"] = query.EntityType
	}
	if query.EntityID != "" {
		filter["entity_id"] = query.EntityID
	}
	if !query.WarehouseID.IsZero() {
		filter["warehouse_id"] = query.WarehouseID
	}

	return s.repo.List(ctx, filter, page, limit)
}
