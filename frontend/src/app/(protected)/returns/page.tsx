"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import api from "@/lib/api";
import { toast } from "@/components/Toast";

interface ReturnRecord {
    id: string;
    rma_no: string;
    order_id: string;
    order_no: string;
    client_name: string;
    status: string;
    notes?: string;
    created_at: string;
    received_at?: string;
}

interface ShippedOrder {
    id: string;
    order_no: string;
    client_name: string;
}

const statusColors: Record<string, string> = {
    OPEN: "var(--color-warning)",
    RECEIVED: "var(--color-success)",
    CLOSED: "var(--color-info)",
    CANCELLED: "var(--color-danger)",
};

export default function ReturnsPage() {
    const [returns, setReturns] = useState<ReturnRecord[]>([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(1);
    const [status, setStatus] = useState("");
    const [loading, setLoading] = useState(true);

    // Create modal
    const [showCreate, setShowCreate] = useState(false);
    const [shippedOrders, setShippedOrders] = useState<ShippedOrder[]>([]);
    const [selectedOrder, setSelectedOrder] = useState("");
    const [notes, setNotes] = useState("");
    const [creating, setCreating] = useState(false);

    const limit = 20;

    const fetchReturns = async () => {
        setLoading(true);
        try {
            const params = new URLSearchParams({ page: String(page), limit: String(limit) });
            if (status) params.set("status", status);
            const { data } = await api.get(`/returns?${params}`);
            setReturns(data.data || []);
            setTotal(data.total || 0);
        } catch {
            toast("error", "Failed to load returns");
        } finally {
            setLoading(false);
        }
    };

    const fetchShippedOrders = async () => {
        try {
            const { data } = await api.get("/orders?status=SHIPPED&limit=100");
            setShippedOrders(
                (data.data || []).map((o: { id: string; order_no: string; client_name: string }) => ({
                    id: o.id,
                    order_no: o.order_no,
                    client_name: o.client_name,
                }))
            );
        } catch {
            toast("error", "Failed to load shipped orders");
        }
    };

    useEffect(() => { fetchReturns(); }, [page, status]);

    const handleCreate = async () => {
        if (!selectedOrder) return toast("error", "Select an order");
        setCreating(true);
        try {
            await api.post("/returns", { order_id: selectedOrder, notes });
            toast("success", "Return created");
            setShowCreate(false);
            setSelectedOrder("");
            setNotes("");
            fetchReturns();
        } catch (err: unknown) {
            const msg = err instanceof Error ? err.message : "Failed";
            toast("error", msg);
        } finally {
            setCreating(false);
        }
    };

    const totalPages = Math.ceil(total / limit);

    return (
        <div className="page-container">
            <div className="page-header">
                <h1>Returns (RMA)</h1>
                <button
                    className="btn btn-primary"
                    onClick={() => { setShowCreate(true); fetchShippedOrders(); }}
                >
                    + Create Return
                </button>
            </div>

            {/* Filters */}
            <div className="filters-bar" style={{ marginBottom: 16, display: "flex", gap: 12 }}>
                <select value={status} onChange={(e) => { setStatus(e.target.value); setPage(1); }} className="input" style={{ width: 180 }}>
                    <option value="">All Statuses</option>
                    <option value="OPEN">Open</option>
                    <option value="RECEIVED">Received</option>
                    <option value="CLOSED">Closed</option>
                    <option value="CANCELLED">Cancelled</option>
                </select>
            </div>

            {/* Table */}
            <div className="table-container">
                <table>
                    <thead>
                        <tr>
                            <th>RMA No</th>
                            <th>Order</th>
                            <th>Client</th>
                            <th>Status</th>
                            <th>Created</th>
                            <th>Received</th>
                            <th>Notes</th>
                        </tr>
                    </thead>
                    <tbody>
                        {loading ? (
                            <tr><td colSpan={7} style={{ textAlign: "center", padding: 32 }}>Loading…</td></tr>
                        ) : returns.length === 0 ? (
                            <tr><td colSpan={7} style={{ textAlign: "center", padding: 32 }}>No returns found</td></tr>
                        ) : (
                            returns.map((r) => (
                                <tr key={r.id}>
                                    <td>
                                        <Link href={`/returns/${r.id}`} style={{ color: "var(--color-primary)", fontWeight: 600 }}>
                                            {r.rma_no}
                                        </Link>
                                    </td>
                                    <td>{r.order_no}</td>
                                    <td>{r.client_name}</td>
                                    <td>
                                        <span className="badge" style={{ background: statusColors[r.status] || "#888", color: "#fff", padding: "2px 10px", borderRadius: 12, fontSize: 12 }}>
                                            {r.status}
                                        </span>
                                    </td>
                                    <td>{new Date(r.created_at).toLocaleDateString()}</td>
                                    <td>{r.received_at ? new Date(r.received_at).toLocaleDateString() : "—"}</td>
                                    <td style={{ maxWidth: 200, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{r.notes || "—"}</td>
                                </tr>
                            ))
                        )}
                    </tbody>
                </table>
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
                <div className="pagination" style={{ display: "flex", gap: 8, marginTop: 16, justifyContent: "center" }}>
                    <button className="btn btn-secondary" disabled={page <= 1} onClick={() => setPage(page - 1)}>← Prev</button>
                    <span style={{ padding: "8px 12px" }}>Page {page} / {totalPages}</span>
                    <button className="btn btn-secondary" disabled={page >= totalPages} onClick={() => setPage(page + 1)}>Next →</button>
                </div>
            )}

            {/* Create Modal */}
            {showCreate && (
                <div className="modal-overlay" onClick={() => setShowCreate(false)}>
                    <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 500 }}>
                        <h2>Create Return (RMA)</h2>
                        <div className="form-group">
                            <label>Shipped Order *</label>
                            <select className="input" value={selectedOrder} onChange={(e) => setSelectedOrder(e.target.value)}>
                                <option value="">Select order…</option>
                                {shippedOrders.map((o) => (
                                    <option key={o.id} value={o.id}>{o.order_no} — {o.client_name}</option>
                                ))}
                            </select>
                        </div>
                        <div className="form-group">
                            <label>Notes</label>
                            <textarea className="input" rows={3} value={notes} onChange={(e) => setNotes(e.target.value)} placeholder="Optional return notes…" />
                        </div>
                        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end", marginTop: 16 }}>
                            <button className="btn btn-secondary" onClick={() => setShowCreate(false)}>Cancel</button>
                            <button className="btn btn-primary" onClick={handleCreate} disabled={creating}>{creating ? "Creating…" : "Create RMA"}</button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
