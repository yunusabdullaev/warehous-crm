# Warehouse CRM — Go-Live Playbook

> **Version**: 1.0 · **Date**: 2026-02-21 · **Author**: SRE Team

---

## Quick Reference — Go-Live Checklist

```
PRE-DEPLOY
  □ DNS A records: staging.wms.example.com + wms.example.com
  □ VPS provisioned (2 vCPU / 4 GB RAM / 40 GB SSD minimum)
  □ JWT_SECRET rotated (openssl rand -base64 48)
  □ MONGO_PASSWORD generated (openssl rand -base64 32)
  □ Stripe: live keys + live price IDs + webhook secret
  □ COOKIE_DOMAIN, CORS_ORIGINS set for prod domain
  □ STRIPE_WEBHOOK_TEST=false

STAGING GATE
  □ scripts/preflight.sh → all PASS
  □ scripts/smoke-staging.sh → all PASS
  □ Manual: login → create order → full pick → ship
  □ Manual: billing checkout → test payment
  □ Monitoring: Prometheus targets UP, Loki logs flowing

PRODUCTION CUTOVER
  □ Maintenance window announced (if any)
  □ Backup taken BEFORE deploy
  □ docker compose up -d --build
  □ Health check: /health → ok
  □ Smoke: register/login → create tenant → create warehouse → product → inbound
  □ Stripe webhook test: stripe trigger payment_intent.succeeded
  □ Alerts verified: fire test alert → Grafana notification arrives
  □ Rollback plan rehearsed
```

---

## PART 1 — Environments

### Domain Layout

| Environment | Domain | Backend Port | API Base |
|-------------|--------|------|----------|
| **Staging** | `staging.wms.example.com` | 3003 | `https://staging.wms.example.com/api/v1` |
| **Production** | `wms.example.com` | 3003 | `https://wms.example.com/api/v1` |

### Environment Variable Differences

| Variable | Staging | Production |
|----------|---------|------------|
| `DOMAIN` | `staging.wms.example.com` | `wms.example.com` |
| `CORS_ORIGINS` | `https://staging.wms.example.com` | `https://wms.example.com` |
| `NEXT_PUBLIC_API_BASE_URL` | `https://staging.wms.example.com/api/v1` | `https://wms.example.com/api/v1` |
| `COOKIE_DOMAIN` | `staging.wms.example.com` | `wms.example.com` |
| `COOKIE_SECURE` | `true` | `true` |
| `COOKIE_SAMESITE` | `Lax` | `Lax` |
| `STRIPE_SECRET_KEY` | `sk_test_...` | `sk_live_...` |
| `STRIPE_WEBHOOK_SECRET` | `whsec_test_...` | `whsec_live_...` |
| `STRIPE_PRICE_PRO` | `price_test_pro_...` | `price_live_pro_...` |
| `STRIPE_PRICE_ENTERPRISE` | `price_test_ent_...` | `price_live_ent_...` |
| `STRIPE_WEBHOOK_TEST` | `true` *(for offline tests only)* | `false` *(NEVER true)* |
| `BILLING_SUCCESS_URL` | `https://staging.wms.example.com/billing?success=true` | `https://wms.example.com/billing?success=true` |
| `BILLING_CANCEL_URL` | `https://staging.wms.example.com/billing?canceled=true` | `https://wms.example.com/billing?canceled=true` |
| `JWT_SECRET` | unique per env | unique per env |
| `MONGO_PASSWORD` | unique per env | unique per env |
| `GF_ADMIN_PASSWORD` | staging password | strong production password |
| `ACCESS_TOKEN_TTL_MIN` | `15` | `15` |
| `REFRESH_TOKEN_TTL_DAYS` | `30` | `30` |

> **⚠️ CRITICAL**: Never share JWT_SECRET or MONGO_PASSWORD between staging and production. Never use `sk_live_` keys on staging.

---

## PART 2 — Staging Deploy Steps

### 2.1 — Provision VPS

```bash
# On a fresh Ubuntu 22.04 VPS (separate from production)
sudo apt update && sudo apt upgrade -y
sudo apt install -y curl git ufw fail2ban jq

# Create deploy user
sudo adduser deploy
sudo usermod -aG sudo deploy

# Install Docker
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker deploy
sudo systemctl enable docker && sudo systemctl start docker
docker --version && docker compose version
```

