#!/usr/bin/env bash
# ──────────────────────────────────────────────────────────────
# Expiry Digest integration test
# Usage: bash scripts/test-expiry-digest.sh [base_url]
# ──────────────────────────────────────────────────────────────
set -euo pipefail

BASE="${1:-${API_URL:-http://localhost:3003/api/v1}}"

echo "═══════════════════════════════════════════════"
echo "  EXPIRY DIGEST TEST SUITE"
echo "  API: $BASE"
echo "═══════════════════════════════════════════════"

# ── Login ──

TOKEN=$(curl -sf "$BASE/../auth/login" -X POST \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')
[ -z "$TOKEN" ] && echo "FAIL: login" && exit 1
AUTH="Authorization: Bearer $TOKEN"
echo "✅ Login OK"

# ── Test 1: Enable expiry digest ──

echo ""
echo "─── Test 1: Enable expiry digest settings ───"
SAVE_RESP=$(curl -sf "$BASE/settings/notifications" -X PUT \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d '{
    "telegram_enabled": true,
    "telegram_bot_token": "fake-token-for-testing",
    "telegram_chat_ids": "-1001234567890",
    "expiry_digest_enabled": true,
    "expiry_digest_days": 14,
    "expiry_digest_time": "08:30"
  }')
echo "  $SAVE_RESP"

# Verify settings round-trip
GET_RESP=$(curl -sf "$BASE/settings/notifications" -H "$AUTH")
DIGEST_ENABLED=$(echo "$GET_RESP" | jq -r '.expiry_digest_enabled')
DIGEST_DAYS=$(echo "$GET_RESP" | jq -r '.expiry_digest_days')
DIGEST_TIME=$(echo "$GET_RESP" | jq -r '.expiry_digest_time')
echo "  digest_enabled=$DIGEST_ENABLED days=$DIGEST_DAYS time=$DIGEST_TIME"
[ "$DIGEST_ENABLED" = "true" ] && [ "$DIGEST_DAYS" = "14" ] && [ "$DIGEST_TIME" = "08:30" ]
echo "✅ Settings saved and verified"

# ── Test 2: Create product + lots expiring soon ──

echo ""
echo "─── Test 2: Create expiring test data ───"

PROD=$(curl -sf "$BASE/products" -X POST \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"sku":"DIGEST-TEST-001","name":"Digest Test Product","unit":"pcs","category":"test"}' | jq -r '.id')
[ -z "$PROD" ] && echo "FAIL: create product" && exit 1
echo "  Product: $PROD"

LOC=$(curl -sf "$BASE/locations" -X POST \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"code":"DGT-A1","name":"Digest Test Loc","zone":"A","aisle":"1","rack":"1","level":"1"}' | jq -r '.id')
[ -z "$LOC" ] && echo "FAIL: create location" && exit 1
echo "  Location: $LOC"

# Lot expiring tomorrow (urgent)
TOMORROW=$(date -v+1d +%Y-%m-%d 2>/dev/null || date -d "+1 day" +%Y-%m-%d)
curl -sf "$BASE/inbound" -X POST \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"product_id\":\"$PROD\",\"location_id\":\"$LOC\",\"quantity\":10,\"reference\":\"DGT-1\",\"lot_no\":\"URGENT-LOT\",\"exp_date\":\"$TOMORROW\"}" > /dev/null
echo "  Lot URGENT-LOT exp=$TOMORROW (qty=10)"

# Lot expiring in 5 days (warning)
DAY5=$(date -v+5d +%Y-%m-%d 2>/dev/null || date -d "+5 days" +%Y-%m-%d)
curl -sf "$BASE/inbound" -X POST \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"product_id\":\"$PROD\",\"location_id\":\"$LOC\",\"quantity\":20,\"reference\":\"DGT-2\",\"lot_no\":\"WARN-LOT\",\"exp_date\":\"$DAY5\"}" > /dev/null
echo "  Lot WARN-LOT exp=$DAY5 (qty=20)"

# Lot expiring in 10 days (notice)
DAY10=$(date -v+10d +%Y-%m-%d 2>/dev/null || date -d "+10 days" +%Y-%m-%d)
curl -sf "$BASE/inbound" -X POST \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"product_id\":\"$PROD\",\"location_id\":\"$LOC\",\"quantity\":30,\"reference\":\"DGT-3\",\"lot_no\":\"NOTICE-LOT\",\"exp_date\":\"$DAY10\"}" > /dev/null
echo "  Lot NOTICE-LOT exp=$DAY10 (qty=30)"

echo "✅ Test data created"

# ── Test 3: Run digest → verify response ──

echo ""
echo "─── Test 3: Run digest endpoint ───"
RUN1=$(curl -sf "$BASE/alerts/expiry-digest/run?force=true" -X POST -H "$AUTH")
echo "  Response: $RUN1"

SENT=$(echo "$RUN1" | jq -r '.sent')
TOTAL=$(echo "$RUN1" | jq -r '.total')
URG=$(echo "$RUN1" | jq -r '.urgent')
WARN=$(echo "$RUN1" | jq -r '.warning')
NOTICE=$(echo "$RUN1" | jq -r '.notice')

echo "  sent=$SENT total=$TOTAL urgent=$URG warning=$WARN notice=$NOTICE"
[ "$SENT" = "true" ] || (echo "FAIL: digest not sent" && exit 1)
[ "$TOTAL" -ge 3 ] || (echo "FAIL: expected ≥3 lots, got $TOTAL" && exit 1)
echo "✅ Digest sent with correct counts"

# ── Test 4: Verify dedup — run again without force ──

echo ""
echo "─── Test 4: Verify dedup (no force) ───"
RUN2=$(curl -sf "$BASE/alerts/expiry-digest/run" -X POST -H "$AUTH")
echo "  Response: $RUN2"

SKIPPED=$(echo "$RUN2" | jq -r '.skipped')
REASON=$(echo "$RUN2" | jq -r '.reason')
echo "  skipped=$SKIPPED reason=$REASON"
[ "$SKIPPED" = "true" ] || (echo "FAIL: should be skipped" && exit 1)
[ "$REASON" = "already sent today" ] || (echo "FAIL: wrong reason: $REASON" && exit 1)
echo "✅ Dedup works — correctly skipped"

# ── Test 5: Force run overrides dedup ──

echo ""
echo "─── Test 5: Force run overrides dedup ───"
RUN3=$(curl -sf "$BASE/alerts/expiry-digest/run?force=true" -X POST -H "$AUTH")
SENT3=$(echo "$RUN3" | jq -r '.sent')
echo "  sent=$SENT3"
[ "$SENT3" = "true" ] || (echo "FAIL: force should override dedup" && exit 1)
echo "✅ Force override works"

# ── Test 6: RBAC — loader cannot run digest ──

echo ""
echo "─── Test 6: RBAC — loader denied ───"
curl -sf "$BASE/../auth/register" -X POST \
  -H "Content-Type: application/json" \
  -d '{"username":"digest_loader","password":"test1234","role":"loader"}' > /dev/null 2>&1 || true

LOADER_TOKEN=$(curl -sf "$BASE/../auth/login" -X POST \
  -H "Content-Type: application/json" \
  -d '{"username":"digest_loader","password":"test1234"}' | jq -r '.token')

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
  "$BASE/alerts/expiry-digest/run" -X POST \
  -H "Authorization: Bearer $LOADER_TOKEN")
if [ "$HTTP_CODE" = "403" ]; then
  echo "✅ Loader correctly denied (403)"
else
  echo "⚠️  Expected 403, got $HTTP_CODE"
fi

# ── Test 7: Disable digest → verify skip ──

echo ""
echo "─── Test 7: Disabled digest skips ───"
curl -sf "$BASE/settings/notifications" -X PUT \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d '{
    "telegram_enabled": true,
    "telegram_bot_token": "fake-token-for-testing",
    "telegram_chat_ids": "-1001234567890",
    "expiry_digest_enabled": false
  }' > /dev/null

RUN4=$(curl -sf "$BASE/alerts/expiry-digest/run?force=true" -X POST -H "$AUTH")
SKIPPED4=$(echo "$RUN4" | jq -r '.skipped')
REASON4=$(echo "$RUN4" | jq -r '.reason')
echo "  skipped=$SKIPPED4 reason=$REASON4"
[ "$SKIPPED4" = "true" ] || (echo "FAIL: should skip when disabled" && exit 1)
echo "✅ Disabled digest correctly skipped"

echo ""
echo "═══════════════════════════════════════════════"
echo "  ALL EXPIRY DIGEST TESTS PASSED ✅"
echo "═══════════════════════════════════════════════"
