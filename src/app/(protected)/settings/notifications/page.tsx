"use client";
import { useEffect, useState } from "react";
import api from "@/lib/api";
import { toast } from "@/components/Toast";

interface NotifySettings {
    telegram_enabled: boolean;
    telegram_bot_token: string;
    telegram_chat_ids: string;
    expiry_digest_enabled: boolean;
    expiry_digest_days: number;
    expiry_digest_time: string;
    expiry_digest_chat_ids: string;
    updated_at: string;
}

export default function NotificationSettingsPage() {
    const [settings, setSettings] = useState<NotifySettings>({
        telegram_enabled: false,
        telegram_bot_token: "",
        telegram_chat_ids: "",
        expiry_digest_enabled: false,
        expiry_digest_days: 14,
        expiry_digest_time: "08:30",
        expiry_digest_chat_ids: "",
        updated_at: "",
    });
    const [loading, setLoading] = useState(true);
    const [saving, setSaving] = useState(false);
    const [testing, setTesting] = useState(false);
    const [sendingDigest, setSendingDigest] = useState(false);

    const fetchSettings = async () => {
        setLoading(true);
        try {
            const res = await api.get<NotifySettings>("/settings/notifications");
            setSettings(res.data);
        } catch { /* interceptor */ }
        setLoading(false);
    };

    useEffect(() => { fetchSettings(); }, []);

    const onSave = async () => {
        setSaving(true);
        try {
            await api.put("/settings/notifications", settings);
            toast("success", "Settings saved");
            fetchSettings();
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Failed to save");
        }
        setSaving(false);
    };

    const onTest = async () => {
        setTesting(true);
        try {
            await api.post("/settings/notifications/test");
            toast("success", "Test message queued");
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Failed to send test");
        }
        setTesting(false);
    };

    const onSendDigest = async () => {
        setSendingDigest(true);
        try {
            const res = await api.post<{
                sent: boolean; skipped: boolean; reason?: string;
                total: number; urgent: number; warning: number; notice: number;
            }>("/alerts/expiry-digest/run?force=true");
            const d = res.data;
            if (d.skipped) {
                toast("info", `Digest skipped: ${d.reason}`);
            } else {
                toast("success", `Digest sent: ${d.total} lots (${d.urgent} urgent, ${d.warning} warning, ${d.notice} notice)`);
            }
        } catch (err: unknown) {
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
            toast("error", msg || "Failed to send digest");
        }
        setSendingDigest(false);
    };

    if (loading) {
        return (
            <div className="flex justify-center py-20">
                <div className="animate-spin w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full" />
            </div>
        );
    }

    return (
        <div>
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-bold">Notification Settings</h1>
            </div>

            <div className="bg-white rounded-xl shadow-sm border p-6 max-w-2xl space-y-8">
                {/* ── Telegram Section ── */}
                <div>
                    <div className="flex items-center justify-between mb-4 pb-3 border-b">
                        <div>
                            <h2 className="font-semibold text-lg">Telegram Notifications</h2>
                            <p className="text-sm text-gray-500 mt-1">
                                Send alerts for order events and stock warnings
                            </p>
                        </div>
                        <button
                            onClick={() => setSettings({ ...settings, telegram_enabled: !settings.telegram_enabled })}
                            className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${settings.telegram_enabled ? "bg-blue-600" : "bg-gray-300"
                                }`}
                        >
                            <span
                                className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${settings.telegram_enabled ? "translate-x-6" : "translate-x-1"
                                    }`}
                            />
                        </button>
                    </div>

                    {/* Bot Token */}
                    <div className="mb-4">
                        <label className="block text-sm font-medium mb-1">Bot Token</label>
                        <input
                            type="password"
                            value={settings.telegram_bot_token}
                            onChange={(e) => setSettings({ ...settings, telegram_bot_token: e.target.value })}
                            className="w-full px-3 py-2 border rounded-lg text-sm"
                            placeholder="123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
                        />
                        <p className="text-xs text-gray-400 mt-1">
                            Get from <a href="https://t.me/BotFather" target="_blank" rel="noopener" className="text-blue-500 hover:underline">@BotFather</a>
                        </p>
                    </div>

                    {/* Chat IDs */}
                    <div className="mb-4">
                        <label className="block text-sm font-medium mb-1">Chat IDs</label>
                        <input
                            type="text"
                            value={settings.telegram_chat_ids}
                            onChange={(e) => setSettings({ ...settings, telegram_chat_ids: e.target.value })}
                            className="w-full px-3 py-2 border rounded-lg text-sm"
                            placeholder="-1001234567890, -1009876543210"
                        />
                        <p className="text-xs text-gray-400 mt-1">
                            Comma-separated. Use <a href="https://t.me/userinfobot" target="_blank" rel="noopener" className="text-blue-500 hover:underline">@userinfobot</a> to find chat IDs
                        </p>
                    </div>

                    {/* Actions */}
                    <div className="flex items-center gap-3">
                        <button
                            onClick={onSave}
                            disabled={saving}
                            className="px-6 py-2 bg-emerald-600 text-white rounded-lg text-sm hover:bg-emerald-700 transition disabled:opacity-50"
                        >
                            {saving ? "Saving..." : "Save Settings"}
                        </button>
                        <button
                            onClick={onTest}
                            disabled={testing || !settings.telegram_enabled}
                            className="px-6 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 transition disabled:opacity-50"
                        >
                            {testing ? "Sending..." : "🧪 Send Test"}
                        </button>
                    </div>

                    {/* Last Updated */}
                    {settings.updated_at && (
                        <p className="text-xs text-gray-400 mt-3">
                            Last updated: {new Date(settings.updated_at).toLocaleString()}
                        </p>
                    )}
                </div>

                {/* ── Expiry Digest Section ── */}
                <div className="pt-2">
                    <div className="flex items-center justify-between mb-4 pb-3 border-b">
                        <div>
                            <h2 className="font-semibold text-lg">📋 Expiry Digest</h2>
                            <p className="text-sm text-gray-500 mt-1">
                                Daily Telegram summary of lots nearing expiration
                            </p>
                        </div>
                        <button
                            onClick={() => setSettings({ ...settings, expiry_digest_enabled: !settings.expiry_digest_enabled })}
                            className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${settings.expiry_digest_enabled ? "bg-blue-600" : "bg-gray-300"
                                }`}
                        >
                            <span
                                className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${settings.expiry_digest_enabled ? "translate-x-6" : "translate-x-1"
                                    }`}
                            />
                        </button>
                    </div>

                    <div className="grid grid-cols-2 gap-4 mb-4">
                        <div>
                            <label className="block text-sm font-medium mb-1">Lookahead Days</label>
                            <input
                                type="number"
                                min={1}
                                max={90}
                                value={settings.expiry_digest_days}
                                onChange={(e) => setSettings({ ...settings, expiry_digest_days: Number(e.target.value) })}
                                className="w-full px-3 py-2 border rounded-lg text-sm"
                            />
                            <p className="text-xs text-gray-400 mt-1">Include lots expiring within this many days</p>
                        </div>
                        <div>
                            <label className="block text-sm font-medium mb-1">Digest Time</label>
                            <input
                                type="time"
                                value={settings.expiry_digest_time}
                                onChange={(e) => setSettings({ ...settings, expiry_digest_time: e.target.value })}
                                className="w-full px-3 py-2 border rounded-lg text-sm"
                            />
                            <p className="text-xs text-gray-400 mt-1">Server local time (for systemd timer)</p>
                        </div>
                    </div>

                    <div className="mb-4">
                        <label className="block text-sm font-medium mb-1">Override Chat IDs <span className="text-gray-400">(optional)</span></label>
                        <input
                            type="text"
                            value={settings.expiry_digest_chat_ids}
                            onChange={(e) => setSettings({ ...settings, expiry_digest_chat_ids: e.target.value })}
                            className="w-full px-3 py-2 border rounded-lg text-sm"
                            placeholder="Leave empty to use global chat IDs above"
                        />
                    </div>

                    <button
                        onClick={onSendDigest}
                        disabled={sendingDigest || !settings.expiry_digest_enabled}
                        className="px-6 py-2 bg-amber-600 text-white rounded-lg text-sm hover:bg-amber-700 transition disabled:opacity-50"
                    >
                        {sendingDigest ? "Sending..." : "📋 Send Expiry Digest Now"}
                    </button>
                </div>

                {/* ── Alert Events ── */}
                <div className="pt-2 border-t">
                    <h3 className="font-medium text-sm mb-3">Events that trigger notifications:</h3>
                    <div className="space-y-2 text-sm text-gray-600">
                        <div className="flex items-center gap-2">
                            <span className="text-lg">📋</span>
                            <span>Order confirmed</span>
                        </div>
                        <div className="flex items-center gap-2">
                            <span className="text-lg">🎯</span>
                            <span>Picking started</span>
                        </div>
                        <div className="flex items-center gap-2">
                            <span className="text-lg">✅</span>
                            <span>Pick task completed</span>
                        </div>
                        <div className="flex items-center gap-2">
                            <span className="text-lg">🚚</span>
                            <span>Order shipped</span>
                        </div>
                        <div className="flex items-center gap-2">
                            <span className="text-lg">⚠️</span>
                            <span>Low stock warning (per-product threshold, 6h cooldown)</span>
                        </div>
                        <div className="flex items-center gap-2">
                            <span className="text-lg">📦</span>
                            <span>Return created / received</span>
                        </div>
                        <div className="flex items-center gap-2">
                            <span className="text-lg">📋</span>
                            <span>Daily expiry digest (configurable)</span>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
