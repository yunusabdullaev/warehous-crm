"use client";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import api from "@/lib/api";
import { can, getUser } from "@/lib/auth";
import { toast } from "@/components/Toast";
import CsvImportModal from "@/components/CsvImportModal";
import type { Product, PaginatedResponse } from "@/lib/types";

const schema = z.object({
    sku: z.string().min(1, "SKU is required"),
    name: z.string().min(1, "Name is required"),
    unit: z.string().min(1, "Unit is required"),
    category: z.string().optional(),
    description: z.string().optional(),
});
type FormData = z.infer<typeof schema>;

export default function ProductsPage() {
    const [products, setProducts] = useState<Product[]>([]);
    const [loading, setLoading] = useState(true);
    const [showForm, setShowForm] = useState(false);
    const [editing, setEditing] = useState<Product | null>(null);
    const [showImport, setShowImport] = useState(false);

    const auth = getUser();
    const isAdmin = auth?.role === "admin";

    const { register, handleSubmit, reset, formState: { errors } } = useForm<FormData>({
        resolver: zodResolver(schema),
    });

    const fetchProducts = async () => {
        setLoading(true);
        try {
            const res = await api.get<PaginatedResponse<Product>>("/products?limit=100");
            setProducts(res.data.data || []);
        } catch { /* interceptor handles */ }
        setLoading(false);
    };

    useEffect(() => { fetchProducts(); }, []);

    const onSubmit = async (data: FormData) => {
        try {
            if (editing) {
                await api.put(`/products/${editing.id}`, data);
                toast("success", "Product updated");
            } else {
                await api.post("/products", data);
                toast("success", "Product created");
            }
            reset();
            setShowForm(false);
            setEditing(null);
            fetchProducts();
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Failed to save product");
        }
    };

    const onDelete = async (id: string) => {
        if (!confirm("Delete this product?")) return;
        try {
            await api.delete(`/products/${id}`);
            toast("success", "Product deleted");
            fetchProducts();
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Failed to delete product");
        }
    };

    const exportCsv = async () => {
        try {
            const res = await api.get("/export/products", { responseType: "blob" });
            const url = URL.createObjectURL(new Blob([res.data]));
            const a = document.createElement("a");
            a.href = url;
            a.download = `products_${new Date().toISOString().slice(0, 19).replace(/[:-]/g, "")}.csv`;
            a.click();
            URL.revokeObjectURL(url);
            toast("success", "Products exported");
        } catch {
            toast("error", "Export failed");
        }
    };

    const startEdit = (p: Product) => {
        setEditing(p);
        reset({ sku: p.sku, name: p.name, unit: p.unit, category: p.category, description: p.description });
        setShowForm(true);
    };

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">Products</h1>
                <div className="flex items-center gap-2">
                    {isAdmin && (
                        <>
                            <button onClick={() => setShowImport(true)}
                                className="px-4 py-2 bg-amber-500 text-white rounded-lg text-sm hover:bg-amber-600 transition">
                                📥 Import CSV
                            </button>
                            <button onClick={exportCsv}
                                className="px-4 py-2 bg-emerald-600 text-white rounded-lg text-sm hover:bg-emerald-700 transition">
                                📤 Export CSV
                            </button>
                        </>
                    )}
                    {can("products:create") && (
                        <button onClick={() => { setEditing(null); reset({ sku: "", name: "", unit: "", category: "", description: "" }); setShowForm(!showForm); }}
                            className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 transition">
                            {showForm ? "Cancel" : "+ New Product"}
                        </button>
                    )}
                </div>
            </div>

            {/* Import Modal */}
            <CsvImportModal
                open={showImport}
                onClose={() => setShowImport(false)}
                endpoint="/import/products"
                title="Import Products CSV"
                onDone={fetchProducts}
            />

            {/* Form */}
            {showForm && (
                <div className="bg-white rounded-xl shadow-sm border p-5 mb-6">
                    <h2 className="text-lg font-semibold mb-4">{editing ? "Edit Product" : "Create Product"}</h2>
                    <form onSubmit={handleSubmit(onSubmit)} className="grid grid-cols-1 md:grid-cols-3 gap-4">
                        <div>
                            <label className="block text-sm font-medium mb-1">SKU *</label>
                            <input {...register("sku")} className="w-full px-3 py-2 border rounded-lg text-sm" />
                            {errors.sku && <p className="text-red-500 text-xs mt-1">{errors.sku.message}</p>}
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Name *</label>
                            <input {...register("name")} className="w-full px-3 py-2 border rounded-lg text-sm" />
                            {errors.name && <p className="text-red-500 text-xs mt-1">{errors.name.message}</p>}
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Unit *</label>
                            <input {...register("unit")} className="w-full px-3 py-2 border rounded-lg text-sm" placeholder="pcs, kg, box" />
                            {errors.unit && <p className="text-red-500 text-xs mt-1">{errors.unit.message}</p>}
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Category</label>
                            <input {...register("category")} className="w-full px-3 py-2 border rounded-lg text-sm" />
                        </div>
                        <div className="md:col-span-2">
                            <label className="block text-sm font-medium mb-1">Description</label>
                            <input {...register("description")} className="w-full px-3 py-2 border rounded-lg text-sm" />
                        </div>
                        <div className="md:col-span-3">
                            <button type="submit" className="px-6 py-2 bg-emerald-600 text-white rounded-lg text-sm hover:bg-emerald-700 transition">
                                {editing ? "Update" : "Create"}
                            </button>
                        </div>
                    </form>
                </div>
            )}

            {/* Table */}
            {loading ? (
                <div className="flex justify-center py-20">
                    <div className="animate-spin w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full" />
                </div>
            ) : products.length === 0 ? (
                <div className="text-center py-20 text-gray-400">
                    <p className="text-4xl mb-2">📦</p>
                    <p>No products yet</p>
                </div>
            ) : (
                <div className="bg-white rounded-xl shadow-sm border overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50">
                            <tr className="text-left text-gray-500 text-xs uppercase tracking-wider">
                                <th className="px-4 py-3">SKU</th>
                                <th className="px-4 py-3">Name</th>
                                <th className="px-4 py-3">Unit</th>
                                <th className="px-4 py-3">Category</th>
                                <th className="px-4 py-3 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {products.map((p) => (
                                <tr key={p.id} className="border-t hover:bg-gray-50 transition">
                                    <td className="px-4 py-3 font-mono text-xs">{p.sku}</td>
                                    <td className="px-4 py-3 font-medium">{p.name}</td>
                                    <td className="px-4 py-3">{p.unit}</td>
                                    <td className="px-4 py-3 text-gray-500">{p.category || "—"}</td>
                                    <td className="px-4 py-3 text-right space-x-2">
                                        {can("products:update") && (
                                            <button onClick={() => startEdit(p)} className="text-blue-600 hover:underline text-xs">Edit</button>
                                        )}
                                        {can("products:delete") && (
                                            <button onClick={() => onDelete(p.id)} className="text-red-600 hover:underline text-xs">Delete</button>
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
