'use client';

import { useState, useEffect } from 'react';
import { ArrowLeft, ExternalLink, RefreshCw, Download, Calendar, Clock, Image, FileText, Play, Eye } from 'lucide-react';
import { VideoDetail, VideoFile, TASK_STEP_NAMES } from '@/types';
import { videoApi } from '@/lib/api';
import TaskStepList from './TaskStepList';
import StatusBadge from '@/components/ui/StatusBadge';
import VideoActions from './VideoActions';
import { MetadataSourceBadge, MetadataComparison } from '@/components/MetadataSourceBadge';

interface VideoDetailPageProps {
  videoId: string;
  onBack: () => void;
}

export default function VideoDetailPage({ videoId, onBack }: VideoDetailPageProps) {
  const [video, setVideo] = useState<VideoDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchVideoDetail = async (showRefreshing = false) => {
    if (showRefreshing) setRefreshing(true);
    else setLoading(true);

    try {
      const response = await videoApi.getVideoDetail(videoId);
      if (response.code === 200 || response.code === 0) {
        setVideo(response.data);
        setError(null);
      } else {
        setError(response.message || '获取视频详情失败');
      }
    } catch (err: any) {
      console.error('获取视频详情失败:', err);
      setError(err.message || '网络错误，请重试');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    fetchVideoDetail();
  }, [videoId]);

  const handleRetryStep = async (stepName: string) => {
    try {
      const response = await videoApi.retryTaskStep(videoId, stepName);
      if (response.code === 200 || response.code === 0) {
        // 重新获取视频详情以更新状态
        setTimeout(() => fetchVideoDetail(true), 1000);
      } else {
        alert(response.message || '重试失败');
      }
    } catch (err: any) {
      console.error('重试任务失败:', err);
      alert('重试失败，请稍后再试');
    }
  };

  const handleResetAllFailed = async () => {
    try {
      const response = await videoApi.resetAllFailedSteps(videoId);
      if (response.code === 200 || response.code === 0) {
        const data = response.data;
        if (data && data.reset_count > 0) {
          alert(`已重置 ${data.reset_count} 个失败步骤，任务将自动重新执行`);
        } else {
          alert('没有需要重置的失败步骤');
        }
        // 重新获取视频详情以更新状态
        setTimeout(() => fetchVideoDetail(true), 1000);
      } else {
        alert(response.message || '重置失败');
      }
    } catch (err: any) {
      console.error('重置失败步骤失败:', err);
      alert('重置失败，请稍后再试');
    }
  };

  const handleRefresh = () => {
    fetchVideoDetail(true);
  };

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  };

  const getFileIcon = (type: VideoFile['type']) => {
    const className = "w-4 h-4";
    switch (type) {
      case 'video':
        return <Play className={className} />;
      case 'subtitle':
        return <FileText className={className} />;
      case 'cover':
        return <Image className={className} />;
      default:
        return <FileText className={className} />;
    }
  };

  const formatFileSize = (bytes: number) => {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-center">
          <div className="inline-block w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full animate-spin mb-4"></div>
          <p className="text-gray-600">加载视频详情...</p>
        </div>
      </div>
    );
  }

  if (error || !video) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-center">
          <div className="text-red-500 text-6xl mb-4">⚠️</div>
          <h2 className="text-xl font-semibold text-gray-900 mb-2">加载失败</h2>
          <p className="text-gray-600 mb-4">{error}</p>
          <div className="space-x-4">
            <button
              onClick={() => fetchVideoDetail()}
              className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
            >
              重试
            </button>
            <button
              onClick={onBack}
              className="px-4 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors"
            >
              返回
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 py-6">
      <div className="container mx-auto px-4 max-w-6xl">
        {/* 头部导航 */}
        <div className="flex items-center justify-between mb-6">
          <button
            onClick={onBack}
            className="flex items-center space-x-2 text-gray-600 hover:text-gray-900 transition-colors"
          >
            <ArrowLeft className="w-5 h-5" />
            <span>返回视频列表</span>
          </button>

          <button
            onClick={handleRefresh}
            disabled={refreshing}
            className="flex items-center space-x-2 px-3 py-2 text-sm text-gray-600 hover:text-gray-900 transition-colors disabled:opacity-50"
          >
            <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin' : ''}`} />
            <span>刷新</span>
          </button>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* 左侧：视频信息 */}
          <div className="lg:col-span-2 space-y-6">
            {/* 基本信息 */}
            <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
              <div className="flex items-start justify-between mb-4">
                <div className="flex-1">
                  <h1 className="text-2xl font-bold text-gray-900 mb-2">
                    {video.generated_title || video.title || '未设置标题'}
                  </h1>
                  {video.generated_title && video.generated_title !== video.title && (
                    <p className="text-gray-500 mb-2">
                      原标题: {video.title}
                    </p>
                  )}
                  <div className="flex items-center space-x-4 text-sm text-gray-500">
                    <span className="flex items-center space-x-1">
                      <Calendar className="w-4 h-4" />
                      <span>{formatDate(video.created_at)}</span>
                    </span>
                    <span>ID: {video.video_id}</span>
                  </div>
                </div>

                <div className="flex flex-col items-end space-y-2">
                  <StatusBadge status={video.status} />

                  {video.cover_image && (
                    <a
                      href={video.cover_image}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-sm text-blue-600 hover:text-blue-800 flex items-center space-x-1"
                    >
                      <Eye className="w-4 h-4" />
                      <span>查看封面</span>
                    </a>
                  )}
                </div>
              </div>

              {/* 元数据来源信息 */}
              {video.metadata_source && (
                <div className="mb-4 p-4 bg-gradient-to-r from-purple-50 to-blue-50 rounded-lg border border-purple-100">
                  <div className="flex items-center justify-between mb-3">
                    <h3 className="text-sm font-semibold text-gray-900">元数据来源</h3>
                    <MetadataSourceBadge source={video.metadata_source} />
                  </div>

                  {/* 标题三栏对比 */}
                  <MetadataComparison
                    label="标题"
                    original={video.title}
                    generated={video.generated_title}
                    upload={video.upload_title}
                    source={video.metadata_source}
                    maxLength={80}
                  />

                  {/* 描述三栏对比 */}
                  <div className="mt-4">
                    <MetadataComparison
                      label="描述"
                      original={video.description}
                      generated={video.generated_desc}
                      upload={video.upload_desc}
                      source={video.metadata_source}
                      maxLength={150}
                    />
                  </div>
                </div>
              )}

              {/* 标签 */}
              {(video.upload_tags || video.generated_tags) && (
                <div className="mb-4">
                  <div className="flex items-center gap-2 mb-2">
                    <h3 className="text-sm font-medium text-gray-900">标签</h3>
                    {video.metadata_source && (
                      <span className="text-xs text-gray-500">
                        ({video.metadata_source === 'ai_generated' ? 'AI生成' : '原始'})
                      </span>
                    )}
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {(video.upload_tags || video.generated_tags || '').split(',').map((tag, index) => (
                      <span
                        key={index}
                        className="inline-block px-2 py-1 text-sm bg-blue-100 text-blue-800 rounded"
                      >
                        {tag.trim()}
                      </span>
                    )).filter(tag => tag.props.children.trim())}
                  </div>
                </div>
              )}

              {/* 原始URL */}
              <div>
                <h3 className="text-sm font-medium text-gray-900 mb-2">视频来源</h3>
                <a
                  href={video.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center space-x-2 text-blue-600 hover:text-blue-800 text-sm"
                >
                  <span className="truncate max-w-md">{video.url}</span>
                  <ExternalLink className="w-4 h-4 flex-shrink-0" />
                </a>
              </div>
            </div>

            {/* 任务步骤 */}
            <TaskStepList
              steps={video.task_steps}
              onRetryStep={handleRetryStep}
              onResetAllFailed={handleResetAllFailed}
              isRetrying={refreshing}
            />
          </div>

          {/* 右侧：进度和文件 */}
          <div className="space-y-6">
            {/* 手动操作按钮 */}
            <VideoActions
              videoId={video.video_id}
              status={video.status}
              onSuccess={handleRefresh}
            />

            {/* 任务进度 */}
            <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
              <h3 className="text-lg font-semibold text-gray-900 mb-4">处理进度</h3>

              <div className="space-y-4">
                {/* 进度条 */}
                <div>
                  <div className="flex justify-between text-sm text-gray-600 mb-2">
                    <span>总体进度</span>
                    <span>{video.progress.progress_percentage}%</span>
                  </div>
                  <div className="w-full bg-gray-200 rounded-full h-2">
                    <div
                      className="bg-blue-600 h-2 rounded-full transition-all duration-300"
                      style={{ width: `${video.progress.progress_percentage}%` }}
                    ></div>
                  </div>
                </div>

                {/* 统计信息 */}
                <div className="grid grid-cols-2 gap-4 text-sm">
                  <div className="text-center p-3 bg-gray-50 rounded-lg">
                    <div className="text-2xl font-bold text-green-600">
                      {video.progress.completed_steps}
                    </div>
                    <div className="text-gray-600">已完成</div>
                  </div>
                  <div className="text-center p-3 bg-gray-50 rounded-lg">
                    <div className="text-2xl font-bold text-red-600">
                      {video.progress.failed_steps}
                    </div>
                    <div className="text-gray-600">失败</div>
                  </div>
                </div>

                {/* 当前步骤 */}
                {video.progress.is_running && video.progress.current_step && (
                  <div className="p-3 bg-blue-50 border border-blue-200 rounded-lg">
                    <div className="flex items-center space-x-2">
                      <Clock className="w-4 h-4 text-blue-600" />
                      <span className="text-sm text-blue-800">
                        正在执行: {TASK_STEP_NAMES[video.progress.current_step as keyof typeof TASK_STEP_NAMES] || video.progress.current_step}
                      </span>
                    </div>
                  </div>
                )}
              </div>
            </div>

            {/* 文件列表 */}
            {video.files && video.files.length > 0 && (
              <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
                <h3 className="text-lg font-semibold text-gray-900 mb-4">相关文件</h3>

                <div className="space-y-3">
                  {video.files.map((file, index) => (
                    <div
                      key={index}
                      className="flex items-center justify-between p-3 border border-gray-200 rounded-lg hover:bg-gray-50 transition-colors"
                    >
                      <div className="flex items-center space-x-3 flex-1 min-w-0">
                        {getFileIcon(file.type)}
                        <div className="min-w-0 flex-1">
                          <p className="text-sm font-medium text-gray-900 truncate">
                            {file.name}
                          </p>
                          <p className="text-xs text-gray-500">
                            {formatFileSize(file.size)}
                          </p>
                        </div>
                      </div>

                      <button
                        onClick={() => window.open(file.path, '_blank')}
                        className="p-1 text-gray-400 hover:text-blue-600 transition-colors"
                        title="下载文件"
                      >
                        <Download className="w-4 h-4" />
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}