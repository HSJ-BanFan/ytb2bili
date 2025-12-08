"use client";

import { useState } from 'react';
import AppLayout from '@/components/layout/AppLayout';
import ScheduleManager from '@/components/schedule/ScheduleManager';
import { useAuth } from '@/hooks/useAuth';
import { Clock } from 'lucide-react';

export default function SchedulePage() {
  const { user, loading, handleLoginSuccess, handleRefreshStatus, handleLogout } = useAuth();
  const [selectedVideoId, setSelectedVideoId] = useState<string | null>(null);

  const handleVideoSelect = (videoId: string) => {
    setSelectedVideoId(videoId);
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
                Bilibili 视频管理平台
              </p>
            </div>

            <div className="bg-white rounded-xl shadow-xl p-8">
              <div className="text-center mb-6">
                <div className="inline-flex items-center justify-center w-16 h-16 bg-green-100 rounded-full mb-4">
                  <Clock className="w-8 h-8 text-green-600" />
                </div>
                <h2 className="text-xl font-semibold text-gray-900">请先登录</h2>
                <p className="text-gray-500 mt-2">登录后即可使用定时上传功能</p>
              </div>

              <a
                href="/login"
                className="w-full flex items-center justify-center px-6 py-4 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors text-lg font-medium shadow-md hover:shadow-lg"
              >
                登录 / 注册
              </a>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <AppLayout userName={user?.name} onLogout={handleLogout}>
      <div className="bg-white rounded-lg shadow-md">
        <div className="p-6 border-b border-gray-200">
          <div className="flex items-center space-x-3">
            <Clock className="w-5 h-5 text-gray-600" />
            <h2 className="text-lg font-medium text-gray-900">定时上传</h2>
          </div>
        </div>

        <div className="p-6">
          <ScheduleManager onVideoSelect={handleVideoSelect} />
        </div>
      </div>
    </AppLayout>
  );
}