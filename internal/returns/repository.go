package returns

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository struct {
	returns     *mongo.Collection
	returnItems *mongo.Collection
	qcHolds     *mongo.Collection
	counters    *mongo.Collection
}

func NewRepository(returns, returnItems, qcHolds, counters *mongo.Collection) *Repository {
	return &Repository{
		returns:     returns,
		returnItems: returnItems,
		qcHolds:     qcHolds,
		counters:    counters,
	}
}

// ── Returns ──

func (r *Repository) Create(ctx context.Context, ret *Return) error {
	ret.CreatedAt = time.Now().UTC()
	ret.Status = StatusOpen
	res, err := r.returns.InsertOne(ctx, ret)
	if err != nil {
		return err
	}
	ret.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *Repository) FindByID(ctx context.Context, id primitive.ObjectID) (*Return, error) {
	var ret Return
	err := r.returns.FindOne(ctx, bson.M{"_id": id}).Decode(&ret)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

func (r *Repository) List(ctx context.Context, filter bson.M, page, limit int) ([]*Return, int64, error) {
	skip := (page - 1) * limit
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.returns.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var results []*Return
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, err
	}

	total, err := r.returns.CountDocuments(ctx, filter)
	return results, total, err
}

func (r *Repository) UpdateStatus(ctx context.Context, id primitive.ObjectID, fromStatus, toStatus string, setFields bson.M) error {
	filter := bson.M{"_id": id, "status": fromStatus}
	if setFields == nil {
		setFields = bson.M{}
	}
	setFields["status"] = toStatus
	update := bson.M{"$set": setFields}

	res, err := r.returns.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// NextRMANo generates a sequential RMA number using the counters collection.
func (r *Repository) NextRMANo(ctx context.Context) (string, error) {
	year := time.Now().UTC().Year()
	counterID := fmt.Sprintf("rma_%d", year)

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

	return fmt.Sprintf("RMA-%d-%06d", year, result.Seq), nil
}

// ── Return Items ──

func (r *Repository) CreateItem(ctx context.Context, item *ReturnItem) error {
	item.CreatedAt = time.Now().UTC()
	res, err := r.returnItems.InsertOne(ctx, item)
	if err != nil {
		return err
	}
	item.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *Repository) ListItems(ctx context.Context, returnID primitive.ObjectID) ([]*ReturnItem, error) {
	cursor, err := r.returnItems.Find(ctx, bson.M{"return_id": returnID},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []*ReturnItem
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// SumReturnedQty aggregates total returned qty for a product across ALL RMAs for a given order.
func (r *Repository) SumReturnedQty(ctx context.Context, orderID, productID primitive.ObjectID) (int, error) {
	// First get all return IDs for this order
	returnsCursor, err := r.returns.Find(ctx, bson.M{
		"order_id": orderID,
		"status":   bson.M{"$ne": StatusCancelled},
	}, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return 0, err
	}
	defer returnsCursor.Close(ctx)

	var returnDocs []struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	if err := returnsCursor.All(ctx, &returnDocs); err != nil {
		return 0, err
	}
	if len(returnDocs) == 0 {
		return 0, nil
	}

	var returnIDs []primitive.ObjectID
	for _, rd := range returnDocs {
		returnIDs = append(returnIDs, rd.ID)
	}

	// Aggregate sum of qty for this product across those returns
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"return_id":  bson.M{"$in": returnIDs},
			"product_id": productID,
		}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: "$qty"}}},
		}}},
	}

	cursor, err := r.returnItems.Aggregate(ctx, pipeline)
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

// ── QC Holds ──

func (r *Repository) CreateQCHold(ctx context.Context, hold *QCHold) error {
	hold.CreatedAt = time.Now().UTC()
	hold.Status = QCStatusHeld
	res, err := r.qcHolds.InsertOne(ctx, hold)
	if err != nil {
		return err
	}
	hold.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}
