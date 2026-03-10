"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { zodResolver } from "@hookform/resolvers/zod";
import api from "@/lib/api";
import { setAuth } from "@/lib/auth";
import type { AuthResponse } from "@/lib/types";

const loginSchema = z.object({
    username: z.string().min(1, "Username is required"),
    password: z.string().min(1, "Password is required"),
});

const totpSchema = z.object({
    code: z.string().min(6, "Enter 6-digit code").max(6),
});

type LoginData = z.infer<typeof loginSchema>;
type TotpData = z.infer<typeof totpSchema>;

export default function LoginPage() {
    const router = useRouter();
    const [error, setError] = useState("");
    const [loading, setLoading] = useState(false);
    const [mode, setMode] = useState<"login" | "2fa">("login");
    const [tempToken, setTempToken] = useState("");

    const loginForm = useForm<LoginData>({
        resolver: zodResolver(loginSchema),
    });

    const totpForm = useForm<TotpData>({
        resolver: zodResolver(totpSchema),
    });

    const onLogin = async (data: LoginData) => {
        setLoading(true);
        setError("");
        try {
            const res = await api.post<AuthResponse>("/auth/login", data);

            if (res.data.requires_2fa && res.data.temp_token) {
                setTempToken(res.data.temp_token);
                setMode("2fa");
                setLoading(false);
                return;
            }

            if (!res.data.token) {
                throw new Error("Invalid response from server (no token)");
            }

            setAuth(res.data.token, res.data.user?.default_warehouse_id);
            router.replace("/dashboard");
        } catch (err: unknown) {
            console.error("Login error:", err);
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error
                || (err as Error)?.message
                || "Login failed. Check your credentials.";
            setError(msg);
        } finally {
            setLoading(false);
        }
    };

    const on2FA = async (data: TotpData) => {
        setLoading(true);
        setError("");
        try {
            const res = await api.post<AuthResponse>("/auth/login-2fa", {
                temp_token: tempToken,
                code: data.code,
            });

            if (!res.data.token) {
                throw new Error("Invalid response from server (no token)");
            }

            setAuth(res.data.token, res.data.user?.default_warehouse_id);
            router.replace("/dashboard");
        } catch (err: unknown) {
            console.error("2FA error:", err);
            const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error
                || (err as Error)?.message
                || "Invalid 2FA code.";
            setError(msg);
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-gray-900 via-gray-800 to-gray-900">
            <div className="w-full max-w-md">
                <div className="bg-white rounded-2xl shadow-2xl p-8">
                    {/* Header */}
                    <div className="text-center mb-6">
                        <div className="text-4xl mb-3">🏭</div>
                        <h1 className="text-2xl font-bold text-gray-900">Warehouse CRM</h1>
                        <p className="text-sm text-gray-500 mt-1">
                            {mode === "login" ? "Sign in to your account" : "Enter your 2FA code"}
                        </p>
                    </div>

                    {/* Error */}
                    {error && (
                        <div className="bg-red-50 border border-red-200 text-red-700 text-sm rounded-lg px-4 py-3 mb-6">
                            {error}
                        </div>
                    )}

                    {/* Login Form */}
                    {mode === "login" && (
                        <form onSubmit={loginForm.handleSubmit(onLogin)} className="space-y-5">
                            <div>
                                <label className="block text-sm font-medium text-gray-700 mb-1.5">Username</label>
                                <input
                                    {...loginForm.register("username")}
                                    type="text"
                                    autoFocus
                                    className="w-full px-4 py-2.5 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent outline-none transition"
                                    placeholder="Enter username"
                                />
                                {loginForm.formState.errors.username && <p className="text-red-500 text-xs mt-1">{loginForm.formState.errors.username.message}</p>}
                            </div>

                            <div>
                                <label className="block text-sm font-medium text-gray-700 mb-1.5">Password</label>
                                <input
                                    {...loginForm.register("password")}
                                    type="password"
                                    className="w-full px-4 py-2.5 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent outline-none transition"
                                    placeholder="Enter password"
                                />
                                {loginForm.formState.errors.password && <p className="text-red-500 text-xs mt-1">{loginForm.formState.errors.password.message}</p>}
                            </div>

                            <button
                                type="submit"
                                disabled={loading}
                                className="w-full bg-blue-600 hover:bg-blue-700 text-white font-medium py-2.5 rounded-lg transition disabled:opacity-50 disabled:cursor-not-allowed"
                            >
                                {loading ? "Signing in..." : "Sign In"}
                            </button>
                        </form>
                    )}

                    {/* 2FA Step */}
                    {mode === "2fa" && (
                        <form onSubmit={totpForm.handleSubmit(on2FA)} className="space-y-5">
                            <div className="text-center mb-2">
                                <div className="text-3xl mb-2">🔐</div>
                                <p className="text-sm text-gray-500">Enter the 6-digit code from your authenticator app</p>
                            </div>
                            <div>
                                <input
                                    {...totpForm.register("code")}
                                    type="text"
                                    inputMode="numeric"
                                    autoComplete="one-time-code"
                                    autoFocus
                                    maxLength={6}
                                    className="w-full px-4 py-3 border border-gray-300 rounded-lg text-center text-2xl tracking-[0.5em] font-mono focus:ring-2 focus:ring-blue-500 focus:border-transparent outline-none transition"
                                    placeholder="000000"
                                />
                                {totpForm.formState.errors.code && <p className="text-red-500 text-xs mt-1 text-center">{totpForm.formState.errors.code.message}</p>}
                            </div>

                            <button
                                type="submit"
                                disabled={loading}
                                className="w-full bg-blue-600 hover:bg-blue-700 text-white font-medium py-2.5 rounded-lg transition disabled:opacity-50 disabled:cursor-not-allowed"
                            >
                                {loading ? "Verifying..." : "Verify"}
                            </button>

                            <button
                                type="button"
                                onClick={() => { setMode("login"); setError(""); setTempToken(""); }}
                                className="w-full text-sm text-gray-500 hover:text-gray-700 py-1"
                            >
                                ← Back to login
                            </button>
                        </form>
                    )}
                </div>
            </div>
        </div>
    );
}
