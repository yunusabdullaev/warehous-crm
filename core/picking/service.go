package picking

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	historyPkg "warehouse-crm/core/history"
	lotPkg "warehouse-crm/core/lot"
	notifyPkg "warehouse-crm/core/notify"
	stockPkg "warehouse-crm/core/stock"
)

var (
	ErrInsufficientStock  = errors.New("INSUFFICIENT_AVAILABLE_STOCK")
	ErrTaskNotFound       = errors.New("PICK_TASK_NOT_FOUND")
	ErrTaskNotPickable    = errors.New("TASK_NOT_PICKABLE")
	ErrLocationMismatch   = errors.New("LOCATION_MISMATCH")
	ErrProductMismatch    = errors.New("PRODUCT_MISMATCH")
	ErrLotMismatch        = errors.New("LOT_MISMATCH")
	ErrQtyExceedsPlanned  = errors.New("QTY_EXCEEDS_PLANNED")
	ErrPickingNotComplete = errors.New("PICKING_NOT_COMPLETE")
)

// Shortage describes a per-product stock shortage during pick planning.
type Shortage struct {
	ProductID string `json:"product_id"`
	Requested int    `json:"requested"`
	Available int    `json:"available"`
}

type Service struct {
	repo       Repository
	stockSvc   *stockPkg.Service
	historySvc *historyPkg.Service
	notifySvc  *notifyPkg.Service
	lotSvc     *lotPkg.Service
}

func NewService(repo Repository, stockSvc *stockPkg.Service, historySvc *historyPkg.Service, lotSvc *lotPkg.Service) *Service {
	return &Service{repo: repo, stockSvc: stockSvc, historySvc: historySvc, lotSvc: lotSvc}
}

// SetNotifySvc sets the notification service (avoids circular init).
func (s *Service) SetNotifySvc(n *notifyPkg.Service) {
	s.notifySvc = n
}

// stockWithLot holds stock row enriched with lot expiry for FEFO sorting.
type stockWithLot struct {
	stock  *stockPkg.Stock
	expiry *time.Time // nil = no expiry (lowest FEFO priority)
}

// GeneratePlan creates pick tasks for an order based on current stock distribution.
// Strategy: FEFO — pick from lots with earliest expDate first.
// Within same lot, pick from locations with highest available qty first.
// Returns error with shortage details if any product cannot be fully covered.
func (s *Service) GeneratePlan(ctx context.Context, orderID primitive.ObjectID, items []PlanItem, userID string) ([]*PickTask, error) {
	// Check all items for shortages first
	var shortages []Shortage
	for _, item := range items {
		stocks, err := s.stockSvc.ListByProduct(ctx, item.ProductID)
		if err != nil {
			return nil, fmt.Errorf("lookup stock for product %s: %w", item.ProductID.Hex(), err)
		}
		totalAvailable := 0
		for _, st := range stocks {
			if st.Quantity > 0 {
				totalAvailable += st.Quantity
			}
		}
		if totalAvailable < item.RequestedQty {
			shortages = append(shortages, Shortage{
				ProductID: item.ProductID.Hex(),
				Requested: item.RequestedQty,
				Available: totalAvailable,
			})
		}
	}

	if len(shortages) > 0 {
		return nil, fmt.Errorf("%w: shortages detected", ErrInsufficientStock)
	}

	// Generate tasks with FEFO
	var tasks []*PickTask
	for _, item := range items {
		stocks, _ := s.stockSvc.ListByProduct(ctx, item.ProductID)

		// Enrich with lot expiry for FEFO
		var enriched []stockWithLot
		lotCache := make(map[primitive.ObjectID]*time.Time) // cache lot expiry lookups
		for _, st := range stocks {
			if st.Quantity <= 0 {
				continue
			}
			exp, ok := lotCache[st.LotID]
			if !ok {
				l, err := s.lotSvc.FindByID(ctx, st.LotID)
				if err == nil && l.ExpDate != nil {
					exp = l.ExpDate
				}
				lotCache[st.LotID] = exp
			}
			enriched = append(enriched, stockWithLot{stock: st, expiry: exp})
		}

		// FEFO sort: earliest expiry first (nil = last), then highest qty
		sort.Slice(enriched, func(i, j int) bool {
			ei, ej := enriched[i].expiry, enriched[j].expiry
			if ei == nil && ej == nil {
				return enriched[i].stock.Quantity > enriched[j].stock.Quantity
			}
			if ei == nil {
				return false // nil goes last
			}
			if ej == nil {
				return true // nil goes last
			}
			if !ei.Equal(*ej) {
				return ei.Before(*ej)
			}
			return enriched[i].stock.Quantity > enriched[j].stock.Quantity
		})

		remaining := item.RequestedQty
		for _, sw := range enriched {
			if remaining <= 0 {
				break
			}
			take := remaining
			if take > sw.stock.Quantity {
				take = sw.stock.Quantity
			}
			tasks = append(tasks, &PickTask{
				OrderID:    orderID,
				ProductID:  item.ProductID,
				LocationID: sw.stock.LocationID,
				LotID:      sw.stock.LotID,
				PlannedQty: take,
				CreatedBy:  userID,
			})
			remaining -= take
		}
	}

	if len(tasks) == 0 {
		return nil, errors.New("no pick tasks generated")
	}

	if err := s.repo.CreateTaskBatch(ctx, tasks); err != nil {
		return nil, fmt.Errorf("create pick tasks: %w", err)
	}

	_ = s.historySvc.RecordAction(ctx,
		historyPkg.ActionPickTaskCreated, historyPkg.EntityPickTask,
		orderID.Hex(), userID,
		fmt.Sprintf("Generated %d pick tasks for order %s (FEFO)", len(tasks), orderID.Hex()),
	)

	return tasks, nil
}

