"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { useSearchParams } from "next/navigation";
import api from "@/lib/api";
import { getToken } from "@/lib/auth";
import type { PickTask, Product, Location, PaginatedResponse } from "@/lib/types";

type Step = "location" | "task" | "qty" | "done";

export default function PickScanPage() {
    const searchParams = useSearchParams();
    const initialOrderId = searchParams.get("orderId") || "";

    const [orderId, setOrderId] = useState(initialOrderId);
    const [step, setStep] = useState<Step>(initialOrderId ? "location" : "location");
    const [tasks, setTasks] = useState<PickTask[]>([]);
    const [filteredTasks, setFilteredTasks] = useState<PickTask[]>([]);
    const [selectedTask, setSelectedTask] = useState<PickTask | null>(null);
    const [scannedLocationId, setScannedLocationId] = useState("");
    const [locationCode, setLocationCode] = useState("");
    const [qty, setQty] = useState("");
    const [loading, setLoading] = useState(false);
    const [message, setMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);
    const [products, setProducts] = useState<Record<string, Product>>({});
    const [locations, setLocations] = useState<Record<string, Location>>({});
    const qtyRef = useRef<HTMLInputElement>(null);


    // Load tasks for order
    const loadTasks = useCallback(async () => {
        if (!orderId) return;
        setLoading(true);
        try {
            const res = await api.get<PaginatedResponse<PickTask>>(`/orders/${orderId}/pick-tasks`);
            const taskList: PickTask[] = (res.data.data || []).filter(
                (t: PickTask) => t.status === "OPEN" || t.status === "IN_PROGRESS"
            );
            setTasks(taskList);

            // Load product/location info
            const prodIds = [...new Set(taskList.map((t) => t.product_id))];
            const locIds = [...new Set(taskList.map((t) => t.location_id))];
            const pm: Record<string, Product> = {};
            const lm: Record<string, Location> = {};
            await Promise.all([
                ...prodIds.map(async (pid) => {
                    try {
                        const r = await api.get<Product>(`/products/${pid}`);
                        pm[pid] = r.data;
                    } catch { }
                }),
                ...locIds.map(async (lid) => {
                    try {
                        const r = await api.get<Location>(`/locations/${lid}`);
                        lm[lid] = r.data;
                    } catch { }
                }),
            ]);
            setProducts(pm);
            setLocations(lm);
        } catch (e: any) {
            setMessage({ type: "error", text: e.response?.data?.error || e.message || "Error" });
        } finally {
            setLoading(false);
        }
    }, [orderId]);

    useEffect(() => { loadTasks(); }, [loadTasks]);

    // Handle location scan/input
    const handleLocationScan = (input: string) => {
        const trimmed = input.trim();
        if (!trimmed) return;

        // Try to match by location ID or code
        let matchedLocId = "";
        for (const t of tasks) {
            if (t.location_id === trimmed) {
                matchedLocId = t.location_id;
                break;
            }
        }
        // Try matching by location code
        if (!matchedLocId) {
            for (const [lid, loc] of Object.entries(locations)) {
                if (loc.code.toUpperCase() === trimmed.toUpperCase()) {
                    matchedLocId = lid;
                    break;
                }
            }
        }

        if (!matchedLocId) {
            setMessage({ type: "error", text: "❌ Location not found in pick plan. Scan a valid location." });
            return;
        }

        const matching = tasks.filter((t) => t.location_id === matchedLocId && (t.status === "OPEN" || t.status === "IN_PROGRESS"));
        if (matching.length === 0) {
            setMessage({ type: "error", text: "❌ No open tasks at this location." });
            return;
        }

        setScannedLocationId(matchedLocId);
        const loc = locations[matchedLocId];
        setLocationCode(loc ? loc.code : matchedLocId);
        setFilteredTasks(matching);
        setMessage(null);
        setStep("task");
    };

    // Handle task selection
    const handleSelectTask = (task: PickTask) => {
        setSelectedTask(task);
        const remaining = task.planned_qty - task.picked_qty;
        setQty(String(remaining));
        setStep("qty");
        setTimeout(() => qtyRef.current?.focus(), 100);
    };

    // Submit scan
    const handleSubmitScan = async () => {
        if (!selectedTask) return;
        const qNum = parseInt(qty, 10);
        if (!qNum || qNum <= 0) {
            setMessage({ type: "error", text: "❌ Enter a valid quantity" });
            return;
        }
        const remaining = selectedTask.planned_qty - selectedTask.picked_qty;
        if (qNum > remaining) {
            setMessage({ type: "error", text: `❌ Max pickable: ${remaining}` });
            return;
        }

        setLoading(true);
        try {
            const res = await api.post<PickTask>(`/pick-tasks/${selectedTask.id}/scan`, {
                location_id: scannedLocationId,
                product_id: selectedTask.product_id,
                lot_id: selectedTask.lot_id,
                qty: qNum,
                scanner: "camera",
                client: "web",
            });

            const updated = res.data;
            setMessage({
                type: "success",
                text: `✅ Picked ${qNum} — now ${updated.picked_qty}/${updated.planned_qty}${updated.status === "DONE" ? " ✓ DONE!" : ""}`,
            });

            // Reload tasks and go back to location step
            await loadTasks();
            setStep("done");
            setTimeout(() => {
                setSelectedTask(null);
                setQty("");
                setStep("location");
                setScannedLocationId("");
                setFilteredTasks([]);
                setMessage(null);
            }, 2000);
        } catch {
            setMessage({ type: "error", text: "❌ Network error" });
        } finally {
            setLoading(false);
        }
    };

    // Calculate overall progress
    const allTotalPlanned = tasks.reduce((s, t) => s + t.planned_qty, 0);
    const allTotalPicked = tasks.reduce((s, t) => s + t.picked_qty, 0);
    const overallPct = allTotalPlanned > 0 ? (allTotalPicked / allTotalPlanned) * 100 : 0;

    return (
        <div style={{ maxWidth: 480, margin: "0 auto", padding: "16px", minHeight: "100dvh", display: "flex", flexDirection: "column" }}>
            {/* Header */}
            <div style={{ marginBottom: 16 }}>
                <h1 style={{ fontSize: 20, fontWeight: 700, margin: 0 }}>🎯 Scan to Pick</h1>
                {!initialOrderId && (
                    <div style={{ display: "flex", gap: 8, marginTop: 8 }}>
                        <input
                            value={orderId}
                            onChange={(e) => setOrderId(e.target.value)}
                            placeholder="Order ID"
                            style={{ flex: 1, padding: "10px 14px", border: "2px solid #d1d5db", borderRadius: 10, fontSize: 16 }}
                        />
                        <button onClick={loadTasks} style={{ padding: "10px 20px", background: "#3b82f6", color: "#fff", border: "none", borderRadius: 10, fontWeight: 600 }}>Load</button>
                    </div>
                )}
            </div>

            {/* Overall progress bar */}
            {tasks.length > 0 && (
                <div style={{ marginBottom: 16, padding: 12, background: "#f0f9ff", borderRadius: 12 }}>
                    <div style={{ display: "flex", justifyContent: "space-between", fontSize: 13, fontWeight: 600, marginBottom: 4, color: "#1e40af" }}>
                        <span>Order Progress</span>
                        <span>{allTotalPicked}/{allTotalPlanned} ({overallPct.toFixed(0)}%)</span>
                    </div>
                    <div style={{ height: 8, background: "#bfdbfe", borderRadius: 4, overflow: "hidden" }}>
                        <div style={{ height: "100%", width: `${overallPct}%`, background: overallPct === 100 ? "#22c55e" : "#3b82f6", borderRadius: 4, transition: "width 0.3s" }} />
                    </div>
                </div>
            )}

            {/* Message */}
            {message && (
                <div style={{
                    padding: "14px 18px",
                    background: message.type === "success" ? "#dcfce7" : "#fee2e2",
                    color: message.type === "success" ? "#166534" : "#991b1b",
                    borderRadius: 12,
                    marginBottom: 16,
                    fontSize: 16,
                    fontWeight: 600,
                    textAlign: "center",
                }}>
                    {message.text}
                </div>
            )}

            {/* Step indicators */}
            <div style={{ display: "flex", gap: 4, marginBottom: 20, justifyContent: "center" }}>
                {(["location", "task", "qty"] as Step[]).map((s, i) => (
                    <div key={s} style={{ display: "flex", alignItems: "center", gap: 4 }}>
                        <div style={{
                            width: 32, height: 32, borderRadius: "50%", display: "flex", alignItems: "center", justifyContent: "center",
                            background: step === s || (["location", "task", "qty"].indexOf(step) > i) ? "#3b82f6" : "#e2e8f0",
                            color: step === s || (["location", "task", "qty"].indexOf(step) > i) ? "#fff" : "#9ca3af",
                            fontWeight: 700, fontSize: 14,
                        }}>
                            {i + 1}
                        </div>
                        <span style={{ fontSize: 12, color: step === s ? "#1d4ed8" : "#9ca3af", fontWeight: step === s ? 600 : 400 }}>
                            {s === "location" ? "Location" : s === "task" ? "Task" : "Qty"}
                        </span>
                        {i < 2 && <span style={{ color: "#d1d5db", margin: "0 4px" }}>→</span>}
                    </div>
                ))}
            </div>

            {/* Step 1: Location */}
            {step === "location" && (
                <div style={{ flex: 1, display: "flex", flexDirection: "column", gap: 12 }}>
                    <h2 style={{ fontSize: 18, fontWeight: 600, textAlign: "center", margin: 0 }}>📍 Scan Location QR</h2>
                    <p style={{ textAlign: "center", color: "#6b7280", fontSize: 14, margin: 0 }}>
                        Scan the location QR code or enter the location code
                    </p>
                    <input
                        autoFocus
                        placeholder="Location code or ID..."
                        onKeyDown={(e) => {
                            if (e.key === "Enter") handleLocationScan((e.target as HTMLInputElement).value);
                        }}
                        style={{ padding: "16px", border: "2px solid #3b82f6", borderRadius: 12, fontSize: 18, textAlign: "center", fontWeight: 600 }}
                    />
                    {/* Show available locations */}
                    <div style={{ marginTop: 8 }}>
                        <p style={{ fontSize: 13, color: "#6b7280", marginBottom: 8 }}>Or tap a location:</p>
                        <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                            {[...new Set(tasks.map((t) => t.location_id))].map((lid) => {
                                const loc = locations[lid];
                                const locTasks = tasks.filter((t) => t.location_id === lid && (t.status === "OPEN" || t.status === "IN_PROGRESS"));
                                if (locTasks.length === 0) return null;
                                return (
                                    <button
                                        key={lid}
                                        onClick={() => handleLocationScan(lid)}
                                        style={{ padding: "12px 16px", background: "#eff6ff", border: "2px solid #bfdbfe", borderRadius: 10, cursor: "pointer", fontWeight: 600, fontSize: 14, color: "#1d4ed8" }}
                                    >
                                        📍 {loc ? loc.code : lid.slice(-6)}
                                        <span style={{ display: "block", fontSize: 11, color: "#6b7280", fontWeight: 400 }}>{locTasks.length} tasks</span>
                                    </button>
                                );
                            })}
                        </div>
                    </div>
                </div>
            )}

            {/* Step 2: Select Task */}
            {step === "task" && (
                <div style={{ flex: 1, display: "flex", flexDirection: "column", gap: 8 }}>
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                        <h2 style={{ fontSize: 18, fontWeight: 600, margin: 0 }}>📦 Select Task at {locationCode}</h2>
                        <button onClick={() => { setStep("location"); setFilteredTasks([]); setScannedLocationId(""); }} style={{ padding: "6px 12px", background: "#e5e7eb", border: "none", borderRadius: 8, fontSize: 13, cursor: "pointer" }}>← Back</button>
                    </div>
                    {filteredTasks.map((t) => {
                        const prod = products[t.product_id];
                        const remaining = t.planned_qty - t.picked_qty;
                        const pct = (t.picked_qty / t.planned_qty) * 100;
                        return (
                            <button
                                key={t.id}
                                onClick={() => handleSelectTask(t)}
                                style={{
                                    padding: "16px", background: "#fff", border: "2px solid #e2e8f0", borderRadius: 12, cursor: "pointer",
                                    textAlign: "left", display: "flex", flexDirection: "column", gap: 8,
                                }}
                            >
                                <div style={{ fontWeight: 600, fontSize: 16 }}>{prod ? `${prod.sku} — ${prod.name}` : t.product_id}</div>
                                <div style={{ display: "flex", justifyContent: "space-between", fontSize: 14, color: "#6b7280" }}>
                                    <span>Remaining: <strong style={{ color: "#1d4ed8" }}>{remaining}</strong></span>
                                    <span>{t.picked_qty}/{t.planned_qty}</span>
                                </div>
                                <div style={{ height: 6, background: "#e2e8f0", borderRadius: 3, overflow: "hidden" }}>
                                    <div style={{ height: "100%", width: `${pct}%`, background: "#f59e0b", borderRadius: 3 }} />
                                </div>
                            </button>
                        );
                    })}
                </div>
            )}

            {/* Step 3: Enter Qty */}
            {step === "qty" && selectedTask && (
                <div style={{ flex: 1, display: "flex", flexDirection: "column", gap: 16, alignItems: "center" }}>
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", width: "100%" }}>
                        <h2 style={{ fontSize: 18, fontWeight: 600, margin: 0 }}>🔢 Enter Quantity</h2>
                        <button onClick={() => { setStep("task"); setSelectedTask(null); }} style={{ padding: "6px 12px", background: "#e5e7eb", border: "none", borderRadius: 8, fontSize: 13, cursor: "pointer" }}>← Back</button>
                    </div>

                    <div style={{ background: "#f8fafc", borderRadius: 12, padding: 16, width: "100%" }}>
                        <div style={{ fontWeight: 600, fontSize: 16 }}>
                            {products[selectedTask.product_id]?.sku || selectedTask.product_id}
                        </div>
                        <div style={{ color: "#6b7280", fontSize: 14 }}>
                            Max: {selectedTask.planned_qty - selectedTask.picked_qty} | Location: {locationCode}
                        </div>
                    </div>

                    <input
                        ref={qtyRef}
                        type="number"
                        inputMode="numeric"
                        value={qty}
                        onChange={(e) => setQty(e.target.value)}
                        onKeyDown={(e) => { if (e.key === "Enter") handleSubmitScan(); }}
                        style={{ padding: "20px", border: "3px solid #3b82f6", borderRadius: 16, fontSize: 36, fontWeight: 700, textAlign: "center", width: "100%", maxWidth: 200 }}
                    />

                    <button
                        onClick={handleSubmitScan}
                        disabled={loading}
                        style={{
                            padding: "18px 48px", background: loading ? "#9ca3af" : "#22c55e", color: "#fff",
                            border: "none", borderRadius: 14, fontWeight: 700, fontSize: 20, cursor: loading ? "default" : "pointer",
                            width: "100%", maxWidth: 300,
                        }}
                    >
                        {loading ? "Submitting..." : "✅ Confirm Pick"}
                    </button>
                </div>
            )}

            {/* Step done */}
            {step === "done" && (
                <div style={{ flex: 1, display: "flex", alignItems: "center", justifyContent: "center" }}>
                    <div style={{ fontSize: 64, animation: "pulse 0.5s" }}>✅</div>
                </div>
            )}
        </div>
    );
}
