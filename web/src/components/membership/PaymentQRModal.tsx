'use client';

import { useState, useEffect } from 'react';
import QRCode from 'qrcode';

interface PaymentQRModalProps {
  isOpen: boolean;
  onClose: () => void;
  payUrl: string;
  orderId: string;
  amount: number;
  productName: string;
}

export default function PaymentQRModal({
  isOpen,
  onClose,
  payUrl,
  orderId,
  amount,
  productName,
}: PaymentQRModalProps) {
  const [qrCode, setQrCode] = useState<string>('');
  const [status, setStatus] = useState<'waiting' | 'paid' | 'expired' | 'failed'>('waiting');
  const [countdown, setCountdown] = useState(300); // 5 分钟倒计时

  // 生成二维码
  useEffect(() => {
    if (isOpen && payUrl) {
      QRCode.toDataURL(payUrl, {
        width: 256,
        margin: 2,
        color: {
          dark: '#000000',
          light: '#ffffff',
        },
      })
        .then(setQrCode)
        .catch((err) => {
          console.error('生成二维码失败:', err);
        });
    }
  }, [isOpen, payUrl]);

  // 轮询支付状态
  useEffect(() => {
    if (!isOpen || status !== 'waiting') return;

    const timer = setInterval(() => {
      checkPaymentStatus();
    }, 2000); // 每 2 秒检查一次

    return () => clearInterval(timer);
  }, [isOpen, status]);

  // 倒计时
  useEffect(() => {
    if (!isOpen || countdown <= 0) return;

    const timer = setInterval(() => {
      setCountdown((prev) => {
        if (prev <= 1) {
          setStatus('expired');
          return 0;
        }
        return prev - 1;
      });
    }, 1000);

    return () => clearInterval(timer);
  }, [isOpen, countdown]);

  const checkPaymentStatus = async () => {
    try {
      // 调用后端 API 检查支付状态
      const res = await fetch(`/api/v1/payment/order-status?order_id=${orderId}`, {
        headers: {
          Authorization: `Bearer ${localStorage.getItem('token')}`,
        },
      });
      const data = await res.json();

      if (data.code === 0) {
        if (data.data.status === 'paid') {
          setStatus('paid');
          setTimeout(() => {
            window.location.reload(); // 刷新页面
          }, 1500);
        } else if (data.data.status === 'expired') {
          setStatus('expired');
        }
      }
    } catch (err) {
      console.error('检查支付状态失败:', err);
    }
  };

  if (!isOpen) return null;

  const formatTime = (seconds: number) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* 背景遮罩 */}
      <div
        className="fixed inset-0 bg-black bg-opacity-50 transition-opacity"
        onClick={onClose}
      ></div>

      {/* 弹窗内容 */}
      <div className="relative bg-white rounded-2xl shadow-xl max-w-md w-full mx-4 overflow-hidden">
        {/* 头部 */}
        <div className="bg-gradient-to-r from-amber-500 to-orange-500 px-6 py-4">
          <h3 className="text-xl font-bold text-white">扫码支付</h3>
          <p className="text-amber-100 text-sm mt-1">{productName}</p>
        </div>

        {/* 内容 */}
        <div className="px-6 py-6">
          {status === 'waiting' && (
            <>
              {/* 金额 */}
              <div className="text-center mb-6">
                <div className="text-4xl font-bold text-gray-900">
                  ¥{amount}
                </div>
                <div className="text-gray-500 text-sm mt-1">
                  请使用手机扫码支付
                </div>
              </div>

              {/* 二维码 */}
              {qrCode && (
                <div className="flex justify-center mb-6">
                  <div className="relative">
                    <img
                      src={qrCode}
                      alt="支付二维码"
                      className="w-64 h-64 border-4 border-gray-100 rounded-lg"
                    />
                    {/* 加载遮罩 */}
                  </div>
                </div>
              )}

              {/* 倒计时 */}
              <div className="text-center text-gray-500 text-sm mb-4">
                请在 <span className="text-amber-600 font-medium">{formatTime(countdown)}</span> 内完成支付
              </div>

              {/* 提示 */}
              <div className="bg-blue-50 border border-blue-200 rounded-lg px-4 py-3">
                <div className="flex items-start gap-2">
                  <svg className="w-5 h-5 text-blue-500 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                  <div className="text-sm text-blue-700">
                    <p className="font-medium mb-1">支付提示</p>
                    <ul className="list-disc list-inside space-y-1 text-xs">
                      <li>请使用微信/支付宝&ldquo;扫一扫&rdquo;功能扫描二维码</li>
                      <li>支付完成后页面将自动刷新</li>
                      <li>如未刷新，请稍后手动刷新页面</li>
                    </ul>
                  </div>
                </div>
              </div>
            </>
          )}

          {status === 'paid' && (
            <div className="text-center py-8">
              <div className="w-16 h-16 bg-green-100 rounded-full flex items-center justify-center mx-auto mb-4">
                <svg className="w-10 h-10 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
              </div>
              <h4 className="text-xl font-bold text-gray-900 mb-2">支付成功！</h4>
              <p className="text-gray-500">页面即将刷新...</p>
            </div>
          )}

          {status === 'expired' && (
            <div className="text-center py-8">
              <div className="w-16 h-16 bg-red-100 rounded-full flex items-center justify-center mx-auto mb-4">
                <svg className="w-10 h-10 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </div>
              <h4 className="text-xl font-bold text-gray-900 mb-2">二维码已过期</h4>
              <p className="text-gray-500 mb-4">请重新下单</p>
              <button
                onClick={onClose}
                className="px-6 py-2 bg-gray-100 hover:bg-gray-200 rounded-lg text-gray-700 font-medium"
              >
                关闭
              </button>
            </div>
          )}
        </div>

        {/* 底部按钮 */}
        {status === 'waiting' && (
          <div className="px-6 py-4 bg-gray-50 border-t">
            <button
              onClick={onClose}
              className="w-full px-4 py-2 bg-gray-100 hover:bg-gray-200 rounded-lg text-gray-700 font-medium"
            >
              取消支付
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
