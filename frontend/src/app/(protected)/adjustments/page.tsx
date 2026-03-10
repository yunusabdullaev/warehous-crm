"use client";

import { useState, useEffect, useCallback } from "react";
import { useForm } from "react-hook-form";
import api from "@/lib/api";
import type { AdjustmentRecord, Product, Location, PaginatedResponse } from "@/lib/types";
import { can } from "@/lib/auth";

interface AdjustmentForm {
    product_id: string;
    location_id: string;
    delta_qty: number;
    reason: string;
    note: string;
}

const REASONS = ["DAMAGED", "LOST", "FOUND", "COUNT_CORRECTION", "OTHER"];

export default function AdjustmentsPage() {
    const [adjustments, setAdjustments] = useState<AdjustmentRecord[]>([]);
    const [products, setProducts] = useState<Product[]>([]);
    const [locations, setLocations] = useState<Location[]>([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(1);
    const [showModal, setShowModal] = useState(false);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState("");
    const { register, handleSubmit, reset, formState: { errors } } = useForm<AdjustmentForm>();

    const fetchAdjustments = useCallback(async () => {
        try {
            const { data } = await api.get<PaginatedResponse<AdjustmentRecord>>("/adjustments", {
                params: { page, limit: 20 },
            });
            setAdjustments(data.data || []);
            setTotal(data.total);
        } catch {
            console.error("Failed to fetch adjustments");
        }
    }, [page]);

    useEffect(() => {
        fetchAdjustments();
        api.get<PaginatedResponse<Product>>("/products", { params: { limit: 200 } }).then(r => setProducts(r.data.data || []));
        api.get<PaginatedResponse<Location>>("/locations", { params: { limit: 200 } }).then(r => setLocations(r.data.data || []));
    }, [fetchAdjustments]);

    const onSubmit = async (form: AdjustmentForm) => {
        setLoading(true);
        setError("");
        try {
            await api.post("/adjustments", {
                product_id: form.product_id,
                location_id: form.location_id,
                delta_qty: Number(form.delta_qty),
                reason: form.reason,
                note: form.note,
            });
            reset();
            setShowModal(false);
            fetchAdjustments();
        } catch (err: unknown) {
            const axiosErr = err as { response?: { data?: { error?: string }; status?: number } };
            const msg = axiosErr?.response?.data?.error || "Failed to create adjustment";
            setError(msg);
        } finally {
            setLoading(false);
        }
    };

    const productName = (id: string) => products.find(p => p.id === id)?.name || id.slice(0, 8);
    const locationCode = (id: string) => locations.find(l => l.id === id)?.code || id.slice(0, 8);
    const totalPages = Math.ceil(total / 20);

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <h1 className="text-2xl font-bold text-gray-100">Adjustments</h1>
                {can("adjustments:create") && (
                    <button
                        onClick={() => { setShowModal(true); setError(""); }}
                        className="px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 transition"
                    >
                        + New Adjustment
                    </button>
                )}
            </div>

            {/* Table */}
            <div className="bg-gray-800 rounded-xl border border-gray-700 overflow-x-auto">
                <table className="min-w-full text-sm text-gray-300">
                    <thead className="bg-gray-750 text-gray-400 uppercase text-xs">
                        <tr>
                            <th className="px-4 py-3 text-left">Date</th>
                            <th className="px-4 py-3 text-left">Product</th>
                            <th className="px-4 py-3 text-left">Location</th>
                            <th className="px-4 py-3 text-right">Delta</th>
                            <th className="px-4 py-3 text-left">Reason</th>
                            <th className="px-4 py-3 text-left">Note</th>
                            <th className="px-4 py-3 text-left">By</th>
                        </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-700">
                        {adjustments.map(a => (
                            <tr key={a.id} className="hover:bg-gray-750 transition">
                                <td className="px-4 py-3 whitespace-nowrap">{new Date(a.created_at).toLocaleDateString()}</td>
                                <td className="px-4 py-3">{productName(a.product_id)}</td>
                                <td className="px-4 py-3">{locationCode(a.location_id)}</td>
                                <td className={`px-4 py-3 text-right font-mono font-semibold ${a.delta_qty > 0 ? "text-emerald-400" : "text-red-400"}`}>
                                    {a.delta_qty > 0 ? `+${a.delta_qty}` : a.delta_qty}
                                </td>
                                <td className="px-4 py-3">
                                    <span className="px-2 py-0.5 rounded-full text-xs bg-gray-700 text-gray-300">{a.reason}</span>
                                </td>
                                <td className="px-4 py-3 text-gray-400 max-w-[200px] truncate">{a.note || "—"}</td>
                                <td className="px-4 py-3 text-gray-400">{a.created_by}</td>
                            </tr>
                        ))}
                        {adjustments.length === 0 && (
                            <tr><td colSpan={7} className="px-4 py-8 text-center text-gray-500">No adjustments yet</td></tr>
                        )}
                    </tbody>
                </table>
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
                <div className="flex justify-center gap-2">
                    <button disabled={page <= 1} onClick={() => setPage(p => p - 1)}
                        className="px-3 py-1 rounded bg-gray-700 text-gray-300 disabled:opacity-40 hover:bg-gray-600">Prev</button>
                    <span className="px-3 py-1 text-gray-400">{page}/{totalPages}</span>
                    <button disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}
                        className="px-3 py-1 rounded bg-gray-700 text-gray-300 disabled:opacity-40 hover:bg-gray-600">Next</button>
                </div>
            )}

            {/* Modal */}
            {showModal && (
                <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
                    <div className="bg-gray-800 rounded-2xl p-6 w-full max-w-md border border-gray-700 shadow-xl">
                        <h2 className="text-lg font-semibold text-gray-100 mb-4">New Stock Adjustment</h2>
                        {error && <div className="mb-3 p-2 bg-red-900/50 border border-red-700 rounded text-red-300 text-sm">{error}</div>}
                        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
                            <div>
                                <label className="block text-sm text-gray-400 mb-1">Product</label>
                                <select {...register("product_id", { required: true })}
                                    className="w-full bg-gray-700 border border-gray-600 rounded-lg p-2 text-gray-200">
                                    <option value="">Select...</option>
                                    {products.map(p => <option key={p.id} value={p.id}>{p.name} ({p.sku})</option>)}
                                </select>
                                {errors.product_id && <span className="text-red-400 text-xs">Required</span>}
                            </div>
                            <div>
                                <label className="block text-sm text-gray-400 mb-1">Location</label>
                                <select {...register("location_id", { required: true })}
                                    className="w-full bg-gray-700 border border-gray-600 rounded-lg p-2 text-gray-200">
                                    <option value="">Select...</option>
                                    {locations.map(l => <option key={l.id} value={l.id}>{l.code} — {l.name}</option>)}
                                </select>
                                {errors.location_id && <span className="text-red-400 text-xs">Required</span>}
                            </div>
                            <div>
                                <label className="block text-sm text-gray-400 mb-1">Quantity (+/-)</label>
                                <input type="number" {...register("delta_qty", { required: true, validate: v => v !== 0 || "Cannot be zero" })}
                                    className="w-full bg-gray-700 border border-gray-600 rounded-lg p-2 text-gray-200" placeholder="e.g. -3 or +10" />
                                {errors.delta_qty && <span className="text-red-400 text-xs">{errors.delta_qty.message || "Required"}</span>}
                            </div>
                            <div>
                                <label className="block text-sm text-gray-400 mb-1">Reason</label>
                                <select {...register("reason", { required: true })}
                                    className="w-full bg-gray-700 border border-gray-600 rounded-lg p-2 text-gray-200">
                                    <option value="">Select...</option>
                                    {REASONS.map(r => <option key={r} value={r}>{r.replace("_", " ")}</option>)}
                                </select>
                                {errors.reason && <span className="text-red-400 text-xs">Required</span>}
                            </div>
                            <div>
                                <label className="block text-sm text-gray-400 mb-1">Note (optional)</label>
                                <textarea {...register("note")} rows={2}
                                    className="w-full bg-gray-700 border border-gray-600 rounded-lg p-2 text-gray-200" />
                            </div>
                            <div className="flex justify-end gap-3 pt-2">
                                <button type="button" onClick={() => setShowModal(false)}
                                    className="px-4 py-2 text-gray-400 hover:text-gray-200 transition">Cancel</button>
                                <button type="submit" disabled={loading}
                                    className="px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 disabled:opacity-50 transition">
                                    {loading ? "Creating..." : "Create"}
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}
        </div>
    );
}
