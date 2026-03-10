"use client";
import { useEffect, useState, useCallback } from "react";
import api from "@/lib/api";
import { isSuperAdmin, getTenantId } from "@/lib/auth";
import type { Tenant } from "@/lib/types";

export default function TenantSelector() {
    const [tenants, setTenants] = useState<Tenant[]>([]);
    const [activeId, setActiveId] = useState<string | null>(null);

    const fetchTenants = useCallback(async () => {
        try {
            const res = await api.get("/tenants?limit=100");
            const list: Tenant[] = res.data?.data ?? res.data ?? [];
            setTenants(list);

            // Auto-select: use stored tenant or first in list
            const stored = getTenantId();
            if (stored && list.find((t) => t.id === stored)) {
                setActiveId(stored);
            } else if (list.length > 0) {
                setActiveId(list[0].id);
                localStorage.setItem("wms_tenant_id", list[0].id);
            }
        } catch {
            // silently fail — user may not have superadmin access
        }
    }, []);

    useEffect(() => {
        if (isSuperAdmin()) {
            fetchTenants();
        }
    }, [fetchTenants]);

    const handleChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
        const id = e.target.value;
        setActiveId(id);
        localStorage.setItem("wms_tenant_id", id);
        // Dispatch event so other components can react
        window.dispatchEvent(
            new CustomEvent("wms-tenant-changed", { detail: { id } })
        );
        // Reload to refresh data with tenant context
        window.location.reload();
    };

    if (!isSuperAdmin() || tenants.length === 0) return null;

    return (
        <div className="px-4 py-3 border-b border-gray-800">
            <label className="text-xs text-gray-500 block mb-1">
                🏪 Tenant
            </label>
            <select
                value={activeId || ""}
                onChange={handleChange}
                className="w-full bg-gray-800 text-gray-200 text-sm rounded-md px-2 py-1.5 border border-gray-700 focus:outline-none focus:border-purple-500"
            >
                {tenants.map((t) => (
                    <option key={t.id} value={t.id}>
                        {t.code} — {t.name}
                    </option>
                ))}
            </select>
        </div>
    );
}
