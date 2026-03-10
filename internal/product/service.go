package product

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

func (s *Service) Create(ctx context.Context, req *CreateProductRequest) (*Product, error) {
	existing, _ := s.repo.FindBySKU(ctx, req.SKU)
	if existing != nil {
		return nil, errors.New("product with this SKU already exists")
	}

	product := &Product{
		SKU:         req.SKU,
		Name:        req.Name,
		Description: req.Description,
		Unit:        req.Unit,
	}

	if err := s.repo.Create(ctx, product); err != nil {
		return nil, err
	}
	return product, nil
}

func (s *Service) GetByID(ctx context.Context, id primitive.ObjectID) (*Product, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *Service) List(ctx context.Context, page, limit int) ([]*Product, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.repo.List(ctx, page, limit)
}

func (s *Service) Update(ctx context.Context, id primitive.ObjectID, req *UpdateProductRequest) (*Product, error) {
	update := bson.M{}
	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.Description != nil {
		update["description"] = *req.Description
	}
	if req.Unit != nil {
		update["unit"] = *req.Unit
	}
	if req.LowStockThreshold != nil {
		update["low_stock_threshold"] = *req.LowStockThreshold
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
