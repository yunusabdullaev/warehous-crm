package stock

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) AddStock(ctx context.Context, warehouseID, productID, locationID, lotID primitive.ObjectID, qty int) error {
	if qty <= 0 {
		return errors.New("quantity must be positive")
	}
	return s.repo.Upsert(ctx, warehouseID, productID, locationID, lotID, qty)
}

func (s *Service) RemoveStock(ctx context.Context, productID, locationID, lotID primitive.ObjectID, qty int) error {
	if qty <= 0 {
		return errors.New("quantity must be positive")
	}

	existing, err := s.repo.FindByProductLocationLot(ctx, productID, locationID, lotID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("no stock found for this product/location/lot")
		}
		return err
	}

	if existing.Quantity < qty {
		return errors.New("insufficient stock")
	}

	// Use the existing stock's warehouseID
	return s.repo.Upsert(ctx, existing.WarehouseID, productID, locationID, lotID, -qty)
}

func (s *Service) GetByProductLocationLot(ctx context.Context, productID, locationID, lotID primitive.ObjectID) (*Stock, error) {
	return s.repo.FindByProductLocationLot(ctx, productID, locationID, lotID)
}

func (s *Service) ListByProduct(ctx context.Context, productID primitive.ObjectID) ([]*Stock, error) {
	return s.repo.ListByProduct(ctx, productID)
}

func (s *Service) ListByLocation(ctx context.Context, locationID primitive.ObjectID) ([]*Stock, error) {
	return s.repo.ListByLocation(ctx, locationID)
}

func (s *Service) AdjustStock(ctx context.Context, warehouseID, productID, locationID, lotID primitive.ObjectID, delta int) error {
	return s.repo.Upsert(ctx, warehouseID, productID, locationID, lotID, delta)
}

func (s *Service) ListAll(ctx context.Context, warehouseID primitive.ObjectID, page, limit int) ([]*Stock, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.repo.ListAll(ctx, warehouseID, page, limit)
}

func (s *Service) ListByLot(ctx context.Context, lotID primitive.ObjectID) ([]*Stock, error) {
	return s.repo.ListByLot(ctx, lotID)
}
