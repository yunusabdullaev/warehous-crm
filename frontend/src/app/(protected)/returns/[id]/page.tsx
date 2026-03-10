"use client";
import { useEffect, useState, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import api from "@/lib/api";
import { toast } from "@/components/Toast";
import { can } from "@/lib/auth";
import type { Product, PaginatedResponse, Location } from "@/lib/types";

interface ReturnRecord {
    id: string;
    rma_no: string;
    order_id: string;
    order_no: string;
    client_name: string;
    status: string;
    notes?: string;
    created_at: string;
    created_by: string;
    received_at?: string;
    received_by?: string;
}

interface ReturnItem {
    id: string;
    return_id: string;
    product_id: string;
    location_id?: string;
    qty: number;
    disposition: string;
    note?: string;
    created_at: string;
    created_by: string;
}

const dispositionColors: Record<string, string> = {
    RESTOCK: "#22c55e",
    DAMAGED: "#ef4444",
    QC_HOLD: "#f59e0b",
};

export default function ReturnDetailPage() {
    const { id } = useParams<{ id: string }>();
    const router = useRouter();
    const [ret, setRet] = useState<ReturnRecord | null>(null);
    const [items, setItems] = useState<ReturnItem[]>([]);
    const [products, setProducts] = useState<Record<string, Product>>({});
    const [locations, setLocations] = useState<Location[]>([]);
    const [loading, setLoading] = useState(true);

    // Add item modal
    const [showAdd, setShowAdd] = useState(false);
    const [addForm, setAddForm] = useState({ product_id: "", qty: 1, disposition: "RESTOCK", location_id: "", note: "" });
    const [submitting, setSubmitting] = useState(false);

    const canManage = can("returns:manage");

    const fetchReturn = useCallback(async () => {
        try {
            const { data } = await api.get(`/returns/${id}`);
            setRet(data.return);
            setItems(data.items || []);

            // Fetch product names
            const prodIds = [...new Set((data.items || []).map((i: ReturnItem) => i.product_id))];
            const prodMap: Record<string, Product> = {};
            for (const pid of prodIds) {
                try {
                    const { data: p } = await api.get(`/products/${pid}`);
                    prodMap[pid as string] = p;
                } catch { /* skip */ }
            }
            setProducts(prodMap);
        } catch {
            toast("error", "Failed to load return");
        } finally {
            setLoading(false);
        }
    }, [id]);

    const fetchLocations = async () => {
        try {
            const { data } = await api.get<PaginatedResponse<Location>>("/locations?limit=200");
            setLocations(data.data || []);
        } catch { /* skip */ }
    };

    useEffect(() => { fetchReturn(); fetchLocations(); }, [fetchReturn]);

    const handleAddItem = async () => {
        if (!addForm.product_id || addForm.qty < 1) return toast("error", "Fill required fields");
        if (addForm.disposition === "RESTOCK" && !addForm.location_id) return toast("error", "Location required for RESTOCK");

        setSubmitting(true);
        try {
            await api.post(`/returns/${id}/items`, {
                product_id: addForm.product_id,
                qty: addForm.qty,
                disposition: addForm.disposition,
                location_id: addForm.disposition === "RESTOCK" ? addForm.location_id : undefined,
                note: addForm.note || undefined,
            });
            toast("success", "Item added");
            setShowAdd(false);
            setAddForm({ product_id: "", qty: 1, disposition: "RESTOCK", location_id: "", note: "" });
            fetchReturn();
        } catch (err: unknown) {
            const msg = err instanceof Error ? err.message : "Failed";
            toast("error", msg);
        } finally {
            setSubmitting(false);
        }
    };

    const handleReceive = async () => {
        if (!confirm("Mark this return as RECEIVED?")) return;
        try {
            await api.post(`/returns/${id}/receive`);
            toast("success", "Return received");
            fetchReturn();
        } catch (err: unknown) {
            const msg = err instanceof Error ? err.message : "Failed";
            toast("error", msg);
        }
    };

    const handleCancel = async () => {
        if (!confirm("Cancel this return? This cannot be undone.")) return;
        try {
            await api.post(`/returns/${id}/cancel`);
            toast("success", "Return cancelled");
            fetchReturn();
        } catch (err: unknown) {
            const msg = err instanceof Error ? err.message : "Failed";
            toast("error", msg);
        }
    };

    if (loading) return <div className="page-container"><p>Loading…</p></div>;
    if (!ret) return <div className="page-container"><p>Return not found</p></div>;

    // Collect products from order items for the add item dropdown
    const allProductsList = Object.values(products);

    return (
        <div className="page-container">
            <div className="page-header">
                <div>
                    <button className="btn btn-secondary" onClick={() => router.push("/returns")} style={{ marginBottom: 8 }}>← Back</button>
                    <h1>{ret.rma_no}</h1>
                </div>
                <div style={{ display: "flex", gap: 8 }}>
                    {ret.status === "OPEN" && (
                        <>
                            <button className="btn btn-primary" onClick={() => setShowAdd(true)}>+ Add Item</button>
                            {canManage && <button className="btn btn-success" onClick={handleReceive}>✓ Receive</button>}
                            {canManage && <button className="btn btn-danger" onClick={handleCancel}>✗ Cancel</button>}
                        </>
                    )}
                    <a href={`${api.defaults.baseURL}/returns/${id}/note.pdf`} target="_blank" rel="noreferrer" className="btn btn-secondary">📄 PDF</a>
                </div>
            </div>

            {/* Info cards */}
            <div className="stats-grid" style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(200px, 1fr))", gap: 16, marginBottom: 24 }}>
                <div className="stat-card">
                    <span className="stat-label">Order</span>
                    <span className="stat-value">{ret.order_no}</span>
                </div>
                <div className="stat-card">
                    <span className="stat-label">Client</span>
                    <span className="stat-value">{ret.client_name}</span>
                </div>
                <div className="stat-card">
                    <span className="stat-label">Status</span>
                    <span className="stat-value">{ret.status}</span>
                </div>
                <div className="stat-card">
                    <span className="stat-label">Created</span>
                    <span className="stat-value">{new Date(ret.created_at).toLocaleString()}</span>
                </div>
                {ret.received_at && (
                    <div className="stat-card">
                        <span className="stat-label">Received</span>
                        <span className="stat-value">{new Date(ret.received_at).toLocaleString()}</span>
                    </div>
                )}
                {ret.notes && (
                    <div className="stat-card" style={{ gridColumn: "span 2" }}>
                        <span className="stat-label">Notes</span>
                        <span className="stat-value">{ret.notes}</span>
                    </div>
                )}
            </div>

            {/* Items table */}
            <h2 style={{ marginBottom: 12 }}>Return Items ({items.length})</h2>
            <div className="table-container">
                <table>
                    <thead>
                        <tr>
                            <th>#</th>
                            <th>Product</th>
                            <th>Qty</th>
                            <th>Disposition</th>
                            <th>Location</th>
                            <th>Note</th>
                            <th>Added</th>
                        </tr>
                    </thead>
                    <tbody>
                        {items.length === 0 ? (
                            <tr><td colSpan={7} style={{ textAlign: "center", padding: 24 }}>No items yet</td></tr>
                        ) : (
                            items.map((item, i) => (
                                <tr key={item.id}>
                                    <td>{i + 1}</td>
                                    <td>{products[item.product_id]?.name || item.product_id.slice(0, 8)}</td>
                                    <td><strong>{item.qty}</strong></td>
                                    <td>
                                        <span style={{ background: dispositionColors[item.disposition] || "#888", color: "#fff", padding: "2px 10px", borderRadius: 12, fontSize: 12, fontWeight: 600 }}>
                                            {item.disposition}
                                        </span>
                                    </td>
                                    <td>{item.location_id || "—"}</td>
                                    <td style={{ maxWidth: 200, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{item.note || "—"}</td>
                                    <td>{new Date(item.created_at).toLocaleString()}</td>
                                </tr>
                            ))
                        )}
                    </tbody>
                </table>
            </div>

            {/* Add Item Modal */}
            {showAdd && (
                <div className="modal-overlay" onClick={() => setShowAdd(false)}>
                    <div className="modal" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 500 }}>
                        <h2>Add Return Item</h2>

                        <div className="form-group">
                            <label>Product *</label>
                            <select className="input" value={addForm.product_id} onChange={(e) => setAddForm({ ...addForm, product_id: e.target.value })}>
                                <option value="">Select…</option>
                                {allProductsList.map((p) => (
                                    <option key={p.id} value={p.id}>{p.sku} — {p.name}</option>
                                ))}
                            </select>
                            <small style={{ color: "var(--color-muted)" }}>
                                If the product isn&apos;t listed, enter its ID manually below.
                            </small>
                            <input className="input" style={{ marginTop: 4 }} placeholder="Or paste Product ID" value={addForm.product_id} onChange={(e) => setAddForm({ ...addForm, product_id: e.target.value })} />
                        </div>

                        <div className="form-group">
                            <label>Qty *</label>
                            <input type="number" className="input" min={1} value={addForm.qty} onChange={(e) => setAddForm({ ...addForm, qty: parseInt(e.target.value) || 1 })} />
                        </div>

                        <div className="form-group">
                            <label>Disposition *</label>
                            <select className="input" value={addForm.disposition} onChange={(e) => setAddForm({ ...addForm, disposition: e.target.value })}>
                                <option value="RESTOCK">RESTOCK — Add back to inventory</option>
                                <option value="DAMAGED">DAMAGED — Log as damaged</option>
                                <option value="QC_HOLD">QC_HOLD — Hold for inspection</option>
                            </select>
                        </div>

                        {addForm.disposition === "RESTOCK" && (
                            <div className="form-group">
                                <label>Location *</label>
                                <select className="input" value={addForm.location_id} onChange={(e) => setAddForm({ ...addForm, location_id: e.target.value })}>
                                    <option value="">Select location…</option>
                                    {locations.map((l) => (
                                        <option key={l.id} value={l.id}>{l.code} — {l.name} ({l.zone})</option>
                                    ))}
                                </select>
                            </div>
                        )}

                        <div className="form-group">
                            <label>Note</label>
                            <input className="input" value={addForm.note} onChange={(e) => setAddForm({ ...addForm, note: e.target.value })} placeholder="Optional note…" />
                        </div>

                        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end", marginTop: 16 }}>
                            <button className="btn btn-secondary" onClick={() => setShowAdd(false)}>Cancel</button>
                            <button className="btn btn-primary" onClick={handleAddItem} disabled={submitting}>{submitting ? "Adding…" : "Add Item"}</button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