### 2.2 — Firewall

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp comment 'SSH'
sudo ufw allow 80/tcp comment 'HTTP'
sudo ufw allow 443/tcp comment 'HTTPS'
sudo ufw enable
```

### 2.3 — Clone & Configure

```bash
sudo mkdir -p /opt/wms && sudo chown deploy:deploy /opt/wms
cd /opt/wms
git clone https://github.com/YOUR_ORG/warehouse-crm.git .

cp deploy/.env.example deploy/.env

# Generate secrets
echo "JWT_SECRET: $(openssl rand -base64 48)"
echo "MONGO_PASSWORD: $(openssl rand -base64 32)"

# Edit .env with staging values
nano deploy/.env
```

**Staging `.env` template** (key differences):

```env
DOMAIN=staging.wms.example.com
CORS_ORIGINS=https://staging.wms.example.com
NEXT_PUBLIC_API_BASE_URL=https://staging.wms.example.com/api/v1
COOKIE_DOMAIN=staging.wms.example.com
COOKIE_SECURE=true
COOKIE_SAMESITE=Lax

# Stripe TEST mode
STRIPE_SECRET_KEY=sk_test_xxxxxxxxxxxxxxxx
STRIPE_WEBHOOK_SECRET=whsec_test_xxxxxxxxxxxxxxxx
STRIPE_PRICE_PRO=price_test_pro_xxxxxxxx
STRIPE_PRICE_ENTERPRISE=price_test_ent_xxxxxxxx
STRIPE_WEBHOOK_TEST=false
BILLING_SUCCESS_URL=https://staging.wms.example.com/billing?success=true
BILLING_CANCEL_URL=https://staging.wms.example.com/billing?canceled=true
```

### 2.4 — Nginx & TLS

```bash
sudo apt install -y nginx certbot python3-certbot-nginx

sudo cp deploy/nginx/wms.conf /etc/nginx/sites-available/wms.conf
sudo sed -i 's/wms.example.com/staging.wms.example.com/g' /etc/nginx/sites-available/wms.conf
sudo ln -sf /etc/nginx/sites-available/wms.conf /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default

sudo nginx -t && sudo systemctl reload nginx

# Obtain TLS cert (DNS must already point to this VPS)
sudo certbot --nginx -d staging.wms.example.com \
  --non-interactive --agree-tos -m admin@example.com

sudo certbot renew --dry-run
```

### 2.5 — Start Stack

```bash
cd /opt/wms

# Build and start
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env up -d --build

# Wait for health
sleep 30
docker compose -f deploy/docker-compose.prod.yml ps

# Expected: all 7 services Up (healthy)
```

### 2.6 — Run Migrations (if first deploy)

```bash
# Only if upgrading from pre-tenant system. Skip for fresh deploys.

# Build migration tools
docker exec wms-backend /bin/sh -c "cd /app && go build -o /tmp/migrate-warehouse cmd/migrate-warehouse/main.go" 2>/dev/null || true
docker exec wms-backend /bin/sh -c "cd /app && go build -o /tmp/migrate-tenant cmd/migrate-tenant/main.go" 2>/dev/null || true

# Or run from host if binaries exist:
# ./migrate-warehouse
# ./migrate-tenant
```

### 2.7 — Setup Backups

```bash
mkdir -p /opt/wms/backups
chmod +x deploy/scripts/backup-mongo.sh

sudo cp deploy/scripts/wms-backup.service /etc/systemd/system/
sudo cp deploy/scripts/wms-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable wms-backup.timer
sudo systemctl start wms-backup.timer

# Verify
systemctl list-timers | grep wms

# Manual test
sudo MONGO_PASSWORD="YOUR_PASS" MONGO_USER="wmsadmin" MONGO_DB="warehouse_crm" \
  /opt/wms/deploy/scripts/backup-mongo.sh
ls -la /opt/wms/backups/
```

### 2.8 — Verify Health

```bash
# Health
curl -s https://staging.wms.example.com/health | jq .
# → {"status":"ok","service":"warehouse-crm"}

