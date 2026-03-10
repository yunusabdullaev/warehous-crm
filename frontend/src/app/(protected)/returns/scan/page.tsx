"use client";
import { useEffect, useState } from "react";
import dynamic from "next/dynamic";
import api from "@/lib/api";
import { toast } from "@/components/Toast";
import type { Product, PaginatedResponse, Location } from "@/lib/types";

const QrScanner = dynamic(() => import("@/components/QrScanner"), { ssr: false });

interface QrPayload {
    type: string;
    orderId?: string;
    orderNo?: string;
    locationId?: string;
    zone?: string;
    rack?: string;
    shelf?: string;
}

interface ReturnRecord {
    id: string;
    rma_no: string;
    order_no: string;
    client_name: string;
    status: string;
}

type Step = "scan_order" | "select_rma" | "scan_location" | "add_item";

export default function ReturnScanPage() {
    const [step, setStep] = useState<Step>("scan_order");
    const [scanning, setScanning] = useState(false);

    // Order
    const [orderIdInput, setOrderIdInput] = useState("");
    const [orderId, setOrderId] = useState("");
    const [orderNo, setOrderNo] = useState("");

    // RMA
    const [existingRMAs, setExistingRMAs] = useState<ReturnRecord[]>([]);
    const [selectedRMA, setSelectedRMA] = useState("");
    const [creatingRMA, setCreatingRMA] = useState(false);

    // Location
    const [locationId, setLocationId] = useState("");
    const [locationLabel, setLocationLabel] = useState("");
    const [locations, setLocations] = useState<Location[]>([]);

    // Product
    const [products, setProducts] = useState<Product[]>([]);
    const [productId, setProductId] = useState("");
    const [qty, setQty] = useState(1);
    const [disposition, setDisposition] = useState("RESTOCK");
    const [note, setNote] = useState("");
    const [submitting, setSubmitting] = useState(false);

    useEffect(() => {
        api.get<PaginatedResponse<Location>>("/locations?limit=200").then(({ data }) => setLocations(data.data || [])).catch(() => { });
        api.get<PaginatedResponse<Product>>("/products?limit=200").then(({ data }) => setProducts(data.data || [])).catch(() => { });
    }, []);

    const handleScan = (data: string) => {
        try {
            const payload: QrPayload = JSON.parse(data);
            if (step === "scan_order" && payload.type === "order" && payload.orderId) {
                setOrderId(payload.orderId);
                setOrderNo(payload.orderNo || "");
                setScanning(false);
                loadRMAs(payload.orderId);
            } else if (step === "scan_location" && payload.type === "location" && payload.locationId) {
                setLocationId(payload.locationId);
                setLocationLabel(`${payload.zone || ""}-${payload.rack || ""}-${payload.shelf || ""}`);
                setScanning(false);
                setStep("add_item");
            }
        } catch {
            toast("error", "Invalid QR code");
        }
    };

    const handleOrderManual = async () => {
        if (!orderIdInput.trim()) return;
        setOrderId(orderIdInput.trim());
        loadRMAs(orderIdInput.trim());
    };

    const loadRMAs = async (oid: string) => {
        try {
            const { data } = await api.get(`/returns?orderId=${oid}&limit=50`);
            setExistingRMAs((data.data || []).filter((r: ReturnRecord) => r.status === "OPEN"));
            setStep("select_rma");
        } catch {
            toast("error", "Failed to load RMAs");
        }
    };

    const createNewRMA = async () => {
        setCreatingRMA(true);
        try {
            const { data } = await api.post("/returns", { order_id: orderId });
            setSelectedRMA(data.id);
            toast("success", `Created ${data.rma_no}`);
            setStep(disposition === "RESTOCK" ? "scan_location" : "add_item");
        } catch (err: unknown) {
            toast("error", err instanceof Error ? err.message : "Failed");
        } finally {
            setCreatingRMA(false);
        }
    };

    const selectExistingRMA = (rmaId: string) => {
        setSelectedRMA(rmaId);
        setStep(disposition === "RESTOCK" ? "scan_location" : "add_item");
    };

    const handleSubmitItem = async () => {
        if (!productId || qty < 1) return toast("error", "Select product and qty");
        if (disposition === "RESTOCK" && !locationId) return toast("error", "Scan location first");

        setSubmitting(true);
        try {
            await api.post(`/returns/${selectedRMA}/items`, {
                product_id: productId,
                qty,
                disposition,
                location_id: disposition === "RESTOCK" ? locationId : undefined,
                note: note || undefined,
            });
            toast("success", "Item added");
            // Reset for next item
            setProductId("");
            setQty(1);
            setNote("");
        } catch (err: unknown) {
            toast("error", err instanceof Error ? err.message : "Failed");
        } finally {
            setSubmitting(false);
        }
    };

    const resetAll = () => {
        setStep("scan_order");
        setOrderId("");
        setOrderIdInput("");
        setOrderNo("");
        setSelectedRMA("");
        setLocationId("");
        setLocationLabel("");
        setProductId("");
        setQty(1);
        setDisposition("RESTOCK");
        setNote("");
    };

    return (
        <div className="page-container" style={{ maxWidth: 600, margin: "0 auto" }}>
            <div className="page-header">
                <h1>📦 Return Scan</h1>
                <button className="btn btn-secondary" onClick={resetAll}>Reset</button>
            </div>

            {/* Progress */}
            <div style={{ display: "flex", gap: 8, marginBottom: 24 }}>
                {(["scan_order", "select_rma", "scan_location", "add_item"] as Step[]).map((s, i) => (
                    <div key={s} style={{
                        flex: 1, padding: "8px 4px", textAlign: "center", borderRadius: 8, fontSize: 12, fontWeight: 600,
                        background: step === s ? "var(--color-primary)" : i < ["scan_order", "select_rma", "scan_location", "add_item"].indexOf(step) ? "var(--color-success)" : "var(--color-surface)",
                        color: step === s || i < ["scan_order", "select_rma", "scan_location", "add_item"].indexOf(step) ? "#fff" : "var(--color-muted)",
                    }}>
                        {["1. Order", "2. RMA", "3. Location", "4. Item"][i]}
                    </div>
                ))}
            </div>

            {/* Step 1: Scan Order */}
            {step === "scan_order" && (
                <div className="card" style={{ padding: 24 }}>
                    <h2>Scan Order QR</h2>
                    {scanning ? (
                        <QrScanner onScan={handleScan} />
                    ) : (
                        <button className="btn btn-primary" onClick={() => setScanning(true)} style={{ width: "100%", marginBottom: 16 }}>
                            📷 Open Scanner
                        </button>
                    )}
                    <div style={{ marginTop: 16 }}>
                        <label>Or enter Order ID manually:</label>
                        <div style={{ display: "flex", gap: 8, marginTop: 8 }}>
                            <input className="input" value={orderIdInput} onChange={(e) => setOrderIdInput(e.target.value)} placeholder="Order ID" style={{ flex: 1 }} />
                            <button className="btn btn-primary" onClick={handleOrderManual}>Go</button>
                        </div>
                    </div>
                </div>
            )}

            {/* Step 2: Select/Create RMA */}
            {step === "select_rma" && (
                <div className="card" style={{ padding: 24 }}>
                    <h2>Select or Create RMA</h2>
                    <p style={{ color: "var(--color-muted)", marginBottom: 16 }}>Order: <strong>{orderNo || orderId}</strong></p>

                    {existingRMAs.length > 0 && (
                        <div style={{ marginBottom: 16 }}>
                            <h3>Open RMAs:</h3>
                            {existingRMAs.map((r) => (
                                <button key={r.id} className="btn btn-secondary" onClick={() => selectExistingRMA(r.id)} style={{ display: "block", width: "100%", marginBottom: 8, textAlign: "left" }}>
                                    {r.rma_no} — {r.client_name}
                                </button>
                            ))}
                        </div>
                    )}

                    <button className="btn btn-primary" onClick={createNewRMA} disabled={creatingRMA} style={{ width: "100%" }}>
                        {creatingRMA ? "Creating…" : "+ Create New RMA"}
                    </button>

                    <div style={{ marginTop: 16 }}>
                        <label>Disposition for items:</label>
                        <select className="input" value={disposition} onChange={(e) => setDisposition(e.target.value)} style={{ marginTop: 4 }}>
                            <option value="RESTOCK">RESTOCK</option>
                            <option value="DAMAGED">DAMAGED</option>
                            <option value="QC_HOLD">QC_HOLD</option>
                        </select>
                    </div>
                </div>
            )}

            {/* Step 3: Scan Location (for RESTOCK) */}
            {step === "scan_location" && (
                <div className="card" style={{ padding: 24 }}>
                    <h2>Scan Location QR</h2>
                    {scanning ? (
                        <QrScanner onScan={handleScan} />
                    ) : (
                        <button className="btn btn-primary" onClick={() => setScanning(true)} style={{ width: "100%", marginBottom: 16 }}>
                            📷 Open Scanner
                        </button>
                    )}
                    <div style={{ marginTop: 16 }}>
                        <label>Or select location:</label>
                        <select className="input" value={locationId} onChange={(e) => {
                            const loc = locations.find(l => l.id === e.target.value);
                            setLocationId(e.target.value);
                            setLocationLabel(loc ? `${loc.zone}-${loc.rack}-${loc.level}` : "");
                            if (e.target.value) setStep("add_item");
                        }} style={{ marginTop: 4 }}>
                            <option value="">Select…</option>
                            {locations.map(l => (
                                <option key={l.id} value={l.id}>{l.code} — {l.name}</option>
                            ))}
                        </select>
                    </div>
                </div>
            )}

            {/* Step 4: Add Item */}
            {step === "add_item" && (
                <div className="card" style={{ padding: 24 }}>
                    <h2>Add Return Item</h2>
                    {locationLabel && <p style={{ color: "var(--color-success)", marginBottom: 12 }}>📍 Location: {locationLabel}</p>}

                    <div className="form-group">
                        <label>Product *</label>
                        <select className="input" value={productId} onChange={(e) => setProductId(e.target.value)}>
                            <option value="">Select…</option>
                            {products.map(p => (
                                <option key={p.id} value={p.id}>{p.sku} — {p.name}</option>
                            ))}
                        </select>
                    </div>

                    <div className="form-group">
                        <label>Qty *</label>
                        <input type="number" className="input" min={1} value={qty} onChange={(e) => setQty(parseInt(e.target.value) || 1)} />
                    </div>

                    <div className="form-group">
                        <label>Disposition: <strong>{disposition}</strong></label>
                    </div>

                    <div className="form-group">
                        <label>Note</label>
                        <input className="input" value={note} onChange={(e) => setNote(e.target.value)} placeholder="Optional" />
                    </div>

                    <button className="btn btn-primary" onClick={handleSubmitItem} disabled={submitting} style={{ width: "100%", marginBottom: 8 }}>
                        {submitting ? "Adding…" : "✓ Submit Item"}
                    </button>
                    <button className="btn btn-secondary" onClick={() => setStep("scan_order")} style={{ width: "100%" }}>
                        Done — New Order
                    </button>
                </div>
            )}
        </div>
    );
}
