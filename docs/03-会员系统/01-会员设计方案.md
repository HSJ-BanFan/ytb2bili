# YTB2BILI 会员服务设计方案

## 一、需求分析

### 1.1 项目特点

YTB2BILI 是一个视频自动化处理系统，核心功能包括：

- 视频下载（yt-dlp）
- 字幕生成与翻译（AI 服务）
- 元数据生成（Gemini/DeepSeek）
- 视频上传到 Bilibili

**资源消耗分析**：
| 功能 | 消耗资源 | 成本来源 |
|------|----------|----------|
| 字幕翻译 | AI API 调用 | DeepSeek/OpenAI Token 费用 |
| 元数据生成 | AI API 调用 | Gemini/DeepSeek Token 费用 |
| 视频存储 | 腾讯云 COS | 存储 + 流量费用 |
| 服务器 | 计算资源 | 服务器费用 |

### 1.2 用户画像

| 用户类型 | 特征                 | 需求               |
| -------- | -------------------- | ------------------ |
| 轻度用户 | 偶尔搬运 1-2 个视频  | 免费试用，体验功能 |
| 中度用户 | 每周搬运 5-10 个视频 | 稳定使用，合理付费 |
| 重度用户 | 批量搬运，MCN 机构   | 大量使用，高级功能 |

---

## 二、会员体系设计

> 基于 `membership/ytb2bili-membership-web` 前端项目的现有设计

### 2.1 用户等级定义

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                              YTB2BILI 会员体系                                    │
├────────────────┬────────────────┬────────────────┬────────────────┬──────────────┤
│    免费版       │    基础版       │    专业版       │    企业版       │   加油包      │
│   Role: free   │  Role: basic   │   Role: pro    │ Role:enterprise│  (叠加购买)   │
├────────────────┼────────────────┼────────────────┼────────────────┼──────────────┤
│ 每日 5 个视频   │ 每日 20 个视频  │ 每日 100 个视频 │   无限视频      │ +10/30/80个  │
│ 单个视频提交    │ 批量5个提交     │ 批量20个提交    │  批量100个提交  │ 7/15/30天    │
│ 基础字幕下载    │ AI 字幕翻译     │ AI 字幕翻译     │  AI 字幕翻译    │ 可叠加购买   │
│ 社区支持       │ 优先邮件支持    │ 自动上传B站     │  API 接口访问   │              │
│               │ 无广告体验      │ 优先处理队列    │  团队协作功能   │              │
│               │               │ 专属客服支持    │  专属技术支持   │              │
└────────────────┴────────────────┴────────────────┴────────────────┴──────────────┘
```

### 2.2 功能权限矩阵

| 功能           | 免费版 | 基础版 | 专业版 | 企业版 | 说明            |
| -------------- | :----: | :----: | :----: | :----: | --------------- |
| 每日视频处理数 |   5    |   20   |  100   |  无限  | 核心限制        |
| 批量提交大小   |   1    |   5    |   20   |  100   | 单次提交数量    |
| AI 字幕翻译    |   ❌   |   ✅   |   ✅   |   ✅   | DeepSeek/Gemini |
| 自动上传 B 站  |   ❌   |   ❌   |   ✅   |   ✅   | 自动投稿        |
| 优先处理队列   |   ❌   |   ❌   |   ✅   |   ✅   | 队列优先级      |
| API 接口访问   |   ❌   |   ❌   |   ❌   |   ✅   | 开放 API        |
| 团队协作       |   ❌   |   ❌   |   ❌   |   ✅   | 多人协作        |
| 专属客服       |   ❌   |   ❌   |   ✅   |   ✅   | 优先支持        |

### 2.3 定价策略

#### 会员套餐

| 套餐   | 价格    | 原价    | 有效期 |    推荐     |
| ------ | ------- | ------- | ------ | :---------: |
| 免费版 | ¥0      | -       | 永久   |             |
| 基础版 | ¥29/月  | ¥39/月  | 31 天  |             |
| 专业版 | ¥99/月  | ¥129/月 | 31 天  | ⭐ 最受欢迎 |
| 企业版 | ¥299/月 | -       | 31 天  |             |

#### 加油包

| 套餐     | 价格  | 视频额度 | 有效期 |   推荐    |
| -------- | ----- | -------- | ------ | :-------: |
| 小加油包 | ¥9.9  | +10 个   | 7 天   |           |
| 中加油包 | ¥19.9 | +30 个   | 15 天  | ⭐ 最划算 |
| 大加油包 | ¥39.9 | +80 个   | 30 天  |           |

**加油包特点**：

- 所有用户（包括免费用户和会员）都可以购买
- 额度可叠加，多次购买有效期顺延
- 优先消耗每日配额，配额用完后再消耗加油包
- 适合临时需要大量处理视频的用户

---

## 三、技术架构设计

### 3.1 数据存储方案

参考 `设计思考与开发.md` 的 Redis 方案，结合 YTB2BILI 现有架构：

#### 方案 A：纯 Redis 方案（推荐轻量级）

```
┌─────────────────────────────────────────────────────────────────┐
│                         Redis 数据结构                           │
├─────────────────────────────────────────────────────────────────┤
│ 用户每日使用量                                                    │
│ Key: ytb2bili:user:{userID}:date:{YYYY-MM-DD}:video_count       │
│ Value: 已处理视频数 (Integer)                                    │
│ TTL: 10 天                                                       │
├─────────────────────────────────────────────────────────────────┤
│ 会员状态                                                         │
│ Key: ytb2bili:user:{userID}:membership                          │
│ Value: 会员等级 (1=免费, 2=月度, 3=年度)                          │
│ TTL: 会员剩余有效期                                              │
├─────────────────────────────────────────────────────────────────┤
│ 加油包余额                                                       │
│ Key: ytb2bili:user:{userID}:boost_pack                          │
│ Value: 剩余视频额度 (Integer)                                    │
│ TTL: 7 天                                                        │
├─────────────────────────────────────────────────────────────────┤
│ 用户配额缓存                                                     │
│ Key: ytb2bili:user:{userID}:quota_cache                         │
│ Value: JSON {daily_limit, used, remaining, boost}               │
│ TTL: 5 分钟                                                      │
└─────────────────────────────────────────────────────────────────┘
```

#### 方案 B：MySQL + Redis 混合方案（推荐生产环境）

```sql
-- 会员订单表
CREATE TABLE cw_membership_orders (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id VARCHAR(100) NOT NULL,
    order_no VARCHAR(64) UNIQUE NOT NULL,
    plan_type ENUM('monthly', 'yearly', 'boost_pack') NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    status ENUM('pending', 'paid', 'cancelled', 'refunded') DEFAULT 'pending',
    payment_method VARCHAR(32),
    payment_time DATETIME,
    expire_time DATETIME,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id),
    INDEX idx_order_no (order_no),
    INDEX idx_status (status)
);