# Frontend
curl -sI https://staging.wms.example.com | head -5
# → HTTP/2 200

# Security headers
curl -sI https://staging.wms.example.com | grep -i strict-transport
# → Strict-Transport-Security: max-age=63072000...

# Metrics (should be blocked externally)
curl -sI https://staging.wms.example.com/metrics
# → 403 Forbidden

# Grafana
echo "Open: https://staging.wms.example.com/grafana/"
echo "Add Prometheus source: http://prometheus:9090"
echo "Add Loki source: http://loki:3100"
```

### 2.9 — Setup Stripe Webhook (Staging)

```bash
# Install Stripe CLI
curl -s https://packages.stripe.com/api/security/keypair/stripe-cli-gpg/public | \
  gpg --dearmor | sudo tee /usr/share/keyrings/stripe.gpg >/dev/null
echo "deb [signed-by=/usr/share/keyrings/stripe.gpg] https://packages.stripe.com/stripe-cli-debian-local stable main" | \
  sudo tee /etc/apt/sources.list.d/stripe.list
sudo apt update && sudo apt install -y stripe

# Forward events (staging only!)
stripe listen --forward-to https://staging.wms.example.com/api/v1/webhooks/stripe
# Copy the whsec_... password and set as STRIPE_WEBHOOK_SECRET in .env
# Restart backend after updating
```

For staging testing with Stripe CLI, you can also set `STRIPE_WEBHOOK_TEST=true` temporarily. **Remember to set it back to `false` before production.**

---

## PART 3 — Automated Preflight Checks

> File: `scripts/preflight.sh`

See the full script at [preflight.sh](file:///Users/yunus/crm%20for%20warehous/scripts/preflight.sh).

Run:
```bash
./scripts/preflight.sh https://staging.wms.example.com
# For localhost: ./scripts/preflight.sh http://localhost:3003
```

The script checks:
1. `/health` returns `{"status":"ok"}`
2. `/metrics` is blocked externally (403) but reachable on localhost
3. MongoDB has authentication enabled and is not exposed publicly
4. Backup timer is enabled and at least one backup file exists
5. Loki container is running and Promtail is shipping logs
6. Stripe webhook endpoint responds (with 4xx, not 5xx)
7. Cookies have Secure flag set and refresh flow works via curl
8. Security headers present (HSTS, X-Content-Type-Options, etc.)
9. Rate limiting on login endpoint returns 429 after threshold
10. TLS certificate is valid

---

## PART 4 — E2E Smoke Suite for Staging

> File: `scripts/smoke-staging.sh`

See the full script at [smoke-staging.sh](file:///Users/yunus/crm%20for%20warehous/scripts/smoke-staging.sh).

Run:
```bash
./scripts/smoke-staging.sh https://staging.wms.example.com/api/v1
# For localhost: ./scripts/smoke-staging.sh http://localhost:3003/api/v1
```

The suite covers:
1. **Tenant lifecycle**: create PRO tenant → verify limits/features
2. **User management**: create admin + operator in tenant → assign warehouse
3. **Warehouse**: create warehouse for tenant
4. **Products & Locations**: create product + location
5. **Inbound**: inbound 500 units + verify stock
6. **Lots**: create lot with expiry date
7. **Order → Pick → Ship**: full workflow including scan-to-pick
8. **Return flow**: create RMA → add items → receive
9. **Billing**: checkout session returns a URL (Stripe test mode)
10. **Tenant suspension**: suspend → verify 403 → reactivate
11. **Cleanup**: delete test tenant

---

## PART 5 — Stripe Live Mode Cutover

### 5.1 — Pre-cutover Checklist

```
□ Stripe Dashboard: create LIVE products & prices
  → Note: price_live_pro_... and price_live_ent_...
□ Stripe Dashboard: Billing Portal enabled
  → Settings → Billing → Customer Portal → Enable
□ Stripe Dashboard: create LIVE webhook endpoint
  → URL: https://wms.example.com/api/v1/webhooks/stripe
  → Events: checkout.session.completed, customer.subscription.created,
            customer.subscription.updated, customer.subscription.deleted,
            invoice.payment_failed, invoice.payment_succeeded
  → Copy whsec_live_... signing secret
