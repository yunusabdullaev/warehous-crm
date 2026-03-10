#!/usr/bin/env bash
# test-returns.sh — Smoke-tests for the Returns (RMA) module
# Usage: BASE=http://localhost:4000/api TOKEN=<jwt> bash scripts/test-returns.sh
set -euo pipefail

BASE="${BASE:-http://localhost:4000/api}"
TOKEN="${TOKEN:-}"
PASS=0; FAIL=0

c() { curl -sf -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" "$@"; }

ok() { echo "  ✅ $1"; PASS=$((PASS+1)); }
fail() { echo "  ❌ $1"; FAIL=$((FAIL+1)); }

# ──────────────────────────────────────────────
echo "═══ 1. Prerequisites: Get a SHIPPED order ═══"

ORDER_JSON=$(c "$BASE/orders?status=SHIPPED&limit=1")
ORDER_ID=$(echo "$ORDER_JSON" | jq -r '.data[0].id // empty')
ORDER_NO=$(echo "$ORDER_JSON" | jq -r '.data[0].order_no // empty')

if [ -z "$ORDER_ID" ]; then
  echo "  ⚠️  No SHIPPED order found. Creating a dummy order flow…"
  
  # Get a product
  PROD_JSON=$(c "$BASE/products?limit=1")
  PROD_ID=$(echo "$PROD_JSON" | jq -r '.data[0].id')
  
  if [ -z "$PROD_ID" ] || [ "$PROD_ID" = "null" ]; then
    echo "  ❌ No products found. Create products first."
    exit 1
  fi

  # Create order
  ORD=$(c -X POST "$BASE/orders" -d "{\"client_name\":\"RMA Test Client\",\"items\":[{\"product_id\":\"$PROD_ID\",\"requested_qty\":5}]}")
  ORDER_ID=$(echo "$ORD" | jq -r '.id')
  ORDER_NO=$(echo "$ORD" | jq -r '.order_no')
  echo "  Created order $ORDER_NO ($ORDER_ID)"

  # Confirm → Picking → Ship
  c -X POST "$BASE/orders/$ORDER_ID/confirm" > /dev/null && echo "  Confirmed"
  sleep 1 # let pick tasks generate
  c -X POST "$BASE/orders/$ORDER_ID/ship" > /dev/null 2>&1 && echo "  Shipped" || echo "  ⚠️  Ship may require picking first"
fi

echo "  Order: $ORDER_NO ($ORDER_ID)"

# ──────────────────────────────────────────────
echo ""
echo "═══ 2. Create Return ═══"

RMA_JSON=$(c -X POST "$BASE/returns" -d "{\"order_id\":\"$ORDER_ID\",\"notes\":\"Test return\"}")
RMA_ID=$(echo "$RMA_JSON" | jq -r '.id')
RMA_NO=$(echo "$RMA_JSON" | jq -r '.rma_no')

if [ -n "$RMA_ID" ] && [ "$RMA_ID" != "null" ]; then
  ok "Created RMA $RMA_NO"
else
  fail "Create RMA failed: $RMA_JSON"
fi

# ──────────────────────────────────────────────
echo ""
echo "═══ 3. List Returns ═══"

LIST=$(c "$BASE/returns?status=OPEN")
COUNT=$(echo "$LIST" | jq '.total')
if [ "$COUNT" -ge 1 ]; then
  ok "Listed returns: total=$COUNT"
else
  fail "Expected at least 1 return"
fi

# ──────────────────────────────────────────────
echo ""
echo "═══ 4. Get Return by ID ═══"

DETAIL=$(c "$BASE/returns/$RMA_ID")
GOT_RMA=$(echo "$DETAIL" | jq -r '.return.rma_no')
if [ "$GOT_RMA" = "$RMA_NO" ]; then
  ok "Get by ID: $GOT_RMA"
else
  fail "Get by ID returned wrong RMA: $GOT_RMA"
fi

# ──────────────────────────────────────────────
echo ""
echo "═══ 5. Add Return Items ═══"

# Get product from order
FIRST_PROD=$(echo "$ORDER_JSON" | jq -r '.data[0].items[0].product_id // empty')
if [ -z "$FIRST_PROD" ] || [ "$FIRST_PROD" = "null" ]; then
  PROD_JSON=$(c "$BASE/products?limit=1")
  FIRST_PROD=$(echo "$PROD_JSON" | jq -r '.data[0].id')
fi

# Get a location for RESTOCK
LOC_JSON=$(c "$BASE/locations?limit=1")
LOC_ID=$(echo "$LOC_JSON" | jq -r '.data[0].id')

# 5a. RESTOCK item
ITEM1=$(c -X POST "$BASE/returns/$RMA_ID/items" -d "{\"product_id\":\"$FIRST_PROD\",\"qty\":1,\"disposition\":\"RESTOCK\",\"location_id\":\"$LOC_ID\",\"note\":\"Good condition\"}" 2>&1) 
if echo "$ITEM1" | jq -r '.id' | grep -q '^[a-f0-9]'; then
  ok "Added RESTOCK item"
else
  fail "RESTOCK item failed: $ITEM1"
fi

# 5b. DAMAGED item
ITEM2=$(c -X POST "$BASE/returns/$RMA_ID/items" -d "{\"product_id\":\"$FIRST_PROD\",\"qty\":1,\"disposition\":\"DAMAGED\",\"note\":\"Box crushed\"}" 2>&1)
if echo "$ITEM2" | jq -r '.id' | grep -q '^[a-f0-9]'; then
  ok "Added DAMAGED item"
else
  fail "DAMAGED item failed: $ITEM2"
fi

# 5c. QC_HOLD item
ITEM3=$(c -X POST "$BASE/returns/$RMA_ID/items" -d "{\"product_id\":\"$FIRST_PROD\",\"qty\":1,\"disposition\":\"QC_HOLD\",\"note\":\"Needs inspection\"}" 2>&1)
if echo "$ITEM3" | jq -r '.id' | grep -q '^[a-f0-9]'; then
  ok "Added QC_HOLD item"
else
  fail "QC_HOLD item failed: $ITEM3"
fi

# ──────────────────────────────────────────────
echo ""
echo "═══ 6. Qty Guard — Over-Return ═══"

OVER=$(c -X POST "$BASE/returns/$RMA_ID/items" -d "{\"product_id\":\"$FIRST_PROD\",\"qty\":999,\"disposition\":\"DAMAGED\"}" 2>&1 || true)
if echo "$OVER" | grep -qi "exceeds\|error"; then
  ok "Qty guard blocked over-return"
else
  fail "Qty guard did not block: $OVER"
fi

# ──────────────────────────────────────────────
echo ""
echo "═══ 7. Receive Return ═══"

RCV=$(c -X POST "$BASE/returns/$RMA_ID/receive")
RCV_STATUS=$(echo "$RCV" | jq -r '.status')
if [ "$RCV_STATUS" = "RECEIVED" ]; then
  ok "Return received"
else
  fail "Receive failed: $RCV"
fi

# ──────────────────────────────────────────────
echo ""
echo "═══ 8. Cancel (should fail — already received) ═══"

CANCEL=$(c -X POST "$BASE/returns/$RMA_ID/cancel" 2>&1 || true)
if echo "$CANCEL" | grep -qi "error\|not in OPEN"; then
  ok "Cancel blocked on non-OPEN return"
else
  fail "Cancel did not block: $CANCEL"
fi

# ──────────────────────────────────────────────
echo ""
echo "═══ 9. PDF Note ═══"

PDF_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $TOKEN" "$BASE/returns/$RMA_ID/note.pdf")
if [ "$PDF_STATUS" = "200" ]; then
  ok "PDF returned 200"
else
  fail "PDF returned $PDF_STATUS"
fi

# ──────────────────────────────────────────────
echo ""
echo "═══ 10. Cancel Flow (new RMA) ═══"

RMA2=$(c -X POST "$BASE/returns" -d "{\"order_id\":\"$ORDER_ID\"}" 2>&1 || true)
RMA2_ID=$(echo "$RMA2" | jq -r '.id // empty')
if [ -n "$RMA2_ID" ] && [ "$RMA2_ID" != "null" ]; then
  CANCEL2=$(c -X POST "$BASE/returns/$RMA2_ID/cancel")
  C2_STATUS=$(echo "$CANCEL2" | jq -r '.status')
  if [ "$C2_STATUS" = "CANCELLED" ]; then
    ok "Cancel flow works"
  else
    fail "Cancel flow failed: $CANCEL2"
  fi
else
  fail "Could not create 2nd RMA to test cancel: $RMA2"
fi

# ──────────────────────────────────────────────
echo ""
echo "═══ 11. Dashboard Counters ═══"

DASH=$(c "$BASE/dashboard/summary")
OPEN_RETURNS=$(echo "$DASH" | jq '.open_returns_count')
if [ "$OPEN_RETURNS" != "null" ]; then
  ok "Dashboard has open_returns_count=$OPEN_RETURNS"
else
  fail "Dashboard missing open_returns_count"
fi

# ──────────────────────────────────────────────
echo ""
echo "═══ 12. Returns Report ═══"

REPORT=$(c "$BASE/reports/returns?groupBy=day")
REPORT_GB=$(echo "$REPORT" | jq -r '.group_by')
if [ "$REPORT_GB" = "day" ]; then
  ok "Returns report works"
else
  fail "Returns report failed: $REPORT"
fi

# ──────────────────────────────────────────────
echo ""
echo "═══ 13. RBAC — Loader Can View ═══"
echo "  ⏭️  Manual: Log in as loader → verify /returns visible and /returns/scan works"
echo "  ⏭️  Manual: Verify loader cannot Create/Receive/Cancel (admin/operator only)"

# ──────────────────────────────────────────────
echo ""
echo "══════════════════════════════════════"
echo "  Results: $PASS passed, $FAIL failed"
echo "══════════════════════════════════════"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
