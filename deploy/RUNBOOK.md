# Warehouse CRM — Production Runbook

Step-by-step guide for deploying and operating WMS on a single Ubuntu 22.04 VPS.

---

## Prerequisites

| Item | Requirement |
|------|-------------|
| VPS | Ubuntu 22.04 LTS, 2+ vCPU, 4+ GB RAM, 40+ GB SSD |
| Domain | `wms.example.com` pointed to VPS IP (A record) |
| SSH access | Root or sudo user |

---

## Step 1 — Provision VPS & Base Setup

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install essential packages
sudo apt install -y curl git ufw fail2ban

# Create deploy user (optional but recommended)
sudo adduser deploy
sudo usermod -aG sudo deploy
sudo usermod -aG docker deploy
```

---

## Step 2 — Install Docker & Compose

```bash
# Install Docker (official script)
curl -fsSL https://get.docker.com | sudo sh

# Enable and start Docker
sudo systemctl enable docker
sudo systemctl start docker

# Verify
docker --version
docker compose version
```

---

## Step 3 — Firewall (UFW)

```bash
# Default deny incoming, allow outgoing
sudo ufw default deny incoming
sudo ufw default allow outgoing

# Allow SSH, HTTP, HTTPS only
sudo ufw allow 22/tcp comment 'SSH'
sudo ufw allow 80/tcp comment 'HTTP'
sudo ufw allow 443/tcp comment 'HTTPS'

# Enable firewall
sudo ufw enable
sudo ufw status verbose
```

> **⚠️ Important:** MongoDB (27017), Prometheus (9090), Grafana (3001), and app
> ports (3002, 3003) are NOT exposed to the internet. They are only accessible
> via Docker's internal network and nginx reverse proxy on localhost.

---

## Step 4 — Clone Repo & Configure

```bash
# Clone to /opt/wms
sudo mkdir -p /opt/wms
sudo chown deploy:deploy /opt/wms
cd /opt/wms
git clone https://github.com/YOUR_ORG/warehouse-crm.git .

# Create production .env from template
cp deploy/.env.example deploy/.env

# Generate secrets and edit .env
openssl rand -base64 48  # → JWT_SECRET (copy this)
openssl rand -base64 32  # → MONGO_PASSWORD (copy this)
nano deploy/.env         # Fill in all values
```

### Key values to set in `.env`:

| Variable | Example |
|----------|---------|
| `DOMAIN` | `wms.example.com` |
| `MONGO_USER` | `wmsadmin` |
| `MONGO_PASSWORD` | (generated above, ≥32 chars) |
| `JWT_SECRET` | (generated above, ≥64 chars) |
| `CORS_ORIGINS` | `https://wms.example.com` |
| `NEXT_PUBLIC_API_BASE_URL` | `https://wms.example.com/api/v1` |
| `GF_ADMIN_PASSWORD` | (strong password for Grafana) |

---

## Step 5 — Start the Stack

```bash
cd /opt/wms

# Build and start all services
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env up -d --build

# Check all services are healthy
docker compose -f deploy/docker-compose.prod.yml ps

# View logs
docker compose -f deploy/docker-compose.prod.yml logs -f --tail 50
```

### Expected output:

```
NAME              STATUS
wms-backend       Up (healthy)
wms-frontend      Up (healthy)
wms-mongodb       Up (healthy)
wms-prometheus    Up
wms-grafana       Up
wms-loki          Up
wms-promtail      Up
```

---

## Step 6 — Install Nginx & HTTPS

```bash
# Install Nginx + Certbot
sudo apt install -y nginx certbot python3-certbot-nginx

# Copy Nginx config
sudo cp deploy/nginx/wms.conf /etc/nginx/sites-available/wms.conf

# Edit the config: replace wms.example.com with your domain
sudo sed -i 's/wms.example.com/YOUR_DOMAIN/g' /etc/nginx/sites-available/wms.conf

# Enable the site
sudo ln -s /etc/nginx/sites-available/wms.conf /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default

# Test Nginx config
sudo nginx -t

# First run: comment out the SSL lines temporarily for certbot to work
# (certbot --nginx will add them automatically)
# Or use HTTP-only mode first:
sudo systemctl reload nginx

# Obtain SSL certificate
sudo certbot --nginx -d YOUR_DOMAIN --non-interactive --agree-tos -m admin@example.com

# Verify auto-renewal
sudo certbot renew --dry-run

# Restart Nginx
sudo systemctl restart nginx
```

---

## Step 7 — Setup Backups

```bash
# Make backup script executable
chmod +x deploy/scripts/backup-mongo.sh

# Create backup directory
sudo mkdir -p /opt/wms/backups

# Install systemd timer
sudo cp deploy/scripts/wms-backup.service /etc/systemd/system/
sudo cp deploy/scripts/wms-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable wms-backup.timer
sudo systemctl start wms-backup.timer

# Verify timer is active
systemctl list-timers | grep wms

# Test backup manually
sudo /opt/wms/deploy/scripts/backup-mongo.sh
ls -la /opt/wms/backups/
```

