package reports

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

// warehouseFilter returns a bson.M that scopes queries by warehouse.
// Zero ObjectID (admin ALL mode) returns empty filter.
func warehouseFilter(warehouseID primitive.ObjectID) bson.M {
	if warehouseID.IsZero() {
		return bson.M{}
	}
	return bson.M{"warehouse_id": warehouseID}
}

func mergeFilters(filters ...bson.M) bson.M {
	merged := bson.M{}
	for _, f := range filters {
		for k, v := range f {
			merged[k] = v
		}
	}
	return merged
}

// ── Movement Report ─────────────────────────────────────────

type MovementBucket struct {
	Period        string `bson:"_id" json:"period"`
	InboundQty    int    `bson:"inbound_qty" json:"inbound_qty"`
	OutboundQty   int    `bson:"outbound_qty" json:"outbound_qty"`
	AdjustmentQty int    `bson:"adjustment_qty" json:"adjustment_qty"`
}

type MovementsReport struct {
	From    string           `json:"from"`
	To      string           `json:"to"`
	GroupBy string           `json:"group_by"`
	Data    []MovementBucket `json:"data"`
}

func dateTruncUnit(groupBy string) string {
	switch groupBy {
	case "week":
		return "week"
	case "month":
		return "month"
	default:
		return "day"
	}
}

func dateFormat(groupBy string) string {
	switch groupBy {
	case "week":
		return "%Y-W%V"
	case "month":
		return "%Y-%m"
	default:
		return "%Y-%m-%d"
	}
}

