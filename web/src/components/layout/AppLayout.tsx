"use client";

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { User, LogOut, Settings, BarChart3, Clock, Puzzle, Sparkles } from 'lucide-react';
import { useJWTAuth } from '@/hooks/useJWTAuth';

interface AppLayoutProps {
  children: React.ReactNode;
  userName?: string;
  onLogout?: () => void;
}

export default function AppLayout({ children, userName, onLogout }: AppLayoutProps) {
  const { user: jwtUser, logout: jwtLogout } = useJWTAuth();
  const pathname = usePathname();

  const displayName = userName || jwtUser?.username || '用户';
  const handleLogout = onLogout || jwtLogout;

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white shadow-sm border-b border-gray-200">
        <div className="container mx-auto px-4">
          <div className="flex items-center justify-between h-16">
            <div className="flex items-center space-x-4">
              <Link href="/" className="text-xl font-semibold text-gray-900">
                Bili-Up Web
              </Link>
            </div>

            <div className="flex items-center space-x-4">
              <div className="flex items-center space-x-2 text-sm text-gray-600">
                <User className="w-4 h-4" />
                <span>{displayName}</span>
              </div>

              <button
                onClick={handleLogout}
                className="flex items-center space-x-2 px-3 py-2 text-sm text-gray-600 hover:text-red-600 transition-colors"
              >
                <LogOut className="w-4 h-4" />
                <span>退出登录</span>
              </button>
            </div>
          </div>
        </div>
      </header>

      <div className="container mx-auto px-4 py-8">
        <div className="flex gap-8">
          <div className="w-64 flex-shrink-0">
            <nav className="bg-white rounded-lg shadow-sm p-4">
              <ul className="space-y-2">
                <li>
                  <Link href="/" className={`w-full flex items-center space-x-3 px-3 py-2 rounded-lg text-left transition-colors ${pathname === '/' ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-50'}`}>
                    <User className="w-5 h-5" />
                    <span>主页</span>
                  </Link>
                </li>
                <li>
                  <Link href="/dashboard" className={`w-full flex items-center space-x-3 px-3 py-2 rounded-lg text-left transition-colors ${pathname === '/dashboard' ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-50'}`}>
                    <BarChart3 className="w-5 h-5" />
                    <span>任务队列</span>
                  </Link>
                </li>
                <li>
                  <Link href="/schedule" className={`w-full flex items-center space-x-3 px-3 py-2 rounded-lg text-left transition-colors ${pathname === '/schedule' ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-50'}`}>
                    <Clock className="w-5 h-5" />
                    <span>定时上传</span>
                  </Link>
                </li>
                <li>
                  <Link href="/extension" className={`w-full flex items-center space-x-3 px-3 py-2 rounded-lg text-left transition-colors ${pathname === '/extension' ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-50'}`}>
                    <Puzzle className="w-5 h-5" />
                    <span>浏览器插件</span>
                  </Link>
                </li>
                <li>
                  <Link href="/user-settings" className={`w-full flex items-center space-x-3 px-3 py-2 rounded-lg text-left transition-colors ${pathname === '/user-settings' ? 'bg-purple-50 text-purple-700' : 'text-gray-700 hover:bg-gray-50'}`}>
                    <Sparkles className="w-5 h-5" />
                    <span>个人设置</span>
                  </Link>
                </li>
                <li>
                  <Link href="/settings" className={`w-full flex items-center space-x-3 px-3 py-2 rounded-lg text-left transition-colors ${pathname === '/settings' ? 'bg-blue-50 text-blue-700' : 'text-gray-700 hover:bg-gray-50'}`}>
                    <Settings className="w-5 h-5" />
                    <span>系统设置</span>
                  </Link>
                </li>
              </ul>
            </nav>
          </div>
          <div className="flex-1">{children}</div>
        </div>
      </div>
    </div>
  );
}