---

## Step 8 — Verify Everything

### 8.1 — Backend Health
```bash
curl -s https://YOUR_DOMAIN/health | jq .
# Expected: {"status":"ok","service":"warehouse-crm"}
```

### 8.2 — Frontend
```bash
curl -sI https://YOUR_DOMAIN | head -20
# Expected: HTTP/2 200, HSTS header, security headers
```

Open `https://YOUR_DOMAIN` in browser — you should see the login page.

### 8.3 — Security Headers
```bash
curl -sI https://YOUR_DOMAIN | grep -E "(Strict-Transport|X-Content-Type|X-Frame|X-XSS)"
# Expected:
# Strict-Transport-Security: max-age=63072000; includeSubDomains; preload
# X-Content-Type-Options: nosniff
# X-Frame-Options: SAMEORIGIN
# X-XSS-Protection: 1; mode=block
```

### 8.4 — Metrics (should be blocked externally)
```bash
# From external machine:
curl -sI https://YOUR_DOMAIN/metrics
# Expected: 403 Forbidden

# From VPS itself:
curl -s http://localhost:3003/metrics | head -5
# Expected: Prometheus metrics text
```

### 8.5 — Grafana
Open `https://YOUR_DOMAIN/grafana/` in browser. Login with GF_ADMIN_USER/GF_ADMIN_PASSWORD.

Add data sources:
1. **Prometheus**: URL = `http://prometheus:9090`
2. **Loki**: URL = `http://loki:3100`

### 8.6 — Backups
```bash
ls -la /opt/wms/backups/
# Expected: at least one .gz file from the manual test
```

### 8.7 — Rate Limiting
```bash
# Rapid-fire login attempts (should get 429 after ~15)
for i in $(seq 1 20); do
  echo -n "Attempt $i: "
  curl -s -o /dev/null -w "%{http_code}" -X POST https://YOUR_DOMAIN/api/v1/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"test","password":"test"}'
  echo
done
# Expected: first ~15 return 401, then 429 (Too Many Requests)
```

---

## Step 9 — Rollback Plan

### Quick rollback (to previous image):

```bash
cd /opt/wms

# Pin to a specific SHA (from a previous deployment)
BACKEND_IMAGE=ghcr.io/YOUR_ORG/wms-backend:PREVIOUS_SHA \
FRONTEND_IMAGE=ghcr.io/YOUR_ORG/wms-frontend:PREVIOUS_SHA \
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env up -d --no-deps backend frontend
```

### Database rollback:

```bash
# Stop the backend
docker compose -f deploy/docker-compose.prod.yml stop backend

# Restore from backup
docker exec -i wms-mongodb mongorestore \
  --username=wmsadmin --password=YOUR_PASS --authenticationDatabase=admin \
  --db=warehouse_crm --drop --gzip --archive < /opt/wms/backups/wms_warehouse_crm_YYYYMMDD_HHMMSS.gz

# Restart backend
docker compose -f deploy/docker-compose.prod.yml start backend
```

### Full rollback (git revert):

```bash
cd /opt/wms
git log --oneline -5        # Find the commit to revert to
git revert HEAD             # Or: git checkout COMMIT_SHA
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env up -d --build
```

---

## Step 10 — Ongoing Operations

### Manual deployment (without CI/CD):

```bash
cd /opt/wms
git pull origin main
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env up -d --build
```

### View logs:

```bash
# All services
docker compose -f deploy/docker-compose.prod.yml logs -f --tail 100

# Specific service
docker compose -f deploy/docker-compose.prod.yml logs -f backend

# Backup logs
journalctl -u wms-backup -f
```

### Restart a service:

```bash
docker compose -f deploy/docker-compose.prod.yml restart backend
```

### Check disk usage:

```bash
df -h
docker system df
du -sh /opt/wms/backups/
```

### Renew SSL (auto, but manual if needed):

```bash
sudo certbot renew
```

---

## File Map

```
/opt/wms/
├── Dockerfile                    # Backend Go image
├── frontend/
│   └── Dockerfile                # Frontend Next.js image
├── deploy/
│   ├── docker-compose.prod.yml   # Production compose
│   ├── .env                      # Production secrets (gitignored)
│   ├── .env.example              # Template
│   ├── nginx/
│   │   └── wms.conf              # Nginx virtual host
│   ├── prometheus/
│   │   └── prometheus.yml        # Scrape config
│   ├── loki/
│   │   └── loki-config.yml       # Log storage config
│   ├── promtail/
│   │   └── promtail-config.yml   # Log shipper config
│   ├── scripts/
│   │   ├── backup-mongo.sh       # Backup script
│   │   ├── wms-backup.service    # Systemd service
│   │   └── wms-backup.timer      # Daily 02:00 UTC
│   └── RUNBOOK.md                # This file
├── .github/
│   └── workflows/
│       └── deploy.yml            # CI/CD pipeline
└── backups/                      # Backup storage (on VPS)
```
