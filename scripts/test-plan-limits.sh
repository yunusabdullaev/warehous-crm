#!/usr/bin/env bash
#
# Plan Limits + Feature Flags Integration Test Script
#
# Usage:
#   ./scripts/test-plan-limits.sh                   # default: http://localhost:3003
#   ./scripts/test-plan-limits.sh https://api.com    # custom base URL
#
# Prerequisites: jq, curl, running server

set -euo pipefail

BASE="${1:-http://localhost:3003}"
API="$BASE/api/v1"

PASS=0
FAIL=0
TOTAL=0

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

echo ""
echo "═══════════════════════════════════════════════════════"
echo "  Plan Limits + Feature Flags Tests"
echo "  Base URL: $API"
echo "═══════════════════════════════════════════════════════"
echo ""

# ── Step 0: Get superadmin token ──
echo "── Step 0: Setup ──"

SA_RES=$(curl -s -X POST "$API/auth/register" \
    -H "Content-Type: application/json" \
    -d '{"username":"sa_plan_'$RANDOM'","password":"test1234","role":"superadmin"}')
SA_TOKEN=$(echo "$SA_RES" | jq -r '.token // empty')

if [ -z "$SA_TOKEN" ]; then
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

# ── Step 1: Create FREE tenant with maxUsers=1 ──
echo ""
echo "── Step 1: Create FREE tenant with maxUsers=1 ──"

T_RES=$(curl -s -X POST "$API/tenants" \
    -H "Content-Type: application/json" \
    -H "$SA_AUTH" \
    -d '{
        "code":"TEN-PLAN-TEST-'$RANDOM'",
        "name":"Plan Test Tenant",
        "plan":"FREE",
        "status":"ACTIVE",
        "limits":{"max_warehouses":1,"max_users":1,"max_products":500,"max_daily_orders":50},
        "features":{"enable_reports":false,"enable_expiry_digest":false,"enable_qr_labels":false,"enable_returns":false,"enable_lots":false,"enable_multi_warehouse":false,"enable_api_export":false}
    }')
T_ID=$(echo "$T_RES" | jq -r '.id // empty')
T_STATUS=$(echo "$T_RES" | jq -r '.status // empty')
T_MAX_USERS=$(echo "$T_RES" | jq -r '.limits.max_users // empty')
check "Tenant created" "true" "$([ -n "$T_ID" ] && echo true || echo false)"
check "Tenant status is ACTIVE" "ACTIVE" "$T_STATUS"
check "Tenant maxUsers = 1" "1" "$T_MAX_USERS"

# ── Step 2: Create first user → should succeed ──
echo ""
echo "── Step 2: Create first user (should succeed) ──"

U1_RES=$(curl -s -w "\n%{http_code}" -X POST "$API/auth/register" \
    -H "Content-Type: application/json" \
    -H "$SA_AUTH" \
    -d '{"username":"planuser1_'$RANDOM'","password":"test1234","role":"operator","tenant_id":"'$T_ID'"}')
U1_CODE=$(echo "$U1_RES" | tail -1)
U1_BODY=$(echo "$U1_RES" | head -1)
U1_TOKEN=$(echo "$U1_BODY" | jq -r '.token // empty')
check "First user created (201)" "201" "$U1_CODE"
check "First user got token" "true" "$([ -n "$U1_TOKEN" ] && echo true || echo false)"

# ── Step 3: Create second user → should be blocked with PLAN_LIMIT ──
echo ""
echo "── Step 3: Create second user (should be blocked) ──"

U2_RES=$(curl -s -w "\n%{http_code}" -X POST "$API/auth/register" \
    -H "Content-Type: application/json" \
    -H "$SA_AUTH" \
    -d '{"username":"planuser2_'$RANDOM'","password":"test1234","role":"operator","tenant_id":"'$T_ID'"}')
