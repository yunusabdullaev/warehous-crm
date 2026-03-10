#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════════
# Multi-Warehouse Integration Test Script
# ═══════════════════════════════════════════════════════════════════
#
# Usage:
#   ./scripts/test-multiwarehouse.sh [BASE_URL]
#
#   BASE_URL defaults to http://localhost:3000
#
# Prerequisites:
#   - The server must be running at BASE_URL
#   - curl + jq must be installed
#
# This script creates test data, validates cross-warehouse
# isolation, RBAC enforcement, and admin ALL mode.
# ═══════════════════════════════════════════════════════════════════

set -euo pipefail

BASE_URL="${1:-http://localhost:3000}"
API="${BASE_URL}/api"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

PASS=0
FAIL=0

# ── Helpers ──────────────────────────────────────────────────────

pass() { ((PASS++)); echo -e "  ${GREEN}✅ PASS${NC} — $1"; }
fail() { ((FAIL++)); echo -e "  ${RED}❌ FAIL${NC} — $1 $2"; }

assert_eq() {
  local label="$1" expected="$2" actual="$3"
  if [[ "$actual" == "$expected" ]]; then
    pass "$label"
  else
    fail "$label" "(expected='$expected', got='$actual')"
  fi
}

assert_ne() {
  local label="$1" not_expected="$2" actual="$3"
  if [[ "$actual" != "$not_expected" ]]; then
    pass "$label"
  else
    fail "$label" "(did not expect='$not_expected')"
  fi
}

assert_gt() {
  local label="$1" threshold="$2" actual="$3"
  if [[ "$actual" -gt "$threshold" ]]; then
    pass "$label"
  else
    fail "$label" "(expected > $threshold, got $actual)"
  fi
}

assert_http() {
  local label="$1" expected_code="$2" actual_code="$3"
  assert_eq "$label" "$expected_code" "$actual_code"
}

http_get() {
  local token="$1" warehouse_id="$2" url="$3"
  if [[ "$warehouse_id" == "NONE" ]]; then
    curl -s -w "\n%{http_code}" -H "Authorization: Bearer $token" "$url"
  else
    curl -s -w "\n%{http_code}" -H "Authorization: Bearer $token" -H "X-Warehouse-Id: $warehouse_id" "$url"
  fi
}

http_post() {
  local token="$1" warehouse_id="$2" url="$3" data="$4"
  if [[ "$warehouse_id" == "NONE" ]]; then
    curl -s -w "\n%{http_code}" -H "Authorization: Bearer $token" -H "Content-Type: application/json" -d "$data" "$url"
  else
    curl -s -w "\n%{http_code}" -H "Authorization: Bearer $token" -H "X-Warehouse-Id: $warehouse_id" -H "Content-Type: application/json" -d "$data" "$url"
  fi
}

http_put() {
  local token="$1" warehouse_id="$2" url="$3" data="$4"
  if [[ "$warehouse_id" == "NONE" ]]; then
    curl -s -w "\n%{http_code}" -H "Authorization: Bearer $token" -H "Content-Type: application/json" -X PUT -d "$data" "$url"
  else
    curl -s -w "\n%{http_code}" -H "Authorization: Bearer $token" -H "X-Warehouse-Id: $warehouse_id" -H "Content-Type: application/json" -X PUT -d "$data" "$url"
  fi
}

http_delete() {
  local token="$1" warehouse_id="$2" url="$3"
  if [[ "$warehouse_id" == "NONE" ]]; then
    curl -s -w "\n%{http_code}" -H "Authorization: Bearer $token" -X DELETE "$url"
  else
    curl -s -w "\n%{http_code}" -H "Authorization: Bearer $token" -H "X-Warehouse-Id: $warehouse_id" -X DELETE "$url"
  fi
}

extract_body() {
  # Remove the last line (http code)
  echo "$1" | sed '$d'
}