-- 用户会员信息表
CREATE TABLE cw_user_memberships (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id VARCHAR(100) UNIQUE NOT NULL,
    role TINYINT DEFAULT 1 COMMENT '1=免费, 2=月度, 3=年度',
    expire_time DATETIME,
    boost_pack_balance INT DEFAULT 0,
    boost_pack_expire DATETIME,
    total_videos_processed INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id),
    INDEX idx_expire_time (expire_time)
);

-- 使用记录表（用于统计和审计）
CREATE TABLE cw_usage_logs (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id VARCHAR(100) NOT NULL,
    video_id VARCHAR(100) NOT NULL,
    action_type ENUM('process', 'upload') NOT NULL,
    quota_type ENUM('daily', 'boost_pack') NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_date (user_id, created_at)
);
```

### 3.2 Go 后端实现

#### 3.2.1 目录结构

```
internal/
├── membership/
│   ├── constants.go      # 常量定义
│   ├── models.go         # 数据模型
│   ├── service.go        # 会员服务
│   ├── quota.go          # 配额管理
│   ├── middleware.go     # 权限中间件
│   └── handler.go        # HTTP 处理器
```

#### 3.2.2 常量定义

```go
// internal/membership/constants.go
package membership

import "time"

// 用户角色
type Role int

const (
    RoleFree    Role = 1 // 免费用户
    RoleMonthly Role = 2 // 月度会员
    RoleYearly  Role = 3 // 年度会员
)

// 每日视频处理限制
var DailyLimits = map[Role]int{
    RoleFree:    3,
    RoleMonthly: 30,
    RoleYearly:  50,
}

