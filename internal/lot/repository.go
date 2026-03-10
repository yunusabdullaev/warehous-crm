package lot

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Repository defines persistence operations for lots.
type Repository interface {
	Create(ctx context.Context, l *Lot) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*Lot, error)
	FindByProductAndLotNo(ctx context.Context, productID primitive.ObjectID, lotNo string) (*Lot, error)
	ListByProduct(ctx context.Context, productID primitive.ObjectID) ([]*Lot, error)
	ListAll(ctx context.Context, page, limit int) ([]*Lot, int64, error)
	FindOrCreate(ctx context.Context, productID primitive.ObjectID, lotNo string, expDate *time.Time, mfgDate *time.Time) (*Lot, error)
	FindExpiring(ctx context.Context, before time.Time) ([]*Lot, error)
}

type mongoRepository struct {
	col *mongo.Collection
}

// NewRepository creates a new lot repository and ensures indexes.
func NewRepository(col *mongo.Collection) Repository {
	ctx := context.Background()
	_, _ = col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "product_id", Value: 1}, {Key: "lot_no", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return &mongoRepository{col: col}
}

func (r *mongoRepository) Create(ctx context.Context, l *Lot) error {
	if l.ID.IsZero() {
		l.ID = primitive.NewObjectID()
	}
	if l.CreatedAt.IsZero() {
		l.CreatedAt = time.Now().UTC()
	}
	_, err := r.col.InsertOne(ctx, l)
	return err
}

func (r *mongoRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*Lot, error) {
	var l Lot
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&l)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *mongoRepository) FindByProductAndLotNo(ctx context.Context, productID primitive.ObjectID, lotNo string) (*Lot, error) {
	var l Lot
	err := r.col.FindOne(ctx, bson.M{"product_id": productID, "lot_no": lotNo}).Decode(&l)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *mongoRepository) ListByProduct(ctx context.Context, productID primitive.ObjectID) ([]*Lot, error) {
	cursor, err := r.col.Find(ctx, bson.M{"product_id": productID}, options.Find().SetSort(bson.D{{Key: "exp_date", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var lots []*Lot
	if err := cursor.All(ctx, &lots); err != nil {
		return nil, err
	}
	return lots, nil
}

func (r *mongoRepository) ListAll(ctx context.Context, page, limit int) ([]*Lot, int64, error) {
	skip := (page - 1) * limit
	opts := options.Find().SetSkip(int64(skip)).SetLimit(int64(limit)).SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	var lots []*Lot
	if err := cursor.All(ctx, &lots); err != nil {
		return nil, 0, err
	}
	total, err := r.col.CountDocuments(ctx, bson.M{})
	return lots, total, err
}

// FindOrCreate atomically finds an existing lot by (productID, lotNo) or creates one if missing.
func (r *mongoRepository) FindOrCreate(ctx context.Context, productID primitive.ObjectID, lotNo string, expDate *time.Time, mfgDate *time.Time) (*Lot, error) {
	filter := bson.M{"product_id": productID, "lot_no": lotNo}

	now := time.Now().UTC()
	setOnInsert := bson.M{
		"_id":        primitive.NewObjectID(),
		"product_id": productID,
		"lot_no":     lotNo,
		"created_at": now,
	}
	if expDate != nil {
		setOnInsert["exp_date"] = *expDate
	}
	if mfgDate != nil {
		setOnInsert["mfg_date"] = *mfgDate
	}

	update := bson.M{"$setOnInsert": setOnInsert}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var l Lot
	err := r.col.FindOneAndUpdate(ctx, filter, update, opts).Decode(&l)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// FindExpiring returns lots whose exp_date is before the given time.
func (r *mongoRepository) FindExpiring(ctx context.Context, before time.Time) ([]*Lot, error) {
	filter := bson.M{
		"exp_date": bson.M{
			"$ne":  nil,
			"$lte": before,
		},
	}
	cursor, err := r.col.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "exp_date", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var lots []*Lot
	if err := cursor.All(ctx, &lots); err != nil {
		return nil, err
	}
	return lots, nil
}