// PlanItem is input to GeneratePlan.
type PlanItem struct {
	ProductID    primitive.ObjectID
	RequestedQty int
}

// Scan validates and records a pick scan — now validates lot match too.
func (s *Service) Scan(ctx context.Context, taskID primitive.ObjectID, locationID, productID, lotID primitive.ObjectID, qty int, userID string, meta PickEventMeta) (*PickTask, error) {
	if qty <= 0 {
		return nil, errors.New("qty must be positive")
	}

	task, err := s.repo.FindTaskByID(ctx, taskID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}

	// Validate location match
	if task.LocationID != locationID {
		return nil, ErrLocationMismatch
	}

	// Validate product match
	if task.ProductID != productID {
		return nil, ErrProductMismatch
	}

	// Validate lot match
	if task.LotID != lotID {
		return nil, ErrLotMismatch
	}

	// Validate status
	if task.Status != TaskStatusOpen && task.Status != TaskStatusInProgress {
		return nil, ErrTaskNotPickable
	}

	// Validate qty bound
	if task.PickedQty+qty > task.PlannedQty {
		return nil, fmt.Errorf("%w: picked=%d + scan=%d > planned=%d", ErrQtyExceedsPlanned, task.PickedQty, qty, task.PlannedQty)
	}

	// Atomic update
	updated, err := s.repo.AtomicAddPicked(ctx, taskID, qty)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("%w: concurrent update or qty exceeds planned", ErrQtyExceedsPlanned)
		}
		return nil, err
	}

	// Record event — includes lotID
	event := &PickEvent{
		OrderID:    task.OrderID,
		PickTaskID: taskID,
		UserID:     userID,
		LocationID: locationID,
		ProductID:  productID,
		LotID:      lotID,
		Qty:        qty,
		Meta:       meta,
	}
	_ = s.repo.InsertEvent(ctx, event)

	_ = s.historySvc.RecordAction(ctx,
		historyPkg.ActionPickEventRecorded, historyPkg.EntityPickTask,
		taskID.Hex(), userID,
		fmt.Sprintf("Picked %d units (now %d/%d) at location %s lot %s", qty, updated.PickedQty, updated.PlannedQty, locationID.Hex(), lotID.Hex()),
	)

	if updated.Status == TaskStatusDone && s.notifySvc != nil {
		go s.notifySvc.NotifyPickTaskDone(updated.OrderID.Hex(), updated.ID.Hex(), updated.PickedQty, updated.PlannedQty)
	}

	return updated, nil
}

// Assign sets the assignedTo user on a task.
func (s *Service) Assign(ctx context.Context, taskID primitive.ObjectID, assignTo string) error {
	err := s.repo.SetAssignee(ctx, taskID, assignTo)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return ErrTaskNotFound
		}
		return err
	}
	return nil
}

// CancelByOrder cancels all pending pick tasks for an order.
func (s *Service) CancelByOrder(ctx context.Context, orderID primitive.ObjectID, userID string) (int64, error) {
	count, err := s.repo.CancelByOrder(ctx, orderID)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		_ = s.historySvc.RecordAction(ctx,
			historyPkg.ActionPickTaskCancelled, historyPkg.EntityPickTask,
			orderID.Hex(), userID,
			fmt.Sprintf("Cancelled %d pick tasks for order %s", count, orderID.Hex()),
		)
	}
	return count, nil
}

// TasksByOrder returns all pick tasks for an order.
func (s *Service) TasksByOrder(ctx context.Context, orderID primitive.ObjectID) ([]*PickTask, error) {
	return s.repo.FindTasksByOrder(ctx, orderID)
}

// TasksByAssignee returns tasks assigned to a user.
func (s *Service) TasksByAssignee(ctx context.Context, userID string) ([]*PickTask, error) {
	return s.repo.FindTasksByAssignee(ctx, userID, []string{TaskStatusOpen, TaskStatusInProgress})
}

// AllDoneForOrder checks if all pick tasks are completed.
func (s *Service) AllDoneForOrder(ctx context.Context, orderID primitive.ObjectID) (bool, error) {
	return s.repo.AllDoneForOrder(ctx, orderID)
}

// EventsByOrder returns all pick events for an order.
func (s *Service) EventsByOrder(ctx context.Context, orderID primitive.ObjectID) ([]*PickEvent, error) {
	return s.repo.EventsByOrder(ctx, orderID)
}

// GetTask returns a single task by ID.
func (s *Service) GetTask(ctx context.Context, taskID primitive.ObjectID) (*PickTask, error) {
	t, err := s.repo.FindTaskByID(ctx, taskID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	return t, nil
}