// 会员有效期
const (
    MonthlyExpire   = 31 * 24 * time.Hour
    YearlyExpire    = 365 * 24 * time.Hour
    BoostPackExpire = 7 * 24 * time.Hour
    BoostPackVideos = 20 // 每个加油包增加的视频数
)

// Redis Key 前缀
const (
    KeyPrefix          = "ytb2bili:"
    KeyUserDailyUsage  = KeyPrefix + "user:%s:date:%s:video_count"
    KeyUserMembership  = KeyPrefix + "user:%s:membership"
    KeyUserBoostPack   = KeyPrefix + "user:%s:boost_pack"
    KeyUserQuotaCache  = KeyPrefix + "user:%s:quota_cache"
)

// 角色名称
var RoleNames = map[Role]string{
    RoleFree:    "免费用户",
    RoleMonthly: "月度会员",
    RoleYearly:  "年度会员",
}
```

#### 3.2.3 会员服务

```go
// internal/membership/service.go
package membership

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

type MembershipService struct {
    redis *redis.Client
    // db    *gorm.DB // 如果使用混合方案
}

func NewMembershipService(redisClient *redis.Client) *MembershipService {
    return &MembershipService{redis: redisClient}
}

// UserQuota 用户配额信息
type UserQuota struct {
    UserID           string    `json:"user_id"`
    Role             Role      `json:"role"`
    RoleName         string    `json:"role_name"`
    DailyLimit       int       `json:"daily_limit"`
    DailyUsed        int       `json:"daily_used"`
    DailyRemaining   int       `json:"daily_remaining"`
    BoostPackBalance int       `json:"boost_pack_balance"`
    TotalRemaining   int       `json:"total_remaining"`
    MembershipExpire int64     `json:"membership_expire"` // 秒
    BoostPackExpire  int64     `json:"boost_pack_expire"` // 秒
}

// GetUserQuota 获取用户配额信息
func (s *MembershipService) GetUserQuota(ctx context.Context, userID string) (*UserQuota, error) {
    // 1. 获取用户角色
    role, membershipTTL, err := s.getUserRole(ctx, userID)
    if err != nil {
        return nil, err
    }

    // 2. 获取每日使用量
    dailyUsed, err := s.getDailyUsage(ctx, userID)
    if err != nil {
        return nil, err
    }

    // 3. 获取加油包余额
    boostBalance, boostTTL, err := s.getBoostPackBalance(ctx, userID)
    if err != nil {
        return nil, err
    }

    // 4. 计算配额
    dailyLimit := DailyLimits[role]
    dailyRemaining := dailyLimit - dailyUsed
    if dailyRemaining < 0 {
        dailyRemaining = 0
    }

    return &UserQuota{
        UserID:           userID,
        Role:             role,
        RoleName:         RoleNames[role],
        DailyLimit:       dailyLimit,
        DailyUsed:        dailyUsed,
        DailyRemaining:   dailyRemaining,
        BoostPackBalance: boostBalance,
        TotalRemaining:   dailyRemaining + boostBalance,
        MembershipExpire: membershipTTL,
        BoostPackExpire:  boostTTL,
    }, nil
}

// CanProcessVideo 检查用户是否可以处理视频
func (s *MembershipService) CanProcessVideo(ctx context.Context, userID string) (bool, string, error) {
    quota, err := s.GetUserQuota(ctx, userID)
    if err != nil {
        return false, "", err
    }

    if quota.TotalRemaining <= 0 {
        if quota.Role == RoleFree {
            return false, "今日免费额度已用完，请升级会员或购买加油包", nil
        }
        return false, "今日额度已用完，请购买加油包获取更多额度", nil
    }

    return true, "", nil
}

// ConsumeQuota 消耗配额（处理视频后调用）
func (s *MembershipService) ConsumeQuota(ctx context.Context, userID string) error {
    quota, err := s.GetUserQuota(ctx, userID)
    if err != nil {
        return err
    }

    // 优先消耗每日配额
    if quota.DailyRemaining > 0 {
        return s.incrDailyUsage(ctx, userID)
    }

    // 其次消耗加油包
    if quota.BoostPackBalance > 0 {
        return s.decrBoostPack(ctx, userID)
    }

    return fmt.Errorf("no quota available")
}

