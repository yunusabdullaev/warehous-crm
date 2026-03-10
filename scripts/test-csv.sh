#!/usr/bin/env bash
set -euo pipefail

# ═══════════════════════════════════════════
#   Warehouse CRM — CSV Import/Export Tests
# ═══════════════════════════════════════════

BASE=${1:-http://localhost:3003}
API="$BASE/api/v1"
SAMPLES_DIR="$(cd "$(dirname "$0")/../samples" && pwd)"

PASS=0
FAIL=0

pass() { ((PASS++)); echo "  ✅ $1"; }
fail() { ((FAIL++)); echo "  ❌ $1"; }

echo "══════════════════════════════════════"
echo "  CSV Import/Export Test Suite"
echo "  API: $API"
echo "══════════════════════════════════════"

# ── 1. Login as admin ──
echo ""
echo "▸ Step 1: Login as admin"
# Register admin user (idempotent — may already exist)
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
    echo "Response: $LOGIN_BODY"
    exit 1
  fi
  pass "Logged in as admin (token=${TOKEN:0:20}...)"
else
  fail "Login failed (HTTP $LOGIN_CODE)"
  echo "Response: $LOGIN_BODY"
  exit 1
fi

AUTH="Authorization: Bearer $TOKEN"

# ── 2. Import Products ──
echo ""
echo "▸ Step 2: Import Products CSV"
if [ ! -f "$SAMPLES_DIR/products.csv" ]; then
  fail "samples/products.csv not found at $SAMPLES_DIR"
  exit 1
fi

IMPORT_P_RES=$(curl -s -w "\n%{http_code}" -X POST "$API/import/products" \
  -H "$AUTH" \
  -F "file=@$SAMPLES_DIR/products.csv")
IMPORT_P_CODE=$(echo "$IMPORT_P_RES" | tail -1)
IMPORT_P_BODY=$(echo "$IMPORT_P_RES" | sed '$d')

if [ "$IMPORT_P_CODE" = "200" ]; then
  pass "Import products: HTTP 200"
  echo "  Report: $IMPORT_P_BODY"
else
  fail "Import products: HTTP $IMPORT_P_CODE"
  echo "  Response: $IMPORT_P_BODY"
fi

# ── 3. Import Locations ──
echo ""
echo "▸ Step 3: Import Locations CSV"
if [ ! -f "$SAMPLES_DIR/locations.csv" ]; then
  fail "samples/locations.csv not found at $SAMPLES_DIR"
  exit 1
fi

IMPORT_L_RES=$(curl -s -w "\n%{http_code}" -X POST "$API/import/locations" \
  -H "$AUTH" \
  -F "file=@$SAMPLES_DIR/locations.csv")
IMPORT_L_CODE=$(echo "$IMPORT_L_RES" | tail -1)
IMPORT_L_BODY=$(echo "$IMPORT_L_RES" | sed '$d')

if [ "$IMPORT_L_CODE" = "200" ]; then
  pass "Import locations: HTTP 200"
  echo "  Report: $IMPORT_L_BODY"
else
  fail "Import locations: HTTP $IMPORT_L_CODE"
  echo "  Response: $IMPORT_L_BODY"
fi

# ── 4. Re-import Products (test upsert/update) ──
echo ""
echo "▸ Step 4: Re-import Products (should update, not duplicate)"
REIMPORT_P_RES=$(curl -s -w "\n%{http_code}" -X POST "$API/import/products" \
  -H "$AUTH" \
  -F "file=@$SAMPLES_DIR/products.csv")
REIMPORT_P_CODE=$(echo "$REIMPORT_P_RES" | tail -1)
REIMPORT_P_BODY=$(echo "$REIMPORT_P_RES" | sed '$d')

if [ "$REIMPORT_P_CODE" = "200" ]; then
  pass "Re-import products: HTTP 200"
  echo "  Report: $REIMPORT_P_BODY"
else
  fail "Re-import products: HTTP $REIMPORT_P_CODE"
fi

# ── 5. Re-import Locations (should skip existing) ──
echo ""
echo "▸ Step 5: Re-import Locations (should skip existing)"
REIMPORT_L_RES=$(curl -s -w "\n%{http_code}" -X POST "$API/import/locations" \
  -H "$AUTH" \
  -F "file=@$SAMPLES_DIR/locations.csv")
REIMPORT_L_CODE=$(echo "$REIMPORT_L_RES" | tail -1)
REIMPORT_L_BODY=$(echo "$REIMPORT_L_RES" | sed '$d')

if [ "$REIMPORT_L_CODE" = "200" ]; then
  pass "Re-import locations: HTTP 200"
  echo "  Report: $REIMPORT_L_BODY"
else
  fail "Re-import locations: HTTP $REIMPORT_L_CODE"
fi

# ── 6. Export Products CSV ──
echo ""
echo "▸ Step 6: Export Products CSV"
EXPORT_P_CODE=$(curl -s -w "%{http_code}" -o /tmp/export_products.csv \
  -H "$AUTH" "$API/export/products")

if [ "$EXPORT_P_CODE" = "200" ]; then
  LINES=$(wc -l < /tmp/export_products.csv | tr -d ' ')
  pass "Export products: HTTP 200 ($LINES lines)"
  echo "  Preview:"
  head -5 /tmp/export_products.csv | sed 's/^/    /'
else
  fail "Export products: HTTP $EXPORT_P_CODE"
fi

# ── 7. Export Locations CSV ──
echo ""
echo "▸ Step 7: Export Locations CSV"
EXPORT_L_CODE=$(curl -s -w "%{http_code}" -o /tmp/export_locations.csv \
  -H "$AUTH" "$API/export/locations")

if [ "$EXPORT_L_CODE" = "200" ]; then
  LINES=$(wc -l < /tmp/export_locations.csv | tr -d ' ')
  pass "Export locations: HTTP 200 ($LINES lines)"
  echo "  Preview:"
  head -5 /tmp/export_locations.csv | sed 's/^/    /'
else
  fail "Export locations: HTTP $EXPORT_L_CODE"
fi

# ── 8. RBAC Test — operator should get 403 ──
echo ""
echo "▸ Step 8: RBAC — create operator and try import"
# Register operator
curl -s -X POST "$API/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"csv_operator","password":"pass123","role":"operator"}' > /dev/null 2>&1

OP_LOGIN=$(curl -s -X POST "$API/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"csv_operator","password":"pass123"}')
OP_TOKEN=$(echo "$OP_LOGIN" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if [ -n "$OP_TOKEN" ]; then
  RBAC_RES=$(curl -s -w "\n%{http_code}" -X POST "$API/import/products" \
    -H "Authorization: Bearer $OP_TOKEN" \
    -F "file=@$SAMPLES_DIR/products.csv")
  RBAC_CODE=$(echo "$RBAC_RES" | tail -1)
  if [ "$RBAC_CODE" = "403" ]; then
    pass "RBAC: operator gets 403 on import"
  else
    fail "RBAC: operator got HTTP $RBAC_CODE (expected 403)"
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
