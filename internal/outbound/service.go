package outbound

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	historyPkg "warehouse-crm/internal/history"
	stockPkg "warehouse-crm/internal/stock"
)

type Service struct {
	repo       Repository
	stockSvc   *stockPkg.Service
	historySvc *historyPkg.Service
}

func NewService(repo Repository, stockSvc *stockPkg.Service, historySvc *historyPkg.Service) *Service {
	return &Service{
		repo:       repo,
		stockSvc:   stockSvc,
		historySvc: historySvc,
	}
}

func (s *Service) Create(ctx context.Context, warehouseID primitive.ObjectID, req *CreateOutboundRequest, userID string) (*Outbound, error) {
	productID, err := primitive.ObjectIDFromHex(req.ProductID)
	if err != nil {
		return nil, errors.New("invalid product_id")
	}
	locationID, err := primitive.ObjectIDFromHex(req.LocationID)
	if err != nil {
		return nil, errors.New("invalid location_id")
	}
	lotID, err := primitive.ObjectIDFromHex(req.LotID)
	if err != nil {
		return nil, errors.New("invalid lot_id")
	}
	if req.Quantity <= 0 {
		return nil, errors.New("quantity must be positive")
	}

	// Check and remove stock (insufficient stock guard inside) — lot-aware
	if err := s.stockSvc.RemoveStock(ctx, productID, locationID, lotID, req.Quantity); err != nil {
		return nil, err
	}

	out := &Outbound{
		WarehouseID: warehouseID,
		ProductID:   productID,
		LocationID:  locationID,
		LotID:       lotID,
		Quantity:    req.Quantity,
		Reference:   req.Reference,
		UserID:      userID,
	}

	if err := s.repo.Create(ctx, out); err != nil {
		return nil, err
	}

	// Record history
	_ = s.historySvc.RecordAction(ctx, historyPkg.ActionOutbound, historyPkg.EntityOutbound, out.ID.Hex(), userID,
		fmt.Sprintf("Outbound %d units of product %s from location %s (lot %s)", req.Quantity, req.ProductID, req.LocationID, req.LotID))

	return out, nil
}

// Reverse marks an outbound record as REVERSED and adds the stock back.
// RBAC rules: admin can reverse anything; operator can only reverse own records within 24h.
func (s *Service) Reverse(ctx context.Context, id primitive.ObjectID, reason, userID, userRole string) (*Outbound, error) {
	out, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("outbound record not found")
		}
		return nil, err
	}

	// Check if already reversed
	if out.Status == StatusReversed {
		return nil, ErrAlreadyReversed
	}

	// RBAC: operator can only reverse own records within 24h
	if userRole == "operator" {
		if out.UserID != userID {
			return nil, ErrForbidden
		}
		if time.Since(out.CreatedAt) > 24*time.Hour {
			return nil, ErrForbidden
		}
	}

	// Atomically mark as reversed
	if err := s.repo.MarkReversed(ctx, id, userID, reason); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrAlreadyReversed
		}
		return nil, err
	}

	// Reverse the stock: add back what was removed — lot-aware
	if err := s.stockSvc.AddStock(ctx, out.WarehouseID, out.ProductID, out.LocationID, out.LotID, out.Quantity); err != nil {
		return nil, err
	}

	// Record history
	_ = s.historySvc.RecordAction(ctx, historyPkg.ActionReverseOutbound, historyPkg.EntityOutbound, out.ID.Hex(), userID,
		fmt.Sprintf("Reversed outbound of %d units (reason: %s), original record: %s", out.Quantity, reason, out.ID.Hex()))

	// Re-fetch to return updated state
	return s.repo.FindByID(ctx, id)
}

func (s *Service) GetByID(ctx context.Context, id primitive.ObjectID) (*Outbound, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *Service) List(ctx context.Context, warehouseID primitive.ObjectID, page, limit int) ([]*Outbound, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.repo.List(ctx, warehouseID, page, limit)
}

func (s *Service) ListByReference(ctx context.Context, reference string) ([]*Outbound, error) {
	return s.repo.FindByReference(ctx, reference)
}

// Sentinel errors
var (
	ErrAlreadyReversed = errors.New("ALREADY_REVERSED")
	ErrForbidden       = errors.New("FORBIDDEN")
)
