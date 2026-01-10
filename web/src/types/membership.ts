// 会员等级
export type MembershipTier = "basic" | "pro" | "enterprise";

// 会员功能特性
export interface TierFeatures {
  auto_upload: boolean;
  subtitle_translation: boolean;
  metadata_generation: boolean;
  custom_templates: boolean;
  priority_support: boolean;
}

// 会员限制
export interface TierLimits {
  max_concurrent_tasks: number;
  daily_upload_limit: number; // 0 = 无限制
  max_video_duration: number; // 0 = 无限制
  storage_limit: number; // MB, 0 = 无限制
}

// 会员等级配置 (Backend)
export interface TierConfig {
  tier: MembershipTier;
  name: string;
  features: TierFeatures;
  limits: TierLimits;

  // 以前的字段 (可选保留兼容)
  daily_limit?: number;
  batch_limit?: number;
  priority?: number;
  price?: number;
  description?: string;
}

// 会员信息
export interface MembershipInfo {
  user_id: string;
  tier: MembershipTier;
  tier_name: string;
  expires_at?: string;
  days_remaining: number;
  is_expired: boolean;
  daily_limit: number;
  batch_limit: number;
  priority: number;
  subscription_id?: string;
}

// 配额信息
export interface QuotaInfo {
  daily_limit: number;
  daily_used: number;
  daily_remaining: number;
  boost_pack_remaining: number;
  total_remaining: number;
  is_unlimited: boolean;
}

// 功能检查结果
export interface FeatureCheckResult {
  feature: string;
  allowed: boolean;
  reason?: string;
  suggestion?: MembershipTier;
}

// 可用功能列表
export interface AvailableFeatures {
  tier: MembershipTier;
  features: string[];
}

// 加油包状态
export interface BoostPackStatus {
  has_pack: boolean;
  videos_remaining: number;
  expires_at?: string;
  days_remaining: number;
}

// 加油包类型
export type BoostPackType = "small" | "medium" | "large";

// 加油包配置
export interface BoostPackConfig {
  type: BoostPackType;
  name: string;
  videos: number;
  price: number;
  valid_days: number;
}

// 购买加油包请求
export interface PurchaseBoostPackRequest {
  pack_type: BoostPackType;
}

// 购买加油包响应
export interface PurchaseBoostPackResponse {
  pack_type: BoostPackType;
  videos_added: number;
  total_videos: number;
  expires_at: string;
}

// 功能名称映射
export const FEATURE_NAMES: Record<string, string> = {
  ai_translation: "AI 字幕翻译",
  translation_optimize: "翻译质量优化",
  ai_title_generation: "AI 标题生成",
  gemini_video_analysis: "Gemini 视频分析",
  auto_upload: "自动上传",
  priority_queue: "优先队列",
  api_access: "API 访问",
  custom_template: "自定义模板",
  data_export: "数据导出",
  team_collaboration: "团队协作",
};

// 等级颜色映射
export const TIER_COLORS: Record<
  MembershipTier,
  { bg: string; text: string; border: string }
> = {
  basic: {
    bg: "bg-blue-100",
    text: "text-blue-700",
    border: "border-blue-300",
  },
  pro: {
    bg: "bg-purple-100",
    text: "text-purple-700",
    border: "border-purple-300",
  },
  enterprise: {
    bg: "bg-amber-100",
    text: "text-amber-700",
    border: "border-amber-300",
  },
};

// 等级图标
export const TIER_ICONS: Record<MembershipTier, string> = {
  basic: "⭐",
  pro: "💎",
  enterprise: "👑",
};

// VIP 商品（来自 pay-unify）
export interface VipProduct {
  product_id: string;
  name: string;
  description: string;
  type: string;
  price: number;
  original_price: number;
  vip_days: number;
  icon: string;
  status: string;
}

// 支付订单创建请求
export interface CreatePaymentOrderRequest {
  product_id: string;
  pay_way: "wechat" | "alipay";
}

// 支付订单响应
export interface PaymentOrderResponse {
  order_no: string;
  order_id: string; // 订单ID（用于轮询状态）
  pay_url: string; // 支付URL或跳转链接
  qr_code?: string; // 二维码URL（可选）
  amount: number;
  pay_way: string;
}
