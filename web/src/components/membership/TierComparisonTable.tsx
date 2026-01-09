'use client';

import React from 'react';
import { Star, ShieldCheck, Rocket, Zap } from 'lucide-react';

interface TierComparisonTableProps {
  currentTier: string;
}

export default function TierComparisonTable({ currentTier }: TierComparisonTableProps) {
  const tiers = [

    {
      id: 'basic',
      name: '基础版 (默认)',
      icon: <Zap className="text-emerald-500" size={16} />,
      dailyQuota: '10',
      batchLimit: '2',
      duration: '60分钟',
      storage: '10GB',
      price: '免费',
    },
    {
      id: 'pro',
      name: '专业版',
      icon: <Rocket className="text-blue-500" size={16} />,
      dailyQuota: '50',
      batchLimit: '5',
      duration: '180分钟',
      storage: '50GB',
      price: '¥30/月',
    },
    {
      id: 'enterprise',
      name: '企业版',
      icon: <ShieldCheck className="text-orange-500" size={16} />,
      dailyQuota: '无限',
      batchLimit: '10',
      duration: '无限',
      storage: '无限',
      price: '¥99/月',
    },
  ];

  return (
    <div className="bg-white rounded-2xl shadow-xl border border-gray-100 overflow-hidden">
      <div className="px-6 py-5 border-b border-gray-100">
        <h3 className="text-xl font-bold text-gray-900 flex items-center gap-2">
          <span className="text-2xl">📊</span>
          等级对比
        </h3>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm text-center">
          <thead className="bg-gray-50/50 text-gray-500 font-medium uppercase tracking-wider text-xs">
            <tr>
              <th className="px-6 py-4 text-left">等级</th>
              <th className="px-6 py-4">每日配额</th>
              <th className="px-6 py-4">批量限制</th>
              <th className="px-6 py-4">视频时长</th>
              <th className="px-6 py-4">存储空间</th>
              <th className="px-6 py-4">价格</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {tiers.map((tier) => {
              const isCurrent = tier.id === currentTier;
              return (
                <tr
                  key={tier.id}
                  className={`transition-colors ${isCurrent ? 'bg-blue-50/30' : 'hover:bg-gray-50/50'}`}
                >
                  <td className="px-6 py-5 text-left">
                    <div className="flex items-center gap-3">
                      <div className={`w-8 h-8 rounded-lg flex items-center justify-center ${isCurrent ? 'bg-blue-100 text-blue-600' : 'bg-gray-100 text-gray-400'}`}>
                        {tier.icon}
                      </div>
                      <div>
                        <span className={`font-bold ${isCurrent ? 'text-blue-700' : 'text-gray-700'}`}>
                          {tier.name}
                        </span>
                        {isCurrent && (
                          <span className="ml-2 px-1.5 py-0.5 rounded bg-blue-100 text-blue-600 text-[10px] font-black uppercase">
                            当前
                          </span>
                        )}
                      </div>
                    </div>
                  </td>
                  <td className={`px-6 py-5 font-semibold ${tier.dailyQuota === '无限' ? 'text-emerald-600' : 'text-gray-600'}`}>
                    {tier.dailyQuota}
                  </td>
                  <td className="px-6 py-5 font-semibold text-gray-600">
                    {tier.batchLimit}
                  </td>
                  <td className="px-6 py-5 font-semibold text-gray-600">
                    {tier.duration}
                  </td>
                  <td className="px-6 py-5 font-semibold text-gray-600">
                    {tier.storage}
                  </td>
                  <td className={`px-6 py-5 font-bold ${isCurrent ? 'text-blue-600' : 'text-gray-900'}`}>
                    {tier.price}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
