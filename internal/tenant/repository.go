package tenant

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository interface {
	Create(ctx context.Context, t *Tenant) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*Tenant, error)
	FindByCode(ctx context.Context, code string) (*Tenant, error)
	List(ctx context.Context, page, limit int) ([]*Tenant, error)
	Count(ctx context.Context) (int64, error)
	Update(ctx context.Context, id primitive.ObjectID, update bson.M) error
	Delete(ctx context.Context, id primitive.ObjectID) error
}

type mongoRepository struct {
	col *mongo.Collection
}

func NewRepository(col *mongo.Collection) Repository {
	return &mongoRepository{col: col}
}

func (r *mongoRepository) Create(ctx context.Context, t *Tenant) error {
	t.ID = primitive.NewObjectID()
	t.CreatedAt = time.Now().UTC()
	_, err := r.col.InsertOne(ctx, t)
	return err
}

func (r *mongoRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*Tenant, error) {
	var t Tenant
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *mongoRepository) FindByCode(ctx context.Context, code string) (*Tenant, error) {
	var t Tenant
	err := r.col.FindOne(ctx, bson.M{"code": code}).Decode(&t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *mongoRepository) List(ctx context.Context, page, limit int) ([]*Tenant, error) {
	skip := int64((page - 1) * limit)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "code", Value: 1}})

	cursor, err := r.col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tenants []*Tenant
	if err := cursor.All(ctx, &tenants); err != nil {
		return nil, err
	}
	return tenants, nil
}

func (r *mongoRepository) Count(ctx context.Context) (int64, error) {
	return r.col.CountDocuments(ctx, bson.M{})
}

func (r *mongoRepository) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func (r *mongoRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
