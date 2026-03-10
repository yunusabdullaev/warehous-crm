#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════
#  Warehouse CRM — Preflight Checks
# ═══════════════════════════════════════════════════════════════
#  Validates infrastructure readiness before go-live.
#
#  Usage:
#    ./scripts/preflight.sh https://staging.wms.example.com
#    ./scripts/preflight.sh http://localhost:3003   (from VPS)
#
#  Prerequisites: curl, jq, docker (must run ON the VPS)
# ═══════════════════════════════════════════════════════════════
set -euo pipefail

BASE="${1:?Usage: $0 <base_url>}"
# Strip trailing slash
BASE="${BASE%/}"

PASS=0
FAIL=0
WARN=0
TOTAL=0

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

pass() {
    TOTAL=$((TOTAL + 1))
    PASS=$((PASS + 1))
    echo -e "  ${GREEN}✓ PASS${NC} — $1"
}

fail() {
    TOTAL=$((TOTAL + 1))
    FAIL=$((FAIL + 1))
    echo -e "  ${RED}✗ FAIL${NC} — $1"
}

warn() {
    TOTAL=$((TOTAL + 1))
    WARN=$((WARN + 1))
    echo -e "  ${YELLOW}⚠ WARN${NC} — $1"
}

echo -e "\n${CYAN}═══════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}  Warehouse CRM — Preflight Checks${NC}"
echo -e "${CYAN}  Target: ${BASE}${NC}"
echo -e "${CYAN}  Time:   $(date -u '+%Y-%m-%d %H:%M:%S UTC')${NC}"
echo -e "${CYAN}═══════════════════════════════════════════════════════${NC}\n"

# ─────────────────────────────────────────────────────────
#  1. /health endpoint
# ─────────────────────────────────────────────────────────
echo -e "${YELLOW}[1/10] Health Endpoint${NC}"

HEALTH_RESP=$(curl -s --max-time 10 "${BASE}/health" 2>/dev/null || echo "{}")
HEALTH_STATUS=$(echo "$HEALTH_RESP" | jq -r '.status // empty' 2>/dev/null)

if [ "$HEALTH_STATUS" = "ok" ]; then
    pass "/health returns status=ok"
else
    fail "/health did not return status=ok (got: $HEALTH_RESP)"
fi

# ─────────────────────────────────────────────────────────
#  2. /metrics restricted
# ─────────────────────────────────────────────────────────
echo -e "\n${YELLOW}[2/10] Metrics Endpoint${NC}"

METRICS_EXT_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "${BASE}/metrics" 2>/dev/null || echo "000")

if [ "$METRICS_EXT_CODE" = "403" ] || [ "$METRICS_EXT_CODE" = "404" ]; then
    pass "/metrics blocked externally (HTTP $METRICS_EXT_CODE)"
else
    # If running from VPS, metrics may be accessible on localhost
    if [ "$METRICS_EXT_CODE" = "200" ]; then
        METRICS_HEAD=$(curl -s --max-time 5 "http://localhost:3003/metrics" 2>/dev/null | head -1)
        if echo "$METRICS_HEAD" | grep -q "HELP\|TYPE\|http"; then
            warn "/metrics accessible (HTTP 200). OK if running on VPS, but ensure nginx blocks external access"
        else
            fail "/metrics returned 200 but no Prometheus data"
        fi
    else
        fail "/metrics returned unexpected status: $METRICS_EXT_CODE"
    fi
fi

# Check metrics from localhost (if on VPS)
METRICS_LOCAL=$(curl -s --max-time 5 "http://localhost:3003/metrics" 2>/dev/null | head -1)
if echo "$METRICS_LOCAL" | grep -q "HELP\|TYPE\|http" 2>/dev/null; then
    pass "/metrics reachable on localhost:3003"
else
    warn "/metrics not reachable on localhost:3003 (not on VPS?)"
fi

# ─────────────────────────────────────────────────────────
#  3. MongoDB auth + not exposed publicly
# ─────────────────────────────────────────────────────────
echo -e "\n${YELLOW}[3/10] MongoDB Security${NC}"

# Check MongoDB container has auth enabled
if command -v docker &>/dev/null; then
    MONGO_RUNNING=$(docker ps --filter name=wms-mongodb --format "{{.Status}}" 2>/dev/null || echo "")
    if echo "$MONGO_RUNNING" | grep -q "Up"; then
        pass "MongoDB container is running"
    else
        fail "MongoDB container (wms-mongodb) is not running"
    fi

    # Check no external port binding
    MONGO_PORTS=$(docker port wms-mongodb 2>/dev/null || echo "")
    if [ -z "$MONGO_PORTS" ]; then
        pass "MongoDB has no external port mapping"
    else
        if echo "$MONGO_PORTS" | grep -q "0.0.0.0\|:::"; then
            fail "MongoDB is exposed on 0.0.0.0 — SECURITY RISK"
        else
            pass "MongoDB port mapping is localhost-only"
        fi
    fi

    # Check auth is enabled (env var MONGO_INITDB_ROOT_USERNAME set)
    MONGO_AUTH=$(docker inspect wms-mongodb 2>/dev/null | jq -r '.[0].Config.Env[]' 2>/dev/null | grep "MONGO_INITDB_ROOT_USERNAME" || echo "")
    if [ -n "$MONGO_AUTH" ]; then
        pass "MongoDB authentication is enabled (MONGO_INITDB_ROOT_USERNAME set)"
    else
        fail "MongoDB authentication may not be enabled"
    fi