func (s *Service) GetMovements(ctx context.Context, warehouseID primitive.ObjectID, from, to time.Time, groupBy string, includeAdjustments bool) (*MovementsReport, error) {
	unit := dateTruncUnit(groupBy)
	format := dateFormat(groupBy)
	wf := warehouseFilter(warehouseID)

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

	matchFilter := mergeFilters(dateFilter, wf)

	groupPipeline := mongo.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "$dateToString", Value: bson.D{
					{Key: "format", Value: format},
					{Key: "date", Value: bson.D{
						{Key: "$dateTrunc", Value: bson.D{
							{Key: "date", Value: "$created_at"},
							{Key: "unit", Value: unit},
						}},
					}},
				}},
			}},
			{Key: "qty", Value: bson.D{{Key: "$sum", Value: "$quantity"}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}

	type bucketResult struct {
		Period string `bson:"_id"`
		Qty    int    `bson:"qty"`
	}

	inCur, err := s.db.Collection("inbounds").Aggregate(ctx, groupPipeline)
	if err != nil {
		return nil, err
	}
	var inResults []bucketResult
	if err := inCur.All(ctx, &inResults); err != nil {
		return nil, err
	}

	outCur, err := s.db.Collection("outbounds").Aggregate(ctx, groupPipeline)
	if err != nil {
		return nil, err
	}
	var outResults []bucketResult
	if err := outCur.All(ctx, &outResults); err != nil {
		return nil, err
	}

	bucketMap := make(map[string]*MovementBucket)
	for _, r := range inResults {
		bucketMap[r.Period] = &MovementBucket{Period: r.Period, InboundQty: r.Qty}
	}
	for _, r := range outResults {
		if b, ok := bucketMap[r.Period]; ok {
			b.OutboundQty = r.Qty
		} else {
			bucketMap[r.Period] = &MovementBucket{Period: r.Period, OutboundQty: r.Qty}
		}
	}

	if includeAdjustments {
		adjGroupPipeline := mongo.Pipeline{
			{{Key: "$match", Value: matchFilter}},
			{{Key: "$group", Value: bson.D{
				{Key: "_id", Value: bson.D{
					{Key: "$dateToString", Value: bson.D{
						{Key: "format", Value: format},
						{Key: "date", Value: bson.D{
							{Key: "$dateTrunc", Value: bson.D{
								{Key: "date", Value: "$created_at"},
								{Key: "unit", Value: unit},
							}},
						}},
					}},
				}},
				{Key: "qty", Value: bson.D{{Key: "$sum", Value: "$delta_qty"}}},
			}}},
			{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
		}
		adjCur, err := s.db.Collection("adjustments").Aggregate(ctx, adjGroupPipeline)
		if err != nil {
			return nil, err
		}
		var adjResults []bucketResult
		if err := adjCur.All(ctx, &adjResults); err != nil {
			return nil, err
		}
		for _, r := range adjResults {
			if b, ok := bucketMap[r.Period]; ok {
				b.AdjustmentQty = r.Qty
			} else {
				bucketMap[r.Period] = &MovementBucket{Period: r.Period, AdjustmentQty: r.Qty}
			}
		}
	}

	data := make([]MovementBucket, 0, len(bucketMap))
	for _, b := range bucketMap {
		data = append(data, *b)
	}
	for i := 0; i < len(data); i++ {
		for j := i + 1; j < len(data); j++ {
			if data[i].Period > data[j].Period {
				data[i], data[j] = data[j], data[i]
			}
		}
	}

	report := &MovementsReport{
		GroupBy: groupBy,
		Data:    data,
	}
	if !from.IsZero() {
		report.From = from.Format("2006-01-02")
	}
	if !to.IsZero() {
		report.To = to.Format("2006-01-02")
	}
	if report.Data == nil {
		report.Data = []MovementBucket{}
	}
	return report, nil
}

// ── Stock Report ────────────────────────────────────────────

type StockGroup struct {
	Group    string `bson:"_id" json:"group"`
	Quantity int    `bson:"quantity" json:"quantity"`
}

type StockReport struct {
	GroupBy string       `json:"group_by"`
	Data    []StockGroup `json:"data"`
}

func (s *Service) GetStockReport(ctx context.Context, warehouseID primitive.ObjectID, groupBy string) (*StockReport, error) {
	wf := warehouseFilter(warehouseID)
	var pipeline mongo.Pipeline

	switch groupBy {
	case "zone":
		pipeline = mongo.Pipeline{
			{{Key: "$match", Value: wf}},
			{{Key: "$lookup", Value: bson.D{
				{Key: "from", Value: "locations"},
				{Key: "localField", Value: "location_id"},
				{Key: "foreignField", Value: "_id"},
				{Key: "as", Value: "loc"},
			}}},
			{{Key: "$unwind", Value: "$loc"}},
			{{Key: "$group", Value: bson.D{
				{Key: "_id", Value: bson.D{{Key: "$ifNull", Value: bson.A{"$loc.zone", "unknown"}}}},
				{Key: "quantity", Value: bson.D{{Key: "$sum", Value: "$quantity"}}},
			}}},
			{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
		}
	case "rack":
		pipeline = mongo.Pipeline{
			{{Key: "$match", Value: wf}},
			{{Key: "$lookup", Value: bson.D{
				{Key: "from", Value: "locations"},
				{Key: "localField", Value: "location_id"},
				{Key: "foreignField", Value: "_id"},
				{Key: "as", Value: "loc"},
			}}},
			{{Key: "$unwind", Value: "$loc"}},
			{{Key: "$group", Value: bson.D{
				{Key: "_id", Value: bson.D{{Key: "$ifNull", Value: bson.A{"$loc.rack", "unknown"}}}},
				{Key: "quantity", Value: bson.D{{Key: "$sum", Value: "$quantity"}}},
			}}},
			{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
		}
	default:
		groupBy = "product"
		pipeline = mongo.Pipeline{
			{{Key: "$match", Value: wf}},
			{{Key: "$group", Value: bson.D{
				{Key: "_id", Value: "$product_id"},
				{Key: "quantity", Value: bson.D{{Key: "$sum", Value: "$quantity"}}},
			}}},
			{{Key: "$sort", Value: bson.D{{Key: "quantity", Value: -1}}}},
		}
	}

	cursor, err := s.db.Collection("stocks").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	if groupBy == "product" {
		var rawResults []struct {
			ID  primitive.ObjectID `bson:"_id"`
			Qty int                `bson:"quantity"`
		}
		if err := cursor.All(ctx, &rawResults); err != nil {
			return nil, err
		}
		data := make([]StockGroup, len(rawResults))
		for i, r := range rawResults {
			data[i] = StockGroup{Group: r.ID.Hex(), Quantity: r.Qty}
		}
		return &StockReport{GroupBy: groupBy, Data: data}, nil
	}

	var data []StockGroup
	if err := cursor.All(ctx, &data); err != nil {
		return nil, err
	}
	if data == nil {
		data = []StockGroup{}
	}
	return &StockReport{GroupBy: groupBy, Data: data}, nil
}

// ── Order Report ────────────────────────────────────────────

type OrderBucket struct {
	Period    string `bson:"_id" json:"period"`
	Created   int    `json:"created"`
	Confirmed int    `json:"confirmed"`
	Shipped   int    `json:"shipped"`
}

type OrdersReport struct {
	From    string        `json:"from"`
	To      string        `json:"to"`
	GroupBy string        `json:"group_by"`
	Data    []OrderBucket `json:"data"`
}

func (s *Service) GetOrderReport(ctx context.Context, warehouseID primitive.ObjectID, from, to time.Time, groupBy string) (*OrdersReport, error) {
	format := dateFormat(groupBy)
	unit := dateTruncUnit(groupBy)
	wf := warehouseFilter(warehouseID)

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

	matchFilter := mergeFilters(dateFilter, wf)

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "$dateToString", Value: bson.D{
					{Key: "format", Value: format},
					{Key: "date", Value: bson.D{
						{Key: "$dateTrunc", Value: bson.D{
							{Key: "date", Value: "$created_at"},
							{Key: "unit", Value: unit},
						}},
					}},
				}},
			}},
			{Key: "created", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "confirmed", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: bson.A{
					bson.D{{Key: "$in", Value: bson.A{"$status", bson.A{"CONFIRMED", "PICKING", "SHIPPED"}}}},
					1, 0,
				}},
			}}}},
			{Key: "shipped", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: bson.A{
					bson.D{{Key: "$eq", Value: bson.A{"$status", "SHIPPED"}}},
					1, 0,
				}},
			}}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}

	cursor, err := s.db.Collection("orders").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	var data []OrderBucket
	if err := cursor.All(ctx, &data); err != nil {
		return nil, err
	}
	if data == nil {
		data = []OrderBucket{}
	}

	report := &OrdersReport{GroupBy: groupBy, Data: data}
	if !from.IsZero() {
		report.From = from.Format("2006-01-02")
	}
	if !to.IsZero() {
		report.To = to.Format("2006-01-02")
	}
	return report, nil
}

