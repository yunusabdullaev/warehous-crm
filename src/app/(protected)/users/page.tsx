"use client";
import { useEffect, useState, useCallback } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import api from "@/lib/api";
import { getUser, isSuperAdmin, getTenantId } from "@/lib/auth";
import { toast } from "@/components/Toast";
import type { User, Tenant, PaginatedResponse } from "@/lib/types";

interface Warehouse {
    id: string;
    code: string;
    name: string;
}

const schema = z.object({
    username: z.string().min(3, "Min 3 characters"),
    password: z.string().min(6, "Min 6 characters"),
    role: z.enum(["admin", "operator", "loader"]),
});
type FormData = z.infer<typeof schema>;

const editSchema = z.object({
    username: z.string().min(3, "Min 3 characters").optional().or(z.literal("")),
    password: z.string().min(6, "Min 6 characters").optional().or(z.literal("")),
    role: z.enum(["admin", "operator", "loader"]),
});
type EditFormData = z.infer<typeof editSchema>;

const ROLE_BADGE: Record<string, string> = {
    superadmin: "bg-purple-100 text-purple-700",
    admin: "bg-red-100 text-red-700",
    operator: "bg-blue-100 text-blue-700",
    loader: "bg-green-100 text-green-700",
    viewer: "bg-gray-100 text-gray-700",
};

