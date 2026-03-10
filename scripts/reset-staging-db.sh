#!/usr/bin/env bash
# ═══════════════════════════════════════════════════════════════
#  reset-staging-db.sh — Drop staging database for clean smoke tests
# ═══════════════════════════════════════════════════════════════
#  SAFETY:
#    - Requires ALLOW_DB_RESET=true
#    - Requires --yes-i-am-sure flag
#    - Aborts if ENVIRONMENT=production
#
#  Usage:
#    ALLOW_DB_RESET=true ./scripts/reset-staging-db.sh --yes-i-am-sure
# ═══════════════════════════════════════════════════════════════
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# ── Safety checks ──

if [ "${ENVIRONMENT:-}" = "production" ]; then
    echo -e "${RED}ABORT: ENVIRONMENT=production — DB reset is NEVER allowed in production.${NC}"
    exit 1
fi

if [ "${ALLOW_DB_RESET:-}" != "true" ]; then
    echo -e "${RED}ABORT: Set ALLOW_DB_RESET=true to enable DB reset.${NC}"
    echo "  This is a destructive operation. Only use in staging/test."
    exit 1
fi

if [ "${1:-}" != "--yes-i-am-sure" ]; then
    echo -e "${YELLOW}WARNING: This will DROP the entire staging database.${NC}"
    echo "  Run with --yes-i-am-sure to confirm:"
    echo "    ALLOW_DB_RESET=true $0 --yes-i-am-sure"
    exit 1
fi

# ── Load config ──

# Source .env if it exists
if [ -f .env ]; then
    set -a; source .env; set +a
fi

DB_NAME="${DB_NAME:-warehouse_crm}"
MONGO_URI="${MONGO_URI:-mongodb://localhost:27017}"

echo -e "${YELLOW}══════════════════════════════════════════${NC}"
echo -e "${YELLOW}  DROPPING DATABASE: ${DB_NAME}${NC}"
echo -e "${YELLOW}  URI: ${MONGO_URI}${NC}"
echo -e "${YELLOW}══════════════════════════════════════════${NC}"

# ── Drop database ──

if command -v mongosh &>/dev/null; then
    mongosh "$MONGO_URI" --eval "db.getSiblingDB('$DB_NAME').dropDatabase()" --quiet
elif command -v mongo &>/dev/null; then
    mongo "$MONGO_URI" --eval "db.getSiblingDB('$DB_NAME').dropDatabase()" --quiet
else
    echo -e "${RED}ABORT: Neither mongosh nor mongo CLI found.${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Database '$DB_NAME' dropped successfully.${NC}"
echo -e "  Restart the server to re-create default tenant and warehouse."
