package warehouse

import (
	"context"
	"errors"
	"log/slog"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Service struct {
	repo Repository
	db   *mongo.Database
}

func NewService(repo Repository, db *mongo.Database) *Service {
	return &Service{repo: repo, db: db}
}

// EnsureDefault creates the DEFAULT warehouse if none exists.
// Should be called once at startup.
func (s *Service) EnsureDefault(ctx context.Context) (*Warehouse, error) {
	existing, err := s.repo.FindDefault(ctx)
	if err == nil && existing != nil {
		slog.Info("warehouse: default warehouse exists", "id", existing.ID.Hex(), "code", existing.Code)
		return existing, nil
	}

	w := &Warehouse{
		Code:      "WH-DEFAULT",
		Name:      "Default Warehouse",
		IsDefault: true,
	}
	if err := s.repo.Create(ctx, w); err != nil {
		return nil, err
	}
	slog.Info("warehouse: created default warehouse", "id", w.ID.Hex())
	return w, nil
}

func (s *Service) Create(ctx context.Context, w *Warehouse) error {
	existing, _ := s.repo.FindByCode(ctx, w.Code)
	if existing != nil {
		return errors.New("warehouse code already exists")
	}
	return s.repo.Create(ctx, w)
}

func (s *Service) GetByID(ctx context.Context, id primitive.ObjectID) (*Warehouse, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]*Warehouse, error) {
	return s.repo.List(ctx)
}

// ListByTenant returns warehouses belonging to a specific tenant.
func (s *Service) ListByTenant(ctx context.Context, tenantID primitive.ObjectID) ([]*Warehouse, error) {
	return s.repo.ListByTenant(ctx, tenantID)
}

// ListForUser returns warehouses the user has access to.
// Superadmin sees all; admin sees all in their tenant; others see only their allowed list.
func (s *Service) ListForUser(ctx context.Context, role string, allowedIDs []primitive.ObjectID, tenantID primitive.ObjectID) ([]*Warehouse, error) {
	var all []*Warehouse
	var err error

	if role == "superadmin" {
		all, err = s.repo.List(ctx)
	} else if !tenantID.IsZero() {
		all, err = s.repo.ListByTenant(ctx, tenantID)
	} else {
		all, err = s.repo.List(ctx)
	}
	if err != nil {
		return nil, err
	}

	if role == "admin" || role == "superadmin" {
		return all, nil
	}

	// Filter to allowed
	allowed := make(map[primitive.ObjectID]bool, len(allowedIDs))
	for _, id := range allowedIDs {
		allowed[id] = true
	}
	var result []*Warehouse
	for _, w := range all {
		if allowed[w.ID] {
			result = append(result, w)
		}
	}
	return result, nil
}

func (s *Service) Update(ctx context.Context, id primitive.ObjectID, code, name, address string) error {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return errors.New("warehouse not found")
	}
	update := bson.M{}
	if code != "" && code != existing.Code {
		dup, _ := s.repo.FindByCode(ctx, code)
		if dup != nil {
			return errors.New("warehouse code already exists")
		}
		update["code"] = code
	}
	if name != "" {
		update["name"] = name
	}
	if address != "" {
		update["address"] = address
	}
	if len(update) == 0 {
		return nil
	}
	return s.repo.Update(ctx, id, update)
}

func (s *Service) Delete(ctx context.Context, id primitive.ObjectID) error {
	w, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return errors.New("warehouse not found")
	}
	if w.IsDefault {
		return errors.New("cannot delete the default warehouse")
	}
	hasData, err := s.repo.HasData(ctx, s.db, id)
	if err != nil {
		return err
	}
	if hasData {
		return errors.New("cannot delete warehouse that contains data")
	}
	return s.repo.Delete(ctx, id)
}

// GetDefault returns the default warehouse.
func (s *Service) GetDefault(ctx context.Context) (*Warehouse, error) {
	return s.repo.FindDefault(ctx)
}
