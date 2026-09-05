"use client";

import { useState } from 'react';
import AppLayout from '@/components/layout/AppLayout';
import TaskQueueStats from '@/components/dashboard/TaskQueueStats';
import { BarChart3 } from 'lucide-react';

export default function DashboardPage() {
  const [selectedVideoId, setSelectedVideoId] = useState<string | null>(null);

  const handleVideoSelect = (videoId: string) => {
    setSelectedVideoId(videoId);
  };

  return (
    <AppLayout>
      <div className="bg-white rounded-lg shadow-md">
        <div className="p-6 border-b border-gray-200">
          <div className="flex items-center space-x-3">
            <BarChart3 className="w-5 h-5 text-gray-600" />
            <h2 className="text-lg font-medium text-gray-900">任务队列</h2>
          </div>
        </div>

        <div className="p-6">
          <TaskQueueStats onVideoSelect={handleVideoSelect} />
        </div>
      </div>
    </AppLayout>
  );
}