U2_CODE=$(echo "$U2_RES" | tail -1)
U2_BODY=$(echo "$U2_RES" | head -1)
U2_ERROR=$(echo "$U2_BODY" | jq -r '.error // empty')
U2_LIMIT=$(echo "$U2_BODY" | jq -r '.limit // empty')
check "Second user blocked (402)" "402" "$U2_CODE"
check "Error is PLAN_LIMIT" "PLAN_LIMIT" "$U2_ERROR"
check "Limit field is maxUsers" "maxUsers" "$U2_LIMIT"

# ── Step 4: Disable enableReports → calling /reports should return FEATURE_DISABLED ──
echo ""
echo "── Step 4: Feature flag — reports disabled ──"

# Create a tenant-admin user with a warehouse so we can call /reports
# First create a tenant-admin
TA_RES=$(curl -s -X POST "$API/auth/register" \
    -H "Content-Type: application/json" \
    -H "$SA_AUTH" \
    -d '{"username":"tadmin_plan_'$RANDOM'","password":"test1234","role":"admin"}')
TA_TOKEN=$(echo "$TA_RES" | jq -r '.token // empty')
# We need this user to be in the test tenant. Update tenant to allow 5 users first
curl -s -X PUT "$API/tenants/$T_ID" \
    -H "Content-Type: application/json" \
    -H "$SA_AUTH" \
    -d '{"limits":{"max_warehouses":1,"max_users":5,"max_products":500,"max_daily_orders":50}}' > /dev/null

# Register admin in the test tenant
TA2_RES=$(curl -s -X POST "$API/auth/register" \
    -H "Content-Type: application/json" \
    -H "$SA_AUTH" \
    -d '{"username":"tadmin_plan2_'$RANDOM'","password":"test1234","role":"admin","tenant_id":"'$T_ID'"}')
TA2_TOKEN=$(echo "$TA2_RES" | jq -r '.token // empty')
TA2_AUTH="Authorization: Bearer $TA2_TOKEN"

# Create a warehouse for the tenant admin to use
WH_RES=$(curl -s -X POST "$API/warehouses" \
    -H "Content-Type: application/json" \
    -H "$TA2_AUTH" \
    -d '{"code":"WH-PLAN-TEST","name":"Plan Test WH"}')
WH_ID=$(echo "$WH_RES" | jq -r '.id // empty')

# Now try to access reports (features.enable_reports = false)
if [ -n "$WH_ID" ] && [ "$WH_ID" != "null" ]; then
    RPT_RES=$(curl -s -w "\n%{http_code}" -X GET "$API/reports/stock" \
        -H "$TA2_AUTH" \
        -H "X-Warehouse-Id: $WH_ID")
    RPT_CODE=$(echo "$RPT_RES" | tail -1)
    RPT_BODY=$(echo "$RPT_RES" | head -1)
    RPT_ERROR=$(echo "$RPT_BODY" | jq -r '.error // empty')
    check "Reports blocked (403)" "403" "$RPT_CODE"
    check "Error is FEATURE_DISABLED" "FEATURE_DISABLED" "$RPT_ERROR"
else
    echo "  ⚠️  Skipped: could not create warehouse"
    TOTAL=$((TOTAL + 2))
    FAIL=$((FAIL + 2))
fi

# ── Step 5: Suspend tenant → normal user gets blocked everywhere ──
echo ""
echo "── Step 5: Suspend tenant → all blocked ──"

curl -s -X PUT "$API/tenants/$T_ID" \
    -H "Content-Type: application/json" \
    -H "$SA_AUTH" \
    -d '{"status":"SUSPENDED"}' > /dev/null

# Wait for cache to expire (or call immediately — depends on TTL)
sleep 2

# Try to access any endpoint as tenant-admin
if [ -n "$WH_ID" ] && [ "$WH_ID" != "null" ]; then
    SUSP_RES=$(curl -s -w "\n%{http_code}" -X GET "$API/products" \
        -H "$TA2_AUTH" \
        -H "X-Warehouse-Id: $WH_ID")
    SUSP_CODE=$(echo "$SUSP_RES" | tail -1)
    SUSP_BODY=$(echo "$SUSP_RES" | head -1)
    SUSP_ERROR=$(echo "$SUSP_BODY" | jq -r '.error // empty')
    check "Suspended tenant blocked (403)" "403" "$SUSP_CODE"
    check "Error is TENANT_SUSPENDED" "TENANT_SUSPENDED" "$SUSP_ERROR"
