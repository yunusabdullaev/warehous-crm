package adjustment

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository interface {
	Create(ctx context.Context, adj *Adjustment) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*Adjustment, error)
	List(ctx context.Context, filter bson.M, page, limit int) ([]*Adjustment, int64, error)
}

type mongoRepository struct {
	collection *mongo.Collection
}

func NewRepository(col *mongo.Collection) Repository {
	return &mongoRepository{collection: col}
}

func (r *mongoRepository) Create(ctx context.Context, adj *Adjustment) error {
	adj.ID = primitive.NewObjectID()
	adj.CreatedAt = time.Now().UTC()
	_, err := r.collection.InsertOne(ctx, adj)
	return err
}

func (r *mongoRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*Adjustment, error) {
	var adj Adjustment
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&adj)
	if err != nil {
		return nil, err
	}
	return &adj, nil
}

func (r *mongoRepository) List(ctx context.Context, filter bson.M, page, limit int) ([]*Adjustment, int64, error) {
	skip := (page - 1) * limit
	opts := options.Find().SetSkip(int64(skip)).SetLimit(int64(limit)).SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var items []*Adjustment
	if err := cursor.All(ctx, &items); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	return items, total, err
}