```

### 5.2 — Switch Environment Variables

```env
# BEFORE (staging)
STRIPE_SECRET_KEY=sk_test_...
STRIPE_WEBHOOK_SECRET=whsec_test_...
STRIPE_PRICE_PRO=price_test_pro_...
STRIPE_PRICE_ENTERPRISE=price_test_ent_...

# AFTER (production)
STRIPE_SECRET_KEY=sk_live_...
STRIPE_WEBHOOK_SECRET=whsec_live_...
STRIPE_PRICE_PRO=price_live_pro_...
STRIPE_PRICE_ENTERPRISE=price_live_ent_...

# CRITICAL
STRIPE_WEBHOOK_TEST=false
BILLING_SUCCESS_URL=https://wms.example.com/billing?success=true
BILLING_CANCEL_URL=https://wms.example.com/billing?canceled=true
```

### 5.3 — Validate First Live Payment Safely

```bash
# 1. Create an "internal" tenant for your organization
curl -s -X POST "https://wms.example.com/api/v1/tenants" \
  -H "Authorization: Bearer $SA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"code":"INTERNAL-001","name":"Internal Test","plan":"FREE","status":"ACTIVE"}'

# 2. Login as the tenant admin and open billing page
#    → Click "Upgrade to PRO"
#    → Use a REAL card (you'll be charged $49)
#    → Complete payment

# 3. Verify in Stripe Dashboard:
#    — Payment appears in Payments tab
#    — Customer created with correct metadata (tenant_id)
#    — Subscription shows "Active"

# 4. Verify in WMS:
curl -s "https://wms.example.com/api/v1/billing/status" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
# → plan=PRO, billing_status=ACTIVE

# 5. Cancel subscription via Stripe Dashboard or billing portal
#    Verify tenant downgrades to FREE

# 6. Refund the test charge from Stripe Dashboard
```

### 5.4 — `stripe listen` Rules

| Environment | `stripe listen` | Notes |
|-------------|----------------|-------|
| Local dev   | ✅ Use freely | Forward to `localhost:3003` |
| Staging     | ✅ Optional | Use for debugging webhook delivery |
| Production  | ❌ **NEVER** | Webhook endpoint is live, events arrive directly |

---

## PART 6 — Security Hardening Checklist

### 6.1 — JWT Secrets

```bash
# Generate a new 64+ character secret
NEW_SECRET=$(openssl rand -base64 48)

# Update deploy/.env
sed -i "s/^JWT_SECRET=.*/JWT_SECRET=${NEW_SECRET}/" deploy/.env

# Restart backend (all existing sessions will be invalidated)
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env restart backend
```

> ⚠️ Rotating JWT_SECRET invalidates ALL existing access tokens. Users will need to re-login. Refresh tokens stored in MongoDB are unaffected (they generate new access tokens with the new secret).

### 6.2 — Stripe Webhook Security

The backend already verifies Stripe webhook signatures via `webhook.ConstructEvent()` when `STRIPE_WEBHOOK_TEST=false`. Additional hardening:

```nginx
# Optionally restrict /webhooks/ to Stripe IPs in nginx
# Stripe publishes IPs at: https://stripe.com/docs/ips
# But signature verification is sufficient — Stripe recommends it over IP allowlisting.
location /api/v1/webhooks/ {
    # Signature verification handles authenticity
    proxy_pass http://backend;
}
```

### 6.3 — HTTPS Enforcement

- ✅ `nginx/wms.conf` already redirects HTTP → HTTPS
- ✅ HSTS header: `max-age=63072000; includeSubDomains; preload`
- ✅ `COOKIE_SECURE=true` ensures cookies are HTTPS-only

### 6.4 — Cookie Configuration

```env
COOKIE_DOMAIN=wms.example.com
COOKIE_SECURE=true
COOKIE_SAMESITE=Lax
```

Verify:
```bash
# Login and check Set-Cookie headers
curl -sD - -X POST "https://wms.example.com/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"xxx"}' -o /dev/null | grep -i set-cookie
# Expect: Secure; HttpOnly; SameSite=Lax; Domain=wms.example.com
```

### 6.5 — Rate Limiting

Already configured in `nginx/wms.conf`:
```nginx
limit_req_zone $binary_remote_addr zone=login_limit:10m rate=10r/m;

