"use client";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { NAV_ITEMS, can, getUser, logout } from "@/lib/auth";
import TenantSelector from "./TenantSelector";
import WarehouseSelector from "./WarehouseSelector";

export default function Sidebar() {
    const pathname = usePathname();
    const user = getUser();

    const visibleItems = NAV_ITEMS.filter(
        (item) => !item.requiredAction || can(item.requiredAction)
    );

    return (
        <aside className="w-60 bg-gray-900 text-gray-100 flex flex-col min-h-screen">
            {/* Brand */}
            <div className="px-5 py-5 border-b border-gray-800">
                <h1 className="text-lg font-bold tracking-tight">🏭 Warehouse CRM</h1>
                <p className="text-xs text-gray-500 mt-0.5">Admin Panel</p>
            </div>

            {/* Tenant Selector (superadmin only) */}
            <TenantSelector />

            {/* Warehouse Selector */}
            <WarehouseSelector />

            {/* Nav */}
            <nav className="flex-1 py-4 px-3 space-y-1">
                {visibleItems.map((item) => {
                    const active = pathname === item.href;
                    return (
                        <Link
                            key={item.href}
                            href={item.href}
                            className={`flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors
                ${active ? "bg-blue-600 text-white" : "text-gray-400 hover:bg-gray-800 hover:text-white"}`}
                        >
                            <span className="text-base">{item.icon}</span>
                            {item.label}
                        </Link>
                    );
                })}
            </nav>

            {/* User info + logout */}
            {user && (
                <div className="border-t border-gray-800 px-4 py-4">
                    <div className="flex items-center justify-between">
                        <div>
                            <p className="text-sm font-medium">{user.username}</p>
                            <p className="text-xs text-gray-500 capitalize">{user.role}</p>
                        </div>
                        <button
                            onClick={logout}
                            className="text-xs text-gray-500 hover:text-red-400 transition-colors"
                        >
                            Logout
                        </button>
                    </div>
                </div>
            )}
        </aside>
    );
}
