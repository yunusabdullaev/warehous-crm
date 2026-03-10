package inbound

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	historyPkg "warehouse-crm/internal/history"
	lotPkg "warehouse-crm/internal/lot"
	stockPkg "warehouse-crm/internal/stock"
)

type Service struct {
	repo       Repository
	stockSvc   *stockPkg.Service
	historySvc *historyPkg.Service
	lotSvc     *lotPkg.Service
}

func NewService(repo Repository, stockSvc *stockPkg.Service, historySvc *historyPkg.Service, lotSvc *lotPkg.Service) *Service {
	return &Service{
		repo:       repo,
		stockSvc:   stockSvc,
		historySvc: historySvc,
		lotSvc:     lotSvc,
	}
}

func (s *Service) Create(ctx context.Context, warehouseID primitive.ObjectID, req *CreateInboundRequest, userID string) (*Inbound, error) {
	productID, err := primitive.ObjectIDFromHex(req.ProductID)
	if err != nil {
		return nil, errors.New("invalid product_id")
	}
	locationID, err := primitive.ObjectIDFromHex(req.LocationID)
	if err != nil {
		return nil, errors.New("invalid location_id")
	}
	if req.Quantity <= 0 {
		return nil, errors.New("quantity must be positive")
	}

	// Resolve lot: either by lotId or by lotNo (find-or-create)
	var lotID primitive.ObjectID
	if req.LotID != "" {
		lotID, err = primitive.ObjectIDFromHex(req.LotID)
		if err != nil {
			return nil, errors.New("invalid lot_id")
		}
		// Verify lot exists
		_, err = s.lotSvc.FindByID(ctx, lotID)
		if err != nil {
			return nil, errors.New("lot not found")
		}
	} else if req.LotNo != "" {
		var expDate, mfgDate *time.Time
		if req.ExpDate != nil && *req.ExpDate != "" {
			t, err := time.Parse("2006-01-02", *req.ExpDate)
			if err != nil {
				return nil, errors.New("invalid exp_date format, use YYYY-MM-DD")
			}
			expDate = &t
		}
		if req.MfgDate != nil && *req.MfgDate != "" {
			t, err := time.Parse("2006-01-02", *req.MfgDate)
			if err != nil {
				return nil, errors.New("invalid mfg_date format, use YYYY-MM-DD")
			}
			mfgDate = &t
		}
		lot, err := s.lotSvc.FindOrCreate(ctx, productID, req.LotNo, expDate, mfgDate)
		if err != nil {
			return nil, fmt.Errorf("resolve lot: %w", err)
		}
		lotID = lot.ID
	} else {
		return nil, errors.New("lot_no or lot_id is required")
	}

	inb := &Inbound{
		WarehouseID: warehouseID,
		ProductID:   productID,
		LocationID:  locationID,
		LotID:       lotID,
		Quantity:    req.Quantity,
		Reference:   req.Reference,
		UserID:      userID,
	}

	if err := s.repo.Create(ctx, inb); err != nil {
		return nil, err
	}

	// Update stock
	if err := s.stockSvc.AddStock(ctx, warehouseID, productID, locationID, lotID, req.Quantity); err != nil {
		return nil, err
	}

	// Record history
	_ = s.historySvc.RecordAction(ctx, historyPkg.ActionInbound, historyPkg.EntityInbound, inb.ID.Hex(), userID,
		fmt.Sprintf("Inbound %d units of product %s to location %s (lot %s)", req.Quantity, req.ProductID, req.LocationID, lotID.Hex()))

	return inb, nil
}

// Reverse marks an inbound record as REVERSED and removes the stock that was added.
// RBAC rules: admin can reverse anything; operator can only reverse own records within 24h.
func (s *Service) Reverse(ctx context.Context, id primitive.ObjectID, reason, userID, userRole string) (*Inbound, error) {
	inb, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("inbound record not found")
		}
		return nil, err
	}

	// Check if already reversed
	if inb.Status == StatusReversed {
		return nil, ErrAlreadyReversed
	}

	// RBAC: operator can only reverse own records within 24h
	if userRole == "operator" {
		if inb.UserID != userID {
			return nil, ErrForbidden
		}
		if time.Since(inb.CreatedAt) > 24*time.Hour {
			return nil, ErrForbidden
		}
	}

	// Atomically mark as reversed (guard against double-reversal race)
	if err := s.repo.MarkReversed(ctx, id, userID, reason); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrAlreadyReversed
		}
		return nil, err
	}

	// Reverse the stock: remove what was added (lot-aware)
	if err := s.stockSvc.AdjustStock(ctx, inb.WarehouseID, inb.ProductID, inb.LocationID, inb.LotID, -inb.Quantity); err != nil {
		return nil, err
	}

	// Record history
	_ = s.historySvc.RecordAction(ctx, historyPkg.ActionReverseInbound, historyPkg.EntityInbound, inb.ID.Hex(), userID,
		fmt.Sprintf("Reversed inbound of %d units (reason: %s), original record: %s", inb.Quantity, reason, inb.ID.Hex()))

	// Re-fetch to return updated state
	return s.repo.FindByID(ctx, id)
}

func (s *Service) GetByID(ctx context.Context, id primitive.ObjectID) (*Inbound, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *Service) List(ctx context.Context, warehouseID primitive.ObjectID, page, limit int) ([]*Inbound, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.repo.List(ctx, warehouseID, page, limit)
}

// Sentinel errors
var (
	ErrAlreadyReversed = errors.New("ALREADY_REVERSED")
	ErrForbidden       = errors.New("FORBIDDEN")
)
