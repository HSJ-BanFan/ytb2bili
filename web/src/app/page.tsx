"use client";

import AppLayout from '@/components/layout/AppLayout';
import { useAuth } from '@/hooks/useAuth';
import { videoApi } from '@/lib/api';
import { Plus, Youtube, Video, Globe, AlertCircle, CheckCircle, LogIn } from 'lucide-react';
import { useState } from 'react';
import Link from 'next/link';

export default function HomePage() {
  const { user, loading, handleLoginSuccess, handleRefreshStatus, handleLogout } = useAuth();
  const [videoUrl, setVideoUrl] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [submitMessage, setSubmitMessage] = useState('');
  const [messageType, setMessageType] = useState<'success' | 'error' | ''>('');

  // 提交视频链接到后端
  const handleSubmitUrl = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!videoUrl.trim()) {
      setMessageType('error');
      setSubmitMessage('请输入视频链接');
      return;
    }

    setIsSubmitting(true);
    setSubmitMessage('');
    setMessageType('');

    try {
      // 使用配置了 JWT 认证的 API 实例
      const result = await videoApi.submitVideo({
        url: videoUrl,
        title: '',
        subtitles: [],
      });

      if (result.code === 0 || result.data) {
        setMessageType('success');
        setSubmitMessage('视频链接已成功提交！');
        setVideoUrl(''); // 清空输入框
      } else {
        setMessageType('error');
        setSubmitMessage(result.message || '提交失败，请重试');
      }
    } catch (error) {
      console.error('提交失败:', error);
      setMessageType('error');
      setSubmitMessage('网络错误，请检查后端服务是否正常运行');
    } finally {
      setIsSubmitting(false);
    }
  };

  // 检测视频平台类型
  const detectPlatform = (url: string) => {
    if (url.includes('youtube.com') || url.includes('youtu.be')) return 'YouTube';
    if (url.includes('bilibili.com')) return 'Bilibili';
    if (url.includes('twitter.com') || url.includes('x.com')) return 'Twitter/X';
    if (url.includes('tiktok.com')) return 'TikTok';
    if (url.includes('instagram.com')) return 'Instagram';
    return '未知平台';
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-center">
          <div className="inline-block w-8 h-8 border-4 border-blue-500 border-t-transparent rounded-full animate-spin mb-4"></div>
          <p className="text-gray-600">加载中...</p>
        </div>
      </div>
    );
  }

  if (!user) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100 flex items-center justify-center">
        <div className="container mx-auto px-4">
          <div className="max-w-md mx-auto">
            <div className="text-center mb-8">
              <h1 className="text-4xl font-bold text-gray-900 mb-3">
                Bili-Up Web
              </h1>
              <p className="text-gray-600 text-lg">
                多平台视频下载与管理平台
              </p>
            </div>
            
            {/* 登录入口 */}
            <div className="bg-white rounded-xl shadow-xl p-8">
              <div className="text-center mb-6">
                <div className="inline-flex items-center justify-center w-16 h-16 bg-blue-100 rounded-full mb-4">
                  <LogIn className="w-8 h-8 text-blue-600" />
                </div>
                <h2 className="text-xl font-semibold text-gray-900">欢迎使用</h2>
                <p className="text-gray-500 mt-2">登录后即可使用视频下载和上传功能</p>
              </div>
              
              {/* 登录按钮 */}
              <Link 
                href="/login"
                className="w-full flex items-center justify-center px-6 py-4 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors text-lg font-medium shadow-md hover:shadow-lg"
              >
                <LogIn className="w-5 h-5 mr-2" />
                登录 / 注册
              </Link>
              
              <p className="text-center text-sm text-gray-500 mt-6">
                登录后可在设置中绑定 B站账号进行视频上传
              </p>
            </div>
            
            {/* 功能介绍 */}
            <div className="mt-8 text-center text-gray-600">
              <p className="text-sm">支持 YouTube、Bilibili、Twitter、TikTok 等平台</p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <AppLayout userName={user?.name} onLogout={handleLogout}>
      <div className="max-w-4xl mx-auto space-y-6">
        {/* 主要功能区域 - 视频链接提交 */}
        <div className="bg-white rounded-lg shadow-md p-8">
          <div className="text-center mb-8">
            <h1 className="text-3xl font-bold text-gray-900 mb-2">
              多平台视频下载
            </h1>
            <p className="text-gray-600">
              支持 YouTube、Bilibili、Twitter、TikTok 等多个平台的视频下载
            </p>
          </div>

          {/* 视频链接输入表单 */}
          <form onSubmit={handleSubmitUrl} className="space-y-6">
            <div>
              <label htmlFor="video-url" className="block text-sm font-medium text-gray-700 mb-2">
                视频链接
              </label>
              <div className="relative">
                <input
                  id="video-url"
                  type="url"
                  value={videoUrl}
                  onChange={(e) => setVideoUrl(e.target.value)}
                  placeholder="请输入视频链接，如：https://www.youtube.com/watch?v=..."
                  className="w-full px-4 py-3 pr-12 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none transition-colors"
                  disabled={isSubmitting}
                />
                <div className="absolute inset-y-0 right-0 flex items-center pr-3">
                  {videoUrl.trim() && (
                    <span className="text-xs text-gray-500 bg-gray-100 px-2 py-1 rounded">
                      {detectPlatform(videoUrl)}
                    </span>
                  )}
                </div>
              </div>
            </div>

            <button
              type="submit"
              disabled={isSubmitting || !videoUrl.trim()}
              className="w-full flex items-center justify-center px-6 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:bg-gray-300 disabled:cursor-not-allowed transition-colors font-medium"
            >
              {isSubmitting ? (
                <>
                  <div className="w-5 h-5 border-2 border-white border-t-transparent rounded-full animate-spin mr-2"></div>
                  提交中...
                </>
              ) : (
                <>
                  <Plus className="w-5 h-5 mr-2" />
                  提交下载
                </>
              )}
            </button>
          </form>

          {/* 提交结果消息 */}
          {submitMessage && (
            <div className={`mt-4 p-4 rounded-lg flex items-center ${
              messageType === 'success' 
                ? 'bg-green-50 border border-green-200 text-green-800'
                : 'bg-red-50 border border-red-200 text-red-800'
            }`}>
              {messageType === 'success' ? (
                <CheckCircle className="w-5 h-5 mr-2 text-green-600" />
              ) : (
                <AlertCircle className="w-5 h-5 mr-2 text-red-600" />
              )}
              {submitMessage}
            </div>
          )}
        </div>

        {/* 支持的平台说明 */}
        <div className="bg-gradient-to-r from-blue-50 to-indigo-50 rounded-lg border border-blue-200 p-6">
          <h3 className="text-lg font-semibold text-gray-900 mb-4 flex items-center">
            <Globe className="w-5 h-5 mr-2 text-blue-600" />
            支持的视频平台
          </h3>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="flex items-center space-x-2">
              <Youtube className="w-5 h-5 text-red-600" />
              <span className="text-sm text-gray-700">YouTube</span>
            </div>
            <div className="flex items-center space-x-2">
              <Video className="w-5 h-5 text-blue-600" />
              <span className="text-sm text-gray-700">Bilibili</span>
            </div>
            <div className="flex items-center space-x-2">
              <Globe className="w-5 h-5 text-blue-400" />
              <span className="text-sm text-gray-700">Twitter/X</span>
            </div>
            <div className="flex items-center space-x-2">
              <Video className="w-5 h-5 text-purple-600" />
              <span className="text-sm text-gray-700">TikTok</span>
            </div>
          </div>
          <p className="mt-4 text-sm text-gray-600">
            基于 yt-dlp 技术，支持超过 1000+ 个视频网站的下载。提交后系统将自动识别平台并开始下载处理。
          </p>
        </div>

        {/* 快捷导航 */}
        <div className="bg-white rounded-lg shadow-md p-6">
          <h3 className="text-lg font-semibold text-gray-900 mb-4">
            管理功能
          </h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            <a
              href="/dashboard"
              className="flex items-center justify-center px-4 py-3 bg-blue-50 text-blue-700 rounded-lg hover:bg-blue-100 transition-colors"
            >
              <Video className="w-5 h-5 mr-2" />
              任务队列
            </a>
            <a
              href="/schedule"
              className="flex items-center justify-center px-4 py-3 bg-green-50 text-green-700 rounded-lg hover:bg-green-100 transition-colors"
            >
              <Plus className="w-5 h-5 mr-2" />
              定时上传
            </a>
            <a
              href="/extension"
              className="flex items-center justify-center px-4 py-3 bg-purple-50 text-purple-700 rounded-lg hover:bg-purple-100 transition-colors"
            >
              <Globe className="w-5 h-5 mr-2" />
              浏览器插件
            </a>
            <a
              href="/settings"
              className="flex items-center justify-center px-4 py-3 bg-gray-50 text-gray-700 rounded-lg hover:bg-gray-100 transition-colors"
            >
              <Video className="w-5 h-5 mr-2" />
              设置
            </a>
          </div>
        </div>
      </div>
    </AppLayout>
  );
}
