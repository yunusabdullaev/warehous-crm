"use client";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import api from "@/lib/api";
import { can, getUser } from "@/lib/auth";
import { toast } from "@/components/Toast";
import CsvImportModal from "@/components/CsvImportModal";
import LabelPreviewModal from "@/components/LabelPreviewModal";
import type { Location, PaginatedResponse } from "@/lib/types";

const schema = z.object({
    code: z.string().min(1, "Code is required"),
    name: z.string().min(1, "Name is required"),
    zone: z.string().optional(),
    aisle: z.string().optional(),
    rack: z.string().optional(),
    level: z.string().optional(),
});
type FormData = z.infer<typeof schema>;

export default function LocationsPage() {
    const [locations, setLocations] = useState<Location[]>([]);
    const [loading, setLoading] = useState(true);
    const [showForm, setShowForm] = useState(false);
    const [showImport, setShowImport] = useState(false);
    const [labelLocation, setLabelLocation] = useState<Location | null>(null);

    const auth = getUser();
    const isAdmin = auth?.role === "admin";

    const { register, handleSubmit, reset, formState: { errors } } = useForm<FormData>({
        resolver: zodResolver(schema),
    });

    const fetchLocations = async () => {
        setLoading(true);
        try {
            const res = await api.get<PaginatedResponse<Location>>("/locations?limit=100");
            setLocations(res.data.data || []);
        } catch { /* interceptor */ }
        setLoading(false);
    };

    useEffect(() => { fetchLocations(); }, []);

    const onSubmit = async (data: FormData) => {
        try {
            await api.post("/locations", data);
            toast("success", "Location created");
            reset();
            setShowForm(false);
            fetchLocations();
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Failed to create location");
        }
    };

    const onDelete = async (id: string) => {
        if (!confirm("Delete this location?")) return;
        try {
            await api.delete(`/locations/${id}`);
            toast("success", "Location deleted");
            fetchLocations();
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Failed to delete location");
        }
    };

    const exportCsv = async () => {
        try {
            const res = await api.get("/export/locations", { responseType: "blob" });
            const url = URL.createObjectURL(new Blob([res.data]));
            const a = document.createElement("a");
            a.href = url;
            a.download = `locations_${new Date().toISOString().slice(0, 19).replace(/[:-]/g, "")}.csv`;
            a.click();
            URL.revokeObjectURL(url);
            toast("success", "Locations exported");
        } catch {
            toast("error", "Export failed");
        }
    };

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">Locations</h1>
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
                    {can("locations:create") && (
                        <button onClick={() => { reset({ code: "", name: "", zone: "", rack: "", level: "" }); setShowForm(!showForm); }}
                            className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 transition">
                            {showForm ? "Cancel" : "+ New Location"}
                        </button>
                    )}
                </div>
            </div>

            {/* Import Modal */}
            <CsvImportModal
                open={showImport}
                onClose={() => setShowImport(false)}
                endpoint="/import/locations"
                title="Import Locations CSV"
                onDone={fetchLocations}
            />

            {showForm && (
                <div className="bg-white rounded-xl shadow-sm border p-5 mb-6">
                    <h2 className="text-lg font-semibold mb-4">Create Location</h2>
                    <form onSubmit={handleSubmit(onSubmit)} className="grid grid-cols-2 md:grid-cols-3 gap-4">
                        <div>
                            <label className="block text-sm font-medium mb-1">Code *</label>
                            <input {...register("code")} className="w-full px-3 py-2 border rounded-lg text-sm" />
                            {errors.code && <p className="text-red-500 text-xs mt-1">{errors.code.message}</p>}
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Name *</label>
                            <input {...register("name")} className="w-full px-3 py-2 border rounded-lg text-sm" />
                            {errors.name && <p className="text-red-500 text-xs mt-1">{errors.name.message}</p>}
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Zone</label>
                            <input {...register("zone")} className="w-full px-3 py-2 border rounded-lg text-sm" placeholder="A, B, C" />
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Aisle</label>
                            <input {...register("aisle")} className="w-full px-3 py-2 border rounded-lg text-sm" />
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Rack</label>
                            <input {...register("rack")} className="w-full px-3 py-2 border rounded-lg text-sm" />
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Level</label>
                            <input {...register("level")} className="w-full px-3 py-2 border rounded-lg text-sm" />
                        </div>
                        <div className="col-span-full">
                            <button type="submit" className="px-6 py-2 bg-emerald-600 text-white rounded-lg text-sm hover:bg-emerald-700 transition">
                                Create
                            </button>
                        </div>
                    </form>
                </div>
            )}

            {loading ? (
                <div className="flex justify-center py-20">
                    <div className="animate-spin w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full" />
                </div>
            ) : locations.length === 0 ? (
                <div className="text-center py-20 text-gray-400">
                    <p className="text-4xl mb-2">📍</p>
                    <p>No locations yet</p>
                </div>
            ) : (
                <div className="bg-white rounded-xl shadow-sm border overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50">
                            <tr className="text-left text-gray-500 text-xs uppercase tracking-wider">
                                <th className="px-4 py-3">Code</th>
                                <th className="px-4 py-3">Name</th>
                                <th className="px-4 py-3">Zone</th>
                                <th className="px-4 py-3">Rack</th>
                                <th className="px-4 py-3">Level</th>
                                <th className="px-4 py-3 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {locations.map((l) => (
                                <tr key={l.id} className="border-t hover:bg-gray-50 transition">
                                    <td className="px-4 py-3 font-mono text-xs">{l.code}</td>
                                    <td className="px-4 py-3 font-medium">{l.name}</td>
                                    <td className="px-4 py-3">{l.zone || "—"}</td>
                                    <td className="px-4 py-3">{l.rack || "—"}</td>
                                    <td className="px-4 py-3">{l.level || "—"}</td>
                                    <td className="px-4 py-3 text-right space-x-2">
                                        {isAdmin && (
                                            <button onClick={() => setLabelLocation(l)} className="text-indigo-600 hover:underline text-xs">🏷️ Label</button>
                                        )}
                                        {can("locations:delete") && (
                                            <button onClick={() => onDelete(l.id)} className="text-red-600 hover:underline text-xs">Delete</button>
                                        )}
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}

            {/* Label Preview Modal */}
            {labelLocation && (
                <LabelPreviewModal
                    open={true}
                    onClose={() => setLabelLocation(null)}
                    location={labelLocation}
                />
            )}
        </div>
    );
}
