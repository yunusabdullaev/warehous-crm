package returns

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	historyPkg "warehouse-crm/core/history"
	lotPkg "warehouse-crm/core/lot"
	notifyPkg "warehouse-crm/core/notify"
	orderPkg "warehouse-crm/core/order"
	stockPkg "warehouse-crm/core/stock"
)

type Service struct {
	repo       *Repository
	orderSvc   *orderPkg.Service
	stockSvc   *stockPkg.Service
	historySvc *historyPkg.Service
	notifySvc  *notifyPkg.Service
	lotSvc     *lotPkg.Service
}

func NewService(
	repo *Repository,
	orderSvc *orderPkg.Service,
	stockSvc *stockPkg.Service,
	historySvc *historyPkg.Service,
	notifySvc *notifyPkg.Service,
	lotSvc *lotPkg.Service,
) *Service {
	return &Service{
		repo:       repo,
		orderSvc:   orderSvc,
		stockSvc:   stockSvc,
		historySvc: historySvc,
		notifySvc:  notifySvc,
		lotSvc:     lotSvc,
	}
}

// ── DTOs ──

type CreateReturnRequest struct {
	OrderID string `json:"order_id"`
	Notes   string `json:"notes,omitempty"`
}

type AddItemRequest struct {
	ProductID   string `json:"product_id"`
	LocationID  string `json:"location_id,omitempty"`
	LotNo       string `json:"lot_no,omitempty"`
	LotID       string `json:"lot_id,omitempty"`
	Qty         int    `json:"qty"`
	Disposition string `json:"disposition"`
	Note        string `json:"note,omitempty"`
}

type ReturnWithItems struct {
	Return *Return       `json:"return"`
	Items  []*ReturnItem `json:"items"`
}

// ── Business Logic ──

// Create creates a new RMA for a shipped order.
func (s *Service) Create(ctx context.Context, warehouseID primitive.ObjectID, req *CreateReturnRequest, userID string) (*Return, error) {
	orderID, err := primitive.ObjectIDFromHex(req.OrderID)
	if err != nil {
		return nil, errors.New("invalid order_id")
	}

	ord, err := s.orderSvc.GetByID(ctx, orderID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("order not found")
		}
		return nil, err
	}
	if ord.Status != orderPkg.StatusShipped {
		return nil, fmt.Errorf("order must be SHIPPED to create a return (current: %s)", ord.Status)
	}

	rmaNo, err := s.repo.NextRMANo(ctx)
	if err != nil {
		return nil, fmt.Errorf("generate RMA number: %w", err)
	}

	ret := &Return{
		WarehouseID: warehouseID,
		RMANo:       rmaNo,
		OrderID:     orderID,
		OrderNo:     ord.OrderNo,
		ClientName:  ord.ClientName,
		Notes:       req.Notes,
		CreatedBy:   userID,
	}

	if err := s.repo.Create(ctx, ret); err != nil {
		return nil, err
	}

	_ = s.historySvc.RecordAction(ctx,
		historyPkg.ActionReturnCreated, historyPkg.EntityReturn,
		ret.ID.Hex(), userID,
		fmt.Sprintf("RMA %s created for order %s", rmaNo, ord.OrderNo),
	)

	if s.notifySvc != nil {
		go s.notifySvc.NotifyReturnCreated(rmaNo, ord.OrderNo, ord.ClientName)
	}

	return ret, nil
}

// GetByID returns a return with its items.
func (s *Service) GetByID(ctx context.Context, id primitive.ObjectID) (*ReturnWithItems, error) {
	ret, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.ListItems(ctx, id)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []*ReturnItem{}
	}
	return &ReturnWithItems{Return: ret, Items: items}, nil
}

// List returns paginated returns with optional filters.
func (s *Service) List(ctx context.Context, filter bson.M, page, limit int) ([]*Return, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.repo.List(ctx, filter, page, limit)
}

