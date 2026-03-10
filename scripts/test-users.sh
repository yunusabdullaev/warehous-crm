#!/usr/bin/env bash
# ── User Management API Test Script ──
# Usage: bash scripts/test-users.sh http://localhost:3003

set -euo pipefail

BASE="${1:-http://localhost:3003}"
API="$BASE/api/v1"
PASS=0 FAIL=0

green() { printf "\033[32m✓ %s\033[0m\n" "$1"; PASS=$((PASS+1)); }
red()   { printf "\033[31m✗ %s\033[0m\n" "$1"; FAIL=$((FAIL+1)); }
check() { [ "$1" = "$2" ] && green "$3" || red "$3 (expected $2, got $1)"; }

echo "══════════════════════════════════════"
echo " User Management Tests"
echo "══════════════════════════════════════"

# ── Setup: register admin ──
ADMIN_RESP=$(curl -s -X POST "$API/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"usrmgmt_admin","password":"admin123","role":"admin"}')
ADMIN_TOKEN=$(echo "$ADMIN_RESP" | jq -r '.token // empty')
if [ -z "$ADMIN_TOKEN" ]; then
  ADMIN_RESP=$(curl -s -X POST "$API/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"usrmgmt_admin","password":"admin123"}')
  ADMIN_TOKEN=$(echo "$ADMIN_RESP" | jq -r '.token')
fi
ADMIN_ID=$(echo "$ADMIN_RESP" | jq -r '.user.id')
echo "Admin token: ${ADMIN_TOKEN:0:20}..."

# ── Register operator for RBAC test ──
OP_RESP=$(curl -s -X POST "$API/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"usrmgmt_oper","password":"oper1234","role":"operator"}')
OP_TOKEN=$(echo "$OP_RESP" | jq -r '.token // empty')
if [ -z "$OP_TOKEN" ]; then
  OP_RESP=$(curl -s -X POST "$API/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"usrmgmt_oper","password":"oper1234"}')
  OP_TOKEN=$(echo "$OP_RESP" | jq -r '.token')
fi

AUTH="Authorization: Bearer $ADMIN_TOKEN"

# ── 1. List users (admin) ──
echo ""
echo "── 1. List Users (admin) ──"
HTTP=$(curl -s -o /dev/null -w '%{http_code}' "$API/users" -H "$AUTH")
check "$HTTP" "200" "GET /users → 200"

BODY=$(curl -s "$API/users?limit=50" -H "$AUTH")
COUNT=$(echo "$BODY" | jq '.data | length')
echo "   Users in system: $COUNT"

# ── 2. Create user via register ──
echo ""
echo "── 2. Create User ──"
CREATE_RESP=$(curl -s -w '\n%{http_code}' -X POST "$API/auth/register" \
  -H "Content-Type: application/json" -H "$AUTH" \
  -d '{"username":"test_user_crud","password":"test123456","role":"loader"}')
CREATE_CODE=$(echo "$CREATE_RESP" | tail -1)
CREATE_BODY=$(echo "$CREATE_RESP" | head -1)
check "$CREATE_CODE" "201" "POST /auth/register → 201"

NEW_USER_ID=$(echo "$CREATE_BODY" | jq -r '.user.id')
echo "   Created user ID: $NEW_USER_ID"

# ── 3. Get user by ID ──
echo ""
echo "── 3. Get User by ID ──"
HTTP=$(curl -s -o /dev/null -w '%{http_code}' "$API/users/$NEW_USER_ID" -H "$AUTH")
check "$HTTP" "200" "GET /users/:id → 200"

GET_BODY=$(curl -s "$API/users/$NEW_USER_ID" -H "$AUTH")
GOT_ROLE=$(echo "$GET_BODY" | jq -r '.role')
check "$GOT_ROLE" "loader" "Role is loader"

# ── 4. Update role ──
echo ""
echo "── 4. Update User Role ──"
UP_RESP=$(curl -s -w '\n%{http_code}' -X PUT "$API/users/$NEW_USER_ID" \
  -H "Content-Type: application/json" -H "$AUTH" \
  -d '{"role":"operator"}')
UP_CODE=$(echo "$UP_RESP" | tail -1)
check "$UP_CODE" "200" "PUT /users/:id (role) → 200"

UP_BODY=$(curl -s "$API/users/$NEW_USER_ID" -H "$AUTH")
NEW_ROLE=$(echo "$UP_BODY" | jq -r '.role')
check "$NEW_ROLE" "operator" "Role updated to operator"

# ── 5. Reset password ──
echo ""
echo "── 5. Reset Password ──"
PW_RESP=$(curl -s -o /dev/null -w '%{http_code}' -X PUT "$API/users/$NEW_USER_ID" \
  -H "Content-Type: application/json" -H "$AUTH" \
  -d '{"password":"newpass789"}')
check "$PW_RESP" "200" "PUT /users/:id (password) → 200"

LOGIN_RESP=$(curl -s -w '\n%{http_code}' -X POST "$API/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"test_user_crud","password":"newpass789"}')
LOGIN_CODE=$(echo "$LOGIN_RESP" | tail -1)
check "$LOGIN_CODE" "200" "Login with new password → 200"

# ── 6. Delete user ──
echo ""
echo "── 6. Delete User ──"
DEL_RESP=$(curl -s -o /dev/null -w '%{http_code}' -X DELETE "$API/users/$NEW_USER_ID" -H "$AUTH")
check "$DEL_RESP" "200" "DELETE /users/:id → 200"

GET_DEL=$(curl -s -o /dev/null -w '%{http_code}' "$API/users/$NEW_USER_ID" -H "$AUTH")
check "$GET_DEL" "404" "GET deleted user → 404"

# ── 7. Self-delete prevention ──
echo ""
echo "── 7. Self-Delete Prevention ──"
SELF_DEL=$(curl -s -o /dev/null -w '%{http_code}' -X DELETE "$API/users/$ADMIN_ID" -H "$AUTH")
check "$SELF_DEL" "403" "DELETE self → 403"

# ── 8. RBAC: operator cannot access /users ──
echo ""
echo "── 8. RBAC: Operator Cannot Manage Users ──"
OP_AUTH="Authorization: Bearer $OP_TOKEN"
OP_LIST=$(curl -s -o /dev/null -w '%{http_code}' "$API/users" -H "$OP_AUTH")
check "$OP_LIST" "403" "GET /users (operator) → 403"

# ── Summary ──
echo ""
echo "══════════════════════════════════════"
echo " Results: $PASS passed, $FAIL failed"
echo "══════════════════════════════════════"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