// ── Picking Report ──────────────────────────────────────────

type PickingBucket struct {
	Period         string  `bson:"_id" json:"period"`
	TasksCreated   int     `json:"tasks_created"`
	TasksCompleted int     `json:"tasks_completed"`
	AvgPickTimeSec float64 `json:"avg_pick_time_sec"`
}

type PickingReport struct {
	From    string          `json:"from"`
	To      string          `json:"to"`
	GroupBy string          `json:"group_by"`
	Data    []PickingBucket `json:"data"`
}

func (s *Service) GetPickingReport(ctx context.Context, warehouseID primitive.ObjectID, from, to time.Time, groupBy string) (*PickingReport, error) {
	format := dateFormat(groupBy)
	unit := dateTruncUnit(groupBy)
	wf := warehouseFilter(warehouseID)

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

	matchFilter := mergeFilters(dateFilter, wf)

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "$dateToString", Value: bson.D{
					{Key: "format", Value: format},
					{Key: "date", Value: bson.D{
						{Key: "$dateTrunc", Value: bson.D{
							{Key: "date", Value: "$created_at"},
							{Key: "unit", Value: unit},
						}},
					}},
				}},
			}},
			{Key: "tasks_created", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "tasks_completed", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: bson.A{
					bson.D{{Key: "$eq", Value: bson.A{"$status", "DONE"}}},
					1, 0,
				}},
			}}}},
			{Key: "avg_pick_time_sec", Value: bson.D{{Key: "$avg", Value: bson.D{
				{Key: "$cond", Value: bson.A{
					bson.D{{Key: "$eq", Value: bson.A{"$status", "DONE"}}},
					bson.D{{Key: "$divide", Value: bson.A{
						bson.D{{Key: "$subtract", Value: bson.A{"$updated_at", "$created_at"}}},
						1000,
					}}},
					nil,
				}},
			}}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}

	cursor, err := s.db.Collection("pick_tasks").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	var data []PickingBucket
	if err := cursor.All(ctx, &data); err != nil {
		return nil, err
	}
	if data == nil {
		data = []PickingBucket{}
	}

	report := &PickingReport{GroupBy: groupBy, Data: data}
	if !from.IsZero() {
		report.From = from.Format("2006-01-02")
	}
	if !to.IsZero() {
		report.To = to.Format("2006-01-02")
	}
	return report, nil
}

