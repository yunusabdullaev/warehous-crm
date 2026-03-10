#!/usr/bin/env bash
#
# Multi-Tenant Integration Test Script
#
# Usage:
#   ./scripts/test-multitenant.sh                  # default: http://localhost:3003
#   ./scripts/test-multitenant.sh https://api.com   # custom base URL
#
# Prerequisites:
#   - Server running with a clean database (or be aware of existing data)
#   - jq installed
#   - curl installed

set -euo pipefail

BASE="${1:-http://localhost:3003}"
API="$BASE/api/v1"

PASS=0
FAIL=0
TOTAL=0

# ── Helpers ──

check() {
    local desc="$1" expected="$2" actual="$3"
    TOTAL=$((TOTAL + 1))
    if [ "$expected" = "$actual" ]; then
        echo "  ✅ $desc"
        PASS=$((PASS + 1))
    else
        echo "  ❌ $desc (expected=$expected, got=$actual)"
        FAIL=$((FAIL + 1))
    fi
}

check_not() {
    local desc="$1" not_expected="$2" actual="$3"
    TOTAL=$((TOTAL + 1))
    if [ "$not_expected" != "$actual" ]; then
        echo "  ✅ $desc"
        PASS=$((PASS + 1))
    else
        echo "  ❌ $desc (expected NOT $not_expected, but got it)"
        FAIL=$((FAIL + 1))
    fi
}

# ── 0. Setup: Register superadmin ──
echo ""
echo "═══════════════════════════════════════════════════════"
echo "  Multi-Tenant Integration Tests"
echo "  Base URL: $API"
echo "═══════════════════════════════════════════════════════"
echo ""

echo "── Step 0: Setup ──"

# Register a superadmin user
SA_RES=$(curl -s -X POST "$API/auth/register" \
    -H "Content-Type: application/json" \
    -d '{"username":"sa_test_'$RANDOM'","password":"test1234","role":"superadmin"}')
SA_TOKEN=$(echo "$SA_RES" | jq -r '.token // empty')

if [ -z "$SA_TOKEN" ]; then
    echo "  ⚠️  Could not register superadmin. Trying login..."
    SA_RES=$(curl -s -X POST "$API/auth/login" \
        -H "Content-Type: application/json" \
        -d '{"username":"superadmin","password":"admin123"}')
    SA_TOKEN=$(echo "$SA_RES" | jq -r '.token // empty')
fi

if [ -z "$SA_TOKEN" ]; then
    echo "  ❌ Cannot get superadmin token. Aborting."
    exit 1
fi
echo "  ✅ Superadmin token acquired"

SA_AUTH="Authorization: Bearer $SA_TOKEN"

# ── 1. Create Tenants ──
echo ""
echo "── Step 1: Create Tenants ──"

T1_RES=$(curl -s -X POST "$API/tenants" \
    -H "Content-Type: application/json" \
    -H "$SA_AUTH" \
    -d '{"code":"TEN-TEST-A","name":"Test Tenant A","plan":"PRO"}')
T1_ID=$(echo "$T1_RES" | jq -r '.id // empty')
check "Create Tenant A" "true" "$([ -n "$T1_ID" ] && echo true || echo false)"

T2_RES=$(curl -s -X POST "$API/tenants" \
    -H "Content-Type: application/json" \
    -H "$SA_AUTH" \
    -d '{"code":"TEN-TEST-B","name":"Test Tenant B","plan":"FREE"}')
T2_ID=$(echo "$T2_RES" | jq -r '.id // empty')
check "Create Tenant B" "true" "$([ -n "$T2_ID" ] && echo true || echo false)"

# ── 2. List Tenants ──
echo ""
echo "── Step 2: List Tenants ──"

LIST_RES=$(curl -s -X GET "$API/tenants?limit=100" -H "$SA_AUTH")
TENANT_COUNT=$(echo "$LIST_RES" | jq -r '.data | length')
check "Tenant list has entries" "true" "$([ "$TENANT_COUNT" -ge 2 ] && echo true || echo false)"

# ── 3. Create tenant-admin users per tenant ──
echo ""
echo "── Step 3: Create Tenant Admins ──"

TA1_RES=$(curl -s -X POST "$API/auth/register" \
    -H "Content-Type: application/json" \
    -H "$SA_AUTH" \
    -d '{"username":"tadmin_a_'$RANDOM'","password":"test1234","role":"admin","tenant_id":"'$T1_ID'"}')
TA1_TOKEN=$(echo "$TA1_RES" | jq -r '.token // empty')
check "Create tenant-admin for Tenant A" "true" "$([ -n "$TA1_TOKEN" ] && echo true || echo false)"
TA1_AUTH="Authorization: Bearer $TA1_TOKEN"

TA2_RES=$(curl -s -X POST "$API/auth/register" \
    -H "Content-Type: application/json" \
    -H "$SA_AUTH" \
    -d '{"username":"tadmin_b_'$RANDOM'","password":"test1234","role":"admin","tenant_id":"'$T2_ID'"}')
