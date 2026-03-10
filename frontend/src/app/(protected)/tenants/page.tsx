"use client";
import { useEffect, useState, useCallback } from "react";
import api from "@/lib/api";
import { isSuperAdmin } from "@/lib/auth";
import { toast } from "@/components/Toast";
import type { Tenant, TenantLimits, TenantFeatures, TenantUsage, PaginatedResponse } from "@/lib/types";

const PLAN_BADGE: Record<string, string> = {
    FREE: "bg-gray-100 text-gray-700",
    PRO: "bg-purple-100 text-purple-700",
    ENTERPRISE: "bg-amber-100 text-amber-700",
};

const STATUS_BADGE: Record<string, string> = {
    ACTIVE: "bg-green-100 text-green-700",
    SUSPENDED: "bg-red-100 text-red-700",
};

const DEFAULT_LIMITS: TenantLimits = {
    max_warehouses: 1, max_users: 5, max_products: 500, max_daily_orders: 50,
};

const DEFAULT_FEATURES: TenantFeatures = {
    enable_reports: false, enable_expiry_digest: false, enable_qr_labels: false,
    enable_returns: false, enable_lots: false, enable_multi_warehouse: false, enable_api_export: false,
};

const FEATURE_LABELS: Record<string, string> = {
    enable_reports: "📊 Reports",
    enable_expiry_digest: "⏰ Expiry Digest",
    enable_qr_labels: "🏷️ QR Labels",
    enable_returns: "↩️ Returns",
    enable_lots: "📦 Lots/Batches",
    enable_multi_warehouse: "🏭 Multi-Warehouse",
    enable_api_export: "📤 API Export",
};

interface FormData {
    code: string;
    name: string;
    plan: string;
    status: string;
    limits: TenantLimits;
    features: TenantFeatures;
}

const emptyForm = (): FormData => ({
    code: "", name: "", plan: "FREE", status: "ACTIVE",
    limits: { ...DEFAULT_LIMITS },
    features: { ...DEFAULT_FEATURES },
});

