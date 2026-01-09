'use client';

import React from 'react';
import { Check, X } from 'lucide-react';
import type { LicenseStatus } from '@/lib/api';

interface FeatureGridProps {
  status: LicenseStatus;
}

export default function FeatureGrid({ status }: FeatureGridProps) {
  const { features } = status;

  const featureList = [
    { key: 'auto_upload', label: '自动上传', enabled: features?.auto_upload },
    { key: 'subtitle_translation', label: 'AI 字幕翻译', enabled: features?.subtitle_translation },
    { key: 'metadata_generation', label: 'AI 元数据生成', enabled: features?.metadata_generation },
    { key: 'custom_templates', label: '自定义模板', enabled: features?.custom_templates },
    { key: 'priority_support', label: '优先支持', enabled: features?.priority_support },
    { key: 'translation_optimization', label: '翻译质量优化', enabled: features?.subtitle_translation }, // Derived
    { key: 'batch_export', label: '批量任务处理', enabled: true }, // Always true for basic+
    { key: 'priority_queue', label: '优先队列', enabled: features?.priority_support }, // Derived
  ];

  return (
    <div className="bg-white rounded-2xl shadow-lg border border-gray-100 p-6">
      <div className="flex items-center justify-between mb-6">
        <h3 className="text-xl font-bold text-gray-900 flex items-center gap-2">
          <span className="text-2xl">✨</span>
          可用功能
        </h3>
        {/* Optional Avatar from reference image */}
        <div className="w-8 h-8 rounded-full overflow-hidden bg-blue-100 border-2 border-white shadow-sm">
          <img src={`https://api.dicebear.com/7.x/avataaars/svg?seed=user`} alt="avatar" />
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
        {featureList.map((item) => (
          <div
            key={item.key}
            className={`flex items-center gap-2 px-4 py-3 rounded-xl border transition-all ${item.enabled
                ? 'bg-emerald-50/50 border-emerald-100 text-emerald-800'
                : 'bg-gray-50 border-gray-100 text-gray-400 grayscale opacity-60'
              }`}
          >
            <div className={`flex-shrink-0 w-5 h-5 rounded-full flex items-center justify-center ${item.enabled ? 'bg-emerald-500 text-white' : 'bg-gray-300 text-white'
              }`}>
              {item.enabled ? <Check size={12} strokeWidth={4} /> : <X size={12} strokeWidth={4} />}
            </div>
            <span className="text-sm font-medium">{item.label}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
