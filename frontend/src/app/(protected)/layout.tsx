"use client";
import Sidebar from "@/components/Sidebar";
import AuthGuard from "@/components/AuthGuard";
import ToastContainer from "@/components/Toast";

export default function ProtectedLayout({ children }: { children: React.ReactNode }) {
    return (
        <AuthGuard>
            <div className="flex min-h-screen">
                <Sidebar />
                <main className="flex-1 p-6 overflow-auto bg-gray-50">
                    {children}
                </main>
            </div>
            <ToastContainer />
        </AuthGuard>
    );
}
