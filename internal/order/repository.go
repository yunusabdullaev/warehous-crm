package order

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository interface {
	Create(ctx context.Context, o *Order) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*Order, error)
	List(ctx context.Context, filter bson.M, page, limit int) ([]*Order, int64, error)
	Update(ctx context.Context, o *Order) error
	UpdateStatus(ctx context.Context, id primitive.ObjectID, fromStatus, toStatus string, setFields bson.M) error
	NextOrderNo(ctx context.Context) (string, error)
}

type mongoRepository struct {
	col      *mongo.Collection
	counters *mongo.Collection
}

func NewRepository(col *mongo.Collection, counters *mongo.Collection) Repository {
	return &mongoRepository{col: col, counters: counters}
}

func (r *mongoRepository) Create(ctx context.Context, o *Order) error {
	o.CreatedAt = time.Now().UTC()
	o.Status = StatusDraft
	res, err := r.col.InsertOne(ctx, o)
	if err != nil {
		return err
	}
	o.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *mongoRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*Order, error) {
	var o Order
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&o)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *mongoRepository) List(ctx context.Context, filter bson.M, page, limit int) ([]*Order, int64, error) {
	skip := (page - 1) * limit
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var orders []*Order
	if err := cursor.All(ctx, &orders); err != nil {
		return nil, 0, err
	}

	total, err := r.col.CountDocuments(ctx, filter)
	return orders, total, err
}

func (r *mongoRepository) Update(ctx context.Context, o *Order) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": o.ID, "status": StatusDraft}, o)
	return err
}

// UpdateStatus atomically transitions status. Returns mongo.ErrNoDocuments if guard fails.
func (r *mongoRepository) UpdateStatus(ctx context.Context, id primitive.ObjectID, fromStatus, toStatus string, setFields bson.M) error {
	filter := bson.M{"_id": id, "status": fromStatus}
	if setFields == nil {
		setFields = bson.M{}
	}
	setFields["status"] = toStatus
	update := bson.M{"$set": setFields}

	res, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// NextOrderNo generates a sequential order number using a counters collection.
func (r *mongoRepository) NextOrderNo(ctx context.Context) (string, error) {
	year := time.Now().UTC().Year()
	counterID := fmt.Sprintf("order_%d", year)

	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)

	var result struct {
		Seq int `bson:"seq"`
	}
	err := r.counters.FindOneAndUpdate(
		ctx,
		bson.M{"_id": counterID},
		bson.M{"$inc": bson.M{"seq": 1}},
		opts,
	).Decode(&result)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("ORD-%d-%06d", year, result.Seq), nil
}
