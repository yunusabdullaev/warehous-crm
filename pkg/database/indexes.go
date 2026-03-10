package database

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EnsureIndexes creates all required indexes idempotently.
// Safe to call on every startup — MongoDB ignores if index already exists.
func EnsureIndexes(db *mongo.Database) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	indexes := []struct {
		Collection string
		Models     []mongo.IndexModel
	}{
		{
			Collection: "users",
			Models: []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "username", Value: 1}},
					Options: options.Index().SetUnique(true),
				},
			},
		},
		{
			Collection: "products",
			Models: []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "sku", Value: 1}},
					Options: options.Index().SetUnique(true),
				},
			},
		},
		{
			Collection: "locations",
			Models: []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "code", Value: 1}},
					Options: options.Index().SetUnique(true),
				},
				{
					Keys:    bson.D{{Key: "zone", Value: 1}, {Key: "rack", Value: 1}, {Key: "level", Value: 1}},
					Options: options.Index().SetUnique(true).SetSparse(true),
				},
			},
		},
		{
			Collection: "inbounds",
			Models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "created_at", Value: -1}}},
				{Keys: bson.D{{Key: "product_id", Value: 1}, {Key: "location_id", Value: 1}}},
				{Keys: bson.D{{Key: "created_at", Value: -1}, {Key: "product_id", Value: 1}}},
				{Keys: bson.D{{Key: "status", Value: 1}, {Key: "created_at", Value: -1}}},
			},
		},
		{
			Collection: "outbounds",
			Models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "created_at", Value: -1}}},
				{Keys: bson.D{{Key: "product_id", Value: 1}, {Key: "location_id", Value: 1}}},
				{Keys: bson.D{{Key: "created_at", Value: -1}, {Key: "product_id", Value: 1}}},
				{Keys: bson.D{{Key: "status", Value: 1}, {Key: "created_at", Value: -1}}},
			},
		},
		{
			Collection: "stocks",
			Models: []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "product_id", Value: 1}, {Key: "location_id", Value: 1}},
					Options: options.Index().SetUnique(true),
				},
			},
		},
		{
			Collection: "history",
			Models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "timestamp", Value: -1}}},
				{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "timestamp", Value: -1}}},
				{Keys: bson.D{{Key: "entity_type", Value: 1}, {Key: "entity_id", Value: 1}}},
			},
		},
		{
			Collection: "adjustments",
			Models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "created_at", Value: -1}}},
				{Keys: bson.D{{Key: "product_id", Value: 1}, {Key: "location_id", Value: 1}}},
				{Keys: bson.D{{Key: "created_at", Value: -1}, {Key: "product_id", Value: 1}}},
			},
		},
		{
			Collection: "orders",
			Models: []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "order_no", Value: 1}},
					Options: options.Index().SetUnique(true),
				},
				{Keys: bson.D{{Key: "status", Value: 1}, {Key: "created_at", Value: -1}}},
				{Keys: bson.D{{Key: "client_name", Value: 1}}},
			},
		},
		{
			Collection: "reservations",
			Models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "order_id", Value: 1}}},
				{Keys: bson.D{{Key: "product_id", Value: 1}, {Key: "status", Value: 1}}},
				{Keys: bson.D{{Key: "status", Value: 1}}},
			},
		},
		{
			Collection: "pick_tasks",
			Models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "order_id", Value: 1}}},
				{Keys: bson.D{{Key: "status", Value: 1}, {Key: "order_id", Value: 1}}},
				{Keys: bson.D{{Key: "assigned_to", Value: 1}, {Key: "status", Value: 1}}},
			},
		},
		{
			Collection: "pick_events",
			Models: []mongo.IndexModel{
				{Keys: bson.D{{Key: "order_id", Value: 1}}},
				{Keys: bson.D{{Key: "pick_task_id", Value: 1}}},
				{Keys: bson.D{{Key: "scanned_at", Value: -1}}},
			},
		},
	}

	for _, idx := range indexes {
		col := db.Collection(idx.Collection)
		names, err := col.Indexes().CreateMany(ctx, idx.Models)
		if err != nil {
			slog.Warn("index creation failed", "collection", idx.Collection, "error", err)
		} else {
			slog.Info("indexes ensured", "collection", idx.Collection, "indexes", names)
		}
	}
}
