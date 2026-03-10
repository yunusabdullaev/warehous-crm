"use client";
import { useEffect, useState } from "react";
import api from "@/lib/api";
import AuthGuard from "@/components/AuthGuard";
import type { HistoryRecord, PaginatedResponse } from "@/lib/types";

export default function HistoryPage() {
    return (
        <AuthGuard requiredAction="history:view">
            <HistoryContent />
        </AuthGuard>
    );
}

function HistoryContent() {
    const [records, setRecords] = useState<HistoryRecord[]>([]);
    const [loading, setLoading] = useState(true);
    const [entityType, setEntityType] = useState("");

    const fetchHistory = async () => {
        setLoading(true);
        try {
            const params = new URLSearchParams({ limit: "100" });
            if (entityType) params.set("entity_type", entityType);
            const res = await api.get<PaginatedResponse<HistoryRecord>>(`/history?${params}`);
            setRecords(res.data.data || []);
        } catch { /* interceptor */ }
        setLoading(false);
    };

    useEffect(() => { fetchHistory(); }, [entityType]);

    const actionColor: Record<string, string> = {
        create: "bg-emerald-100 text-emerald-700",
        update: "bg-blue-100 text-blue-700",
        delete: "bg-red-100 text-red-700",
    };

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">History</h1>
                <select value={entityType} onChange={(e) => setEntityType(e.target.value)}
                    className="px-3 py-2 border rounded-lg text-sm">
                    <option value="">All entity types</option>
                    <option value="product">Product</option>
                    <option value="location">Location</option>
                    <option value="inbound">Inbound</option>
                    <option value="outbound">Outbound</option>
                </select>
            </div>

            {loading ? (
                <div className="flex justify-center py-20">
                    <div className="animate-spin w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full" />
                </div>
            ) : records.length === 0 ? (
                <div className="text-center py-20 text-gray-400">
                    <p className="text-4xl mb-2">📜</p>
                    <p>No history records</p>
                </div>
            ) : (
                <div className="space-y-3">
                    {records.map((h) => (
                        <div key={h.id} className="bg-white rounded-xl shadow-sm border px-5 py-4 flex items-start justify-between">
                            <div className="flex items-start gap-3">
                                <span className={`px-2 py-0.5 rounded text-xs font-medium ${actionColor[h.action] || "bg-gray-100 text-gray-700"}`}>
                                    {h.action}
                                </span>
                                <div>
                                    <p className="text-sm">
                                        <span className="font-medium capitalize">{h.entity_type}</span>
                                        <span className="text-gray-500 ml-1 font-mono text-xs">{h.entity_id.slice(-6)}</span>
                                    </p>
                                    {h.details && <p className="text-xs text-gray-500 mt-0.5">{h.details}</p>}
                                </div>
                            </div>
                            <div className="text-right text-xs text-gray-400 shrink-0">
                                <p>{new Date(h.timestamp).toLocaleString()}</p>
                                <p className="font-mono">{h.user_id.slice(-6)}</p>
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}