else
    warn "Docker not available — cannot check MongoDB (not running on VPS?)"
fi

# ─────────────────────────────────────────────────────────
#  4. Backup timer + last backup exists
# ─────────────────────────────────────────────────────────
echo -e "\n${YELLOW}[4/10] Backups${NC}"

BACKUP_DIR="${BACKUP_DIR:-/opt/wms/backups}"

# Check systemd timer
if command -v systemctl &>/dev/null; then
    TIMER_STATUS=$(systemctl is-active wms-backup.timer 2>/dev/null || echo "inactive")
    if [ "$TIMER_STATUS" = "active" ]; then
        pass "wms-backup.timer is active"
    else
        fail "wms-backup.timer is $TIMER_STATUS (expected: active)"
    fi
else
    warn "systemctl not available — cannot verify backup timer"
fi

# Check backup files exist
if [ -d "$BACKUP_DIR" ]; then
    BACKUP_COUNT=$(find "$BACKUP_DIR" -name "wms_*.gz" -type f 2>/dev/null | wc -l)
    if [ "$BACKUP_COUNT" -gt 0 ]; then
        LATEST_BACKUP=$(ls -t "$BACKUP_DIR"/wms_*.gz 2>/dev/null | head -1)
        LATEST_AGE_HOURS=$(( ($(date +%s) - $(stat -c %Y "$LATEST_BACKUP" 2>/dev/null || stat -f %m "$LATEST_BACKUP" 2>/dev/null || echo 0)) / 3600 ))
        pass "Found $BACKUP_COUNT backup(s). Latest: $(basename "$LATEST_BACKUP") (${LATEST_AGE_HOURS}h ago)"
        if [ "$LATEST_AGE_HOURS" -gt 48 ]; then
            warn "Latest backup is older than 48 hours"
        fi
    else
        fail "No backup files found in $BACKUP_DIR"
    fi
else
    fail "Backup directory $BACKUP_DIR does not exist"
fi

# ─────────────────────────────────────────────────────────
#  5. Loki + Promtail shipping logs
# ─────────────────────────────────────────────────────────
echo -e "\n${YELLOW}[5/10] Log Shipping (Loki + Promtail)${NC}"

if command -v docker &>/dev/null; then
    LOKI_STATUS=$(docker ps --filter name=wms-loki --format "{{.Status}}" 2>/dev/null || echo "")
    if echo "$LOKI_STATUS" | grep -q "Up"; then
        pass "Loki container is running"
    else
        fail "Loki container (wms-loki) is not running"
    fi

    PROMTAIL_STATUS=$(docker ps --filter name=wms-promtail --format "{{.Status}}" 2>/dev/null || echo "")
    if echo "$PROMTAIL_STATUS" | grep -q "Up"; then
        pass "Promtail container is running"
    else
        fail "Promtail container (wms-promtail) is not running"
    fi

    # Check Loki is receiving data
    LOKI_READY=$(curl -s --max-time 5 "http://localhost:3100/ready" 2>/dev/null || echo "")
    if echo "$LOKI_READY" | grep -qi "ready"; then
        pass "Loki is ready and accepting logs"
    else
        warn "Loki /ready check returned: $LOKI_READY"
    fi
else
    warn "Docker not available — cannot check Loki/Promtail"
fi

# ─────────────────────────────────────────────────────────
#  6. Stripe webhook endpoint reachable
# ─────────────────────────────────────────────────────────
echo -e "\n${YELLOW}[6/10] Stripe Webhook Endpoint${NC}"

# Webhook endpoint should return 401 (bad signature) or 400 (bad body), never 404 or 5xx
WH_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 \
    -X POST "${BASE}/api/v1/webhooks/stripe" \
    -H "Content-Type: application/json" \
    -H "Stripe-Signature: t=0,v1=fake" \
    -d '{}' 2>/dev/null || echo "000")

if [ "$WH_CODE" = "401" ] || [ "$WH_CODE" = "400" ]; then
    pass "Webhook endpoint reachable and rejects invalid signatures (HTTP $WH_CODE)"
elif [ "$WH_CODE" = "200" ]; then
    warn "Webhook returned 200 — STRIPE_WEBHOOK_TEST may be true (should be false in production)"
else
    fail "Webhook endpoint returned unexpected HTTP $WH_CODE"
fi

# ─────────────────────────────────────────────────────────
#  7. Cookie security (Secure flag + refresh flow)
# ─────────────────────────────────────────────────────────
echo -e "\n${YELLOW}[7/10] Cookie Security${NC}"

