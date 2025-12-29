# YTB2BILI 会员服务完整实现方案

> 基于业界最佳实践，结合项目特点设计的完整会员系统实现指南

## 一、技术选型

### 1.1 推荐技术栈

| 组件         | 技术选择                 | 理由                             |
| ------------ | ------------------------ | -------------------------------- |
| **支付服务** | Lemon Squeezy            | 个人友好、全球可用、自动处理税务 |
| **数据库**   | PostgreSQL + Drizzle ORM | 类型安全、与现有项目兼容         |
| **缓存**     | Redis (Upstash)          | 配额实时计算、高性能             |
| **认证**     | 设备 ID + 可选用户系统   | 简化流程、支持扩展               |
| **前端**     | Next.js 15 + Tailwind    | 现有项目技术栈                   |

### 1.2 架构图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              YTB2BILI 会员系统架构                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────────────────────┐  │
│   │   前端 Web   │────▶│  Next.js    │────▶│     Lemon Squeezy          │  │
│   │  (会员页面)  │     │  API Routes │     │   (支付 & 订阅管理)         │  │
│   └─────────────┘     └──────┬──────┘     └──────────────┬──────────────┘  │
│                              │                           │                  │
│                              ▼                           │ Webhook          │
│                       ┌─────────────┐                    │                  │
│                       │   Redis     │◀───────────────────┘                  │
│                       │  (Upstash)  │                                       │
│                       │ - 配额缓存   │                                       │
│                       │ - 会员状态   │                                       │
│                       └──────┬──────┘                                       │
│                              │                                              │
│                              ▼                                              │
│                       ┌─────────────┐     ┌─────────────────────────────┐  │
│                       │ PostgreSQL  │────▶│      Go 后端服务             │  │
│                       │  (订单记录)  │     │   (视频处理 + 配额检查)      │  │
│                       └─────────────┘     └─────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 二、数据库设计

### 2.1 Drizzle Schema 定义

```typescript
// src/db/schema.ts
import {
  pgTable,
  serial,
  text,
  integer,
  boolean,
  timestamp,
  jsonb,
  decimal,
} from "drizzle-orm/pg-core";

// 用户表
export const users = pgTable("users", {
  id: serial("id").primaryKey(),
  deviceId: text("device_id").unique().notNull(),
  email: text("email"),
  name: text("name"),
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().notNull(),
});

// 会员计划表 (从 Lemon Squeezy 同步)
export const plans = pgTable("plans", {
  id: serial("id").primaryKey(),
  productId: text("product_id").notNull(), // Lemon Squeezy product ID
  variantId: text("variant_id").unique().notNull(), // Lemon Squeezy variant ID
  name: text("name").notNull(),
  description: text("description"),
  price: decimal("price", { precision: 10, scale: 2 }).notNull(),
  interval: text("interval"), // month, year, null(一次性)
  intervalCount: integer("interval_count"),
  isUsageBased: boolean("is_usage_based").default(false),
  sort: integer("sort").default(0),
  // 功能限制
  videosPerDay: integer("videos_per_day").default(5),
  batchSize: integer("batch_size").default(1),
  aiTranslation: boolean("ai_translation").default(false),
  autoUpload: boolean("auto_upload").default(false),
  priorityQueue: boolean("priority_queue").default(false),
  apiAccess: boolean("api_access").default(false),
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().notNull(),
});

// 订阅表
export const subscriptions = pgTable("subscriptions", {
  id: serial("id").primaryKey(),
  lemonSqueezyId: text("lemonsqueezy_id").unique().notNull(),
  orderId: integer("order_id").notNull(),
  userId: integer("user_id")
    .references(() => users.id)
    .notNull(),
  planId: integer("plan_id")
    .references(() => plans.id)
    .notNull(),
  status: text("status").notNull(), // active, cancelled, expired, past_due, paused
  statusFormatted: text("status_formatted"),
  renewsAt: timestamp("renews_at"),
  endsAt: timestamp("ends_at"),
  trialEndsAt: timestamp("trial_ends_at"),
  isPaused: boolean("is_paused").default(false),
  customerId: text("customer_id"),
  customerPortalUrl: text("customer_portal_url"),
  createdAt: timestamp("created_at").defaultNow().notNull(),
  updatedAt: timestamp("updated_at").defaultNow().notNull(),
});

// 加油包购买记录
export const boostPacks = pgTable("boost_packs", {
  id: serial("id").primaryKey(),
  lemonSqueezyId: text("lemonsqueezy_id").unique(),
  orderId: text("order_id"),
  userId: integer("user_id")
    .references(() => users.id)
    .notNull(),
  packType: text("pack_type").notNull(), // small, medium, large
  videosAdded: integer("videos_added").notNull(),
  videosRemaining: integer("videos_remaining").notNull(),
  expiresAt: timestamp("expires_at").notNull(),
  status: text("status").default("active"), // active, expired, depleted
  createdAt: timestamp("created_at").defaultNow().notNull(),
});

// 使用记录表
export const usageLogs = pgTable("usage_logs", {
  id: serial("id").primaryKey(),
  userId: integer("user_id")
    .references(() => users.id)
    .notNull(),
  videoId: text("video_id").notNull(),
  quotaType: text("quota_type").notNull(), // daily, boost_pack
  boostPackId: integer("boost_pack_id").references(() => boostPacks.id),
  createdAt: timestamp("created_at").defaultNow().notNull(),
});

// Webhook 事件记录
export const webhookEvents = pgTable("webhook_events", {
  id: serial("id").primaryKey(),
  eventName: text("event_name").notNull(),
  body: jsonb("body").notNull(),
  processed: boolean("processed").default(false),
  processingError: text("processing_error"),
  createdAt: timestamp("created_at").defaultNow().notNull(),
});
```

