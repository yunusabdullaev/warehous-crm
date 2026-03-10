package product

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository interface {
	Create(ctx context.Context, product *Product) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*Product, error)
	FindBySKU(ctx context.Context, sku string) (*Product, error)
	List(ctx context.Context, page, limit int) ([]*Product, int64, error)
	Update(ctx context.Context, id primitive.ObjectID, update bson.M) error
	Delete(ctx context.Context, id primitive.ObjectID) error
}

type mongoRepository struct {
	collection *mongo.Collection
}

func NewRepository(col *mongo.Collection) Repository {
	return &mongoRepository{collection: col}
}

func (r *mongoRepository) Create(ctx context.Context, product *Product) error {
	product.ID = primitive.NewObjectID()
	product.CreatedAt = time.Now().UTC()
	product.UpdatedAt = time.Now().UTC()
	_, err := r.collection.InsertOne(ctx, product)
	return err
}

func (r *mongoRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*Product, error) {
	var p Product
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *mongoRepository) FindBySKU(ctx context.Context, sku string) (*Product, error) {
	var p Product
	err := r.collection.FindOne(ctx, bson.M{"sku": sku}).Decode(&p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *mongoRepository) List(ctx context.Context, page, limit int) ([]*Product, int64, error) {
	skip := (page - 1) * limit
	opts := options.Find().SetSkip(int64(skip)).SetLimit(int64(limit)).SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var products []*Product
	if err := cursor.All(ctx, &products); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
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
