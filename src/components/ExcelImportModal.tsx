"use client";
import { useState, useRef } from "react";
import api from "@/lib/api";

interface RowError {
    row: number;
    field: string;
    message: string;
}

interface ImportReport {
    inserted: number;
    updated: number;
    skipped: number;
    failed: number;
    errors: RowError[];
}

interface Props {
    open: boolean;
    onClose: () => void;
    endpoint: string; // e.g. "/import/products"
    title: string;
    onDone?: () => void;
}

export default function ExcelImportModal({ open, onClose, endpoint, title, onDone }: Props) {
    const [file, setFile] = useState<File | null>(null);
    const [uploading, setUploading] = useState(false);
    const [progress, setProgress] = useState(0);
    const [report, setReport] = useState<ImportReport | null>(null);
    const [error, setError] = useState("");
    const inputRef = useRef<HTMLInputElement>(null);

    if (!open) return null;

    const handleUpload = async () => {
        if (!file) return;
        setUploading(true);
        setError("");
        setReport(null);
        setProgress(0);

        const formData = new FormData();
        formData.append("file", file);

        try {
            const res = await api.post<ImportReport>(endpoint, formData, {
                headers: { "Content-Type": "multipart/form-data" },
                onUploadProgress: (e) => {
                    if (e.total) setProgress(Math.round((e.loaded / e.total) * 100));
                },
            });
            setReport(res.data);
            if (onDone) onDone();
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            setError(msg || "Upload failed");
        } finally {
            setUploading(false);
        }
    };

    const handleClose = () => {
        setFile(null);
        setReport(null);
        setError("");
        setProgress(0);
        onClose();
    };

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
            <div className="bg-white rounded-2xl shadow-2xl w-full max-w-lg mx-4 max-h-[90vh] overflow-auto">
                {/* Header */}
                <div className="flex items-center justify-between p-5 border-b">
                    <h2 className="text-lg font-bold">{title}</h2>
                    <button onClick={handleClose} className="text-gray-400 hover:text-gray-600 text-xl leading-none">×</button>
                </div>

                <div className="p-5 space-y-5">
                    {/* File Picker */}
                    {!report && (
                        <>
                            <div
                                onClick={() => inputRef.current?.click()}
                                className="border-2 border-dashed border-gray-300 rounded-xl p-8 text-center cursor-pointer hover:border-blue-400 transition"
                            >
                                {file ? (
                                    <div>
                                        <p className="text-sm font-medium">{file.name}</p>
                                        <p className="text-xs text-gray-400 mt-1">{(file.size / 1024).toFixed(1)} KB</p>
                                    </div>
                                ) : (
                                    <div>
                                        <p className="text-3xl mb-2">📊</p>
                                        <p className="text-sm text-gray-500">Click to select Excel file (.xlsx)</p>
                                    </div>
                                )}
                                <input
                                    ref={inputRef}
                                    type="file"
                                    accept=".xlsx"
                                    className="hidden"
                                    onChange={(e) => setFile(e.target.files?.[0] || null)}
                                />
                            </div>

                            {/* Progress */}
                            {uploading && (
                                <div className="w-full bg-gray-200 rounded-full h-2">
                                    <div className="bg-blue-600 h-2 rounded-full transition-all" style={{ width: `${progress}%` }} />
                                </div>
                            )}

                            {error && (
                                <div className="bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg px-4 py-3">
                                    {error}
                                </div>
                            )}

                            <button
                                onClick={handleUpload}
                                disabled={!file || uploading}
                                className="w-full bg-blue-600 hover:bg-blue-700 text-white font-medium py-2.5 rounded-lg transition disabled:opacity-50 disabled:cursor-not-allowed"
                            >
                                {uploading ? `Uploading... ${progress}%` : "Upload & Import"}
                            </button>
                        </>
                    )}

                    {/* Report */}
                    {report && (
                        <div className="space-y-4">
                            {/* Summary Cards */}
                            <div className="grid grid-cols-4 gap-3">
                                <div className="bg-emerald-50 rounded-lg p-3 text-center">
                                    <p className="text-xs text-emerald-600">Inserted</p>
                                    <p className="text-xl font-bold text-emerald-700">{report.inserted}</p>
                                </div>
                                <div className="bg-blue-50 rounded-lg p-3 text-center">
                                    <p className="text-xs text-blue-600">Updated</p>
                                    <p className="text-xl font-bold text-blue-700">{report.updated}</p>
                                </div>
                                <div className="bg-yellow-50 rounded-lg p-3 text-center">
                                    <p className="text-xs text-yellow-600">Skipped</p>
                                    <p className="text-xl font-bold text-yellow-700">{report.skipped}</p>
                                </div>
                                <div className="bg-red-50 rounded-lg p-3 text-center">
                                    <p className="text-xs text-red-600">Failed</p>
                                    <p className="text-xl font-bold text-red-700">{report.failed}</p>
                                </div>
                            </div>

                            {/* Errors Table */}
                            {report.errors.length > 0 && (
                                <div>
                                    <h3 className="text-sm font-semibold text-gray-600 mb-2">Errors ({report.errors.length})</h3>
                                    <div className="max-h-48 overflow-auto border rounded-lg">
                                        <table className="w-full text-xs">
                                            <thead className="bg-gray-50 sticky top-0">
                                                <tr>
                                                    <th className="px-3 py-2 text-left">Row</th>
                                                    <th className="px-3 py-2 text-left">Field</th>
                                                    <th className="px-3 py-2 text-left">Message</th>
                                                </tr>
                                            </thead>
                                            <tbody>
                                                {report.errors.map((e, i) => (
                                                    <tr key={i} className="border-t">
                                                        <td className="px-3 py-1.5 font-mono">{e.row}</td>
                                                        <td className="px-3 py-1.5 font-medium text-red-600">{e.field || "—"}</td>
                                                        <td className="px-3 py-1.5 text-gray-600">{e.message}</td>
                                                    </tr>
                                                ))}
                                            </tbody>
                                        </table>
                                    </div>
                                </div>
                            )}

                            <button onClick={handleClose}
                                className="w-full bg-gray-100 hover:bg-gray-200 text-gray-700 font-medium py-2.5 rounded-lg transition">
                                Close
                            </button>
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}