else
    echo "  ⚠️  Skipped: no warehouse available"
    TOTAL=$((TOTAL + 2))
    FAIL=$((FAIL + 2))
fi

# ── Step 6: Reactivate + upgrade to PRO → same actions succeed ──
echo ""
echo "── Step 6: Upgrade to PRO → features enabled ──"

# Reactivate tenant and upgrade
curl -s -X PUT "$API/tenants/$T_ID/plan" \
    -H "Content-Type: application/json" \
    -H "$SA_AUTH" \
    -d '{"plan":"PRO"}' > /dev/null

curl -s -X PUT "$API/tenants/$T_ID" \
    -H "Content-Type: application/json" \
    -H "$SA_AUTH" \
    -d '{"status":"ACTIVE"}' > /dev/null

sleep 2

# Re-login as tenant-admin to get fresh cache state
TA3_RES=$(curl -s -X POST "$API/auth/register" \
    -H "Content-Type: application/json" \
    -H "$SA_AUTH" \
    -d '{"username":"tadmin_pro_'$RANDOM'","password":"test1234","role":"admin","tenant_id":"'$T_ID'"}')
TA3_TOKEN=$(echo "$TA3_RES" | jq -r '.token // empty')
TA3_AUTH="Authorization: Bearer $TA3_TOKEN"

if [ -n "$WH_ID" ] && [ "$WH_ID" != "null" ] && [ -n "$TA3_TOKEN" ]; then
    # Reports should now work (PRO has enable_reports=true)
    RPT2_RES=$(curl -s -w "\n%{http_code}" -X GET "$API/reports/stock" \
        -H "$TA3_AUTH" \
        -H "X-Warehouse-Id: $WH_ID")
    RPT2_CODE=$(echo "$RPT2_RES" | tail -1)
    check "PRO: reports now accessible" "true" "$([ "$RPT2_CODE" != "403" ] && echo true || echo false)"

    # Products endpoint accessible (not suspended)
    PROD_RES=$(curl -s -w "\n%{http_code}" -X GET "$API/products" \
        -H "$TA3_AUTH" \
        -H "X-Warehouse-Id: $WH_ID")
    PROD_CODE=$(echo "$PROD_RES" | tail -1)
    check "PRO: products accessible" "200" "$PROD_CODE"
else
    echo "  ⚠️  Skipped: no auth or warehouse"
    TOTAL=$((TOTAL + 2))
    FAIL=$((FAIL + 2))
fi

# ── Step 7: Check tenant usage API ──
echo ""
echo "── Step 7: Tenant Usage API ──"

USAGE_RES=$(curl -s -w "\n%{http_code}" -X GET "$API/tenants/$T_ID/usage" -H "$SA_AUTH")
USAGE_CODE=$(echo "$USAGE_RES" | tail -1)
USAGE_BODY=$(echo "$USAGE_RES" | head -1)
USAGE_USERS=$(echo "$USAGE_BODY" | jq -r '.users // 0')
check "Usage endpoint returns 200" "200" "$USAGE_CODE"
check "Usage shows users > 0" "true" "$([ "$USAGE_USERS" -gt 0 ] 2>/dev/null && echo true || echo false)"

# ── Cleanup ──
echo ""
echo "── Cleanup ──"
curl -s -X DELETE "$API/tenants/$T_ID" -H "$SA_AUTH" > /dev/null 2>&1 || true
echo "  🧹 Cleaned up test tenant"

# ── Summary ──
echo ""
echo "═══════════════════════════════════════════════════════"
echo "  Results: $PASS/$TOTAL passed, $FAIL failed"
echo "═══════════════════════════════════════════════════════"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
echo "  🎉 All tests passed!"
