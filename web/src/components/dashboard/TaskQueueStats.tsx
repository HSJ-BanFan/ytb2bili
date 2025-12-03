'use client';

import { useState, useEffect } from 'react';
import { RefreshCw, Clock, Play, CheckCircle, Upload, AlertCircle, Trash2, RotateCcw, ExternalLink } from 'lucide-react';

interface Video {
  id: number;
  video_id: string;
  title: string;
  url?: string;
  status: string;
  created_at: string;
  updated_at: string;
  task_steps?: TaskStep[];
  bili_bvid?: string;
}

interface TaskStep {
  step_name: string;
  step_order: number;
  status: string;
  start_time: string;
  end_time: string;
  error_msg: string;
  can_retry: boolean;
}

type TabType = 'all' | 'pending' | 'preparing' | 'ready' | 'uploading' | 'completed' | 'failed';

interface TaskQueueStatsProps {
  onVideoSelect?: (videoId: string) => void;
}

export default function TaskQueueStats({ onVideoSelect }: TaskQueueStatsProps) {
  const [videos, setVideos] = useState<Video[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<TabType>('all');
  const [refreshing, setRefreshing] = useState(false);
  const [expandedVideoId, setExpandedVideoId] = useState<number | null>(null);
  const [detailedVideo, setDetailedVideo] = useState<Video | null>(null);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);
  const itemsPerPage = 10;

  useEffect(() => {
    fetchVideos();
    const interval = setInterval(fetchVideos, 30000);
    return () => clearInterval(interval);
  }, []);

  const fetchVideos = async () => {
    try {
      setRefreshing(true);
      const response = await fetch('/api/v1/videos?page=1&limit=1000');
      const data = await response.json();
      
      console.log('视频数据响应:', data); // 调试日志
      
      if ((data.code === 0 || data.code === 200) && data.data) {
        setVideos(data.data.videos || []);
        console.log('成功加载视频:', data.data.videos.length); // 调试日志
      }
    } catch (error) {
      console.error('获取视频列表失败:', error);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  const handleToggleDetails = async (videoId: number) => {
    if (expandedVideoId === videoId) {
      setExpandedVideoId(null);
      setDetailedVideo(null);
    } else {
      setExpandedVideoId(videoId);
      setIsDetailLoading(true);
      try {
        const response = await fetch(`/api/v1/videos/${videoId}`);
        const data = await response.json();
        if (data.code === 200 || data.code === 0) {
          setDetailedVideo(data.data);
        } else {
          console.error('Failed to fetch video details:', data.message);
        }
      } catch (error) {
        console.error('Error fetching video details:', error);
      } finally {
        setIsDetailLoading(false);
      }
    }
  };

  const handleRetryStep = async (videoId: number, stepName: string) => {
    try {
      const response = await fetch(`/api/v1/videos/${videoId}/steps/${stepName}/retry`, {
        method: 'POST',
      });
      const data = await response.json();
      if (data.code === 200 || data.code === 0) {
        // 刷新详情
        handleToggleDetails(videoId);
      } else {
        console.error('Failed to retry step:', data.message);
      }
    } catch (error) {
      console.error('Error retrying step:', error);
    }
  };

  const handleResetAllFailed = async (videoId: number, resetAll: boolean = false) => {
    try {
      const endpoint = resetAll 
        ? `/api/v1/videos/${videoId}/steps/reset-all`
        : `/api/v1/videos/${videoId}/steps/reset-failed`;
      const response = await fetch(endpoint, {
        method: 'POST',
      });
      const data = await response.json();
      if (data.code === 200 || data.code === 0) {
        const resetCount = data.data?.reset_count || 0;
        if (resetCount > 0) {
          alert(`已重置 ${resetCount} 个步骤，任务将自动重新执行`);
        } else {
          alert('没有需要重置的步骤');
        }
        // 刷新详情和列表
        handleToggleDetails(videoId);
        fetchVideos();
      } else {
        alert(`重置失败：${data.message}`);
      }
    } catch (error) {
      console.error('Error resetting steps:', error);
      alert('网络错误，重置失败');
    }
  };

  const handleDeleteVideo = async (videoId: string, videoTitle: string, e: React.MouseEvent) => {
    e.stopPropagation(); // 阻止事件冒泡，避免触发详情展开
    
    if (!confirm(`确定要删除视频 "${videoTitle || videoId}" 吗？\n\n此操作将删除：\n- 所有任务步骤\n- 视频文件和字幕文件\n- 数据库记录\n\n此操作无法恢复！`)) {
      return;
    }

    try {
      const response = await fetch(`/api/v1/videos/${videoId}`, {
        method: 'DELETE',
      });

      const data = await response.json();

      if (data.code === 200) {
        alert('✅ 删除成功！');
        // 刷新列表
        fetchVideos();
      } else {
        alert(`❌ 删除失败：${data.message}`);
      }
    } catch (error) {
      console.error('删除视频失败:', error);
      alert('❌ 网络错误，删除失败');
    }
  };
  
  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed':
        return 'bg-green-100 text-green-800';
      case 'failed':
        return 'bg-red-100 text-red-800';
      case 'running':
        return 'bg-blue-100 text-blue-800';
      case 'pending':
      default:
        return 'bg-gray-100 text-gray-800';
    }
  };

  // 状态映射
  const getStatusInfo = (status: string) => {
    const statusMap: { [key: string]: { label: string; color: string; icon: any; category: TabType } } = {
      '001': { label: '待处理', color: 'bg-gray-100 text-gray-700', icon: Clock, category: 'pending' },
      '002': { label: '处理中', color: 'bg-blue-100 text-blue-700', icon: Play, category: 'preparing' },
      '200': { label: '准备就绪', color: 'bg-green-100 text-green-700', icon: CheckCircle, category: 'ready' },
      '201': { label: '上传视频中', color: 'bg-purple-100 text-purple-700', icon: Upload, category: 'uploading' },
      '299': { label: '上传失败', color: 'bg-red-100 text-red-700', icon: AlertCircle, category: 'failed' },
      '300': { label: '视频已上传', color: 'bg-cyan-100 text-cyan-700', icon: CheckCircle, category: 'uploading' },
      '301': { label: '上传字幕中', color: 'bg-indigo-100 text-indigo-700', icon: Upload, category: 'uploading' },
      '399': { label: '字幕上传失败', color: 'bg-orange-100 text-orange-700', icon: AlertCircle, category: 'failed' },
      '400': { label: '全部完成', color: 'bg-emerald-100 text-emerald-700', icon: CheckCircle, category: 'completed' },
      '999': { label: '任务失败', color: 'bg-red-100 text-red-700', icon: AlertCircle, category: 'failed' },
    };
    return statusMap[status] || { label: '未知', color: 'bg-gray-100 text-gray-700', icon: AlertCircle, category: 'all' };
  };

  // 获取当前阶段描述
  const getStageDescription = (status: string) => {
    const stageMap: { [key: string]: string } = {
      '001': '等待开始处理',
      '002': '正在执行准备任务链（下载视频→生成字幕→翻译字幕→生成元数据）',
      '200': '准备阶段完成，等待视频上传（每小时上传1个）',
      '201': '正在上传视频到Bilibili',
      '299': '视频上传失败，需要重试',
      '300': '视频已上传，等待1小时后上传字幕',
      '301': '正在上传字幕到Bilibili',
      '399': '字幕上传失败，需要重试',
      '400': '所有任务已完成',
      '999': '准备阶段失败，需要检查任务步骤',
    };
    return stageMap[status] || '未知状态';
  };

  // 分类视频
  const categorizeVideos = () => {
    return {
      pending: videos.filter(v => v.status === '001'),
      preparing: videos.filter(v => v.status === '002'),
      ready: videos.filter(v => v.status === '200'),
      uploading: videos.filter(v => ['201', '300', '301'].includes(v.status)),
      completed: videos.filter(v => v.status === '400'),
      failed: videos.filter(v => ['299', '399', '999'].includes(v.status)),
    };
  };

  const categories = categorizeVideos();
  const filteredVideos = activeTab === 'all' ? videos : categories[activeTab as keyof typeof categories];

  // 分页逻辑
  const totalItems = filteredVideos.length;
  const totalPages = Math.ceil(totalItems / itemsPerPage);
  const paginatedVideos = filteredVideos.slice(
    (currentPage - 1) * itemsPerPage,
    currentPage * itemsPerPage
  );

  const handlePageChange = (page: number) => {
    if (page >= 1 && page <= totalPages) {
      setCurrentPage(page);
    }
  };

  const tabs = [
    { key: 'all', label: '全部', count: videos.length },
    { key: 'pending', label: '待处理', count: categories.pending.length },
    { key: 'preparing', label: '准备中', count: categories.preparing.length },
    { key: 'ready', label: '准备就绪', count: categories.ready.length },
    { key: 'uploading', label: '上传中', count: categories.uploading.length },
    { key: 'completed', label: '已完成', count: categories.completed.length },
    { key: 'failed', label: '失败', count: categories.failed.length },
  ];

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <div className="inline-block w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full animate-spin mb-4"></div>
          <p className="text-gray-600">加载任务数据...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* 标题栏 */}
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold text-gray-900">任务管理</h2>
        <button
          onClick={fetchVideos}
          disabled={refreshing}
          className="flex items-center space-x-2 px-4 py-2 text-sm text-gray-600 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition-colors disabled:opacity-50"
        >
          <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin' : ''}`} />
          <span>刷新</span>
        </button>
      </div>

      {/* 标签页 */}
      <div className="border-b border-gray-200">
        <div className="flex space-x-8">
          {tabs.map(tab => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key as TabType)}
              className={`pb-3 px-1 text-sm font-medium border-b-2 transition-colors ${
                activeTab === tab.key
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              }`}
            >
              {tab.label}
              {tab.count > 0 && (
                <span className={`ml-2 text-xs px-2 py-0.5 rounded ${
                  activeTab === tab.key ? 'bg-blue-100' : 'bg-gray-100'
                }`}>
                  {tab.count}
                </span>
              )}
            </button>
          ))}
        </div>
      </div>

      {/* 视频处理任务链 */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-semibold text-gray-900">视频处理任务链</h3>
        </div>
        <div className="bg-white rounded-lg border border-gray-200">
          {paginatedVideos.length === 0 ? (
            <div className="p-12 text-center text-gray-500">
              暂无任务数据
            </div>
          ) : (
            <div className="divide-y divide-gray-200">
              {paginatedVideos.map(video => {
                const statusInfo = getStatusInfo(video.status);
                const Icon = statusInfo.icon;
                return (
                  <div key={video.id}>
                    <div 
                      className="p-4 hover:bg-gray-50 transition-colors cursor-pointer"
                      onClick={() => handleToggleDetails(video.id)}
                    >
                      <div className="flex items-start justify-between">
                        <div className="flex-1">
                          <div className="flex items-center space-x-3 mb-2">
                            <h4 className="font-medium text-gray-900">
                              {video.title || video.video_id}
                            </h4>
                            <span className={`flex items-center space-x-1 text-xs px-2 py-1 rounded ${statusInfo.color}`}>
                              <Icon className="w-3 h-3" />
                              <span>{statusInfo.label}</span>
                            </span>
                            {video.bili_bvid && (
                              <a
                                href={`https://www.bilibili.com/video/${video.bili_bvid}`}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="text-xs text-blue-600 hover:underline"
                              >
                                {video.bili_bvid}
                              </a>
                            )}
                          </div>
                          <p className="text-sm text-gray-600 mb-3">
                            {getStageDescription(video.status)}
                          </p>
                          <div className="flex items-center space-x-6 text-xs text-gray-500">
                            <span>视频ID: {video.video_id}</span>
                            {video.url && (
                              <a
                                href={video.url}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="flex items-center text-blue-600 hover:text-blue-800 hover:underline"
                                onClick={(e) => e.stopPropagation()}
                              >
                                <ExternalLink className="w-3 h-3 mr-1" />
                                原视频
                              </a>
                            )}
                            <span>创建: {new Date(video.created_at).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}</span>
                            <span>更新: {new Date(video.updated_at).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}</span>
                          </div>
                        </div>
                        <div className="ml-4 flex items-center space-x-2">
                          <button 
                            onClick={(e) => handleDeleteVideo(video.video_id, video.title, e)}
                            className="px-3 py-2 text-sm text-red-600 hover:bg-red-50 rounded transition-colors flex items-center space-x-1"
                            title="删除视频"
                          >
                            <Trash2 className="w-4 h-4" />
                            <span>删除</span>
                          </button>
                          <button 
                            className="px-4 py-2 text-sm text-blue-600 hover:bg-blue-50 rounded transition-colors"
                          >
                            {expandedVideoId === video.id ? '收起详情' : '查看详情'}
                          </button>
                        </div>
                      </div>
                    </div>
                    {expandedVideoId === video.id && (
                      <div className="p-4 border-t border-gray-200 bg-gray-50">
                        {isDetailLoading ? (
                          <div className="text-center text-gray-500">加载任务步骤...</div>
                        ) : detailedVideo && detailedVideo.task_steps ? (
                          <TaskStepDetail 
                            steps={detailedVideo.task_steps} 
                            onRetry={(stepName) => handleRetryStep(video.id, stepName)}
                            onResetAllFailed={(resetAll) => handleResetAllFailed(video.id, resetAll)}
                          />
                        ) : (
                          <div className="text-center text-gray-500">无任务步骤信息</div>
                        )}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>

      {/* 分页控件 */}
      {totalPages > 1 && (
        <div className="flex justify-center items-center space-x-2 mt-6">
          <button
            onClick={() => handlePageChange(currentPage - 1)}
            disabled={currentPage === 1}
            className="px-3 py-1 text-sm text-gray-600 bg-white border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50"
          >
            上一页
          </button>
          <span className="text-sm text-gray-700">
            第 {currentPage} 页 / 共 {totalPages} 页
          </span>
          <button
            onClick={() => handlePageChange(currentPage + 1)}
            disabled={currentPage === totalPages}
            className="px-3 py-1 text-sm text-gray-600 bg-white border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50"
          >
            下一页
          </button>
        </div>
      )}

      {/* 自动化调度说明 */}
      <div>
        <h3 className="text-lg font-semibold text-gray-900 mb-4">自动化调度策略</h3>
        <div className="bg-white rounded-lg border border-gray-200">
          <div className="p-6 space-y-4">
            <div className="flex items-start space-x-3">
              <div className="flex-shrink-0 w-8 h-8 bg-blue-100 rounded-lg flex items-center justify-center">
                <Play className="w-4 h-4 text-blue-600" />
              </div>
              <div className="flex-1">
                <h4 className="font-medium text-gray-900 mb-1">准备阶段任务链</h4>
                <p className="text-sm text-gray-600">
                  每5秒检查一次待处理任务，依次执行：下载视频 → 生成字幕 → 翻译字幕 → 生成元数据
                </p>
                <div className="mt-2 text-xs text-gray-500">
                  当前准备中: {categories.preparing.length} 个 | 待处理: {categories.pending.length} 个
                </div>
              </div>
            </div>

            <div className="flex items-start space-x-3">
              <div className="flex-shrink-0 w-8 h-8 bg-purple-100 rounded-lg flex items-center justify-center">
                <Upload className="w-4 h-4 text-purple-600" />
              </div>
              <div className="flex-1">
                <h4 className="font-medium text-gray-900 mb-1">视频上传调度</h4>
                <p className="text-sm text-gray-600">
                  每5分钟检查一次，每小时自动上传1个准备就绪的视频到Bilibili
                </p>
                <div className="mt-2 text-xs text-gray-500">
                  准备就绪: {categories.ready.length} 个 | 上传中: {categories.uploading.length} 个
                </div>
              </div>
            </div>

            <div className="flex items-start space-x-3">
              <div className="flex-shrink-0 w-8 h-8 bg-indigo-100 rounded-lg flex items-center justify-center">
                <Upload className="w-4 h-4 text-indigo-600" />
              </div>
              <div className="flex-1">
                <h4 className="font-medium text-gray-900 mb-1">字幕上传调度</h4>
                <p className="text-sm text-gray-600">
                  视频上传完成1小时后，自动上传对应的字幕文件
                </p>
                <div className="mt-2 text-xs text-gray-500">
                  等待上传字幕: {videos.filter(v => v.status === '300').length} 个
                </div>
              </div>
            </div>

            {categories.failed.length > 0 && (
              <div className="flex items-start space-x-3 p-3 bg-red-50 rounded-lg">
                <div className="flex-shrink-0 w-8 h-8 bg-red-100 rounded-lg flex items-center justify-center">
                  <AlertCircle className="w-4 h-4 text-red-600" />
                </div>
                <div className="flex-1">
                  <h4 className="font-medium text-red-900 mb-1">失败任务提醒</h4>
                  <p className="text-sm text-red-700">
                    当前有 {categories.failed.length} 个任务失败，请查看详情并手动重试
                  </p>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* 统计汇总 */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="bg-white rounded-lg border border-gray-200 p-4">
          <div className="text-sm text-gray-600 mb-1">总任务数</div>
          <div className="text-2xl font-bold text-gray-900">{videos.length}</div>
        </div>
        <div className="bg-white rounded-lg border border-gray-200 p-4">
          <div className="text-sm text-gray-600 mb-1">进行中</div>
          <div className="text-2xl font-bold text-blue-600">
            {categories.preparing.length + categories.uploading.length}
          </div>
        </div>
        <div className="bg-white rounded-lg border border-gray-200 p-4">
          <div className="text-sm text-gray-600 mb-1">已完成</div>
          <div className="text-2xl font-bold text-green-600">{categories.completed.length}</div>
        </div>
        <div className="bg-white rounded-lg border border-gray-200 p-4">
          <div className="text-sm text-gray-600 mb-1">失败</div>
          <div className="text-2xl font-bold text-red-600">{categories.failed.length}</div>
        </div>
      </div>
    </div>
  );
}

const TaskStepDetail = ({ steps, onRetry, onResetAllFailed }: { steps: TaskStep[], onRetry: (stepName: string) => void, onResetAllFailed?: (resetAll: boolean) => void }) => {
  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed':
        return 'bg-green-100 text-green-800';
      case 'failed':
        return 'bg-red-100 text-red-800';
      case 'running':
        return 'bg-blue-100 text-blue-800';
      case 'skipped':
        return 'bg-yellow-100 text-yellow-800';
      case 'pending':
      default:
        return 'bg-gray-100 text-gray-800';
    }
  };

  // 检查是否有失败或跳过的步骤
  const hasFailedSteps = steps.some(step => step.status === 'failed' || step.status === 'skipped');

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h5 className="font-semibold text-gray-800">任务步骤</h5>
        {onResetAllFailed && (
          <button
            onClick={() => onResetAllFailed(!hasFailedSteps)}
            className={`inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-lg transition-colors ${
              hasFailedSteps 
                ? 'text-orange-700 bg-orange-50 border border-orange-300 hover:bg-orange-100' 
                : 'text-blue-700 bg-blue-50 border border-blue-300 hover:bg-blue-100'
            }`}
          >
            <RotateCcw className="w-4 h-4 mr-1.5" />
            {hasFailedSteps ? '一键重置失败任务' : '重置任务'}
          </button>
        )}
      </div>
      <ul className="space-y-2">
        {steps.sort((a, b) => a.step_order - b.step_order).map(step => (
          <li key={step.step_name} className="p-3 bg-white rounded-lg border border-gray-200">
            <div className="flex items-center justify-between">
              <div className="flex-1">
                <div className="flex items-center space-x-2">
                  <span className="font-medium text-gray-700">{step.step_name}</span>
                  <span className={`text-xs px-2 py-0.5 rounded-full ${getStatusColor(step.status)}`}>
                    {step.status}
                  </span>
                </div>
                {step.error_msg && step.status === 'failed' && (
                  <p className="text-xs text-red-600 mt-1">错误: {step.error_msg}</p>
                )}
                {step.error_msg && step.status === 'skipped' && (
                  <p className="text-xs text-yellow-600 mt-1">⚠️ 已跳过: {step.error_msg}</p>
                )}
              </div>
              {step.can_retry && (
                <button
                  onClick={() => onRetry(step.step_name)}
                  className="px-3 py-1 text-xs text-blue-600 bg-blue-100 hover:bg-blue-200 rounded"
                >
                  重试
                </button>
              )}
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
};