// ── Returns Report ──────────────────────────────────────────

type ReturnsBucket struct {
	Period          string `bson:"_id" json:"period"`
	ReturnsCreated  int    `json:"returns_created"`
	ReturnsReceived int    `json:"returns_received"`
	QtyRestocked    int    `json:"qty_restocked"`
	QtyDamaged      int    `json:"qty_damaged"`
	QtyQCHold       int    `json:"qty_qc_hold"`
}

type ReturnsReport struct {
	From    string          `json:"from"`
	To      string          `json:"to"`
	GroupBy string          `json:"group_by"`
	Data    []ReturnsBucket `json:"data"`
}

func (s *Service) GetReturnsReport(ctx context.Context, warehouseID primitive.ObjectID, from, to time.Time, groupBy string) (*ReturnsReport, error) {
	format := dateFormat(groupBy)
	unit := dateTruncUnit(groupBy)
	wf := warehouseFilter(warehouseID)

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

	matchFilter := mergeFilters(dateFilter, wf)

	returnsPipeline := mongo.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "$dateToString", Value: bson.D{
					{Key: "format", Value: format},
					{Key: "date", Value: bson.D{
						{Key: "$dateTrunc", Value: bson.D{
							{Key: "date", Value: "$created_at"},
							{Key: "unit", Value: unit},
						}},
					}},
				}},
			}},
			{Key: "created", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "received", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: bson.A{
					bson.D{{Key: "$eq", Value: bson.A{"$status", "RECEIVED"}}},
					1, 0,
				}},
			}}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}

	type retBucket struct {
		Period   string `bson:"_id"`
		Created  int    `bson:"created"`
		Received int    `bson:"received"`
	}

	retCur, err := s.db.Collection("returns").Aggregate(ctx, returnsPipeline)
	if err != nil {
		return nil, err
	}
	var retResults []retBucket
	if err := retCur.All(ctx, &retResults); err != nil {
		return nil, err
	}

	bucketMap := make(map[string]*ReturnsBucket)
	for _, r := range retResults {
		bucketMap[r.Period] = &ReturnsBucket{
			Period:          r.Period,
			ReturnsCreated:  r.Created,
			ReturnsReceived: r.Received,
		}
	}

	// Return items qty by disposition per period — warehouse-scoped
	itemsPipeline := mongo.Pipeline{
		{{Key: "$match", Value: matchFilter}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "period", Value: bson.D{
					{Key: "$dateToString", Value: bson.D{
						{Key: "format", Value: format},
						{Key: "date", Value: bson.D{
							{Key: "$dateTrunc", Value: bson.D{
								{Key: "date", Value: "$created_at"},
								{Key: "unit", Value: unit},
							}},
						}},
					}},
				}},
				{Key: "disposition", Value: "$disposition"},
			}},
			{Key: "total_qty", Value: bson.D{{Key: "$sum", Value: "$qty"}}},
		}}},
	}

	type itemBucket struct {
		ID struct {
			Period      string `bson:"period"`
			Disposition string `bson:"disposition"`
		} `bson:"_id"`
		TotalQty int `bson:"total_qty"`
	}

	itemCur, err := s.db.Collection("return_items").Aggregate(ctx, itemsPipeline)
	if err != nil {
		return nil, err
	}
	var itemResults []itemBucket
	if err := itemCur.All(ctx, &itemResults); err != nil {
		return nil, err
	}

	for _, r := range itemResults {
		b, ok := bucketMap[r.ID.Period]
		if !ok {
			b = &ReturnsBucket{Period: r.ID.Period}
			bucketMap[r.ID.Period] = b
		}
		switch r.ID.Disposition {
		case "RESTOCK":
			b.QtyRestocked = r.TotalQty
		case "DAMAGED":
			b.QtyDamaged = r.TotalQty
		case "QC_HOLD":
			b.QtyQCHold = r.TotalQty
		}
	}

	data := make([]ReturnsBucket, 0, len(bucketMap))
	for _, b := range bucketMap {
		data = append(data, *b)
	}
	for i := 0; i < len(data); i++ {
		for j := i + 1; j < len(data); j++ {
			if data[i].Period > data[j].Period {
				data[i], data[j] = data[j], data[i]
			}
		}
	}

	report := &ReturnsReport{GroupBy: groupBy, Data: data}
	if !from.IsZero() {
		report.From = from.Format("2006-01-02")
	}
	if !to.IsZero() {
		report.To = to.Format("2006-01-02")
	}
	if report.Data == nil {
		report.Data = []ReturnsBucket{}
	}
	return report, nil
}

