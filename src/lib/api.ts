import axios, { AxiosError, InternalAxiosRequestConfig } from "axios";

const getBaseURL = () => {
    if (typeof window !== "undefined") {
        // If we're on a real domain (not localhost), always prefer the current origin
        if (window.location.hostname !== "localhost") {
            return `${window.location.origin}/api/v1`;
        }
    }
    // Fallback to env var or localhost
    return process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:3003/api/v1";
};

const apiBaseURL = getBaseURL();
if (typeof window !== "undefined") {
    console.log("🔗 API Base URL:", apiBaseURL);
}

const api = axios.create({
    baseURL: apiBaseURL,
    headers: { "Content-Type": "application/json" },
    timeout: 15000,
    withCredentials: true, // send cookies (wms_refresh, wms_access)
});

// ── Request interceptor: attach JWT + Warehouse context ──
api.interceptors.request.use((config) => {
    if (typeof window !== "undefined") {
        const token = localStorage.getItem("wms_token");
        if (token) {
            config.headers.Authorization = `Bearer ${token}`;
        }
        const warehouseId = localStorage.getItem("wms_warehouse_id");
        if (warehouseId) {
            config.headers["X-Warehouse-Id"] = warehouseId;
        }
    }
    return config;
});

// ── Token refresh logic ──
let isRefreshing = false;
let pendingQueue: Array<{
    resolve: (token: string) => void;
    reject: (error: unknown) => void;
}> = [];

function processQueue(error: unknown, token: string | null) {
    pendingQueue.forEach((p) => {
        if (error) {
            p.reject(error);
        } else {
            p.resolve(token!);
        }
    });
    pendingQueue = [];
}

// ── Response interceptor: auto-refresh on 401 ──
api.interceptors.response.use(
    (res) => res,
    async (error: AxiosError) => {
        const originalRequest = error.config as InternalAxiosRequestConfig & { _retry?: boolean };

        if (typeof window === "undefined" || !error.response) {
            return Promise.reject(error);
        }

        // 403 — permission denied toast
        if (error.response.status === 403) {
            window.dispatchEvent(
                new CustomEvent("wms-toast", {
                    detail: {
                        type: "error",
                        message:
                            (error.response.data as { message?: string })?.message ||
                            "You don't have permission for this action",
                    },
                })
            );
            return Promise.reject(error);
        }

        // 401 — try refresh (but not for /auth/refresh or /auth/login itself)
        if (
            error.response.status === 401 &&
            !originalRequest._retry &&
            !originalRequest.url?.includes("/auth/refresh") &&
            !originalRequest.url?.includes("/auth/login")
        ) {
            if (isRefreshing) {
                // Queue this request until refresh completes
                return new Promise<string>((resolve, reject) => {
                    pendingQueue.push({ resolve, reject });
                }).then((token) => {
                    originalRequest.headers.Authorization = `Bearer ${token}`;
                    return api(originalRequest);
                });
            }

            originalRequest._retry = true;
            isRefreshing = true;

            try {
                const res = await api.post<{ token: string }>("/auth/refresh", {});
                const newToken = res.data.token;

                // Update localStorage
                localStorage.setItem("wms_token", newToken);

                // Retry original + queued requests
                processQueue(null, newToken);
                originalRequest.headers.Authorization = `Bearer ${newToken}`;
                return api(originalRequest);
            } catch (refreshError) {
                processQueue(refreshError, null);

                // Refresh failed — force logout
                localStorage.removeItem("wms_token");
                localStorage.removeItem("wms_user");
                localStorage.removeItem("wms_warehouse_id");
                localStorage.removeItem("wms_tenant_id");
                window.location.href = "/login";
                return Promise.reject(refreshError);
            } finally {
                isRefreshing = false;
            }
        }

        // Non-retryable 401 — logout
        if (error.response.status === 401) {
            localStorage.removeItem("wms_token");
            localStorage.removeItem("wms_user");
            localStorage.removeItem("wms_warehouse_id");
            localStorage.removeItem("wms_tenant_id");
            window.location.href = "/login";
        }

        return Promise.reject(error);
    }
);

export default api;
