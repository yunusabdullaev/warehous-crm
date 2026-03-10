"use client";
import { useEffect, useState, useCallback } from "react";
import api from "@/lib/api";
import { getRole, isSuperAdmin } from "@/lib/auth";
import { toast } from "@/components/Toast";
import type { BillingStatus } from "@/lib/types";

const PLAN_BADGE: Record<string, string> = {
    FREE: "bg-gray-100 text-gray-700 border-gray-200",
    PRO: "bg-purple-100 text-purple-700 border-purple-200",
    ENTERPRISE: "bg-amber-100 text-amber-700 border-amber-200",
};

const BILLING_BADGE: Record<string, string> = {
    ACTIVE: "bg-green-100 text-green-700",
    TRIALING: "bg-blue-100 text-blue-700",
    PAST_DUE: "bg-red-100 text-red-700",
    CANCELED: "bg-gray-100 text-gray-500",
    INCOMPLETE: "bg-yellow-100 text-yellow-700",
};

export default function BillingPage() {
    const [billing, setBilling] = useState<BillingStatus | null>(null);
    const [loading, setLoading] = useState(true);
    const [actionLoading, setActionLoading] = useState<string | null>(null);

    const role = getRole();
    const isAdmin = role === "admin" || isSuperAdmin();

    const fetchBilling = useCallback(async () => {
        try {
            setLoading(true);
            const res = await api.get<BillingStatus>("/billing/status");
            setBilling(res.data);
        } catch {
            toast("error", "Failed to load billing status");
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        if (!isAdmin) return;
        fetchBilling();
    }, [fetchBilling, isAdmin]);

    const handleCheckout = async (plan: string) => {
        setActionLoading(plan);
        try {
            const res = await api.post<{ url: string }>("/billing/checkout-session", { plan });
            window.location.href = res.data.url;
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || "Failed to create checkout session";
            toast("error", msg);
        } finally {
            setActionLoading(null);
        }
    };

    const handlePortal = async () => {
        setActionLoading("portal");
        try {
            const res = await api.post<{ url: string }>("/billing/portal-session", {});
            window.location.href = res.data.url;
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || "Failed to open billing portal";
            toast("error", msg);
        } finally {
            setActionLoading(null);
        }
    };

    if (!isAdmin) {
        return (
            <div className="flex justify-center items-center py-20">
                <p className="text-gray-400">🔒 Admin access required</p>
            </div>
        );
    }

    if (loading) {
        return (
            <div className="flex justify-center py-20">
                <div className="animate-spin w-8 h-8 border-4 border-purple-500 border-t-transparent rounded-full" />
            </div>
        );
    }

    if (!billing) {
        return (
            <div className="text-center py-20 text-gray-400">
                <p>Failed to load billing information</p>
            </div>
        );
    }

    const isSuspended = billing.status === "SUSPENDED";
    const isPastDue = billing.billing_status === "PAST_DUE";
    const isFree = billing.plan === "FREE";
    const isPro = billing.plan === "PRO";

    return (
        <div className="max-w-3xl mx-auto">
            <h1 className="text-2xl font-bold mb-6">Billing & Subscription</h1>

            {/* Suspension banner */}
            {(isSuspended || isPastDue) && (
                <div className="bg-red-50 border border-red-200 rounded-xl p-4 mb-6">
                    <div className="flex items-start gap-3">
                        <span className="text-2xl">⚠️</span>
                        <div className="flex-1">
                            <h3 className="font-semibold text-red-800">Account Suspended</h3>
                            <p className="text-sm text-red-600 mt-1">
                                Your account has been suspended due to a payment issue. Please update your payment method to restore access.
                            </p>
                            <button
                                onClick={handlePortal}
                                disabled={actionLoading === "portal"}
                                className="mt-3 bg-red-600 hover:bg-red-700 text-white px-4 py-2 rounded-lg text-sm font-medium transition-colors disabled:opacity-50"
                            >
                                {actionLoading === "portal" ? "Opening..." : "Update Payment Method"}
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* Current plan card */}
            <div className="bg-white rounded-xl shadow-sm border p-6 mb-6">
                <div className="flex items-center justify-between mb-4">
                    <h2 className="text-lg font-semibold">Current Plan</h2>
                    <span className={`px-3 py-1 rounded-full text-sm font-semibold border ${PLAN_BADGE[billing.plan] || PLAN_BADGE.FREE}`}>
                        {billing.plan}
                    </span>
                </div>

                <div className="grid grid-cols-2 gap-4 text-sm">
                    <div>
                        <span className="text-gray-500">Billing Status</span>
                        <div className="mt-1">
                            <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${BILLING_BADGE[billing.billing_status] || "bg-gray-100 text-gray-500"}`}>
                                {billing.billing_status || "—"}
                            </span>
                        </div>
                    </div>
                    <div>
                        <span className="text-gray-500">Period Ends</span>
                        <p className="mt-1 font-medium">
                            {billing.current_period_end
                                ? new Date(billing.current_period_end).toLocaleDateString("en-US", {
                                    year: "numeric", month: "long", day: "numeric",
                                })
                                : "—"}
                        </p>
                    </div>
                    {billing.cancel_at_period_end && (
                        <div className="col-span-2">
                            <span className="inline-flex items-center gap-1 text-amber-600 text-xs bg-amber-50 px-2 py-1 rounded-lg">
                                ⏳ Cancels at period end
                            </span>
                        </div>
                    )}
                </div>

                {/* Manage subscription button */}
                {billing.stripe_customer_id && (
                    <div className="mt-4 pt-4 border-t">
                        <button
                            onClick={handlePortal}
                            disabled={actionLoading === "portal"}
                            className="text-purple-600 hover:text-purple-700 text-sm font-medium transition-colors disabled:opacity-50"
                        >
                            {actionLoading === "portal" ? "Opening..." : "Manage Subscription →"}
                        </button>
                    </div>
                )}
            </div>

            {/* Upgrade options */}
            <h2 className="text-lg font-semibold mb-4">Available Plans</h2>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                {/* FREE */}
                <PlanCard
                    name="FREE"
                    price="$0"
                    period="/mo"
                    features={["1 Warehouse", "5 Users", "500 Products", "50 Orders/day"]}
                    current={isFree}
                    onSelect={() => { }}
                    disabled={true}
                />
                {/* PRO */}
                <PlanCard
                    name="PRO"
                    price="$49"
                    period="/mo"
                    features={["5 Warehouses", "25 Users", "5,000 Products", "500 Orders/day", "Reports, Returns, Lots", "QR Labels, Multi-WH"]}
                    current={isPro}
                    highlighted
                    loading={actionLoading === "PRO"}
                    onSelect={() => handleCheckout("PRO")}
                    disabled={isPro || actionLoading !== null}
                />
                {/* ENTERPRISE */}
                <PlanCard
                    name="ENTERPRISE"
                    price="$199"
                    period="/mo"
                    features={["Unlimited Warehouses", "Unlimited Users", "Unlimited Products", "Unlimited Orders", "All Features", "API Export"]}
                    current={billing.plan === "ENTERPRISE"}
                    loading={actionLoading === "ENTERPRISE"}
                    onSelect={() => handleCheckout("ENTERPRISE")}
                    disabled={billing.plan === "ENTERPRISE" || actionLoading !== null}
                />
            </div>
        </div>
    );
}

/* ── Plan Card Component ── */
function PlanCard({
    name, price, period, features, current, highlighted, loading, onSelect, disabled,
}: {
    name: string; price: string; period: string; features: string[];
    current?: boolean; highlighted?: boolean; loading?: boolean;
    onSelect: () => void; disabled?: boolean;
}) {
    return (
        <div className={`rounded-xl border p-5 flex flex-col ${highlighted ? "border-purple-300 shadow-md ring-2 ring-purple-100" : "border-gray-200"} ${current ? "bg-purple-50/50" : "bg-white"}`}>
            <h3 className="font-bold text-lg">{name}</h3>
            <div className="mt-2 mb-4">
                <span className="text-3xl font-bold">{price}</span>
                <span className="text-gray-500 text-sm">{period}</span>
            </div>
            <ul className="flex-1 space-y-2 mb-4">
                {features.map(f => (
                    <li key={f} className="flex items-center gap-2 text-sm text-gray-600">
                        <span className="text-green-500">✓</span> {f}
                    </li>
                ))}
            </ul>
            {current ? (
                <div className="text-center text-sm text-purple-600 font-medium py-2 bg-purple-50 rounded-lg">
                    Current Plan
                </div>
            ) : (
                <button
                    onClick={onSelect}
                    disabled={disabled}
                    className={`w-full py-2 rounded-lg text-sm font-medium transition-colors disabled:opacity-50 ${highlighted
                            ? "bg-purple-600 hover:bg-purple-700 text-white"
                            : "bg-gray-100 hover:bg-gray-200 text-gray-700"
                        }`}
                >
                    {loading ? "Redirecting..." : `Upgrade to ${name}`}
                </button>
            )}
        </div>
    );
}