// Upgrade 升级/续费会员
func (s *MembershipService) Upgrade(ctx context.Context, userID string, planType string) error {
    var expire time.Duration
    var role Role

    switch planType {
    case "monthly":
        expire = MonthlyExpire
        role = RoleMonthly
    case "yearly":
        expire = YearlyExpire
        role = RoleYearly
    default:
        return fmt.Errorf("invalid plan type: %s", planType)
    }

    key := fmt.Sprintf(KeyUserMembership, userID)

    // 检查是否已是会员
    currentRole, ttl, _ := s.getUserRole(ctx, userID)
    if currentRole >= role && ttl > 0 {
        // 续费：延长有效期
        newTTL := time.Duration(ttl)*time.Second + expire
        return s.redis.Expire(ctx, key, newTTL).Err()
    }

    // 新购或升级
    return s.redis.Set(ctx, key, int(role), expire).Err()
}

// PurchaseBoostPack 购买加油包
func (s *MembershipService) PurchaseBoostPack(ctx context.Context, userID string) error {
    key := fmt.Sprintf(KeyUserBoostPack, userID)

    // 检查是否已有加油包
    balance, ttl, _ := s.getBoostPackBalance(ctx, userID)

    if balance > 0 && ttl > 0 {
        // 已有加油包，增加余额和延长有效期
        newBalance := balance + BoostPackVideos
        newTTL := time.Duration(ttl)*time.Second + BoostPackExpire
        return s.redis.Set(ctx, key, newBalance, newTTL).Err()
    }

    // 新购加油包
    return s.redis.Set(ctx, key, BoostPackVideos, BoostPackExpire).Err()
}

// 私有方法
func (s *MembershipService) getUserRole(ctx context.Context, userID string) (Role, int64, error) {
    key := fmt.Sprintf(KeyUserMembership, userID)

    val, err := s.redis.Get(ctx, key).Int()
    if err == redis.Nil {
        return RoleFree, 0, nil
    }
    if err != nil {
        return RoleFree, 0, err
    }

    ttl, _ := s.redis.TTL(ctx, key).Result()
    return Role(val), int64(ttl.Seconds()), nil
}

func (s *MembershipService) getDailyUsage(ctx context.Context, userID string) (int, error) {
    today := time.Now().Format("2006-01-02")
    key := fmt.Sprintf(KeyUserDailyUsage, userID, today)

    val, err := s.redis.Get(ctx, key).Int()
    if err == redis.Nil {
        return 0, nil
    }
    return val, err
}

func (s *MembershipService) incrDailyUsage(ctx context.Context, userID string) error {
    today := time.Now().Format("2006-01-02")
    key := fmt.Sprintf(KeyUserDailyUsage, userID, today)

    pipe := s.redis.Pipeline()
    pipe.Incr(ctx, key)
    pipe.Expire(ctx, key, 10*24*time.Hour) // 10天过期
    _, err := pipe.Exec(ctx)
    return err
}

func (s *MembershipService) getBoostPackBalance(ctx context.Context, userID string) (int, int64, error) {
    key := fmt.Sprintf(KeyUserBoostPack, userID)

    val, err := s.redis.Get(ctx, key).Int()
    if err == redis.Nil {
        return 0, 0, nil
    }
    if err != nil {
        return 0, 0, err
    }

    ttl, _ := s.redis.TTL(ctx, key).Result()
    return val, int64(ttl.Seconds()), nil
}

func (s *MembershipService) decrBoostPack(ctx context.Context, userID string) error {
    key := fmt.Sprintf(KeyUserBoostPack, userID)
    return s.redis.Decr(ctx, key).Err()
}
```

#### 3.2.4 权限中间件

```go
// internal/membership/middleware.go
package membership

import (
    "net/http"

    "github.com/gin-gonic/gin"
)

// QuotaCheckMiddleware 配额检查中间件
func QuotaCheckMiddleware(svc *MembershipService) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id") // 从认证中间件获取
        if userID == "" {
            c.JSON(http.StatusUnauthorized, gin.H{
                "code":    401,
                "message": "未登录",
            })
            c.Abort()
            return
        }

        canProcess, reason, err := svc.CanProcessVideo(c.Request.Context(), userID)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{
                "code":    500,
                "message": "检查配额失败",
            })
            c.Abort()
            return
        }

        if !canProcess {
            c.JSON(http.StatusForbidden, gin.H{
                "code":    403,
                "message": reason,
                "data": gin.H{
                    "need_upgrade": true,
                },
            })
            c.Abort()
            return
        }

        c.Next()
    }
}

