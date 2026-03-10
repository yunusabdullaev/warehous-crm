"use client";
import { useEffect, useState } from "react";
import dynamic from "next/dynamic";
import api from "@/lib/api";
import { can } from "@/lib/auth";
import { toast } from "@/components/Toast";
import type { Product, StockRecord, PaginatedResponse } from "@/lib/types";

const QrScanner = dynamic(() => import("@/components/QrScanner"), { ssr: false });

interface QrPayload {
    type: string;
    locationId: string;
    zone: string;
    rack: string;
    shelf: string;
}

interface StockWithProduct extends StockRecord {
    product?: Product;
}

export default function OutboundScanPage() {
    const [scanning, setScanning] = useState(true);
    const [qrData, setQrData] = useState<QrPayload | null>(null);
    const [locationName, setLocationName] = useState("");
    const [stockItems, setStockItems] = useState<StockWithProduct[]>([]);
    const [allProducts, setAllProducts] = useState<Product[]>([]);
    const [productId, setProductId] = useState("");
    const [maxQty, setMaxQty] = useState(0);
    const [quantity, setQuantity] = useState(1);
    const [reference, setReference] = useState("");
    const [submitting, setSubmitting] = useState(false);
    const [cameraError, setCameraError] = useState("");
    const [loadingStock, setLoadingStock] = useState(false);

    // RBAC guard — loader must not access
    const allowed = can("outbound:view");

    // Fetch products on mount for name resolution
    useEffect(() => {
        api.get<PaginatedResponse<Product>>("/products?limit=500")
            .then((res) => setAllProducts(res.data.data || []))
            .catch(() => { });
    }, []);

    const handleScan = async (data: string) => {
        try {
            const payload: QrPayload = JSON.parse(data);
            if (payload.type !== "location" || !payload.locationId) {
                toast("error", "Invalid QR code — not a location");
                setScanning(true);
                return;
            }
            setQrData(payload);
            setScanning(false);
            setLoadingStock(true);

            // Fetch location name
            try {
                const locRes = await api.get(`/locations/${payload.locationId}`);
                setLocationName((locRes.data as { name: string }).name);
            } catch {
                setLocationName("");
            }

            // Fetch stock at this location
            const res = await api.get<StockRecord[]>(`/stock/location/${payload.locationId}`);
            const stocks = Array.isArray(res.data) ? res.data : [];
            // Merge product names
            const enriched: StockWithProduct[] = stocks
                .filter((s) => s.quantity > 0)
                .map((s) => ({
                    ...s,
                    product: allProducts.find((p) => p.id === s.product_id),
                }));
            setStockItems(enriched);
            setLoadingStock(false);
        } catch {
            toast("error", "Could not parse QR code");
            setScanning(true);
        }
    };

    const handleProductSelect = (pid: string) => {
        setProductId(pid);
        const item = stockItems.find((s) => s.product_id === pid);
        setMaxQty(item?.quantity || 0);
        setQuantity(1);
    };

    const handleSubmit = async () => {
        if (!qrData || !productId || quantity < 1) {
            toast("error", "Please fill all fields");
            return;
        }
        if (quantity > maxQty) {
            toast("error", `Insufficient stock — max available: ${maxQty}`);
            return;
        }
        setSubmitting(true);
        try {
            await api.post("/outbound", {
                product_id: productId,
                location_id: qrData.locationId,
                quantity,
                reference: reference || `SCAN-OUT-${Date.now()}`,
            });
            toast("success", "Outbound recorded successfully!");
            // Reset
            setQrData(null);
            setStockItems([]);
            setProductId("");
            setMaxQty(0);
            setQuantity(1);
            setReference("");
            setScanning(true);
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Failed to create outbound");
        }
        setSubmitting(false);
    };

    const resetScan = () => {
        setQrData(null);
        setStockItems([]);
        setProductId("");
        setScanning(true);
    };

    if (!allowed) {
        return (
            <div className="text-center py-20 text-gray-400">
                <p className="text-4xl mb-2">🔒</p>
                <p>You do not have access to outbound scanning</p>
            </div>
        );
    }

    return (
        <div className="max-w-lg mx-auto">
            <h1 className="text-2xl font-bold mb-6">📸 Scan Outbound (Pick)</h1>

            {/* Step 1: Scan */}
            {scanning && (
                <div className="space-y-4">
                    <div className="bg-white rounded-xl shadow-sm border p-4">
                        <h2 className="text-sm font-semibold text-gray-500 uppercase mb-3">Scan Location QR Code</h2>
                        {cameraError ? (
                            <div className="bg-red-50 text-red-700 rounded-lg p-4 text-sm">
                                <p className="font-medium">Camera Error</p>
                                <p>{cameraError}</p>
                                <button onClick={() => { setCameraError(""); setScanning(true); }}
                                    className="mt-2 text-red-600 underline text-xs">
                                    Try Again
                                </button>
                            </div>
                        ) : (
                            <QrScanner
                                onScan={handleScan}
                                onError={(err) => setCameraError(err)}
                            />
                        )}
                    </div>
                    <p className="text-center text-sm text-gray-400">
                        Point your camera at a location QR code to pick items
                    </p>
                </div>
            )}

            {/* Step 2: Pick form */}
            {!scanning && qrData && (
                <div className="space-y-4">
                    {/* Location Card */}
                    <div className="bg-amber-50 border border-amber-200 rounded-xl p-4">
                        <div className="flex items-center justify-between">
                            <div>
                                <p className="text-xs font-semibold text-amber-600 uppercase">Picking from ✓</p>
                                <p className="text-2xl font-black mt-1">
                                    {qrData.zone}-{qrData.rack}-{qrData.shelf}
                                </p>
                                {locationName && <p className="text-sm text-gray-500">{locationName}</p>}
                            </div>
                            <button onClick={resetScan} className="text-sm text-amber-600 hover:underline">
                                Re-scan
                            </button>
                        </div>
                    </div>

                    {loadingStock ? (
                        <div className="flex justify-center py-8">
                            <div className="animate-spin w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full" />
                        </div>
                    ) : stockItems.length === 0 ? (
                        <div className="bg-white rounded-xl shadow-sm border p-8 text-center text-gray-400">
                            <p className="text-3xl mb-2">📭</p>
                            <p>No stock at this location</p>
                            <button onClick={resetScan} className="mt-3 text-blue-600 underline text-sm">Scan another</button>
                        </div>
                    ) : (
                        <div className="bg-white rounded-xl shadow-sm border p-5 space-y-4">
                            <div>
                                <label className="block text-sm font-medium mb-1">Product *</label>
                                <select
                                    value={productId}
                                    onChange={(e) => handleProductSelect(e.target.value)}
                                    className="w-full px-3 py-2 border rounded-lg text-sm"
                                >
                                    <option value="">Select a product</option>
                                    {stockItems.map((s) => (
                                        <option key={s.product_id} value={s.product_id}>
                                            {s.product?.sku || s.product_id} — {s.product?.name || "Unknown"} (avail: {s.quantity})
                                        </option>
                                    ))}
                                </select>
                            </div>
                            {productId && (
                                <>
                                    <div>
                                        <label className="block text-sm font-medium mb-1">
                                            Quantity * <span className="text-gray-400 text-xs">(max: {maxQty})</span>
                                        </label>
                                        <input
                                            type="number"
                                            min={1}
                                            max={maxQty}
                                            value={quantity}
                                            onChange={(e) => setQuantity(parseInt(e.target.value) || 1)}
                                            className="w-full px-3 py-2 border rounded-lg text-sm"
                                        />
                                    </div>
                                    <div>
                                        <label className="block text-sm font-medium mb-1">Reference (optional)</label>
                                        <input
                                            value={reference}
                                            onChange={(e) => setReference(e.target.value)}
                                            placeholder="e.g. SO-12345"
                                            className="w-full px-3 py-2 border rounded-lg text-sm"
                                        />
                                    </div>
                                    <button
                                        onClick={handleSubmit}
                                        disabled={submitting || !productId || quantity > maxQty}
                                        className="w-full bg-amber-500 hover:bg-amber-600 text-white font-medium py-2.5 rounded-lg transition disabled:opacity-50 disabled:cursor-not-allowed"
                                    >
                                        {submitting ? "Submitting..." : "📤 Record Outbound"}
                                    </button>
                                </>
                            )}
                        </div>
                    )}
                </div>
            )}
        </div>
    );
}