### 2.2 Redis Key 设计

```typescript
// src/lib/redis-keys.ts

export const REDIS_KEYS = {
  // 用户每日使用量: ytb2bili:usage:{userId}:{date}
  dailyUsage: (userId: number, date: string) =>
    `ytb2bili:usage:${userId}:${date}`,

  // 用户会员状态缓存: ytb2bili:membership:{userId}
  membershipCache: (userId: number) => `ytb2bili:membership:${userId}`,

  // 用户配额缓存: ytb2bili:quota:{userId}
  quotaCache: (userId: number) => `ytb2bili:quota:${userId}`,

  // 加油包余额: ytb2bili:boost:{userId}
  boostBalance: (userId: number) => `ytb2bili:boost:${userId}`,
};

export const REDIS_TTL = {
  dailyUsage: 60 * 60 * 24 * 10, // 10 天
  membershipCache: 60 * 5, // 5 分钟
  quotaCache: 60 * 5, // 5 分钟
};
```

---

## 三、Lemon Squeezy 集成

### 3.1 环境变量配置

```env
# .env.local
LEMONSQUEEZY_API_KEY=your_api_key
LEMONSQUEEZY_STORE_ID=your_store_id
LEMONSQUEEZY_WEBHOOK_SECRET=your_webhook_secret

# 产品 Variant IDs (在 Lemon Squeezy 后台获取)
LEMONSQUEEZY_BASIC_VARIANT_ID=123456
LEMONSQUEEZY_PRO_VARIANT_ID=123457
LEMONSQUEEZY_ENTERPRISE_VARIANT_ID=123458
LEMONSQUEEZY_BOOST_SMALL_VARIANT_ID=123459
LEMONSQUEEZY_BOOST_MEDIUM_VARIANT_ID=123460
LEMONSQUEEZY_BOOST_LARGE_VARIANT_ID=123461

# Redis
UPSTASH_REDIS_REST_URL=your_redis_url
UPSTASH_REDIS_REST_TOKEN=your_redis_token
```

### 3.2 Lemon Squeezy SDK 配置

