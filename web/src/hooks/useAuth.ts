import { useState, useEffect, useCallback } from 'react';

interface UserInfo {
  id: string;
  name: string;
  mid?: string;
  avatar?: string;
  username?: string;
  role?: string; // 'admin' or 'user'
  bili_mid?: string;
}

// 获取API基础URL
const getApiBaseUrl = () => {
  if (typeof window !== 'undefined') {
    const { protocol, hostname, port } = window.location;
    return `${protocol}//${hostname}${port ? ':' + port : ''}`;
  }
  return 'http://localhost:8096';
};

export function useAuth() {
  const [user, setUser] = useState<UserInfo | null>(null);
  const [loading, setLoading] = useState(true);

  // 统一认证检查：优先 JWT，其次尝试 B站Session 换取 JWT
  const checkAuthStatus = useCallback(async () => {
    try {
      // 1. 优先检查 JWT Token
      const jwtToken = localStorage.getItem('jwt_token');
      const jwtUserStr = localStorage.getItem('jwt_user');

      if (jwtToken && jwtUserStr) {
        try {
          // 验证 JWT Token 是否有效
          const apiBaseUrl = getApiBaseUrl();
          const res = await fetch(`${apiBaseUrl}/api/v1/user/me`, {
            headers: { 'Authorization': `Bearer ${jwtToken}` }
          });

          if (res.ok) {
            const data = await res.json();
            if (data.code === 0 && data.data) {
              const userData: UserInfo = {
                id: String(data.data.id),
                name: data.data.username,
                username: data.data.username,
                avatar: data.data.avatar,
                role: data.data.role,
                bili_mid: data.data.bili_mid,
              };
              setUser(userData);
              localStorage.setItem('jwt_user', JSON.stringify(userData));
              console.log('✅ JWT 认证成功:', userData.name);
              setLoading(false);
              return;
            }
          }
        } catch (e) {
          console.warn('JWT Token 验证失败，尝试其他方式');
        }
        // JWT 无效，清除
        localStorage.removeItem('jwt_token');
        localStorage.removeItem('jwt_refresh_token');
        localStorage.removeItem('jwt_user');
      }

      // 2. 未登录
      console.log('❌ 未登录');
      setUser(null);
    } catch (error) {
      console.error('检查登录状态失败:', error);
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  // 检查登录状态
  useEffect(() => {
    checkAuthStatus();
  }, [checkAuthStatus]);

  // B站账户绑定成功后调用（刷新用户状态）
  const handleLoginSuccess = async (_biliUserData?: UserInfo) => {
    // 绑定成功后，刷新用户状态
    await checkAuthStatus();
  };

  // 刷新状态
  const handleRefreshStatus = async () => {
    await checkAuthStatus();
  };

  // 登出：同时清除 JWT 和 B站 Session
  const handleLogout = async () => {
    try {
      const apiBaseUrl = getApiBaseUrl();

      // 清除 JWT Token
      const jwtToken = localStorage.getItem('jwt_token');
      if (jwtToken) {
        try {
          await fetch(`${apiBaseUrl}/api/v1/user/logout`, {
            method: 'POST',
            headers: { 'Authorization': `Bearer ${jwtToken}` }
          });
        } catch (e) {
          console.warn('JWT 登出失败:', e);
        }
      }

      // 清除 B站 Session
      await fetch(`${apiBaseUrl}/api/v1/auth/logout`, { method: 'POST' });

      // 清除所有本地存储
      localStorage.removeItem('jwt_token');
      localStorage.removeItem('jwt_refresh_token');
      localStorage.removeItem('jwt_user');
      localStorage.removeItem('bili_user');

      setUser(null);
      console.log('✅ 登出成功');

      // 跳转到登录页面
      window.location.href = '/login';
    } catch (error) {
      console.error('登出失败:', error);
      // 即使失败也跳转到登录页面
      window.location.href = '/login';
    }
  };

  return {
    user,
    loading,
    isLoggedIn: !!user,
    handleLoginSuccess,
    handleRefreshStatus,
    handleLogout,
  };
}