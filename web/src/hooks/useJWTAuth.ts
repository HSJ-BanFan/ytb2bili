"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import { authApi } from "@/lib/api";

interface JWTUser {
  id: number;
  username: string;
  email: string;
}

export function useJWTAuth() {
  const router = useRouter();
  const [user, setUser] = useState<JWTUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [isAuthenticated, setIsAuthenticated] = useState(false);

  // 检查本地存储的认证状态
  const checkAuth = useCallback(() => {
    if (typeof window === "undefined") {
      setLoading(false);
      return;
    }

    const token = localStorage.getItem("jwt_token");
    const userStr = localStorage.getItem("jwt_user");

    if (token && userStr) {
      try {
        const userData = JSON.parse(userStr);
        setUser(userData);
        setIsAuthenticated(true);
      } catch (e) {
        console.error("解析用户信息失败:", e);
        clearAuth();
      }
    } else {
      setIsAuthenticated(false);
      setUser(null);
    }
    setLoading(false);
  }, []);

  // 清除认证信息
  const clearAuth = useCallback(() => {
    if (typeof window !== "undefined") {
      localStorage.removeItem("jwt_token");
      localStorage.removeItem("jwt_refresh_token");
      localStorage.removeItem("jwt_user");
    }
    setUser(null);
    setIsAuthenticated(false);
  }, []);

  // 登出
  const logout = useCallback(async () => {
    try {
      const token = localStorage.getItem("jwt_token");
      if (token) {
        await authApi.jwtLogout();
      }
    } catch (e) {
      console.error("登出请求失败:", e);
    } finally {
      clearAuth();
      // 使用 window.location 强制跳转，确保页面完全刷新
      window.location.href = "/login";
    }
  }, [clearAuth]);

  // 刷新 Token
  const refreshToken = useCallback(async () => {
    const refreshTokenStr = localStorage.getItem("jwt_refresh_token");
    if (!refreshTokenStr) {
      clearAuth();
      return false;
    }

    try {
      const response = await authApi.refreshToken(refreshTokenStr);
      if (response.code === 0 && response.data) {
        localStorage.setItem("jwt_token", response.data.token.access_token);
        localStorage.setItem("jwt_refresh_token", response.data.token.refresh_token);
        localStorage.setItem("jwt_user", JSON.stringify(response.data.user));
        setUser(response.data.user);
        setIsAuthenticated(true);
        return true;
      }
    } catch (e) {
      console.error("刷新 Token 失败:", e);
    }

    clearAuth();
    return false;
  }, [clearAuth]);

  // 要求认证（未登录则跳转）
  const requireAuth = useCallback(() => {
    if (!loading && !isAuthenticated) {
      router.push("/login");
    }
  }, [loading, isAuthenticated, router]);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  return {
    user,
    loading,
    isAuthenticated,
    logout,
    refreshToken,
    requireAuth,
    checkAuth,
  };
}