extract_code() {
  echo "$1" | tail -n 1
}

# ── Start ────────────────────────────────────────────────────────

echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}  Multi-Warehouse Integration Tests${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════${NC}"
echo -e "  Target: ${BASE_URL}"
echo ""

# Health check
echo -e "${YELLOW}▸ Health Check${NC}"
HEALTH=$(curl -s -w "\n%{http_code}" "${BASE_URL}/health")
HEALTH_CODE=$(extract_code "$HEALTH")
assert_http "Server is healthy" "200" "$HEALTH_CODE"
echo ""

# ┌─────────────────────────────────────────────────────────────┐
# │ 1. Setup — Register admin and create test warehouses        │
# └─────────────────────────────────────────────────────────────┘
echo -e "${YELLOW}▸ 1. Setup — Admin Registration${NC}"

TS=$(date +%s)
ADMIN_USER="testadmin_${TS}"
ADMIN_PASS="password123"

RESP=$(curl -s -w "\n%{http_code}" -H "Content-Type: application/json" \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\",\"role\":\"admin\"}" \
  "${API}/auth/register")
CODE=$(extract_code "$RESP")

if [[ "$CODE" != "200" && "$CODE" != "201" ]]; then
  # Try login instead (user may exist)
  RESP=$(curl -s -w "\n%{http_code}" -H "Content-Type: application/json" \
    -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\"}" \
    "${API}/auth/login")
  CODE=$(extract_code "$RESP")
fi

BODY=$(extract_body "$RESP")
ADMIN_TOKEN=$(echo "$BODY" | jq -r '.token')
assert_ne "Got admin token" "" "$ADMIN_TOKEN"

# ── Create two warehouses ──
echo ""
echo -e "${YELLOW}▸ 1b. Setup — Create Test Warehouses${NC}"

RESP=$(http_post "$ADMIN_TOKEN" "NONE" "${API}/warehouses" \
  "{\"code\":\"WH_A_${TS}\",\"name\":\"Test Warehouse A\",\"address\":\"Address A\"}")
WH_A_BODY=$(extract_body "$RESP")
WH_A_ID=$(echo "$WH_A_BODY" | jq -r '.id // .data.id // empty')
# Fallback: if create returned 409 or similar, try listing
if [[ -z "$WH_A_ID" ]]; then
  # Create with different approach
  WH_A_ID="SKIP"
fi
assert_ne "Created Warehouse A" "" "$WH_A_ID"

RESP=$(http_post "$ADMIN_TOKEN" "NONE" "${API}/warehouses" \
  "{\"code\":\"WH_B_${TS}\",\"name\":\"Test Warehouse B\",\"address\":\"Address B\"}")
WH_B_BODY=$(extract_body "$RESP")
WH_B_ID=$(echo "$WH_B_BODY" | jq -r '.id // .data.id // empty')
if [[ -z "$WH_B_ID" ]]; then
  WH_B_ID="SKIP"
fi
assert_ne "Created Warehouse B" "" "$WH_B_ID"

if [[ "$WH_A_ID" == "SKIP" || "$WH_B_ID" == "SKIP" ]]; then
  echo -e "${RED}Cannot proceed without two warehouses. Exiting.${NC}"
  exit 1
fi

# ┌─────────────────────────────────────────────────────────────┐
# │ 2. Cross-Warehouse Isolation — Locations                    │
# └─────────────────────────────────────────────────────────────┘
echo ""
echo -e "${YELLOW}▸ 2. Cross-Warehouse Isolation — Locations${NC}"

# Create a location in WH_A
RESP=$(http_post "$ADMIN_TOKEN" "$WH_A_ID" "${API}/locations" \
  "{\"zone\":\"ZONE_A\",\"rack\":\"R1\",\"shelf\":\"S1\",\"bin\":\"B1\"}")
CODE=$(extract_code "$RESP")
LOC_A_BODY=$(extract_body "$RESP")
LOC_A_ID=$(echo "$LOC_A_BODY" | jq -r '.id // empty')
assert_ne "Created location in WH_A" "" "$LOC_A_ID"

# Create a location in WH_B
RESP=$(http_post "$ADMIN_TOKEN" "$WH_B_ID" "${API}/locations" \
  "{\"zone\":\"ZONE_B\",\"rack\":\"R2\",\"shelf\":\"S2\",\"bin\":\"B2\"}")
CODE=$(extract_code "$RESP")
LOC_B_BODY=$(extract_body "$RESP")
LOC_B_ID=$(echo "$LOC_B_BODY" | jq -r '.id // empty')
assert_ne "Created location in WH_B" "" "$LOC_B_ID"

# List locations from WH_A — should NOT see WH_B location
RESP=$(http_get "$ADMIN_TOKEN" "$WH_A_ID" "${API}/locations")
BODY=$(extract_body "$RESP")
ZONES=$(echo "$BODY" | jq -r '[.data[].zone] | join(",")')
if echo "$ZONES" | grep -q "ZONE_B"; then
  fail "WH_A locations should NOT include ZONE_B" "(got $ZONES)"
else
  pass "WH_A locations do NOT include WH_B data (isolation)"
fi

# List locations from WH_B — should NOT see WH_A location
RESP=$(http_get "$ADMIN_TOKEN" "$WH_B_ID" "${API}/locations")
BODY=$(extract_body "$RESP")
ZONES=$(echo "$BODY" | jq -r '[.data[].zone] | join(",")')
if echo "$ZONES" | grep -q "ZONE_A"; then
  fail "WH_B locations should NOT include ZONE_A" "(got $ZONES)"
else
  pass "WH_B locations do NOT include WH_A data (isolation)"
fi

# ┌─────────────────────────────────────────────────────────────┐
# │ 3. Dashboard Scoping                                        │
# └─────────────────────────────────────────────────────────────┘
echo ""
echo -e "${YELLOW}▸ 3. Dashboard Scoping${NC}"

# Get dashboard for WH_A
RESP=$(http_get "$ADMIN_TOKEN" "$WH_A_ID" "${API}/dashboard/summary")
CODE=$(extract_code "$RESP")
assert_http "Dashboard WH_A returns 200" "200" "$CODE"

BODY=$(extract_body "$RESP")
LOC_COUNT_A=$(echo "$BODY" | jq '.total_locations')
assert_gt "WH_A dashboard shows locations" "0" "$LOC_COUNT_A"

# Get dashboard for WH_B
RESP=$(http_get "$ADMIN_TOKEN" "$WH_B_ID" "${API}/dashboard/summary")
CODE=$(extract_code "$RESP")
assert_http "Dashboard WH_B returns 200" "200" "$CODE"

# Get dashboard with ALL mode (admin)
RESP=$(http_get "$ADMIN_TOKEN" "ALL" "${API}/dashboard/summary")
CODE=$(extract_code "$RESP")
assert_http "Dashboard ALL mode returns 200 for admin" "200" "$CODE"

# ┌─────────────────────────────────────────────────────────────┐
# │ 4. Reports Scoping                                          │
# └─────────────────────────────────────────────────────────────┘
echo ""
echo -e "${YELLOW}▸ 4. Reports Scoping${NC}"

# Movements report
RESP=$(http_get "$ADMIN_TOKEN" "$WH_A_ID" "${API}/reports/movements?from=2020-01-01&to=2030-01-01")
CODE=$(extract_code "$RESP")
assert_http "Movements report WH_A returns 200" "200" "$CODE"

# Stock report
RESP=$(http_get "$ADMIN_TOKEN" "$WH_A_ID" "${API}/reports/stock?groupBy=zone")
CODE=$(extract_code "$RESP")
assert_http "Stock report WH_A returns 200" "200" "$CODE"

# Orders report
RESP=$(http_get "$ADMIN_TOKEN" "$WH_A_ID" "${API}/reports/orders?from=2020-01-01&to=2030-01-01")
CODE=$(extract_code "$RESP")
assert_http "Orders report WH_A returns 200" "200" "$CODE"

# Picking report
RESP=$(http_get "$ADMIN_TOKEN" "$WH_A_ID" "${API}/reports/picking?from=2020-01-01&to=2030-01-01")
CODE=$(extract_code "$RESP")
assert_http "Picking report WH_A returns 200" "200" "$CODE"

# Returns report
RESP=$(http_get "$ADMIN_TOKEN" "$WH_A_ID" "${API}/reports/returns?from=2020-01-01&to=2030-01-01")
CODE=$(extract_code "$RESP")
assert_http "Returns report WH_A returns 200" "200" "$CODE"

# Expiry report
RESP=$(http_get "$ADMIN_TOKEN" "$WH_A_ID" "${API}/reports/expiry?days=30")
CODE=$(extract_code "$RESP")
assert_http "Expiry report WH_A returns 200" "200" "$CODE"

# ALL mode reports
RESP=$(http_get "$ADMIN_TOKEN" "ALL" "${API}/reports/stock?groupBy=product")
CODE=$(extract_code "$RESP")
assert_http "Stock report ALL mode returns 200" "200" "$CODE"

# ┌─────────────────────────────────────────────────────────────┐
# │ 5. RBAC — Operator restricted to assigned warehouses        │
# └─────────────────────────────────────────────────────────────┘
echo ""
echo -e "${YELLOW}▸ 5. RBAC — Operator Warehouse Restriction${NC}"

OP_USER="testop_${TS}"
OP_PASS="password123"

# Register operator
RESP=$(curl -s -w "\n%{http_code}" -H "Content-Type: application/json" \
  -d "{\"username\":\"$OP_USER\",\"password\":\"$OP_PASS\",\"role\":\"operator\"}" \
  "${API}/auth/register")
BODY=$(extract_body "$RESP")
OP_TOKEN=$(echo "$BODY" | jq -r '.token')
OP_ID=$(echo "$BODY" | jq -r '.user.id')
assert_ne "Registered operator" "" "$OP_TOKEN"

# As admin, assign operator to only WH_A
RESP=$(http_put "$ADMIN_TOKEN" "$WH_A_ID" "${API}/users/${OP_ID}" \
  "{\"allowed_warehouse_ids\":[\"$WH_A_ID\"],\"default_warehouse_id\":\"$WH_A_ID\"}")
CODE=$(extract_code "$RESP")
assert_http "Admin assigned operator to WH_A" "200" "$CODE"

# Operator can access WH_A
RESP=$(http_get "$OP_TOKEN" "$WH_A_ID" "${API}/locations")
CODE=$(extract_code "$RESP")
assert_http "Operator CAN access WH_A" "200" "$CODE"

# Operator CANNOT access WH_B (not in allowed list)
RESP=$(http_get "$OP_TOKEN" "$WH_B_ID" "${API}/locations")
CODE=$(extract_code "$RESP")
assert_http "Operator CANNOT access WH_B" "403" "$CODE"

# Operator CANNOT use ALL mode
RESP=$(http_get "$OP_TOKEN" "ALL" "${API}/dashboard/summary")
CODE=$(extract_code "$RESP")
assert_http "Operator CANNOT use ALL mode" "400" "$CODE"

# ┌─────────────────────────────────────────────────────────────┐
# │ 6. User Warehouse Assignment Validation                     │
# └─────────────────────────────────────────────────────────────┘
echo ""
echo -e "${YELLOW}▸ 6. User Warehouse Assignment Validation${NC}"

# Default must be in allowed list
RESP=$(http_put "$ADMIN_TOKEN" "$WH_A_ID" "${API}/users/${OP_ID}" \
  "{\"allowed_warehouse_ids\":[\"$WH_A_ID\"],\"default_warehouse_id\":\"$WH_B_ID\"}")
CODE=$(extract_code "$RESP")
BODY=$(extract_body "$RESP")
MSG=$(echo "$BODY" | jq -r '.error // empty')
if [[ "$CODE" != "200" ]] && echo "$MSG" | grep -qi "default.*allowed\|allowed.*default"; then
  pass "Rejected: default WH not in allowed list"
else
  fail "Should reject default not in allowed list" "(code=$CODE, msg=$MSG)"
fi

# ┌─────────────────────────────────────────────────────────────┐
# │ 7. Inbound Isolation                                        │
# └─────────────────────────────────────────────────────────────┘
echo ""
echo -e "${YELLOW}▸ 7. Inbound Isolation${NC}"

# Create product first (global)
RESP=$(http_post "$ADMIN_TOKEN" "$WH_A_ID" "${API}/products" \
  "{\"sku\":\"TEST_SKU_${TS}\",\"name\":\"Test Product\",\"unit\":\"pcs\"}")
BODY=$(extract_body "$RESP")
PRODUCT_ID=$(echo "$BODY" | jq -r '.id // empty')

if [[ -n "$PRODUCT_ID" && "$PRODUCT_ID" != "null" ]]; then
  # Create inbound in WH_A
  RESP=$(http_post "$ADMIN_TOKEN" "$WH_A_ID" "${API}/inbounds" \
    "{\"product_id\":\"$PRODUCT_ID\",\"location_id\":\"$LOC_A_ID\",\"quantity\":100}")
  CODE=$(extract_code "$RESP")
  assert_http "Inbound in WH_A returns 200/201" "200" "$CODE"

  # List inbounds from WH_B — should not see WH_A inbound
  RESP=$(http_get "$ADMIN_TOKEN" "$WH_B_ID" "${API}/inbounds")
  BODY=$(extract_body "$RESP")
  COUNT=$(echo "$BODY" | jq '.data | length')
  assert_eq "WH_B inbounds list is empty" "0" "$COUNT"
else
  fail "Could not create test product" ""
fi

# ┌─────────────────────────────────────────────────────────────┐
# │ 8. Missing Warehouse Header                                 │
# └─────────────────────────────────────────────────────────────┘
echo ""
echo -e "${YELLOW}▸ 8. Missing Warehouse Header Handling${NC}"

# Non-admin user without default should get 400
LOADER_USER="testloader_${TS}"
RESP=$(curl -s -w "\n%{http_code}" -H "Content-Type: application/json" \
  -d "{\"username\":\"$LOADER_USER\",\"password\":\"$OP_PASS\",\"role\":\"loader\"}" \
  "${API}/auth/register")
BODY=$(extract_body "$RESP")
LOADER_TOKEN=$(echo "$BODY" | jq -r '.token')

if [[ -n "$LOADER_TOKEN" && "$LOADER_TOKEN" != "null" ]]; then
  RESP=$(http_get "$LOADER_TOKEN" "NONE" "${API}/locations")
  CODE=$(extract_code "$RESP")
  # Should be 400 (no warehouse context) or 403 (no allowed warehouses)
  if [[ "$CODE" == "400" || "$CODE" == "403" ]]; then
    pass "No header + no default → $CODE error"
  else
    fail "Expected 400/403 without warehouse context" "(got $CODE)"
  fi
fi

# ┌─────────────────────────────────────────────────────────────┐
# │ Summary                                                     │
# └─────────────────────────────────────────────────────────────┘
echo ""
echo -e "${CYAN}═══════════════════════════════════════════════════════${NC}"
echo -e "  ${GREEN}Passed: ${PASS}${NC}   ${RED}Failed: ${FAIL}${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════${NC}"

if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi

echo -e "\n${GREEN}All tests passed!${NC}\n"