export default function UsersPage() {
    const [users, setUsers] = useState<User[]>([]);
    const [warehouses, setWarehouses] = useState<Warehouse[]>([]);
    const [tenants, setTenants] = useState<Tenant[]>([]);
    const [loading, setLoading] = useState(true);
    const [showForm, setShowForm] = useState(false);
    const [editing, setEditing] = useState<User | null>(null);
    const [editAllowed, setEditAllowed] = useState<string[]>([]);
    const [editDefault, setEditDefault] = useState<string>("");
    const [whError, setWhError] = useState("");
    const superadmin = isSuperAdmin();

    const currentUser = getUser();

    const { register, handleSubmit, reset, formState: { errors } } = useForm<FormData>({
        resolver: zodResolver(schema),
    });

    const {
        register: registerEdit,
        handleSubmit: handleEditSubmit,
        reset: resetEdit,
        watch: watchEdit,
        formState: { errors: editErrors },
    } = useForm<EditFormData>({
        resolver: zodResolver(editSchema),
    });

    const editRole = watchEdit("role");

    const fetchUsers = async () => {
        setLoading(true);
        try {
            const res = await api.get<PaginatedResponse<User>>("/users?limit=100");
            setUsers(res.data.data || []);
        } catch { /* interceptor handles */ }
        setLoading(false);
    };

    const fetchWarehouses = useCallback(async () => {
        try {
            const res = await api.get("/warehouses");
            setWarehouses(res.data?.data ?? res.data ?? []);
        } catch { /* silently fail */ }
    }, []);

    const fetchTenants = useCallback(async () => {
        if (!superadmin) return;
        try {
            const res = await api.get("/tenants?limit=100");
            setTenants(res.data?.data ?? res.data ?? []);
        } catch { /* silently fail */ }
    }, [superadmin]);

    useEffect(() => { fetchUsers(); fetchWarehouses(); fetchTenants(); }, [fetchWarehouses, fetchTenants]);

    const getTenantName = (id?: string) => {
        if (!id) return "—";
        const t = tenants.find(t => t.id === id);
        return t ? t.name : id.slice(-6);
    };

    const onCreateSubmit = async (data: FormData) => {
        try {
            await api.post("/auth/register", data);
            toast("success", "User created");
            reset();
            setShowForm(false);
            fetchUsers();
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Failed to create user");
        }
    };

    const onEditSubmit = async (data: EditFormData) => {
        if (!editing) return;
        setWhError("");

        // Validate warehouse assignment
        if (data.role !== "admin" && editAllowed.length === 0) {
            setWhError("Non-admin users must have at least one allowed warehouse");
            return;
        }
        if (editDefault && editAllowed.length > 0 && !editAllowed.includes(editDefault)) {
            setWhError("Default warehouse must be in the allowed list");
            return;
        }

        try {
            const payload: Record<string, unknown> = {};
            if (data.role) payload.role = data.role;
            if (data.username) payload.username = data.username;
            if (data.password) payload.password = data.password;
            payload.allowed_warehouse_ids = editAllowed;
            if (editDefault) payload.default_warehouse_id = editDefault;

            await api.put(`/users/${editing.id}`, payload);
            toast("success", "User updated");
            setEditing(null);
            resetEdit();
            fetchUsers();
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Failed to update user");
        }
    };

    const onDelete = async (id: string) => {
        if (!confirm("Delete this user? This action cannot be undone.")) return;
        try {
            await api.delete(`/users/${id}`);
            toast("success", "User deleted");
            fetchUsers();
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Failed to delete user");
        }
    };

    const onResetPassword = async (id: string) => {
        try {
            const res = await api.post<{ token: string; expires_in: string }>(`/users/${id}/reset-token`, {});
            const token = res.data.token;
            toast("success", `Reset token generated (expires: ${res.data.expires_in})`);
            // Show token in prompt for admin to share
            prompt("Share this one-time reset token with the user:", token);
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Failed to generate reset token");
        }
    };

    const onRevokeSessions = async (id: string) => {
        if (!confirm("Revoke all sessions for this user? They will be logged out.")) return;
        try {
            const res = await api.post<{ revoked: number }>(`/users/${id}/revoke-sessions`, {});
            toast("success", `Revoked ${res.data.revoked} session(s)`);
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Failed to revoke sessions");
        }
    };

    const startEdit = (u: User) => {
        setEditing(u);
        setEditAllowed(u.allowed_warehouse_ids || []);
        setEditDefault(u.default_warehouse_id || "");
        setWhError("");
        resetEdit({ username: u.username, role: u.role as "admin" | "operator" | "loader", password: "" });
        setShowForm(false);
    };

    const toggleAllowed = (whId: string) => {
        setEditAllowed(prev => {
            if (prev.includes(whId)) {
                const next = prev.filter(id => id !== whId);
                // Clear default if removed from allowed
                if (editDefault === whId) setEditDefault("");
                return next;
            }
            return [...prev, whId];
        });
    };

    const getWarehouseName = (id: string) => {
        const w = warehouses.find(w => w.id === id);
        return w ? `${w.code} — ${w.name}` : id.slice(-6);
    };

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">Users</h1>
                <button
                    onClick={() => { setEditing(null); reset({ username: "", password: "", role: "operator" }); setShowForm(!showForm); }}
                    className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 transition"
                >
                    {showForm ? "Cancel" : "+ New User"}
                </button>
            </div>

            {/* Create Form */}
            {showForm && (
                <div className="bg-white rounded-xl shadow-sm border p-5 mb-6">
                    <h2 className="text-lg font-semibold mb-4">Create User</h2>
                    <form onSubmit={handleSubmit(onCreateSubmit)} className="grid grid-cols-1 md:grid-cols-3 gap-4">
                        <div>
                            <label className="block text-sm font-medium mb-1">Username *</label>
                            <input {...register("username")} className="w-full px-3 py-2 border rounded-lg text-sm" placeholder="min 3 characters" />
                            {errors.username && <p className="text-red-500 text-xs mt-1">{errors.username.message}</p>}
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Password *</label>
                            <input {...register("password")} type="password" className="w-full px-3 py-2 border rounded-lg text-sm" placeholder="min 6 characters" />
                            {errors.password && <p className="text-red-500 text-xs mt-1">{errors.password.message}</p>}
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Role *</label>
                            <select {...register("role")} className="w-full px-3 py-2 border rounded-lg text-sm bg-white">
                                <option value="operator">Operator</option>
                                <option value="loader">Loader</option>
                                <option value="admin">Admin</option>
                            </select>
                            {errors.role && <p className="text-red-500 text-xs mt-1">{errors.role.message}</p>}
                        </div>
                        <div className="md:col-span-3">
                            <button type="submit" className="px-6 py-2 bg-emerald-600 text-white rounded-lg text-sm hover:bg-emerald-700 transition">
                                Create
                            </button>
                        </div>
                    </form>
                </div>
            )}

            {/* Edit Form */}
            {editing && (
                <div className="bg-white rounded-xl shadow-sm border p-5 mb-6 border-l-4 border-l-blue-500">
                    <div className="flex items-center justify-between mb-4">
                        <h2 className="text-lg font-semibold">Edit User — {editing.username}</h2>
                        <button onClick={() => setEditing(null)} className="text-gray-400 hover:text-gray-600 text-sm">✕ Close</button>
                    </div>
                    <form onSubmit={handleEditSubmit(onEditSubmit)} className="space-y-4">
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                            <div>
                                <label className="block text-sm font-medium mb-1">Username</label>
                                <input {...registerEdit("username")} className="w-full px-3 py-2 border rounded-lg text-sm" />
                                {editErrors.username && <p className="text-red-500 text-xs mt-1">{editErrors.username.message}</p>}
                            </div>
                            <div>
                                <label className="block text-sm font-medium mb-1">New Password</label>
                                <input {...registerEdit("password")} type="password" className="w-full px-3 py-2 border rounded-lg text-sm" placeholder="leave empty to keep current" />
                                {editErrors.password && <p className="text-red-500 text-xs mt-1">{editErrors.password.message}</p>}
                            </div>
                            <div>
                                <label className="block text-sm font-medium mb-1">Role</label>
                                <select {...registerEdit("role")} className="w-full px-3 py-2 border rounded-lg text-sm bg-white">
                                    <option value="operator">Operator</option>
                                    <option value="loader">Loader</option>
                                    <option value="admin">Admin</option>
                                </select>
                            </div>
                        </div>

                        {/* Warehouse Assignment */}
                        <div className="border-t pt-4 mt-4">
                            <h3 className="text-sm font-semibold mb-3">Warehouse Assignment</h3>
                            {editRole === "admin" && (
                                <p className="text-xs text-gray-500 mb-2 italic">Admins have access to all warehouses. Assignment below is optional.</p>
                            )}

                            {whError && (
                                <div className="bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg px-3 py-2 mb-3">
                                    {whError}
                                </div>
                            )}

                            {/* Multi-select: Allowed Warehouses */}
                            <label className="block text-xs font-medium text-gray-600 mb-1.5">Allowed Warehouses</label>
                            <div className="flex flex-wrap gap-2 mb-4">
                                {warehouses.map((w) => (
                                    <button
                                        key={w.id}
                                        type="button"
                                        onClick={() => toggleAllowed(w.id)}
                                        className={`px-3 py-1.5 rounded-full text-xs font-medium border transition ${editAllowed.includes(w.id)
                                            ? "bg-blue-50 border-blue-300 text-blue-700"
                                            : "bg-gray-50 border-gray-200 text-gray-500 hover:border-gray-300"
                                            }`}
                                    >
                                        {editAllowed.includes(w.id) ? "✓ " : ""}{w.code} — {w.name}
                                    </button>
                                ))}
                                {warehouses.length === 0 && (
                                    <span className="text-xs text-gray-400">No warehouses available</span>
                                )}
                            </div>

                            {/* Single-select: Default Warehouse (filtered to allowed list) */}
                            <label className="block text-xs font-medium text-gray-600 mb-1.5">Default Warehouse</label>
                            <select
                                value={editDefault}
                                onChange={(e) => setEditDefault(e.target.value)}
                                className="w-full max-w-xs px-3 py-2 border rounded-lg text-sm bg-white"
                            >
                                <option value="">— None —</option>
                                {editAllowed.map((id) => (
                                    <option key={id} value={id}>
                                        {getWarehouseName(id)}
                                    </option>
                                ))}
                            </select>
                        </div>

                        <div className="pt-2">
                            <button type="submit" className="px-6 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 transition">
                                Update
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
            ) : users.length === 0 ? (
                <div className="text-center py-20 text-gray-400">
                    <p className="text-4xl mb-2">👥</p>
                    <p>No users yet</p>
                </div>
            ) : (
                <div className="bg-white rounded-xl shadow-sm border overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-gray-50">
                            <tr className="text-left text-gray-500 text-xs uppercase tracking-wider">
                                <th className="px-4 py-3">Username</th>
                                <th className="px-4 py-3">Role</th>
                                {superadmin && <th className="px-4 py-3">Tenant</th>}
                                <th className="px-4 py-3">Warehouses</th>
                                <th className="px-4 py-3">Default</th>
                                <th className="px-4 py-3">Created</th>
                                <th className="px-4 py-3 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {users.map((u) => (
                                <tr key={u.id} className="border-t hover:bg-gray-50 transition">
                                    <td className="px-4 py-3 font-medium">
                                        {u.username}
                                        {u.id === currentUser?.id && (
                                            <span className="ml-2 text-xs text-gray-400">(you)</span>
                                        )}
                                    </td>
                                    <td className="px-4 py-3">
                                        <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${ROLE_BADGE[u.role] || ROLE_BADGE.viewer}`}>
                                            {u.role}
                                        </span>
                                    </td>
                                    {superadmin && (
                                        <td className="px-4 py-3">
                                            <span className="text-xs bg-purple-50 text-purple-700 px-1.5 py-0.5 rounded">
                                                {getTenantName(u.tenant_id)}
                                            </span>
                                        </td>
                                    )}
                                    <td className="px-4 py-3">
                                        {u.role === "admin" && (!u.allowed_warehouse_ids || u.allowed_warehouse_ids.length === 0) ? (
                                            <span className="text-xs text-gray-400 italic">All</span>
                                        ) : (u.allowed_warehouse_ids || []).length > 0 ? (
                                            <div className="flex flex-wrap gap-1">
                                                {(u.allowed_warehouse_ids || []).map(id => (
                                                    <span key={id} className="bg-blue-50 text-blue-700 text-xs px-1.5 py-0.5 rounded">
                                                        {getWarehouseName(id)}
                                                    </span>
                                                ))}
                                            </div>
                                        ) : (
                                            <span className="text-xs text-gray-400">—</span>
                                        )}
                                    </td>
                                    <td className="px-4 py-3 text-xs text-gray-500">
                                        {u.default_warehouse_id ? getWarehouseName(u.default_warehouse_id) : "—"}
                                    </td>
                                    <td className="px-4 py-3 text-gray-500">
                                        {new Date(u.created_at).toLocaleDateString()}
                                    </td>
                                    <td className="px-4 py-3 text-right space-x-2">
                                        <button onClick={() => startEdit(u)} className="text-blue-600 hover:underline text-xs">
                                            Edit
                                        </button>
                                        {u.id !== currentUser?.id && (
                                            <>
                                                <button onClick={() => onResetPassword(u.id)} className="text-amber-600 hover:underline text-xs">
                                                    Reset PW
                                                </button>
                                                <button onClick={() => onRevokeSessions(u.id)} className="text-purple-600 hover:underline text-xs">
                                                    Revoke
                                                </button>
                                                <button onClick={() => onDelete(u.id)} className="text-red-600 hover:underline text-xs">
                                                    Delete
                                                </button>
                                            </>
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
