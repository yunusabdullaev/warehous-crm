"use client";
import { useEffect, useState } from "react";
import api from "@/lib/api";
import AuthGuard from "@/components/AuthGuard";
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from "recharts";
import type { MovementsReport, StockReport } from "@/lib/types";

export default function ReportsPage() {
    return (
        <AuthGuard requiredAction="reports:view">
            <ReportsContent />
        </AuthGuard>
    );
}

function ReportsContent() {
    const [movements, setMovements] = useState<MovementsReport | null>(null);
    const [stockReport, setStockReport] = useState<StockReport | null>(null);
    const [groupBy, setGroupBy] = useState("day");
    const [stockGroupBy, setStockGroupBy] = useState("zone");
    const [from, setFrom] = useState(() => {
        const d = new Date();
        d.setDate(d.getDate() - 30);
        return d.toISOString().split("T")[0];
    });
    const [to, setTo] = useState(() => new Date().toISOString().split("T")[0]);
    const [loading, setLoading] = useState(true);

    const fetchMovements = async () => {
        setLoading(true);
        try {
            const res = await api.get<MovementsReport>(`/reports/movements?from=${from}&to=${to}&groupBy=${groupBy}`);
            setMovements(res.data);
        } catch { /* interceptor */ }
        setLoading(false);
    };

    const fetchStock = async () => {
        try {
            const res = await api.get<StockReport>(`/reports/stock?groupBy=${stockGroupBy}`);
            setStockReport(res.data);
        } catch { /* interceptor */ }
    };

    useEffect(() => { fetchMovements(); }, []);
    useEffect(() => { fetchStock(); }, [stockGroupBy]);

    return (
        <div>
            <h1 className="text-2xl font-bold mb-6">Reports</h1>

            {/* Movements Chart */}
            <div className="bg-white rounded-xl shadow-sm border p-5 mb-6">
                <div className="flex items-center justify-between mb-4">
                    <h2 className="text-lg font-semibold">📈 Movements</h2>
                    <div className="flex items-center gap-3">
                        <input type="date" value={from} onChange={(e) => setFrom(e.target.value)}
                            className="px-3 py-1.5 border rounded-lg text-sm" />
                        <input type="date" value={to} onChange={(e) => setTo(e.target.value)}
                            className="px-3 py-1.5 border rounded-lg text-sm" />
                        <select value={groupBy} onChange={(e) => setGroupBy(e.target.value)}
                            className="px-3 py-1.5 border rounded-lg text-sm">
                            <option value="day">Daily</option>
                            <option value="week">Weekly</option>
                            <option value="month">Monthly</option>
                        </select>
                        <button onClick={fetchMovements}
                            className="px-4 py-1.5 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 transition">
                            Apply
                        </button>
                    </div>
                </div>

                {loading ? (
                    <div className="flex justify-center py-20">
                        <div className="animate-spin w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full" />
                    </div>
                ) : movements && movements.data.length > 0 ? (
                    <ResponsiveContainer width="100%" height={350}>
                        <BarChart data={movements.data} margin={{ top: 5, right: 20, bottom: 5, left: 0 }}>
                            <CartesianGrid strokeDasharray="3 3" stroke="#f0f0f0" />
                            <XAxis dataKey="period" tick={{ fontSize: 12 }} />
                            <YAxis tick={{ fontSize: 12 }} />
                            <Tooltip />
                            <Legend />
                            <Bar dataKey="inbound_qty" name="Inbound" fill="#10b981" radius={[4, 4, 0, 0]} />
                            <Bar dataKey="outbound_qty" name="Outbound" fill="#f97316" radius={[4, 4, 0, 0]} />
                        </BarChart>
                    </ResponsiveContainer>
                ) : (
                    <p className="text-gray-400 text-sm py-10 text-center">No movements data for this period</p>
                )}
            </div>

            {/* Stock Grouping */}
            <div className="bg-white rounded-xl shadow-sm border p-5">
                <div className="flex items-center justify-between mb-4">
                    <h2 className="text-lg font-semibold">🗃️ Stock Grouping</h2>
                    <select value={stockGroupBy} onChange={(e) => setStockGroupBy(e.target.value)}
                        className="px-3 py-1.5 border rounded-lg text-sm">
                        <option value="zone">By Zone</option>
                        <option value="rack">By Rack</option>
                        <option value="product">By Product</option>
                    </select>
                </div>

                {stockReport && stockReport.data.length > 0 ? (
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                        {stockReport.data.map((d, i) => (
                            <div key={i} className="bg-gray-50 rounded-lg p-4 text-center">
                                <p className="text-xs text-gray-500 mb-1">{d.group || "Unknown"}</p>
                                <p className="text-xl font-bold text-gray-800">{d.quantity.toLocaleString()}</p>
                            </div>
                        ))}
                    </div>
                ) : (
                    <p className="text-gray-400 text-sm py-10 text-center">No stock data</p>
                )}
            </div>
        </div>
    );
}
