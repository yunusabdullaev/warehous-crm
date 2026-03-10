"use client";
import { useEffect, useState, useCallback } from "react";
import api from "@/lib/api";
import { isSuperAdmin } from "@/lib/auth";
import type { Tenant, PaginatedResponse } from "@/lib/types";

interface Warehouse {
    id: string;
    code: string;
    name: string;
    address: string;
    is_default: boolean;
    tenant_id?: string;
    created_at: string;
}

export default function WarehousesPage() {
    const [warehouses, setWarehouses] = useState<Warehouse[]>([]);
    const [loading, setLoading] = useState(true);
    const [showModal, setShowModal] = useState(false);
    const [editId, setEditId] = useState<string | null>(null);
    const [form, setForm] = useState({ code: "", name: "", address: "" });
    const [error, setError] = useState("");
    const [tenants, setTenants] = useState<Tenant[]>([]);
    const superadmin = isSuperAdmin();

    const fetchWarehouses = async () => {
        try {
            setLoading(true);
            const res = await api.get("/warehouses");
            setWarehouses(res.data?.data ?? res.data ?? []);
        } catch {
            setError("Failed to load warehouses");
        } finally {
            setLoading(false);
        }
    };

    const fetchTenants = useCallback(async () => {
        if (!superadmin) return;
        try {
            const res = await api.get<PaginatedResponse<Tenant>>("/tenants?limit=100");
            setTenants(res.data.data || []);
        } catch { /* silently fail */ }
    }, [superadmin]);

    useEffect(() => { fetchWarehouses(); fetchTenants(); }, [fetchTenants]);

    const getTenantName = (id?: string) => {
        if (!id) return "—";
        const t = tenants.find(t => t.id === id);
        return t ? t.name : id.slice(-6);
    };

    const openCreate = () => {
        setEditId(null);
        setForm({ code: "", name: "", address: "" });
        setError("");
        setShowModal(true);
    };

    const openEdit = (w: Warehouse) => {
        setEditId(w.id);
        setForm({ code: w.code, name: w.name, address: w.address || "" });
        setError("");
        setShowModal(true);
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setError("");
        try {
            if (editId) {
                await api.put(`/warehouses/${editId}`, form);
            } else {
                await api.post("/warehouses", form);
            }
            setShowModal(false);
            fetchWarehouses();
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || "Operation failed";
            setError(msg);
        }
    };

    const handleDelete = async (id: string, name: string) => {
        if (!confirm(`Delete warehouse "${name}"? This cannot be undone.`)) return;
        try {
            await api.delete(`/warehouses/${id}`);
            fetchWarehouses();
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || "Delete failed";
            alert(msg);
        }
    };

    return (
        <div className="p-6 max-w-4xl">
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">Warehouses</h1>
                <button
                    onClick={openCreate}
                    className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg text-sm font-medium transition-colors"
                >
                    + New Warehouse
                </button>
            </div>

            {loading ? (
                <p className="text-gray-400">Loading…</p>
            ) : (
                <div className="bg-gray-800 rounded-xl overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-700">
                            <tr>
                                <th className="text-left px-4 py-3 text-gray-300 font-medium">Code</th>
                                <th className="text-left px-4 py-3 text-gray-300 font-medium">Name</th>
                                <th className="text-left px-4 py-3 text-gray-300 font-medium">Address</th>
                                {superadmin && <th className="text-left px-4 py-3 text-gray-300 font-medium">Tenant</th>}
                                <th className="text-left px-4 py-3 text-gray-300 font-medium">Default</th>
                                <th className="text-right px-4 py-3 text-gray-300 font-medium">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-gray-700">
                            {warehouses.map((w) => (
                                <tr key={w.id} className="hover:bg-gray-750">
                                    <td className="px-4 py-3 font-mono text-blue-400">{w.code}</td>
                                    <td className="px-4 py-3">{w.name}</td>
                                    <td className="px-4 py-3 text-gray-400">{w.address || "—"}</td>
                                    {superadmin && (
                                        <td className="px-4 py-3">
                                            <span className="bg-purple-900/30 text-purple-300 text-xs px-2 py-0.5 rounded-full">
                                                {getTenantName(w.tenant_id)}
                                            </span>
                                        </td>
                                    )}
                                    <td className="px-4 py-3">
                                        {w.is_default ? (
                                            <span className="bg-green-900 text-green-300 text-xs px-2 py-0.5 rounded-full">Default</span>
                                        ) : "—"}
                                    </td>
                                    <td className="px-4 py-3 text-right space-x-2">
                                        <button
                                            onClick={() => openEdit(w)}
                                            className="text-blue-400 hover:text-blue-300 text-xs"
                                        >
                                            Edit
                                        </button>
                                        {!w.is_default && (
                                            <button
                                                onClick={() => handleDelete(w.id, w.name)}
                                                className="text-red-400 hover:text-red-300 text-xs"
                                            >
                                                Delete
                                            </button>
                                        )}
                                    </td>
                                </tr>
                            ))}
                            {warehouses.length === 0 && (
                                <tr>
                                    <td colSpan={superadmin ? 6 : 5} className="px-4 py-8 text-center text-gray-500">
                                        No warehouses found
                                    </td>
                                </tr>
                            )}
                        </tbody>
                    </table>
                </div>
            )}

            {/* Modal */}
            {showModal && (
                <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
                    <form onSubmit={handleSubmit} className="bg-gray-800 rounded-xl p-6 w-full max-w-md space-y-4">
                        <h2 className="text-lg font-semibold">
                            {editId ? "Edit Warehouse" : "New Warehouse"}
                        </h2>

                        {error && <p className="text-red-400 text-sm">{error}</p>}

                        <div>
                            <label className="text-xs text-gray-400 block mb-1">Code</label>
                            <input
                                value={form.code}
                                onChange={(e) => setForm({ ...form, code: e.target.value })}
                                className="w-full bg-gray-700 text-white rounded px-3 py-2 text-sm border border-gray-600 focus:border-blue-500 focus:outline-none"
                                placeholder="WH-001"
                                required
                            />
                        </div>
                        <div>
                            <label className="text-xs text-gray-400 block mb-1">Name</label>
                            <input
                                value={form.name}
                                onChange={(e) => setForm({ ...form, name: e.target.value })}
                                className="w-full bg-gray-700 text-white rounded px-3 py-2 text-sm border border-gray-600 focus:border-blue-500 focus:outline-none"
                                placeholder="Main Warehouse"
                                required
                            />
                        </div>
                        <div>
                            <label className="text-xs text-gray-400 block mb-1">Address</label>
                            <input
                                value={form.address}
                                onChange={(e) => setForm({ ...form, address: e.target.value })}
                                className="w-full bg-gray-700 text-white rounded px-3 py-2 text-sm border border-gray-600 focus:border-blue-500 focus:outline-none"
                                placeholder="123 Industrial Dr"
                            />
                        </div>

                        <div className="flex justify-end gap-3 pt-2">
                            <button
                                type="button"
                                onClick={() => setShowModal(false)}
                                className="px-4 py-2 text-sm text-gray-400 hover:text-white transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                type="submit"
                                className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg text-sm font-medium transition-colors"
                            >
                                {editId ? "Update" : "Create"}
                            </button>
                        </div>
                    </form>
                </div>
            )}
        </div>
    );
}
