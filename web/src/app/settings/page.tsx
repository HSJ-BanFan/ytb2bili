"use client";

import { useState, useEffect, useCallback } from 'react';
import AppLayout from '@/components/layout/AppLayout';
import QRLogin from '@/components/auth/QRLogin';
import AIModelSettings from '@/components/settings/AIModelSettings';
import { useAuth } from '@/hooks/useAuth';
import { authApi, BiliAccount } from '@/lib/api';
import { Settings, Bot, Upload, User, Link2, CheckCircle, AlertCircle, Trash2, Star, ToggleLeft, ToggleRight, Plus, RefreshCw } from 'lucide-react';

export default function SettingsPage() {
  const { user, loading, handleLoginSuccess, handleRefreshStatus, handleLogout } = useAuth();
  const [activeTab, setActiveTab] = useState<'general' | 'ai' | 'account'>('general');
  const [showBindQR, setShowBindQR] = useState(false);
  const [biliStatus, setBiliStatus] = useState<{ isLoggedIn: boolean; user?: { name: string; mid: string; avatar?: string } } | null>(null);
  const [downloadConfig, setDownloadConfig] = useState({
    autoUploadMode: 'delayed',
    videoUploadDelay: 10,
    subtitleUploadDelay: 10
  });

  const [autoUpload, setAutoUpload] = useState<boolean>(false);

  // 多账号状态
  const [accounts, setAccounts] = useState<BiliAccount[]>([]);
  const [accountsLoading, setAccountsLoading] = useState(false);
  const [accountError, setAccountError] = useState<string | null>(null);

  // Fetch download config from backend
  const fetchDownloadConfig = useCallback(async () => {
    try {
      const res = await fetch('/api/v1/config/download');
      const data = await res.json();
      if (data.code === 200) {
        setAutoUpload(data.data.auto_upload_enabled);
        setDownloadConfig({
          autoUploadMode: data.data.auto_upload_mode || 'delayed',
          videoUploadDelay: data.data.video_upload_delay || 10,
          subtitleUploadDelay: data.data.subtitle_upload_delay || 10
        });
      }
    } catch (e) {
      console.error('获取下载配置失败:', e);
    }
  }, []);

  // Update download config
  const updateDownloadConfig = async (key: string, value: any) => {
    // Process value to ensure numbers for delay fields
    let processedValue = value;
    if (key === 'videoUploadDelay' || key === 'subtitleUploadDelay') {
      processedValue = parseInt(value) || 0;
    }

    const newConfig = {
      auto_upload_enabled: key === 'autoUploadEnabled' ? value : autoUpload,
      auto_upload_mode: key === 'autoUploadMode' ? value : downloadConfig.autoUploadMode,
      video_upload_delay: key === 'videoUploadDelay' ? processedValue : parseInt(String(downloadConfig.videoUploadDelay)),
      subtitle_upload_delay: key === 'subtitleUploadDelay' ? processedValue : parseInt(String(downloadConfig.subtitleUploadDelay))
    };

    try {
      const res = await fetch('/api/v1/config/download', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(newConfig),
      });
      const data = await res.json();
      if (data.code === 200) {
        if (key === 'autoUploadEnabled') setAutoUpload(value);
        else {
          setDownloadConfig(prev => ({
            ...prev,
            [key]: processedValue
          }));
        }
      } else {
        alert('更新配置失败: ' + data.message);
      }
    } catch (e) {
      console.error('更新配置失败:', e);
      alert('更新配置失败');
    }
  };

  useEffect(() => {
    if (user) {
      fetchDownloadConfig();
    }
  }, [user, fetchDownloadConfig]);

  // 加载账号列表
  const loadAccounts = useCallback(async () => {
    setAccountsLoading(true);
    setAccountError(null);
    try {
      const res = await authApi.getAccounts() as any;
      if (res.code === 0 && res.accounts) {
        setAccounts(res.accounts);
      } else if (res.code === 0 && res.data?.accounts) {
        setAccounts(res.data.accounts);
      }
    } catch (e) {
      console.error('加载账号列表失败:', e);
      setAccountError('加载账号列表失败');
    } finally {
      setAccountsLoading(false);
    }
  }, []);

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
      loadAccounts();
    }
  }, [user, loadAccounts]);

  // B站扫码绑定成功回调
  const handleBiliBindSuccess = async () => {
    setShowBindQR(false);
    await checkBiliStatus();
    await loadAccounts();
    // 同时刷新 JWT 用户信息（关联 bili_mid）
    await handleRefreshStatus();
  };

  // 删除账号
  const handleRemoveAccount = async (mid: number) => {
    console.log('[handleRemoveAccount] 被调用, mid =', mid);
    if (!confirm('确定要删除此账号吗？')) return;
    try {
      await authApi.removeAccount(String(mid));
      await loadAccounts();
      await checkBiliStatus();
    } catch (e) {
      console.error('删除账号失败:', e);
      alert('删除账号失败');
    }
  };

  // 切换账号启用状态
  const handleToggleEnabled = async (mid: number, currentEnabled: boolean) => {
    try {
      await authApi.setAccountEnabled(String(mid), !currentEnabled);
      await loadAccounts();
    } catch (e) {
      console.error('切换账号状态失败:', e);
      alert('切换账号状态失败');
    }
  };

  // 设置主账号
  const handleSetPrimary = async (mid: number) => {
    try {
      await authApi.setPrimaryAccount(String(mid));
      await loadAccounts();
      await checkBiliStatus();
    } catch (e) {
      console.error('设置主账号失败:', e);
      alert('设置主账号失败');
    }
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
      <div className="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100 flex items-center justify-center">
        <div className="container mx-auto px-4">
          <div className="max-w-md mx-auto">
            <div className="text-center mb-8">
              <h1 className="text-4xl font-bold text-gray-900 mb-3">
                Bili-Up Web
              </h1>
              <p className="text-gray-600 text-lg">
                Bilibili 视频管理平台
              </p>
            </div>

            <div className="bg-white rounded-xl shadow-xl p-8">
              <div className="text-center mb-6">
                <div className="inline-flex items-center justify-center w-16 h-16 bg-blue-100 rounded-full mb-4">
                  <Settings className="w-8 h-8 text-blue-600" />
                </div>
                <h2 className="text-xl font-semibold text-gray-900">请先登录</h2>
                <p className="text-gray-500 mt-2">登录后即可访问设置页面</p>
              </div>

              <a
                href="/login"
                className="w-full flex items-center justify-center px-6 py-4 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors text-lg font-medium shadow-md hover:shadow-lg"
              >
                登录 / 注册
              </a>
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
                className={`px-6 py-4 text-sm font-medium border-b-2 transition-colors ${activeTab === 'general'
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                  }`}
              >
                <Settings className="w-4 h-4 inline mr-2" />
                通用设置
              </button>
              <button
                onClick={() => setActiveTab('ai')}
                className={`px-6 py-4 text-sm font-medium border-b-2 transition-colors ${activeTab === 'ai'
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                  }`}
              >
                <Bot className="w-4 h-4 inline mr-2" />
                AI 大模型
              </button>
              <button
                onClick={() => setActiveTab('account')}
                className={`px-6 py-4 text-sm font-medium border-b-2 transition-colors ${activeTab === 'account'
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
                      onChange={(e) => updateDownloadConfig('autoUploadEnabled', e.target.checked)}
                      className="sr-only"
                    />
                    <div
                      className={`w-10 h-6 rounded-full cursor-pointer transition-colors ${autoUpload ? 'bg-blue-600' : 'bg-gray-300'}`}
                    >
                      <div className={`absolute top-1 left-1 w-4 h-4 bg-white rounded-full transition-transform ${autoUpload ? 'translate-x-4' : ''}`} />
                    </div>
                  </div>
                </label>

                {/* 自动上传配置详情 */}
                {autoUpload && (
                  <div className="bg-gray-50 p-4 rounded-md space-y-4 border border-gray-200">
                    <h3 className="text-sm font-medium text-gray-700 mb-2">上传策略配置</h3>

                    {/* 上传模式 */}
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">
                          上传模式
                        </label>
                        <select
                          value={downloadConfig.autoUploadMode}
                          onChange={(e) => updateDownloadConfig('autoUploadMode', e.target.value)}
                          className="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm p-2 border"
                        >
                          <option value="delayed">延迟上传 (推荐)</option>
                          <option value="immediate">立即上传</option>
                        </select>
                        <p className="mt-1 text-xs text-gray-500">
                          {downloadConfig.autoUploadMode === 'delayed'
                            ? '处理完成后等待一段时间再上传，也是为了防止被B站频繁请求拦截'
                            : '处理完成后立即尝试上传视频'}
                        </p>
                      </div>
                    </div>

                    {/* 延迟时间设置 */}
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">
                          视频上传延迟 (分钟)
                        </label>
                        <input
                          type="number"
                          min="0"
                          max="1440"
                          value={downloadConfig.videoUploadDelay}
                          onChange={(e) => updateDownloadConfig('videoUploadDelay', e.target.value)}
                          disabled={downloadConfig.autoUploadMode !== 'delayed'}
                          className="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm p-2 border disabled:bg-gray-100 disabled:text-gray-400"
                        />
                        <p className="mt-1 text-xs text-gray-500">
                          视频处理完成后，等待多久开始上传视频
                        </p>
                      </div>

                      <div>
                        <label className="block text-sm font-medium text-gray-700 mb-1">
                          字幕上传延迟 (分钟)
                        </label>
                        <input
                          type="number"
                          min="0"
                          max="1440"
                          value={downloadConfig.subtitleUploadDelay}
                          onChange={(e) => updateDownloadConfig('subtitleUploadDelay', e.target.value)}
                          className="w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm p-2 border"
                        />
                        <p className="mt-1 text-xs text-gray-500">
                          视频上传成功后，等待多久上传字幕
                        </p>
                      </div>
                    </div>
                  </div>
                )}

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
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-3">
                  <Link2 className="w-5 h-5 text-gray-600" />
                  <h2 className="text-lg font-medium text-gray-900">B站账号管理</h2>
                </div>
                <button
                  onClick={loadAccounts}
                  disabled={accountsLoading}
                  className="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-lg transition-colors"
                  title="刷新账号列表"
                >
                  <RefreshCw className={`w-4 h-4 ${accountsLoading ? 'animate-spin' : ''}`} />
                </button>
              </div>
              <p className="text-sm text-gray-500 mt-1">管理多个 Bilibili 账号，支持添加、删除和切换主账号</p>
            </div>

            <div className="p-6">
              {/* 账号列表 */}
              <div className="mb-6">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="text-sm font-medium text-gray-700">已绑定账号 ({accounts.length})</h3>
                </div>

                {accountsLoading ? (
                  <div className="flex items-center justify-center py-8">
                    <div className="inline-block w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full animate-spin"></div>
                    <span className="ml-2 text-gray-500">加载中...</span>
                  </div>
                ) : accountError ? (
                  <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-600 text-sm">
                    {accountError}
                  </div>
                ) : accounts.length === 0 ? (
                  <div className="bg-orange-50 border border-orange-200 rounded-lg p-4">
                    <div className="flex items-center gap-3">
                      <AlertCircle className="w-5 h-5 text-orange-600" />
                      <div>
                        <p className="font-medium text-orange-800">暂无绑定账号</p>
                        <p className="text-xs text-orange-600">点击下方按钮添加 B站账号</p>
                      </div>
                    </div>
                  </div>
                ) : (
                  <div className="space-y-3">
                    {accounts.map((account) => (
                      <div
                        key={account.id}
                        className={`flex items-center justify-between p-4 rounded-lg border ${account.is_primary
                          ? 'bg-blue-50 border-blue-200'
                          : account.is_expired
                            ? 'bg-red-50 border-red-200'
                            : account.is_enabled
                              ? 'bg-green-50 border-green-200'
                              : 'bg-gray-50 border-gray-200'
                          }`}
                      >
                        <div className="flex items-center gap-3">
                          {account.face ? (
                            <img
                              src={account.face}
                              alt=""
                              className="w-10 h-10 rounded-full bg-gray-200"
                              onError={(e) => {
                                e.currentTarget.style.display = 'none';
                                e.currentTarget.nextElementSibling?.classList.remove('hidden');
                              }}
                            />
                          ) : null}
                          <div className={`w-10 h-10 bg-gradient-to-br from-pink-400 to-blue-400 rounded-full flex items-center justify-center text-white font-bold ${account.face ? 'hidden' : ''}`}>
                            {account.name?.charAt(0)?.toUpperCase() || 'B'}
                          </div>
                          <div>
                            <div className="flex items-center gap-2">
                              <p className="font-medium text-gray-800">{account.name}</p>
                              {account.is_primary && (
                                <span className="px-2 py-0.5 bg-blue-500 text-white text-xs rounded-full flex items-center gap-1">
                                  <Star className="w-3 h-3" />
                                  主账号
                                </span>
                              )}
                              {account.is_expired && (
                                <span className="px-2 py-0.5 bg-red-500 text-white text-xs rounded-full">
                                  已过期
                                </span>
                              )}
                              {!account.is_enabled && !account.is_expired && (
                                <span className="px-2 py-0.5 bg-gray-400 text-white text-xs rounded-full">
                                  已禁用
                                </span>
                              )}
                            </div>
                            <p className="text-xs text-gray-500">UID: {account.mid}</p>
                          </div>
                        </div>
                        <div className="flex items-center gap-2">
                          {/* 设为主账号 */}
                          {!account.is_primary && account.is_enabled && !account.is_expired && (
                            <button
                              onClick={() => handleSetPrimary(account.mid)}
                              className="p-2 text-blue-600 hover:bg-blue-100 rounded-lg transition-colors"
                              title="设为主账号"
                            >
                              <Star className="w-4 h-4" />
                            </button>
                          )}
                          {/* 启用/禁用 */}
                          {!account.is_primary && (
                            <button
                              onClick={() => handleToggleEnabled(account.mid, account.is_enabled)}
                              className={`p-2 rounded-lg transition-colors ${account.is_enabled
                                ? 'text-green-600 hover:bg-green-100'
                                : 'text-gray-400 hover:bg-gray-100'
                                }`}
                              title={account.is_enabled ? '禁用账号' : '启用账号'}
                            >
                              {account.is_enabled ? (
                                <ToggleRight className="w-5 h-5" />
                              ) : (
                                <ToggleLeft className="w-5 h-5" />
                              )}
                            </button>
                          )}
                          {/* 删除 */}
                          <button
                            onClick={() => handleRemoveAccount(account.mid)}
                            className="p-2 text-red-600 hover:bg-red-100 rounded-lg transition-colors"
                            title="删除账号"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              {/* 添加账号 */}
              <div className="border-t pt-6">
                <h3 className="text-sm font-medium text-gray-700 mb-3">添加新账号</h3>

                {!showBindQR ? (
                  <button
                    onClick={() => setShowBindQR(true)}
                    className="px-4 py-2 bg-pink-500 text-white rounded-lg hover:bg-pink-600 transition-colors flex items-center gap-2"
                  >
                    <Plus className="w-4 h-4" />
                    扫码添加 B站账号
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
                    <li>可以添加多个 B站账号，系统将使用<strong>主账号</strong>进行视频上传</li>
                    <li>点击 <Star className="w-3 h-3 inline" /> 可将账号设为主账号</li>
                    <li>禁用的账号不会被用于上传，但登录信息会保留</li>
                    <li>账号过期后需要重新扫码登录</li>
                    <li>所有登录信息仅保存在本地服务器，不会上传到云端</li>
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
