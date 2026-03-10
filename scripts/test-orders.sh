#!/usr/bin/env bash
set -euo pipefail

BASE="${1:?Usage: $0 <base_url>}"
API="$BASE/api/v1"

pass=0; fail=0
check() {
  local label="$1" expected="$2" actual="$3"
  if [ "$actual" = "$expected" ]; then
    echo "  ✅ $label (HTTP $actual)"
    ((pass++))
  else
    echo "  ❌ $label — expected $expected, got $actual"
    ((fail++))
  fi
}

echo "═══════════════════════════════════════════"
echo "  Order → Picking → Shipment Test Suite"
echo "═══════════════════════════════════════════"

# ── 0. Authenticate ──
echo -e "\n── Step 0: Login ──"
LOGIN=$(curl -s -w "\n%{http_code}" -X POST "$API/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}')
HTTP=$(echo "$LOGIN" | tail -1)
BODY=$(echo "$LOGIN" | head -n -1)
TOKEN=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])" 2>/dev/null || true)
if [ -z "$TOKEN" ]; then
  echo "❌ Login failed (HTTP $HTTP). Make sure admin/admin123 exists."
  exit 1
fi
echo "  ✅ Logged in as admin"
AUTH="Authorization: Bearer $TOKEN"

# ── 1. Seed: create product + location + inbound for stock ──
echo -e "\n── Step 1: Seed data ──"
SKU="TST-ORD-$(date +%s)"
PROD=$(curl -s -X POST "$API/products" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"sku\":\"$SKU\",\"name\":\"Order Test Product\",\"unit\":\"pcs\",\"category\":\"test\"}")
PRODUCT_ID=$(echo "$PROD" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || true)
echo "  Product: $PRODUCT_ID ($SKU)"

LOC=$(curl -s -X POST "$API/locations" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"code\":\"LOC-ORD-$(date +%s)\",\"name\":\"Order Test Loc\",\"zone\":\"TEST\",\"aisle\":\"1\",\"rack\":\"A\",\"level\":\"1\"}")
LOCATION_ID=$(echo "$LOC" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || true)
echo "  Location: $LOCATION_ID"

# Add 1000 units of stock
curl -s -o /dev/null -X POST "$API/inbound" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"product_id\":\"$PRODUCT_ID\",\"location_id\":\"$LOCATION_ID\",\"quantity\":1000,\"reference\":\"SEED\"}"
echo "  Inbound: 1000 units"

# ── 2. Create order ──
echo -e "\n── Test 1: Create order ──"
CREATE=$(curl -s -w "\n%{http_code}" -X POST "$API/orders" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"client_name\":\"Test Client\",\"notes\":\"Automated test\",\"items\":[{\"product_id\":\"$PRODUCT_ID\",\"requested_qty\":100}]}")
HTTP=$(echo "$CREATE" | tail -1)
BODY=$(echo "$CREATE" | head -n -1)
check "Create order" "201" "$HTTP"
ORDER_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || true)
ORDER_NO=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['order_no'])" 2>/dev/null || true)
echo "  Order: $ORDER_NO ($ORDER_ID)"

# ── 3. Confirm order → creates reservations ──
echo -e "\n── Test 2: Confirm order ──"
CONFIRM=$(curl -s -w "\n%{http_code}" -X POST "$API/orders/$ORDER_ID/confirm" -H "$AUTH")
HTTP=$(echo "$CONFIRM" | tail -1)
BODY=$(echo "$CONFIRM" | head -n -1)
check "Confirm order" "200" "$HTTP"
STATUS=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null || true)
check "Status is CONFIRMED" "CONFIRMED" "$STATUS"

# Check reservations created
RESERVATIONS=$(curl -s "$API/reservations?orderId=$ORDER_ID" -H "$AUTH")
RES_COUNT=$(echo "$RESERVATIONS" | python3 -c "import sys,json; d=json.load(sys.stdin)['data']; print(len(d) if d else 0)" 2>/dev/null || true)
check "Reservations created" "1" "$RES_COUNT"

# ── 4. Double confirm → 409 ──
echo -e "\n── Test 3: Double confirm → 409 ──"
RECONFIRM=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/orders/$ORDER_ID/confirm" -H "$AUTH")
check "Re-confirm returns 409" "409" "$RECONFIRM"

