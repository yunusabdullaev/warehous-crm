package history

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository interface {
	Create(ctx context.Context, h *History) error
	List(ctx context.Context, filter bson.M, page, limit int) ([]*History, int64, error)
}

type mongoRepository struct {
	collection *mongo.Collection
}

func NewRepository(col *mongo.Collection) Repository {
	return &mongoRepository{collection: col}
}

func (r *mongoRepository) Create(ctx context.Context, h *History) error {
	h.ID = primitive.NewObjectID()
	h.Timestamp = time.Now().UTC()
	_, err := r.collection.InsertOne(ctx, h)
	return err
}

func (r *mongoRepository) List(ctx context.Context, filter bson.M, page, limit int) ([]*History, int64, error) {
	skip := (page - 1) * limit
	opts := options.Find().SetSkip(int64(skip)).SetLimit(int64(limit)).SetSort(bson.D{{Key: "timestamp", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var histories []*History
	if err := cursor.All(ctx, &histories); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	return histories, total, err
}
