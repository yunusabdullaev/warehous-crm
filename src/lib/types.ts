/* ── API response types for Warehouse CRM ── */

export interface AuthResponse {
  token: string;
  requires_2fa?: boolean;
  temp_token?: string;
  user: {
    id: string;
    username: string;
    role: string;
    tenant_id?: string;
    default_warehouse_id?: string;
    allowed_warehouse_ids?: string[];
    two_factor_enabled?: boolean;
  };
}

export interface Product {
  id: string;
  sku: string;
  name: string;
  unit: string;
  category: string;
  description: string;
  low_stock_threshold?: number | null;
  created_at: string;
  updated_at: string;
}

export interface Location {
  id: string;
  code: string;
  name: string;
  zone: string;
  aisle: string;
  rack: string;
  level: string;
  created_at: string;
  updated_at: string;
}

export interface InboundRecord {
  id: string;
  product_id: string;
  location_id: string;
  lot_id: string;
  quantity: number;
  reference: string;
  user_id: string;
  status: string;
  reversed_at?: string;
  reversed_by?: string;
  reverse_reason?: string;
  created_at: string;
}

export interface OutboundRecord {
  id: string;
  product_id: string;
  location_id: string;
  lot_id: string;
  quantity: number;
  reference: string;
  user_id: string;
  status: string;
  reversed_at?: string;
  reversed_by?: string;
  reverse_reason?: string;
  created_at: string;
}

export interface AdjustmentRecord {
  id: string;
  product_id: string;
  location_id: string;
  lot_id?: string;
  delta_qty: number;
  reason: string;
  note?: string;
  created_by: string;
  created_at: string;
}

export interface StockRecord {
  id: string;
  product_id: string;
  location_id: string;
  lot_id: string;
  quantity: number;
  reserved_qty: number;
  available_qty: number;
  last_updated: string;
}

export interface HistoryRecord {
  id: string;
  action: string;
  entity_type: string;
  entity_id: string;
  user_id: string;
  details: string;
  timestamp: string;
}

/* ── Order types ── */

export interface OrderItem {
  product_id: string;
  requested_qty: number;
  reserved_qty: number;
  shipped_qty: number;
}

export interface Order {
  id: string;
  order_no: string;
  client_name: string;
  status: string;
  notes?: string;
  items: OrderItem[];
  created_by: string;
  created_at: string;
  confirmed_at?: string;
  shipped_at?: string;
  cancelled_at?: string;
}

export interface Reservation {
  id: string;
  order_id: string;
  product_id: string;
  qty: number;
  status: string;
  created_at: string;
  created_by: string;
  released_at?: string;
  released_by?: string;
  reason?: string;
}

/* ── Pick types ── */

export interface PickTask {
  id: string;
  order_id: string;
  product_id: string;
  location_id: string;
  lot_id: string;
  planned_qty: number;
  picked_qty: number;
  status: string;
  assigned_to?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface PickEvent {
  id: string;
  order_id: string;
  pick_task_id: string;
  user_id: string;
  location_id: string;
  product_id: string;
  lot_id: string;
  qty: number;
  scanned_at: string;
  scanner?: string;
  client?: string;
}

/* ── Dashboard & Reports ── */

export interface DashboardSummary {
  total_products: number;
  total_locations: number;
  total_stock_qty: number;
  inbound_count: number;
  outbound_count: number;
  inbound_qty_total: number;
  outbound_qty_total: number;
  adjustments_count: number;
  adjustments_qty_net: number;
  adjustments_qty_abs: number;
  open_orders_count: number;
  reserved_qty_total: number;
  picking_orders_count: number;
  pick_tasks_open: number;
  open_returns_count: number;
  returns_received_count: number;
  top_moving_products: { product_id: string; total_qty: number }[];
  stock_by_zone: { zone: string; quantity: number }[];
}

export interface MovementsReport {
  from: string;
  to: string;
  group_by: string;
  data: { period: string; inbound_qty: number; outbound_qty: number; adjustment_qty: number }[];
}

export interface StockReport {
  group_by: string;
  data: { group: string; quantity: number }[];
}

export interface OrdersReport {
  from: string;
  to: string;
  group_by: string;
  data: { period: string; created: number; confirmed: number; shipped: number }[];
}

export interface PickingReport {
  from: string;
  to: string;
  group_by: string;
  data: { period: string; tasks_created: number; tasks_completed: number; avg_pick_time_sec: number }[];
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  limit: number;
}

export type Role = "superadmin" | "admin" | "operator" | "loader";

export interface User {
  id: string;
  username: string;
  role: Role;
  tenant_id?: string;
  allowed_warehouse_ids: string[];
  default_warehouse_id: string;
  created_at: string;
  updated_at: string;
}

export interface TenantLimits {
  max_warehouses: number;
  max_users: number;
  max_products: number;
  max_daily_orders: number;
  max_storage_mb?: number;
}

export interface TenantFeatures {
  enable_reports: boolean;
  enable_expiry_digest: boolean;
  enable_qr_labels: boolean;
  enable_returns: boolean;
  enable_lots: boolean;
  enable_multi_warehouse: boolean;
  enable_api_export: boolean;
}

export interface TenantUsage {
  warehouses: number;
  users: number;
  products: number;
  today_orders: number;
}

export interface Tenant {
  id: string;
  code: string;
  name: string;
  plan: string;
  status: string;
  limits: TenantLimits;
  features: TenantFeatures;
  created_at: string;
  updated_at: string;
  // Billing
  stripe_customer_id?: string;
  stripe_subscription_id?: string;
  billing_status?: string;
  current_period_end?: string;
  cancel_at_period_end?: boolean;
}

export interface BillingStatus {
  plan: string;
  status: string;
  billing_status: string;
  stripe_customer_id?: string;
  current_period_end?: string;
  cancel_at_period_end: boolean;
}

/* ── Return types ── */

export interface ReturnRecord {
  id: string;
  rma_no: string;
  order_id: string;
  order_no: string;
  client_name: string;
  status: string;
  notes?: string;
  created_at: string;
  created_by: string;
  received_at?: string;
  received_by?: string;
}

export interface ReturnItem {
  id: string;
  return_id: string;
  product_id: string;
  location_id?: string;
  lot_id?: string;
  qty: number;
  disposition: string;
  note?: string;
  created_at: string;
  created_by: string;
}

/* ── Lot types ── */

export interface Lot {
  id: string;
  product_id: string;
  lot_no: string;
  exp_date?: string;
  mfg_date?: string;
  created_at: string;
}

export interface ReturnWithItems {
  return: ReturnRecord;
  items: ReturnItem[];
}

export interface ReturnsReport {
  from: string;
  to: string;
  group_by: string;
  data: {
    period: string;
    returns_created: number;
    returns_received: number;
    qty_restocked: number;
    qty_damaged: number;
    qty_qc_hold: number;
  }[];
}
