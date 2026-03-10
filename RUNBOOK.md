# Warehouse CRM — Developer Runbook

## Prerequisites

- **Docker Desktop** ≥ 4.x (with Docker Compose v2 built-in)
- **jq** (for smoke tests): `brew install jq`
- Free ports: `3000` (app), `27017` (MongoDB)

---

## 1. Start the Stack

```bash
cd "/Users/yunus/crm for warehous"

# Build & start (detached)
docker compose up -d --build

# Check containers are running & healthy
docker ps

# Expected output:
# warehouse-crm-app    ... (healthy)   0.0.0.0:3000->3000/tcp
# warehouse-crm-mongo  ... (healthy)   0.0.0.0:27017->27017/tcp
```

## 2. Verify Health

```bash
curl -s http://localhost:3000/health | jq .
# → {"service":"warehouse-crm","status":"ok"}

curl -s http://localhost:3000/metrics | head -5
# → Prometheus metrics output
```

## 3. View Logs

```bash
# All services
docker compose logs -f

# App only
docker compose logs -f app

# MongoDB only
docker compose logs -f mongodb
```

## 4. Stop & Clean

```bash
# Stop (keeps data)
docker compose down

# Stop + delete volumes (RESETS MongoDB data)
docker compose down -v
```

## 5. Run Smoke Tests

```bash
chmod +x scripts/smoke-test.sh
./scripts/smoke-test.sh
```

## 6. Seed Sample Data

```bash
chmod +x scripts/seed-data.sh
./scripts/seed-data.sh
```

---

## Troubleshooting

### Port 3000 already in use
```bash
lsof -ti:3000 | xargs kill -9
docker compose up -d
```

### Port 27017 already in use
```bash
# Stop local Mongo
brew services stop mongodb-community
# OR
lsof -ti:27017 | xargs kill -9
docker compose up -d
```

### App exits with "connection refused" to MongoDB
This means Mongo wasn't ready when the app started. The healthcheck + `depends_on` condition should prevent this, but if it happens:
```bash
docker compose down
docker compose up -d
# Wait 15-20 seconds for Mongo healthcheck to pass
docker ps   # verify both show (healthy)
```

### Containers restart in a loop
```bash
# Check logs for the failing container
docker compose logs app --tail 50

# Common causes:
# 1. Missing env var → check docker-compose.yml environment section
# 2. Wrong MONGO_URI → must be mongodb://mongodb:27017 (service name, not localhost)
```

### "go.sum is out of date" during build
```bash
cd "/Users/yunus/crm for warehous"
go mod tidy
docker compose up -d --build
```

### Reset everything from scratch
```bash
docker compose down -v --rmi all
docker compose up -d --build
```