// PriorityMiddleware 优先级中间件（用于任务队列）
func (svc *MembershipService) GetUserPriority(userID string) int {
    quota, err := svc.GetUserQuota(nil, userID)
    if err != nil {
        return 0 // 最低优先级
    }

    switch quota.Role {
    case RoleYearly:
        return 100 // 最高优先级
    case RoleMonthly:
        return 50
    default:
        return 10
    }
}
```

#### 3.2.5 HTTP 处理器

```go
// internal/membership/handler.go
package membership

import (
    "net/http"

    "github.com/gin-gonic/gin"
)

type MembershipHandler struct {
    svc *MembershipService
}

func NewMembershipHandler(svc *MembershipService) *MembershipHandler {
    return &MembershipHandler{svc: svc}
}

func (h *MembershipHandler) RegisterRoutes(api *gin.RouterGroup) {
    membership := api.Group("/membership")
    {
        membership.GET("/quota", h.GetQuota)
        membership.POST("/upgrade", h.Upgrade)
        membership.POST("/boost-pack", h.PurchaseBoostPack)
        membership.GET("/status", h.GetStatus)
    }
}

// GetQuota 获取用户配额
func (h *MembershipHandler) GetQuota(c *gin.Context) {
    userID := c.GetString("user_id")

    quota, err := h.svc.GetUserQuota(c.Request.Context(), userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "code":    500,
            "message": "获取配额失败",
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "code":    200,
        "message": "success",
        "data":    quota,
    })
}

