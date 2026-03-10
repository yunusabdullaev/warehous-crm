package order

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	historyPkg "warehouse-crm/core/history"
	notifyPkg "warehouse-crm/core/notify"
	outboundPkg "warehouse-crm/core/outbound"
	pickingPkg "warehouse-crm/core/picking"
	reservationPkg "warehouse-crm/core/reservation"
	stockPkg "warehouse-crm/core/stock"
)

var (
	ErrInvalidTransition  = errors.New("INVALID_STATUS_TRANSITION")
	ErrOrderNotFound      = errors.New("ORDER_NOT_FOUND")
	ErrNotDraft           = errors.New("ORDER_NOT_DRAFT")
	ErrPickingNotComplete = errors.New("PICKING_NOT_COMPLETE")
	ErrAdminRequired      = errors.New("ADMIN_REQUIRED")
)

type Service struct {
	repo           Repository
	reservationSvc *reservationPkg.Service
	stockSvc       *stockPkg.Service
	outboundSvc    *outboundPkg.Service
	historySvc     *historyPkg.Service
	pickingSvc     *pickingPkg.Service
	notifySvc      *notifyPkg.Service
}

func NewService(
	repo Repository,
	reservationSvc *reservationPkg.Service,
	stockSvc *stockPkg.Service,
	outboundSvc *outboundPkg.Service,
	historySvc *historyPkg.Service,
	pickingSvc *pickingPkg.Service,
	notifySvc *notifyPkg.Service,
) *Service {
	return &Service{
		repo:           repo,
		reservationSvc: reservationSvc,
		stockSvc:       stockSvc,
		outboundSvc:    outboundSvc,
		historySvc:     historySvc,
		pickingSvc:     pickingSvc,
		notifySvc:      notifySvc,
	}
}

func (s *Service) Create(ctx context.Context, warehouseID primitive.ObjectID, req *CreateOrderRequest, userID string) (*Order, error) {
	if req.ClientName == "" {
		return nil, errors.New("client_name is required")
	}
	if len(req.Items) == 0 {
		return nil, errors.New("at least one item is required")
	}

	items := make([]OrderItem, len(req.Items))
	for i, it := range req.Items {
		pid, err := primitive.ObjectIDFromHex(it.ProductID)
		if err != nil {
			return nil, fmt.Errorf("invalid product_id at index %d", i)
		}
		if it.RequestedQty <= 0 {
			return nil, fmt.Errorf("requested_qty must be positive at index %d", i)
		}
		items[i] = OrderItem{
			ProductID:    pid,
			RequestedQty: it.RequestedQty,
		}
	}

	orderNo, err := s.repo.NextOrderNo(ctx)
	if err != nil {
		return nil, fmt.Errorf("generate order number: %w", err)
	}

	order := &Order{
		WarehouseID: warehouseID,
		OrderNo:     orderNo,
		ClientName:  req.ClientName,
		Notes:       req.Notes,
		Items:       items,
		CreatedBy:   userID,
	}

	if err := s.repo.Create(ctx, order); err != nil {
		return nil, err
	}

	_ = s.historySvc.RecordAction(ctx,
		historyPkg.ActionOrderCreated, historyPkg.EntityOrder,
		order.ID.Hex(), userID,
		fmt.Sprintf("Order %s created with %d items for %s", order.OrderNo, len(items), req.ClientName),
	)

	return order, nil
}

func (s *Service) Update(ctx context.Context, id primitive.ObjectID, req *UpdateOrderRequest, userID string) (*Order, error) {
	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	if order.Status != StatusDraft {
		return nil, ErrNotDraft
	}

	if req.ClientName != "" {
		order.ClientName = req.ClientName
	}
	if req.Notes != "" {
		order.Notes = req.Notes
	}
	if len(req.Items) > 0 {
		items := make([]OrderItem, len(req.Items))
		for i, it := range req.Items {
			pid, err := primitive.ObjectIDFromHex(it.ProductID)
			if err != nil {
				return nil, fmt.Errorf("invalid product_id at index %d", i)
			}
			if it.RequestedQty <= 0 {
				return nil, fmt.Errorf("requested_qty must be positive at index %d", i)
			}
			items[i] = OrderItem{ProductID: pid, RequestedQty: it.RequestedQty}
		}
		order.Items = items
	}

	if err := s.repo.Update(ctx, order); err != nil {
		return nil, err
	}

	_ = s.historySvc.RecordAction(ctx,
		historyPkg.ActionOrderUpdated, historyPkg.EntityOrder,
		order.ID.Hex(), userID,
		fmt.Sprintf("Order %s updated", order.OrderNo),
	)

	return order, nil
}

