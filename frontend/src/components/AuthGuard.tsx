"use client";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { isAuthenticated, can } from "@/lib/auth";

interface Props {
    children: React.ReactNode;
    requiredAction?: string;
}

export default function AuthGuard({ children, requiredAction }: Props) {
    const router = useRouter();
    const [ok, setOk] = useState(false);

    useEffect(() => {
        if (!isAuthenticated()) {
            router.replace("/login");
            return;
        }
        if (requiredAction && !can(requiredAction)) {
            router.replace("/dashboard");
            return;
        }
        setOk(true);
    }, [router, requiredAction]);

    if (!ok) {
        return (
            <div className="flex items-center justify-center h-full">
                <div className="animate-spin w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full" />
            </div>
        );
    }

    return <>{children}</>;
}
