'use client';

import { useState } from 'react';
import { Upload, FileText, AlertCircle, CheckCircle, Loader2, Trash2 } from 'lucide-react';
import { authFetch } from '@/lib/authFetch';

interface VideoActionsProps {
  videoId: string;
  status: string;
  onSuccess?: () => void;
}

export default function VideoActions({ videoId, status, onSuccess }: VideoActionsProps) {
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const handleManualUploadVideo = async () => {
    if (!confirm('确定要立即上传视频吗？这将打断定时上传队列。')) {
      return;
    }

    setUploading(true);
    setError(null);
    setSuccess(null);

    try {
      const response = await authFetch(`/api/v1/videos/${videoId}/upload/video`, {
        method: 'POST',
      });

      const data = await response.json();

      if (data.code === 200) {
        setSuccess('视频上传任务已启动，请等待上传完成');
        setTimeout(() => {
          onSuccess?.();
        }, 2000);
      } else {
        setError(data.message || '上传视频失败');
      }
    } catch (err: any) {
      console.error('上传视频失败:', err);
      setError('网络错误，请重试');
    } finally {
      setUploading(false);
    }
  };

  const handleManualUploadSubtitle = async () => {
    if (!confirm('确定要立即上传字幕吗？')) {
      return;
    }

    setUploading(true);
    setError(null);
    setSuccess(null);

    try {
      const response = await authFetch(`/api/v1/videos/${videoId}/upload/subtitle`, {
        method: 'POST',
      });

      const data = await response.json();

      if (data.code === 200) {
        setSuccess('字幕上传任务已启动，请等待上传完成');
        setTimeout(() => {
          onSuccess?.();
        }, 2000);
      } else {
        setError(data.message || '上传字幕失败');
      }
    } catch (err: any) {
      console.error('上传字幕失败:', err);
      setError('网络错误，请重试');
    } finally {
      setUploading(false);
    }
  };

  const handleDeleteVideo = async () => {
    if (!confirm('⚠️ 确定要删除这个视频吗？\n\n此操作将删除：\n- 所有任务步骤\n- 视频文件和字幕文件\n- 数据库记录\n\n此操作无法恢复！')) {
      return;
    }

    setUploading(true);
    setError(null);
    setSuccess(null);

    try {
      const response = await authFetch(`/api/v1/videos/${videoId}`, {
        method: 'DELETE',
      });

      const data = await response.json();

      if (data.code === 200) {
        setSuccess('视频已删除，即将返回列表...');
        setTimeout(() => {
          // 返回列表页或刷新
          window.location.href = '/dashboard';
        }, 1500);
      } else {
        setError(data.message || '删除视频失败');
      }
    } catch (err: any) {
      console.error('删除视频失败:', err);
      setError('网络错误，请重试');
    } finally {
      setUploading(false);
    }
  };

  // 根据状态决定显示哪些操作按钮
  // 200: 准备就绪（可自动上传）
  // 205: 等待手动上传（Free 用户或手动上传模式）
  // 299: 上传失败（可重试）
  const canUploadVideo = ['200', '205', '299'].includes(status);
  // 300: 视频已上传，等待字幕上传
  // 399: 字幕上传失败（可重试）
  const canUploadSubtitle = ['300', '399'].includes(status);

  return (
    <div className="space-y-4">
      {/* 上传操作 */}
      {(canUploadVideo || canUploadSubtitle) && (
        <div className="bg-white rounded-lg border border-gray-200 p-4">
          <h3 className="text-sm font-medium text-gray-900 mb-3 flex items-center">
            <Upload className="w-4 h-4 mr-2" />
            手动操作
          </h3>

          {/* 成功消息 */}
          {success && (
            <div className="mb-3 p-3 bg-green-50 border border-green-200 rounded-lg flex items-start space-x-2">
              <CheckCircle className="w-5 h-5 text-green-600 flex-shrink-0 mt-0.5" />
              <span className="text-sm text-green-800">{success}</span>
            </div>
          )}

          {/* 错误消息 */}
          {error && (
            <div className="mb-3 p-3 bg-red-50 border border-red-200 rounded-lg flex items-start space-x-2">
              <AlertCircle className="w-5 h-5 text-red-600 flex-shrink-0 mt-0.5" />
              <span className="text-sm text-red-800">{error}</span>
            </div>
          )}

          <div className="space-y-2">
            {/* 手动上传视频按钮 */}
            {canUploadVideo && (
              <div>
                <button
                  onClick={handleManualUploadVideo}
                  disabled={uploading}
                  className="w-full flex items-center justify-center space-x-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  {uploading ? (
                    <>
                      <Loader2 className="w-4 h-4 animate-spin" />
                      <span>处理中...</span>
                    </>
                  ) : (
                    <>
                      <Upload className="w-4 h-4" />
                      <span>立即上传视频</span>
                    </>
                  )}
                </button>
                <p className="text-xs text-gray-500 mt-1">
                  {status === '200' && '视频已准备就绪，可以立即上传'}
                  {status === '205' && '等待手动上传，点击按钮立即上传'}
                  {status === '299' && '视频上传失败，可以重试'}
                </p>
              </div>
            )}

            {/* 手动上传字幕按钮 */}
            {canUploadSubtitle && (
              <div>
                <button
                  onClick={handleManualUploadSubtitle}
                  disabled={uploading}
                  className="w-full flex items-center justify-center space-x-2 px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                >
                  {uploading ? (
                    <>
                      <Loader2 className="w-4 h-4 animate-spin" />
                      <span>处理中...</span>
                    </>
                  ) : (
                    <>
                      <FileText className="w-4 h-4" />
                      <span>立即上传字幕</span>
                    </>
                  )}
                </button>
                <p className="text-xs text-gray-500 mt-1">
                  {status === '300' ? '视频已上传，可以立即上传字幕' : '字幕上传失败，可以重试'}
                </p>
              </div>
            )}
          </div>

          {/* 提示信息 */}
          <div className="mt-3 p-3 bg-yellow-50 border border-yellow-200 rounded-lg">
            <p className="text-xs text-yellow-800">
              💡 <strong>提示：</strong>手动上传会打断定时任务队列，建议只在紧急情况下使用。
            </p>
          </div>
        </div>
      )}

      {/* 删除操作 */}
      <div className="bg-white rounded-lg border border-red-200 p-4">
        <h3 className="text-sm font-medium text-gray-900 mb-3 flex items-center">
          <Trash2 className="w-4 h-4 mr-2 text-red-600" />
          危险操作
        </h3>

        {/* 成功消息 */}
        {success && (
          <div className="mb-3 p-3 bg-green-50 border border-green-200 rounded-lg flex items-start space-x-2">
            <CheckCircle className="w-5 h-5 text-green-600 flex-shrink-0 mt-0.5" />
            <span className="text-sm text-green-800">{success}</span>
          </div>
        )}

        {/* 错误消息 */}
        {error && (
          <div className="mb-3 p-3 bg-red-50 border border-red-200 rounded-lg flex items-start space-x-2">
            <AlertCircle className="w-5 h-5 text-red-600 flex-shrink-0 mt-0.5" />
            <span className="text-sm text-red-800">{error}</span>
          </div>
        )}

        <button
          onClick={handleDeleteVideo}
          disabled={uploading}
          className="w-full flex items-center justify-center space-x-2 px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          {uploading ? (
            <>
              <Loader2 className="w-4 h-4 animate-spin" />
              <span>删除中...</span>
            </>
          ) : (
            <>
              <Trash2 className="w-4 h-4" />
              <span>删除视频</span>
            </>
          )}
        </button>

        <p className="text-xs text-gray-500 mt-2">
          删除后将无法恢复，请谨慎操作
        </p>
      </div>
    </div>
  );
}
