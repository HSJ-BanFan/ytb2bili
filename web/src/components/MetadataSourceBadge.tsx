import React from 'react';

interface MetadataSourceBadgeProps {
  source?: 'original' | 'ai_generated' | 'user_edited';
  size?: 'sm' | 'md' | 'lg';
}

export const MetadataSourceBadge: React.FC<MetadataSourceBadgeProps> = ({
  source,
  size = 'md'
}) => {
  if (!source) return null;

  const config = {
    original: {
      label: '📹 原始',
      color: 'bg-gray-100 text-gray-800',
      icon: '📹',
      description: 'YouTube原始数据'
    },
    ai_generated: {
      label: '✨ AI生成',
      color: 'bg-purple-100 text-purple-800',
      icon: '✨',
      description: 'AI增强生成'
    },
    user_edited: {
      label: '📝 用户编辑',
      color: 'bg-blue-100 text-blue-800',
      icon: '📝',
      description: '用户手动编辑'
    }
  }[source];

  const sizeClasses = {
    sm: 'text-xs px-2 py-0.5',
    md: 'text-sm px-2.5 py-1',
    lg: 'text-base px-3 py-1.5'
  }[size];

  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full font-medium ${config.color} ${sizeClasses}`}
      title={config.description}
    >
      {config.label}
    </span>
  );
};

interface MetadataComparisonProps {
  original?: string;
  generated?: string;
  upload?: string;
  source?: 'original' | 'ai_generated' | 'user_edited';
  label: string;
  maxLength?: number;
}

export const MetadataComparison: React.FC<MetadataComparisonProps> = ({
  original,
  generated,
  upload,
  source,
  label,
  maxLength = 100
}) => {
  const truncate = (str?: string) => {
    if (!str) return '';
    if (str.length <= maxLength) return str;
    return str.substring(0, maxLength) + '...';
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <label className="text-sm font-medium text-gray-700">{label}</label>
        {source && <MetadataSourceBadge source={source} size="sm" />}
      </div>

      <div className="grid grid-cols-3 gap-2 text-xs">
        {/* 原始数据 */}
        <div className="p-2 bg-gray-50 rounded border">
          <div className="font-medium text-gray-600 mb-1">原始数据</div>
          <div className="text-gray-500 break-all">{truncate(original) || '-'}</div>
        </div>

        {/* AI生成 */}
        <div className="p-2 bg-purple-50 rounded border">
          <div className="font-medium text-purple-600 mb-1">AI生成</div>
          <div className="text-purple-700 break-all">{truncate(generated) || '-'}</div>
        </div>

        {/* 最终版本 */}
        <div className="p-2 bg-blue-50 rounded border border-blue-200">
          <div className="font-medium text-blue-600 mb-1">最终版本</div>
          <div className="text-blue-700 break-all font-medium">{truncate(upload) || '-'}</div>
        </div>
      </div>
    </div>
  );
};

export default MetadataSourceBadge;
