"use client";
import { useEffect, useState } from "react";
import api from "@/lib/api";
import type { DashboardSummary } from "@/lib/types";

export default function DashboardPage() {
    const [data, setData] = useState<DashboardSummary | null>(null);
    const [loading, setLoading] = useState(true);
    const [from, setFrom] = useState("");
    const [to, setTo] = useState("");

    const fetchData = async () => {
        setLoading(true);
        try {
            const params = new URLSearchParams();
            if (from) params.set("from", from);
            if (to) params.set("to", to);
            const res = await api.get<DashboardSummary>(`/dashboard/summary?${params}`);
            setData(res.data);
        } catch { /* toast handles 403 */ }
        setLoading(false);
    };

    useEffect(() => { fetchData(); }, []);

    const cards = data ? [
        { label: "Total Stock Qty", value: data.total_stock_qty.toLocaleString(), color: "bg-blue-600" },
        { label: "Inbound Qty", value: data.inbound_qty_total.toLocaleString(), color: "bg-emerald-600" },
        { label: "Outbound Qty", value: data.outbound_qty_total.toLocaleString(), color: "bg-orange-600" },
        { label: "Adjustments", value: data.adjustments_count.toString(), color: "bg-amber-600" },
        { label: "Adj. Net Qty", value: (data.adjustments_qty_net >= 0 ? "+" : "") + data.adjustments_qty_net.toString(), color: "bg-teal-600" },
        { label: "Open Orders", value: (data.open_orders_count ?? 0).toString(), color: "bg-violet-600" },
        { label: "Reserved Qty", value: (data.reserved_qty_total ?? 0).toLocaleString(), color: "bg-rose-600" },
        { label: "Picking Orders", value: (data.picking_orders_count ?? 0).toString(), color: "bg-yellow-600" },
        { label: "Pick Tasks Open", value: (data.pick_tasks_open ?? 0).toString(), color: "bg-orange-600" },
        { label: "Inbound Count", value: data.inbound_count.toString(), color: "bg-cyan-600" },
        { label: "Outbound Count", value: data.outbound_count.toString(), color: "bg-purple-600" },
        { label: "Products", value: data.total_products.toString(), color: "bg-indigo-600" },
        { label: "Locations", value: data.total_locations.toString(), color: "bg-pink-600" },
    ] : [];

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">Dashboard</h1>
                <div className="flex items-center gap-3">
                    <input type="date" value={from} onChange={(e) => setFrom(e.target.value)}
                        className="px-3 py-1.5 border rounded-lg text-sm" placeholder="From" />
                    <input type="date" value={to} onChange={(e) => setTo(e.target.value)}
                        className="px-3 py-1.5 border rounded-lg text-sm" placeholder="To" />
                    <button onClick={fetchData}
                        className="px-4 py-1.5 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 transition">
                        Filter
                    </button>
                </div>
            </div>

            {loading ? (
                <div className="flex justify-center py-20">
                    <div className="animate-spin w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full" />
                </div>
            ) : data ? (
                <>
                    {/* Stats Cards */}
                    <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-7 gap-4 mb-8">
                        {cards.map((c) => (
                            <div key={c.label} className={`${c.color} text-white rounded-xl p-4 shadow-lg`}>
                                <p className="text-xs opacity-80">{c.label}</p>
                                <p className="text-2xl font-bold mt-1">{c.value}</p>
                            </div>
                        ))}
                    </div>

                    {/* Tables */}
                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                        {/* Top Moving Products */}
                        <div className="bg-white rounded-xl shadow-sm border p-5">
                            <h2 className="text-lg font-semibold mb-4">🔥 Top Moving Products</h2>
                            {data.top_moving_products.length === 0 ? (
                                <p className="text-gray-400 text-sm">No data yet</p>
                            ) : (
                                <table className="w-full text-sm">
                                    <thead><tr className="text-left text-gray-500 border-b">
                                        <th className="pb-2">Product ID</th><th className="pb-2 text-right">Total Qty</th>
                                    </tr></thead>
                                    <tbody>
                                        {data.top_moving_products.map((p, i) => (
                                            <tr key={i} className="border-b last:border-0">
                                                <td className="py-2 font-mono text-xs">{p.product_id}</td>
                                                <td className="py-2 text-right font-semibold">{p.total_qty.toLocaleString()}</td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            )}
                        </div>

                        {/* Stock by Zone */}
                        <div className="bg-white rounded-xl shadow-sm border p-5">
                            <h2 className="text-lg font-semibold mb-4">🗺️ Stock by Zone</h2>
                            {data.stock_by_zone.length === 0 ? (
                                <p className="text-gray-400 text-sm">No data yet</p>
                            ) : (
                                <table className="w-full text-sm">
                                    <thead><tr className="text-left text-gray-500 border-b">
                                        <th className="pb-2">Zone</th><th className="pb-2 text-right">Quantity</th>
                                    </tr></thead>
                                    <tbody>
                                        {data.stock_by_zone.map((z, i) => (
                                            <tr key={i} className="border-b last:border-0">
                                                <td className="py-2 font-medium">{z.zone}</td>
                                                <td className="py-2 text-right font-semibold">{z.quantity.toLocaleString()}</td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            )}
                        </div>
                    </div>
                </>
            ) : (
                <p className="text-gray-400">Failed to load dashboard data.</p>
            )}
        </div>
    );
}
