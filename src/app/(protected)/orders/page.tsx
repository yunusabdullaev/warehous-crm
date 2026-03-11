"use client";

import { useEffect, useState, useCallback } from "react";
import api from "@/lib/api";
import { getToken, can, getUser } from "@/lib/auth";
import type { Order, Product, PaginatedResponse } from "@/lib/types";
import { useRouter } from "next/navigation";

const STATUS_COLORS: Record<string, string> = {
    DRAFT: "bg-gray-100 text-gray-700",
    CONFIRMED: "bg-blue-100 text-blue-700",
    PICKING: "bg-yellow-100 text-yellow-700",
    SHIPPED: "bg-emerald-100 text-emerald-700",
    CANCELLED: "bg-red-100 text-red-700",
};

export default function OrdersPage() {
    const [orders, setOrders] = useState<Order[]>([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(1);
    const [statusFilter, setStatusFilter] = useState("");
    const [clientFilter, setClientFilter] = useState("");
    const [loading, setLoading] = useState(true);

    // Create modal state
    const [showCreate, setShowCreate] = useState(false);
    const [products, setProducts] = useState<Product[]>([]);
    const [clientName, setClientName] = useState("");
    const [notes, setNotes] = useState("");
    const [items, setItems] = useState<{ product_id: string; requested_qty: number }[]>([
        { product_id: "", requested_qty: 1 },
    ]);
    const [creating, setCreating] = useState(false);
    const [error, setError] = useState("");

    const router = useRouter();
    const limit = 20;

    const fetchOrders = useCallback(async () => {
        setLoading(true);
        try {
            const params: Record<string, string> = { page: String(page), limit: String(limit) };
            if (statusFilter) params.status = statusFilter;
            if (clientFilter) params.client = clientFilter;

            const res = await api.get<PaginatedResponse<Order>>("/orders", { params });
            setOrders(res.data.data || []);
            setTotal(res.data.total);
        } catch (err) {
            console.error("fetchOrders failed", err);
        }
        setLoading(false);
    }, [page, statusFilter, clientFilter]);

    useEffect(() => { fetchOrders(); }, [fetchOrders]);

    useEffect(() => {
        // Fetch products for create modal
        api.get<PaginatedResponse<Product>>("/products", { params: { limit: 200 } })
            .then((res) => setProducts(res.data.data || []));
    }, []);

    const handleCreate = async () => {
        setCreating(true);
        setError("");
        try {
            const res = await api.post("/orders", { client_name: clientName, notes, items });
            setShowCreate(false);
            setClientName("");
            setNotes("");
            setItems([{ product_id: "", requested_qty: 1 }]);
            fetchOrders();
        } catch (err: any) {
            setError(err.response?.data?.error || "Failed to create order");
        }
        setCreating(false);
    };

    const addItem = () => setItems([...items, { product_id: "", requested_qty: 1 }]);
    const removeItem = (idx: number) => setItems(items.filter((_, i) => i !== idx));
    const updateItem = (idx: number, field: string, value: string | number) => {
        const updated = [...items];
        (updated[idx] as Record<string, string | number>)[field] = value;
        setItems(updated);
    };

    const totalPages = Math.ceil(total / limit);

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">Orders</h1>
                {can("orders:create") && (
                    <button
                        onClick={() => setShowCreate(true)}
                        className="px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 transition"
                    >
                        + New Order
                    </button>
                )}
            </div>

            {/* Filters */}
            <div className="flex gap-3 mb-4">
                <select
                    value={statusFilter}
                    onChange={(e) => { setStatusFilter(e.target.value); setPage(1); }}
                    className="px-3 py-2 border rounded-lg text-sm"
                >
                    <option value="">All Statuses</option>
                    <option value="DRAFT">Draft</option>
                    <option value="CONFIRMED">Confirmed</option>
                    <option value="PICKING">Picking</option>
                    <option value="SHIPPED">Shipped</option>
                    <option value="CANCELLED">Cancelled</option>
                </select>
                <input
                    type="text"
                    placeholder="Search client…"
                    value={clientFilter}
                    onChange={(e) => { setClientFilter(e.target.value); setPage(1); }}
                    className="px-3 py-2 border rounded-lg text-sm flex-1 max-w-xs"
                />
            </div>

            {/* Table */}
            <div className="overflow-x-auto bg-white rounded-xl shadow">
                <table className="min-w-full text-sm">
                    <thead className="bg-gray-50 text-gray-600 uppercase text-xs">
                        <tr>
                            <th className="px-4 py-3 text-left">Order No</th>
                            <th className="px-4 py-3 text-left">Client</th>
                            <th className="px-4 py-3 text-left">Status</th>
                            <th className="px-4 py-3 text-left">Items</th>
                            <th className="px-4 py-3 text-left">Created</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y">
                        {loading ? (
                            <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-400">Loading…</td></tr>
                        ) : orders.length === 0 ? (
                            <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-400">No orders found</td></tr>
                        ) : (
                            orders.map((o) => (
                                <tr
                                    key={o.id}
                                    className="hover:bg-gray-50 cursor-pointer transition"
                                    onClick={() => router.push(`/orders/${o.id}`)}
                                >
                                    <td className="px-4 py-3 font-mono font-medium">{o.order_no}</td>
                                    <td className="px-4 py-3">{o.client_name}</td>
                                    <td className="px-4 py-3">
                                        <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${STATUS_COLORS[o.status] || ""}`}>
                                            {o.status}
                                        </span>
                                    </td>
                                    <td className="px-4 py-3">{o.items?.length || 0} items</td>
                                    <td className="px-4 py-3 text-gray-500">{new Date(o.created_at).toLocaleDateString()}</td>
                                </tr>
                            ))
                        )}
                    </tbody>
                </table>
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
                <div className="flex justify-center gap-2 mt-4">
                    <button disabled={page <= 1} onClick={() => setPage(page - 1)} className="px-3 py-1 text-sm border rounded disabled:opacity-40">← Prev</button>
                    <span className="px-3 py-1 text-sm">Page {page} of {totalPages}</span>
                    <button disabled={page >= totalPages} onClick={() => setPage(page + 1)} className="px-3 py-1 text-sm border rounded disabled:opacity-40">Next →</button>
                </div>
            )}

            {/* Create Modal */}
            {showCreate && (
                <div className="fixed inset-0 bg-black/40 z-50 flex items-center justify-center p-4">
                    <div className="bg-white rounded-2xl shadow-xl w-full max-w-lg max-h-[90vh] overflow-y-auto p-6">
                        <h2 className="text-xl font-bold mb-4">Create Order</h2>
                        {error && <p className="text-red-600 text-sm mb-3 bg-red-50 p-2 rounded">{error}</p>}

                        <label className="block text-sm font-medium mb-1">Client Name *</label>
                        <input
                            value={clientName}
                            onChange={(e) => setClientName(e.target.value)}
                            className="w-full px-3 py-2 border rounded-lg mb-3"
                            placeholder="Client name"
                        />

                        <label className="block text-sm font-medium mb-1">Notes</label>
                        <input
                            value={notes}
                            onChange={(e) => setNotes(e.target.value)}
                            className="w-full px-3 py-2 border rounded-lg mb-3"
                            placeholder="Optional notes"
                        />

                        <div className="flex items-center justify-between mb-2">
                            <label className="text-sm font-medium">Items</label>
                            <button onClick={addItem} className="text-xs text-indigo-600 hover:underline">+ Add Item</button>
                        </div>

                        {items.map((item, i) => (
                            <div key={i} className="flex gap-2 mb-2 items-center">
                                <select
                                    value={item.product_id}
                                    onChange={(e) => updateItem(i, "product_id", e.target.value)}
                                    className="flex-1 px-2 py-1.5 border rounded text-sm"
                                >
                                    <option value="">Select product…</option>
                                    {products.map((p) => (
                                        <option key={p.id} value={p.id}>{p.name} ({p.sku})</option>
                                    ))}
                                </select>
                                <input
                                    type="number"
                                    min={1}
                                    value={item.requested_qty}
                                    onChange={(e) => updateItem(i, "requested_qty", parseInt(e.target.value) || 1)}
                                    className="w-20 px-2 py-1.5 border rounded text-sm"
                                />
                                {items.length > 1 && (
                                    <button onClick={() => removeItem(i)} className="text-red-500 text-sm hover:underline">✕</button>
                                )}
                            </div>
                        ))}

                        <div className="flex gap-3 mt-4">
                            <button
                                onClick={handleCreate}
                                disabled={creating || !clientName || items.some((it) => !it.product_id)}
                                className="flex-1 px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 disabled:opacity-50 transition"
                            >
                                {creating ? "Creating…" : "Create Order"}
                            </button>
                            <button
                                onClick={() => { setShowCreate(false); setError(""); }}
                                className="px-4 py-2 border rounded-lg hover:bg-gray-50 transition"
                            >
                                Cancel
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
