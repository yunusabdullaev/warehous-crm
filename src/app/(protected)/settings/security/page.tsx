"use client";
import { useEffect, useState, useCallback } from "react";
import api from "@/lib/api";
import { getRole, isSuperAdmin } from "@/lib/auth";
import { toast } from "@/components/Toast";

interface SessionItem {
    id: string;
    ip?: string;
    user_agent?: string;
    created_at: string;
    last_used_at: string;
    current?: boolean;
}

interface TwoFASetup {
    secret: string;
    uri: string;
    qr: string; // base64 PNG
}

export default function SecurityPage() {
    const [sessions, setSessions] = useState<SessionItem[]>([]);
    const [loading, setLoading] = useState(true);
    const [revoking, setRevoking] = useState<string | null>(null);

    // 2FA state
    const [has2FA, setHas2FA] = useState(false);
    const [setup, setSetup] = useState<TwoFASetup | null>(null);
    const [verifyCode, setVerifyCode] = useState("");
    const [setting2FA, setSetting2FA] = useState(false);

    // Password change
    const [oldPw, setOldPw] = useState("");
    const [newPw, setNewPw] = useState("");
    const [changingPw, setChangingPw] = useState(false);

    const role = getRole();
    const isAdmin = role === "admin" || isSuperAdmin();

    const fetchSessions = useCallback(async () => {
        try {
            const res = await api.get<{ sessions: SessionItem[] }>("/auth/sessions");
            setSessions(res.data.sessions || []);
        } catch {
            toast("error", "Failed to load sessions");
        } finally {
            setLoading(false);
        }
    }, []);

    const fetchProfile = useCallback(async () => {
        try {
            // Check if user has 2FA from stored data
            const userData = localStorage.getItem("wms_user");
            if (userData) {
                const user = JSON.parse(userData);
                setHas2FA(!!user.two_factor_enabled);
            }
        } catch { /* ignore */ }
    }, []);

    useEffect(() => {
        fetchSessions();
        fetchProfile();
    }, [fetchSessions, fetchProfile]);

    const revokeSession = async (id: string) => {
        setRevoking(id);
        try {
            await api.delete(`/auth/sessions/${id}`);
            setSessions((prev) => prev.filter((s) => s.id !== id));
            toast("success", "Session revoked");
        } catch {
            toast("error", "Failed to revoke session");
        } finally {
            setRevoking(null);
        }
    };

    const revokeAll = async () => {
        try {
            await api.post("/auth/logout-all", {});
            toast("success", "All sessions revoked — you will be logged out");
            setTimeout(() => {
                localStorage.removeItem("wms_token");
                localStorage.removeItem("wms_user");
                window.location.href = "/login";
            }, 1500);
        } catch {
            toast("error", "Failed to revoke all sessions");
        }
    };

    // ── 2FA ──
    const setup2FA = async () => {
        setSetting2FA(true);
        try {
            const res = await api.post<TwoFASetup>("/auth/2fa/setup", {});
            setSetup(res.data);
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Failed to setup 2FA");
        } finally {
            setSetting2FA(false);
        }
    };

    const verify2FA = async () => {
        if (verifyCode.length !== 6) return;
        setSetting2FA(true);
        try {
            await api.post("/auth/2fa/verify", { code: verifyCode });
            setHas2FA(true);
            setSetup(null);
            setVerifyCode("");
            toast("success", "2FA enabled successfully");
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Invalid code");
        } finally {
            setSetting2FA(false);
        }
    };

    const disable2FA = async () => {
        if (!confirm("Are you sure you want to disable 2FA?")) return;
        try {
            await api.delete("/auth/2fa");
            setHas2FA(false);
            toast("success", "2FA disabled");
        } catch {
            toast("error", "Failed to disable 2FA");
        }
    };

    // ── Password change ──
    const changePassword = async () => {
        if (!newPw || newPw.length < 10) {
            toast("error", "Password must be at least 10 characters");
            return;
        }
        setChangingPw(true);
        try {
            // Get current user ID from localStorage
            const userData = localStorage.getItem("wms_user");
            if (!userData) throw new Error("not logged in");
            const user = JSON.parse(userData);
            await api.put(`/users/${user.id}`, { password: newPw });
            setOldPw("");
            setNewPw("");
            toast("success", "Password changed");
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Failed to change password");
        } finally {
            setChangingPw(false);
        }
    };

    const parseUA = (ua?: string) => {
        if (!ua) return "Unknown device";
        if (ua.includes("Chrome")) return "Chrome";
        if (ua.includes("Firefox")) return "Firefox";
        if (ua.includes("Safari")) return "Safari";
        if (ua.includes("curl")) return "curl";
        return ua.substring(0, 40);
    };

    return (
        <div className="max-w-3xl mx-auto">
            <h1 className="text-2xl font-bold mb-6">Security Settings</h1>

            {/* ── Active Sessions ── */}
            <div className="bg-white rounded-xl shadow-sm border p-6 mb-6">
                <div className="flex items-center justify-between mb-4">
                    <h2 className="text-lg font-semibold">Active Sessions</h2>
                    <button
                        onClick={revokeAll}
                        className="text-red-600 hover:text-red-700 text-sm font-medium"
                    >
                        Revoke All
                    </button>
                </div>

                {loading ? (
                    <div className="flex justify-center py-8">
                        <div className="animate-spin w-6 h-6 border-3 border-purple-500 border-t-transparent rounded-full" />
                    </div>
                ) : sessions.length === 0 ? (
                    <p className="text-gray-400 text-sm">No active sessions</p>
                ) : (
                    <div className="space-y-3">
                        {sessions.map((s) => (
                            <div key={s.id} className="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
                                <div>
                                    <div className="flex items-center gap-2">
                                        <span className="text-sm font-medium">{parseUA(s.user_agent)}</span>
                                        {s.current && (
                                            <span className="text-xs bg-green-100 text-green-700 px-1.5 py-0.5 rounded">Current</span>
                                        )}
                                    </div>
                                    <div className="text-xs text-gray-400 mt-0.5">
                                        {s.ip && <span>IP: {s.ip} · </span>}
                                        Last used: {new Date(s.last_used_at).toLocaleString()}
                                    </div>
                                </div>
                                <button
                                    onClick={() => revokeSession(s.id)}
                                    disabled={revoking === s.id}
                                    className="text-red-500 hover:text-red-600 text-xs font-medium disabled:opacity-50"
                                >
                                    {revoking === s.id ? "..." : "Revoke"}
                                </button>
                            </div>
                        ))}
                    </div>
                )}
            </div>

            {/* ── Two-Factor Authentication ── */}
            <div className="bg-white rounded-xl shadow-sm border p-6 mb-6">
                <h2 className="text-lg font-semibold mb-4">Two-Factor Authentication</h2>

                {has2FA ? (
                    <div className="flex items-center justify-between">
                        <div className="flex items-center gap-3">
                            <span className="text-2xl">🔐</span>
                            <div>
                                <p className="font-medium text-green-700">2FA Enabled</p>
                                <p className="text-xs text-gray-500">Your account is protected with TOTP</p>
                            </div>
                        </div>
                        <button
                            onClick={disable2FA}
                            className="text-red-600 hover:text-red-700 text-sm font-medium"
                        >
                            Disable
                        </button>
                    </div>
                ) : setup ? (
                    <div>
                        <p className="text-sm text-gray-600 mb-4">
                            Scan this QR code with your authenticator app (Google Authenticator, Authy, etc.)
                        </p>
                        <div className="flex justify-center mb-4">
                            <img
                                src={`data:image/png;base64,${setup.qr}`}
                                alt="2FA QR Code"
                                className="w-48 h-48 border rounded-lg"
                            />
                        </div>
                        <p className="text-xs text-gray-400 text-center mb-4 font-mono break-all">
                            Secret: {setup.secret}
                        </p>
                        <div className="flex gap-2">
                            <input
                                type="text"
                                inputMode="numeric"
                                maxLength={6}
                                value={verifyCode}
                                onChange={(e) => setVerifyCode(e.target.value.replace(/\D/g, ""))}
                                className="flex-1 px-4 py-2 border rounded-lg text-center font-mono text-lg tracking-widest"
                                placeholder="000000"
                            />
                            <button
                                onClick={verify2FA}
                                disabled={setting2FA || verifyCode.length !== 6}
                                className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg text-sm font-medium disabled:opacity-50"
                            >
                                {setting2FA ? "..." : "Verify"}
                            </button>
                        </div>
                    </div>
                ) : (
                    <div className="flex items-center justify-between">
                        <div>
                            <p className="text-sm text-gray-600">Add an extra layer of security</p>
                            <p className="text-xs text-gray-400 mt-1">Requires ENTERPRISE plan</p>
                        </div>
                        <button
                            onClick={setup2FA}
                            disabled={setting2FA}
                            className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg text-sm font-medium disabled:opacity-50"
                        >
                            {setting2FA ? "Setting up..." : "Setup 2FA"}
                        </button>
                    </div>
                )}
            </div>

            {/* ── Change Password ── */}
            <div className="bg-white rounded-xl shadow-sm border p-6">
                <h2 className="text-lg font-semibold mb-4">Change Password</h2>
                <div className="space-y-3">
                    <input
                        type="password"
                        value={newPw}
                        onChange={(e) => setNewPw(e.target.value)}
                        className="w-full px-4 py-2 border rounded-lg text-sm"
                        placeholder="New password (min 10 chars, 3 of: upper/lower/digit/symbol)"
                    />
                    <button
                        onClick={changePassword}
                        disabled={changingPw || !newPw}
                        className="bg-gray-800 hover:bg-gray-900 text-white px-4 py-2 rounded-lg text-sm font-medium disabled:opacity-50"
                    >
                        {changingPw ? "Changing..." : "Change Password"}
                    </button>
                </div>
            </div>
        </div>
    );
}
