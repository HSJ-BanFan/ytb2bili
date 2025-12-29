"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useRouter } from "next/navigation";
import { authApi, BiliAccount } from "@/lib/api";

export default function BiliAccountsPage() {
    const router = useRouter();
    const [accounts, setAccounts] = useState<BiliAccount[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState("");
    const [success, setSuccess] = useState("");
    const [binding, setBinding] = useState(false);
    const [qrCode, setQrCode] = useState<string | null>(null);
    const [qrStatus, setQrStatus] = useState<"idle" | "loading" | "waiting" | "success" | "expired">("idle");

    // 使用 useRef 追踪轮询是否应该停止（组件卸载时设置为 true）
    const pollingStoppedRef = useRef(false);

    // 加载账户列表
    const loadAccounts = useCallback(async () => {
        try {
            const response = await authApi.getBiliAccounts();
            if (response.code === 0 && response.data) {
                setAccounts(response.data);
            }
        } catch (err) {
            console.error("加载账户列表失败:", err);
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        // 检查登录状态
        const token = localStorage.getItem("jwt_token");
        if (!token) {
            router.push("/login");
            return;
        }
        loadAccounts();

        // 清理函数：组件卸载时停止轮询
        return () => {
            pollingStoppedRef.current = true;
        };
    }, [loadAccounts, router]);

    // 开始扫码绑定
    const startQRCodeBinding = async () => {
        setError("");
        setSuccess("");
        setBinding(true);
        setQrStatus("loading");
        // 重置轮询停止标志
        pollingStoppedRef.current = false;

        try {
            // 获取二维码
            const response = await fetch("/api/v1/auth/qrcode");
            const data = await response.json();

            // 后端返回格式: { code: 0, message: "success", qr_code_url: "...", auth_code: "..." }
            if (data.code === 0 && data.qr_code_url) {
                setQrCode(data.qr_code_url);
                setQrStatus("waiting");
                // 开始轮询二维码状态
                pollQRCodeStatus(data.auth_code);
            } else {
                setError(data.message || "获取二维码失败");
                setQrStatus("idle");
                setBinding(false);
            }
        } catch (err: any) {
            setError(err.message || "获取二维码失败");
            setQrStatus("idle");
            setBinding(false);
        }
    };

    // 轮询二维码状态
    const pollQRCodeStatus = async (authCode: string) => {
        const maxAttempts = 60; // 最多轮询60次（约3分钟）
        let attempts = 0;

        const poll = async () => {
            // 如果已停止或超过最大次数，不再继续
            if (pollingStoppedRef.current || attempts >= maxAttempts) {
                if (attempts >= maxAttempts && !pollingStoppedRef.current) {
                    setQrStatus("expired");
                    setBinding(false);
                }
                return;
            }

            try {
                // 后端使用 POST /api/v1/auth/poll 进行轮询
                const response = await fetch("/api/v1/auth/poll", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ auth_code: authCode }),
                });
                const data = await response.json();

                if (pollingStoppedRef.current) return; // 再次检查，避免组件卸载后更新状态

                if (data.code === 0 && data.login_info) {
                    // 扫码成功，标记停止并调用绑定接口
                    pollingStoppedRef.current = true;
                    setQrStatus("success");
                    await bindAccountFromQRCode();
                    return;
                } else if (data.code === 86038 || data.message?.includes("过期")) {
                    pollingStoppedRef.current = true;
                    setQrStatus("expired");
                    setBinding(false);
                    return;
                } else if (data.code === 86090) {
                    // 已扫码，等待确认
                    // 继续轮询
                }

                attempts++;
                if (!pollingStoppedRef.current) {
                    setTimeout(poll, 3000); // 3秒后继续轮询
                }
            } catch (err) {
                if (pollingStoppedRef.current) return;
                attempts++;
                if (!pollingStoppedRef.current) {
                    setTimeout(poll, 3000);
                }
            }
        };

        poll();
    };

    // 从扫码结果绑定账户
    const bindAccountFromQRCode = async () => {
        try {
            const response = await authApi.bindBiliAccountFromQRCode();
            if (response.code === 0) {
                setSuccess("B站账户绑定成功！");
                setQrCode(null);
                setQrStatus("idle");
                loadAccounts();
            } else {
                setError(response.message || "绑定失败");
            }
        } catch (err: any) {
            setError(err.response?.data?.message || err.message || "绑定失败");
        } finally {
            setBinding(false);
        }
    };

    // 解绑账户
    const handleUnbind = async (account: BiliAccount) => {
        if (!confirm(`确定要解绑 ${account.bili_name} 吗？`)) return;

        try {
            const response = await authApi.unbindBiliAccount(Number(account.id));
            if (response.code === 0) {
                setSuccess("解绑成功");
                loadAccounts();
            } else {
                setError(response.message || "解绑失败");
            }
        } catch (err: any) {
            setError(err.response?.data?.message || err.message || "解绑失败");
        }
    };

    // 设为主账户
    const handleSetPrimary = async (account: BiliAccount) => {
        try {
            const response = await authApi.setBiliAccountPrimary(Number(account.id));
            if (response.code === 0) {
                setSuccess(`已将 ${account.bili_name} 设为主账户`);
                loadAccounts();
            } else {
                setError(response.message || "设置失败");
            }
        } catch (err: any) {
            setError(err.response?.data?.message || err.message || "设置失败");
        }
    };

    if (loading) {
        return (
            <div className="min-h-screen bg-gray-50 flex items-center justify-center">
                <div className="text-gray-500">加载中...</div>
            </div>
        );
    }

    return (
        <div className="min-h-screen bg-gray-50 py-8">
            <div className="max-w-4xl mx-auto px-4">
                {/* 页头 */}
                <div className="flex items-center justify-between mb-8">
                    <div>
                        <h1 className="text-2xl font-bold text-gray-800">B站账户管理</h1>
                        <p className="text-gray-500 mt-1">管理您绑定的 B站账户，用于视频上传</p>
                    </div>
                    <button
                        onClick={() => router.push("/")}
                        className="text-gray-500 hover:text-gray-700"
                    >
                        返回首页
                    </button>
                </div>

                {/* 提示信息 */}
                {error && (
                    <div className="mb-4 p-4 bg-red-50 border border-red-200 text-red-600 rounded-lg">
                        {error}
                    </div>
                )}
                {success && (
                    <div className="mb-4 p-4 bg-green-50 border border-green-200 text-green-600 rounded-lg">
                        {success}
                    </div>
                )}

                {/* 绑定新账户 */}
                <div className="bg-white rounded-xl shadow-sm p-6 mb-6">
                    <h2 className="text-lg font-semibold text-gray-800 mb-4">绑定新账户</h2>

                    {qrStatus === "idle" && (
                        <button
                            onClick={startQRCodeBinding}
                            disabled={binding}
                            className="px-6 py-3 bg-pink-500 text-white rounded-lg hover:bg-pink-600 transition disabled:opacity-50"
                        >
                            扫码绑定 B站账户
                        </button>
                    )}

                    {qrStatus === "loading" && (
                        <div className="text-gray-500">正在获取二维码...</div>
                    )}

                    {qrStatus === "waiting" && qrCode && (
                        <div className="flex flex-col items-center">
                            <img
                                src={qrCode}
                                alt="扫码绑定"
                                className="w-48 h-48 border rounded-lg"
                            />
                            <p className="mt-4 text-gray-600">请使用 B站APP 扫描二维码</p>
                            <button
                                onClick={() => {
                                    setQrCode(null);
                                    setQrStatus("idle");
                                    setBinding(false);
                                }}
                                className="mt-2 text-gray-500 hover:text-gray-700"
                            >
                                取消
                            </button>
                        </div>
                    )}

                    {qrStatus === "success" && (
                        <div className="text-green-600">扫码成功，正在绑定...</div>
                    )}

                    {qrStatus === "expired" && (
                        <div className="flex flex-col items-center">
                            <p className="text-red-500 mb-2">二维码已过期</p>
                            <button
                                onClick={startQRCodeBinding}
                                className="text-pink-500 hover:text-pink-600"
                            >
                                重新获取
                            </button>
                        </div>
                    )}
                </div>

                {/* 已绑定账户列表 */}
                <div className="bg-white rounded-xl shadow-sm p-6">
                    <h2 className="text-lg font-semibold text-gray-800 mb-4">
                        已绑定账户 ({accounts.length})
                    </h2>

                    {accounts.length === 0 ? (
                        <div className="text-center py-8 text-gray-500">
                            暂无绑定的 B站账户，请点击上方按钮绑定
                        </div>
                    ) : (
                        <div className="space-y-4">
                            {accounts.map((account) => (
                                <div
                                    key={account.id}
                                    className={`flex items-center justify-between p-4 border rounded-lg ${account.is_primary ? "border-pink-300 bg-pink-50" : "border-gray-200"
                                        }`}
                                >
                                    <div className="flex items-center gap-4">
                                        <img
                                            src={account.bili_face || "/default-avatar.png"}
                                            alt={account.bili_name}
                                            className="w-12 h-12 rounded-full"
                                        />
                                        <div>
                                            <div className="flex items-center gap-2">
                                                <span className="font-medium text-gray-800">
                                                    {account.bili_name}
                                                </span>
                                                {account.is_primary && (
                                                    <span className="px-2 py-0.5 text-xs bg-pink-500 text-white rounded">
                                                        主账户
                                                    </span>
                                                )}
                                                {!account.is_enabled && (
                                                    <span className="px-2 py-0.5 text-xs bg-gray-400 text-white rounded">
                                                        已禁用
                                                    </span>
                                                )}
                                            </div>
                                            <div className="text-sm text-gray-500">
                                                UID: {account.bili_mid}
                                            </div>
                                        </div>
                                    </div>

                                    <div className="flex items-center gap-2">
                                        {!account.is_primary && (
                                            <button
                                                onClick={() => handleSetPrimary(account)}
                                                className="px-3 py-1.5 text-sm text-pink-500 hover:bg-pink-50 rounded transition"
                                            >
                                                设为主账户
                                            </button>
                                        )}
                                        <button
                                            onClick={() => handleUnbind(account)}
                                            className="px-3 py-1.5 text-sm text-red-500 hover:bg-red-50 rounded transition"
                                        >
                                            解绑
                                        </button>
                                    </div>
                                </div>
                            ))}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}
