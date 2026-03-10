#!/usr/bin/env bash
# ============================================================
# Warehouse CRM — Seed Data Script
# ============================================================
# Creates: 1 admin user + 60 sample locations
#   Zones: A, B, C
#   Racks: R1..R5  (per zone)
#   Shelves: S1..S4 (per rack)
#
# Usage: ./scripts/seed-data.sh [BASE_URL]
# ============================================================

set -euo pipefail

BASE=${1:-http://localhost:3000}
API="${BASE}/api/v1"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "\n${CYAN}═══════════════════════════════════════════${NC}"
echo -e "${CYAN}  Warehouse CRM — Seed Data${NC}"
echo -e "${CYAN}  Target: ${BASE}${NC}"
echo -e "${CYAN}═══════════════════════════════════════════${NC}\n"

# ─── 1. Create Admin User ────────────────────────────────────
echo -e "${YELLOW}[1/3] Creating admin user...${NC}"
REG=$(curl -s -w "\n%{http_code}" -X POST "${API}/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123","role":"admin"}')
REG_CODE=$(echo "$REG" | tail -1)
if [ "$REG_CODE" = "201" ]; then
  echo -e "  ${GREEN}✓${NC} Admin user created (admin / admin123)"
elif [ "$REG_CODE" = "409" ]; then
  echo -e "  ${YELLOW}⊘${NC} Admin user already exists — skipping"
else
  echo -e "  ${RED}✗${NC} Unexpected response: HTTP $REG_CODE"
fi

# ─── 2. Login ────────────────────────────────────────────────
echo -e "\n${YELLOW}[2/3] Logging in...${NC}"
LOGIN=$(curl -s -X POST "${API}/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}')
TOKEN=$(echo "$LOGIN" | jq -r '.token')
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo -e "  ${RED}✗ Login failed. Cannot seed data.${NC}"
  echo "  Response: $LOGIN"
  exit 1
fi
echo -e "  ${GREEN}✓${NC} Logged in successfully"

AUTH="Authorization: Bearer ${TOKEN}"

# ─── 3. Create Sample Locations ──────────────────────────────
echo -e "\n${YELLOW}[3/3] Creating 60 sample locations (3 zones × 5 racks × 4 shelves)...${NC}"

CREATED=0
SKIPPED=0
FAILED=0

for ZONE in A B C; do
  for RACK in 1 2 3 4 5; do
    for SHELF in 1 2 3 4; do
      CODE="${ZONE}-R${RACK}-S${SHELF}"
      NAME="Zone ${ZONE} Rack ${RACK} Shelf ${SHELF}"

      RESP=$(curl -s -w "\n%{http_code}" -X POST "${API}/locations" \
        -H "Content-Type: application/json" \
        -H "$AUTH" \
        -d "{\"code\":\"${CODE}\",\"name\":\"${NAME}\",\"zone\":\"${ZONE}\",\"aisle\":\"${RACK}\",\"rack\":\"R${RACK}\",\"level\":\"S${SHELF}\"}")

      HTTP_CODE=$(echo "$RESP" | tail -1)
      if [ "$HTTP_CODE" = "201" ]; then
        CREATED=$((CREATED + 1))
      elif [ "$HTTP_CODE" = "409" ]; then
        SKIPPED=$((SKIPPED + 1))
      else
        FAILED=$((FAILED + 1))
      fi
    done
  done
done

echo -e "  ${GREEN}✓ Created: $CREATED${NC}  |  Skipped: $SKIPPED  |  Failed: $FAILED"

# ─── Summary ─────────────────────────────────────────────────
echo -e "\n${CYAN}═══════════════════════════════════════════${NC}"
echo -e "  Seed data complete!"
echo -e "  Admin: admin / admin123"
echo -e "  Locations: $((CREATED + SKIPPED)) total"
echo -e "${CYAN}═══════════════════════════════════════════${NC}\n"