// AddItem adds a return item with qty guard and disposition handling — lot-aware.
func (s *Service) AddItem(ctx context.Context, returnID primitive.ObjectID, req *AddItemRequest, userID string) (*ReturnItem, error) {
	// Validate return exists and is OPEN
	ret, err := s.repo.FindByID(ctx, returnID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("return not found")
		}
		return nil, err
	}
	if ret.Status != StatusOpen {
		return nil, fmt.Errorf("return must be OPEN to add items (current: %s)", ret.Status)
	}

	// Validate inputs
	productID, err := primitive.ObjectIDFromHex(req.ProductID)
	if err != nil {
		return nil, errors.New("invalid product_id")
	}
	if req.Qty <= 0 {
		return nil, errors.New("qty must be > 0")
	}
	if req.Disposition != DispositionRestock && req.Disposition != DispositionDamaged && req.Disposition != DispositionQCHold {
		return nil, errors.New("disposition must be RESTOCK, DAMAGED, or QC_HOLD")
	}

	var locationID *primitive.ObjectID
	if req.Disposition == DispositionRestock {
		if req.LocationID == "" {
			return nil, errors.New("location_id is required for RESTOCK disposition")
		}
		lid, err := primitive.ObjectIDFromHex(req.LocationID)
		if err != nil {
			return nil, errors.New("invalid location_id")
		}
		locationID = &lid
	}

	// Resolve lot (required for RESTOCK, optional for others)
	var lotID *primitive.ObjectID
	if req.LotID != "" {
		lid, err := primitive.ObjectIDFromHex(req.LotID)
		if err != nil {
			return nil, errors.New("invalid lot_id")
		}
		lotID = &lid
	} else if req.LotNo != "" {
		lot, err := s.lotSvc.FindOrCreate(ctx, productID, req.LotNo, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("resolve lot: %w", err)
		}
		lotID = &lot.ID
	}

	if req.Disposition == DispositionRestock && lotID == nil {
		return nil, errors.New("lot_no or lot_id is required for RESTOCK disposition")
	}

	// ── Qty guard: check shipped qty for this product ──
	ord, err := s.orderSvc.GetByID(ctx, ret.OrderID)
	if err != nil {
		return nil, fmt.Errorf("lookup order: %w", err)
	}

	var shippedQty int
	for _, item := range ord.Items {
		if item.ProductID == productID {
			shippedQty = item.ShippedQty
			break
		}
	}
	if shippedQty == 0 {
		return nil, fmt.Errorf("product %s was not shipped in order %s", req.ProductID, ord.OrderNo)
	}

	alreadyReturned, err := s.repo.SumReturnedQty(ctx, ret.OrderID, productID)
	if err != nil {
		return nil, fmt.Errorf("check returned qty: %w", err)
	}
	if alreadyReturned+req.Qty > shippedQty {
		return nil, fmt.Errorf("return qty exceeds shipped: shipped=%d, already_returned=%d, requested=%d", shippedQty, alreadyReturned, req.Qty)
	}

	// ── Create the item ──
	item := &ReturnItem{
		ReturnID:    returnID,
		ProductID:   productID,
		LocationID:  locationID,
		LotID:       lotID,
		Qty:         req.Qty,
		Disposition: req.Disposition,
		Note:        req.Note,
		CreatedBy:   userID,
	}
	if err := s.repo.CreateItem(ctx, item); err != nil {
		return nil, err
	}

	// ── Disposition side effects ──
	switch req.Disposition {
	case DispositionRestock:
		// Atomically add stock back — lot-aware
		if err := s.stockSvc.AddStock(ctx, item.WarehouseID, productID, *locationID, *lotID, req.Qty); err != nil {
			return nil, fmt.Errorf("restock failed: %w", err)
		}

	case DispositionDamaged:
		// No stock change — record as adjustment with reason DAMAGED for audit

	case DispositionQCHold:
		hold := &QCHold{
			ReturnID:  returnID,
			ProductID: productID,
			LotID:     lotID,
			Qty:       req.Qty,
		}
		if err := s.repo.CreateQCHold(ctx, hold); err != nil {
			return nil, fmt.Errorf("create QC hold: %w", err)
		}
	}

	_ = s.historySvc.RecordAction(ctx,
		historyPkg.ActionReturnItemAdded, historyPkg.EntityReturnItem,
		item.ID.Hex(), userID,
		fmt.Sprintf("RMA %s: %s %d units of product %s (%s)", ret.RMANo, req.Disposition, req.Qty, req.ProductID, req.Note),
	)

	return item, nil
}

// Receive transitions OPEN → RECEIVED.
func (s *Service) Receive(ctx context.Context, id primitive.ObjectID, userID string) (*Return, error) {
	now := time.Now().UTC()
	err := s.repo.UpdateStatus(ctx, id, StatusOpen, StatusReceived, bson.M{
		"received_at": now,
		"received_by": userID,
	})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("return not found or not in OPEN status")
		}
		return nil, err
	}

	ret, _ := s.repo.FindByID(ctx, id)
	items, _ := s.repo.ListItems(ctx, id)

	_ = s.historySvc.RecordAction(ctx,
		historyPkg.ActionReturnReceived, historyPkg.EntityReturn,
		id.Hex(), userID,
		fmt.Sprintf("RMA %s received with %d items", ret.RMANo, len(items)),
	)

	if s.notifySvc != nil {
		go s.notifySvc.NotifyReturnReceived(ret.RMANo, ret.OrderNo, len(items))
	}

	return ret, nil
}

// Cancel transitions OPEN → CANCELLED.
func (s *Service) Cancel(ctx context.Context, id primitive.ObjectID, userID string) (*Return, error) {
	err := s.repo.UpdateStatus(ctx, id, StatusOpen, StatusCancelled, nil)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("return not found or not in OPEN status")
		}
		return nil, err
	}

	ret, _ := s.repo.FindByID(ctx, id)

	_ = s.historySvc.RecordAction(ctx,
		historyPkg.ActionReturnCancelled, historyPkg.EntityReturn,
		id.Hex(), userID,
		fmt.Sprintf("RMA %s cancelled", ret.RMANo),
	)

	return ret, nil
}
