package reservation

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	historyPkg "warehouse-crm/core/history"
	stockPkg "warehouse-crm/core/stock"
)

var (
	ErrInsufficientStock = errors.New("INSUFFICIENT_AVAILABLE_STOCK")
	ErrAlreadyReleased   = errors.New("ALREADY_RELEASED")
)

type Service struct {
	repo       Repository
	stockSvc   *stockPkg.Service
	historySvc *historyPkg.Service
}

func NewService(repo Repository, stockSvc *stockPkg.Service, historySvc *historyPkg.Service) *Service {
	return &Service{repo: repo, stockSvc: stockSvc, historySvc: historySvc}
}

// Reserve creates a reservation after checking that availableQty >= qty.
// availableQty = physicalQty (sum across all locations) - reservedQty (active reservations).
func (s *Service) Reserve(ctx context.Context, orderID, productID primitive.ObjectID, qty int, userID string) (*Reservation, error) {
	if qty <= 0 {
		return nil, errors.New("qty must be positive")
	}

	// Get physical stock total for product
	stocks, err := s.stockSvc.ListByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	physicalQty := 0
	for _, st := range stocks {
		physicalQty += st.Quantity
	}

	// Get currently reserved qty
	reservedQty, err := s.repo.SumActiveByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}

	availableQty := physicalQty - reservedQty
	if availableQty < qty {
		return nil, fmt.Errorf("%w: available=%d, requested=%d", ErrInsufficientStock, availableQty, qty)
	}

	r := &Reservation{
		OrderID:   orderID,
		ProductID: productID,
		Qty:       qty,
		CreatedBy: userID,
	}
	if err := s.repo.Create(ctx, r); err != nil {
		return nil, err
	}

	_ = s.historySvc.RecordAction(ctx,
		historyPkg.ActionReservationCreated, historyPkg.EntityReservation,
		r.ID.Hex(), userID,
		fmt.Sprintf("Reserved %d units of product %s for order %s", qty, productID.Hex(), orderID.Hex()),
	)

	return r, nil
}

// ReleaseByOrder releases all active reservations for a given order.
func (s *Service) ReleaseByOrder(ctx context.Context, orderID primitive.ObjectID, userID, reason string) (int64, error) {
	count, err := s.repo.ReleaseByOrder(ctx, orderID, userID, reason)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		_ = s.historySvc.RecordAction(ctx,
			historyPkg.ActionReservationReleased, historyPkg.EntityReservation,
			orderID.Hex(), userID,
			fmt.Sprintf("Released %d reservations for order %s (reason: %s)", count, orderID.Hex(), reason),
		)
	}
	return count, nil
}

// ReleaseOne releases a single reservation (admin/operator use).
func (s *Service) ReleaseOne(ctx context.Context, reservationID primitive.ObjectID, userID, reason string) error {
	err := s.repo.Release(ctx, reservationID, userID, reason)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return ErrAlreadyReleased
		}
		return err
	}
	_ = s.historySvc.RecordAction(ctx,
		historyPkg.ActionReservationReleased, historyPkg.EntityReservation,
		reservationID.Hex(), userID,
		fmt.Sprintf("Released reservation %s (reason: %s)", reservationID.Hex(), reason),
	)
	return nil
}

func (s *Service) FindByOrder(ctx context.Context, orderID primitive.ObjectID) ([]*Reservation, error) {
	return s.repo.FindByOrder(ctx, orderID)
}

func (s *Service) SumActiveByProduct(ctx context.Context, productID primitive.ObjectID) (int, error) {
	return s.repo.SumActiveByProduct(ctx, productID)
}

func (s *Service) List(ctx context.Context, filter map[string]interface{}, page, limit int) ([]*Reservation, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.repo.List(ctx, filter, page, limit)
}
