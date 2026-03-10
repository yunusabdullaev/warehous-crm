package warehouse

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository interface {
	Create(ctx context.Context, w *Warehouse) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*Warehouse, error)
	FindByCode(ctx context.Context, code string) (*Warehouse, error)
	FindDefault(ctx context.Context) (*Warehouse, error)
	List(ctx context.Context) ([]*Warehouse, error)
	ListByTenant(ctx context.Context, tenantID primitive.ObjectID) ([]*Warehouse, error)
	Update(ctx context.Context, id primitive.ObjectID, update bson.M) error
	Delete(ctx context.Context, id primitive.ObjectID) error
	HasData(ctx context.Context, db *mongo.Database, warehouseID primitive.ObjectID) (bool, error)
}

type mongoRepository struct {
	col *mongo.Collection
}

func NewRepository(col *mongo.Collection) Repository {
	return &mongoRepository{col: col}
}

func (r *mongoRepository) Create(ctx context.Context, w *Warehouse) error {
	w.ID = primitive.NewObjectID()
	w.CreatedAt = time.Now().UTC()
	_, err := r.col.InsertOne(ctx, w)
	return err
}

func (r *mongoRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*Warehouse, error) {
	var w Warehouse
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&w)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *mongoRepository) FindByCode(ctx context.Context, code string) (*Warehouse, error) {
	var w Warehouse
	err := r.col.FindOne(ctx, bson.M{"code": code}).Decode(&w)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *mongoRepository) FindDefault(ctx context.Context) (*Warehouse, error) {
	var w Warehouse
	err := r.col.FindOne(ctx, bson.M{"is_default": true}).Decode(&w)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *mongoRepository) List(ctx context.Context) ([]*Warehouse, error) {
	opts := options.Find().SetSort(bson.D{{Key: "is_default", Value: -1}, {Key: "code", Value: 1}})
	cursor, err := r.col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var warehouses []*Warehouse
	if err := cursor.All(ctx, &warehouses); err != nil {
		return nil, err
	}
	return warehouses, nil
}

func (r *mongoRepository) ListByTenant(ctx context.Context, tenantID primitive.ObjectID) ([]*Warehouse, error) {
	opts := options.Find().SetSort(bson.D{{Key: "is_default", Value: -1}, {Key: "code", Value: 1}})
	cursor, err := r.col.Find(ctx, bson.M{"tenant_id": tenantID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var warehouses []*Warehouse
	if err := cursor.All(ctx, &warehouses); err != nil {
		return nil, err
	}
	return warehouses, nil
}

func (r *mongoRepository) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func (r *mongoRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

// HasData checks if any warehouse-scoped collection references this warehouse.
func (r *mongoRepository) HasData(ctx context.Context, db *mongo.Database, warehouseID primitive.ObjectID) (bool, error) {
	collections := []string{
		"locations", "stocks", "inbounds", "outbounds",
		"orders", "reservations", "pick_tasks",
		"adjustments", "returns", "history",
	}
	for _, col := range collections {
		count, err := db.Collection(col).CountDocuments(ctx,
			bson.M{"warehouse_id": warehouseID},
			options.Count().SetLimit(1))
		if err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}
