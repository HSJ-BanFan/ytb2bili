'use client';

import { useState, useEffect } from 'react';
import { membershipApi } from '@/lib/api';
import { MembershipCard, QuotaDisplay, BoostPackCard, UpgradeModal } from '@/components/membership';
import AppLayout from '@/components/layout/AppLayout';
import { useAuth } from '@/hooks/useAuth';
import type { MembershipInfo, TierConfig, AvailableFeatures } from '@/types';

const FEATURE_NAMES: Record<string, string> = {
  ai_translation: 'AI 字幕翻译',
  translation_optimize: '翻译质量优化',
  ai_title_generation: 'AI 标题生成',
  gemini_video_analysis: 'Gemini 视频分析',
  auto_upload: '自动上传',
  priority_queue: '优先队列',
  api_access: 'API 访问',
  custom_template: '自定义模板',
  data_export: '数据导出',
  team_collaboration: '团队协作',
};

export default function MembershipPage() {
  const { user, loading: authLoading, handleLogout } = useAuth();
  const [membership, setMembership] = useState<MembershipInfo | null>(null);
  const [tiers, setTiers] = useState<TierConfig[]>([]);
  const [features, setFeatures] = useState<AvailableFeatures | null>(null);
  const [dataLoading, setDataLoading] = useState(true);
  const [showUpgrade, setShowUpgrade] = useState(false);

  useEffect(() => {
    if (user) {
      fetchData();
    }
  }, [user]);

  const fetchData = async () => {
    try {
      setDataLoading(true);
      const [membershipRes, tiersRes, featuresRes] = await Promise.all([
        membershipApi.getMembershipInfo(),
        membershipApi.getAllTiers(),
        membershipApi.getAvailableFeatures(),
      ]);

      if (membershipRes.code === 0) setMembership(membershipRes.data);
      if (tiersRes.code === 0) setTiers(tiersRes.data);
      if (featuresRes.code === 0) setFeatures(featuresRes.data);
    } catch (err) {
      console.error('获取会员信息失败:', err);
    } finally {
      setDataLoading(false);
    }
  };

  return (
    <AppLayout userName={user?.name} onLogout={handleLogout}>
      {(authLoading || dataLoading) ? (
        <div className="space-y-6">
          <div className="animate-pulse">
            <div className="h-8 bg-gray-200 rounded w-1/4 mb-6"></div>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div className="h-48 bg-gray-200 rounded-lg"></div>
              <div className="h-48 bg-gray-200 rounded-lg"></div>
            </div>
          </div>
        </div>
      ) : (
      <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">会员中心</h1>
        {membership?.tier === 'free' && (
          <button
            onClick={() => setShowUpgrade(true)}
            className="px-4 py-2 bg-gradient-to-r from-purple-500 to-pink-500 text-white rounded-lg hover:from-purple-600 hover:to-pink-600 transition-all"
          >
            升级会员
          </button>
        )}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* 左侧：会员信息和配额 */}
        <div className="lg:col-span-2 space-y-6">
          {/* 会员状态 */}
          <div className="bg-white rounded-lg shadow p-6">
            <h2 className="text-lg font-semibold text-gray-900 mb-4">会员状态</h2>
            {membership && (
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <div className="text-center p-4 bg-gray-50 rounded-lg">
                  <div className="text-2xl mb-1">
                    {membership.tier === 'free' ? '🆓' : 
                     membership.tier === 'basic' ? '⭐' : 
                     membership.tier === 'pro' ? '💎' : '👑'}
                  </div>
                  <div className="text-sm text-gray-500">当前等级</div>
                  <div className="font-semibold text-gray-900">{membership.tier_name}</div>
                </div>
                <div className="text-center p-4 bg-gray-50 rounded-lg">
                  <div className="text-2xl mb-1">📅</div>
                  <div className="text-sm text-gray-500">剩余天数</div>
                  <div className="font-semibold text-gray-900">
                    {membership.days_remaining === -1 ? '永久' : `${membership.days_remaining} 天`}
                  </div>
                </div>
                <div className="text-center p-4 bg-gray-50 rounded-lg">
                  <div className="text-2xl mb-1">🎬</div>
                  <div className="text-sm text-gray-500">每日配额</div>
                  <div className="font-semibold text-gray-900">
                    {membership.daily_limit === -1 ? '无限' : membership.daily_limit}
                  </div>
                </div>
                <div className="text-center p-4 bg-gray-50 rounded-lg">
                  <div className="text-2xl mb-1">📦</div>
                  <div className="text-sm text-gray-500">批量限制</div>
                  <div className="font-semibold text-gray-900">{membership.batch_limit}</div>
                </div>
              </div>
            )}
          </div>

          {/* 可用功能 */}
          <div className="bg-white rounded-lg shadow p-6">
            <h2 className="text-lg font-semibold text-gray-900 mb-4">可用功能</h2>
            {features && (
              <div className="grid grid-cols-2 md:grid-cols-3 gap-3">
                {Object.entries(FEATURE_NAMES).map(([key, name]) => {
                  const isEnabled = features.features.includes(key);
                  return (
                    <div
                      key={key}
                      className={`flex items-center gap-2 p-3 rounded-lg ${
                        isEnabled ? 'bg-green-50 text-green-700' : 'bg-gray-50 text-gray-400'
                      }`}
                    >
                      {isEnabled ? (
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                        </svg>
                      ) : (
                        <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                        </svg>
                      )}
                      <span className="text-sm">{name}</span>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {/* 等级对比 */}
          <div className="bg-white rounded-lg shadow p-6">
            <h2 className="text-lg font-semibold text-gray-900 mb-4">等级对比</h2>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b">
                    <th className="text-left py-3 px-4">等级</th>
                    <th className="text-center py-3 px-4">每日配额</th>
                    <th className="text-center py-3 px-4">批量限制</th>
                    <th className="text-center py-3 px-4">优先级</th>
                    <th className="text-center py-3 px-4">价格</th>
                  </tr>
                </thead>
                <tbody>
                  {tiers.map((tier) => (
                    <tr 
                      key={tier.tier} 
                      className={`border-b ${tier.tier === membership?.tier ? 'bg-blue-50' : ''}`}
                    >
                      <td className="py-3 px-4 font-medium">
                        {tier.tier === 'free' ? '🆓' : 
                         tier.tier === 'basic' ? '⭐' : 
                         tier.tier === 'pro' ? '💎' : '👑'} {tier.name}
                        {tier.tier === membership?.tier && (
                          <span className="ml-2 text-xs bg-blue-100 text-blue-700 px-2 py-0.5 rounded">当前</span>
                        )}
                      </td>
                      <td className="text-center py-3 px-4">
                        {tier.daily_limit === -1 ? '无限' : tier.daily_limit}
                      </td>
                      <td className="text-center py-3 px-4">{tier.batch_limit}</td>
                      <td className="text-center py-3 px-4">{tier.priority}</td>
                      <td className="text-center py-3 px-4">
                        {tier.price ? `¥${tier.price}/月` : '免费'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>

        {/* 右侧：配额和加油包 */}
        <div className="space-y-6">
          <QuotaDisplay />
          <BoostPackCard onPurchaseSuccess={fetchData} />
        </div>
      </div>

      <UpgradeModal 
        isOpen={showUpgrade} 
        onClose={() => setShowUpgrade(false)}
        currentTier={membership?.tier}
      />
      </div>
      )}
    </AppLayout>
  );
}