# ── 5. Start pick → PICKING ──
echo -e "\n── Test 4: Start picking ──"
PICK=$(curl -s -w "\n%{http_code}" -X POST "$API/orders/$ORDER_ID/start-pick" -H "$AUTH")
HTTP=$(echo "$PICK" | tail -1)
BODY=$(echo "$PICK" | head -n -1)
check "Start pick" "200" "$HTTP"
STATUS=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null || true)
check "Status is PICKING" "PICKING" "$STATUS"

# ── 6. Ship → SHIPPED, outbound created, reservations released ──
echo -e "\n── Test 5: Ship order ──"
SHIP=$(curl -s -w "\n%{http_code}" -X POST "$API/orders/$ORDER_ID/ship" -H "$AUTH")
HTTP=$(echo "$SHIP" | tail -1)
BODY=$(echo "$SHIP" | head -n -1)
check "Ship order" "200" "$HTTP"
STATUS=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['status'])" 2>/dev/null || true)
check "Status is SHIPPED" "SHIPPED" "$STATUS"
SHIPPED_QTY=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['items'][0]['shipped_qty'])" 2>/dev/null || true)
check "Shipped qty = 100" "100" "$SHIPPED_QTY"

# Check reservations released
RESERVATIONS2=$(curl -s "$API/reservations?orderId=$ORDER_ID&status=RELEASED" -H "$AUTH")
REL_COUNT=$(echo "$RESERVATIONS2" | python3 -c "import sys,json; d=json.load(sys.stdin)['data']; print(len(d) if d else 0)" 2>/dev/null || true)
check "Reservations released" "1" "$REL_COUNT"

# ── 7. Cancel after shipped → 409 ──
echo -e "\n── Test 6: Cancel shipped → 409 ──"
CANCEL=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/orders/$ORDER_ID/cancel" -H "$AUTH")
check "Cancel shipped order returns 409" "409" "$CANCEL"

# ── 8. Reserve more than available → 400 ──
echo -e "\n── Test 7: Over-reserve → 400 ──"
OVER=$(curl -s -w "\n%{http_code}" -X POST "$API/orders" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"client_name\":\"Over Client\",\"items\":[{\"product_id\":\"$PRODUCT_ID\",\"requested_qty\":999999}]}")
HTTP=$(echo "$OVER" | tail -1)
BODY=$(echo "$OVER" | head -n -1)
OVER_ID=$(echo "$BODY" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null || true)
# Confirm it → should fail with insufficient stock
if [ -n "$OVER_ID" ]; then
  OVER_CONFIRM=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/orders/$OVER_ID/confirm" -H "$AUTH")
  check "Over-reserve returns 400" "400" "$OVER_CONFIRM"
else
  echo "  ❌ Failed to create over-reserve order"
  ((fail++))
fi

# ── 9. History entries ──
echo -e "\n── Test 8: History audit ──"
HISTORY=$(curl -s "$API/history?limit=50" -H "$AUTH")
ORDER_HIST=$(echo "$HISTORY" | python3 -c "
import sys, json
data = json.load(sys.stdin).get('data', [])
actions = [h['action'] for h in data if h.get('entity_type') == 'order']
print(','.join(sorted(set(actions))))
" 2>/dev/null || true)
echo "  Order history actions: $ORDER_HIST"
# Check for at least order_created and order_confirmed
echo "$ORDER_HIST" | grep -q "order_created" && echo "  ✅ order_created found" && ((pass++)) || { echo "  ❌ order_created missing"; ((fail++)); }
echo "$ORDER_HIST" | grep -q "order_confirmed" && echo "  ✅ order_confirmed found" && ((pass++)) || { echo "  ❌ order_confirmed missing"; ((fail++)); }
echo "$ORDER_HIST" | grep -q "order_shipped" && echo "  ✅ order_shipped found" && ((pass++)) || { echo "  ❌ order_shipped missing"; ((fail++)); }

echo -e "\n═══════════════════════════════════════════"
echo "  Results: $pass passed, $fail failed"
echo "═══════════════════════════════════════════"
[ $fail -eq 0 ] && exit 0 || exit 1
