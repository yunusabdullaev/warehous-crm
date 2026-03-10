package tenant

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Service struct {
	repo Repository
	db   *mongo.Database
}

func NewService(repo Repository, db ...*mongo.Database) *Service {
	s := &Service{repo: repo}
	if len(db) > 0 {
		s.db = db[0]
	}
	return s
}

// EnsureDefault creates TEN-DEFAULT if none exists. Call at startup.
func (s *Service) EnsureDefault(ctx context.Context) (*Tenant, error) {
	existing, err := s.repo.FindByCode(ctx, "TEN-DEFAULT")
	if err == nil && existing != nil {
		slog.Info("tenant: default tenant exists", "id", existing.ID.Hex(), "code", existing.Code)
		return existing, nil
	}
	limits, features := PlanDefaults(PlanFree)
	t := &Tenant{
		Code:     "TEN-DEFAULT",
		Name:     "Default Tenant",
		Plan:     PlanFree,
		Status:   StatusActive,
		Limits:   limits,
		Features: features,
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	slog.Info("tenant: created default tenant", "id", t.ID.Hex())
	return t, nil
}

func (s *Service) Create(ctx context.Context, t *Tenant) error {
	existing, _ := s.repo.FindByCode(ctx, t.Code)
	if existing != nil {
		return errors.New("tenant code already exists")
	}

	// Apply plan defaults if limits/features are empty
	if t.Plan == "" {
		t.Plan = PlanFree
	}
	if t.Status == "" {
		t.Status = StatusActive
	}
	if t.Limits == (TenantLimits{}) {
		limits, _ := PlanDefaults(t.Plan)
		t.Limits = limits
	}
	if t.Features == (TenantFeatures{}) {
		_, features := PlanDefaults(t.Plan)
		t.Features = features
	}

	return s.repo.Create(ctx, t)
}

func (s *Service) GetByID(ctx context.Context, id primitive.ObjectID) (*Tenant, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *Service) List(ctx context.Context, page, limit int) ([]*Tenant, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	list, err := s.repo.List(ctx, page, limit)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (s *Service) Update(ctx context.Context, id primitive.ObjectID, req UpdateRequest) error {
	_, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return errors.New("tenant not found")
	}

	update := bson.M{}

	if req.Code != "" {
		dup, _ := s.repo.FindByCode(ctx, req.Code)
		if dup != nil && dup.ID != id {
			return errors.New("tenant code already exists")
		}
		update["code"] = req.Code
	}
	if req.Name != "" {
		update["name"] = req.Name
	}
	if req.Plan != "" {
		update["plan"] = req.Plan
	}
	if req.Status != "" {
		if req.Status != StatusActive && req.Status != StatusSuspended {
			return errors.New("status must be ACTIVE or SUSPENDED")
		}
		update["status"] = req.Status
	}
	if req.Limits != nil {
		update["limits"] = *req.Limits
	}
	if req.Features != nil {
		update["features"] = *req.Features
	}

	if len(update) == 0 {
		return nil
	}
	update["updated_at"] = time.Now().UTC()
	return s.repo.Update(ctx, id, update)
}

// UpdatePlan changes the plan and resets limits/features to plan defaults.
func (s *Service) UpdatePlan(ctx context.Context, id primitive.ObjectID, plan string) error {
	_, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return errors.New("tenant not found")
	}
	if plan != PlanFree && plan != PlanPro && plan != PlanEnterprise {
		return errors.New("plan must be FREE, PRO, or ENTERPRISE")
	}
	limits, features := PlanDefaults(plan)
	update := bson.M{
		"plan":       plan,
		"limits":     limits,
		"features":   features,
		"updated_at": time.Now().UTC(),
	}
	return s.repo.Update(ctx, id, update)
}

func (s *Service) Delete(ctx context.Context, id primitive.ObjectID) error {
	t, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return errors.New("tenant not found")
	}
	if t.Code == "TEN-DEFAULT" {
		return errors.New("cannot delete the default tenant")
	}
	return s.repo.Delete(ctx, id)
}

// GetUsage returns current usage counts for a tenant.
func (s *Service) GetUsage(ctx context.Context, tenantID primitive.ObjectID) (*TenantUsage, error) {
	if s.db == nil {
		return nil, errors.New("database reference not available")
	}

	usage := &TenantUsage{}
	filter := bson.M{"tenant_id": tenantID}

	// Count warehouses
	wCount, err := s.db.Collection("warehouses").CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}
	usage.Warehouses = wCount

	// Count users
	uCount, err := s.db.Collection("users").CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}
	usage.Users = uCount

	// Count products (products are warehouse-scoped, collect warehouse IDs first)
	// Products have warehouse_id, so we count all products in tenant's warehouses
	var whIDs []primitive.ObjectID
	cursor, err := s.db.Collection("warehouses").Find(ctx, filter)
	if err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var wh struct {
				ID primitive.ObjectID `bson:"_id"`
			}
			if cursor.Decode(&wh) == nil {
				whIDs = append(whIDs, wh.ID)
			}
		}
	}
	if len(whIDs) > 0 {
		pCount, err := s.db.Collection("products").CountDocuments(ctx, bson.M{
			"warehouse_id": bson.M{"$in": whIDs},
		})
		if err == nil {
			usage.Products = pCount
		}
	}

	// Count today's orders
	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	oCount, err := s.db.Collection("orders").CountDocuments(ctx, bson.M{
		"warehouse_id": bson.M{"$in": whIDs},
		"created_at":   bson.M{"$gte": startOfDay},
	})
	if err == nil {
		usage.TodayOrders = oCount
	}

	return usage, nil
}

// UpdateRequest holds fields for updating a tenant.
type UpdateRequest struct {
	Code     string          `json:"code"`
	Name     string          `json:"name"`
	Plan     string          `json:"plan"`
	Status   string          `json:"status"`
	Limits   *TenantLimits   `json:"limits,omitempty"`
	Features *TenantFeatures `json:"features,omitempty"`
}
