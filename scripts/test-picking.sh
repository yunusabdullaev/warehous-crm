#!/usr/bin/env bash
set -euo pipefail

BASE="${1:?Usage: $0 <base_url>}"
API="$BASE/api/v1"

pass=0; fail=0
check() {
  local label="$1" expected="$2" actual="$3"
  if [ "$actual" = "$expected" ]; then
    echo "  ✅ $label (got $actual)"
    ((pass++))
  else
    echo "  ❌ $label (expected $expected, got $actual)"
    ((fail++))
  fi
}

echo "=== Picking Workflow Tests ==="

# 1. Login as admin
echo "── 1. Login"
TOKEN=$(curl -s "$API/auth/login" -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
AUTH="Authorization: Bearer $TOKEN"

# 2. Create product
echo "── 2. Create product"
PROD=$(curl -s -X POST "$API/products" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"sku":"PICK-TEST-'$RANDOM'","name":"Pick Test Product","unit":"pcs","category":"test"}')
PROD_ID=$(echo "$PROD" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
echo "  Product: $PROD_ID"

# 3. Create 2 locations
echo "── 3. Create 2 locations"
LOC1=$(curl -s -X POST "$API/locations" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"code":"PT-L1-'$RANDOM'","name":"Pick Loc 1","zone":"A","aisle":"1","rack":"R1","level":"S1"}')
LOC1_ID=$(echo "$LOC1" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')

LOC2=$(curl -s -X POST "$API/locations" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"code":"PT-L2-'$RANDOM'","name":"Pick Loc 2","zone":"B","aisle":"2","rack":"R2","level":"S2"}')
LOC2_ID=$(echo "$LOC2" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
echo "  Locations: $LOC1_ID, $LOC2_ID"

# 4. Inbound stock to BOTH locations
echo "── 4. Inbound stock to both locations"
curl -s -X POST "$API/inbound" -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"product_id\":\"$PROD_ID\",\"location_id\":\"$LOC1_ID\",\"quantity\":150,\"reference\":\"pick-test\"}" > /dev/null
curl -s -X POST "$API/inbound" -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"product_id\":\"$PROD_ID\",\"location_id\":\"$LOC2_ID\",\"quantity\":100,\"reference\":\"pick-test\"}" > /dev/null
echo "  Inbound: 150 to Loc1, 100 to Loc2"

# 5. Create order requiring split pick (200 units)
echo "── 5. Create order (200 qty, needs split across locations)"
ORDER=$(curl -s -X POST "$API/orders" -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"client_name\":\"Pick Test Client\",\"items\":[{\"product_id\":\"$PROD_ID\",\"requested_qty\":200}]}")
ORDER_ID=$(echo "$ORDER" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
echo "  Order: $ORDER_ID"

# 6. Confirm order (creates reservations)
echo "── 6. Confirm → reserves stock"
CONFIRM_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/orders/$ORDER_ID/confirm" -H "$AUTH")
check "confirm order" "200" "$CONFIRM_STATUS"

# 7. Start pick → generates pick tasks
echo "── 7. Start pick → pick tasks generated"
START_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/orders/$ORDER_ID/start-pick" -H "$AUTH")
check "start-pick" "200" "$START_STATUS"

# 8. Fetch pick tasks
echo "── 8. Fetch pick tasks"
TASKS=$(curl -s "$API/orders/$ORDER_ID/pick-tasks" -H "$AUTH")
TASK_COUNT=$(echo "$TASKS" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d.get('data',[])))" 2>/dev/null || echo "0")
check "pick tasks created (>1 = split)" "2" "$TASK_COUNT"

# Get task IDs  
TASK1_ID=$(echo "$TASKS" | python3 -c "import sys,json; print(json.load(sys.stdin)['data'][0]['id'])" 2>/dev/null)
TASK2_ID=$(echo "$TASKS" | python3 -c "import sys,json; print(json.load(sys.stdin)['data'][1]['id'])" 2>/dev/null)
T1_LOC=$(echo "$TASKS" | python3 -c "import sys,json; print(json.load(sys.stdin)['data'][0]['location_id'])" 2>/dev/null)
T2_LOC=$(echo "$TASKS" | python3 -c "import sys,json; print(json.load(sys.stdin)['data'][1]['location_id'])" 2>/dev/null)
T1_PLANNED=$(echo "$TASKS" | python3 -c "import sys,json; print(json.load(sys.stdin)['data'][0]['planned_qty'])" 2>/dev/null)
T2_PLANNED=$(echo "$TASKS" | python3 -c "import sys,json; print(json.load(sys.stdin)['data'][1]['planned_qty'])" 2>/dev/null)
echo "  Task 1: $TASK1_ID (loc=$T1_LOC, planned=$T1_PLANNED)"
echo "  Task 2: $TASK2_ID (loc=$T2_LOC, planned=$T2_PLANNED)"

# 9. Attempt ship before picks → 400
echo "── 9. Ship before picks complete → 400"
SHIP_EARLY=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/orders/$ORDER_ID/ship" -H "$AUTH")
check "ship blocked (not all done)" "400" "$SHIP_EARLY"

# 10. Scan partial pick on task 1
echo "── 10. Partial scan on task 1 → IN_PROGRESS"
SCAN1=$(curl -s -w "\n%{http_code}" -X POST "$API/pick-tasks/$TASK1_ID/scan" -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"location_id\":\"$T1_LOC\",\"product_id\":\"$PROD_ID\",\"qty\":50}")
SCAN1_STATUS=$(echo "$SCAN1" | tail -1)
SCAN1_BODY=$(echo "$SCAN1" | head -1)
check "partial scan status" "200" "$SCAN1_STATUS"
SCAN1_TASK_STATUS=$(echo "$SCAN1_BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status',''))" 2>/dev/null)
check "task status after partial" "IN_PROGRESS" "$SCAN1_TASK_STATUS"

# 11. Scan remaining on task 1 → DONE
echo "── 11. Scan remaining on task 1 → DONE"
T1_REMAINING=$((T1_PLANNED - 50))
SCAN2=$(curl -s -w "\n%{http_code}" -X POST "$API/pick-tasks/$TASK1_ID/scan" -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"location_id\":\"$T1_LOC\",\"product_id\":\"$PROD_ID\",\"qty\":$T1_REMAINING}")
SCAN2_STATUS=$(echo "$SCAN2" | tail -1)
SCAN2_BODY=$(echo "$SCAN2" | head -1)
check "complete scan status" "200" "$SCAN2_STATUS"
SCAN2_TASK_STATUS=$(echo "$SCAN2_BODY" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status',''))" 2>/dev/null)
check "task 1 DONE" "DONE" "$SCAN2_TASK_STATUS"

# 12. Still can't ship (task 2 not done)
echo "── 12. Ship still blocked (task 2 open)"
SHIP_STILL=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/orders/$ORDER_ID/ship" -H "$AUTH")
check "ship still blocked" "400" "$SHIP_STILL"

# 13. Complete task 2
echo "── 13. Scan full task 2 → DONE"
SCAN3=$(curl -s -w "\n%{http_code}" -X POST "$API/pick-tasks/$TASK2_ID/scan" -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"location_id\":\"$T2_LOC\",\"product_id\":\"$PROD_ID\",\"qty\":$T2_PLANNED}")
SCAN3_STATUS=$(echo "$SCAN3" | tail -1)
check "task 2 scan" "200" "$SCAN3_STATUS"

# 14. Ship after all done → 200
echo "── 14. Ship after all picked → 200"
SHIP_OK=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/orders/$ORDER_ID/ship" -H "$AUTH")
check "ship success" "200" "$SHIP_OK"

# 15. Verify order is shipped
ORDER_FINAL=$(curl -s "$API/orders/$ORDER_ID" -H "$AUTH")
FINAL_STATUS=$(echo "$ORDER_FINAL" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status',''))" 2>/dev/null)
check "order shipped" "SHIPPED" "$FINAL_STATUS"

# 16. Wrong location scan → 400
echo "── 16. Wrong location scan → 400"
# Create another order to test wrong scan
ORDER2=$(curl -s -X POST "$API/orders" -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"client_name\":\"Wrong Scan Client\",\"items\":[{\"product_id\":\"$PROD_ID\",\"requested_qty\":10}]}")
ORDER2_ID=$(echo "$ORDER2" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
curl -s -o /dev/null -X POST "$API/orders/$ORDER2_ID/confirm" -H "$AUTH"

# Need to inbound more stock since it was consumed
curl -s -X POST "$API/inbound" -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"product_id\":\"$PROD_ID\",\"location_id\":\"$LOC1_ID\",\"quantity\":50,\"reference\":\"pick-test-2\"}" > /dev/null

curl -s -o /dev/null -X POST "$API/orders/$ORDER2_ID/start-pick" -H "$AUTH"
TASKS2=$(curl -s "$API/orders/$ORDER2_ID/pick-tasks" -H "$AUTH")
TASK3_ID=$(echo "$TASKS2" | python3 -c "import sys,json; print(json.load(sys.stdin)['data'][0]['id'])" 2>/dev/null)
T3_LOC=$(echo "$TASKS2" | python3 -c "import sys,json; print(json.load(sys.stdin)['data'][0]['location_id'])" 2>/dev/null)

# Use wrong location
WRONG_LOC="000000000000000000000000"
WRONG_SCAN=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/pick-tasks/$TASK3_ID/scan" -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"location_id\":\"$WRONG_LOC\",\"product_id\":\"$PROD_ID\",\"qty\":5}")
check "wrong location → 400" "400" "$WRONG_SCAN"

# 17. Over-pick → 400
echo "── 17. Over-pick → 400"
OVER_PICK=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/pick-tasks/$TASK3_ID/scan" -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"location_id\":\"$T3_LOC\",\"product_id\":\"$PROD_ID\",\"qty\":999}")
check "over-pick → 400" "400" "$OVER_PICK"

echo ""
echo "═══════════════════════════════"
echo "  PASS: $pass  FAIL: $fail"
echo "═══════════════════════════════"
[ "$fail" -eq 0 ] && exit 0 || exit 1