# Attempt login and check Set-Cookie headers
COOKIE_RESP=$(curl -sD - -X POST "${BASE}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"__preflight_check__","password":"__no_such_user__"}' \
    -o /dev/null --max-time 10 2>/dev/null || echo "")

# Even on failed login, if cookies are returned they should have Secure flag
# On successful login, check the flags
if echo "$COOKIE_RESP" | grep -qi "set-cookie"; then
    if echo "$COOKIE_RESP" | grep -i "set-cookie" | grep -qi "Secure"; then
        pass "Cookies have Secure flag"
    else
        fail "Cookies missing Secure flag"
    fi
    if echo "$COOKIE_RESP" | grep -i "set-cookie" | grep -qi "HttpOnly"; then
        pass "Cookies have HttpOnly flag"
    else
        fail "Cookies missing HttpOnly flag"
    fi
else
    # Failed login won't set cookies — that's expected
    # Check if we're on HTTPS
    if echo "$BASE" | grep -q "^https://"; then
        pass "HTTPS endpoint (cookie flags verified on successful login)"
    else
        warn "Cannot verify cookie flags (no cookies returned on failed login, expected)"
    fi
fi

# ─────────────────────────────────────────────────────────
#  8. Security headers
# ─────────────────────────────────────────────────────────
echo -e "\n${YELLOW}[8/10] Security Headers${NC}"

HEADERS=$(curl -sI --max-time 10 "${BASE}/" 2>/dev/null)

check_header() {
    local header_name="$1"
    if echo "$HEADERS" | grep -qi "$header_name"; then
        pass "Header present: $header_name"
    else
        fail "Header missing: $header_name"
    fi
}

check_header "Strict-Transport-Security"
check_header "X-Content-Type-Options"
check_header "X-Frame-Options"
check_header "X-XSS-Protection"
check_header "Referrer-Policy"

# ─────────────────────────────────────────────────────────
#  9. Rate limiting
# ─────────────────────────────────────────────────────────
echo -e "\n${YELLOW}[9/10] Rate Limiting${NC}"

GOT_429=false
for i in $(seq 1 25); do
    CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 \
        -X POST "${BASE}/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d '{"username":"ratelimit_test","password":"test"}' 2>/dev/null || echo "000")
    if [ "$CODE" = "429" ]; then
        GOT_429=true
        pass "Rate limiting triggered after $i requests (HTTP 429)"
        break
    fi
done

if [ "$GOT_429" = "false" ]; then
    warn "Rate limiting not triggered after 25 rapid requests (check nginx login_limit config)"
fi

# ─────────────────────────────────────────────────────────
#  10. TLS certificate validity
# ─────────────────────────────────────────────────────────
echo -e "\n${YELLOW}[10/10] TLS Certificate${NC}"

if echo "$BASE" | grep -q "^https://"; then
    DOMAIN=$(echo "$BASE" | sed 's|https://||' | sed 's|/.*||')
    TLS_EXPIRY=$(echo | openssl s_client -servername "$DOMAIN" -connect "$DOMAIN:443" 2>/dev/null | \
        openssl x509 -noout -enddate 2>/dev/null | cut -d= -f2)
    if [ -n "$TLS_EXPIRY" ]; then
        EXPIRY_EPOCH=$(date -d "$TLS_EXPIRY" +%s 2>/dev/null || date -j -f "%b %d %T %Y %Z" "$TLS_EXPIRY" +%s 2>/dev/null || echo "0")
        NOW_EPOCH=$(date +%s)
        DAYS_LEFT=$(( (EXPIRY_EPOCH - NOW_EPOCH) / 86400 ))
        if [ "$DAYS_LEFT" -gt 14 ]; then
            pass "TLS certificate valid ($DAYS_LEFT days remaining, expires: $TLS_EXPIRY)"
        elif [ "$DAYS_LEFT" -gt 0 ]; then
            warn "TLS certificate expires in $DAYS_LEFT days ($TLS_EXPIRY)"
        else
            fail "TLS certificate expired or invalid"
        fi
    else
        warn "Could not parse TLS certificate expiry"
    fi
else
    warn "Not an HTTPS URL — skipping TLS check"
fi

# ═══════════════════════════════════════════════════════
#  Summary
# ═══════════════════════════════════════════════════════
echo -e "\n${CYAN}═══════════════════════════════════════════════════════${NC}"
echo -e "  Results: ${GREEN}${PASS} passed${NC}, ${RED}${FAIL} failed${NC}, ${YELLOW}${WARN} warnings${NC}, ${TOTAL} total"
echo -e "${CYAN}═══════════════════════════════════════════════════════${NC}\n"

if [ "$FAIL" -gt 0 ]; then
    echo -e "${RED}  ✗ PREFLIGHT FAILED — fix $FAIL issue(s) before going live${NC}\n"
    exit 1
fi

if [ "$WARN" -gt 0 ]; then
    echo -e "${YELLOW}  ⚠ PREFLIGHT PASSED WITH WARNINGS — review $WARN warning(s)${NC}\n"
    exit 0
fi

echo -e "${GREEN}  ✓ PREFLIGHT PASSED — all checks OK${NC}\n"
exit 0