func (s *Service) GetByID(ctx context.Context, id primitive.ObjectID) (*Order, error) {
	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	return order, nil
}

func (s *Service) List(ctx context.Context, filter bson.M, page, limit int) ([]*Order, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.repo.List(ctx, filter, page, limit)
}

// Confirm transitions DRAFT → CONFIRMED and creates reservations for each item.
func (s *Service) Confirm(ctx context.Context, id primitive.ObjectID, userID string) (*Order, error) {
	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	if order.Status != StatusDraft {
		return nil, ErrInvalidTransition
	}

	// Create reservations for each item
	for i, item := range order.Items {
		res, err := s.reservationSvc.Reserve(ctx, order.ID, item.ProductID, item.RequestedQty, userID)
		if err != nil {
			// Rollback reservations created so far
			_, _ = s.reservationSvc.ReleaseByOrder(ctx, order.ID, userID, "rollback: reservation failed")
			return nil, fmt.Errorf("reserve item %d: %w", i, err)
		}
		order.Items[i].ReservedQty = res.Qty
	}

	now := time.Now().UTC()
	setFields := bson.M{
		"confirmed_at": now,
		"items":        order.Items,
	}
	if err := s.repo.UpdateStatus(ctx, id, StatusDraft, StatusConfirmed, setFields); err != nil {
		// Rollback reservations
		_, _ = s.reservationSvc.ReleaseByOrder(ctx, order.ID, userID, "rollback: status update failed")
		if err == mongo.ErrNoDocuments {
			return nil, ErrInvalidTransition
		}
		return nil, err
	}

	_ = s.historySvc.RecordAction(ctx,
		historyPkg.ActionOrderConfirmed, historyPkg.EntityOrder,
		order.ID.Hex(), userID,
		fmt.Sprintf("Order %s confirmed, %d items reserved", order.OrderNo, len(order.Items)),
	)

	order.Status = StatusConfirmed
	order.ConfirmedAt = &now

	if s.notifySvc != nil {
		go s.notifySvc.NotifyOrderConfirmed(order.OrderNo, order.ClientName, len(order.Items))
	}

	return order, nil
}

// Cancel transitions DRAFT/CONFIRMED/PICKING → CANCELLED.
// PICKING cancel requires admin role (enforced in handler).
func (s *Service) Cancel(ctx context.Context, id primitive.ObjectID, userID, role string) (*Order, error) {
	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	if order.Status != StatusDraft && order.Status != StatusConfirmed && order.Status != StatusPicking {
		return nil, ErrInvalidTransition
	}

	// PICKING cancellation requires admin
	if order.Status == StatusPicking && role != "admin" {
		return nil, ErrAdminRequired
	}

	now := time.Now().UTC()

	// Try transitioning from current status
	err = s.repo.UpdateStatus(ctx, id, order.Status, StatusCancelled, bson.M{"cancelled_at": now})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrInvalidTransition
		}
		return nil, err
	}

	// Cancel pick tasks if in PICKING
	if order.Status == StatusPicking {
		_, _ = s.pickingSvc.CancelByOrder(ctx, order.ID, userID)
	}

	// Release reservations (if any were created during confirm)
	if order.Status == StatusConfirmed || order.Status == StatusPicking {
		_, _ = s.reservationSvc.ReleaseByOrder(ctx, order.ID, userID, "order cancelled")
	}

	_ = s.historySvc.RecordAction(ctx,
		historyPkg.ActionOrderCancelled, historyPkg.EntityOrder,
		order.ID.Hex(), userID,
		fmt.Sprintf("Order %s cancelled from %s", order.OrderNo, order.Status),
	)

	order.Status = StatusCancelled
	order.CancelledAt = &now
	return order, nil
}