location = /api/v1/auth/login {
    limit_req zone=login_limit burst=5 nodelay;
    limit_req_status 429;
    proxy_pass http://backend;
}
```

The backend also has brute-force protection: 10 failed attempts per username+IP in 15min → account locked.

### 6.6 — Debug Flags

```bash
# Verify these are set correctly in production:
grep "STRIPE_WEBHOOK_TEST" deploy/.env
# Must be: STRIPE_WEBHOOK_TEST=false (or absent)

# No debug/dev flags should be set
grep -i "debug\|dev\|test" deploy/.env | grep -v "^#"
```

### 6.7 — Tenant Suspension Verification

```bash
# Suspend a test tenant
curl -s -X PUT "https://wms.example.com/api/v1/tenants/$T_ID" \
  -H "Authorization: Bearer $SA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status":"SUSPENDED"}'

sleep 2

# Verify tenant users get 403 TENANT_SUSPENDED
curl -s "https://wms.example.com/api/v1/products" \
  -H "Authorization: Bearer $TENANT_USER_TOKEN" \
  -H "X-Warehouse-Id: $WH_ID"
# → {"error":"TENANT_SUSPENDED","message":"..."}

# Reactivate
curl -s -X PUT "https://wms.example.com/api/v1/tenants/$T_ID" \
  -H "Authorization: Bearer $SA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"status":"ACTIVE"}'
```

---

## PART 7 — Production Cutover Plan (Minute-by-Minute)

### Prerequisites

- [x] Staging fully verified (preflight + smoke pass)
- [x] DNS `wms.example.com` pointed to production VPS
- [x] Production `.env` finalized with live Stripe keys
- [x] Rollback plan rehearsed on staging

### Maintenance Window

Choose a low-traffic window (e.g., Saturday 02:00 UTC). For new deploys (no existing users), no maintenance window is needed.

### Timeline

```
T-30min   Announce maintenance (if existing users)
T-15min   Take pre-deploy backup
T-10min   Pull latest code
T-5min    Build images
T-0       Deploy
T+2min    Health check
T+5min    Quick smoke test
T+10min   Stripe webhook test
T+15min   Enable alerts and verify
T+20min   All clear — go-live complete
```

### Step-by-Step

```bash
# ═══════════════════════════════════════════════
#  T-15min: Pre-deploy backup
# ═══════════════════════════════════════════════
cd /opt/wms
sudo MONGO_PASSWORD="$MONGO_PASSWORD" MONGO_USER="$MONGO_USER" MONGO_DB="$MONGO_DB" \
  deploy/scripts/backup-mongo.sh
ls -la backups/ | tail -3
BACKUP_FILE=$(ls -t backups/wms_*.gz | head -1)
echo "Backup: $BACKUP_FILE"

# ═══════════════════════════════════════════════
#  T-10min: Pull latest code
# ═══════════════════════════════════════════════
git pull origin main
git log --oneline -3
DEPLOY_SHA=$(git rev-parse --short HEAD)
echo "Deploying: $DEPLOY_SHA"

# ═══════════════════════════════════════════════
#  T-5min: Build images
# ═══════════════════════════════════════════════
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env build --no-cache

# ═══════════════════════════════════════════════
#  T-0: Deploy
# ═══════════════════════════════════════════════
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env up -d
sleep 30
docker compose -f deploy/docker-compose.prod.yml ps

# ═══════════════════════════════════════════════
#  T+2min: Health check
# ═══════════════════════════════════════════════
curl -s https://wms.example.com/health | jq .
# → {"status":"ok","service":"warehouse-crm"}

curl -sI https://wms.example.com | head -5
# → HTTP/2 200

