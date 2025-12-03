"use client";

import { useState, useEffect } from 'react';
import AppLayout from '@/components/layout/AppLayout';
import QRLogin from '@/components/auth/QRLogin';
import AIModelSettings from '@/components/settings/AIModelSettings';
import { useAuth } from '@/hooks/useAuth';
import { Settings, Bot, Upload, User, Link2, CheckCircle, AlertCircle } from 'lucide-react';

export default function SettingsPage() {
  const { user, loading, handleLoginSuccess, handleRefreshStatus, handleLogout } = useAuth();
  const [activeTab, setActiveTab] = useState<'general' | 'ai' | 'account'>('general');
  const [showBindQR, setShowBindQR] = useState(false);
  const [biliStatus, setBiliStatus] = useState<{isLoggedIn: boolean; user?: {name: string; mid: string; avatar?: string}} | null>(null);
  const [autoUpload, setAutoUpload] = useState<boolean>(() => {
    if (typeof window !== 'undefined') {
      try {
        const v = localStorage.getItem('biliup:autoUpload');
        return v === '1';
      } catch {
        return false;
      }
    }
    return false;
  });

  useEffect(() => {
    if (typeof window !== 'undefined') {
      try {
        localStorage.setItem('biliup:autoUpload', autoUpload ? '1' : '0');
      } catch {
        // ignore
      }
    }
  }, [autoUpload]);

  // 检查 B站账号绑定状态
  const checkBiliStatus = async () => {
    try {
      const { protocol, hostname, port } = window.location;
      const apiBaseUrl = `${protocol}//${hostname}${port ? ':' + port : ''}`;
      const res = await fetch(`${apiBaseUrl}/api/v1/auth/status`);
      const data = await res.json();
      if (data.code === 0) {
        setBiliStatus({
          isLoggedIn: data.is_logged_in,
          user: data.user ? { name: data.user.name, mid: data.user.mid, avatar: data.user.avatar } : undefined
        });
      }
    } catch (e) {
      console.error('检查B站状态失败:', e);
    }
  };

  useEffect(() => {
    if (user) {
      checkBiliStatus();
    }
  }, [user]);

  // B站扫码绑定成功回调
  const handleBiliBindSuccess = async () => {
    setShowBindQR(false);
    await checkBiliStatus();
    // 同时刷新 JWT 用户信息（关联 bili_mid）
    await handleRefreshStatus();
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-center">
          <div className="inline-block w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full animate-spin mb-4"></div>
          <p className="text-gray-600">加载中...</p>
        </div>
      </div>
    );
  }

  if (!user) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100">
        <div className="container mx-auto px-4 py-16">
          <div className="max-w-md mx-auto">
            <div className="text-center mb-8">
              <h1 className="text-3xl font-bold text-gray-900 mb-2">
                Bili-Up Web
              </h1>
              <p className="text-gray-600">
                Bilibili 视频管理平台
              </p>
            </div>
            
            <div className="bg-white rounded-lg shadow-lg">
              <QRLogin 
                onLoginSuccess={handleLoginSuccess}
                onRefreshStatus={handleRefreshStatus}
              />
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <AppLayout userName={user?.name} onLogout={handleLogout}>
      <div className="space-y-6">
        {/* 标签页导航 */}
        <div className="bg-white rounded-lg shadow-md">
          <div className="border-b border-gray-200">
            <nav className="flex -mb-px">
              <button
                onClick={() => setActiveTab('general')}
                className={`px-6 py-4 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === 'general'
                    ? 'border-blue-500 text-blue-600'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                }`}
              >
                <Settings className="w-4 h-4 inline mr-2" />
                通用设置
              </button>
              <button
                onClick={() => setActiveTab('ai')}
                className={`px-6 py-4 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === 'ai'
                    ? 'border-blue-500 text-blue-600'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                }`}
              >
                <Bot className="w-4 h-4 inline mr-2" />
                AI 大模型
              </button>
              <button
                onClick={() => setActiveTab('account')}
                className={`px-6 py-4 text-sm font-medium border-b-2 transition-colors ${
                  activeTab === 'account'
                    ? 'border-blue-500 text-blue-600'
                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                }`}
              >
                <Link2 className="w-4 h-4 inline mr-2" />
                账号绑定
              </button>
            </nav>
          </div>
        </div>

        {/* 通用设置 */}
        {activeTab === 'general' && (
          <div className="bg-white rounded-lg shadow-md">
            <div className="p-6 border-b border-gray-200">
              <div className="flex items-center space-x-3">
                <Settings className="w-5 h-5 text-gray-600" />
                <h2 className="text-lg font-medium text-gray-900">通用设置</h2>
              </div>
            </div>

            <div className="p-6">
              <div className="space-y-4">
                <label className="flex items-center justify-between bg-gray-50 p-4 rounded-md">
                  <div>
                    <div className="text-sm font-medium flex items-center">
                      <Upload className="w-4 h-4 mr-2 text-gray-500" />
                      自动上传
                    </div>
                    <div className="text-xs text-gray-500 mt-1">视频提交后自动开始上传任务</div>
                  </div>
                  <div className="relative">
                    <input
                      type="checkbox"
                      checked={autoUpload}
                      onChange={(e) => setAutoUpload(e.target.checked)}
                      className="sr-only"
                    />
                    <div 
                      onClick={() => setAutoUpload(!autoUpload)}
                      className={`w-10 h-6 rounded-full cursor-pointer transition-colors ${autoUpload ? 'bg-blue-600' : 'bg-gray-300'}`}
                    >
                      <div className={`absolute top-1 left-1 w-4 h-4 bg-white rounded-full transition-transform ${autoUpload ? 'translate-x-4' : ''}`} />
                    </div>
                  </div>
                </label>

                <div className="bg-blue-50 p-4 rounded-md">
                  <div className="text-sm text-blue-800">
                    <strong>提示：</strong> 更多通用设置项将在后续版本中添加。
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* AI 大模型设置 */}
        {activeTab === 'ai' && <AIModelSettings />}

        {/* 账号绑定设置 */}
        {activeTab === 'account' && (
          <div className="bg-white rounded-lg shadow-md">
            <div className="p-6 border-b border-gray-200">
              <div className="flex items-center space-x-3">
                <Link2 className="w-5 h-5 text-gray-600" />
                <h2 className="text-lg font-medium text-gray-900">B站账号绑定</h2>
              </div>
              <p className="text-sm text-gray-500 mt-1">绑定 Bilibili 账号后可上传视频到 B 站</p>
            </div>

            <div className="p-6">
              {/* 当前绑定状态 */}
              <div className="mb-6">
                <h3 className="text-sm font-medium text-gray-700 mb-3">当前绑定状态</h3>
                {biliStatus?.isLoggedIn && biliStatus.user ? (
                  <div className="flex items-center justify-between bg-green-50 border border-green-200 rounded-lg p-4">
                    <div className="flex items-center gap-3">
                      {biliStatus.user.avatar ? (
                        <img src={biliStatus.user.avatar} alt="头像" className="w-10 h-10 rounded-full" />
                      ) : (
                        <div className="w-10 h-10 bg-green-200 rounded-full flex items-center justify-center">
                          <User className="w-5 h-5 text-green-600" />
                        </div>
                      )}
                      <div>
                        <p className="font-medium text-green-800">{biliStatus.user.name}</p>
                        <p className="text-xs text-green-600">UID: {biliStatus.user.mid}</p>
                      </div>
                    </div>
                    <div className="flex items-center gap-2 text-green-600">
                      <CheckCircle className="w-5 h-5" />
                      <span className="text-sm font-medium">已绑定</span>
                    </div>
                  </div>
                ) : (
                  <div className="flex items-center justify-between bg-orange-50 border border-orange-200 rounded-lg p-4">
                    <div className="flex items-center gap-3">
                      <div className="w-10 h-10 bg-orange-200 rounded-full flex items-center justify-center">
                        <AlertCircle className="w-5 h-5 text-orange-600" />
                      </div>
                      <div>
                        <p className="font-medium text-orange-800">未绑定 B站账号</p>
                        <p className="text-xs text-orange-600">绑定后才能上传视频到 Bilibili</p>
                      </div>
                    </div>
                  </div>
                )}
              </div>

              {/* 绑定/换绑操作 */}
              <div>
                <h3 className="text-sm font-medium text-gray-700 mb-3">
                  {biliStatus?.isLoggedIn ? '更换绑定账号' : '绑定 B站账号'}
                </h3>
                
                {!showBindQR ? (
                  <button
                    onClick={() => setShowBindQR(true)}
                    className="px-4 py-2 bg-pink-500 text-white rounded-lg hover:bg-pink-600 transition-colors flex items-center gap-2"
                  >
                    <Link2 className="w-4 h-4" />
                    {biliStatus?.isLoggedIn ? '扫码换绑' : '扫码绑定 B站账号'}
                  </button>
                ) : (
                  <div className="border rounded-lg p-4">
                    <div className="flex justify-between items-center mb-4">
                      <span className="text-sm text-gray-600">请使用 Bilibili 手机客户端扫描二维码</span>
                      <button
                        onClick={() => setShowBindQR(false)}
                        className="text-sm text-gray-500 hover:text-gray-700"
                      >
                        取消
                      </button>
                    </div>
                    <QRLogin 
                      onLoginSuccess={handleBiliBindSuccess}
                      onRefreshStatus={checkBiliStatus}
                    />
                  </div>
                )}
              </div>

              {/* 提示信息 */}
              <div className="mt-6 bg-blue-50 p-4 rounded-md">
                <div className="text-sm text-blue-800">
                  <strong>说明：</strong>
                  <ul className="list-disc list-inside mt-1 space-y-1">
                    <li>绑定 B站账号后，系统可以将下载的视频自动上传到您的 B站账号</li>
                    <li>B站登录信息仅保存在本地服务器，不会上传到云端</li>
                    <li>如需更换账号，点击「扫码换绑」重新扫码即可</li>
                  </ul>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </AppLayout>
  );
}