// ── Expiry Report ──────────────────────────────────────────

type ExpiryItem struct {
	LotID     string `json:"lot_id"`
	LotNo     string `json:"lot_no"`
	ProductID string `json:"product_id"`
	ExpDate   string `json:"exp_date"`
	TotalQty  int    `json:"total_qty"`
}

type ExpiryReport struct {
	Days int          `json:"days"`
	Data []ExpiryItem `json:"data"`
}

func (s *Service) GetExpiryReport(ctx context.Context, warehouseID primitive.ObjectID, days int) (*ExpiryReport, error) {
	if days <= 0 {
		days = 30
	}

	deadline := time.Now().UTC().AddDate(0, 0, days)

	lotMatch := bson.D{
		{Key: "exp_date", Value: bson.D{
			{Key: "$ne", Value: nil},
			{Key: "$lte", Value: deadline},
		}},
	}

	// Build a stock sub-pipeline scoped to warehouse
	var stockSubPipeline mongo.Pipeline
	if !warehouseID.IsZero() {
		stockSubPipeline = mongo.Pipeline{
			{{Key: "$match", Value: bson.D{
				{Key: "$expr", Value: bson.D{
					{Key: "$and", Value: bson.A{
						bson.D{{Key: "$eq", Value: bson.A{"$lot_id", "$$lot_id"}}},
						bson.D{{Key: "$eq", Value: bson.A{"$warehouse_id", warehouseID}}},
					}},
				}},
			}}},
		}
	} else {
		stockSubPipeline = mongo.Pipeline{
			{{Key: "$match", Value: bson.D{
				{Key: "$expr", Value: bson.D{
					{Key: "$eq", Value: bson.A{"$lot_id", "$$lot_id"}},
				}},
			}}},
		}
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: lotMatch}},
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "stocks"},
			{Key: "let", Value: bson.D{{Key: "lot_id", Value: "$_id"}}},
			{Key: "pipeline", Value: stockSubPipeline},
			{Key: "as", Value: "stock_rows"},
		}}},
		{{Key: "$addFields", Value: bson.D{
			{Key: "total_qty", Value: bson.D{{Key: "$sum", Value: "$stock_rows.quantity"}}},
		}}},
		// Only include lots that have stock in the scoped warehouse
		{{Key: "$match", Value: bson.M{"total_qty": bson.M{"$gt": 0}}}},
		{{Key: "$project", Value: bson.D{
			{Key: "lot_no", Value: 1},
			{Key: "product_id", Value: 1},
			{Key: "exp_date", Value: 1},
			{Key: "total_qty", Value: 1},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "exp_date", Value: 1}}}},
	}

	cursor, err := s.db.Collection("lots").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	type rawResult struct {
		ID        primitive.ObjectID `bson:"_id"`
		LotNo     string             `bson:"lot_no"`
		ProductID primitive.ObjectID `bson:"product_id"`
		ExpDate   time.Time          `bson:"exp_date"`
		TotalQty  int                `bson:"total_qty"`
	}

	var rawResults []rawResult
	if err := cursor.All(ctx, &rawResults); err != nil {
		return nil, err
	}

	data := make([]ExpiryItem, len(rawResults))
	for i, r := range rawResults {
		data[i] = ExpiryItem{
			LotID:     r.ID.Hex(),
			LotNo:     r.LotNo,
			ProductID: r.ProductID.Hex(),
			ExpDate:   r.ExpDate.Format("2006-01-02"),
			TotalQty:  r.TotalQty,
		}
	}

	return &ExpiryReport{Days: days, Data: data}, nil
}
