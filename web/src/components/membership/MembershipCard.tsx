'use client';

import { useState, useEffect } from 'react';
import { membershipApi } from '@/lib/api';
import type { MembershipInfo, QuotaInfo, TIER_COLORS, TIER_ICONS } from '@/types';

const TIER_COLORS_MAP: Record<string, { bg: string; text: string; border: string }> = {
  basic: { bg: 'bg-blue-100', text: 'text-blue-700', border: 'border-blue-300' },
  pro: { bg: 'bg-purple-100', text: 'text-purple-700', border: 'border-purple-300' },
  enterprise: { bg: 'bg-amber-100', text: 'text-amber-700', border: 'border-amber-300' },
};

const TIER_ICONS_MAP: Record<string, string> = {
  basic: '⭐',
  pro: '💎',
  enterprise: '👑',
};

interface MembershipCardProps {
  onUpgradeClick?: () => void;
}

export default function MembershipCard({ onUpgradeClick }: MembershipCardProps) {
  const [membership, setMembership] = useState<MembershipInfo | null>(null);
  const [quota, setQuota] = useState<QuotaInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchMembershipData();
  }, []);

  const fetchMembershipData = async () => {
    try {
      setLoading(true);
      const [membershipRes, quotaRes] = await Promise.all([
        membershipApi.getMembershipInfo(),
        membershipApi.getQuotaInfo(),
      ]);

      if (membershipRes.code === 0) {
        setMembership(membershipRes.data);
      }
      if (quotaRes.code === 0) {
        setQuota(quotaRes.data);
      }
    } catch (err: any) {
      setError(err.message || '获取会员信息失败');
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <div className="bg-white rounded-lg shadow p-4 animate-pulse">
        <div className="h-6 bg-gray-200 rounded w-1/3 mb-4"></div>
        <div className="h-4 bg-gray-200 rounded w-2/3 mb-2"></div>
        <div className="h-4 bg-gray-200 rounded w-1/2"></div>
      </div>
    );
  }

  if (error || !membership) {
    return (
      <div className="bg-white rounded-lg shadow p-4">
        <p className="text-gray-500 text-sm">{error || '未登录'}</p>
      </div>
    );
  }

  const tierColors = TIER_COLORS_MAP[membership.tier] || TIER_COLORS_MAP.basic;
  const tierIcon = TIER_ICONS_MAP[membership.tier] || '⭐';

  return (
    <div className={`bg-white rounded-lg shadow overflow-hidden border-l-4 ${tierColors.border}`}>
      {/* 头部 */}
      <div className={`px-4 py-3 ${tierColors.bg}`}>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="text-2xl">{tierIcon}</span>
            <div>
              <h3 className={`font-semibold ${tierColors.text}`}>{membership.tier_name}</h3>
              {membership.expires_at && !membership.is_expired && (
                <p className="text-xs text-gray-500">
                  {membership.days_remaining > 0 
                    ? `${membership.days_remaining} 天后到期` 
                    : '永久有效'}
                </p>
              )}
            </div>
          </div>
          {membership.tier === 'basic' && onUpgradeClick && (
            <button
              onClick={onUpgradeClick}
              className="px-3 py-1 text-sm bg-gradient-to-r from-purple-500 to-pink-500 text-white rounded-full hover:from-purple-600 hover:to-pink-600 transition-all"
            >
              升级会员
            </button>
          )}
        </div>
      </div>

      {/* 配额信息 */}
      {quota && (
        <div className="px-4 py-3">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm text-gray-600">今日配额</span>
            <span className="text-sm font-medium">
              {quota.is_unlimited ? (
                <span className="text-green-600">无限制</span>
              ) : (
                <>
                  <span className="text-blue-600">{quota.daily_remaining}</span>
                  <span className="text-gray-400"> / {quota.daily_limit}</span>
                </>
              )}
            </span>
          </div>

          {!quota.is_unlimited && (
            <>
              {/* 进度条 */}
              <div className="w-full bg-gray-200 rounded-full h-2 mb-2">
                <div
                  className="bg-blue-500 h-2 rounded-full transition-all"
                  style={{
                    width: `${Math.min(100, (quota.daily_used / quota.daily_limit) * 100)}%`,
                  }}
                ></div>
              </div>

              {/* 加油包 */}
              {quota.boost_pack_remaining > 0 && (
                <div className="flex items-center justify-between text-xs">
                  <span className="text-gray-500">🚀 加油包剩余</span>
                  <span className="text-orange-600 font-medium">
                    {quota.boost_pack_remaining} 个视频
                  </span>
                </div>
              )}
            </>
          )}
        </div>
      )}

      {/* 快捷操作 */}
      <div className="px-4 py-2 bg-gray-50 border-t flex justify-between text-xs">
        <span className="text-gray-500">
          批量限制: {membership.batch_limit} 个
        </span>
        {membership.priority > 0 && (
          <span className="text-purple-600">
            优先级 +{membership.priority}
          </span>
        )}
      </div>
    </div>
  );
}
