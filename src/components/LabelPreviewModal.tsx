"use client";
import { useEffect, useState } from "react";
import api from "@/lib/api";
import type { Location } from "@/lib/types";

interface Props {
    open: boolean;
    onClose: () => void;
    location: Location;
}

export default function LabelPreviewModal({ open, onClose, location }: Props) {
    const [loading, setLoading] = useState(false);

    if (!open) return null;


    const downloadPdf = async () => {
        setLoading(true);
        try {
            const res = await api.get(`/locations/${location.id}/label`, { responseType: "blob" });
            const url = URL.createObjectURL(new Blob([res.data], { type: "application/pdf" }));
            const a = document.createElement("a");
            a.href = url;
            a.download = `label_${location.code}.pdf`;
            a.click();
            URL.revokeObjectURL(url);
        } catch {
            alert("Failed to download PDF");
        }
        setLoading(false);
    };

    const printLabel = async () => {
        setLoading(true);
        try {
            const res = await api.get(`/locations/${location.id}/label`, { responseType: "blob" });
            const url = URL.createObjectURL(new Blob([res.data], { type: "application/pdf" }));
            const w = window.open(url, "_blank");
            if (w) {
                w.addEventListener("load", () => w.print());
            }
        } catch {
            alert("Failed to print label");
        }
        setLoading(false);
    };

    const label = [location.zone, location.rack, location.level].filter(Boolean).join("-") || location.code;

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
            <div className="bg-white rounded-2xl shadow-2xl w-full max-w-md mx-4">
                {/* Header */}
                <div className="flex items-center justify-between p-5 border-b">
                    <h2 className="text-lg font-bold">📋 Location Label</h2>
                    <button onClick={onClose} className="text-gray-400 hover:text-gray-600 text-xl leading-none">×</button>
                </div>

                <div className="p-6 space-y-5">
                    {/* Label preview card */}
                    <div className="border-2 border-dashed border-gray-200 rounded-xl p-6 text-center bg-gray-50">
                        <h3 className="text-3xl font-black tracking-wider mb-3">{label}</h3>
                        <p className="text-sm text-gray-500 mb-4">{location.name}</p>

                        {/* QR preview (fetched from API with auth header via img won't work, so we use the api client) */}
                        <div className="inline-block border rounded-lg overflow-hidden bg-white p-2">
                            <QrPreview locationId={location.id} />
                        </div>

                        <p className="text-xs text-gray-400 mt-3 font-mono">ID: {location.id}</p>
                    </div>

                    {/* Actions */}
                    <div className="flex gap-3">
                        <button
                            onClick={downloadPdf}
                            disabled={loading}
                            className="flex-1 bg-blue-600 hover:bg-blue-700 text-white font-medium py-2.5 rounded-lg transition disabled:opacity-50"
                        >
                            📥 Download PDF
                        </button>
                        <button
                            onClick={printLabel}
                            disabled={loading}
                            className="flex-1 bg-emerald-600 hover:bg-emerald-700 text-white font-medium py-2.5 rounded-lg transition disabled:opacity-50"
                        >
                            🖨️ Print
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
}

// Internal component to fetch QR as blob and render as <img>
function QrPreview({ locationId }: { locationId: string }) {
    const [src, setSrc] = useState<string | null>(null);

    // Fetch on mount
    useEffect(() => {
        api.get(`/locations/${locationId}/qr`, { responseType: "blob" })
            .then((res) => {
                const url = URL.createObjectURL(new Blob([res.data]));
                setSrc(url);
            })
            .catch(() => { });
    }, [locationId]);

    if (!src) {
        return (
            <div className="w-32 h-32 flex items-center justify-center">
                <div className="animate-spin w-6 h-6 border-3 border-blue-500 border-t-transparent rounded-full" />
            </div>
        );
    }

    return <img src={src} alt="QR Code" className="w-32 h-32" />;
}
