import type { Role } from "./types";

interface JWTPayload {
    user_id: string;
    username: string;
    role: Role;
    tenant_id?: string;
    exp: number;
}

// ── Decode JWT without library (base64) ──
export function decodeToken(token: string): JWTPayload | null {
    try {
        const payload = token.split(".")[1];
        const decoded = JSON.parse(atob(payload));
        return decoded as JWTPayload;
    } catch {
        return null;
    }
}

// ── Store / Retrieve ──
export function setAuth(token: string, warehouseId?: string) {
    localStorage.setItem("wms_token", token);
    const decoded = decodeToken(token);
    if (decoded) {
        localStorage.setItem(
            "wms_user",
            JSON.stringify({
                id: decoded.user_id,
                username: decoded.username,
                role: decoded.role,
            })
        );
        if (decoded.tenant_id) {
            localStorage.setItem("wms_tenant_id", decoded.tenant_id);
        }
    }
    if (warehouseId) {
        localStorage.setItem("wms_warehouse_id", warehouseId);
    }
}

export function getToken(): string | null {
    if (typeof window === "undefined") return null;
    return localStorage.getItem("wms_token");
}

export function getUser(): { id: string; username: string; role: Role } | null {
    if (typeof window === "undefined") return null;
    try {
        const raw = localStorage.getItem("wms_user");
        return raw ? JSON.parse(raw) : null;
    } catch {
        return null;
    }
}

export function getRole(): Role | null {
    return getUser()?.role ?? null;
}

export async function logout() {
    try {
        const { default: api } = await import("@/lib/api");
        await api.post("/auth/logout", {});
    } catch { /* fire-and-forget */ }
    localStorage.removeItem("wms_token");
    localStorage.removeItem("wms_user");
    localStorage.removeItem("wms_warehouse_id");
    localStorage.removeItem("wms_tenant_id");
    window.location.href = "/login";
}

// ── Tenant helpers ──
export function getTenantId(): string | null {
    if (typeof window === "undefined") return null;
    return localStorage.getItem("wms_tenant_id");
}

export function isSuperAdmin(): boolean {
    return getRole() === "superadmin";
}

// ── Warehouse helpers ──
export function getActiveWarehouseId(): string | null {
    if (typeof window === "undefined") return null;
    return localStorage.getItem("wms_warehouse_id");
}

export function setActiveWarehouseId(id: string) {
    localStorage.setItem("wms_warehouse_id", id);
    // Dispatch event so Sidebar and other components can react
    window.dispatchEvent(new CustomEvent("wms-warehouse-changed", { detail: { id } }));
}

export function isAuthenticated(): boolean {
    const token = getToken();
    if (!token) return false;
    const decoded = decodeToken(token);
    if (!decoded) return false;
    // Check expiry
    return decoded.exp * 1000 > Date.now();
}

// ── Permission helpers ──
const PERMS: Record<string, Role[]> = {
    "products:create": ["admin"],
    "products:update": ["admin"],
    "products:delete": ["admin"],
    "locations:create": ["admin"],
    "locations:update": ["admin"],
    "locations:delete": ["admin"],
    "inbound:view": ["admin", "operator", "loader"],
    "outbound:create": ["admin", "operator"],
    "outbound:view": ["admin", "operator"],
    "adjustments:view": ["admin", "operator"],
    "adjustments:create": ["admin", "operator"],
    "orders:view": ["admin", "operator", "loader"],
    "orders:create": ["admin", "operator"],
    "orders:manage": ["admin", "operator"],
    "picking:view": ["admin", "operator", "loader"],
    "picking:scan": ["admin", "operator", "loader"],
    "history:view": ["admin", "operator"],
    "dashboard:view": ["admin", "operator"],
    "reports:view": ["admin"],
    "users:manage": ["admin"],
    "settings:manage": ["admin"],
    "returns:view": ["admin", "operator", "loader"],
    "returns:manage": ["admin", "operator"],
    "returns:receive": ["admin", "operator", "loader"],
    "lots:view": ["admin", "operator"],
    "tenants:manage": ["superadmin"],
    "billing:manage": ["admin"],
};

export function can(action: string): boolean {
    const role = getRole();
    if (!role) return false;
    if (role === "admin" || role === "superadmin") return true; // admin/superadmin can do everything
    const allowed = PERMS[action];
    if (!allowed) return true; // no restriction defined = all can access
    return allowed.includes(role);
}

// Nav items visible to role
export interface NavItem {
    label: string;
    href: string;
    icon: string;
    requiredAction?: string;
}

export const NAV_ITEMS: NavItem[] = [
    { label: "Dashboard", href: "/dashboard", icon: "📊", requiredAction: "dashboard:view" },
    { label: "Products", href: "/products", icon: "📦" },
    { label: "Locations", href: "/locations", icon: "📍" },
    { label: "Inbound", href: "/inbound", icon: "📥", requiredAction: "inbound:view" },
    { label: "Outbound", href: "/outbound", icon: "📤", requiredAction: "outbound:view" },
    { label: "Adjustments", href: "/adjustments", icon: "🔧", requiredAction: "adjustments:view" },
    { label: "Orders", href: "/orders", icon: "🛒", requiredAction: "orders:view" },
    { label: "Pick Scan", href: "/pick/scan", icon: "🎯", requiredAction: "picking:scan" },
    { label: "Scan In", href: "/inbound/scan", icon: "📷", requiredAction: "inbound:view" },
    { label: "Scan Out", href: "/outbound/scan", icon: "📸", requiredAction: "outbound:view" },
    { label: "Stock", href: "/stock", icon: "🏭" },
    { label: "Lots", href: "/lots", icon: "🏷️", requiredAction: "lots:view" },
    { label: "History", href: "/history", icon: "📜", requiredAction: "history:view" },
    { label: "Reports", href: "/reports", icon: "📈", requiredAction: "reports:view" },
    { label: "Users", href: "/users", icon: "👥", requiredAction: "users:manage" },
    { label: "Returns", href: "/returns", icon: "↩️", requiredAction: "returns:view" },
    { label: "Return Scan", href: "/returns/scan", icon: "📦", requiredAction: "returns:receive" },
    { label: "Warehouses", href: "/warehouses", icon: "🏢", requiredAction: "settings:manage" },
    { label: "Billing", href: "/billing", icon: "💳", requiredAction: "billing:manage" },
    { label: "Tenants", href: "/tenants", icon: "🏪", requiredAction: "tenants:manage" },
    { label: "Security", href: "/settings/security", icon: "🔒" },
    { label: "Settings", href: "/settings/notifications", icon: "⚙️", requiredAction: "settings:manage" },
];
