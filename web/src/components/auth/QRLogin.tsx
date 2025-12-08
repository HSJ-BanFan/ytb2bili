'use client';

import { useState, useEffect, useCallback } from 'react';
import { QrCode, RefreshCw, CheckCircle, XCircle } from 'lucide-react';

interface QRLoginProps {
  onLoginSuccess?: (user: any) => void;
  onRefreshStatus?: () => void; // 添加状态刷新回调
}

export default function QRLogin({ onLoginSuccess, onRefreshStatus }: QRLoginProps) {
  const [qrCodeUrl, setQrCodeUrl] = useState<string>('');
  const [authCode, setAuthCode] = useState<string>('');
  const [status, setStatus] = useState<'idle' | 'loading' | 'scanning' | 'success' | 'error'>('idle');
  const [message, setMessage] = useState<string>('');
  const [polling, setPolling] = useState<boolean>(false);

  // 获取API基础URL
  const getApiBaseUrl = () => {
    // 在静态部署模式下，使用当前域名和端口
    if (typeof window !== 'undefined') {
      const { protocol, hostname, port } = window.location;
      // 如果前端和后端运行在同一个端口（embed模式），直接使用当前地址
      // 否则使用默认的8096端口
      return `${protocol}//${hostname}${port ? ':' + port : ''}`;
    }
    // 服务端渲染时的默认值
    return 'http://localhost:8096';
  };

  // 生成二维码
  const generateQRCode = useCallback(async () => {
    setStatus('loading');
    setMessage('正在生成二维码...');
    
    try {
      const apiBaseUrl = getApiBaseUrl();
      const response = await fetch(`${apiBaseUrl}/api/v1/auth/qrcode`);
      console.log('API Base URL:', apiBaseUrl);
      console.log('Response status:', response.status);
      console.log('Response headers:', response.headers);
      
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      
      const data = await response.json();
      console.log('QR Code API Response:', data);
      
      if (data.code === 0) {
        // 后端现在直接返回完整的二维码URL
        console.log('QR code URL from backend:', data.qr_code_url);
        
        setQrCodeUrl(data.qr_code_url);
        setAuthCode(data.auth_code);
        setStatus('scanning');
        setMessage('请使用 Bilibili 手机客户端扫描二维码');
        startPolling(data.auth_code);
      } else {
        throw new Error(data.message || '生成二维码失败');
      }
    } catch (error) {
      console.error('生成二维码失败:', error);
      setStatus('error');
      const errorMessage = error instanceof Error ? error.message : '未知错误';
      setMessage(`生成二维码失败: ${errorMessage}`);
    }
  }, []);

  // 轮询检查登录状态
  const startPolling = (code: string) => {
    if (polling) return;
    
    setPolling(true);
    const pollInterval = setInterval(async () => {
      try {
        const apiBaseUrl = getApiBaseUrl();
        const response = await fetch(`${apiBaseUrl}/api/v1/auth/poll`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({ auth_code: code }),
        });
        
        const data = await response.json();
        
        if (data.code === 0 && data.login_info) {
          // 登录成功
          setStatus('success');
          setMessage('登录成功！正在跳转...');
          setPolling(false);
          clearInterval(pollInterval);
          
          // 延迟一下让用户看到成功消息，然后刷新主页面状态
          setTimeout(() => {
            if (onRefreshStatus) {
              onRefreshStatus(); // 通知主页面刷新登录状态
            }
            
            if (onLoginSuccess) {
              onLoginSuccess({
                id: data.login_info.token_info?.mid?.toString() || '',
                name: data.login_info.token_info?.uname || 'Bilibili用户',
                mid: data.login_info.token_info?.mid?.toString() || '',
                avatar: data.login_info.token_info?.face || ''
              });
            }
          }, 1000);
        } else if (response.status === 403) {
          // 权限不足（多账号上传需要企业版）
          setStatus('error');
          setMessage(data.message || '多账号上传是企业版功能，请升级会员');
          setPolling(false);
          clearInterval(pollInterval);
        } else if (response.status === 400 || response.status === 500) {
          // 二维码过期或无效
          setStatus('error');
          setMessage('二维码已过期，请重新生成');
          setPolling(false);
          clearInterval(pollInterval);
        }
      } catch (error) {
        console.error('检查登录状态失败:', error);
      }
    }, 3000); // 每3秒检查一次

    // 5分钟后自动停止轮询
    setTimeout(() => {
      if (polling) {
        setPolling(false);
        clearInterval(pollInterval);
        if (status === 'scanning') {
          setStatus('error');
          setMessage('二维码已过期，请重新生成');
        }
      }
    }, 300000);
  };

  useEffect(() => {
    generateQRCode();
    
    return () => {
      setPolling(false);
    };
  }, []); // 移除依赖项，避免无限循环

  const handleRefresh = () => {
    setPolling(false);
    generateQRCode();
  };

  return (
    <div className="flex flex-col items-center justify-center space-y-6 p-8">
      <div className="text-center">
        <h2 className="text-2xl font-bold text-gray-900 mb-2">
          Bilibili 扫码登录
        </h2>
        <p className="text-gray-600">
          使用 Bilibili 手机客户端扫描下方二维码完成登录
        </p>
      </div>

      <div className="relative">
        {/* 二维码容器 */}
        <div className="w-64 h-64 border-2 border-gray-200 rounded-lg flex items-center justify-center bg-white">
          {status === 'loading' && (
            <div className="flex flex-col items-center space-y-2">
              <RefreshCw className="w-8 h-8 animate-spin text-blue-500" />
              <span className="text-sm text-gray-500">生成中...</span>
            </div>
          )}
          
          {status === 'scanning' && qrCodeUrl && (
            <div className="relative">
              <img
                src={qrCodeUrl}
                alt="登录二维码"
                width={240}
                height={240}
                className="rounded"
                onLoad={() => console.log('QR code image loaded successfully')}
                onError={(e) => {
                  console.error('QR code image load error:', e);
                  console.error('Failed to load QR code URL:', qrCodeUrl);
                  setStatus('error');
                  setMessage('二维码图片加载失败，请重试');
                }}
              />
              {/* 显示二维码URL用于调试 */}
              <div className="absolute bottom-0 left-0 right-0 bg-black bg-opacity-50 text-white text-xs p-1 rounded-b truncate">
                {qrCodeUrl}
              </div>
            </div>
          )}
          
          {status === 'success' && (
            <div className="flex flex-col items-center space-y-2">
              <CheckCircle className="w-12 h-12 text-green-500" />
              <span className="text-sm text-green-600 font-medium">登录成功</span>
            </div>
          )}
          
          {status === 'error' && (
            <div className="flex flex-col items-center space-y-2">
              <XCircle className="w-12 h-12 text-red-500" />
              <span className="text-sm text-red-600 text-center">
                {message}
              </span>
            </div>
          )}
        </div>

        {/* 状态指示器 */}
        {status === 'scanning' && (
          <div className="absolute -top-2 -right-2">
            <div className="w-4 h-4 bg-blue-500 rounded-full animate-pulse"></div>
          </div>
        )}
      </div>

      {/* 状态消息 */}
      <div className="text-center">
        <p className={`text-sm ${
          status === 'success' ? 'text-green-600' : 
          status === 'error' ? 'text-red-600' : 
          'text-gray-600'
        }`}>
          {message}
        </p>
      </div>

      {/* 操作按钮 */}
      <div className="flex space-x-4">
        {(status === 'error' || status === 'idle') && (
          <button
            onClick={handleRefresh}
            className="flex items-center space-x-2 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors"
          >
            <RefreshCw className="w-4 h-4" />
            <span>重新生成</span>
          </button>
        )}
        
        {status === 'scanning' && (
          <button
            onClick={handleRefresh}
            className="flex items-center space-x-2 px-4 py-2 bg-gray-500 text-white rounded-lg hover:bg-gray-600 transition-colors"
          >
            <RefreshCw className="w-4 h-4" />
            <span>刷新二维码</span>
          </button>
        )}
      </div>

      {/* 扫码说明 */}
      <div className="text-xs text-gray-500 text-center max-w-sm">
        <p>
          打开 Bilibili 手机客户端，点击右上角扫一扫图标，
          扫描上方二维码即可快速登录
        </p>
      </div>
    </div>
  );
}