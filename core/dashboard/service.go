package dashboard

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Service struct {
	db *mongo.Database
}

func NewService(db *mongo.Database) *Service {
	return &Service{db: db}
}

type SummaryResponse struct {
	TotalProducts        int64              `json:"total_products"`
	TotalLocations       int64              `json:"total_locations"`
	TotalStockQty        int                `json:"total_stock_qty"`
	InboundCount         int                `json:"inbound_count"`
	OutboundCount        int                `json:"outbound_count"`
	InboundQtyTotal      int                `json:"inbound_qty_total"`
	OutboundQtyTotal     int                `json:"outbound_qty_total"`
	AdjustmentsCount     int                `json:"adjustments_count"`
	AdjustmentsQtyNet    int                `json:"adjustments_qty_net"`
	AdjustmentsQtyAbs    int                `json:"adjustments_qty_abs"`
	OpenOrdersCount      int64              `json:"open_orders_count"`
	ReservedQtyTotal     int                `json:"reserved_qty_total"`
	PickingOrdersCount   int64              `json:"picking_orders_count"`
	PickTasksOpen        int64              `json:"pick_tasks_open"`
	OpenReturnsCount     int64              `json:"open_returns_count"`
	ReturnsReceivedCount int64              `json:"returns_received_count"`
	ExpiringLotsCount    int64              `json:"expiring_lots_count"`
	TopMoving            []TopMovingProduct `json:"top_moving_products"`
	StockByZone          []ZoneStock        `json:"stock_by_zone"`
}

type TopMovingProduct struct {
	ProductID string `bson:"_id" json:"product_id"`
	TotalQty  int    `bson:"total_qty" json:"total_qty"`
}

type ZoneStock struct {
	Zone     string `bson:"_id" json:"zone"`
	Quantity int    `bson:"quantity" json:"quantity"`
}

// warehouseFilter returns a bson.M that scopes queries to a specific warehouse.
// If warehouseID is zero (admin ALL mode), returns empty filter (no scoping).
func warehouseFilter(warehouseID primitive.ObjectID) bson.M {
	if warehouseID.IsZero() {
		return bson.M{}
	}
	return bson.M{"warehouse_id": warehouseID}
}

// mergeFilters combines multiple bson.M into one.
func mergeFilters(filters ...bson.M) bson.M {
	merged := bson.M{}
	for _, f := range filters {
		for k, v := range f {
			merged[k] = v
		}
	}
	return merged
}