```typescript
// src/lib/lemonsqueezy.ts
import {
  lemonSqueezySetup,
  getProduct,
  listProducts,
  createCheckout,
} from "@lemonsqueezy/lemonsqueezy.js";

export function configureLemonSqueezy() {
  const apiKey = process.env.LEMONSQUEEZY_API_KEY;
  if (!apiKey) {
    throw new Error("Missing LEMONSQUEEZY_API_KEY");
  }

  lemonSqueezySetup({
    apiKey,
    onError: (error) => console.error("Lemon Squeezy Error:", error),
  });
}

// 创建结账会话
export async function createCheckoutSession({
  variantId,
  userId,
  deviceId,
  email,
  redirectUrl,
}: {
  variantId: string;
  userId: number;
  deviceId: string;
  email?: string;
  redirectUrl: string;
}) {
  configureLemonSqueezy();

  const checkout = await createCheckout(
    process.env.LEMONSQUEEZY_STORE_ID!,
    variantId,
    {
      checkoutOptions: {
        embed: false,
        media: true,
        logo: true,
      },
      checkoutData: {
        email: email ?? undefined,
        custom: {
          user_id: userId.toString(),
          device_id: deviceId,
        },
      },
      productOptions: {
        enabledVariants: [parseInt(variantId)],
        redirectUrl,
        receiptButtonText: "返回应用",
        receiptLinkUrl: redirectUrl,
      },
    }
  );

  return checkout.data?.data.attributes.url;
}
```

### 3.3 Webhook 处理

