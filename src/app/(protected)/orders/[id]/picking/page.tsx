"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import api from "@/lib/api";
import { can } from "@/lib/auth";
import type { PickTask, Order, Product, Location, PaginatedResponse } from "@/lib/types";

const STATUS_COLORS: Record<string, string> = {
    OPEN: "#3b82f6",
    IN_PROGRESS: "#f59e0b",
    DONE: "#22c55e",
    CANCELLED: "#6b7280",
};

export default function PickingPage() {
    const { id } = useParams();
    const router = useRouter();
    const [order, setOrder] = useState<Order | null>(null);
    const [tasks, setTasks] = useState<PickTask[]>([]);
    const [products, setProducts] = useState<Record<string, Product>>({});
    const [locations, setLocations] = useState<Record<string, Location>>({});
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");
    const [assignUser, setAssignUser] = useState("");
    const [assigningId, setAssigningId] = useState<string | null>(null);
    const [actionMsg, setActionMsg] = useState("");


    const loadData = useCallback(async () => {
        try {
            const [orderRes, tasksRes] = await Promise.all([
                api.get<Order>(`/orders/${id}`),
                api.get<PaginatedResponse<PickTask>>(`/orders/${id}/pick-tasks`),
            ]);
            setOrder(orderRes.data);

            const taskList: PickTask[] = tasksRes.data.data || [];
            setTasks(taskList);

            // Load product & location names
            const prodIds = [...new Set(taskList.map((t) => t.product_id))];
            const locIds = [...new Set(taskList.map((t) => t.location_id))];

            const pm: Record<string, Product> = {};
            const lm: Record<string, Location> = {};

            await Promise.all([
                ...prodIds.map(async (pid) => {
                    try {
                        const r = await api.get<Product>(`/products/${pid}`);
                        pm[pid] = r.data;
                    } catch { }
                }),
                ...locIds.map(async (lid) => {
                    try {
                        const r = await api.get<Location>(`/locations/${lid}`);
                        lm[lid] = r.data;
                    } catch { }
                }),
            ]);
            setProducts(pm);
            setLocations(lm);
        } catch (e: any) {
            setError(e.response?.data?.error || e.message || "Failed to load");
        } finally {
            setLoading(false);
        }
    }, [id]);

    useEffect(() => { loadData(); }, [loadData]);

    const handleAssign = async (taskId: string) => {
        if (!assignUser.trim()) return;
        try {
            await api.post(`/pick-tasks/${taskId}/assign`, { assign_to: assignUser.trim() });
            setActionMsg("✅ Task assigned");
            setAssigningId(null);
            setAssignUser("");
            loadData();
        } catch (e: any) {
            setActionMsg(`❌ ${e.response?.data?.error || "Network error"}`);
        }
    };

    // Group tasks by location
    const grouped = tasks.reduce<Record<string, PickTask[]>>((acc, t) => {
        const key = t.location_id;
        if (!acc[key]) acc[key] = [];
        acc[key].push(t);
        return acc;
    }, {});

    const totalPlanned = tasks.reduce((s, t) => s + t.planned_qty, 0);
    const totalPicked = tasks.reduce((s, t) => s + t.picked_qty, 0);
    const progress = totalPlanned > 0 ? (totalPicked / totalPlanned) * 100 : 0;

    if (loading) return <div style={{ padding: 32, textAlign: "center" }}>Loading pick tasks...</div>;
    if (error) return <div style={{ padding: 32, color: "#ef4444" }}>{error}</div>;

    return (
        <div style={{ padding: "24px", maxWidth: 960, margin: "0 auto" }}>
            {/* Header */}
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 24 }}>
                <div>
                    <button onClick={() => router.push(`/orders/${id}`)} style={{ background: "none", border: "none", color: "#3b82f6", cursor: "pointer", fontSize: 14, marginBottom: 8 }}>
                        ← Back to Order
                    </button>
                    <h1 style={{ fontSize: 24, fontWeight: 700, margin: 0 }}>
                        Pick Tasks — {order?.order_no}
                    </h1>
                    <p style={{ color: "#6b7280", margin: "4px 0 0" }}>
                        {tasks.length} tasks · {order?.client_name}
                    </p>
                </div>
                <button
                    onClick={() => router.push(`/pick/scan?orderId=${id}`)}
                    style={{ padding: "12px 24px", background: "#8b5cf6", color: "#fff", border: "none", borderRadius: 8, fontWeight: 600, fontSize: 16, cursor: "pointer" }}
                >
                    🎯 Scan to Pick
                </button>
            </div>

            {actionMsg && (
                <div style={{ padding: "10px 16px", background: actionMsg.startsWith("✅") ? "#dcfce7" : "#fee2e2", borderRadius: 8, marginBottom: 16, fontSize: 14 }}>
                    {actionMsg}
                </div>
            )}

            {/* Overall Progress */}
            <div style={{ background: "#f8fafc", borderRadius: 12, padding: 20, marginBottom: 24, border: "1px solid #e2e8f0" }}>
                <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 8 }}>
                    <span style={{ fontWeight: 600 }}>Overall Progress</span>
                    <span style={{ fontWeight: 600 }}>{totalPicked} / {totalPlanned} ({progress.toFixed(0)}%)</span>
                </div>
                <div style={{ height: 12, background: "#e2e8f0", borderRadius: 6, overflow: "hidden" }}>
                    <div style={{ height: "100%", width: `${progress}%`, background: progress === 100 ? "#22c55e" : "#3b82f6", borderRadius: 6, transition: "width 0.3s" }} />
                </div>
            </div>

            {/* Tasks grouped by location */}
            {Object.entries(grouped).map(([locId, locTasks]) => {
                const loc = locations[locId];
                const locLabel = loc ? `${loc.zone}-${loc.aisle}${loc.rack}-${loc.level} (${loc.code})` : locId;
                return (
                    <div key={locId} style={{ marginBottom: 20, border: "1px solid #e2e8f0", borderRadius: 12, overflow: "hidden" }}>
                        <div style={{ background: "#f1f5f9", padding: "12px 16px", fontWeight: 600, fontSize: 14, display: "flex", alignItems: "center", gap: 8 }}>
                            📍 {locLabel}
                        </div>
                        {locTasks.map((t) => {
                            const prod = products[t.product_id];
                            const pct = t.planned_qty > 0 ? (t.picked_qty / t.planned_qty) * 100 : 0;
                            return (
                                <div key={t.id} style={{ padding: "14px 16px", borderTop: "1px solid #e2e8f0", display: "flex", alignItems: "center", gap: 12, flexWrap: "wrap" }}>
                                    <span style={{ background: STATUS_COLORS[t.status] || "#6b7280", color: "#fff", padding: "2px 10px", borderRadius: 12, fontSize: 11, fontWeight: 600, textTransform: "uppercase", minWidth: 90, textAlign: "center" }}>
                                        {t.status.replace("_", " ")}
                                    </span>
                                    <span style={{ flex: 1, fontWeight: 500 }}>{prod ? `${prod.sku} — ${prod.name}` : t.product_id}</span>
                                    <div style={{ width: 160 }}>
                                        <div style={{ fontSize: 12, color: "#6b7280", marginBottom: 2 }}>{t.picked_qty} / {t.planned_qty}</div>
                                        <div style={{ height: 6, background: "#e2e8f0", borderRadius: 3, overflow: "hidden" }}>
                                            <div style={{ height: "100%", width: `${pct}%`, background: pct === 100 ? "#22c55e" : "#f59e0b", borderRadius: 3 }} />
                                        </div>
                                    </div>
                                    {t.assigned_to && <span style={{ fontSize: 12, color: "#6b7280" }}>👤 {t.assigned_to}</span>}

                                    {can("orders:manage") && t.status !== "DONE" && t.status !== "CANCELLED" && (
                                        <>
                                            {assigningId === t.id ? (
                                                <div style={{ display: "flex", gap: 4 }}>
                                                    <input
                                                        value={assignUser}
                                                        onChange={(e) => setAssignUser(e.target.value)}
                                                        placeholder="User ID"
                                                        style={{ padding: "4px 8px", border: "1px solid #d1d5db", borderRadius: 6, fontSize: 12, width: 120 }}
                                                    />
                                                    <button onClick={() => handleAssign(t.id)} style={{ padding: "4px 8px", background: "#3b82f6", color: "#fff", border: "none", borderRadius: 6, fontSize: 12, cursor: "pointer" }}>OK</button>
                                                    <button onClick={() => setAssigningId(null)} style={{ padding: "4px 8px", background: "#e5e7eb", border: "none", borderRadius: 6, fontSize: 12, cursor: "pointer" }}>✕</button>
                                                </div>
                                            ) : (
                                                <button onClick={() => setAssigningId(t.id)} style={{ padding: "4px 10px", background: "#e0e7ff", color: "#4f46e5", border: "none", borderRadius: 6, fontSize: 12, cursor: "pointer", fontWeight: 500 }}>Assign</button>
                                            )}
                                        </>
                                    )}
                                </div>
                            );
                        })}
                    </div>
                );
            })}

            {tasks.length === 0 && (
                <div style={{ textAlign: "center", padding: 48, color: "#9ca3af" }}>No pick tasks generated yet.</div>
            )}
        </div>
    );
}
