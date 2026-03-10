#!/usr/bin/env bash
set -euo pipefail

# ═══════════════════════════════════════════
#   Warehouse CRM — QR/Label Endpoint Tests
# ═══════════════════════════════════════════

BASE=${1:-http://localhost:3003}
API="$BASE/api/v1"

PASS=0
FAIL=0

pass() { ((PASS++)); echo "  ✅ $1"; }
fail() { ((FAIL++)); echo "  ❌ $1"; }

echo "══════════════════════════════════════"
echo "  QR / Label Endpoint Test Suite"
echo "  API: $API"
echo "══════════════════════════════════════"

# ── 1. Login as admin ──
echo ""
echo "▸ Step 1: Login as admin"
curl -s -X POST "$API/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123","role":"admin"}' > /dev/null 2>&1

LOGIN_RES=$(curl -s -w "\n%{http_code}" -X POST "$API/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}')
LOGIN_CODE=$(echo "$LOGIN_RES" | tail -1)
LOGIN_BODY=$(echo "$LOGIN_RES" | sed '$d')

if [ "$LOGIN_CODE" = "200" ]; then
  TOKEN=$(echo "$LOGIN_BODY" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
  if [ -z "$TOKEN" ]; then
    fail "Login returned 200 but no token"
    exit 1
  fi
  pass "Logged in as admin"
else
  fail "Login failed (HTTP $LOGIN_CODE)"
  exit 1
fi

AUTH="Authorization: Bearer $TOKEN"

# ── 2. Create a test location ──
echo ""
echo "▸ Step 2: Create test location"
LOC_RES=$(curl -s -w "\n%{http_code}" -X POST "$API/locations" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"code":"QR-TEST-01","name":"QR Test Location","zone":"A","rack":"R2","level":"S3"}')
LOC_CODE=$(echo "$LOC_RES" | tail -1)
LOC_BODY=$(echo "$LOC_RES" | sed '$d')

LOC_ID=""
if [ "$LOC_CODE" = "201" ]; then
  LOC_ID=$(echo "$LOC_BODY" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
  pass "Location created (id=${LOC_ID:0:12}...)"
elif [ "$LOC_CODE" = "409" ]; then
  # Already exists — fetch it
  LIST_RES=$(curl -s -X GET "$API/locations?limit=100" -H "$AUTH")
  LOC_ID=$(echo "$LIST_RES" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
  pass "Location already exists (id=${LOC_ID:0:12}...)"
else
  fail "Create location: HTTP $LOC_CODE"
  echo "  Response: $LOC_BODY"
  exit 1
fi

# ── 3. GET QR code PNG ──
echo ""
echo "▸ Step 3: GET /locations/:id/qr → PNG"
QR_CODE=$(curl -s -w "%{http_code}" -o /tmp/qr_test.png \
  -H "$AUTH" "$API/locations/$LOC_ID/qr")

if [ "$QR_CODE" = "200" ]; then
  FTYPE=$(file -b --mime-type /tmp/qr_test.png)
  if [[ "$FTYPE" == image/png ]]; then
    SIZE=$(wc -c < /tmp/qr_test.png | tr -d ' ')
    pass "QR endpoint: 200 OK, image/png, ${SIZE} bytes"
  else
    fail "QR endpoint: 200 but content-type is $FTYPE"
  fi
else
  fail "QR endpoint: HTTP $QR_CODE"
fi

# ── 4. GET label PDF ──
echo ""
echo "▸ Step 4: GET /locations/:id/label → PDF"
LABEL_CODE=$(curl -s -w "%{http_code}" -o /tmp/label_test.pdf \
  -H "$AUTH" "$API/locations/$LOC_ID/label")

if [ "$LABEL_CODE" = "200" ]; then
  FTYPE=$(file -b --mime-type /tmp/label_test.pdf)
  if [[ "$FTYPE" == application/pdf ]]; then
    SIZE=$(wc -c < /tmp/label_test.pdf | tr -d ' ')
    pass "Label endpoint: 200 OK, application/pdf, ${SIZE} bytes"
  else
    fail "Label endpoint: 200 but content-type is $FTYPE"
  fi
else
  fail "Label endpoint: HTTP $LABEL_CODE"
fi

# ── 5. RBAC: operator should get 403 ──
echo ""
echo "▸ Step 5: RBAC — operator should get 403 on QR and label"
curl -s -X POST "$API/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"qr_operator","password":"pass123","role":"operator"}' > /dev/null 2>&1

OP_LOGIN=$(curl -s -X POST "$API/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"qr_operator","password":"pass123"}')
OP_TOKEN=$(echo "$OP_LOGIN" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if [ -n "$OP_TOKEN" ]; then
  QR_RBAC=$(curl -s -w "%{http_code}" -o /dev/null \
    -H "Authorization: Bearer $OP_TOKEN" "$API/locations/$LOC_ID/qr")
  LABEL_RBAC=$(curl -s -w "%{http_code}" -o /dev/null \
    -H "Authorization: Bearer $OP_TOKEN" "$API/locations/$LOC_ID/label")

  if [ "$QR_RBAC" = "403" ] && [ "$LABEL_RBAC" = "403" ]; then
    pass "RBAC: operator gets 403 on both QR and label"
  else
    fail "RBAC: operator got QR=$QR_RBAC label=$LABEL_RBAC (expected 403)"
  fi
else
  fail "RBAC: could not login as operator"
fi

# ── Summary ──
echo ""
echo "══════════════════════════════════════"
echo "  Results: $PASS passed, $FAIL failed"
echo "══════════════════════════════════════"

[ "$FAIL" -eq 0 ] && exit 0 || exit 1