```typescript
// src/app/api/webhook/route.ts
import crypto from "node:crypto";
import { db } from "@/db";
import { webhookEvents, subscriptions, boostPacks, users } from "@/db/schema";
import { eq } from "drizzle-orm";
import { redis } from "@/lib/redis";
import { REDIS_KEYS } from "@/lib/redis-keys";

export async function POST(request: Request) {
  const secret = process.env.LEMONSQUEEZY_WEBHOOK_SECRET;
  if (!secret) {
    return new Response("Webhook secret not configured", { status: 500 });
  }

  // 验证签名
  const rawBody = await request.text();
  const hmac = crypto.createHmac("sha256", secret);
  const digest = Buffer.from(hmac.update(rawBody).digest("hex"), "utf8");
  const signature = Buffer.from(
    request.headers.get("X-Signature") || "",
    "utf8"
  );

  if (!crypto.timingSafeEqual(digest, signature)) {
    return new Response("Invalid signature", { status: 401 });
  }

  const data = JSON.parse(rawBody);
  const eventName = data.meta.event_name;
  const customData = data.meta.custom_data;

  // 存储 webhook 事件
  const [event] = await db
    .insert(webhookEvents)
    .values({
      eventName,
      body: data,
    })
    .returning();

  try {
    // 处理不同事件
    switch (eventName) {
      case "subscription_created":
      case "subscription_updated":
        await handleSubscriptionEvent(data, customData);
        break;

      case "subscription_cancelled":
      case "subscription_expired":
        await handleSubscriptionEnd(data);
        break;

      case "order_created":
        // 处理一次性购买（加油包）
        if (isBoostPackOrder(data)) {
          await handleBoostPackPurchase(data, customData);
        }
        break;
    }

    // 标记为已处理
    await db
      .update(webhookEvents)
      .set({ processed: true })
      .where(eq(webhookEvents.id, event.id));
  } catch (error) {
    console.error("Webhook processing error:", error);
    await db
      .update(webhookEvents)
      .set({ processingError: String(error) })
      .where(eq(webhookEvents.id, event.id));
  }

  return new Response("OK", { status: 200 });
}

async function handleSubscriptionEvent(data: any, customData: any) {
  const attrs = data.data.attributes;
  const userId = parseInt(customData?.user_id);

  if (!userId) {
    throw new Error("Missing user_id in custom data");
  }

  // 更新或创建订阅记录
  await db
    .insert(subscriptions)
    .values({
      lemonSqueezyId: data.data.id,
      orderId: attrs.order_id,
      userId,
      planId: attrs.variant_id, // 需要映射到本地 plan ID
      status: attrs.status,
      statusFormatted: attrs.status_formatted,
      renewsAt: attrs.renews_at ? new Date(attrs.renews_at) : null,
      endsAt: attrs.ends_at ? new Date(attrs.ends_at) : null,
      customerId: attrs.customer_id?.toString(),
      customerPortalUrl: attrs.urls?.customer_portal,
    })
    .onConflictDoUpdate({
      target: subscriptions.lemonSqueezyId,
      set: {
        status: attrs.status,
        statusFormatted: attrs.status_formatted,
        renewsAt: attrs.renews_at ? new Date(attrs.renews_at) : null,
        endsAt: attrs.ends_at ? new Date(attrs.ends_at) : null,
        isPaused: attrs.pause !== null,
        updatedAt: new Date(),
      },
    });

  // 清除缓存
  await redis.del(REDIS_KEYS.membershipCache(userId));
  await redis.del(REDIS_KEYS.quotaCache(userId));
}

async function handleBoostPackPurchase(data: any, customData: any) {
  const userId = parseInt(customData?.user_id);
  const packType = customData?.pack_type;

  if (!userId || !packType) {
    throw new Error("Missing user_id or pack_type");
  }

  const packConfig = {
    small: { videos: 10, days: 7 },
    medium: { videos: 30, days: 15 },
    large: { videos: 80, days: 30 },
  }[packType];

  if (!packConfig) {
    throw new Error(`Invalid pack type: ${packType}`);
  }

  const expiresAt = new Date();
  expiresAt.setDate(expiresAt.getDate() + packConfig.days);

  // 检查是否有现有加油包，如果有则叠加
  const existingPack = await db.query.boostPacks.findFirst({
    where: (bp, { and, eq, gt }) =>
      and(
        eq(bp.userId, userId),
        eq(bp.status, "active"),
        gt(bp.expiresAt, new Date())
      ),
  });

  if (existingPack) {
    // 叠加：增加额度，延长有效期
    const newExpiry = new Date(existingPack.expiresAt);
    newExpiry.setDate(newExpiry.getDate() + packConfig.days);

    await db
      .update(boostPacks)
      .set({
        videosRemaining: existingPack.videosRemaining + packConfig.videos,
        expiresAt: newExpiry,
      })
      .where(eq(boostPacks.id, existingPack.id));
  } else {
    // 新建加油包
    await db.insert(boostPacks).values({
      lemonSqueezyId: data.data.id,
      orderId: data.data.attributes.order_id?.toString(),
      userId,
      packType,
      videosAdded: packConfig.videos,
      videosRemaining: packConfig.videos,
      expiresAt,
    });
  }

  // 清除缓存
  await redis.del(REDIS_KEYS.boostBalance(userId));
  await redis.del(REDIS_KEYS.quotaCache(userId));
}

function isBoostPackOrder(data: any): boolean {
  const variantId =
    data.data.attributes.first_order_item?.variant_id?.toString();
  const boostVariants = [
    process.env.LEMONSQUEEZY_BOOST_SMALL_VARIANT_ID,
    process.env.LEMONSQUEEZY_BOOST_MEDIUM_VARIANT_ID,
    process.env.LEMONSQUEEZY_BOOST_LARGE_VARIANT_ID,
  ];
  return boostVariants.includes(variantId);
}
```

---

## 四、配额管理服务

### 4.1 配额服务实现

