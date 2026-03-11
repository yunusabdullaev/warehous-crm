"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import api from "@/lib/api";
import { can } from "@/lib/auth";
import type { Order, Reservation, Product, PaginatedResponse } from "@/lib/types";

const STATUS_COLORS: Record<string, string> = {
    DRAFT: "bg-gray-100 text-gray-700",
    CONFIRMED: "bg-blue-100 text-blue-700",
    PICKING: "bg-yellow-100 text-yellow-700",
    SHIPPED: "bg-emerald-100 text-emerald-700",
    CANCELLED: "bg-red-100 text-red-700",
};

export default function OrderDetailPage() {
    const { id } = useParams<{ id: string }>();
    const router = useRouter();
    const [order, setOrder] = useState<Order | null>(null);
    const [reservations, setReservations] = useState<Reservation[]>([]);
    const [products, setProducts] = useState<Record<string, Product>>({});
    const [loading, setLoading] = useState(true);
    const [actionError, setActionError] = useState("");
    const [acting, setActing] = useState("");


    const fetchOrder = useCallback(async () => {
        try {
            const res = await api.get<Order>(`/orders/${id}`);
            const o = res.data;
            setOrder(o);
            // Fetch product names for items
            const productIds = [...new Set(o.items.map((it) => it.product_id))];
            const prodMap: Record<string, Product> = {};
            await Promise.all(
                productIds.map(async (pid) => {
                    try {
                        const pr = await api.get<Product>(`/products/${pid}`);
                        prodMap[pid] = pr.data;
                    } catch { }
                })
            );
            setProducts(prodMap);
        } catch (err) {
            console.error("fetchOrder failed", err);
        }
        setLoading(false);
    }, [id]);

    const fetchReservations = useCallback(async () => {
        try {
            const res = await api.get<PaginatedResponse<Reservation>>("/reservations", {
                params: { orderId: id, limit: 100 }
            });
            setReservations(res.data.data || []);
        } catch { }
    }, [id]);

    useEffect(() => {
        fetchOrder();
        fetchReservations();
    }, [fetchOrder, fetchReservations]);

    const doAction = async (action: string, label: string) => {
        setActing(action);
        setActionError("");
        try {
            await api.post(`/orders/${id}/${action}`);
            fetchOrder();
            fetchReservations();
        } catch (err: any) {
            const res = err.response;
            if (res?.status === 409) {
                setActionError(`${label} conflict: ${res.data?.error}`);
            } else if (res?.status === 400) {
                setActionError(res.data?.error || "Bad request");
            } else {
                setActionError(res.data?.error || "Action failed");
            }
        }
        setActing("");
    };

    if (loading) return <div className="text-center py-12 text-gray-400">Loading…</div>;
    if (!order) return <div className="text-center py-12 text-red-500">Order not found</div>;

    return (
        <div className="max-w-4xl mx-auto">
            {/* Header */}
            <div className="flex items-center gap-3 mb-6">
                <button onClick={() => router.push("/orders")} className="text-gray-500 hover:text-gray-700">← Back</button>
                <h1 className="text-2xl font-bold flex-1">{order.order_no}</h1>
                <span className={`px-3 py-1 rounded-full text-sm font-medium ${STATUS_COLORS[order.status] || ""}`}>
                    {order.status}
                </span>
            </div>

            {/* Error */}
            {actionError && (
                <div className="bg-red-50 text-red-700 p-3 rounded-lg mb-4 text-sm">{actionError}</div>
            )}

            {/* Actions */}
            {can("orders:manage") && (
                <div className="flex gap-2 mb-6 flex-wrap">
                    {order.status === "DRAFT" && (
                        <>
                            <button
                                onClick={() => doAction("confirm", "Confirm")}
                                disabled={acting !== ""}
                                className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 transition"
                            >
                                {acting === "confirm" ? "Confirming…" : "✓ Confirm Order"}
                            </button>
                            <button
                                onClick={() => doAction("cancel", "Cancel")}
                                disabled={acting !== ""}
                                className="px-4 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100 disabled:opacity-50 transition"
                            >
                                {acting === "cancel" ? "Cancelling…" : "✕ Cancel"}
                            </button>
                        </>
                    )}
                    {order.status === "CONFIRMED" && (
                        <>
                            <button
                                onClick={() => doAction("start-pick", "Start Pick")}
                                disabled={acting !== ""}
                                className="px-4 py-2 bg-yellow-600 text-white rounded-lg hover:bg-yellow-700 disabled:opacity-50 transition"
                            >
                                {acting === "start-pick" ? "Starting…" : "📋 Start Picking"}
                            </button>
                            <button
                                onClick={() => doAction("cancel", "Cancel")}
                                disabled={acting !== ""}
                                className="px-4 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100 disabled:opacity-50 transition"
                            >
                                {acting === "cancel" ? "Cancelling…" : "✕ Cancel"}
                            </button>
                        </>
                    )}
                    {order.status === "PICKING" && (
                        <>
                            <button
                                onClick={() => router.push(`/orders/${id}/picking`)}
                                className="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 transition"
                            >
                                📋 View Pick Tasks
                            </button>
                            <button
                                onClick={() => router.push(`/pick/scan?orderId=${id}`)}
                                className="px-4 py-2 bg-violet-600 text-white rounded-lg hover:bg-violet-700 transition"
                            >
                                🎯 Scan to Pick
                            </button>
                            <button
                                onClick={() => doAction("ship", "Ship")}
                                disabled={acting !== ""}
                                className="px-4 py-2 bg-emerald-600 text-white rounded-lg hover:bg-emerald-700 disabled:opacity-50 transition"
                            >
                                {acting === "ship" ? "Shipping…" : "🚚 Ship Order"}
                            </button>
                            <button
                                onClick={() => doAction("cancel", "Cancel")}
                                disabled={acting !== ""}
                                className="px-4 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100 disabled:opacity-50 transition"
                            >
                                {acting === "cancel" ? "Cancelling…" : "✕ Cancel"}
                            </button>
                        </>
                    )}
                </div>
            )}

            {/* Order info */}
            <div className="bg-white rounded-xl shadow p-5 mb-6 grid grid-cols-2 gap-4 text-sm">
                <div>
                    <span className="text-gray-500">Client</span>
                    <p className="font-medium">{order.client_name}</p>
                </div>
                <div>
                    <span className="text-gray-500">Created By</span>
                    <p className="font-medium">{order.created_by}</p>
                </div>
                <div>
                    <span className="text-gray-500">Created At</span>
                    <p>{new Date(order.created_at).toLocaleString()}</p>
                </div>
                {order.confirmed_at && (
                    <div>
                        <span className="text-gray-500">Confirmed At</span>
                        <p>{new Date(order.confirmed_at).toLocaleString()}</p>
                    </div>
                )}
                {order.shipped_at && (
                    <div>
                        <span className="text-gray-500">Shipped At</span>
                        <p>{new Date(order.shipped_at).toLocaleString()}</p>
                    </div>
                )}
                {order.cancelled_at && (
                    <div>
                        <span className="text-gray-500">Cancelled At</span>
                        <p>{new Date(order.cancelled_at).toLocaleString()}</p>
                    </div>
                )}
                {order.notes && (
                    <div className="col-span-2">
                        <span className="text-gray-500">Notes</span>
                        <p>{order.notes}</p>
                    </div>
                )}
            </div>

            {/* Items table */}
            <h2 className="text-lg font-semibold mb-3">Items</h2>
            <div className="overflow-x-auto bg-white rounded-xl shadow mb-6">
                <table className="min-w-full text-sm">
                    <thead className="bg-gray-50 text-gray-600 uppercase text-xs">
                        <tr>
                            <th className="px-4 py-3 text-left">Product</th>
                            <th className="px-4 py-3 text-right">Requested</th>
                            <th className="px-4 py-3 text-right">Reserved</th>
                            <th className="px-4 py-3 text-right">Shipped</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y">
                        {order.items.map((item, i) => {
                            const prod = products[item.product_id];
                            return (
                                <tr key={i}>
                                    <td className="px-4 py-3">
                                        {prod ? `${prod.name} (${prod.sku})` : item.product_id}
                                    </td>
                                    <td className="px-4 py-3 text-right font-mono">{item.requested_qty}</td>
                                    <td className="px-4 py-3 text-right font-mono text-blue-600">{item.reserved_qty}</td>
                                    <td className="px-4 py-3 text-right font-mono text-emerald-600">{item.shipped_qty}</td>
                                </tr>
                            );
                        })}
                    </tbody>
                </table>
            </div>

            {/* Reservations */}
            {reservations.length > 0 && (
                <>
                    <h2 className="text-lg font-semibold mb-3">Reservations</h2>
                    <div className="overflow-x-auto bg-white rounded-xl shadow">
                        <table className="min-w-full text-sm">
                            <thead className="bg-gray-50 text-gray-600 uppercase text-xs">
                                <tr>
                                    <th className="px-4 py-3 text-left">Product</th>
                                    <th className="px-4 py-3 text-right">Qty</th>
                                    <th className="px-4 py-3 text-left">Status</th>
                                    <th className="px-4 py-3 text-left">Reason</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y">
                                {reservations.map((r) => {
                                    const prod = products[r.product_id];
                                    return (
                                        <tr key={r.id} className={r.status === "RELEASED" ? "opacity-50" : ""}>
                                            <td className="px-4 py-3">{prod ? prod.name : r.product_id}</td>
                                            <td className="px-4 py-3 text-right font-mono">{r.qty}</td>
                                            <td className="px-4 py-3">
                                                <span className={`px-2 py-0.5 rounded-full text-xs ${r.status === "ACTIVE" ? "bg-blue-100 text-blue-700" : "bg-gray-100 text-gray-600"}`}>
                                                    {r.status}
                                                </span>
                                            </td>
                                            <td className="px-4 py-3 text-gray-500 text-xs">{r.reason || "—"}</td>
                                        </tr>
                                    );
                                })}
                            </tbody>
                        </table>
                    </div>
                </>
            )}
        </div>
    );
}
