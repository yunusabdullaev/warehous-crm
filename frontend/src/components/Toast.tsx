"use client";
import { useEffect, useState, useCallback } from "react";

interface Toast {
    id: number;
    type: "success" | "error" | "info";
    message: string;
}

let _id = 0;

export function toast(type: Toast["type"], message: string) {
    window.dispatchEvent(new CustomEvent("wms-toast", { detail: { type, message } }));
}

export default function ToastContainer() {
    const [toasts, setToasts] = useState<Toast[]>([]);

    const addToast = useCallback((e: Event) => {
        const { type, message } = (e as CustomEvent).detail;
        const id = ++_id;
        setToasts((prev) => [...prev, { id, type, message }]);
        setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== id)), 4000);
    }, []);

    useEffect(() => {
        window.addEventListener("wms-toast", addToast);
        return () => window.removeEventListener("wms-toast", addToast);
    }, [addToast]);

    if (toasts.length === 0) return null;

    return (
        <div className="fixed top-4 right-4 z-50 flex flex-col gap-2 max-w-sm">
            {toasts.map((t) => (
                <div
                    key={t.id}
                    className={`px-4 py-3 rounded-lg shadow-lg text-sm font-medium text-white animate-slide-in
            ${t.type === "success" ? "bg-emerald-600" : t.type === "error" ? "bg-red-600" : "bg-blue-600"}`}
                >
                    {t.message}
                </div>
            ))}
        </div>
    );
}
