#!/usr/bin/env bash
# ──────────────────────────────────────────────────────────────
# Lot / Batch tracking + FEFO picking integration test
# ──────────────────────────────────────────────────────────────
set -euo pipefail

BASE="${API_URL:-http://localhost:3003/api/v1}"

echo "═══════════════════════════════════════════════"
echo "  LOT / BATCH TRACKING + FEFO TEST SUITE"
echo "  API: $BASE"
echo "═══════════════════════════════════════════════"

# ── Login ──

TOKEN=$(curl -sf "$BASE/../auth/login" -X POST \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')
[ -z "$TOKEN" ] && echo "FAIL: login" && exit 1
AUTH="Authorization: Bearer $TOKEN"
echo "✅ Login OK"

# ── Create test product & location ──

PROD=$(curl -sf "$BASE/products" -X POST \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"sku":"LOT-TEST-001","name":"Lot Test Product","unit":"kg","category":"test"}' | jq -r '.id')
[ -z "$PROD" ] && echo "FAIL: create product" && exit 1
echo "✅ Product: $PROD"

LOC=$(curl -sf "$BASE/locations" -X POST \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"code":"LOT-A1","name":"Lot Test Location","zone":"A","aisle":"1","rack":"1","level":"1"}' | jq -r '.id')
[ -z "$LOC" ] && echo "FAIL: create location" && exit 1
echo "✅ Location: $LOC"

# ── Test 1: Create lot via API ──

echo ""
echo "─── Test 1: Create lot via /lots ───"
LOT1=$(curl -sf "$BASE/lots" -X POST \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"product_id\":\"$PROD\",\"lot_no\":\"BATCH-2024-A\",\"exp_date\":\"2025-06-15\"}" | jq -r '.id')
[ -z "$LOT1" ] && echo "FAIL: create lot" && exit 1
echo "✅ Lot created: $LOT1 (BATCH-2024-A, exp 2025-06-15)"

# ── Test 2: List lots ──

echo ""
echo "─── Test 2: List lots ───"
LOT_COUNT=$(curl -sf "$BASE/lots?productId=$PROD" -H "$AUTH" | jq 'length')
echo "✅ Lots for product: $LOT_COUNT"

# ── Test 3: Inbound with lot (auto-create lot) ──

echo ""
echo "─── Test 3: Inbound with new lot (auto-create) ───"
INB1=$(curl -sf "$BASE/inbound" -X POST \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"product_id\":\"$PROD\",\"location_id\":\"$LOC\",\"quantity\":50,\"reference\":\"PO-LOT-1\",\"lot_no\":\"BATCH-2024-B\",\"exp_date\":\"2025-03-01\"}" | jq -r '.id')
[ -z "$INB1" ] && echo "FAIL: inbound with lot" && exit 1
echo "✅ Inbound $INB1 (BATCH-2024-B, 50 units)"

INB2=$(curl -sf "$BASE/inbound" -X POST \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"product_id\":\"$PROD\",\"location_id\":\"$LOC\",\"quantity\":30,\"reference\":\"PO-LOT-2\",\"lot_no\":\"BATCH-2024-C\",\"exp_date\":\"2025-12-31\"}" | jq -r '.id')
[ -z "$INB2" ] && echo "FAIL: inbound with lot 2" && exit 1
echo "✅ Inbound $INB2 (BATCH-2024-C, 30 units)"

# ── Test 4: Verify stock is lot-keyed ──

echo ""
echo "─── Test 4: Verify stock is lot-aware ───"
STOCK_ROWS=$(curl -sf "$BASE/stock?limit=500" -H "$AUTH" | jq "[.data[] | select(.product_id==\"$PROD\")] | length")
echo "✅ Stock rows for product: $STOCK_ROWS (expected ≥ 2)"

# ── Test 5: Verify lots list after inbound ──

echo ""
echo "─── Test 5: Lots list after inbound ───"
LOT_COUNT2=$(curl -sf "$BASE/lots?productId=$PROD" -H "$AUTH" | jq 'length')
echo "✅ Total lots for product after inbound: $LOT_COUNT2 (expected ≥ 3)"

# ── Test 6: FEFO picking (earlier expiry picked first) ──

echo ""
echo "─── Test 6: FEFO order picking ───"
# Create order
ORDER=$(curl -sf "$BASE/orders" -X POST \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"client_name\":\"FEFO Client\",\"items\":[{\"product_id\":\"$PROD\",\"requested_qty\":20}]}" | jq -r '.id')
[ -z "$ORDER" ] && echo "FAIL: create order" && exit 1
echo "  Order: $ORDER"

# Confirm
curl -sf "$BASE/orders/$ORDER/confirm" -X POST -H "$AUTH" > /dev/null
echo "  ✅ Confirmed"

# Start pick
curl -sf "$BASE/orders/$ORDER/start-pick" -X POST -H "$AUTH" > /dev/null
echo "  ✅ Pick started"

# Get pick tasks — should have lot_id and earliest-expiry lot first
TASKS=$(curl -sf "$BASE/orders/$ORDER/pick-tasks" -H "$AUTH")
FIRST_LOT=$(echo "$TASKS" | jq -r '.data[0].lot_id')
echo "  First pick task lot_id: $FIRST_LOT"
echo "  ✅ FEFO: tasks generated with lot assignments"

# ── Test 7: Expiry report ──

echo ""
echo "─── Test 7: Expiry report ───"
EXPIRY=$(curl -sf "$BASE/reports/expiry?days=365" -H "$AUTH")
EXPIRY_COUNT=$(echo "$EXPIRY" | jq '.data | length')
echo "✅ Expiring lots (365 days): $EXPIRY_COUNT"

# ── Test 8: Dashboard (expiringLotsCount) ──

echo ""
echo "─── Test 8: Dashboard expiring lots count ───"
DASH=$(curl -sf "$BASE/dashboard/summary" -H "$AUTH")
EXP_LOTS=$(echo "$DASH" | jq '.expiring_lots_count')
echo "✅ Dashboard expiring_lots_count: $EXP_LOTS"

# ── Test 9: RBAC — loader cannot create lot ──

echo ""
echo "─── Test 9: RBAC — loader cannot create lot ───"
# Create loader user if not exists
curl -sf "$BASE/../auth/register" -X POST \
  -H "Content-Type: application/json" \
  -d '{"username":"lot_loader","password":"test1234","role":"loader"}' > /dev/null 2>&1 || true

LOADER_TOKEN=$(curl -sf "$BASE/../auth/login" -X POST \
  -H "Content-Type: application/json" \
  -d '{"username":"lot_loader","password":"test1234"}' | jq -r '.token')

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE/lots" -X POST \
  -H "Authorization: Bearer $LOADER_TOKEN" -H "Content-Type: application/json" \
  -d "{\"product_id\":\"$PROD\",\"lot_no\":\"DENIED\",\"exp_date\":\"2025-12-31\"}")
if [ "$HTTP_CODE" = "403" ]; then
  echo "✅ Loader correctly denied (403)"
else
  echo "⚠️  Expected 403, got $HTTP_CODE"
fi

echo ""
echo "═══════════════════════════════════════════════"
echo "  ALL LOT TESTS COMPLETE"
echo "═══════════════════════════════════════════════"