```typescript
// src/lib/quota-service.ts
import { db } from "@/db";
import { subscriptions, boostPacks, usageLogs, plans } from "@/db/schema";
import { eq, and, gte, sql } from "drizzle-orm";
import { redis } from "@/lib/redis";
import { REDIS_KEYS, REDIS_TTL } from "@/lib/redis-keys";

export interface UserQuota {
  userId: number;
  tier: "free" | "basic" | "pro" | "enterprise";
  tierName: string;
  dailyLimit: number;
  dailyUsed: number;
  dailyRemaining: number;
  boostPackBalance: number;
  boostPackExpire: Date | null;
  totalRemaining: number;
  membershipExpire: Date | null;
  features: {
    aiTranslation: boolean;
    autoUpload: boolean;
    priorityQueue: boolean;
    apiAccess: boolean;
    batchSize: number;
  };
}

const FREE_PLAN = {
  tier: "free" as const,
  tierName: "免费版",
  dailyLimit: 5,
  features: {
    aiTranslation: false,
    autoUpload: false,
    priorityQueue: false,
    apiAccess: false,
    batchSize: 1,
  },
};

export class QuotaService {
  // 获取用户配额信息
  async getUserQuota(userId: number): Promise<UserQuota> {
    // 尝试从缓存获取
    const cacheKey = REDIS_KEYS.quotaCache(userId);
    const cached = await redis.get(cacheKey);
    if (cached) {
      return JSON.parse(cached);
    }

    // 获取会员信息
    const membership = await this.getMembershipInfo(userId);

    // 获取今日使用量
    const dailyUsed = await this.getDailyUsage(userId);

    // 获取加油包余额
    const boostInfo = await this.getBoostPackInfo(userId);

    const dailyRemaining = Math.max(0, membership.dailyLimit - dailyUsed);

    const quota: UserQuota = {
      userId,
      tier: membership.tier,
      tierName: membership.tierName,
      dailyLimit: membership.dailyLimit,
      dailyUsed,
      dailyRemaining,
      boostPackBalance: boostInfo.balance,
      boostPackExpire: boostInfo.expiresAt,
      totalRemaining: dailyRemaining + boostInfo.balance,
      membershipExpire: membership.expiresAt,
      features: membership.features,
    };

    // 缓存结果
    await redis.set(cacheKey, JSON.stringify(quota), {
      ex: REDIS_TTL.quotaCache,
    });

    return quota;
  }

  // 检查是否可以处理视频
  async canProcessVideo(
    userId: number
  ): Promise<{ allowed: boolean; reason?: string }> {
    const quota = await this.getUserQuota(userId);

    if (quota.totalRemaining <= 0) {
      if (quota.tier === "free") {
        return {
          allowed: false,
          reason: "今日免费额度已用完，请升级会员或购买加油包",
        };
      }
      return {
        allowed: false,
        reason: "今日额度已用完，请购买加油包获取更多额度",
      };
    }

    return { allowed: true };
  }

  // 消耗配额
  async consumeQuota(userId: number, videoId: string): Promise<void> {
    const quota = await this.getUserQuota(userId);

    if (quota.totalRemaining <= 0) {
      throw new Error("No quota available");
    }

    let quotaType: "daily" | "boost_pack" = "daily";
    let boostPackId: number | null = null;

    // 优先消耗每日配额
    if (quota.dailyRemaining > 0) {
      await this.incrementDailyUsage(userId);
    } else if (quota.boostPackBalance > 0) {
      // 消耗加油包
      quotaType = "boost_pack";
      boostPackId = await this.decrementBoostPack(userId);
    }

    // 记录使用日志
    await db.insert(usageLogs).values({
      userId,
      videoId,
      quotaType,
      boostPackId,
    });

    // 清除缓存
    await redis.del(REDIS_KEYS.quotaCache(userId));
  }

  // 获取会员信息
  private async getMembershipInfo(userId: number) {
    const subscription = await db.query.subscriptions.findFirst({
      where: and(
        eq(subscriptions.userId, userId),
        eq(subscriptions.status, "active")
      ),
      with: {
        plan: true,
      },
    });

    if (!subscription || !subscription.plan) {
      return {
        ...FREE_PLAN,
        expiresAt: null,
      };
    }

    const plan = subscription.plan;
    return {
      tier: this.getTierFromPlan(plan.name),
      tierName: plan.name,
      dailyLimit: plan.videosPerDay ?? 5,
      expiresAt: subscription.renewsAt,
      features: {
        aiTranslation: plan.aiTranslation ?? false,
        autoUpload: plan.autoUpload ?? false,
        priorityQueue: plan.priorityQueue ?? false,
        apiAccess: plan.apiAccess ?? false,
        batchSize: plan.batchSize ?? 1,
      },
    };
  }

  // 获取今日使用量
  private async getDailyUsage(userId: number): Promise<number> {
    const today = new Date().toISOString().split("T")[0];
    const key = REDIS_KEYS.dailyUsage(userId, today);

    const usage = await redis.get(key);
    return usage ? parseInt(usage) : 0;
  }

  // 增加今日使用量
  private async incrementDailyUsage(userId: number): Promise<void> {
    const today = new Date().toISOString().split("T")[0];
    const key = REDIS_KEYS.dailyUsage(userId, today);

    await redis.incr(key);
    await redis.expire(key, REDIS_TTL.dailyUsage);
  }

  // 获取加油包信息
  private async getBoostPackInfo(userId: number) {
    const activePack = await db.query.boostPacks.findFirst({
      where: and(
        eq(boostPacks.userId, userId),
        eq(boostPacks.status, "active"),
        gte(boostPacks.expiresAt, new Date())
      ),
    });

    return {
      balance: activePack?.videosRemaining ?? 0,
      expiresAt: activePack?.expiresAt ?? null,
    };
  }

  // 减少加油包余额
  private async decrementBoostPack(userId: number): Promise<number> {
    const pack = await db.query.boostPacks.findFirst({
      where: and(
        eq(boostPacks.userId, userId),
        eq(boostPacks.status, "active"),
        gte(boostPacks.expiresAt, new Date())
      ),
    });

    if (!pack || pack.videosRemaining <= 0) {
      throw new Error("No boost pack available");
    }

    const newRemaining = pack.videosRemaining - 1;

    await db
      .update(boostPacks)
      .set({
        videosRemaining: newRemaining,
        status: newRemaining <= 0 ? "depleted" : "active",
      })
      .where(eq(boostPacks.id, pack.id));

    // 清除缓存
    await redis.del(REDIS_KEYS.boostBalance(userId));

    return pack.id;
  }

  private getTierFromPlan(
    planName: string
  ): "free" | "basic" | "pro" | "enterprise" {
    const name = planName.toLowerCase();
    if (name.includes("enterprise") || name.includes("企业"))
      return "enterprise";
    if (name.includes("pro") || name.includes("专业")) return "pro";
    if (name.includes("basic") || name.includes("基础")) return "basic";
    return "free";
  }
}

export const quotaService = new QuotaService();
```

