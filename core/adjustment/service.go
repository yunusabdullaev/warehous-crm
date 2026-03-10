package adjustment

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	historyPkg "warehouse-crm/core/history"
	stockPkg "warehouse-crm/core/stock"
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

func (s *Service) Create(ctx context.Context, warehouseID primitive.ObjectID, req *CreateAdjustmentRequest, userID string) (*Adjustment, error) {
	productID, err := primitive.ObjectIDFromHex(req.ProductID)
	if err != nil {
		return nil, errors.New("invalid product_id")
	}
	locationID, err := primitive.ObjectIDFromHex(req.LocationID)
	if err != nil {
		return nil, errors.New("invalid location_id")
	}
	if req.DeltaQty == 0 {
		return nil, errors.New("delta_qty cannot be zero")
	}
	if !ValidReasons[req.Reason] {
		return nil, errors.New("invalid reason; must be one of: DAMAGED, LOST, FOUND, COUNT_CORRECTION, OTHER")
	}

	// Resolve lotID — required for stock adjustment
	var lotID primitive.ObjectID
	if req.LotID != "" {
		lotID, err = primitive.ObjectIDFromHex(req.LotID)
		if err != nil {
			return nil, errors.New("invalid lot_id")
		}
	} else {
		return nil, errors.New("lot_id is required")
	}

	// Guard: if delta is negative, check resulting stock >= 0
	if req.DeltaQty < 0 && !req.AllowNegative {
		existing, err := s.stockSvc.GetByProductLocationLot(ctx, productID, locationID, lotID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return nil, errors.New("no stock found; cannot apply negative adjustment")
			}
			return nil, err
		}
		if existing.Quantity+req.DeltaQty < 0 {
			return nil, fmt.Errorf("insufficient stock: current %d, adjustment %d would result in negative", existing.Quantity, req.DeltaQty)
		}
	}

	// Apply to stock atomically via $inc — lot-aware
	if err := s.stockSvc.AdjustStock(ctx, warehouseID, productID, locationID, lotID, req.DeltaQty); err != nil {
		return nil, err
	}

	lotPtr := &lotID
	adj := &Adjustment{
		WarehouseID: warehouseID,
		ProductID:   productID,
		LocationID:  locationID,
		LotID:       lotPtr,
		DeltaQty:    req.DeltaQty,
		Reason:      req.Reason,
		Note:        req.Note,
		CreatedBy:   userID,
	}

	if err := s.repo.Create(ctx, adj); err != nil {
		return nil, err
	}

	// Record history
	_ = s.historySvc.RecordAction(ctx, historyPkg.ActionCreateAdjustment, historyPkg.EntityAdjustment, adj.ID.Hex(), userID,
		fmt.Sprintf("Adjustment %+d units (reason: %s) product %s at location %s lot %s", req.DeltaQty, req.Reason, req.ProductID, req.LocationID, req.LotID))

	return adj, nil
}

func (s *Service) List(ctx context.Context, filter bson.M, page, limit int) ([]*Adjustment, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.repo.List(ctx, filter, page, limit)
}
