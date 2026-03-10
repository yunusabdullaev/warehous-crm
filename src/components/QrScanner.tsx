"use client";
import { useEffect, useRef, useState } from "react";
import { Html5Qrcode } from "html5-qrcode";

interface Props {
    onScan: (data: string) => void;
    onError?: (err: string) => void;
}

export default function QrScanner({ onScan, onError }: Props) {
    const [starting, setStarting] = useState(true);
    const scannerRef = useRef<Html5Qrcode | null>(null);
    const containerRef = useRef<string>("qr-reader-" + Math.random().toString(36).slice(2, 8));

    useEffect(() => {
        const scanner = new Html5Qrcode(containerRef.current);
        scannerRef.current = scanner;

        scanner
            .start(
                { facingMode: "environment" },
                { fps: 10, qrbox: { width: 250, height: 250 } },
                (decodedText) => {
                    // Stop scanning after first successful read
                    scanner.stop().catch(() => { });
                    onScan(decodedText);
                },
                () => { } // ignore scan failures (no QR in frame)
            )
            .then(() => setStarting(false))
            .catch((err) => {
                setStarting(false);
                if (onError) onError(String(err));
            });

        return () => {
            scanner.stop().catch(() => { });
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    return (
        <div className="relative">
            {starting && (
                <div className="absolute inset-0 flex items-center justify-center bg-gray-900/70 rounded-xl z-10">
                    <div className="text-center text-white">
                        <div className="animate-spin w-8 h-8 border-4 border-white border-t-transparent rounded-full mx-auto mb-3" />
                        <p className="text-sm">Starting camera...</p>
                    </div>
                </div>
            )}
            <div
                id={containerRef.current}
                className="rounded-xl overflow-hidden bg-black"
                style={{ minHeight: 300 }}
            />
        </div>
    );
}