---

## 五、前端 API 路由

### 5.1 获取配额

```typescript
// src/app/api/membership/quota/route.ts
import { NextResponse } from "next/server";
import { quotaService } from "@/lib/quota-service";
import { getUserFromRequest } from "@/lib/auth";

export async function GET(request: Request) {
  try {
    const user = await getUserFromRequest(request);
    if (!user) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    const quota = await quotaService.getUserQuota(user.id);

    return NextResponse.json({
      code: 200,
      message: "success",
      data: quota,
    });
  } catch (error) {
    console.error("Get quota error:", error);
    return NextResponse.json({ error: "Internal error" }, { status: 500 });
  }
}
```

### 5.2 创建结账会话

```typescript
// src/app/api/membership/checkout/route.ts
import { NextResponse } from "next/server";
import { createCheckoutSession } from "@/lib/lemonsqueezy";
import { getUserFromRequest } from "@/lib/auth";

const VARIANT_MAP: Record<string, string> = {
  basic: process.env.LEMONSQUEEZY_BASIC_VARIANT_ID!,
  pro: process.env.LEMONSQUEEZY_PRO_VARIANT_ID!,
  enterprise: process.env.LEMONSQUEEZY_ENTERPRISE_VARIANT_ID!,
  "boost-small": process.env.LEMONSQUEEZY_BOOST_SMALL_VARIANT_ID!,
  "boost-medium": process.env.LEMONSQUEEZY_BOOST_MEDIUM_VARIANT_ID!,
  "boost-large": process.env.LEMONSQUEEZY_BOOST_LARGE_VARIANT_ID!,
};

export async function POST(request: Request) {
  try {
    const user = await getUserFromRequest(request);
    if (!user) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    const { planId, type } = await request.json();

    let variantId: string;
    if (type === "boost") {
      variantId = VARIANT_MAP[`boost-${planId}`];
    } else {
      variantId = VARIANT_MAP[planId];
    }

    if (!variantId) {
      return NextResponse.json({ error: "Invalid plan" }, { status: 400 });
    }

    const checkoutUrl = await createCheckoutSession({
      variantId,
      userId: user.id,
      deviceId: user.deviceId,
      email: user.email,
      redirectUrl: `${process.env.NEXT_PUBLIC_APP_URL}/payment/success`,
    });

    return NextResponse.json({
      code: 200,
      message: "success",
      data: { checkoutUrl },
    });
  } catch (error) {
    console.error("Create checkout error:", error);
    return NextResponse.json({ error: "Internal error" }, { status: 500 });
  }
}
```

