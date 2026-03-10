package stock

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository interface {
	Upsert(ctx context.Context, warehouseID, productID, locationID, lotID primitive.ObjectID, qtyDelta int) error
	FindByProductLocationLot(ctx context.Context, productID, locationID, lotID primitive.ObjectID) (*Stock, error)
	ListByProduct(ctx context.Context, productID primitive.ObjectID) ([]*Stock, error)
	ListByLocation(ctx context.Context, locationID primitive.ObjectID) ([]*Stock, error)
	ListAll(ctx context.Context, warehouseID primitive.ObjectID, page, limit int) ([]*Stock, int64, error)
	ListByLot(ctx context.Context, lotID primitive.ObjectID) ([]*Stock, error)
}

type mongoRepository struct {
	collection *mongo.Collection
}

func NewRepository(col *mongo.Collection) Repository {
	ctx := context.Background()
	// New compound unique index for the 3-key
	_, _ = col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "product_id", Value: 1}, {Key: "location_id", Value: 1}, {Key: "lot_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return &mongoRepository{collection: col}
}

func (r *mongoRepository) Upsert(ctx context.Context, warehouseID, productID, locationID, lotID primitive.ObjectID, qtyDelta int) error {
	filter := bson.M{
		"product_id":  productID,
		"location_id": locationID,
		"lot_id":      lotID,
	}

	update := bson.M{
		"$inc": bson.M{"quantity": qtyDelta},
		"$set": bson.M{"last_updated": time.Now().UTC()},
		"$setOnInsert": bson.M{
			"warehouse_id": warehouseID,
			"product_id":   productID,
			"location_id":  locationID,
			"lot_id":       lotID,
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := r.collection.UpdateOne(ctx, filter, update, opts)
	return err
}

func (r *mongoRepository) FindByProductLocationLot(ctx context.Context, productID, locationID, lotID primitive.ObjectID) (*Stock, error) {
	var s Stock
	err := r.collection.FindOne(ctx, bson.M{
		"product_id":  productID,
		"location_id": locationID,
		"lot_id":      lotID,
	}).Decode(&s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *mongoRepository) ListByProduct(ctx context.Context, productID primitive.ObjectID) ([]*Stock, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"product_id": productID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var stocks []*Stock
	if err := cursor.All(ctx, &stocks); err != nil {
		return nil, err
	}
	return stocks, nil
}

func (r *mongoRepository) ListByLocation(ctx context.Context, locationID primitive.ObjectID) ([]*Stock, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"location_id": locationID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var stocks []*Stock
	if err := cursor.All(ctx, &stocks); err != nil {
		return nil, err
	}
	return stocks, nil
}

func (r *mongoRepository) ListAll(ctx context.Context, warehouseID primitive.ObjectID, page, limit int) ([]*Stock, int64, error) {
	skip := (page - 1) * limit
	opts := options.Find().SetSkip(int64(skip)).SetLimit(int64(limit)).SetSort(bson.D{{Key: "last_updated", Value: -1}})

	filter := bson.M{}
	if !warehouseID.IsZero() {
		filter["warehouse_id"] = warehouseID
	}
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var stocks []*Stock
	if err := cursor.All(ctx, &stocks); err != nil {
		return nil, 0, err
	}

	total, err := r.collection.CountDocuments(ctx, bson.M{})
	return stocks, total, err
}

func (r *mongoRepository) ListByLot(ctx context.Context, lotID primitive.ObjectID) ([]*Stock, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"lot_id": lotID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var stocks []*Stock
	if err := cursor.All(ctx, &stocks); err != nil {
		return nil, err
	}
	return stocks, nil
}
