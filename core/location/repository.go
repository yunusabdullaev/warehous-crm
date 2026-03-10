package location

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository interface {
	Create(ctx context.Context, loc *Location) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*Location, error)
	FindByCode(ctx context.Context, code string) (*Location, error)
	List(ctx context.Context, warehouseID primitive.ObjectID, page, limit int) ([]*Location, int64, error)
	Update(ctx context.Context, id primitive.ObjectID, update bson.M) error
	Delete(ctx context.Context, id primitive.ObjectID) error
}

type mongoRepository struct {
	collection *mongo.Collection
}

func NewRepository(col *mongo.Collection) Repository {
	return &mongoRepository{collection: col}
}

func (r *mongoRepository) Create(ctx context.Context, loc *Location) error {
	loc.ID = primitive.NewObjectID()
	loc.CreatedAt = time.Now().UTC()
	loc.UpdatedAt = time.Now().UTC()
	_, err := r.collection.InsertOne(ctx, loc)
	return err
}

func (r *mongoRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*Location, error) {
	var loc Location
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&loc)
	if err != nil {
		return nil, err
	}
	return &loc, nil
}

func (r *mongoRepository) FindByCode(ctx context.Context, code string) (*Location, error) {
	var loc Location
	err := r.collection.FindOne(ctx, bson.M{"code": code}).Decode(&loc)
	if err != nil {
		return nil, err
	}
	return &loc, nil
}

func (r *mongoRepository) List(ctx context.Context, warehouseID primitive.ObjectID, page, limit int) ([]*Location, int64, error) {
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

	var locations []*Location
	if err := cursor.All(ctx, &locations); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	return locations, total, err
}

func (r *mongoRepository) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["updated_at"] = time.Now().UTC()
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func (r *mongoRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
