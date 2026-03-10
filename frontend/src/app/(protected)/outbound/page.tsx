"use client";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import api from "@/lib/api";
import AuthGuard from "@/components/AuthGuard";
import { toast } from "@/components/Toast";
import { getUser } from "@/lib/auth";
import type { OutboundRecord, Product, Location, PaginatedResponse } from "@/lib/types";

interface FormData {
    product_id: string;
    location_id: string;
    quantity: number;
    reference?: string;
}

export default function OutboundPage() {
    return (
        <AuthGuard requiredAction="outbound:create">
            <OutboundContent />
        </AuthGuard>
    );
}

function OutboundContent() {
    const [records, setRecords] = useState<OutboundRecord[]>([]);
    const [products, setProducts] = useState<Product[]>([]);
    const [locations, setLocations] = useState<Location[]>([]);
    const [loading, setLoading] = useState(true);
    const [showForm, setShowForm] = useState(false);
    const [reversingId, setReversingId] = useState<string | null>(null);

    const { register, handleSubmit, reset, formState: { errors } } = useForm<FormData>();

    const user = getUser();
    const role = user?.role;

    const fetchAll = async () => {
        setLoading(true);
        try {
            const [outRes, pRes, lRes] = await Promise.all([
                api.get<PaginatedResponse<OutboundRecord>>("/outbound?limit=100"),
                api.get<PaginatedResponse<Product>>("/products?limit=100"),
                api.get<PaginatedResponse<Location>>("/locations?limit=100"),
            ]);
            setRecords(outRes.data.data || []);
            setProducts(pRes.data.data || []);
            setLocations(lRes.data.data || []);
        } catch { /* interceptor */ }
        setLoading(false);
    };

    useEffect(() => { fetchAll(); }, []);

    const onSubmit = async (data: FormData) => {
        try {
            await api.post("/outbound", data);
            toast("success", "Outbound created");
            reset();
            setShowForm(false);
            fetchAll();
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Failed to create outbound");
        }
    };

    const canReverse = (r: OutboundRecord) => {
        if (r.status === "REVERSED") return false;
        if (role === "admin") return true;
        if (role === "operator") {
            const ownRecord = r.user_id === user?.id;
            const within24h = (Date.now() - new Date(r.created_at).getTime()) < 24 * 60 * 60 * 1000;
            return ownRecord && within24h;
        }
        return false;
    };

    const handleReverse = async (id: string) => {
        const reason = prompt("Reason for reversal:");
        if (!reason) return;
        setReversingId(id);
        try {
            await api.post(`/outbound/${id}/reverse`, { reason });
            toast("success", "Outbound reversed");
            fetchAll();
        } catch (err: unknown) {
            const axiosErr = err as { response?: { data?: { error?: string }; status?: number } };
            const status = axiosErr?.response?.status;
            const msg = axiosErr?.response?.data?.error;
            if (status === 409) toast("error", msg || "Already reversed");
            else if (status === 403) toast("error", msg || "Not authorized to reverse");
            else toast("error", msg || "Reversal failed");
        } finally {
            setReversingId(null);
        }
    };

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">Outbound</h1>
                <button onClick={() => { reset(); setShowForm(!showForm); }}
                    className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 transition">
                    {showForm ? "Cancel" : "+ New Outbound"}
                </button>
            </div>

            {showForm && (
                <div className="bg-white rounded-xl shadow-sm border p-5 mb-6">
                    <h2 className="text-lg font-semibold mb-4">Create Outbound</h2>
                    <form onSubmit={handleSubmit(onSubmit)} className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div>
                            <label className="block text-sm font-medium mb-1">Product *</label>
                            <select {...register("product_id", { required: "Product is required" })} className="w-full px-3 py-2 border rounded-lg text-sm">
                                <option value="">Select product</option>
                                {products.map((p) => <option key={p.id} value={p.id}>{p.name} ({p.sku})</option>)}
                            </select>
                            {errors.product_id && <p className="text-red-500 text-xs mt-1">{errors.product_id.message}</p>}
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Location *</label>
                            <select {...register("location_id", { required: "Location is required" })} className="w-full px-3 py-2 border rounded-lg text-sm">
                                <option value="">Select location</option>
                                {locations.map((l) => <option key={l.id} value={l.id}>{l.name} ({l.code})</option>)}
                            </select>
                            {errors.location_id && <p className="text-red-500 text-xs mt-1">{errors.location_id.message}</p>}
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Quantity *</label>
                            <input {...register("quantity", { required: "Quantity is required", valueAsNumber: true, min: { value: 1, message: "Qty must be > 0" } })} type="number" min={1} className="w-full px-3 py-2 border rounded-lg text-sm" />
                            {errors.quantity && <p className="text-red-500 text-xs mt-1">{errors.quantity.message}</p>}
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Reference</label>
                            <input {...register("reference")} className="w-full px-3 py-2 border rounded-lg text-sm" placeholder="SO-2024-001" />
                        </div>
                        <div className="md:col-span-2">
                            <button type="submit" className="px-6 py-2 bg-orange-600 text-white rounded-lg text-sm hover:bg-orange-700 transition">
                                Create Outbound
                            </button>
                        </div>
                    </form>
                </div>
            )}

            {loading ? (
                <div className="flex justify-center py-20">
                    <div className="animate-spin w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full" />
                </div>
            ) : records.length === 0 ? (
                <div className="text-center py-20 text-gray-400">
                    <p className="text-4xl mb-2">📤</p>
                    <p>No outbound records yet</p>
                </div>
            ) : (
                <div className="bg-white rounded-xl shadow-sm border overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50">
                            <tr className="text-left text-gray-500 text-xs uppercase tracking-wider">
                                <th className="px-4 py-3">ID</th>
                                <th className="px-4 py-3">Product</th>
                                <th className="px-4 py-3">Location</th>
                                <th className="px-4 py-3 text-right">Qty</th>
                                <th className="px-4 py-3">Reference</th>
                                <th className="px-4 py-3">Status</th>
                                <th className="px-4 py-3">Date</th>
                                <th className="px-4 py-3">Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {records.map((r) => (
                                <tr key={r.id} className={`border-t hover:bg-gray-50 ${r.status === "REVERSED" ? "opacity-60" : ""}`}>
                                    <td className="px-4 py-3 font-mono text-xs">{r.id.slice(-6)}</td>
                                    <td className="px-4 py-3">{products.find((p) => p.id === r.product_id)?.name || r.product_id.slice(-6)}</td>
                                    <td className="px-4 py-3">{locations.find((l) => l.id === r.location_id)?.name || r.location_id.slice(-6)}</td>
                                    <td className="px-4 py-3 text-right font-semibold text-orange-600">-{r.quantity}</td>
                                    <td className="px-4 py-3 text-gray-500">{r.reference || "—"}</td>
                                    <td className="px-4 py-3">
                                        {(r.status === "REVERSED") ? (
                                            <span className="px-2 py-0.5 rounded-full text-xs bg-red-100 text-red-700" title={r.reverse_reason}>REVERSED</span>
                                        ) : (
                                            <span className="px-2 py-0.5 rounded-full text-xs bg-emerald-100 text-emerald-700">ACTIVE</span>
                                        )}
                                    </td>
                                    <td className="px-4 py-3 text-gray-500 text-xs">{new Date(r.created_at).toLocaleDateString()}</td>
                                    <td className="px-4 py-3">
                                        {canReverse(r) && (
                                            <button
                                                onClick={() => handleReverse(r.id)}
                                                disabled={reversingId === r.id}
                                                className="px-2 py-1 text-xs bg-red-50 text-red-600 rounded hover:bg-red-100 transition disabled:opacity-50"
                                            >
                                                {reversingId === r.id ? "..." : "Reverse"}
                                            </button>
                                        )}
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </div>
    );
}
