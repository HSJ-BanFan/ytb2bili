"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { authApi } from "@/lib/api";

type PageView = "login" | "register" | "forgot-password" | "reset-password";

export default function LoginPage() {
  const router = useRouter();
  const [view, setView] = useState<PageView>("login");
  const [loginMethod, setLoginMethod] = useState<"password" | "code">("password");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [sendingCode, setSendingCode] = useState(false);
  const [countdown, setCountdown] = useState(0);
  const [codeSent, setCodeSent] = useState(false);
  const [emailDomainOpen, setEmailDomainOpen] = useState(false);

  // 支持的邮箱域名列表
  const emailDomains = [
    "gmail.com",
    "qq.com",
    "163.com",
    "outlook.com",
    "icloud.com",
    "proton.me",
    "yahoo.com",
    "hotmail.com"
  ];

  const [formData, setFormData] = useState({
    username: "",
    emailPrefix: "",
    emailDomain: "gmail.com",
    password: "",
    confirmPassword: "",
    verificationCode: "",
    newPassword: "",
  });

  // 拼接完整邮箱
  const getFullEmail = () => `${formData.emailPrefix}@${formData.emailDomain}`;

  // 倒计时效果
  useEffect(() => {
    if (countdown > 0) {
      const timer = setTimeout(() => setCountdown(countdown - 1), 1000);
      return () => clearTimeout(timer);
    }
  }, [countdown]);

  // 重置表单
  const resetForm = () => {
    setFormData({
      username: "",
      emailPrefix: "",
      emailDomain: "gmail.com",
      password: "",
      confirmPassword: "",
      verificationCode: "",
      newPassword: "",
    });
    setError("");
    setSuccess("");
    setCodeSent(false);
    setCountdown(0);
  };

  // 发送验证码
  const handleSendCode = async (type: "register" | "login" | "reset_password") => {
    const email = getFullEmail();
    if (!formData.emailPrefix) {
      setError("请先输入邮箱前缀");
      return;
    }

    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    if (!emailRegex.test(email)) {
      setError("请输入有效的邮箱地址");
      return;
    }

    setError("");
    setSendingCode(true);

    try {
      let response;
      if (type === "reset_password") {
        response = await authApi.forgotPassword({ email });
      } else {
        response = await authApi.sendVerificationCode({
          email,
          type: type,
        });
      }

      if (response.code === 0) {
        setCodeSent(true);
        setCountdown(60);
        if (type === "reset_password") {
          setSuccess("验证码已发送到您的邮箱");
          // 自动切换到重置密码页面
          setView("reset-password");
        }
      } else {
        setError(response.message || "发送验证码失败");
      }
    } catch (err: any) {
      setError(err.response?.data?.message || err.message || "发送验证码失败");
    } finally {
      setSendingCode(false);
    }
  };

  // 处理登录
  const handleLogin = async () => {
    setError("");
    setLoading(true);

    try {
      let response;

      if (loginMethod === "password") {
        response = await authApi.login({
          email: getFullEmail(),
          password: formData.password,
        });
      } else {
        response = await authApi.loginWithCode({
          email: getFullEmail(),
          verification_code: formData.verificationCode,
        });
      }

      if (response.code === 0 && response.data) {
        localStorage.setItem("jwt_token", response.data.token.access_token);
        localStorage.setItem("jwt_refresh_token", response.data.token.refresh_token);
        localStorage.setItem("jwt_user", JSON.stringify(response.data.user));
        router.push("/");
      } else {
        setError(response.message || "登录失败");
      }
    } catch (err: any) {
      setError(err.response?.data?.message || err.message || "登录失败");
    } finally {
      setLoading(false);
    }
  };

  // 处理注册
  const handleRegister = async () => {
    if (formData.password !== formData.confirmPassword) {
      setError("两次输入的密码不一致");
      return;
    }

    if (!formData.verificationCode) {
      setError("请输入验证码");
      return;
    }

    setError("");
    setLoading(true);

    try {
      const response = await authApi.register({
        username: formData.username || formData.emailPrefix,
        email: getFullEmail(),
        password: formData.password,
        verification_code: formData.verificationCode,
      });

      if (response.code === 0 && response.data) {
        localStorage.setItem("jwt_token", response.data.token.access_token);
        localStorage.setItem("jwt_refresh_token", response.data.token.refresh_token);
        localStorage.setItem("jwt_user", JSON.stringify(response.data.user));
        router.push("/");
      } else {
        setError(response.message || "注册失败");
      }
    } catch (err: any) {
      setError(err.response?.data?.message || err.message || "注册失败");
    } finally {
      setLoading(false);
    }
  };

  // 处理重置密码
  const handleResetPassword = async () => {
    if (formData.newPassword.length < 6) {
      setError("密码长度至少6位");
      return;
    }

    if (!formData.verificationCode) {
      setError("请输入验证码");
      return;
    }

    setError("");
    setLoading(true);

    try {
      const response = await authApi.resetPassword({
        email: getFullEmail(),
        verification_code: formData.verificationCode,
        new_password: formData.newPassword,
      });

      if (response.code === 0) {
        setSuccess("密码重置成功，请使用新密码登录");
        setTimeout(() => {
          setView("login");
          resetForm();
        }, 2000);
      } else {
        setError(response.message || "重置密码失败");
      }
    } catch (err: any) {
      setError(err.response?.data?.message || err.message || "重置密码失败");
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    switch (view) {
      case "login":
        await handleLogin();
        break;
      case "register":
        await handleRegister();
        break;
      case "reset-password":
        await handleResetPassword();
        break;
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-blue-50 to-indigo-100">
      <div className="bg-white p-8 rounded-2xl shadow-xl w-full max-w-md">
        <div className="text-center mb-8">
          <h1 className="text-3xl font-bold text-gray-800">Bili-Up Web</h1>
          <p className="text-gray-500 mt-2">
            {view === "login" && "登录您的账户"}
            {view === "register" && "创建新账户"}
            {view === "forgot-password" && "找回密码"}
            {view === "reset-password" && "重置密码"}
          </p>
        </div>

        {error && (
          <div className="mb-4 p-3 bg-red-50 border border-red-200 text-red-600 rounded-lg text-sm">
            {error}
          </div>
        )}

        {success && (
          <div className="mb-4 p-3 bg-green-50 border border-green-200 text-green-600 rounded-lg text-sm">
            {success}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          {/* 邮箱输入 */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              邮箱 <span className="text-red-500">*</span>
            </label>
            <div className="flex items-center gap-2">
              <div className="flex-1 flex items-center border border-gray-300 rounded-lg focus-within:ring-2 focus-within:ring-blue-500 focus-within:border-transparent transition overflow-hidden">
                <span className="pl-3 text-gray-400">✉</span>
                <input
                  type="text"
                  value={formData.emailPrefix}
                  onChange={(e) => setFormData({ ...formData, emailPrefix: e.target.value })}
                  className="flex-1 px-2 py-2 border-none focus:outline-none"
                  placeholder="请输入邮箱前缀"
                  required
                />
                <span className="text-gray-400 px-1">@</span>
              </div>
              <div className="relative">
                <button
                  type="button"
                  onClick={() => setEmailDomainOpen(!emailDomainOpen)}
                  className="flex items-center gap-1 px-3 py-2 border border-gray-300 rounded-lg bg-white hover:bg-gray-50 transition min-w-[130px] justify-between"
                >
                  <span className="text-gray-700">{formData.emailDomain}</span>
                  <svg className={`w-4 h-4 text-gray-400 transition-transform ${emailDomainOpen ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                  </svg>
                </button>
                {emailDomainOpen && (
                  <div className="absolute top-full left-0 mt-1 w-full bg-white border border-gray-200 rounded-lg shadow-lg z-10 max-h-48 overflow-y-auto">
                    {emailDomains.map((domain) => (
                      <button
                        key={domain}
                        type="button"
                        onClick={() => {
                          setFormData({ ...formData, emailDomain: domain });
                          setEmailDomainOpen(false);
                        }}
                        className={`w-full px-3 py-2 text-left hover:bg-blue-50 transition ${formData.emailDomain === domain ? 'text-blue-600 bg-blue-50' : 'text-gray-700'}`}
                      >
                        {domain}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* 登录模式 - 选择登录方式 */}
          {view === "login" && (
            <div className="flex gap-2 bg-gray-100 p-1 rounded-lg">
              <button
                type="button"
                onClick={() => { setLoginMethod("password"); setError(""); }}
                className={`flex-1 py-2 text-sm rounded-md transition ${loginMethod === "password"
                  ? "bg-white text-blue-600 shadow"
                  : "text-gray-500"
                  }`}
              >
                密码登录
              </button>
              <button
                type="button"
                onClick={() => { setLoginMethod("code"); setError(""); }}
                className={`flex-1 py-2 text-sm rounded-md transition ${loginMethod === "code"
                  ? "bg-white text-blue-600 shadow"
                  : "text-gray-500"
                  }`}
              >
                验证码登录
              </button>
            </div>
          )}

          {/* 登录 - 密码方式 */}
          {view === "login" && loginMethod === "password" && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                密码
              </label>
              <input
                type="password"
                value={formData.password}
                onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                placeholder="请输入密码"
                required
              />
            </div>
          )}

          {/* 登录 - 验证码方式 */}
          {view === "login" && loginMethod === "code" && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                验证码
              </label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={formData.verificationCode}
                  onChange={(e) => setFormData({ ...formData, verificationCode: e.target.value })}
                  className="flex-1 px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                  placeholder="请输入6位验证码"
                  maxLength={6}
                  required
                />
                <button
                  type="button"
                  onClick={() => handleSendCode("login")}
                  disabled={sendingCode || countdown > 0}
                  className="px-4 py-2 bg-blue-100 text-blue-600 rounded-lg font-medium hover:bg-blue-200 transition disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
                >
                  {sendingCode ? "发送中..." : countdown > 0 ? `${countdown}s` : "获取验证码"}
                </button>
              </div>
            </div>
          )}

          {/* 注册模式 */}
          {view === "register" && (
            <>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  昵称 <span className="text-gray-400 text-xs">（选填）</span>
                </label>
                <input
                  type="text"
                  value={formData.username}
                  onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                  className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                  placeholder="请输入昵称（不填则使用邮箱前缀）"
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  邮箱验证码
                </label>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={formData.verificationCode}
                    onChange={(e) => setFormData({ ...formData, verificationCode: e.target.value })}
                    className="flex-1 px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                    placeholder="请输入6位验证码"
                    maxLength={6}
                    required
                  />
                  <button
                    type="button"
                    onClick={() => handleSendCode("register")}
                    disabled={sendingCode || countdown > 0}
                    className="px-4 py-2 bg-blue-100 text-blue-600 rounded-lg font-medium hover:bg-blue-200 transition disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
                  >
                    {sendingCode ? "发送中..." : countdown > 0 ? `${countdown}s` : "获取验证码"}
                  </button>
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  密码
                </label>
                <input
                  type="password"
                  value={formData.password}
                  onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                  className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                  placeholder="请输入密码（至少6位）"
                  required
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  确认密码
                </label>
                <input
                  type="password"
                  value={formData.confirmPassword}
                  onChange={(e) => setFormData({ ...formData, confirmPassword: e.target.value })}
                  className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                  placeholder="请再次输入密码"
                  required
                />
              </div>
            </>
          )}

          {/* 忘记密码 - 发送验证码 */}
          {view === "forgot-password" && (
            <div className="text-center">
              <p className="text-gray-600 mb-4">
                输入您的邮箱，我们将发送验证码到您的邮箱
              </p>
              <button
                type="button"
                onClick={() => {
                  handleSendCode("reset_password");
                  if (codeSent) {
                    setView("reset-password");
                  } else {
                    // 发送后切换到重置密码页面
                    setTimeout(() => {
                      if (countdown > 0) {
                        setView("reset-password");
                      }
                    }, 500);
                  }
                }}
                disabled={sendingCode || countdown > 0}
                className="w-full py-3 bg-blue-600 text-white rounded-lg font-medium hover:bg-blue-700 focus:ring-4 focus:ring-blue-200 transition disabled:opacity-50"
              >
                {sendingCode ? "发送中..." : countdown > 0 ? `已发送 (${countdown}s)` : "发送验证码"}
              </button>
            </div>
          )}

          {/* 重置密码 */}
          {view === "reset-password" && (
            <>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  验证码
                </label>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={formData.verificationCode}
                    onChange={(e) => setFormData({ ...formData, verificationCode: e.target.value })}
                    className="flex-1 px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                    placeholder="请输入6位验证码"
                    maxLength={6}
                    required
                  />
                  <button
                    type="button"
                    onClick={() => handleSendCode("reset_password")}
                    disabled={sendingCode || countdown > 0}
                    className="px-4 py-2 bg-blue-100 text-blue-600 rounded-lg font-medium hover:bg-blue-200 transition disabled:opacity-50 disabled:cursor-not-allowed whitespace-nowrap"
                  >
                    {sendingCode ? "发送中..." : countdown > 0 ? `${countdown}s` : "重新发送"}
                  </button>
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  新密码
                </label>
                <input
                  type="password"
                  value={formData.newPassword}
                  onChange={(e) => setFormData({ ...formData, newPassword: e.target.value })}
                  className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition"
                  placeholder="请输入新密码（至少6位）"
                  required
                />
              </div>
            </>
          )}

          {/* 提交按钮 */}
          {view !== "forgot-password" && (
            <button
              type="submit"
              disabled={loading}
              className="w-full py-3 bg-blue-600 text-white rounded-lg font-medium hover:bg-blue-700 focus:ring-4 focus:ring-blue-200 transition disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? "处理中..." :
                view === "login" ? "登录" :
                  view === "register" ? "注册" :
                    "重置密码"}
            </button>
          )}
        </form>

        {/* 底部链接 */}
        <div className="mt-6 text-center space-y-2">
          {view === "login" && (
            <>
              <button
                onClick={() => { setView("forgot-password"); resetForm(); }}
                className="text-gray-500 hover:text-gray-700 text-sm block w-full"
              >
                忘记密码？
              </button>
              <button
                onClick={() => { setView("register"); resetForm(); }}
                className="text-blue-600 hover:text-blue-700 text-sm"
              >
                没有账户？点击注册
              </button>
            </>
          )}

          {view === "register" && (
            <button
              onClick={() => { setView("login"); resetForm(); }}
              className="text-blue-600 hover:text-blue-700 text-sm"
            >
              已有账户？点击登录
            </button>
          )}

          {(view === "forgot-password" || view === "reset-password") && (
            <button
              onClick={() => { setView("login"); resetForm(); }}
              className="text-blue-600 hover:text-blue-700 text-sm"
            >
              返回登录
            </button>
          )}
        </div>

        <div className="mt-8 pt-6 border-t border-gray-200">
          <p className="text-xs text-gray-500 text-center">
            登录后即可使用视频转载功能
          </p>
        </div>
      </div>
    </div>
  );
}
