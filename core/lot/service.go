package lot

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Service exposes lot business logic.
type Service struct {
	repo Repository
}

// NewService creates a new lot service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create creates a new lot.
func (s *Service) Create(ctx context.Context, productID primitive.ObjectID, lotNo string, expDate, mfgDate *time.Time) (*Lot, error) {
	if lotNo == "" {
		return nil, errors.New("lot_no is required")
	}
	// Check duplicate
	existing, err := s.repo.FindByProductAndLotNo(ctx, productID, lotNo)
	if err == nil && existing != nil {
		return nil, errors.New("lot already exists for this product")
	}
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, err
	}

	l := &Lot{
		ProductID: productID,
		LotNo:     lotNo,
		ExpDate:   expDate,
		MfgDate:   mfgDate,
	}
	if err := s.repo.Create(ctx, l); err != nil {
		return nil, err
	}
	return l, nil
}

// FindByID returns a lot by its ID.
func (s *Service) FindByID(ctx context.Context, id primitive.ObjectID) (*Lot, error) {
	return s.repo.FindByID(ctx, id)
}

// ListByProduct returns all lots for a product.
func (s *Service) ListByProduct(ctx context.Context, productID primitive.ObjectID) ([]*Lot, error) {
	return s.repo.ListByProduct(ctx, productID)
}

// ListAll returns paginated lots.
func (s *Service) ListAll(ctx context.Context, page, limit int) ([]*Lot, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.repo.ListAll(ctx, page, limit)
}

// FindOrCreate atomically resolves a lot by (productID, lotNo),
// creating it if it doesn't exist. Used by inbound and returns.
func (s *Service) FindOrCreate(ctx context.Context, productID primitive.ObjectID, lotNo string, expDate, mfgDate *time.Time) (*Lot, error) {
	if lotNo == "" {
		return nil, errors.New("lot_no is required")
	}
	return s.repo.FindOrCreate(ctx, productID, lotNo, expDate, mfgDate)
}

// FindExpiring returns lots expiring within the given number of days.
func (s *Service) FindExpiring(ctx context.Context, days int) ([]*Lot, error) {
	before := time.Now().UTC().AddDate(0, 0, days)
	return s.repo.FindExpiring(ctx, before)
}
