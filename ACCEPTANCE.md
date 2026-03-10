# Warehouse CRM — MVP Acceptance Criteria

## How to Use
Run through this checklist after `docker compose up -d`. Check each box when verified.

---

## Infrastructure

- [ ] `docker compose up -d` starts both containers without errors
- [ ] `docker ps` shows **warehouse-crm-app** as `(healthy)`
- [ ] `docker ps` shows **warehouse-crm-mongo** as `(healthy)`
- [ ] App starts on port **3000** without conflict
- [ ] App auto-reconnects if MongoDB restarts

## Health & Monitoring

- [ ] `GET /health` → `{"status":"ok","service":"warehouse-crm"}`
- [ ] `GET /metrics` → valid Prometheus exposition format
- [ ] Metrics include `http_requests_total` counter
- [ ] Metrics include request duration histogram

## Authentication

- [ ] `POST /api/v1/auth/register` creates new user → 201
- [ ] Duplicate registration → 409 Conflict
- [ ] `POST /api/v1/auth/login` returns JWT token → 200
- [ ] Wrong password → 401 Unauthorized
- [ ] Protected endpoints reject request without token → 401
- [ ] Protected endpoints reject expired/invalid token → 401

## Product CRUD

- [ ] `POST /api/v1/products` creates product → 201, returns ID
- [ ] `GET /api/v1/products` lists all products with pagination
- [ ] `GET /api/v1/products/:id` returns single product
- [ ] `PUT /api/v1/products/:id` updates product fields
- [ ] `DELETE /api/v1/products/:id` removes product
- [ ] Duplicate SKU → appropriate error

## Location CRUD

- [ ] `POST /api/v1/locations` creates location → 201, returns ID
- [ ] `GET /api/v1/locations` lists with pagination
- [ ] `GET /api/v1/locations/:id` returns single location
- [ ] `PUT /api/v1/locations/:id` updates location
- [ ] `DELETE /api/v1/locations/:id` removes location

## Inbound (Receiving)

- [ ] `POST /api/v1/inbound` accepts `{product_id, location_id, quantity}` → 201
- [ ] Inbound automatically increases stock by quantity
- [ ] Inbound automatically creates history record
- [ ] `GET /api/v1/inbound` lists with pagination

## Outbound (Shipping)

- [ ] `POST /api/v1/outbound` accepts `{product_id, location_id, quantity}` → 201
- [ ] Outbound automatically decreases stock by quantity
- [ ] Outbound rejects if quantity > available stock
- [ ] Outbound automatically creates history record
- [ ] `GET /api/v1/outbound` lists with pagination

## Stock Calculations (Critical ✓)

- [ ] After inbound of 1000 → stock shows 1000
- [ ] After outbound of 200 → stock shows 800
- [ ] `GET /api/v1/stock` lists all stock entries
- [ ] `GET /api/v1/stock/product/:id` filters by product
- [ ] `GET /api/v1/stock/location/:id` filters by location

## History / Audit Trail

- [ ] `GET /api/v1/history` returns all records with pagination
- [ ] History records contain: action, entity_type, entity_id, user_id, timestamp
- [ ] Filterable by `user_id`, `entity_type`, `entity_id`
- [ ] At least 2 records after inbound + outbound

## Automated Smoke Test

- [ ] `./scripts/smoke-test.sh` passes all checks (exit code 0)

---

**MVP is ready when ALL boxes above are checked ✅**