// Upgrade 升级会员
func (h *MembershipHandler) Upgrade(c *gin.Context) {
    userID := c.GetString("user_id")

    var req struct {
        PlanType string `json:"plan_type" binding:"required,oneof=monthly yearly"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "code":    400,
            "message": "参数错误",
        })
        return
    }

    // TODO: 这里应该先验证支付状态
    // 实际生产中，应该在支付回调中调用 Upgrade

    if err := h.svc.Upgrade(c.Request.Context(), userID, req.PlanType); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "code":    500,
            "message": "升级失败",
        })
        return
    }

    quota, _ := h.svc.GetUserQuota(c.Request.Context(), userID)
    c.JSON(http.StatusOK, gin.H{
        "code":    200,
        "message": "升级成功",
        "data":    quota,
    })
}

// PurchaseBoostPack 购买加油包
func (h *MembershipHandler) PurchaseBoostPack(c *gin.Context) {
    userID := c.GetString("user_id")

    // TODO: 验证支付状态

    if err := h.svc.PurchaseBoostPack(c.Request.Context(), userID); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "code":    500,
            "message": "购买失败",
        })
        return
    }

    quota, _ := h.svc.GetUserQuota(c.Request.Context(), userID)
    c.JSON(http.StatusOK, gin.H{
        "code":    200,
        "message": "购买成功",
        "data":    quota,
    })
}

// GetStatus 获取会员状态
func (h *MembershipHandler) GetStatus(c *gin.Context) {
    userID := c.GetString("user_id")

    quota, err := h.svc.GetUserQuota(c.Request.Context(), userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "code":    500,
            "message": "获取状态失败",
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "code":    200,
        "message": "success",
        "data": gin.H{
            "is_member":          quota.Role > RoleFree,
            "role":               quota.Role,
            "role_name":          quota.RoleName,
            "daily_remaining":    quota.DailyRemaining,
            "boost_remaining":    quota.BoostPackBalance,
            "total_remaining":    quota.TotalRemaining,
            "membership_expire":  quota.MembershipExpire,
            "boost_pack_expire":  quota.BoostPackExpire,
        },
    })
}
```

### 3.3 与现有系统集成

#### 3.3.1 修改任务链处理器

在 `chain_task_handler.go` 中集成配额检查：

```go
// internal/chain_task/chain_task_handler.go

func (h *ChainTaskHandler) RunTaskChain(video *model.SavedVideo) {
    // 1. 检查用户配额
    userID := video.UserID // 假设视频关联了用户
    canProcess, reason, err := h.MembershipService.CanProcessVideo(context.Background(), userID)
    if err != nil || !canProcess {
        h.App.Logger.Warnf("用户 %s 配额不足: %s", userID, reason)
        h.SavedVideoService.UpdateStatus(video.ID, "quota_exceeded")
        return
    }

    // 2. 执行任务链...
    // ... 原有逻辑 ...

    // 3. 任务完成后消耗配额
    if err := h.MembershipService.ConsumeQuota(context.Background(), userID); err != nil {
        h.App.Logger.Errorf("消耗配额失败: %v", err)
    }
}
```

#### 3.3.2 修改上传调度器

根据会员等级调整处理优先级：

```go
// internal/chain_task/upload_scheduler.go

func (s *UploadScheduler) getNextVideoToUpload() (*model.SavedVideo, error) {
    // 获取所有待上传视频
    videos, err := s.SavedVideoService.GetPendingVideos()
    if err != nil {
        return nil, err
    }

    // 按用户优先级排序
    sort.Slice(videos, func(i, j int) bool {
        priorityI := s.MembershipService.GetUserPriority(videos[i].UserID)
        priorityJ := s.MembershipService.GetUserPriority(videos[j].UserID)
        return priorityI > priorityJ
    })

    if len(videos) > 0 {
        return videos[0], nil
    }
    return nil, nil
}
```

---

## 四、前端实现建议

### 4.1 会员状态组件

```tsx
// components/MembershipStatus.tsx
"use client";

import { useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

interface QuotaInfo {
  role: number;
  role_name: string;
  daily_limit: number;
  daily_used: number;
  daily_remaining: number;
  boost_pack_balance: number;
  total_remaining: number;
  membership_expire: number;
  boost_pack_expire: number;
}

export function MembershipStatus() {
  const [quota, setQuota] = useState<QuotaInfo | null>(null);

  useEffect(() => {
    fetch("/api/v1/membership/quota")
      .then((res) => res.json())
      .then((data) => setQuota(data.data));
  }, []);

  if (!quota) return <div>加载中...</div>;

  const usagePercent = (quota.daily_used / quota.daily_limit) * 100;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center justify-between">
          <span>会员状态</span>
          <Badge variant={quota.role > 1 ? "default" : "secondary"}>
            {quota.role_name}
          </Badge>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* 每日配额 */}
        <div>
          <div className="flex justify-between text-sm mb-2">
            <span>今日配额</span>
            <span>
              {quota.daily_used} / {quota.daily_limit}
            </span>
          </div>
          <Progress value={usagePercent} />
        </div>

        {/* 加油包 */}
        {quota.boost_pack_balance > 0 && (
          <div className="flex justify-between text-sm">
            <span>加油包余额</span>
            <span>{quota.boost_pack_balance} 个视频</span>
          </div>
        )}

        {/* 会员到期时间 */}
        {quota.membership_expire > 0 && (
          <div className="flex justify-between text-sm">
            <span>会员到期</span>
            <span>{formatExpireTime(quota.membership_expire)}</span>
          </div>
        )}

        {/* 操作按钮 */}
        <div className="flex gap-2 pt-4">
          {quota.role === 1 && <Button className="flex-1">升级会员</Button>}
          <Button variant="outline" className="flex-1">
            购买加油包
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

function formatExpireTime(seconds: number): string {
  const days = Math.floor(seconds / 86400);
  if (days > 0) return `${days} 天后`;
  const hours = Math.floor(seconds / 3600);
  return `${hours} 小时后`;
}
```

### 4.2 配额不足提示

```tsx
// components/QuotaExceededDialog.tsx
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

interface Props {
  open: boolean;
  onClose: () => void;
  isMember: boolean;
}

export function QuotaExceededDialog({ open, onClose, isMember }: Props) {
  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>今日配额已用完</DialogTitle>
          <DialogDescription>
            {isMember
              ? "您今日的视频处理配额已用完，可以购买加油包获取更多额度。"
              : "免费用户每日可处理 3 个视频，升级会员可享受更多额度。"}
          </DialogDescription>
        </DialogHeader>
        <div className="flex gap-4 mt-4">
          {!isMember && (
            <Button
              className="flex-1"
              onClick={() => {
                /* 跳转升级 */
              }}
            >
              升级会员 (¥29.9/月)
            </Button>
          )}
          <Button
            variant="outline"
            className="flex-1"
            onClick={() => {
              /* 购买加油包 */
            }}
          >
            购买加油包 (¥9.9)
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
```

---

## 五、支付集成建议

### 5.1 国内支付方案

| 方案     | 优点       | 缺点         |
| -------- | ---------- | ------------ |
| 微信支付 | 用户基数大 | 需要企业资质 |
| 支付宝   | 用户基数大 | 需要企业资质 |
| 虎皮椒   | 个人可用   | 手续费较高   |
| 易支付   | 接入简单   | 稳定性一般   |

### 5.2 国际支付方案

| 方案          | 优点       | 缺点         |
| ------------- | ---------- | ------------ |
| Stripe        | 功能强大   | 国内不可用   |
| Lemon Squeezy | 个人友好   | 需要海外账户 |
| Paddle        | 税务处理好 | 费率较高     |

### 5.3 支付回调处理

```go
// internal/handler/payment_handler.go

func (h *PaymentHandler) HandleWebhook(c *gin.Context) {
    // 1. 验证签名
    // 2. 解析订单信息
    // 3. 更新会员状态

    orderNo := c.PostForm("order_no")
    status := c.PostForm("status")

    if status == "paid" {
        order, err := h.OrderService.GetByOrderNo(orderNo)
        if err != nil {
            c.JSON(500, gin.H{"error": "order not found"})
            return
        }

        switch order.PlanType {
        case "monthly", "yearly":
            h.MembershipService.Upgrade(c, order.UserID, order.PlanType)
        case "boost_pack":
            h.MembershipService.PurchaseBoostPack(c, order.UserID)
        }

        h.OrderService.UpdateStatus(orderNo, "paid")
    }

    c.JSON(200, gin.H{"success": true})
}
```

---

## 六、风险控制

### 6.1 防刷策略

```go
// 1. 请求频率限制
func RateLimitMiddleware() gin.HandlerFunc {
    limiter := rate.NewLimiter(rate.Every(time.Second), 10) // 每秒10次
    return func(c *gin.Context) {
        if !limiter.Allow() {
            c.JSON(429, gin.H{"error": "请求过于频繁"})
            c.Abort()
            return
        }
        c.Next()
    }
}

// 2. IP 限制
// 3. 设备指纹
// 4. 行为分析
```

### 6.2 数据一致性

```go
// 使用 Redis 事务确保原子性
func (s *MembershipService) ConsumeQuotaAtomic(ctx context.Context, userID string) error {
    return s.redis.Watch(ctx, func(tx *redis.Tx) error {
        // 检查配额
        quota, err := s.GetUserQuota(ctx, userID)
        if err != nil {
            return err
        }
        if quota.TotalRemaining <= 0 {
            return fmt.Errorf("no quota")
        }

        // 执行消耗
        _, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
            if quota.DailyRemaining > 0 {
                today := time.Now().Format("2006-01-02")
                key := fmt.Sprintf(KeyUserDailyUsage, userID, today)
                pipe.Incr(ctx, key)
            } else {
                key := fmt.Sprintf(KeyUserBoostPack, userID)
                pipe.Decr(ctx, key)
            }
            return nil
        })
        return err
    }, fmt.Sprintf(KeyUserDailyUsage, userID, time.Now().Format("2006-01-02")))
}
```

### 6.3 监控告警

- Redis 连接状态监控
- 配额消耗异常告警
- 支付回调失败告警
- 用户投诉自动处理

---

## 七、实施路线图

### Phase 1: 基础会员系统 (1-2 周)

- [ ] Redis 数据结构设计
- [ ] 会员服务核心逻辑
- [ ] 配额检查中间件
- [ ] 基础 API 接口

### Phase 2: 前端集成 (1 周)

- [ ] 会员状态组件
- [ ] 配额提示弹窗
- [ ] 会员中心页面

### Phase 3: 支付集成 (1-2 周)

- [ ] 选择支付方案
- [ ] 订单系统
- [ ] 支付回调处理

### Phase 4: 优化完善 (持续)

- [ ] 数据统计分析
- [ ] 风控策略
- [ ] 运营工具

---

## 八、总结

本设计方案基于以下原则：

1. **渐进式实现** - 先实现核心配额功能，再逐步添加支付
2. **低耦合** - 会员模块独立，易于维护和扩展
3. **高性能** - 使用 Redis 缓存，减少数据库压力
4. **可扩展** - 支持添加更多会员等级和权益

建议从 Phase 1 开始，先实现配额限制功能，验证业务逻辑后再接入支付。
