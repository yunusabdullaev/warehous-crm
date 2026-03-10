package inbound

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository interface {
	Create(ctx context.Context, inb *Inbound) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*Inbound, error)
	List(ctx context.Context, warehouseID primitive.ObjectID, page, limit int) ([]*Inbound, int64, error)
	ListByProduct(ctx context.Context, productID primitive.ObjectID, page, limit int) ([]*Inbound, int64, error)
	MarkReversed(ctx context.Context, id primitive.ObjectID, reversedBy, reason string) error
}

type mongoRepository struct {
	collection *mongo.Collection
}

func NewRepository(col *mongo.Collection) Repository {
	return &mongoRepository{collection: col}
}

func (r *mongoRepository) Create(ctx context.Context, inb *Inbound) error {
	inb.ID = primitive.NewObjectID()
	inb.CreatedAt = time.Now().UTC()
	inb.Status = StatusActive
	_, err := r.collection.InsertOne(ctx, inb)
	return err
}

func (r *mongoRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*Inbound, error) {
	var inb Inbound
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&inb)
	if err != nil {
		return nil, err
	}
	return &inb, nil
}

func (r *mongoRepository) List(ctx context.Context, warehouseID primitive.ObjectID, page, limit int) ([]*Inbound, int64, error) {
	skip := (page - 1) * limit
	opts := options.Find().SetSkip(int64(skip)).SetLimit(int64(limit)).SetSort(bson.D{{Key: "created_at", Value: -1}})

	filter := bson.M{}
	if !warehouseID.IsZero() {
		filter["warehouse_id"] = warehouseID
	}
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var items []*Inbound
	if err := cursor.All(ctx, &items); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, bson.M{})
	return items, total, err
}

func (r *mongoRepository) ListByProduct(ctx context.Context, productID primitive.ObjectID, page, limit int) ([]*Inbound, int64, error) {
	filter := bson.M{"product_id": productID}
	skip := (page - 1) * limit
	opts := options.Find().SetSkip(int64(skip)).SetLimit(int64(limit)).SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var items []*Inbound
	if err := cursor.All(ctx, &items); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	return items, total, err
}

// MarkReversed atomically sets status=REVERSED only if currently ACTIVE (prevents double-reversal).
func (r *mongoRepository) MarkReversed(ctx context.Context, id primitive.ObjectID, reversedBy, reason string) error {
	now := time.Now().UTC()
	filter := bson.M{
		"_id":    id,
		"status": StatusActive,
	}
	update := bson.M{
		"$set": bson.M{
			"status":         StatusReversed,
			"reversed_at":    now,
			"reversed_by":    reversedBy,
			"reverse_reason": reason,
		},
	}
	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments // will be interpreted as "already reversed"
	}
	return nil
}