func (s *Service) GetSummary(ctx context.Context, warehouseID primitive.ObjectID, from, to time.Time) (*SummaryResponse, error) {
	resp := &SummaryResponse{}
	wf := warehouseFilter(warehouseID)

	// 1. Total products (global — not warehouse-scoped)
	totalProducts, err := s.db.Collection("products").CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	resp.TotalProducts = totalProducts

	// 2. Total locations (warehouse-scoped)
	totalLocations, err := s.db.Collection("locations").CountDocuments(ctx, wf)
	if err != nil {
		return nil, err
	}
	resp.TotalLocations = totalLocations

	// 3. Total stock qty (warehouse-scoped)
	stockPipeline := mongo.Pipeline{
		{{Key: "$match", Value: wf}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: "$quantity"}}},
		}}},
	}
	stockCur, err := s.db.Collection("stocks").Aggregate(ctx, stockPipeline)
	if err != nil {
		return nil, err
	}
	var stockResult []struct {
		Total int `bson:"total"`
	}
	if err := stockCur.All(ctx, &stockResult); err != nil {
		return nil, err
	}
	if len(stockResult) > 0 {
		resp.TotalStockQty = stockResult[0].Total
	}

	// 4. Inbound count + qty (date-filtered + warehouse-scoped)
	dateFilter := bson.M{}
	if !from.IsZero() || !to.IsZero() {
		dateRange := bson.M{}
		if !from.IsZero() {
			dateRange["$gte"] = from
		}
		if !to.IsZero() {
			dateRange["$lte"] = to
		}
		dateFilter["created_at"] = dateRange
	}

	inboundMatch := mergeFilters(dateFilter, wf)
	inboundPipeline := mongo.Pipeline{
		{{Key: "$match", Value: inboundMatch}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "total_qty", Value: bson.D{{Key: "$sum", Value: "$quantity"}}},
		}}},
	}
	inCur, err := s.db.Collection("inbounds").Aggregate(ctx, inboundPipeline)
	if err != nil {
		return nil, err
	}
	var inResult []struct {
		Count    int `bson:"count"`
		TotalQty int `bson:"total_qty"`
	}
	if err := inCur.All(ctx, &inResult); err != nil {
		return nil, err
	}
	if len(inResult) > 0 {
		resp.InboundCount = inResult[0].Count
		resp.InboundQtyTotal = inResult[0].TotalQty
	}

	// 5. Outbound count + qty (date-filtered + warehouse-scoped)
	outboundMatch := mergeFilters(dateFilter, wf)
	outboundPipeline := mongo.Pipeline{
		{{Key: "$match", Value: outboundMatch}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "total_qty", Value: bson.D{{Key: "$sum", Value: "$quantity"}}},
		}}},
	}
	outCur, err := s.db.Collection("outbounds").Aggregate(ctx, outboundPipeline)
	if err != nil {
		return nil, err
	}
	var outResult []struct {
		Count    int `bson:"count"`
		TotalQty int `bson:"total_qty"`
	}
	if err := outCur.All(ctx, &outResult); err != nil {
		return nil, err
	}
	if len(outResult) > 0 {
		resp.OutboundCount = outResult[0].Count
		resp.OutboundQtyTotal = outResult[0].TotalQty
	}

	// 6. Adjustments count + net qty + abs qty (date-filtered + warehouse-scoped)
	adjMatch := mergeFilters(dateFilter, wf)
	adjPipeline := mongo.Pipeline{
		{{Key: "$match", Value: adjMatch}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "net_qty", Value: bson.D{{Key: "$sum", Value: "$delta_qty"}}},
			{Key: "abs_qty", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$abs", Value: "$delta_qty"}}}}},
		}}},
	}
	adjCur, err := s.db.Collection("adjustments").Aggregate(ctx, adjPipeline)
	if err != nil {
		return nil, err
	}
	var adjResult []struct {
		Count  int `bson:"count"`
		NetQty int `bson:"net_qty"`
		AbsQty int `bson:"abs_qty"`
	}
	if err := adjCur.All(ctx, &adjResult); err != nil {
		return nil, err
	}
	if len(adjResult) > 0 {
		resp.AdjustmentsCount = adjResult[0].Count
		resp.AdjustmentsQtyNet = adjResult[0].NetQty
		resp.AdjustmentsQtyAbs = adjResult[0].AbsQty
	}

	// 6b. Open orders count (CONFIRMED + PICKING) — warehouse-scoped
	openOrderFilter := mergeFilters(bson.M{
		"status": bson.M{"$in": bson.A{"CONFIRMED", "PICKING"}},
	}, wf)
	openCount, err := s.db.Collection("orders").CountDocuments(ctx, openOrderFilter)
	if err == nil {
		resp.OpenOrdersCount = openCount
	}

	// 6c. Picking orders count — warehouse-scoped
	pickingFilter := mergeFilters(bson.M{"status": "PICKING"}, wf)
	pickingCount, err := s.db.Collection("orders").CountDocuments(ctx, pickingFilter)
	if err == nil {
		resp.PickingOrdersCount = pickingCount
	}

	// 6d. Open pick tasks count — warehouse-scoped
	pickTaskFilter := mergeFilters(bson.M{
		"status": bson.M{"$in": bson.A{"OPEN", "IN_PROGRESS"}},
	}, wf)
	pickTasksOpen, err := s.db.Collection("pick_tasks").CountDocuments(ctx, pickTaskFilter)
	if err == nil {
		resp.PickTasksOpen = pickTasksOpen
	}

	// 6e. Reserved qty total (all ACTIVE reservations) — warehouse-scoped
	resMatch := mergeFilters(bson.M{"status": "ACTIVE"}, wf)
	resPipeline := mongo.Pipeline{
		{{Key: "$match", Value: resMatch}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: "$qty"}}},
		}}},
	}
	resCur, err := s.db.Collection("reservations").Aggregate(ctx, resPipeline)
	if err == nil {
		var resResult []struct {
			Total int `bson:"total"`
		}
		if err := resCur.All(ctx, &resResult); err == nil && len(resResult) > 0 {
			resp.ReservedQtyTotal = resResult[0].Total
		}
	}

	// 7. Top 5 moving products (inbound + outbound qty combined) — warehouse-scoped
	topInMatch := mergeFilters(dateFilter, wf)
	topOutMatch := mergeFilters(dateFilter, wf)
	topPipeline := mongo.Pipeline{
		{{Key: "$match", Value: topInMatch}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$product_id"},
			{Key: "total_qty", Value: bson.D{{Key: "$sum", Value: "$quantity"}}},
		}}},
		{{Key: "$unionWith", Value: bson.D{
			{Key: "coll", Value: "outbounds"},
			{Key: "pipeline", Value: mongo.Pipeline{
				{{Key: "$match", Value: topOutMatch}},
				{{Key: "$group", Value: bson.D{
					{Key: "_id", Value: "$product_id"},
					{Key: "total_qty", Value: bson.D{{Key: "$sum", Value: "$quantity"}}},
				}}},
			}},
		}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$_id"},
			{Key: "total_qty", Value: bson.D{{Key: "$sum", Value: "$total_qty"}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "total_qty", Value: -1}}}},
		{{Key: "$limit", Value: 5}},
	}
	topCur, err := s.db.Collection("inbounds").Aggregate(ctx, topPipeline)
	if err != nil {
		return nil, err
	}
	var topProducts []TopMovingProduct
	if err := topCur.All(ctx, &topProducts); err != nil {
		return nil, err
	}
	for i, tp := range topProducts {
		topProducts[i].ProductID = tp.ProductID
	}
	resp.TopMoving = topProducts
	if resp.TopMoving == nil {
		resp.TopMoving = []TopMovingProduct{}
	}

	// 8. Stock by zone (lookup locations) — warehouse-scoped
	zonePipeline := mongo.Pipeline{
		{{Key: "$match", Value: wf}},
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "locations"},
			{Key: "localField", Value: "location_id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "location"},
		}}},
		{{Key: "$unwind", Value: bson.D{
			{Key: "path", Value: "$location"},
			{Key: "preserveNullAndEmptyArrays", Value: true},
		}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{{Key: "$ifNull", Value: bson.A{"$location.zone", "unknown"}}}},
			{Key: "quantity", Value: bson.D{{Key: "$sum", Value: "$quantity"}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}
	zoneCur, err := s.db.Collection("stocks").Aggregate(ctx, zonePipeline)
	if err != nil {
		return nil, err
	}
	var zoneStocks []ZoneStock
	if err := zoneCur.All(ctx, &zoneStocks); err != nil {
		return nil, err
	}
	resp.StockByZone = zoneStocks
	if resp.StockByZone == nil {
		resp.StockByZone = []ZoneStock{}
	}

	// 9. Open returns count — warehouse-scoped
	openReturnFilter := mergeFilters(bson.M{"status": "OPEN"}, wf)
	openReturns, err := s.db.Collection("returns").CountDocuments(ctx, openReturnFilter)
	if err == nil {
		resp.OpenReturnsCount = openReturns
	}

	// 9b. Expiring lots count (within 30 days) — uses stocks collection for warehouse scope
	expiryDeadline := time.Now().UTC().AddDate(0, 0, 30)
	lotFilter := bson.M{
		"exp_date": bson.M{
			"$ne":  nil,
			"$lte": expiryDeadline,
			"$gte": time.Now().UTC(),
		},
	}
	if !warehouseID.IsZero() {
		// Count lots that have stock rows in this warehouse
		expiryPipeline := mongo.Pipeline{
			{{Key: "$match", Value: lotFilter}},
			{{Key: "$lookup", Value: bson.D{
				{Key: "from", Value: "stocks"},
				{Key: "let", Value: bson.D{{Key: "lot_id", Value: "$_id"}}},
				{Key: "pipeline", Value: mongo.Pipeline{
					{{Key: "$match", Value: bson.D{
						{Key: "$expr", Value: bson.D{
							{Key: "$and", Value: bson.A{
								bson.D{{Key: "$eq", Value: bson.A{"$lot_id", "$$lot_id"}}},
								bson.D{{Key: "$eq", Value: bson.A{"$warehouse_id", warehouseID}}},
							}},
						}},
					}}},
				}},
				{Key: "as", Value: "wh_stocks"},
			}}},
			{{Key: "$match", Value: bson.M{"wh_stocks": bson.M{"$ne": bson.A{}}}}},
			{{Key: "$count", Value: "count"}},
		}
		expCur, err := s.db.Collection("lots").Aggregate(ctx, expiryPipeline)
		if err == nil {
			var expResult []struct {
				Count int64 `bson:"count"`
			}
			if err := expCur.All(ctx, &expResult); err == nil && len(expResult) > 0 {
				resp.ExpiringLotsCount = expResult[0].Count
			}
		}
	} else {
		expiringLots, err := s.db.Collection("lots").CountDocuments(ctx, lotFilter)
		if err == nil {
			resp.ExpiringLotsCount = expiringLots
		}
	}

	// 10. Returns received count (date-filtered + warehouse-scoped)
	returnsReceivedFilter := mergeFilters(bson.M{"status": "RECEIVED"}, wf)
	if !from.IsZero() || !to.IsZero() {
		dateRange := bson.M{}
		if !from.IsZero() {
			dateRange["$gte"] = from
		}
		if !to.IsZero() {
			dateRange["$lte"] = to
		}
		returnsReceivedFilter["received_at"] = dateRange
	}
	returnsReceived, err := s.db.Collection("returns").CountDocuments(ctx, returnsReceivedFilter)
	if err == nil {
		resp.ReturnsReceivedCount = returnsReceived
	}

	return resp, nil
}
