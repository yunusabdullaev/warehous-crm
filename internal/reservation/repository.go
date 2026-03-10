package reservation

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository interface {
	Create(ctx context.Context, r *Reservation) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*Reservation, error)
	Release(ctx context.Context, id primitive.ObjectID, releasedBy, reason string) error
	ReleaseByOrder(ctx context.Context, orderID primitive.ObjectID, releasedBy, reason string) (int64, error)
	FindByOrder(ctx context.Context, orderID primitive.ObjectID) ([]*Reservation, error)
	SumActiveByProduct(ctx context.Context, productID primitive.ObjectID) (int, error)
	List(ctx context.Context, filter bson.M, page, limit int) ([]*Reservation, int64, error)
}

type mongoRepository struct {
	col *mongo.Collection
}

func NewRepository(col *mongo.Collection) Repository {
	return &mongoRepository{col: col}
}

func (r *mongoRepository) Create(ctx context.Context, res *Reservation) error {
	res.CreatedAt = time.Now().UTC()
	res.Status = StatusActive
	result, err := r.col.InsertOne(ctx, res)
	if err != nil {
		return err
	}
	res.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *mongoRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*Reservation, error) {
	var res Reservation
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// Release atomically sets status=RELEASED only if currently ACTIVE.
func (r *mongoRepository) Release(ctx context.Context, id primitive.ObjectID, releasedBy, reason string) error {
	now := time.Now().UTC()
	filter := bson.M{"_id": id, "status": StatusActive}
	update := bson.M{"$set": bson.M{
		"status":      StatusReleased,
		"released_at": now,
		"released_by": releasedBy,
		"reason":      reason,
	}}
	res, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// ReleaseByOrder releases all ACTIVE reservations for a given order.
func (r *mongoRepository) ReleaseByOrder(ctx context.Context, orderID primitive.ObjectID, releasedBy, reason string) (int64, error) {
	now := time.Now().UTC()
	filter := bson.M{"order_id": orderID, "status": StatusActive}
	update := bson.M{"$set": bson.M{
		"status":      StatusReleased,
		"released_at": now,
		"released_by": releasedBy,
		"reason":      reason,
	}}
	res, err := r.col.UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

func (r *mongoRepository) FindByOrder(ctx context.Context, orderID primitive.ObjectID) ([]*Reservation, error) {
	cursor, err := r.col.Find(ctx, bson.M{"order_id": orderID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var results []*Reservation
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// SumActiveByProduct returns total reserved qty for a product across all ACTIVE reservations.
func (r *mongoRepository) SumActiveByProduct(ctx context.Context, productID primitive.ObjectID) (int, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"product_id": productID, "status": StatusActive}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: "$qty"}}},
		}}},
	}
	cursor, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)
	var result []struct {
		Total int `bson:"total"`
	}
	if err := cursor.All(ctx, &result); err != nil {
		return 0, err
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0].Total, nil
}

func (r *mongoRepository) List(ctx context.Context, filter bson.M, page, limit int) ([]*Reservation, int64, error) {
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

	var results []*Reservation
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, err
	}

	total, err := r.col.CountDocuments(ctx, filter)
	return results, total, err
}