export default function TenantsPage() {
    const [tenants, setTenants] = useState<Tenant[]>([]);
    const [loading, setLoading] = useState(true);
    const [showModal, setShowModal] = useState(false);
    const [editId, setEditId] = useState<string | null>(null);
    const [form, setForm] = useState<FormData>(emptyForm());
    const [error, setError] = useState("");
    const [usageMap, setUsageMap] = useState<Record<string, TenantUsage>>({});
    const [expandedId, setExpandedId] = useState<string | null>(null);

    const fetchTenants = useCallback(async () => {
        try {
            setLoading(true);
            const res = await api.get<PaginatedResponse<Tenant>>("/tenants?limit=100");
            setTenants(res.data.data || []);
        } catch {
            setError("Failed to load tenants");
        } finally {
            setLoading(false);
        }
    }, []);

    const fetchUsage = async (id: string) => {
        try {
            const res = await api.get<TenantUsage>(`/tenants/${id}/usage`);
            setUsageMap(prev => ({ ...prev, [id]: res.data }));
        } catch {
            toast("error", "Failed to load usage");
        }
    };

    useEffect(() => {
        if (!isSuperAdmin()) { toast("error", "Superadmin access required"); return; }
        fetchTenants();
    }, [fetchTenants]);

    const toggleExpand = (id: string) => {
        if (expandedId === id) { setExpandedId(null); return; }
        setExpandedId(id);
        if (!usageMap[id]) fetchUsage(id);
    };

    const openCreate = () => {
        setEditId(null);
        setForm(emptyForm());
        setError("");
        setShowModal(true);
    };

    const openEdit = (t: Tenant) => {
        setEditId(t.id);
        setForm({
            code: t.code, name: t.name, plan: t.plan || "FREE", status: t.status || "ACTIVE",
            limits: t.limits || { ...DEFAULT_LIMITS },
            features: t.features || { ...DEFAULT_FEATURES },
        });
        setError("");
        setShowModal(true);
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError("");
        try {
            if (editId) {
                await api.put(`/tenants/${editId}`, form);
                toast("success", "Tenant updated");
            } else {
                await api.post("/tenants", form);
                toast("success", "Tenant created");
            }
            setShowModal(false);
            fetchTenants();
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || "Operation failed";
            setError(msg);
        }
    };

    const handleDelete = async (id: string, name: string) => {
        if (!confirm(`Delete tenant "${name}"? This cannot be undone.`)) return;
        try {
            await api.delete(`/tenants/${id}`);
            toast("success", "Tenant deleted");
            fetchTenants();
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || "Delete failed";
            toast("error", msg);
        }
    };

    const toggleStatus = async (t: Tenant) => {
        const newStatus = t.status === "ACTIVE" ? "SUSPENDED" : "ACTIVE";
        try {
            await api.put(`/tenants/${t.id}`, { status: newStatus });
            toast("success", `Tenant ${newStatus === "ACTIVE" ? "activated" : "suspended"}`);
            fetchTenants();
        } catch {
            toast("error", "Failed to update status");
        }
    };

    const setLimit = (key: keyof TenantLimits, val: string) => {
        setForm(f => ({ ...f, limits: { ...f.limits, [key]: parseInt(val) || 0 } }));
    };

    const toggleFeature = (key: keyof TenantFeatures) => {
        setForm(f => ({ ...f, features: { ...f.features, [key]: !f.features[key] } }));
    };

    if (!isSuperAdmin()) {
        return (
            <div className="flex justify-center items-center py-20">
                <p className="text-gray-400">🔒 Superadmin access required</p>
            </div>
        );
    }

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">Tenants</h1>
                <button onClick={openCreate} className="bg-purple-600 hover:bg-purple-700 text-white px-4 py-2 rounded-lg text-sm font-medium transition-colors">
                    + New Tenant
                </button>
            </div>

            {loading ? (
                <div className="flex justify-center py-20">
                    <div className="animate-spin w-8 h-8 border-4 border-purple-500 border-t-transparent rounded-full" />
                </div>
            ) : tenants.length === 0 ? (
                <div className="text-center py-20 text-gray-400">
                    <p className="text-4xl mb-2">🏪</p>
                    <p>No tenants yet</p>
                </div>
            ) : (
                <div className="bg-white rounded-xl shadow-sm border overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50">
                            <tr className="text-left text-gray-500 text-xs uppercase tracking-wider">
                                <th className="px-4 py-3 w-8"></th>
                                <th className="px-4 py-3">Code</th>
                                <th className="px-4 py-3">Name</th>
                                <th className="px-4 py-3">Plan</th>
                                <th className="px-4 py-3">Status</th>
                                <th className="px-4 py-3">Features</th>
                                <th className="px-4 py-3 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {tenants.map((t) => (
                                <>
                                    <tr key={t.id} className="border-t hover:bg-gray-50 transition cursor-pointer" onClick={() => toggleExpand(t.id)}>
                                        <td className="px-4 py-3 text-gray-400">{expandedId === t.id ? "▼" : "▶"}</td>
                                        <td className="px-4 py-3 font-mono text-purple-600 font-medium">{t.code}</td>
                                        <td className="px-4 py-3 font-medium">{t.name}</td>
                                        <td className="px-4 py-3">
                                            <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${PLAN_BADGE[t.plan] || PLAN_BADGE.FREE}`}>
                                                {t.plan || "FREE"}
                                            </span>
                                        </td>
                                        <td className="px-4 py-3">
                                            <button
                                                onClick={(e) => { e.stopPropagation(); toggleStatus(t); }}
                                                className={`px-2 py-0.5 rounded-full text-xs font-medium cursor-pointer transition ${STATUS_BADGE[t.status] || STATUS_BADGE.ACTIVE}`}
                                            >
                                                {t.status || "ACTIVE"}
                                            </button>
                                        </td>
                                        <td className="px-4 py-3">
                                            <span className="text-xs text-gray-400">
                                                {t.features ? Object.values(t.features).filter(Boolean).length : 0}/7 enabled
                                            </span>
                                        </td>
                                        <td className="px-4 py-3 text-right space-x-2" onClick={(e) => e.stopPropagation()}>
                                            <button onClick={() => openEdit(t)} className="text-blue-600 hover:underline text-xs">Edit</button>
                                            <button onClick={() => handleDelete(t.id, t.name)} className="text-red-600 hover:underline text-xs">Delete</button>
                                        </td>
                                    </tr>
                                    {expandedId === t.id && (
                                        <tr key={`${t.id}-detail`} className="bg-gray-50/50">
                                            <td colSpan={7} className="px-6 py-4">
                                                <UsagePanel tenant={t} usage={usageMap[t.id]} onRefresh={() => fetchUsage(t.id)} />
                                            </td>
                                        </tr>
                                    )}
                                </>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}

            {/* Modal */}
            {showModal && (
                <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 overflow-y-auto py-4">
                    <form onSubmit={handleSubmit} className="bg-white rounded-xl p-6 w-full max-w-2xl space-y-5 shadow-xl m-4">
                        <h2 className="text-lg font-semibold">{editId ? "Edit Tenant" : "New Tenant"}</h2>
                        {error && <p className="text-red-500 text-sm">{error}</p>}

                        {/* Basic info */}
                        <div className="grid grid-cols-2 gap-4">
                            <div>
                                <label className="text-xs text-gray-500 block mb-1">Code</label>
                                <input value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value })} className="w-full px-3 py-2 border rounded-lg text-sm focus:border-purple-500 focus:outline-none" placeholder="TEN-001" required />
                            </div>
                            <div>
                                <label className="text-xs text-gray-500 block mb-1">Name</label>
                                <input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} className="w-full px-3 py-2 border rounded-lg text-sm focus:border-purple-500 focus:outline-none" placeholder="Acme Corp" required />
                            </div>
                            <div>
                                <label className="text-xs text-gray-500 block mb-1">Plan</label>
                                <select value={form.plan} onChange={(e) => setForm({ ...form, plan: e.target.value })} className="w-full px-3 py-2 border rounded-lg text-sm bg-white focus:border-purple-500 focus:outline-none">
                                    <option value="FREE">FREE</option>
                                    <option value="PRO">PRO</option>
                                    <option value="ENTERPRISE">ENTERPRISE</option>
                                </select>
                            </div>
                            <div>
                                <label className="text-xs text-gray-500 block mb-1">Status</label>
                                <select value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value })} className="w-full px-3 py-2 border rounded-lg text-sm bg-white focus:border-purple-500 focus:outline-none">
                                    <option value="ACTIVE">ACTIVE</option>
                                    <option value="SUSPENDED">SUSPENDED</option>
                                </select>
                            </div>
                        </div>

                        {/* Limits */}
                        <div>
                            <h3 className="text-sm font-semibold text-gray-700 mb-2">📏 Limits</h3>
                            <div className="grid grid-cols-2 gap-3">
                                {([
                                    ["max_warehouses", "Max Warehouses"],
                                    ["max_users", "Max Users"],
                                    ["max_products", "Max Products"],
                                    ["max_daily_orders", "Max Daily Orders"],
                                ] as const).map(([key, label]) => (
                                    <div key={key}>
                                        <label className="text-xs text-gray-500 block mb-1">{label}</label>
                                        <input
                                            type="number" min="0"
                                            value={form.limits[key]}
                                            onChange={(e) => setLimit(key, e.target.value)}
                                            className="w-full px-3 py-2 border rounded-lg text-sm focus:border-purple-500 focus:outline-none"
                                        />
                                    </div>
                                ))}
                            </div>
                        </div>

                        {/* Features */}
                        <div>
                            <h3 className="text-sm font-semibold text-gray-700 mb-2">🎛️ Features</h3>
                            <div className="grid grid-cols-2 gap-2">
                                {(Object.keys(FEATURE_LABELS) as (keyof TenantFeatures)[]).map((key) => (
                                    <label key={key} className="flex items-center gap-2 p-2 rounded-lg hover:bg-gray-50 cursor-pointer">
                                        <div
                                            onClick={() => toggleFeature(key)}
                                            className={`relative w-10 h-5 rounded-full transition-colors cursor-pointer ${form.features[key] ? "bg-purple-500" : "bg-gray-300"}`}
                                        >
                                            <div className={`absolute top-0.5 w-4 h-4 bg-white rounded-full shadow transition-transform ${form.features[key] ? "translate-x-5" : "translate-x-0.5"}`} />
                                        </div>
                                        <span className="text-sm">{FEATURE_LABELS[key]}</span>
                                    </label>
                                ))}
                            </div>
                        </div>

                        <div className="flex justify-end gap-3 pt-2 border-t">
                            <button type="button" onClick={() => setShowModal(false)} className="px-4 py-2 text-sm text-gray-500 hover:text-gray-700 transition-colors">Cancel</button>
                            <button type="submit" className="bg-purple-600 hover:bg-purple-700 text-white px-4 py-2 rounded-lg text-sm font-medium transition-colors">{editId ? "Update" : "Create"}</button>
                        </div>
                    </form>
                </div>
            )}
        </div>
    );
}

/* ── Usage Panel ── */
function UsagePanel({ tenant, usage, onRefresh }: { tenant: Tenant; usage?: TenantUsage; onRefresh: () => void }) {
    if (!usage) {
        return (
            <div className="flex items-center gap-2 text-gray-400 text-sm">
                <div className="animate-spin w-4 h-4 border-2 border-purple-500 border-t-transparent rounded-full" />
                Loading usage...
            </div>
        );
    }

    const limits = tenant.limits || DEFAULT_LIMITS;
    const bars: { label: string; current: number; max: number; icon: string }[] = [
        { label: "Warehouses", current: usage.warehouses, max: limits.max_warehouses, icon: "🏭" },
        { label: "Users", current: usage.users, max: limits.max_users, icon: "👥" },
        { label: "Products", current: usage.products, max: limits.max_products, icon: "📦" },
        { label: "Today Orders", current: usage.today_orders, max: limits.max_daily_orders, icon: "📋" },
    ];

    return (
        <div>
            <div className="flex items-center justify-between mb-3">
                <h4 className="text-sm font-semibold text-gray-700">📊 Usage</h4>
                <button onClick={onRefresh} className="text-xs text-purple-600 hover:underline">↻ Refresh</button>
            </div>
            <div className="grid grid-cols-2 gap-3">
                {bars.map(b => {
                    const pct = b.max > 0 ? Math.min((b.current / b.max) * 100, 100) : 0;
                    const color = pct >= 90 ? "bg-red-500" : pct >= 70 ? "bg-amber-500" : "bg-green-500";
                    return (
                        <div key={b.label} className="p-3 bg-white rounded-lg border">
                            <div className="flex justify-between text-xs text-gray-500 mb-1">
                                <span>{b.icon} {b.label}</span>
                                <span className="font-mono font-medium">{b.current} / {b.max}</span>
                            </div>
                            <div className="w-full bg-gray-200 rounded-full h-2">
                                <div className={`${color} h-2 rounded-full transition-all`} style={{ width: `${pct}%` }} />
                            </div>
                        </div>
                    );
                })}
            </div>
            <div className="mt-3 grid grid-cols-2 gap-2">
                {(Object.keys(FEATURE_LABELS) as (keyof TenantFeatures)[]).map(key => (
                    <div key={key} className="flex items-center gap-2 text-xs">
                        <span className={`w-2 h-2 rounded-full ${tenant.features?.[key] ? "bg-green-500" : "bg-gray-300"}`} />
                        <span className={tenant.features?.[key] ? "text-gray-700" : "text-gray-400"}>{FEATURE_LABELS[key]}</span>
                    </div>
                ))}
            </div>
            {/* Billing info (read-only for superadmin) */}
            {(tenant.stripe_customer_id || tenant.billing_status) && (
                <div className="mt-3 pt-3 border-t">
                    <h4 className="text-xs font-semibold text-gray-500 mb-2">💳 Billing</h4>
                    <div className="grid grid-cols-2 gap-2 text-xs">
                        {tenant.stripe_customer_id && (
                            <div>
                                <span className="text-gray-400">Stripe ID:</span>
                                <span className="ml-1 font-mono text-gray-600">{tenant.stripe_customer_id}</span>
                            </div>
                        )}
                        {tenant.billing_status && (
                            <div>
                                <span className="text-gray-400">Billing:</span>
                                <span className={`ml-1 px-1.5 py-0.5 rounded text-xs font-medium ${tenant.billing_status === "ACTIVE" ? "bg-green-100 text-green-700" :
                                        tenant.billing_status === "PAST_DUE" ? "bg-red-100 text-red-700" :
                                            "bg-gray-100 text-gray-600"
                                    }`}>{tenant.billing_status}</span>
                            </div>
                        )}
                    </div>
                </div>
            )}
        </div>
    );
}
