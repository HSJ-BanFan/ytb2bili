'use client';

import { useState, useEffect } from 'react';
import { licenseApi } from '@/lib/api';
import type { LicenseStatus, LicenseActivation } from '@/lib/api';
import AppLayout from '@/components/layout/AppLayout';
import { useAuth } from '@/hooks/useAuth';
import MemberStatusCard from '@/components/membership/MemberStatusCard';
import FeatureGrid from '@/components/membership/FeatureGrid';
import TierComparisonTable from '@/components/membership/TierComparisonTable';

export default function MembershipPage() {
  const { user, loading: authLoading, handleLogout } = useAuth();
  const [status, setStatus] = useState<LicenseStatus | null>(null);
  const [licenseList, setLicenseList] = useState<LicenseActivation[]>([]);
  const [loading, setLoading] = useState(true);

  // 激活许可证状态
  const [licenseKey, setLicenseKey] = useState('');
  const [activating, setActivating] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  useEffect(() => {
    if (user) {
      fetchData();
    }
  }, [user]);

  const fetchData = async () => {
    try {
      setLoading(true);
      const [statusRes, listRes] = await Promise.all([
        licenseApi.getStatus(),
        licenseApi.getLicenseList(),
      ]);

      if (statusRes.code === 0) setStatus(statusRes.data);
      if (listRes.code === 0) setLicenseList(listRes.data || []);
    } catch (err) {
      console.error('获取会员信息失败:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleActivate = async () => {
    if (!licenseKey.trim()) return;

    try {
      setActivating(true);
      setMessage(null);
      const res = await licenseApi.activateLicense(licenseKey);

      if (res.code === 0) {
        setMessage({ type: 'success', text: `激活成功！当前等级: ${res.data.tier}，有效期至: ${res.data.expires_at}` });
        setLicenseKey('');
        fetchData(); // 刷新数据
      } else {
        setMessage({ type: 'error', text: res.message || '激活失败' });
      }
    } catch (err: any) {
      setMessage({ type: 'error', text: err.message || '激活失败' });
    } finally {
      setActivating(false);
    }
  };

  const getTierIcon = (tier: string) => {
    switch (tier) {
      case 'basic': return '⭐';
      case 'pro': return '💎';
      case 'enterprise': return '👑';
      default: return '📦';
    }
  };

  const getTierName = (tier: string) => {
    switch (tier) {
      case 'basic': return '基础版';
      case 'pro': return '专业版';
      case 'enterprise': return '企业版';
      default: return tier;
    }
  };

  return (
    <AppLayout userName={user?.name} onLogout={handleLogout}>
      {(authLoading || loading) ? (
        <div className="space-y-6 animate-pulse">
          <div className="h-48 bg-gray-200 rounded-2xl"></div>
          <div className="h-64 bg-gray-200 rounded-2xl"></div>
          <div className="h-96 bg-gray-200 rounded-2xl"></div>
        </div>
      ) : (
        <div className="space-y-8">
          {/* 页面标题 */}
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-3xl font-bold text-gray-900">会员中心</h1>
              <p className="text-gray-600 mt-1">管理您的会员等级和权益</p>
            </div>
          </div>

          {/* 主要内容网格 */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
            {/* 左列 */}
            <div className="space-y-8">
              {/* 会员状态卡片 */}
              {status && <MemberStatusCard status={status} />}

              {/* 激活许可证 */}
              <div className="bg-white rounded-2xl shadow-lg border border-gray-200 p-6">
                <h3 className="text-xl font-bold text-gray-900 mb-6 flex items-center gap-2">
                  <span className="text-2xl">🔑</span>
                  激活新许可证
                </h3>

                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      许可证密钥
                    </label>
                    <input
                      type="text"
                      value={licenseKey}
                      onChange={(e) => setLicenseKey(e.target.value)}
                      placeholder="请输入您的许可证密钥..."
                      className="w-full px-4 py-3 rounded-xl border border-gray-300 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-all"
                    />
                  </div>

                  {message && (
                    <div
                      className={`p-4 rounded-xl text-sm ${
                        message.type === 'success'
                          ? 'bg-green-50 text-green-700 border border-green-200'
                          : 'bg-red-50 text-red-700 border border-red-200'
                      }`}
                    >
                      {message.text}
                    </div>
                  )}

                  <button
                    onClick={handleActivate}
                    disabled={activating || !licenseKey.trim()}
                    className={`w-full py-3 rounded-xl text-white font-medium transition-all ${
                      activating || !licenseKey.trim()
                        ? 'bg-gray-400 cursor-not-allowed'
                        : 'bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-700 hover:to-indigo-700 shadow-lg hover:shadow-xl transform hover:-translate-y-0.5'
                    }`}
                  >
                    {activating ? '激活中...' : '立即激活'}
                  </button>

                  <p className="text-xs text-gray-500 text-center">
                    激活后，您的会员时长将会相应增加
                  </p>
                </div>
              </div>
            </div>

            {/* 右列 */}
            <div className="space-y-8">
              {/* 功能网格 */}
              {status && <FeatureGrid status={status} />}

              {/* 激活记录 */}
              <div className="bg-white rounded-2xl shadow-lg border border-gray-200 overflow-hidden">
                <div className="px-6 py-4 border-b border-gray-200 bg-gray-50">
                  <h3 className="text-lg font-semibold text-gray-900 flex items-center gap-2">
                    <span className="text-xl">📋</span>
                    激活记录
                  </h3>
                </div>

                {licenseList.length === 0 ? (
                  <div className="p-8 text-center text-gray-500">
                    <div className="text-4xl mb-2">📭</div>
                    <p>暂无激活记录</p>
                  </div>
                ) : (
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead className="bg-gray-50 text-gray-600 font-medium">
                        <tr>
                          <th className="px-6 py-3 text-left">许可证密钥</th>
                          <th className="px-6 py-3 text-left">等级</th>
                          <th className="px-6 py-3 text-left">计划</th>
                          <th className="px-6 py-3 text-left">激活时间</th>
                          <th className="px-6 py-3 text-left">状态</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-gray-100">
                        {licenseList.map((item) => (
                          <tr key={item.id} className="hover:bg-gray-50 transition-colors">
                            <td className="px-6 py-4 font-mono text-gray-600 text-xs">
                              {item.license_key}
                            </td>
                            <td className="px-6 py-4">
                              <div className="flex items-center gap-2">
                                <span>{getTierIcon(item.tier)}</span>
                                <span className="text-gray-700">
                                  {getTierName(item.tier)}
                                </span>
                              </div>
                            </td>
                            <td className="px-6 py-4 text-gray-600">{item.plan}</td>
                            <td className="px-6 py-4 text-gray-600">
                              {new Date(item.activated_at).toLocaleString()}
                            </td>
                            <td className="px-6 py-4">
                              <span
                                className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${
                                  item.is_active
                                    ? 'bg-green-100 text-green-800'
                                    : 'bg-gray-100 text-gray-800'
                                }`}
                              >
                                {item.is_active ? '✅ 已生效' : '❌ 已失效'}
                              </span>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* 等级对比表 - 全宽 */}
          <div className="pt-4">
            {status && <TierComparisonTable currentTier={status.tier} />}
          </div>
        </div>
      )}
    </AppLayout>
  );
}