---

## 六、前端组件

### 6.1 配额状态组件

```tsx
// src/components/QuotaStatus.tsx
"use client";

import { useEffect, useState } from "react";
import { UserQuota } from "@/lib/quota-service";

export function QuotaStatus() {
  const [quota, setQuota] = useState<UserQuota | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchQuota();
  }, []);

  const fetchQuota = async () => {
    try {
      const res = await fetch("/api/membership/quota");
      const data = await res.json();
      if (data.code === 200) {
        setQuota(data.data);
      }
    } catch (error) {
      console.error("Failed to fetch quota:", error);
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return <div className="animate-pulse h-32 bg-gray-100 rounded-lg" />;
  }

  if (!quota) {
    return null;
  }

  const usagePercent = (quota.dailyUsed / quota.dailyLimit) * 100;

  return (
    <div className="bg-white rounded-xl shadow-sm border p-6">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold">使用配额</h3>
        <span
          className={`px-3 py-1 rounded-full text-sm font-medium ${
            quota.tier === "free"
              ? "bg-gray-100 text-gray-700"
              : quota.tier === "basic"
              ? "bg-blue-100 text-blue-700"
              : quota.tier === "pro"
              ? "bg-purple-100 text-purple-700"
              : "bg-gradient-to-r from-purple-500 to-pink-500 text-white"
          }`}
        >
          {quota.tierName}
        </span>
      </div>

      {/* 每日配额进度条 */}
      <div className="mb-4">
        <div className="flex justify-between text-sm text-gray-600 mb-2">
          <span>今日配额</span>
          <span>
            {quota.dailyUsed} / {quota.dailyLimit}
          </span>
        </div>
        <div className="h-2 bg-gray-100 rounded-full overflow-hidden">
          <div
            className={`h-full transition-all ${
              usagePercent >= 90
                ? "bg-red-500"
                : usagePercent >= 70
                ? "bg-yellow-500"
                : "bg-green-500"
            }`}
            style={{ width: `${Math.min(usagePercent, 100)}%` }}
          />
        </div>
      </div>

      {/* 加油包余额 */}
      {quota.boostPackBalance > 0 && (
        <div className="flex items-center justify-between py-3 border-t">
          <div className="flex items-center gap-2">
            <span className="text-orange-500">⚡</span>
            <span className="text-sm text-gray-600">加油包余额</span>
          </div>
          <div className="text-right">
            <span className="font-semibold">{quota.boostPackBalance}</span>
            <span className="text-sm text-gray-500 ml-1">个视频</span>
            {quota.boostPackExpire && (
              <p className="text-xs text-gray-400">
                {formatExpireDate(quota.boostPackExpire)} 到期
              </p>
            )}
          </div>
        </div>
      )}

      {/* 总剩余 */}
      <div className="mt-4 p-4 bg-gray-50 rounded-lg">
        <div className="flex items-center justify-between">
          <span className="text-gray-600">今日可用</span>
          <span className="text-2xl font-bold text-gray-900">
            {quota.totalRemaining}
          </span>
        </div>
      </div>

      {/* 会员到期提醒 */}
      {quota.membershipExpire && (
        <p className="mt-4 text-sm text-gray-500 text-center">
          会员有效期至 {formatExpireDate(quota.membershipExpire)}
        </p>
      )}
    </div>
  );
}

function formatExpireDate(date: Date | string): string {
  const d = new Date(date);
  return d.toLocaleDateString("zh-CN", {
    year: "numeric",
    month: "long",
    day: "numeric",
  });
}
```