// StartPick transitions CONFIRMED → PICKING and generates pick tasks.
// Blocks with error if insufficient stock to generate pick plan.
func (s *Service) StartPick(ctx context.Context, id primitive.ObjectID, userID string) (*Order, error) {
	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	if order.Status != StatusConfirmed {
		return nil, ErrInvalidTransition
	}

	// Build plan items from order items
	planItems := make([]pickingPkg.PlanItem, len(order.Items))
	for i, item := range order.Items {
		planItems[i] = pickingPkg.PlanItem{
			ProductID:    item.ProductID,
			RequestedQty: item.RequestedQty,
		}
	}

	// Generate pick plan (blocks if insufficient stock)
	_, err = s.pickingSvc.GeneratePlan(ctx, order.ID, planItems, userID)
	if err != nil {
		return nil, fmt.Errorf("generate pick plan: %w", err)
	}

	// Transition status
	if err := s.repo.UpdateStatus(ctx, id, StatusConfirmed, StatusPicking, nil); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrInvalidTransition
		}
		return nil, err
	}

	_ = s.historySvc.RecordAction(ctx,
		historyPkg.ActionOrderUpdated, historyPkg.EntityOrder,
		id.Hex(), userID, "Order picking started with pick plan generated",
	)

	if s.notifySvc != nil {
		go s.notifySvc.NotifyOrderPicking(order.OrderNo)
	}

	return s.repo.FindByID(ctx, id)
}

// Ship transitions PICKING → SHIPPED.
// Blocks unless ALL pick tasks are DONE.
// Creates outbound records from actual pick events (qty per product/location).
func (s *Service) Ship(ctx context.Context, id primitive.ObjectID, userID string) (*Order, error) {
	order, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	if order.Status != StatusPicking {
		return nil, ErrInvalidTransition
	}

	// Check all pick tasks are DONE
	allDone, err := s.pickingSvc.AllDoneForOrder(ctx, order.ID)
	if err != nil {
		return nil, fmt.Errorf("check pick completion: %w", err)
	}
	if !allDone {
		return nil, ErrPickingNotComplete
	}

	// Get pick events to determine actual picked quantities per product/location
	events, err := s.pickingSvc.EventsByOrder(ctx, order.ID)
	if err != nil {
		return nil, fmt.Errorf("get pick events: %w", err)
	}

	// Aggregate pick events: (productID, locationID, lotID) → total qty
	type locKey struct {
		ProductID  primitive.ObjectID
		LocationID primitive.ObjectID
		LotID      primitive.ObjectID
	}
	aggregated := make(map[locKey]int)
	productTotals := make(map[primitive.ObjectID]int)
	for _, ev := range events {
		key := locKey{ProductID: ev.ProductID, LocationID: ev.LocationID, LotID: ev.LotID}
		aggregated[key] += ev.Qty
		productTotals[ev.ProductID] += ev.Qty
	}

	// Create outbound records from actual picks — lot-aware
	for key, qty := range aggregated {
		outReq := &outboundPkg.CreateOutboundRequest{
			ProductID:  key.ProductID.Hex(),
			LocationID: key.LocationID.Hex(),
			LotID:      key.LotID.Hex(),
			Quantity:   qty,
			Reference:  fmt.Sprintf("ORDER:%s", order.OrderNo),
		}
		_, err := s.outboundSvc.Create(ctx, order.WarehouseID, outReq, userID)
		if err != nil {
			return nil, fmt.Errorf("create outbound: %w", err)
		}
	}

	// Update shipped qty on order items
	for i, item := range order.Items {
		if total, ok := productTotals[item.ProductID]; ok {
			order.Items[i].ShippedQty = total
		}
	}

	// Release all reservations
	_, _ = s.reservationSvc.ReleaseByOrder(ctx, order.ID, userID, "order shipped")

	// Update status
	now := time.Now().UTC()
	setFields := bson.M{
		"shipped_at": now,
		"items":      order.Items,
	}
	if err := s.repo.UpdateStatus(ctx, id, StatusPicking, StatusShipped, setFields); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrInvalidTransition
		}
		return nil, err
	}

	_ = s.historySvc.RecordAction(ctx,
		historyPkg.ActionOrderShipped, historyPkg.EntityOrder,
		order.ID.Hex(), userID,
		fmt.Sprintf("Order %s shipped with %d items", order.OrderNo, len(order.Items)),
	)

	order.Status = StatusShipped
	order.ShippedAt = &now

	if s.notifySvc != nil {
		go s.notifySvc.NotifyOrderShipped(order.OrderNo, order.ClientName, len(order.Items))
		// Check low stock for each product in the order
		for _, item := range order.Items {
			pid := item.ProductID
			go s.notifySvc.CheckLowStock(pid)
		}
	}

	return order, nil
}
