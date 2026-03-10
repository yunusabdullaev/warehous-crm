"use client";
import { useEffect, useState } from "react";
import api from "@/lib/api";
import type { StockRecord, Product, Location, Lot, PaginatedResponse } from "@/lib/types";

export default function StockPage() {
    const [stocks, setStocks] = useState<StockRecord[]>([]);
    const [products, setProducts] = useState<Product[]>([]);
    const [locations, setLocations] = useState<Location[]>([]);
    const [lots, setLots] = useState<Lot[]>([]);
    const [loading, setLoading] = useState(true);
    const [filter, setFilter] = useState("");

    useEffect(() => {
        (async () => {
            setLoading(true);
            try {
                const [sRes, pRes, lRes, lotRes] = await Promise.all([
                    api.get<PaginatedResponse<StockRecord>>("/stock?limit=500"),
                    api.get<PaginatedResponse<Product>>("/products?limit=500"),
                    api.get<PaginatedResponse<Location>>("/locations?limit=500"),
                    api.get<Lot[]>("/lots"),
                ]);
                setStocks(sRes.data.data || []);
                setProducts(pRes.data.data || []);
                setLocations(lRes.data.data || []);
                const lotData = Array.isArray(lotRes.data) ? lotRes.data : (lotRes.data as unknown as PaginatedResponse<Lot>).data || [];
                setLots(lotData);
            } catch { /* interceptor */ }
            setLoading(false);
        })();
    }, []);

    const pMap = Object.fromEntries(products.map((p) => [p.id, p]));
    const lMap = Object.fromEntries(locations.map((l) => [l.id, l]));
    const lotMap = Object.fromEntries(lots.map((l) => [l.id, l]));

    const filtered = stocks.filter((s) => {
        if (!filter) return true;
        const q = filter.toLowerCase();
        const pName = pMap[s.product_id]?.name?.toLowerCase() || "";
        const lName = lMap[s.location_id]?.name?.toLowerCase() || "";
        const zone = lMap[s.location_id]?.zone?.toLowerCase() || "";
        const lotNo = lotMap[s.lot_id]?.lot_no?.toLowerCase() || "";
        return pName.includes(q) || lName.includes(q) || zone.includes(q) || lotNo.includes(q);
    });

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">Stock</h1>
                <input
                    type="text"
                    value={filter}
                    onChange={(e) => setFilter(e.target.value)}
                    placeholder="Filter by product, location, zone, lot..."
                    className="px-4 py-2 border rounded-lg text-sm w-72"
                />
            </div>

            {loading ? (
                <div className="flex justify-center py-20">
                    <div className="animate-spin w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full" />
                </div>
            ) : filtered.length === 0 ? (
                <div className="text-center py-20 text-gray-400">
                    <p className="text-4xl mb-2">🏭</p>
                    <p>{stocks.length === 0 ? "No stock records yet" : "No results matching filter"}</p>
                </div>
            ) : (
                <div className="bg-white rounded-xl shadow-sm border overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50">
                            <tr className="text-left text-gray-500 text-xs uppercase tracking-wider">
                                <th className="px-4 py-3">Product</th>
                                <th className="px-4 py-3">SKU</th>
                                <th className="px-4 py-3">Location</th>
                                <th className="px-4 py-3">Zone</th>
                                <th className="px-4 py-3">Lot</th>
                                <th className="px-4 py-3">Exp Date</th>
                                <th className="px-4 py-3 text-right">Quantity</th>
                                <th className="px-4 py-3">Last Updated</th>
                            </tr>
                        </thead>
                        <tbody>
                            {filtered.map((s) => {
                                const prod = pMap[s.product_id];
                                const loc = lMap[s.location_id];
                                const lot = lotMap[s.lot_id];
                                const isExpired = lot?.exp_date && new Date(lot.exp_date).getTime() < Date.now();
                                return (
                                    <tr key={s.id} className="border-t hover:bg-gray-50">
                                        <td className="px-4 py-3 font-medium">{prod?.name || s.product_id.slice(-6)}</td>
                                        <td className="px-4 py-3 font-mono text-xs text-gray-500">{prod?.sku || "—"}</td>
                                        <td className="px-4 py-3">{loc?.name || s.location_id.slice(-6)}</td>
                                        <td className="px-4 py-3">{loc?.zone || "—"}</td>
                                        <td className="px-4 py-3 font-mono text-xs">{lot?.lot_no || "—"}</td>
                                        <td className={`px-4 py-3 text-xs ${isExpired ? "text-red-600 font-bold" : "text-gray-500"}`}>
                                            {lot?.exp_date ? new Date(lot.exp_date).toLocaleDateString() : "—"}
                                        </td>
                                        <td className={`px-4 py-3 text-right font-bold ${s.quantity > 0 ? "text-emerald-600" : "text-red-600"}`}>
                                            {s.quantity.toLocaleString()}
                                        </td>
                                        <td className="px-4 py-3 text-gray-500 text-xs">
                                            {s.last_updated ? new Date(s.last_updated).toLocaleString() : "—"}
                                        </td>
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>
                    <div className="px-4 py-3 text-xs text-gray-400 border-t">
                        Showing {filtered.length} of {stocks.length} records
                    </div>
                </div>
            )}
        </div>
    );
}