---

## 七、Go 后端集成

### 7.1 配额检查中间件

```go
// internal/membership/middleware.go
package membership

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/redis/go-redis/v9"
)

type QuotaMiddleware struct {
    redis *redis.Client
}

func NewQuotaMiddleware(redisClient *redis.Client) *QuotaMiddleware {
    return &QuotaMiddleware{redis: redisClient}
}

func (m *QuotaMiddleware) CheckQuota() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        if userID == "" {
            c.JSON(http.StatusUnauthorized, gin.H{
                "code":    401,
                "message": "未登录",
            })
            c.Abort()
            return
        }

        // 从 Redis 获取配额缓存
        cacheKey := fmt.Sprintf("ytb2bili:quota:%s", userID)
        cached, err := m.redis.Get(c.Request.Context(), cacheKey).Result()

        var quota UserQuota
        if err == nil {
            json.Unmarshal([]byte(cached), &quota)
        } else {
            // 缓存不存在，调用配额服务获取
            // 这里可以调用 Next.js API 或直接查询数据库
            quota, err = m.fetchQuotaFromDB(c.Request.Context(), userID)
            if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{
                    "code":    500,
                    "message": "获取配额失败",
                })
                c.Abort()
                return
            }
        }

        if quota.TotalRemaining <= 0 {
            c.JSON(http.StatusForbidden, gin.H{
                "code":    403,
                "message": "今日配额已用完，请升级会员或购买加油包",
                "data": gin.H{
                    "need_upgrade":   true,
                    "quota":          quota,
                },
            })
            c.Abort()
            return
        }

        // 将配额信息存入上下文
        c.Set("quota", quota)
        c.Next()
    }
}
```

---

## 八、部署清单

### 8.1 Lemon Squeezy 配置

1. **创建产品**

   - 基础版 (月付 ¥29)
   - 专业版 (月付 ¥99)
   - 企业版 (月付 ¥299)
   - 小加油包 (一次性 ¥9.9)
   - 中加油包 (一次性 ¥19.9)
   - 大加油包 (一次性 ¥39.9)

2. **配置 Webhook**
   - URL: `https://your-domain.com/api/webhook`
   - 事件: `subscription_created`, `subscription_updated`, `subscription_cancelled`, `order_created`

### 8.2 数据库迁移

```bash
# 使用 Drizzle 迁移
npx drizzle-kit generate:pg
npx drizzle-kit push:pg
```

### 8.3 环境变量

确保在 Vercel/生产环境配置所有必要的环境变量。

---

## 九、测试清单

- [ ] 会员购买流程
- [ ] 加油包购买流程
- [ ] Webhook 签名验证
- [ ] 配额计算准确性
- [ ] 加油包叠加逻辑
- [ ] 会员续费逻辑
- [ ] 缓存失效机制
- [ ] 并发配额消耗

---

## 十、总结

本方案的核心优势：

1. **支付安全** - Lemon Squeezy 处理所有支付，无需自建支付系统
2. **实时配额** - Redis 缓存确保配额检查高性能
3. **灵活计费** - 支持订阅 + 一次性购买（加油包）
4. **可扩展** - 易于添加新的会员等级和功能
5. **国际化** - Lemon Squeezy 支持全球支付和自动税务处理

建议按以下顺序实施：

1. 数据库 Schema 和 Redis 配置
2. Lemon Squeezy 产品配置
3. Webhook 处理
4. 配额服务
5. 前端集成
6. Go 后端集成
