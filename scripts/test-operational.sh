#!/usr/bin/env bash
set -euo pipefail

# ═══════════════════════════════════════════
#   Warehouse CRM — Operational Test Suite
#   Tests: RBAC, Dashboard, Reports
# ═══════════════════════════════════════════

BASE="${1:-http://localhost:3003}"
API="$BASE/api/v1"
GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; NC='\033[0m'
PASS=0; FAIL=0; TOTAL=0

pass() { ((PASS++)); ((TOTAL++)); echo -e "  ${GREEN}✓ PASS${NC} — $1"; }
fail() { ((FAIL++)); ((TOTAL++)); echo -e "  ${RED}✗ FAIL${NC} — $1"; }

echo ""
echo "═══════════════════════════════════════════"
echo "  Warehouse CRM — Operational Test Suite"
echo "  Target: $BASE"
echo "═══════════════════════════════════════════"
echo ""

# ──────────────────────────────────────────────
# 1. Create users of each role
# ──────────────────────────────────────────────
echo "[1/8] Create test users (admin, operator, loader)"

# Admin
ADM_BODY=$(curl -s -X POST "$API/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"test_admin","password":"pass123","role":"admin"}')
ADM_TOKEN=$(echo "$ADM_BODY" | jq -r '.token // empty')
if [ -n "$ADM_TOKEN" ]; then pass "Admin registered"; else
  ADM_BODY=$(curl -s -X POST "$API/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"test_admin","password":"pass123"}')
  ADM_TOKEN=$(echo "$ADM_BODY" | jq -r '.token // empty')
  if [ -n "$ADM_TOKEN" ]; then pass "Admin logged in (already exists)"; else fail "Admin auth failed"; fi
fi

# Operator
OP_BODY=$(curl -s -X POST "$API/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"test_operator","password":"pass123","role":"operator"}')
OP_TOKEN=$(echo "$OP_BODY" | jq -r '.token // empty')
if [ -n "$OP_TOKEN" ]; then pass "Operator registered"; else
  OP_BODY=$(curl -s -X POST "$API/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"test_operator","password":"pass123"}')
  OP_TOKEN=$(echo "$OP_BODY" | jq -r '.token // empty')
  if [ -n "$OP_TOKEN" ]; then pass "Operator logged in (already exists)"; else fail "Operator auth failed"; fi
fi

# Loader
LDR_BODY=$(curl -s -X POST "$API/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"test_loader","password":"pass123","role":"loader"}')
LDR_TOKEN=$(echo "$LDR_BODY" | jq -r '.token // empty')
if [ -n "$LDR_TOKEN" ]; then pass "Loader registered"; else
  LDR_BODY=$(curl -s -X POST "$API/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"test_loader","password":"pass123"}')
  LDR_TOKEN=$(echo "$LDR_BODY" | jq -r '.token // empty')
  if [ -n "$LDR_TOKEN" ]; then pass "Loader logged in (already exists)"; else fail "Loader auth failed"; fi
fi

# ──────────────────────────────────────────────
# 2. Admin creates base data
# ──────────────────────────────────────────────
echo ""
echo "[2/8] Admin creates product + location"

PID=$(curl -s -X POST "$API/products" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADM_TOKEN" \
  -d '{"sku":"RBAC-TEST-001","name":"RBAC Test Product","unit":"pcs","category":"test"}' \
  | jq -r '.id // empty')
if [ -n "$PID" ]; then pass "Product created PID=$PID"; else fail "Product creation failed"; fi

LID=$(curl -s -X POST "$API/locations" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADM_TOKEN" \
  -d '{"code":"RBAC-LOC-01","name":"RBAC Test Location","zone":"A","rack":"R1","level":"L1"}' \
  | jq -r '.id // empty')
if [ -n "$LID" ]; then pass "Location created LID=$LID"; else fail "Location creation failed"; fi

# ──────────────────────────────────────────────
# 3. RBAC — Operator forbidden actions
# ──────────────────────────────────────────────
echo ""
echo "[3/8] RBAC — Operator forbidden actions"

# Operator should NOT be able to create products
HTTP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/products" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OP_TOKEN" \
  -d '{"sku":"OP-FAIL","name":"Should Fail","unit":"pcs"}')
if [ "$HTTP" = "403" ]; then pass "Operator cannot create product (403)"; else fail "Expected 403, got $HTTP"; fi

# Operator should NOT be able to delete products
HTTP=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$API/products/$PID" \
  -H "Authorization: Bearer $OP_TOKEN")
if [ "$HTTP" = "403" ]; then pass "Operator cannot delete product (403)"; else fail "Expected 403, got $HTTP"; fi

# Operator CAN view products
HTTP=$(curl -s -o /dev/null -w "%{http_code}" "$API/products" \
  -H "Authorization: Bearer $OP_TOKEN")
if [ "$HTTP" = "200" ]; then pass "Operator can view products (200)"; else fail "Expected 200, got $HTTP"; fi

# Operator CAN create inbound
HTTP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/inbound" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OP_TOKEN" \
  -d "{\"product_id\":\"$PID\",\"location_id\":\"$LID\",\"quantity\":500,\"reference\":\"OP-IN\"}")
if [ "$HTTP" = "201" ]; then pass "Operator can create inbound (201)"; else fail "Expected 201, got $HTTP"; fi

# Operator CAN create outbound
HTTP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/outbound" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OP_TOKEN" \
  -d "{\"product_id\":\"$PID\",\"location_id\":\"$LID\",\"quantity\":100,\"reference\":\"OP-OUT\"}")
if [ "$HTTP" = "201" ]; then pass "Operator can create outbound (201)"; else fail "Expected 201, got $HTTP"; fi

# Operator should NOT access reports
HTTP=$(curl -s -o /dev/null -w "%{http_code}" "$API/reports/movements" \
  -H "Authorization: Bearer $OP_TOKEN")
if [ "$HTTP" = "403" ]; then pass "Operator cannot access reports (403)"; else fail "Expected 403, got $HTTP"; fi

# ──────────────────────────────────────────────
# 4. RBAC — Loader forbidden actions
# ──────────────────────────────────────────────
echo ""
echo "[4/8] RBAC — Loader forbidden actions"

# Loader CANNOT create products
HTTP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/products" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $LDR_TOKEN" \
  -d '{"sku":"LDR-FAIL","name":"Should Fail","unit":"pcs"}')
if [ "$HTTP" = "403" ]; then pass "Loader cannot create product (403)"; else fail "Expected 403, got $HTTP"; fi

# Loader CAN view products
HTTP=$(curl -s -o /dev/null -w "%{http_code}" "$API/products" \
  -H "Authorization: Bearer $LDR_TOKEN")
if [ "$HTTP" = "200" ]; then pass "Loader can view products (200)"; else fail "Expected 200, got $HTTP"; fi

# Loader CAN view stock
HTTP=$(curl -s -o /dev/null -w "%{http_code}" "$API/stock" \
  -H "Authorization: Bearer $LDR_TOKEN")
if [ "$HTTP" = "200" ]; then pass "Loader can view stock (200)"; else fail "Expected 200, got $HTTP"; fi

# Loader CAN create inbound
HTTP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/inbound" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $LDR_TOKEN" \
  -d "{\"product_id\":\"$PID\",\"location_id\":\"$LID\",\"quantity\":200,\"reference\":\"LDR-IN\"}")
if [ "$HTTP" = "201" ]; then pass "Loader can create inbound (201)"; else fail "Expected 201, got $HTTP"; fi

# Loader CANNOT create outbound
HTTP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/outbound" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $LDR_TOKEN" \
  -d "{\"product_id\":\"$PID\",\"location_id\":\"$LID\",\"quantity\":50,\"reference\":\"LDR-OUT\"}")
if [ "$HTTP" = "403" ]; then pass "Loader cannot create outbound (403)"; else fail "Expected 403, got $HTTP"; fi

# Loader CANNOT view history
HTTP=$(curl -s -o /dev/null -w "%{http_code}" "$API/history" \
  -H "Authorization: Bearer $LDR_TOKEN")
if [ "$HTTP" = "403" ]; then pass "Loader cannot view history (403)"; else fail "Expected 403, got $HTTP"; fi

# Loader CANNOT access dashboard
HTTP=$(curl -s -o /dev/null -w "%{http_code}" "$API/dashboard/summary" \
  -H "Authorization: Bearer $LDR_TOKEN")
if [ "$HTTP" = "403" ]; then pass "Loader cannot access dashboard (403)"; else fail "Expected 403, got $HTTP"; fi

# Loader CANNOT access reports
HTTP=$(curl -s -o /dev/null -w "%{http_code}" "$API/reports/movements" \
  -H "Authorization: Bearer $LDR_TOKEN")
if [ "$HTTP" = "403" ]; then pass "Loader cannot access reports (403)"; else fail "Expected 403, got $HTTP"; fi

# ──────────────────────────────────────────────
# 5. Dashboard — Admin
# ──────────────────────────────────────────────
echo ""
echo "[5/8] Dashboard Summary (Admin)"

DASH=$(curl -s "$API/dashboard/summary" \
  -H "Authorization: Bearer $ADM_TOKEN")
echo "  Response:"
echo "$DASH" | jq '.' 2>/dev/null || echo "$DASH"

TOTAL_PRODUCTS=$(echo "$DASH" | jq -r '.total_products // 0')
if [ "$TOTAL_PRODUCTS" -ge 1 ]; then pass "Dashboard total_products >= 1 (got: $TOTAL_PRODUCTS)"; else fail "Dashboard total_products < 1"; fi

TOTAL_STOCK=$(echo "$DASH" | jq -r '.total_stock_qty // 0')
if [ "$TOTAL_STOCK" -ge 1 ]; then pass "Dashboard total_stock_qty >= 1 (got: $TOTAL_STOCK)"; else fail "Dashboard total_stock_qty < 1"; fi

# ──────────────────────────────────────────────
# 6. Dashboard — Operator can access
# ──────────────────────────────────────────────
echo ""
echo "[6/8] Dashboard Summary (Operator — should succeed)"

HTTP=$(curl -s -o /dev/null -w "%{http_code}" "$API/dashboard/summary" \
  -H "Authorization: Bearer $OP_TOKEN")
if [ "$HTTP" = "200" ]; then pass "Operator can access dashboard (200)"; else fail "Expected 200, got $HTTP"; fi

# ──────────────────────────────────────────────
# 7. Reports — Movements
# ──────────────────────────────────────────────
echo ""
echo "[7/8] Reports — Movements (Admin)"

TODAY=$(date +%Y-%m-%d)
MOVEMENTS=$(curl -s "$API/reports/movements?from=$TODAY&to=$TODAY&groupBy=day" \
  -H "Authorization: Bearer $ADM_TOKEN")
echo "  Response:"
echo "$MOVEMENTS" | jq '.' 2>/dev/null || echo "$MOVEMENTS"

DATA_LEN=$(echo "$MOVEMENTS" | jq -r '.data | length // 0')
if [ "$DATA_LEN" -ge 1 ]; then pass "Movements report has data (buckets: $DATA_LEN)"; else fail "Movements report empty"; fi

# ──────────────────────────────────────────────
# 8. Reports — Stock by zone
# ──────────────────────────────────────────────
echo ""
echo "[8/8] Reports — Stock by zone (Admin)"

STOCK_RPT=$(curl -s "$API/reports/stock?groupBy=zone" \
  -H "Authorization: Bearer $ADM_TOKEN")
echo "  Response:"
echo "$STOCK_RPT" | jq '.' 2>/dev/null || echo "$STOCK_RPT"

STOCK_LEN=$(echo "$STOCK_RPT" | jq -r '.data | length // 0')
if [ "$STOCK_LEN" -ge 1 ]; then pass "Stock report has data (groups: $STOCK_LEN)"; else fail "Stock report empty"; fi

# ──────────────────────────────────────────────
# Summary
# ──────────────────────────────────────────────
echo ""
echo "═══════════════════════════════════════════"
echo -e "  Results: ${GREEN}${PASS} passed${NC}, ${RED}${FAIL} failed${NC}, ${TOTAL} total"
echo "═══════════════════════════════════════════"

[ "$FAIL" -eq 0 ] && exit 0 || exit 1
