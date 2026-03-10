"use client";
import { useEffect, useState } from "react";
import dynamic from "next/dynamic";
import api from "@/lib/api";
import { toast } from "@/components/Toast";
import type { Product, PaginatedResponse, Location } from "@/lib/types";

// Dynamic import for QR scanner (uses browser APIs / camera)
const QrScanner = dynamic(() => import("@/components/QrScanner"), { ssr: false });

interface QrPayload {
    type: string;
    locationId: string;
    zone: string;
    rack: string;
    shelf: string;
}

export default function InboundScanPage() {
    const [scanning, setScanning] = useState(true);
    const [scannedLocation, setScannedLocation] = useState<Location | null>(null);
    const [qrData, setQrData] = useState<QrPayload | null>(null);
    const [products, setProducts] = useState<Product[]>([]);
    const [productId, setProductId] = useState("");
    const [quantity, setQuantity] = useState(1);
    const [reference, setReference] = useState("");
    const [lotNo, setLotNo] = useState("");
    const [expDate, setExpDate] = useState("");
    const [submitting, setSubmitting] = useState(false);
    const [cameraError, setCameraError] = useState("");

    useEffect(() => {
        api.get<PaginatedResponse<Product>>("/products?limit=500")
            .then((res) => setProducts(res.data.data || []))
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
            // Fetch full location details
            const res = await api.get<Location>(`/locations/${payload.locationId}`);
            setScannedLocation(res.data);
            setScanning(false);
        } catch {
            toast("error", "Could not parse QR code");
            setScanning(true);
        }
    };

    const handleSubmit = async () => {
        if (!scannedLocation || !productId || quantity < 1) {
            toast("error", "Please fill all fields");
            return;
        }
        setSubmitting(true);
        try {
            await api.post("/inbound", {
                product_id: productId,
                location_id: scannedLocation.id,
                quantity,
                reference: reference || `SCAN-IN-${Date.now()}`,
                lot_no: lotNo,
                exp_date: expDate || undefined,
            });
            toast("success", "Inbound recorded successfully!");
            // Reset for next scan
            setScannedLocation(null);
            setQrData(null);
            setProductId("");
            setQuantity(1);
            setReference("");
            setLotNo("");
            setExpDate("");
            setScanning(true);
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Failed to create inbound");
        }
        setSubmitting(false);
    };

    const resetScan = () => {
        setScannedLocation(null);
        setQrData(null);
        setScanning(true);
    };

    return (
        <div className="max-w-lg mx-auto">
            <h1 className="text-2xl font-bold mb-6">📷 Scan Inbound</h1>

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
                        Point your camera at a location QR code
                    </p>
                </div>
            )}

            {/* Step 2: Form */}
            {!scanning && scannedLocation && (
                <div className="space-y-4">
                    {/* Scanned Location Card */}
                    <div className="bg-emerald-50 border border-emerald-200 rounded-xl p-4">
                        <div className="flex items-center justify-between">
                            <div>
                                <p className="text-xs font-semibold text-emerald-600 uppercase">Location Scanned ✓</p>
                                <p className="text-2xl font-black mt-1">
                                    {qrData?.zone}-{qrData?.rack}-{qrData?.shelf}
                                </p>
                                <p className="text-sm text-gray-500">{scannedLocation.name}</p>
                            </div>
                            <button onClick={resetScan} className="text-sm text-emerald-600 hover:underline">
                                Re-scan
                            </button>
                        </div>
                    </div>

                    {/* Product + Quantity */}
                    <div className="bg-white rounded-xl shadow-sm border p-5 space-y-4">
                        <div>
                            <label className="block text-sm font-medium mb-1">Product *</label>
                            <select
                                value={productId}
                                onChange={(e) => setProductId(e.target.value)}
                                className="w-full px-3 py-2 border rounded-lg text-sm"
                            >
                                <option value="">Select a product</option>
                                {products.map((p) => (
                                    <option key={p.id} value={p.id}>
                                        {p.sku} — {p.name} ({p.unit})
                                    </option>
                                ))}
                            </select>
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Quantity *</label>
                            <input
                                type="number"
                                min={1}
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
                                placeholder="e.g. PO-12345"
                                className="w-full px-3 py-2 border rounded-lg text-sm"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Lot No *</label>
                            <input
                                value={lotNo}
                                onChange={(e) => setLotNo(e.target.value)}
                                placeholder="e.g. LOT-2024-001"
                                className="w-full px-3 py-2 border rounded-lg text-sm"
                            />
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Exp Date (optional)</label>
                            <input
                                type="date"
                                value={expDate}
                                onChange={(e) => setExpDate(e.target.value)}
                                className="w-full px-3 py-2 border rounded-lg text-sm"
                            />
                        </div>
                        <button
                            onClick={handleSubmit}
                            disabled={submitting || !productId || !lotNo}
                            className="w-full bg-emerald-600 hover:bg-emerald-700 text-white font-medium py-2.5 rounded-lg transition disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                            {submitting ? "Submitting..." : "📥 Record Inbound"}
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
}
