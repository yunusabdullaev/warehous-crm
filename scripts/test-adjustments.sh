#!/usr/bin/env bash
set -euo pipefail

# ──────────────────────────────────────────────
# Inventory-Integrity Test Script
# Usage: bash scripts/test-adjustments.sh <base_url>
# Example: bash scripts/test-adjustments.sh http://localhost:3003
# ──────────────────────────────────────────────

BASE="${1:?Usage: $0 <base_url>}"
API="$BASE/api/v1"
PASS=0
FAIL=0

green() { printf "\033[32m✔ %s\033[0m\n" "$1"; PASS=$((PASS+1)); }
red()   { printf "\033[31m✘ %s\033[0m\n" "$1"; FAIL=$((FAIL+1)); }
sep()   { printf "\n── %s ──\n" "$1"; }

# ── 0. Setup: Login & get IDs ───────────────────

sep "Setup"

# Login as admin
ADMIN_TOKEN=$(curl -sf "$API/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

if [ -z "$ADMIN_TOKEN" ] || [ "$ADMIN_TOKEN" = "null" ]; then
  echo "FATAL: Cannot login as admin. Make sure the server is running and the admin user exists."
  exit 1
fi
echo "Admin token obtained."

AUTH="Authorization: Bearer $ADMIN_TOKEN"

# Get first product and location
PRODUCT_ID=$(curl -sf "$API/products?limit=1" -H "$AUTH" | jq -r '.data[0].id')
LOCATION_ID=$(curl -sf "$API/locations?limit=1" -H "$AUTH" | jq -r '.data[0].id')

if [ "$PRODUCT_ID" = "null" ] || [ "$LOCATION_ID" = "null" ]; then
  echo "FATAL: No products or locations found. Create some first."
  exit 1
fi
echo "Product: $PRODUCT_ID  Location: $LOCATION_ID"

# Get current stock
STOCK_BEFORE=$(curl -sf "$API/stock/product/$PRODUCT_ID" -H "$AUTH" | jq '[.[] | .quantity] | add // 0')
echo "Stock before tests: $STOCK_BEFORE"

# ── 1. Create adjustment +10 ───────────────────

sep "Test 1: Create adjustment +10"

ADJ_RESP=$(curl -sf -w "\n%{http_code}" "$API/adjustments" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"product_id\":\"$PRODUCT_ID\",\"location_id\":\"$LOCATION_ID\",\"delta_qty\":10,\"reason\":\"FOUND\",\"note\":\"test +10\"}")

ADJ_CODE=$(echo "$ADJ_RESP" | tail -1)
ADJ_BODY=$(echo "$ADJ_RESP" | head -n -1)

if [ "$ADJ_CODE" = "201" ]; then
  green "Adjustment +10 created (201)"
  ADJ_ID=$(echo "$ADJ_BODY" | jq -r '.id')
else
  red "Expected 201, got $ADJ_CODE"
fi

# Verify stock increased
STOCK_AFTER_1=$(curl -sf "$API/stock/product/$PRODUCT_ID" -H "$AUTH" | jq '[.[] | .quantity] | add // 0')
EXPECTED_1=$((STOCK_BEFORE + 10))
if [ "$STOCK_AFTER_1" -eq "$EXPECTED_1" ]; then
  green "Stock increased to $STOCK_AFTER_1 (expected $EXPECTED_1)"
else
  red "Stock is $STOCK_AFTER_1, expected $EXPECTED_1"
fi

# ── 2. Create adjustment -5 ────────────────────

sep "Test 2: Create adjustment -5"

ADJ2_RESP=$(curl -sf -w "\n%{http_code}" "$API/adjustments" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"product_id\":\"$PRODUCT_ID\",\"location_id\":\"$LOCATION_ID\",\"delta_qty\":-5,\"reason\":\"DAMAGED\",\"note\":\"test -5\"}")

ADJ2_CODE=$(echo "$ADJ2_RESP" | tail -1)

if [ "$ADJ2_CODE" = "201" ]; then
  green "Adjustment -5 created (201)"
else
  red "Expected 201, got $ADJ2_CODE"
fi

STOCK_AFTER_2=$(curl -sf "$API/stock/product/$PRODUCT_ID" -H "$AUTH" | jq '[.[] | .quantity] | add // 0')
EXPECTED_2=$((EXPECTED_1 - 5))
if [ "$STOCK_AFTER_2" -eq "$EXPECTED_2" ]; then
  green "Stock decreased to $STOCK_AFTER_2 (expected $EXPECTED_2)"
else
  red "Stock is $STOCK_AFTER_2, expected $EXPECTED_2"
fi

# ── 3. Reverse inbound ─────────────────────────

sep "Test 3: Reverse inbound"

# Create an inbound first
INB_RESP=$(curl -sf "$API/inbound" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"product_id\":\"$PRODUCT_ID\",\"location_id\":\"$LOCATION_ID\",\"quantity\":20,\"reference\":\"TEST-REV\"}")
INB_ID=$(echo "$INB_RESP" | jq -r '.id')

STOCK_PRE_REV=$(curl -sf "$API/stock/product/$PRODUCT_ID" -H "$AUTH" | jq '[.[] | .quantity] | add // 0')

REV_RESP=$(curl -sf -w "\n%{http_code}" "$API/inbound/$INB_ID/reverse" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"reason":"test reversal"}')

REV_CODE=$(echo "$REV_RESP" | tail -1)
REV_BODY=$(echo "$REV_RESP" | head -n -1)

if [ "$REV_CODE" = "200" ]; then
  STATUS=$(echo "$REV_BODY" | jq -r '.status')
  if [ "$STATUS" = "REVERSED" ]; then
    green "Inbound reversed, status=REVERSED"
  else
    red "Expected status=REVERSED, got $STATUS"
  fi
else
  red "Expected 200, got $REV_CODE"
fi

STOCK_POST_REV=$(curl -sf "$API/stock/product/$PRODUCT_ID" -H "$AUTH" | jq '[.[] | .quantity] | add // 0')
EXPECTED_REV=$((STOCK_PRE_REV - 20))
if [ "$STOCK_POST_REV" -eq "$EXPECTED_REV" ]; then
  green "Stock decreased by 20 after inbound reversal"
else
  red "Stock is $STOCK_POST_REV, expected $EXPECTED_REV"
fi

# ── 4. Reverse outbound ────────────────────────

sep "Test 4: Reverse outbound"

# Create outbound (need enough stock)
OUT_RESP=$(curl -sf "$API/outbound" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"product_id\":\"$PRODUCT_ID\",\"location_id\":\"$LOCATION_ID\",\"quantity\":5,\"reference\":\"TEST-OUT-REV\"}")
OUT_ID=$(echo "$OUT_RESP" | jq -r '.id')

STOCK_PRE_OUT_REV=$(curl -sf "$API/stock/product/$PRODUCT_ID" -H "$AUTH" | jq '[.[] | .quantity] | add // 0')

OREV_RESP=$(curl -sf -w "\n%{http_code}" "$API/outbound/$OUT_ID/reverse" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"reason":"test outbound reversal"}')

OREV_CODE=$(echo "$OREV_RESP" | tail -1)
OREV_BODY=$(echo "$OREV_RESP" | head -n -1)

if [ "$OREV_CODE" = "200" ]; then
  OSTATUS=$(echo "$OREV_BODY" | jq -r '.status')
  if [ "$OSTATUS" = "REVERSED" ]; then
    green "Outbound reversed, status=REVERSED"
  else
    red "Expected status=REVERSED, got $OSTATUS"
  fi
else
  red "Expected 200, got $OREV_CODE"
fi

STOCK_POST_OUT_REV=$(curl -sf "$API/stock/product/$PRODUCT_ID" -H "$AUTH" | jq '[.[] | .quantity] | add // 0')
EXPECTED_OREV=$((STOCK_PRE_OUT_REV + 5))
if [ "$STOCK_POST_OUT_REV" -eq "$EXPECTED_OREV" ]; then
  green "Stock increased by 5 after outbound reversal"
else
  red "Stock is $STOCK_POST_OUT_REV, expected $EXPECTED_OREV"
fi

# ── 5. Operator 24h rule ───────────────────────

sep "Test 5: Operator reversal RBAC"

# Try to register an operator. If it fails (already exists), try logging in.
OP_RESP=$(curl -s -w "\n%{http_code}" "$API/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"testop","password":"testop123","role":"operator"}')
OP_CODE=$(echo "$OP_RESP" | tail -1)

OP_TOKEN=$(curl -sf "$API/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"testop","password":"testop123"}' | jq -r '.token')

OP_AUTH="Authorization: Bearer $OP_TOKEN"

# Operator creates inbound
OP_INB=$(curl -sf "$API/inbound" \
  -H "$OP_AUTH" -H "Content-Type: application/json" \
  -d "{\"product_id\":\"$PRODUCT_ID\",\"location_id\":\"$LOCATION_ID\",\"quantity\":3,\"reference\":\"OP-TEST\"}")
OP_INB_ID=$(echo "$OP_INB" | jq -r '.id')

# Operator reverses own recent → should succeed
OPREV_RESP=$(curl -s -w "\n%{http_code}" "$API/inbound/$OP_INB_ID/reverse" \
  -H "$OP_AUTH" -H "Content-Type: application/json" \
  -d '{"reason":"operator self-reverse"}')
OPREV_CODE=$(echo "$OPREV_RESP" | tail -1)

if [ "$OPREV_CODE" = "200" ]; then
  green "Operator reversed own recent inbound (200)"
else
  red "Expected 200 for operator self-reverse, got $OPREV_CODE"
fi

# Operator tries to reverse admin's inbound (the one from test 3 is already reversed; create a fresh admin one)
ADMIN_INB2=$(curl -sf "$API/inbound" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"product_id\":\"$PRODUCT_ID\",\"location_id\":\"$LOCATION_ID\",\"quantity\":2,\"reference\":\"ADMIN-OWN\"}")
ADMIN_INB2_ID=$(echo "$ADMIN_INB2" | jq -r '.id')

OPREV2_RESP=$(curl -s -w "\n%{http_code}" "$API/inbound/$ADMIN_INB2_ID/reverse" \
  -H "$OP_AUTH" -H "Content-Type: application/json" \
  -d '{"reason":"operator tries admin record"}')
OPREV2_CODE=$(echo "$OPREV2_RESP" | tail -1)

if [ "$OPREV2_CODE" = "403" ]; then
  green "Operator blocked from reversing admin's inbound (403)"
else
  red "Expected 403 for operator on admin's record, got $OPREV2_CODE"
fi

# ── 6. Double-reverse → 409 ───────────────────

sep "Test 6: Double-reverse idempotency"

DOUBLE_RESP=$(curl -s -w "\n%{http_code}" "$API/inbound/$INB_ID/reverse" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"reason":"double try"}')
DOUBLE_CODE=$(echo "$DOUBLE_RESP" | tail -1)

if [ "$DOUBLE_CODE" = "409" ]; then
  green "Double-reverse returned 409 Conflict"
else
  red "Expected 409 for double-reverse, got $DOUBLE_CODE"
fi

# ── 7. History records created ─────────────────

sep "Test 7: History audit trail"

HISTORY_COUNT=$(curl -sf "$API/history?limit=100" -H "$AUTH" | jq '.data | length')

if [ "$HISTORY_COUNT" -gt 0 ]; then
  # Check for adjustment and reversal actions
  ADJ_HIST=$(curl -sf "$API/history?limit=100" -H "$AUTH" | jq '[.data[] | select(.action == "create_adjustment")] | length')
  REV_HIST=$(curl -sf "$API/history?limit=100" -H "$AUTH" | jq '[.data[] | select(.action == "reverse_inbound" or .action == "reverse_outbound")] | length')

  if [ "$ADJ_HIST" -gt 0 ] && [ "$REV_HIST" -gt 0 ]; then
    green "History has adjustment ($ADJ_HIST) and reversal ($REV_HIST) entries"
  else
    red "Missing history entries: adjustments=$ADJ_HIST reversals=$REV_HIST"
  fi
else
  red "No history records found at all"
fi

# ── Summary ────────────────────────────────────

sep "SUMMARY"
echo "Passed: $PASS | Failed: $FAIL"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
echo "All tests passed! ✅"
