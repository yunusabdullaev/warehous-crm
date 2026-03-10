package location

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, warehouseID primitive.ObjectID, req *CreateLocationRequest) (*Location, error) {
	existing, _ := s.repo.FindByCode(ctx, req.Code)
	if existing != nil {
		return nil, errors.New("location with this code already exists")
	}

	loc := &Location{
		WarehouseID: warehouseID,
		Code:        req.Code,
		Name:        req.Name,
		Zone:        req.Zone,
		Aisle:       req.Aisle,
		Rack:        req.Rack,
		Level:       req.Level,
	}

	if err := s.repo.Create(ctx, loc); err != nil {
		return nil, err
	}
	return loc, nil
}

func (s *Service) GetByID(ctx context.Context, id primitive.ObjectID) (*Location, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *Service) List(ctx context.Context, warehouseID primitive.ObjectID, page, limit int) ([]*Location, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.repo.List(ctx, warehouseID, page, limit)
}

func (s *Service) Update(ctx context.Context, id primitive.ObjectID, req *UpdateLocationRequest) (*Location, error) {
	update := bson.M{}
	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.Zone != nil {
		update["zone"] = *req.Zone
	}
	if req.Aisle != nil {
		update["aisle"] = *req.Aisle
	}
	if req.Rack != nil {
		update["rack"] = *req.Rack
	}
	if req.Level != nil {
		update["level"] = *req.Level
	}

	if len(update) == 0 {
		return nil, errors.New("no fields to update")
	}

	if err := s.repo.Update(ctx, id, update); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id primitive.ObjectID) error {
	return s.repo.Delete(ctx, id)
}