# ═══════════════════════════════════════════════
#  T+5min: Quick smoke
# ═══════════════════════════════════════════════
# Login as superadmin
SA_TOKEN=$(curl -s -X POST "https://wms.example.com/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"superadmin","password":"YOUR_SA_PASSWORD"}' | jq -r '.token')
echo "SA Token: ${SA_TOKEN:0:20}..."

# Create tenant
curl -s -X POST "https://wms.example.com/api/v1/tenants" \
  -H "Authorization: Bearer $SA_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"code":"SMOKE-PROD","name":"Smoke Test","plan":"FREE","status":"ACTIVE"}' | jq .

# ═══════════════════════════════════════════════
#  T+10min: Stripe webhook verification
# ═══════════════════════════════════════════════
# From Stripe Dashboard → Webhooks → Send test webhook
# → Select: checkout.session.completed
# → Verify: backend logs show "billing: checkout completed"
docker compose -f deploy/docker-compose.prod.yml logs backend --tail 20 | grep billing

# ═══════════════════════════════════════════════
#  T+15min: Verify monitoring
# ═══════════════════════════════════════════════
# Prometheus targets
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[].health'
# → ["up", "up"]

# Loki logs flowing
docker compose -f deploy/docker-compose.prod.yml logs promtail --tail 5

# Grafana accessible
curl -sI https://wms.example.com/grafana/ | head -3
# → HTTP/2 200

# ═══════════════════════════════════════════════
#  T+20min: ALL CLEAR
# ═══════════════════════════════════════════════
echo "✅ Go-live complete: $(date) — SHA: $DEPLOY_SHA"
```

### Rollback Plan

#### Quick Rollback (revert to previous image)

```bash
cd /opt/wms

# If using image tags:
BACKEND_IMAGE=ghcr.io/YOUR_ORG/wms-backend:PREVIOUS_SHA \
FRONTEND_IMAGE=ghcr.io/YOUR_ORG/wms-frontend:PREVIOUS_SHA \
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env \
  up -d --no-deps backend frontend

# If using git:
git checkout PREVIOUS_SHA
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env up -d --build
```

#### Database Rollback (only if data corruption)

```bash
# 1. Stop backend
docker compose -f deploy/docker-compose.prod.yml stop backend

# 2. Restore from backup
docker exec -i wms-mongodb mongorestore \
  --username=$MONGO_USER --password=$MONGO_PASSWORD \
  --authenticationDatabase=admin \
  --db=$MONGO_DB --drop --gzip \
  --archive < $BACKUP_FILE

# 3. Restart backend
docker compose -f deploy/docker-compose.prod.yml start backend

# 4. Verify health
curl -s https://wms.example.com/health | jq .
```

#### Emergency: Invalidate All Sessions (compromised credentials)

```bash
# This invalidates all refresh tokens but not JWTs (they expire naturally in 15min)
docker exec wms-mongodb mongosh \
  --username $MONGO_USER --password $MONGO_PASSWORD \
  --authenticationDatabase admin \
  --eval "
    use $MONGO_DB;
    db.sessions.updateMany({}, {\$set: {revoked: true, revoked_at: new Date()}});
    print('All sessions revoked: ' + db.sessions.countDocuments({revoked: true}));
  "

# To also invalidate access tokens immediately, rotate JWT_SECRET:
NEW_SECRET=$(openssl rand -base64 48)
sed -i "s/^JWT_SECRET=.*/JWT_SECRET=${NEW_SECRET}/" deploy/.env
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env restart backend
echo "⚠️ All tokens invalidated. All users must re-login."
```

---

## Appendix A — File Map

```
/opt/wms/
├── deploy/
│   ├── GO_LIVE_PLAYBOOK.md      ← this file
│   ├── RUNBOOK.md               ← operations guide
│   ├── docker-compose.prod.yml  ← production compose (7 services)
│   ├── .env                     ← production secrets (gitignored)
│   ├── .env.example             ← template
│   ├── nginx/wms.conf           ← reverse proxy + TLS + rate limits
│   ├── prometheus/              ← scrape config + alert rules
│   ├── loki/                    ← log storage config
│   ├── promtail/                ← log shipper config
│   └── scripts/                 ← backup scripts + systemd units
├── scripts/
│   ├── preflight.sh             ← infrastructure readiness checks
│   ├── smoke-staging.sh         ← E2E staging smoke suite
│   ├── smoke-test.sh            ← basic API smoke test
│   ├── test-*.sh                ← per-module integration tests
│   └── fixtures/                ← webhook JSON fixtures
└── cmd/
    ├── main.go                  ← application entrypoint
    ├── migrate-warehouse/       ← warehouse migration tool
    └── migrate-tenant/          ← tenant migration tool
```
