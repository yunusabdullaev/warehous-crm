"use client";
import { useEffect, useState, useCallback } from "react";
import api from "@/lib/api";
import { getActiveWarehouseId, setActiveWarehouseId, getUser } from "@/lib/auth";

interface Warehouse {
    id: string;
    code: string;
    name: string;
    is_default: boolean;
}

export default function WarehouseSelector() {
    const [warehouses, setWarehouses] = useState<Warehouse[]>([]);
    const [activeId, setActiveId] = useState<string | null>(null);
    const user = getUser();

    const fetchWarehouses = useCallback(async () => {
        try {
            const res = await api.get("/warehouses");
            let list: Warehouse[] = res.data?.data ?? res.data ?? [];

            // Non-admin users: filter to only their allowed warehouses
            if (user && user.role !== "admin") {
                // Fetch current user details to get allowed list
                try {
                    const meRes = await api.get(`/users/${user.id}`);
                    const allowedIds: string[] = meRes.data?.allowed_warehouse_ids ?? [];
                    if (allowedIds.length > 0) {
                        list = list.filter(w => allowedIds.includes(w.id));
                    }
                } catch {
                    // Fall through — show all available warehouses
                }
            }

            setWarehouses(list);

            // Auto-select: if no active warehouse, pick the default or first
            const current = getActiveWarehouseId();
            if (!current && list.length > 0) {
                const def = list.find((w) => w.is_default) || list[0];
                setActiveWarehouseId(def.id);
                setActiveId(def.id);
            } else if (current && list.length > 0) {
                // Ensure current is still in the allowed list
                const valid = list.find(w => w.id === current);
                if (!valid) {
                    const def = list.find((w) => w.is_default) || list[0];
                    setActiveWarehouseId(def.id);
                    setActiveId(def.id);
                } else {
                    setActiveId(current);
                }
            } else {
                setActiveId(current);
            }
        } catch {
            // silently fail — user may not have access
        }
    }, [user]);

    useEffect(() => {
        fetchWarehouses();
    }, [fetchWarehouses]);

    const handleChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
        const id = e.target.value;
        setActiveId(id);
        setActiveWarehouseId(id);
        // Reload the current page to refresh data with new warehouse context
        window.location.reload();
    };

    if (!user || warehouses.length <= 1) return null;

    return (
        <div className="px-4 py-3 border-b border-gray-800">
            <label className="text-xs text-gray-500 block mb-1">Warehouse</label>
            <select
                value={activeId || ""}
                onChange={handleChange}
                className="w-full bg-gray-800 text-gray-200 text-sm rounded-md px-2 py-1.5 border border-gray-700 focus:outline-none focus:border-blue-500"
            >
                {warehouses.map((w) => (
                    <option key={w.id} value={w.id}>
                        {w.code} — {w.name}
                    </option>
                ))}
            </select>
        </div>
    );
}
