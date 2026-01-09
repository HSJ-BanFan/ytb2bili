'use client';

import React from 'react';
import { Crown, Calendar, Zap, Layers } from 'lucide-react';
import type { LicenseStatus } from '@/lib/api';

interface MemberStatusCardProps {
  status: LicenseStatus;
}

export default function MemberStatusCard({ status }: MemberStatusCardProps) {
  const getTierColor = (tier: string) => {
    switch (tier) {
      case 'enterprise':
        return 'from-orange-400 to-amber-600';
      case 'pro':
        return 'from-blue-500 to-indigo-600';
      case 'basic':
        return 'from-emerald-400 to-teal-600';
      default:
        return 'from-gray-400 to-gray-600';
    }
  };

  const getTierIcon = (tier: string) => {
    switch (tier) {
      case 'enterprise':
        return '👑';
      case 'pro':
        return '💎';
      case 'basic':
        return '⭐';
      default:
        return '👤';
    }
  };

  const calculateDaysRemaining = (expiresAt: string | null) => {
    if (!expiresAt) return '永久有效';
    const expiry = new Date(expiresAt);
    const now = new Date();
    const diffTime = expiry.getTime() - now.getTime();
    const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));

    if (diffDays > 25000) return '永久有效'; // For those 2099-12-31 dates
    return diffDays > 0 ? `${diffDays} 天` : '已过期';
  };

  return (
    <div className="bg-white rounded-2xl shadow-lg border border-gray-100 overflow-hidden">
      <div className="grid grid-cols-1 md:grid-cols-4 divide-y md:divide-y-0 md:divide-x divide-gray-100">
        {/* Tier Info */}
        <div className="p-6 flex flex-col items-center justify-center text-center space-y-2 bg-gray-50/30">
          <div className={`w-12 h-12 rounded-full flex items-center justify-center text-2xl shadow-inner bg-gradient-to-br ${getTierColor(status.tier)} text-white`}>
            {getTierIcon(status.tier)}
          </div>
          <div>
            <p className="text-xs text-gray-500 font-medium">当前等级</p>
            <h4 className="text-lg font-bold text-gray-900">{status.tier_name}</h4>
          </div>
        </div>

        {/* Days Remaining */}
        <div className="p-6 flex flex-col items-center justify-center text-center space-y-2">
          <div className="w-10 h-10 rounded-xl bg-blue-50 flex items-center justify-center text-blue-600">
            <Calendar size={20} />
          </div>
          <div>
            <p className="text-xs text-gray-500 font-medium">剩余天数</p>
            <h4 className="text-lg font-bold text-gray-900">{calculateDaysRemaining(status.expires_at)}</h4>
          </div>
        </div>

        {/* Daily Quota */}
        <div className="p-6 flex flex-col items-center justify-center text-center space-y-2">
          <div className="w-10 h-10 rounded-xl bg-purple-50 flex items-center justify-center text-purple-600">
            <Zap size={20} />
          </div>
          <div>
            <p className="text-xs text-gray-500 font-medium">每日配额</p>
            <h4 className="text-lg font-bold text-gray-900">
              {status.limits?.daily_upload_limit === 0 ? '无限' : status.limits?.daily_upload_limit}
            </h4>
          </div>
        </div>

        {/* Batch Limit */}
        <div className="p-6 flex flex-col items-center justify-center text-center space-y-2">
          <div className="w-10 h-10 rounded-xl bg-orange-50 flex items-center justify-center text-orange-600">
            <Layers size={20} />
          </div>
          <div>
            <p className="text-xs text-gray-500 font-medium">批量限制</p>
            <h4 className="text-lg font-bold text-gray-900">
              {status.limits?.max_concurrent_tasks}
            </h4>
          </div>
        </div>
      </div>
    </div>
  );
}
