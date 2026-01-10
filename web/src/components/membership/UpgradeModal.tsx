'use client';

import { useState, useEffect } from 'react';
import { paymentApi } from '@/lib/api';
import type { VipProduct } from '@/types';
import PaymentQRModal from './PaymentQRModal';

interface UpgradeModalProps {
  isOpen: boolean;
  onClose: () => void;
  currentTier?: string;
}

// 商品展示配置
const PRODUCT_CONFIG: Record<string, { icon: string; color: string; features: string[] }> = {
  'ytb2bili_vip_enterprise_monthly': {
    icon: '👑',
    color: 'from-amber-500 to-orange-500',
    features: ['无限视频', '所有功能', 'API 访问', '团队协作', '专属支持', '批量处理 100 个'],
  },
  'ytb2bili_vip_enterprise_yearly': {
    icon: '👑',
    color: 'from-amber-500 to-orange-500',
    features: ['无限视频', '所有功能', 'API 访问', '团队协作', '专属支持', '批量处理 100 个', '年付省 17%'],
  },
};

export default function UpgradeModal({ isOpen, onClose, currentTier = 'basic' }: UpgradeModalProps) {
  const [products, setProducts] = useState<VipProduct[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [billingCycle, setBillingCycle] = useState<'monthly' | 'yearly'>('monthly');
  const [payWay, setPayWay] = useState<'wechat' | 'alipay'>('wechat');
  const [purchasing, setPurchasing] = useState<string | null>(null);

  // 支付二维码弹窗状态
  const [paymentQR, setPaymentQR] = useState<{
    isOpen: boolean;
    payUrl: string;
    orderId: string;
    amount: number;
    productName: string;
  }>({
    isOpen: false,
    payUrl: '',
    orderId: '',
    amount: 0,
    productName: '',
  });

  useEffect(() => {
    if (isOpen) {
      fetchProducts();
    }
  }, [isOpen]);

  const fetchProducts = async () => {
    try {
      setLoading(true);
      setError(null);
      const res = await paymentApi.getProducts();
      if (res.code === 0 && res.data) {
        setProducts(res.data);
      } else {
        setError('获取商品信息失败');
      }
    } catch (err) {
      console.error('获取商品信息失败:', err);
      setError('获取商品信息失败，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  const handlePurchase = async (product: VipProduct) => {
    try {
      setPurchasing(product.product_id);
      const res = await paymentApi.createOrder({
        product_id: product.product_id,
        pay_way: payWay,
      });

      if (res.code === 0 && res.data) {
        // 显示二维码弹窗
        setPaymentQR({
          isOpen: true,
          payUrl: res.data.pay_url || res.data.qr_code || '',
          orderId: res.data.order_no || '',  // 后端返回的是 order_no
          amount: product.price,
          productName: product.name,
        });
      } else {
        alert('创建订单失败：' + (res.message || '未知错误'));
      }
    } catch (err) {
      console.error('创建订单失败:', err);
      alert('创建订单失败，请稍后重试');
    } finally {
      setPurchasing(null);
    }
  };

  if (!isOpen) return null;

  // 根据计费周期筛选商品
  const filteredProducts = products.filter(p => {
    if (billingCycle === 'monthly') {
      return p.product_id.includes('monthly');
    } else {
      return p.product_id.includes('yearly');
    }
  });

  const isEnterprise = currentTier === 'enterprise';

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      {/* 背景遮罩 */}
      <div
        className="fixed inset-0 bg-black bg-opacity-50 transition-opacity"
        onClick={onClose}
      ></div>

      {/* 弹窗内容 */}
      <div className="flex min-h-full items-center justify-center p-4">
        <div className="relative bg-white rounded-2xl shadow-xl max-w-2xl w-full max-h-[90vh] overflow-y-auto">
          {/* 头部 */}
          <div className="sticky top-0 bg-white border-b px-6 py-4 flex items-center justify-between">
            <div>
              <h2 className="text-xl font-bold text-gray-900">升级会员</h2>
              <p className="text-sm text-gray-500 mt-1">解锁全部功能，享受无限制服务</p>
            </div>
            <button
              onClick={onClose}
              className="text-gray-400 hover:text-gray-600 transition-colors"
            >
              <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          {/* 计费周期切换 */}
          <div className="px-6 py-4 flex justify-center">
            <div className="bg-gray-100 rounded-lg p-1 inline-flex">
              <button
                onClick={() => setBillingCycle('monthly')}
                className={`px-4 py-2 rounded-md text-sm font-medium transition-colors ${billingCycle === 'monthly'
                  ? 'bg-white text-gray-900 shadow'
                  : 'text-gray-600 hover:text-gray-900'
                  }`}
              >
                月付
              </button>
              <button
                onClick={() => setBillingCycle('yearly')}
                className={`px-4 py-2 rounded-md text-sm font-medium transition-colors ${billingCycle === 'yearly'
                  ? 'bg-white text-gray-900 shadow'
                  : 'text-gray-600 hover:text-gray-900'
                  }`}
              >
                年付 <span className="text-green-600 text-xs">省 17%</span>
              </button>
            </div>
          </div>

          {/* 支付方式选择 */}
          <div className="px-6 flex justify-center pb-4">
            <div className="inline-flex space-x-4">
              <label
                className={`cursor-pointer border rounded-lg px-4 py-2 flex items-center space-x-2 transition-all ${payWay === 'wechat'
                  ? 'border-green-500 bg-green-50 text-green-700 ring-1 ring-green-500'
                  : 'border-gray-200 hover:border-green-300'
                  }`}
                onClick={() => setPayWay('wechat')}
              >
                <span className="text-xl">💚</span>
                <span className="font-medium">微信支付</span>
              </label>
              <label
                className={`cursor-pointer border rounded-lg px-4 py-2 flex items-center space-x-2 transition-all ${payWay === 'alipay'
                  ? 'border-blue-500 bg-blue-50 text-blue-700 ring-1 ring-blue-500'
                  : 'border-gray-200 hover:border-blue-300'
                  }`}
                onClick={() => setPayWay('alipay')}
              >
                <span className="text-xl">💙</span>
                <span className="font-medium">支付宝</span>
              </label>
            </div>
          </div>

          {/* 商品卡片 */}
          <div className="px-6 pb-6">
            {loading ? (
              <div className="flex justify-center py-12">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-amber-500"></div>
              </div>
            ) : error ? (
              <div className="text-center py-12">
                <p className="text-red-500 mb-4">{error}</p>
                <button
                  onClick={fetchProducts}
                  className="px-4 py-2 bg-gray-100 hover:bg-gray-200 rounded-lg text-sm"
                >
                  重试
                </button>
              </div>
            ) : filteredProducts.length === 0 ? (
              <div className="text-center py-12 text-gray-500">
                暂无可购买的商品
              </div>
            ) : (
              <div className="space-y-4">
                {filteredProducts.map((product) => {
                  const config = PRODUCT_CONFIG[product.product_id] || {
                    icon: '👑',
                    color: 'from-amber-500 to-orange-500',
                    features: [],
                  };

                  return (
                    <div
                      key={product.product_id}
                      className="relative rounded-xl border-2 border-amber-200 bg-gradient-to-br from-amber-50 to-orange-50 p-6 shadow-lg"
                    >
                      <div className="flex items-start justify-between">
                        <div className="flex-1">
                          <div className="flex items-center gap-3 mb-2">
                            <span className="text-4xl">{config.icon}</span>
                            <div>
                              <h3 className="text-xl font-bold text-gray-900">{product.name}</h3>
                              <p className="text-sm text-gray-600">{product.description}</p>
                            </div>
                          </div>

                          {/* 功能列表 */}
                          <ul className="grid grid-cols-2 gap-2 mt-4">
                            {config.features.map((feature, idx) => (
                              <li key={idx} className="flex items-center gap-2 text-sm">
                                <svg className="w-4 h-4 text-amber-500 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                                </svg>
                                <span className="text-gray-600">{feature}</span>
                              </li>
                            ))}
                          </ul>
                        </div>

                        {/* 价格和购买 */}
                        <div className="text-right ml-6">
                          <div className="mb-4">
                            {product.original_price > product.price && (
                              <div className="text-gray-400 line-through text-sm">
                                ¥{product.original_price}
                              </div>
                            )}
                            <div className="flex items-baseline justify-end gap-1">
                              <span className="text-3xl font-bold text-amber-600">¥{product.price}</span>
                              <span className="text-gray-500 text-sm">
                                /{product.vip_days >= 365 ? '年' : '月'}
                              </span>
                            </div>
                            <div className="text-xs text-gray-500 mt-1">
                              {product.vip_days} 天会员
                            </div>
                          </div>

                          <button
                            onClick={() => handlePurchase(product)}
                            disabled={isEnterprise || purchasing === product.product_id}
                            className={`px-6 py-2 rounded-lg font-medium transition-all ${isEnterprise
                              ? 'bg-gray-200 text-gray-400 cursor-not-allowed'
                              : purchasing === product.product_id
                                ? 'bg-amber-300 text-white cursor-wait'
                                : 'bg-gradient-to-r from-amber-500 to-orange-500 text-white hover:from-amber-600 hover:to-orange-600 shadow-lg hover:shadow-xl'
                              }`}
                          >
                            {isEnterprise ? '已是企业版' : purchasing === product.product_id ? '处理中...' : '立即购买'}
                          </button>
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {/* 底部说明 */}
          <div className="px-6 py-4 bg-gray-50 border-t text-center text-sm text-gray-500">
            <p>支持微信/支付宝支付 · 7 天无理由退款 · 随时可取消订阅</p>
          </div>
        </div>
      </div>

      {/* 支付二维码弹窗 */}
      <PaymentQRModal
        isOpen={paymentQR.isOpen}
        onClose={() => setPaymentQR({ ...paymentQR, isOpen: false })}
        payUrl={paymentQR.payUrl}
        orderId={paymentQR.orderId}
        amount={paymentQR.amount}
        productName={paymentQR.productName}
      />
    </div>
  );
}
