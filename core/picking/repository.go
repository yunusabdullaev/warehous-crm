package picking

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Repository interface {
	CreateTask(ctx context.Context, t *PickTask) error
	CreateTaskBatch(ctx context.Context, tasks []*PickTask) error
	FindTaskByID(ctx context.Context, id primitive.ObjectID) (*PickTask, error)
	FindTasksByOrder(ctx context.Context, orderID primitive.ObjectID) ([]*PickTask, error)
	FindTasksByAssignee(ctx context.Context, userID string, statusFilter []string) ([]*PickTask, error)
	AtomicAddPicked(ctx context.Context, taskID primitive.ObjectID, qty int) (*PickTask, error)
	SetAssignee(ctx context.Context, taskID primitive.ObjectID, userID string) error
	CancelByOrder(ctx context.Context, orderID primitive.ObjectID) (int64, error)
	AllDoneForOrder(ctx context.Context, orderID primitive.ObjectID) (bool, error)
	InsertEvent(ctx context.Context, e *PickEvent) error
	EventsByOrder(ctx context.Context, orderID primitive.ObjectID) ([]*PickEvent, error)
	EventsByTask(ctx context.Context, taskID primitive.ObjectID) ([]*PickEvent, error)
}

type mongoRepository struct {
	tasks  *mongo.Collection
	events *mongo.Collection
}

func NewRepository(tasks, events *mongo.Collection) Repository {
	return &mongoRepository{tasks: tasks, events: events}
}

func (r *mongoRepository) CreateTask(ctx context.Context, t *PickTask) error {
	now := time.Now().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now
	t.Status = TaskStatusOpen
	t.PickedQty = 0
	res, err := r.tasks.InsertOne(ctx, t)
	if err != nil {
		return err
	}
	t.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *mongoRepository) CreateTaskBatch(ctx context.Context, tasks []*PickTask) error {
	now := time.Now().UTC()
	docs := make([]interface{}, len(tasks))
	for i, t := range tasks {
		t.CreatedAt = now
		t.UpdatedAt = now
		t.Status = TaskStatusOpen
		t.PickedQty = 0
		docs[i] = t
	}
	res, err := r.tasks.InsertMany(ctx, docs)
	if err != nil {
		return err
	}
	for i, id := range res.InsertedIDs {
		tasks[i].ID = id.(primitive.ObjectID)
	}
	return nil
}

func (r *mongoRepository) FindTaskByID(ctx context.Context, id primitive.ObjectID) (*PickTask, error) {
	var t PickTask
	err := r.tasks.FindOne(ctx, bson.M{"_id": id}).Decode(&t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *mongoRepository) FindTasksByOrder(ctx context.Context, orderID primitive.ObjectID) ([]*PickTask, error) {
	opts := options.Find().SetSort(bson.D{{Key: "location_id", Value: 1}, {Key: "product_id", Value: 1}})
	cursor, err := r.tasks.Find(ctx, bson.M{"order_id": orderID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var tasks []*PickTask
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r *mongoRepository) FindTasksByAssignee(ctx context.Context, userID string, statusFilter []string) ([]*PickTask, error) {
	filter := bson.M{"assigned_to": userID}
	if len(statusFilter) > 0 {
		filter["status"] = bson.M{"$in": statusFilter}
	}
	opts := options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}})
	cursor, err := r.tasks.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var tasks []*PickTask
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// AtomicAddPicked increments pickedQty atomically.
// Guard: status must be OPEN or IN_PROGRESS AND pickedQty + qty <= plannedQty.
// Transitions: OPEN→IN_PROGRESS on first pick, any→DONE when pickedQty == plannedQty.
// Returns updated task or mongo.ErrNoDocuments if guard fails.
func (r *mongoRepository) AtomicAddPicked(ctx context.Context, taskID primitive.ObjectID, qty int) (*PickTask, error) {
	// Use findOneAndUpdate with a filter that guards pickedQty + qty <= plannedQty
	filter := bson.M{
		"_id":    taskID,
		"status": bson.M{"$in": bson.A{TaskStatusOpen, TaskStatusInProgress}},
		"$expr": bson.M{
			"$lte": bson.A{
				bson.M{"$add": bson.A{"$picked_qty", qty}},
				"$planned_qty",
			},
		},
	}

	now := time.Now().UTC()
	update := bson.A{
		bson.M{"$set": bson.M{
			"picked_qty": bson.M{"$add": bson.A{"$picked_qty", qty}},
			"updated_at": now,
			"status": bson.M{
				"$cond": bson.A{
					bson.M{"$eq": bson.A{bson.M{"$add": bson.A{"$picked_qty", qty}}, "$planned_qty"}},
					TaskStatusDone,
					TaskStatusInProgress,
				},
			},
		}},
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var result PickTask
	err := r.tasks.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *mongoRepository) SetAssignee(ctx context.Context, taskID primitive.ObjectID, userID string) error {
	filter := bson.M{
		"_id":    taskID,
		"status": bson.M{"$in": bson.A{TaskStatusOpen, TaskStatusInProgress}},
	}
	update := bson.M{"$set": bson.M{
		"assigned_to": userID,
		"updated_at":  time.Now().UTC(),
	}}
	res, err := r.tasks.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// CancelByOrder sets OPEN/IN_PROGRESS tasks to CANCELLED for an order.
func (r *mongoRepository) CancelByOrder(ctx context.Context, orderID primitive.ObjectID) (int64, error) {
	filter := bson.M{
		"order_id": orderID,
		"status":   bson.M{"$in": bson.A{TaskStatusOpen, TaskStatusInProgress}},
	}
	update := bson.M{"$set": bson.M{
		"status":     TaskStatusCancelled,
		"updated_at": time.Now().UTC(),
	}}
	res, err := r.tasks.UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

// AllDoneForOrder returns true if all pick tasks for an order have status DONE.
func (r *mongoRepository) AllDoneForOrder(ctx context.Context, orderID primitive.ObjectID) (bool, error) {
	// Count tasks that are NOT done
	notDone, err := r.tasks.CountDocuments(ctx, bson.M{
		"order_id": orderID,
		"status":   bson.M{"$ne": TaskStatusDone},
	})
	if err != nil {
		return false, err
	}
	// Also ensure there's at least one task
	total, err := r.tasks.CountDocuments(ctx, bson.M{"order_id": orderID})
	if err != nil {
		return false, err
	}
	return total > 0 && notDone == 0, nil
}

func (r *mongoRepository) InsertEvent(ctx context.Context, e *PickEvent) error {
	e.ScannedAt = time.Now().UTC()
	res, err := r.events.InsertOne(ctx, e)
	if err != nil {
		return err
	}
	e.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *mongoRepository) EventsByOrder(ctx context.Context, orderID primitive.ObjectID) ([]*PickEvent, error) {
	opts := options.Find().SetSort(bson.D{{Key: "scanned_at", Value: 1}})
	cursor, err := r.events.Find(ctx, bson.M{"order_id": orderID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var events []*PickEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}
	return events, nil
}

func (r *mongoRepository) EventsByTask(ctx context.Context, taskID primitive.ObjectID) ([]*PickEvent, error) {
	opts := options.Find().SetSort(bson.D{{Key: "scanned_at", Value: 1}})
	cursor, err := r.events.Find(ctx, bson.M{"pick_task_id": taskID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var events []*PickEvent
	if err := cursor.All(ctx, &events); err != nil {
		return nil, err
	}
	return events, nil
}
