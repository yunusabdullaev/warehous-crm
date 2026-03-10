#!/usr/bin/env bash
# ── Telegram Notification Tests ──
# Usage: bash scripts/test-telegram.sh http://localhost:3003
#
# Tests with TELEGRAM_ENABLED=false (no real messages sent).
# For live testing: enable telegram in /settings/notifications and use "Send Test".

set -euo pipefail

BASE="${1:-http://localhost:3003}"
API="$BASE/api/v1"
PASS=0 FAIL=0

green() { printf "\033[32m✓ %s\033[0m\n" "$1"; PASS=$((PASS+1)); }
red()   { printf "\033[31m✗ %s\033[0m\n" "$1"; FAIL=$((FAIL+1)); }
check() { [ "$1" = "$2" ] && green "$3" || red "$3 (expected $2, got $1)"; }

echo "══════════════════════════════════════"
echo " Telegram Notification Tests"
echo "══════════════════════════════════════"

# ── Setup: get admin token ──
ADMIN_RESP=$(curl -s -X POST "$API/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"notify_admin","password":"admin123","role":"admin"}')
ADMIN_TOKEN=$(echo "$ADMIN_RESP" | jq -r '.token // empty')
if [ -z "$ADMIN_TOKEN" ]; then
  ADMIN_RESP=$(curl -s -X POST "$API/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"notify_admin","password":"admin123"}')
  ADMIN_TOKEN=$(echo "$ADMIN_RESP" | jq -r '.token')
fi
AUTH="Authorization: Bearer $ADMIN_TOKEN"

# ── Operator token for RBAC ──
OP_RESP=$(curl -s -X POST "$API/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"notify_oper","password":"oper1234","role":"operator"}')
OP_TOKEN=$(echo "$OP_RESP" | jq -r '.token // empty')
if [ -z "$OP_TOKEN" ]; then
  OP_RESP=$(curl -s -X POST "$API/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"notify_oper","password":"oper1234"}')
  OP_TOKEN=$(echo "$OP_RESP" | jq -r '.token')
fi
OP_AUTH="Authorization: Bearer $OP_TOKEN"

# ── 1. Update notification settings ──
echo ""
echo "── 1. Update Settings ──"
HTTP=$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$API/settings/notifications" \
  -H "Content-Type: application/json" -H "$AUTH" \
  -d '{"telegram_enabled":false,"telegram_bot_token":"test-token-123","telegram_chat_ids":"-100123456"}')
check "$HTTP" "200" "PUT /settings/notifications → 200"

# ── 2. Get settings ──
echo ""
echo "── 2. Get Settings ──"
GET_RESP=$(curl -s "$API/settings/notifications" -H "$AUTH")
HTTP=$(echo "$GET_RESP" | jq -r '.telegram_enabled')
check "$HTTP" "false" "telegram_enabled = false"

CHAT=$(echo "$GET_RESP" | jq -r '.telegram_chat_ids')
check "$CHAT" "-100123456" "chat_ids preserved"

# Token should be masked
TOKEN_MASKED=$(echo "$GET_RESP" | jq -r '.telegram_bot_token')
echo "   Token (masked): $TOKEN_MASKED"
if echo "$TOKEN_MASKED" | grep -q '\*'; then
  green "Token is masked"
else
  red "Token should be masked"
fi

# ── 3. Test with disabled ──
echo ""
echo "── 3. Test Message (disabled) ──"
HTTP=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/settings/notifications/test" -H "$AUTH")
check "$HTTP" "400" "POST /settings/notifications/test (disabled) → 400"

# ── 4. Product with low stock threshold ──
echo ""
echo "── 4. Low Stock Threshold on Product ──"
PROD_RESP=$(curl -s -w '\n%{http_code}' -X POST "$API/products" \
  -H "Content-Type: application/json" -H "$AUTH" \
  -d '{"sku":"NOTIFY-TEST-001","name":"Test Alert Item","unit":"pcs"}')
PROD_CODE=$(echo "$PROD_RESP" | tail -1)
PROD_BODY=$(echo "$PROD_RESP" | head -1)
check "$PROD_CODE" "201" "Create product → 201"

PROD_ID=$(echo "$PROD_BODY" | jq -r '.id')

UPD_RESP=$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$API/products/$PROD_ID" \
  -H "Content-Type: application/json" -H "$AUTH" \
  -d '{"low_stock_threshold":10}')
check "$UPD_RESP" "200" "Set low_stock_threshold=10 → 200"

GET_PROD=$(curl -s "$API/products/$PROD_ID" -H "$AUTH")
THRESHOLD=$(echo "$GET_PROD" | jq -r '.low_stock_threshold')
check "$THRESHOLD" "10" "low_stock_threshold = 10"

# ── 5. RBAC: operator cannot access settings ──
echo ""
echo "── 5. RBAC Check ──"
HTTP=$(curl -s -o /dev/null -w '%{http_code}' "$API/settings/notifications" -H "$OP_AUTH")
check "$HTTP" "403" "GET /settings/notifications (operator) → 403"

HTTP=$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$API/settings/notifications" \
  -H "Content-Type: application/json" -H "$OP_AUTH" \
  -d '{"telegram_enabled":true}')
check "$HTTP" "403" "PUT /settings/notifications (operator) → 403"

HTTP=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/settings/notifications/test" -H "$OP_AUTH")
check "$HTTP" "403" "POST /settings/notifications/test (operator) → 403"

# ── 6. Enable and test order flow logs ──
echo ""
echo "── 6. Order Flow (telegram disabled — log only) ──"
# Create a product + location + inbound to have stock
LOC_RESP=$(curl -s -X POST "$API/locations" \
  -H "Content-Type: application/json" -H "$AUTH" \
  -d '{"code":"TG-A1-01","name":"Test Loc","zone":"A","aisle":"1","rack":"1","level":"1"}')
LOC_ID=$(echo "$LOC_RESP" | jq -r '.id')

# Inbound stock
curl -s -X POST "$API/inbound" \
  -H "Content-Type: application/json" -H "$AUTH" \
  -d "{\"product_id\":\"$PROD_ID\",\"location_id\":\"$LOC_ID\",\"quantity\":50,\"reference\":\"NOTIFY-TEST\"}" > /dev/null

# Create order
ORD_RESP=$(curl -s -X POST "$API/orders" \
  -H "Content-Type: application/json" -H "$AUTH" \
  -d "{\"client_name\":\"TG Test Client\",\"items\":[{\"product_id\":\"$PROD_ID\",\"requested_qty\":5}]}")
ORD_ID=$(echo "$ORD_RESP" | jq -r '.id')

# Confirm
HTTP=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/orders/$ORD_ID/confirm" -H "$AUTH")
check "$HTTP" "200" "Order confirm → 200 (telegram queued in log)"

# Start pick
HTTP=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/orders/$ORD_ID/start-pick" -H "$AUTH")
check "$HTTP" "200" "Order start-pick → 200 (telegram queued in log)"

# Get pick tasks and scan
TASKS=$(curl -s "$API/orders/$ORD_ID/pick-tasks" -H "$AUTH")
TASK_ID=$(echo "$TASKS" | jq -r '.[0].id')
TASK_LOC=$(echo "$TASKS" | jq -r '.[0].location_id')

SCAN_RESP=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/pick-tasks/$TASK_ID/scan" \
  -H "Content-Type: application/json" -H "$AUTH" \
  -d "{\"location_id\":\"$TASK_LOC\",\"product_id\":\"$PROD_ID\",\"qty\":5,\"meta\":{\"scanner\":\"test\",\"client\":\"test\"}}")
check "$SCAN_RESP" "200" "Pick scan → 200 (task DONE notification queued)"

# Ship
HTTP=$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/orders/$ORD_ID/ship" -H "$AUTH")
check "$HTTP" "200" "Order ship → 200 (shipped + low stock check)"

# ── Summary ──
echo ""
echo "══════════════════════════════════════"
echo " Results: $PASS passed, $FAIL failed"
echo "══════════════════════════════════════"
echo ""
echo "── Manual Live Test ──"
echo "1. Go to /settings/notifications in admin UI"
echo "2. Enter your Telegram bot token (from @BotFather)"
echo "3. Enter your chat ID (from @userinfobot)"
echo "4. Toggle 'Enabled' on → Save"
echo "5. Click 'Send Test' — check Telegram"
echo "6. Confirm an order → check Telegram for notification"
echo ""
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
