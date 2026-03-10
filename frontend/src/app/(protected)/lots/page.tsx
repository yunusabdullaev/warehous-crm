"use client";
import { useEffect, useState } from "react";
import api from "@/lib/api";
import type { Lot, Product, PaginatedResponse } from "@/lib/types";

export default function LotsPage() {
    const [lots, setLots] = useState<Lot[]>([]);
    const [products, setProducts] = useState<Product[]>([]);
    const [loading, setLoading] = useState(true);
    const [filter, setFilter] = useState("");

    useEffect(() => {
        (async () => {
            setLoading(true);
            try {
                const [lotRes, pRes] = await Promise.all([
                    api.get<Lot[]>("/lots"),
                    api.get<PaginatedResponse<Product>>("/products?limit=500"),
                ]);
                setLots(Array.isArray(lotRes.data) ? lotRes.data : (lotRes.data as unknown as PaginatedResponse<Lot>).data || []);
                setProducts(pRes.data.data || []);
            } catch { /* interceptor */ }
            setLoading(false);
        })();
    }, []);

    const pMap = Object.fromEntries(products.map((p) => [p.id, p]));

    const filtered = lots.filter((l) => {
        if (!filter) return true;
        const q = filter.toLowerCase();
        const pName = pMap[l.product_id]?.name?.toLowerCase() || "";
        return l.lot_no.toLowerCase().includes(q) || pName.includes(q);
    });

    const isExpiringSoon = (d?: string) => {
        if (!d) return false;
        const diff = (new Date(d).getTime() - Date.now()) / (1000 * 60 * 60 * 24);
        return diff >= 0 && diff <= 30;
    };
    const isExpired = (d?: string) => {
        if (!d) return false;
        return new Date(d).getTime() < Date.now();
    };

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">🏷️ Lots / Batch</h1>
                <input
                    type="text"
                    value={filter}
                    onChange={(e) => setFilter(e.target.value)}
                    placeholder="Filter by lot number, product..."
                    className="px-4 py-2 border rounded-lg text-sm w-72"
                />
            </div>

            {loading ? (
                <div className="flex justify-center py-20">
                    <div className="animate-spin w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full" />
                </div>
            ) : filtered.length === 0 ? (
                <div className="text-center py-20 text-gray-400">
                    <p className="text-4xl mb-2">🏷️</p>
                    <p>{lots.length === 0 ? "No lots yet. Lots are created via inbound." : "No lots matching filter"}</p>
                </div>
            ) : (
                <div className="bg-white rounded-xl shadow-sm border overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50">
                            <tr className="text-left text-gray-500 text-xs uppercase tracking-wider">
                                <th className="px-4 py-3">Lot No</th>
                                <th className="px-4 py-3">Product</th>
                                <th className="px-4 py-3">SKU</th>
                                <th className="px-4 py-3">Mfg Date</th>
                                <th className="px-4 py-3">Exp Date</th>
                                <th className="px-4 py-3">Status</th>
                                <th className="px-4 py-3">Created</th>
                            </tr>
                        </thead>
                        <tbody>
                            {filtered.map((l) => {
                                const prod = pMap[l.product_id];
                                const expired = isExpired(l.exp_date);
                                const expiring = isExpiringSoon(l.exp_date);
                                return (
                                    <tr key={l.id} className="border-t hover:bg-gray-50">
                                        <td className="px-4 py-3 font-mono font-bold">{l.lot_no}</td>
                                        <td className="px-4 py-3">{prod?.name || l.product_id.slice(-6)}</td>
                                        <td className="px-4 py-3 font-mono text-xs text-gray-500">{prod?.sku || "—"}</td>
                                        <td className="px-4 py-3 text-gray-500 text-xs">
                                            {l.mfg_date ? new Date(l.mfg_date).toLocaleDateString() : "—"}
                                        </td>
                                        <td className={`px-4 py-3 font-medium ${expired ? "text-red-600" : expiring ? "text-amber-600" : "text-gray-700"}`}>
                                            {l.exp_date ? new Date(l.exp_date).toLocaleDateString() : "—"}
                                        </td>
                                        <td className="px-4 py-3">
                                            {expired ? (
                                                <span className="inline-block px-2 py-0.5 text-xs font-semibold bg-red-100 text-red-800 rounded-full">EXPIRED</span>
                                            ) : expiring ? (
                                                <span className="inline-block px-2 py-0.5 text-xs font-semibold bg-amber-100 text-amber-800 rounded-full">EXPIRING SOON</span>
                                            ) : l.exp_date ? (
                                                <span className="inline-block px-2 py-0.5 text-xs font-semibold bg-emerald-100 text-emerald-800 rounded-full">OK</span>
                                            ) : (
                                                <span className="inline-block px-2 py-0.5 text-xs font-semibold bg-gray-100 text-gray-600 rounded-full">NO EXP</span>
                                            )}
                                        </td>
                                        <td className="px-4 py-3 text-gray-500 text-xs">
                                            {new Date(l.created_at).toLocaleDateString()}
                                        </td>
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>
                    <div className="px-4 py-3 text-xs text-gray-400 border-t">
                        Showing {filtered.length} of {lots.length} lots
                    </div>
                </div>
            )}
        </div>
    );
}
