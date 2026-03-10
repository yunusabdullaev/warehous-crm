package auth

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository interface {
	Create(ctx context.Context, user *User) error
	FindByUsername(ctx context.Context, username string) (*User, error)
	FindByID(ctx context.Context, id primitive.ObjectID) (*User, error)
	List(ctx context.Context, page, limit int) ([]User, error)
	Count(ctx context.Context) (int64, error)
	ListByTenant(ctx context.Context, tenantID primitive.ObjectID, page, limit int) ([]User, error)
	CountByTenant(ctx context.Context, tenantID primitive.ObjectID) (int64, error)
	Update(ctx context.Context, id primitive.ObjectID, update bson.M) error
	Delete(ctx context.Context, id primitive.ObjectID) error
}

type mongoRepository struct {
	collection *mongo.Collection
}

func NewRepository(col *mongo.Collection) Repository {
	return &mongoRepository{collection: col}
}

func (r *mongoRepository) Create(ctx context.Context, user *User) error {
	user.ID = primitive.NewObjectID()
	user.CreatedAt = time.Now().UTC()
	user.UpdatedAt = time.Now().UTC()
	_, err := r.collection.InsertOne(ctx, user)
	return err
}

func (r *mongoRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	err := r.collection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *mongoRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*User, error) {
	var user User
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *mongoRepository) List(ctx context.Context, page, limit int) ([]User, error) {
	skip := int64((page - 1) * limit)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *mongoRepository) Count(ctx context.Context) (int64, error) {
	return r.collection.CountDocuments(ctx, bson.M{})
}

func (r *mongoRepository) ListByTenant(ctx context.Context, tenantID primitive.ObjectID, page, limit int) ([]User, error) {
	skip := int64((page - 1) * limit)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	filter := bson.M{"tenant_id": tenantID}
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *mongoRepository) CountByTenant(ctx context.Context, tenantID primitive.ObjectID) (int64, error) {
	return r.collection.CountDocuments(ctx, bson.M{"tenant_id": tenantID})
}

func (r *mongoRepository) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["updated_at"] = time.Now().UTC()
	_, err := r.collection.UpdateByID(ctx, id, bson.M{"$set": update})
	return err
}

func (r *mongoRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