TA2_TOKEN=$(echo "$TA2_RES" | jq -r '.token // empty')
check "Create tenant-admin for Tenant B" "true" "$([ -n "$TA2_TOKEN" ] && echo true || echo false)"
TA2_AUTH="Authorization: Bearer $TA2_TOKEN"

# ── 4. Create warehouses per tenant ──
echo ""
echo "── Step 4: Create Warehouses per Tenant ──"

WH1_RES=$(curl -s -X POST "$API/warehouses" \
    -H "Content-Type: application/json" \
    -H "$TA1_AUTH" \
    -d '{"code":"WH-TA-1","name":"Warehouse Tenant A"}')
WH1_ID=$(echo "$WH1_RES" | jq -r '.id // empty')
check "Create warehouse for Tenant A" "true" "$([ -n "$WH1_ID" ] && echo true || echo false)"

WH2_RES=$(curl -s -X POST "$API/warehouses" \
    -H "Content-Type: application/json" \
    -H "$TA2_AUTH" \
    -d '{"code":"WH-TB-1","name":"Warehouse Tenant B"}')
WH2_ID=$(echo "$WH2_RES" | jq -r '.id // empty')
check "Create warehouse for Tenant B" "true" "$([ -n "$WH2_ID" ] && echo true || echo false)"

# ── 5. Cross-tenant isolation: Tenant A should NOT see Tenant B's warehouses ──
echo ""
echo "── Step 5: Cross-Tenant Isolation ──"

A_WH_LIST=$(curl -s -X GET "$API/warehouses" -H "$TA1_AUTH")
A_WH_IDS=$(echo "$A_WH_LIST" | jq -r '[.data[]?.id] | join(",")')
check "Tenant A cannot see Tenant B warehouse" "false" "$(echo "$A_WH_IDS" | grep -q "$WH2_ID" && echo true || echo false)"

B_WH_LIST=$(curl -s -X GET "$API/warehouses" -H "$TA2_AUTH")
B_WH_IDS=$(echo "$B_WH_LIST" | jq -r '[.data[]?.id] | join(",")')
check "Tenant B cannot see Tenant A warehouse" "false" "$(echo "$B_WH_IDS" | grep -q "$WH1_ID" && echo true || echo false)"

# ── 6. Superadmin can see ALL tenants' warehouses ──
echo ""
echo "── Step 6: Superadmin Global Access ──"

SA_WH_LIST=$(curl -s -X GET "$API/warehouses" -H "$SA_AUTH" -H "X-Warehouse-Id: ALL")
SA_WH_COUNT=$(echo "$SA_WH_LIST" | jq -r '.data | length')
check "Superadmin sees all warehouses" "true" "$([ "$SA_WH_COUNT" -ge 2 ] && echo true || echo false)"

# ── 7. Superadmin tenant CRUD ──
echo ""
echo "── Step 7: Superadmin Tenant Management ──"

# Update tenant
UPD_RES=$(curl -s -w "\n%{http_code}" -X PUT "$API/tenants/$T1_ID" \
    -H "Content-Type: application/json" \
    -H "$SA_AUTH" \
    -d '{"code":"TEN-TEST-A","name":"Test Tenant A (Updated)","plan":"PRO"}')
UPD_CODE=$(echo "$UPD_RES" | tail -1)
check "Superadmin can update tenant" "200" "$UPD_CODE"

# Get tenant by ID
GET_RES=$(curl -s -w "\n%{http_code}" -X GET "$API/tenants/$T1_ID" -H "$SA_AUTH")
GET_CODE=$(echo "$GET_RES" | tail -1)
check "Superadmin can get tenant by ID" "200" "$GET_CODE"

# ── 8. Tenant admin cannot access tenant endpoints ──
echo ""
echo "── Step 8: RBAC — Tenant Admin Restrictions ──"

TA_TENANT_RES=$(curl -s -w "\n%{http_code}" -X GET "$API/tenants" -H "$TA1_AUTH")
TA_TENANT_CODE=$(echo "$TA_TENANT_RES" | tail -1)
check "Tenant admin blocked from /tenants (not 200)" "true" "$([ "$TA_TENANT_CODE" != "200" ] && echo true || echo false)"

# ── Cleanup: Delete test tenants ──
echo ""
echo "── Cleanup ──"

curl -s -X DELETE "$API/tenants/$T1_ID" -H "$SA_AUTH" > /dev/null 2>&1 || true
curl -s -X DELETE "$API/tenants/$T2_ID" -H "$SA_AUTH" > /dev/null 2>&1 || true
echo "  🧹 Cleaned up test tenants"

# ── Summary ──
echo ""
echo "═══════════════════════════════════════════════════════"
echo "  Results: $PASS/$TOTAL passed, $FAIL failed"
echo "═══════════════════════════════════════════════════════"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
echo "  🎉 All tests passed!"
