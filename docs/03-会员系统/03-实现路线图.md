# YTB2BILI 会员系统实施路线图

> 详尽的实施顺序、代码结构、关键设计和最佳实践指南

---

## 目录

1. [项目概述](#一项目概述)
2. [架构设计](#二架构设计)
3. [Phase 1: 基础设施层](#三phase-1-基础设施层)
4. [Phase 2: 核心服务层](#四phase-2-核心服务层)
5. [Phase 3: 集成层](#五phase-3-集成层)
6. [Phase 4: 前端集成](#六phase-4-前端集成)
7. [Phase 5: 支付集成](#七phase-5-支付集成)
8. [测试策略](#八测试策略)
9. [部署与运维](#九部署与运维)
10. [扩展性设计](#十扩展性设计)

---

## 一、项目概述

### 1.1 目标

为 YTB2BILI 项目实现完整的会员体系：

- 4 个会员等级（免费、基础、专业、企业）
- 功能权限控制（AI 翻译、Gemini 分析、自动上传等）
- 配额管理（每日视频数、批量提交数）
- 加油包系统（额外配额购买）
- 支付集成（Lemon Squeezy）

### 1.2 技术栈

| 组件      | 技术选型          | 说明               |
| --------- | ----------------- | ------------------ |
| 后端框架  | Go + Gin          | 现有项目技术栈     |
| 依赖注入  | uber-go/fx        | 现有项目使用       |
| 缓存/计数 | Redis             | 配额计数、状态缓存 |
| 持久化    | PostgreSQL/SQLite | 会员订阅记录       |
| 支付      | Lemon Squeezy     | 国际支付平台       |
| 前端      | Next.js + React   | 会员页面           |

### 1.3 实施时间表（约 3 周）

```
Week 1: 基础设施 + 核心服务
├── Day 1-2: 数据结构 + 存储实现
├── Day 3-4: 检查服务 + 配额服务
└── Day 5:   单元测试

Week 2: 后端集成
├── Day 1-2: 中间件 + 配额集成
├── Day 3:   功能开关集成
├── Day 4:   加油包服务
└── Day 5:   集成测试

Week 3: 前端 + 支付 + 部署
├── Day 1-2: 前端组件 + 集成
├── Day 3:   支付集成
├── Day 4:   E2E 测试
└── Day 5:   部署上线
```

---

## 二、架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        会员系统架构                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐                                                │
│  │ 前端 Next.js │                                                │
│  └──────┬──────┘                                                │
│         ▼                                                       │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    API 层 (Gin)                          │   │
│  │  会员API │ 配额API │ 加油包API │ Webhook                  │   │
│  └─────────────────────────────────────────────────────────┘   │
│         ▼                                                       │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    中间件层                               │   │
│  │  AuthMiddleware │ QuotaMiddleware │ FeatureMiddleware    │   │
│  └─────────────────────────────────────────────────────────┘   │
│         ▼                                                       │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    服务层                                 │   │
│  │  FeatureChecker │ QuotaService │ BoostPackService        │   │
│  │  MembershipService │ PaymentService                      │   │
│  └─────────────────────────────────────────────────────────┘   │
│         ▼                                                       │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    存储层                                 │   │
│  │  Redis (缓存/计数) │ PostgreSQL (持久化) │ Lemon Squeezy  │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 目录结构

```
internal/
├── membership/                    # 会员模块（新增）
│   ├── tier.go                   # 等级定义和配置
│   ├── store.go                  # 存储接口定义
│   ├── redis_store.go            # Redis 存储实现
│   ├── checker.go                # 功能检查器
│   ├── quota.go                  # 配额服务
│   ├── boost_pack.go             # 加油包服务
│   └── payment/                  # 支付子模块
│       ├── lemon_squeezy.go
│       └── webhook.go
│
├── handler/
│   ├── middleware/
│   │   ├── quota_middleware.go   # 配额中间件（新增）
│   │   └── feature_middleware.go # 功能中间件（新增）
│   ├── membership_handler.go     # 会员 API（新增）
│   └── boost_pack_handler.go     # 加油包 API（新增）
│
└── chain_task/handlers/
    ├── translate_subtitle.go     # 添加功能检查（修改）
    └── generate_metadata.go      # 添加功能检查（修改）
```

---

## 三、Phase 1: 基础设施层

> **目标**：建立会员系统的数据基础，不影响现有功能
> **时间**：2-3 天

### 3.1 等级定义 (tier.go)

```go
// internal/membership/tier.go
package membership

import "time"

// Tier 会员等级
type Tier string

const (
    TierFree       Tier = "free"
    TierBasic      Tier = "basic"
    TierPro        Tier = "pro"
    TierEnterprise Tier = "enterprise"
)

// Limits 配额限制
type Limits struct {
    VideosPerDay int `json:"videos_per_day"` // -1 表示无限
    BatchSize    int `json:"batch_size"`
}

// Features 功能开关
type Features struct {
    AITranslation       bool `json:"ai_translation"`
    TranslationOptimize bool `json:"translation_optimize"`
    AITitleGeneration   bool `json:"ai_title_generation"`
    GeminiVideoAnalysis bool `json:"gemini_video_analysis"`
    AutoUpload          bool `json:"auto_upload"`
    PriorityQueue       bool `json:"priority_queue"`
    APIAccess           bool `json:"api_access"`
    CustomTemplate      bool `json:"custom_template"`
    DataExport          bool `json:"data_export"`
    TeamCollaboration   bool `json:"team_collaboration"`
}

// TierConfig 等级配置
type TierConfig struct {
    Tier        Tier     `json:"tier"`
    Name        string   `json:"name"`
    Price       float64  `json:"price"`
    YearlyPrice float64  `json:"yearly_price"`
    Limits      Limits   `json:"limits"`
    Features    Features `json:"features"`
    Priority    int      `json:"priority"`
}

// DefaultTierConfigs 默认等级配置
var DefaultTierConfigs = map[Tier]TierConfig{
    TierFree: {
        Tier: TierFree, Name: "免费版", Price: 0,
        Limits:   Limits{VideosPerDay: 5, BatchSize: 1},
        Features: Features{}, // 全部 false
        Priority: 0,
    },
    TierBasic: {
        Tier: TierBasic, Name: "基础版", Price: 29, YearlyPrice: 290,
        Limits: Limits{VideosPerDay: 20, BatchSize: 5},
        Features: Features{
            AITranslation: true, AITitleGeneration: true, CustomTemplate: true,
        },
        Priority: 1,
    },
    TierPro: {
        Tier: TierPro, Name: "专业版", Price: 99, YearlyPrice: 990,
        Limits: Limits{VideosPerDay: 100, BatchSize: 20},
        Features: Features{
            AITranslation: true, TranslationOptimize: true,
            AITitleGeneration: true, GeminiVideoAnalysis: true,
            AutoUpload: true, PriorityQueue: true,
            CustomTemplate: true, DataExport: true,
        },
        Priority: 2,
    },
    TierEnterprise: {
        Tier: TierEnterprise, Name: "企业版", Price: 299, YearlyPrice: 2990,
        Limits: Limits{VideosPerDay: -1, BatchSize: 100},
        Features: Features{
            AITranslation: true, TranslationOptimize: true,
            AITitleGeneration: true, GeminiVideoAnalysis: true,
            AutoUpload: true, PriorityQueue: true,
            APIAccess: true, CustomTemplate: true,
            DataExport: true, TeamCollaboration: true,
        },
        Priority: 3,
    },
}

// UserMembership 用户会员状态
type UserMembership struct {
    UserID         string    `json:"user_id"`
    Tier           Tier      `json:"tier"`
    ExpiresAt      time.Time `json:"expires_at"`
    SubscriptionID string    `json:"subscription_id,omitempty"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
}

// IsExpired 检查会员是否过期
func (m *UserMembership) IsExpired() bool {
    if m.Tier == TierFree {
        return false
    }
    return time.Now().After(m.ExpiresAt)
}

// GetEffectiveTier 获取有效等级
func (m *UserMembership) GetEffectiveTier() Tier {
    if m.IsExpired() {
        return TierFree
    }
    return m.Tier
}

// GetConfig 获取当前有效的等级配置
func (m *UserMembership) GetConfig() TierConfig {
    tier := m.GetEffectiveTier()
    if config, ok := DefaultTierConfigs[tier]; ok {
        return config
    }
    return DefaultTierConfigs[TierFree]
}
```

### 3.2 存储接口 (store.go)

```go
// internal/membership/store.go
package membership

import (
    "context"
    "time"
)

// MembershipStore 会员存储接口
type MembershipStore interface {
    GetUserMembership(ctx context.Context, userID string) (*UserMembership, error)
    SaveUserMembership(ctx context.Context, m *UserMembership) error
    GetDailyUsage(ctx context.Context, userID string, date string) (int, error)
    IncrDailyUsage(ctx context.Context, userID string, date string) (int, error)
    GetUserBoostPack(ctx context.Context, userID string) (*UserBoostPack, error)
    SaveUserBoostPack(ctx context.Context, pack *UserBoostPack) error
    DecrBoostPack(ctx context.Context, userID string) error
}

// BoostPackType 加油包类型
type BoostPackType string

const (
    BoostPackSmall  BoostPackType = "small"
    BoostPackMedium BoostPackType = "medium"
    BoostPackLarge  BoostPackType = "large"
)

// BoostPackConfig 加油包配置
type BoostPackConfig struct {
    Type        BoostPackType `json:"type"`
    Name        string        `json:"name"`
    Price       float64       `json:"price"`
    Videos      int           `json:"videos"`
    ValidDays   int           `json:"valid_days"`
    Description string        `json:"description"`
}

var DefaultBoostPackConfigs = map[BoostPackType]BoostPackConfig{
    BoostPackSmall:  {Type: BoostPackSmall, Name: "小加油包", Price: 6.9, Videos: 10, ValidDays: 7},
    BoostPackMedium: {Type: BoostPackMedium, Name: "中加油包", Price: 19.9, Videos: 30, ValidDays: 15},
    BoostPackLarge:  {Type: BoostPackLarge, Name: "大加油包", Price: 39.9, Videos: 80, ValidDays: 30},
}

// UserBoostPack 用户加油包状态
type UserBoostPack struct {
    UserID          string    `json:"user_id"`
    VideosRemaining int       `json:"videos_remaining"`
    ExpiresAt       time.Time `json:"expires_at"`
    LastPurchaseAt  time.Time `json:"last_purchase_at"`
}

func (b *UserBoostPack) IsValid() bool {
    return b.VideosRemaining > 0 && time.Now().Before(b.ExpiresAt)
}
```

### 3.3 Redis 存储实现 (redis_store.go)

```go
// internal/membership/redis_store.go
package membership

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    "github.com/redis/go-redis/v9"
)

const (
    keyPrefixMembership = "ytb2bili:membership:"
    keyPrefixUsage      = "ytb2bili:usage:"
    keyPrefixBoostPack  = "ytb2bili:boost:"
    membershipCacheTTL  = 5 * time.Minute
    usageKeyTTL         = 48 * time.Hour
)

type RedisMembershipStore struct {
    client *redis.Client
}

func NewRedisMembershipStore(client *redis.Client) *RedisMembershipStore {
    return &RedisMembershipStore{client: client}
}

func (s *RedisMembershipStore) GetUserMembership(ctx context.Context, userID string) (*UserMembership, error) {
    key := keyPrefixMembership + userID
    data, err := s.client.Get(ctx, key).Bytes()
    if err == redis.Nil {
        return &UserMembership{UserID: userID, Tier: TierFree, CreatedAt: time.Now()}, nil
    }
    if err != nil {
        return nil, err
    }
    var m UserMembership
    json.Unmarshal(data, &m)
    return &m, nil
}

func (s *RedisMembershipStore) SaveUserMembership(ctx context.Context, m *UserMembership) error {
    key := keyPrefixMembership + m.UserID
    m.UpdatedAt = time.Now()
    data, _ := json.Marshal(m)
    ttl := membershipCacheTTL
    if !m.ExpiresAt.IsZero() && m.ExpiresAt.After(time.Now()) {
        ttl = time.Until(m.ExpiresAt) + time.Hour
    }
    return s.client.Set(ctx, key, data, ttl).Err()
}

func (s *RedisMembershipStore) GetDailyUsage(ctx context.Context, userID, date string) (int, error) {
    key := fmt.Sprintf("%s%s:%s", keyPrefixUsage, userID, date)
    count, err := s.client.Get(ctx, key).Int()
    if err == redis.Nil {
        return 0, nil
    }
    return count, err
}

func (s *RedisMembershipStore) IncrDailyUsage(ctx context.Context, userID, date string) (int, error) {
    key := fmt.Sprintf("%s%s:%s", keyPrefixUsage, userID, date)
    pipe := s.client.Pipeline()
    incr := pipe.Incr(ctx, key)
    pipe.Expire(ctx, key, usageKeyTTL)
    pipe.Exec(ctx)
    return int(incr.Val()), nil
}

func (s *RedisMembershipStore) GetUserBoostPack(ctx context.Context, userID string) (*UserBoostPack, error) {
    key := keyPrefixBoostPack + userID
    data, err := s.client.Get(ctx, key).Bytes()
    if err == redis.Nil {
        return &UserBoostPack{UserID: userID, VideosRemaining: 0}, nil
    }
    if err != nil {
        return nil, err
    }
    var pack UserBoostPack
    json.Unmarshal(data, &pack)
    return &pack, nil
}

func (s *RedisMembershipStore) SaveUserBoostPack(ctx context.Context, pack *UserBoostPack) error {
    key := keyPrefixBoostPack + pack.UserID
    data, _ := json.Marshal(pack)
    ttl := time.Until(pack.ExpiresAt) + time.Hour
    if ttl < time.Hour {
        ttl = time.Hour
    }
    return s.client.Set(ctx, key, data, ttl).Err()
}

func (s *RedisMembershipStore) DecrBoostPack(ctx context.Context, userID string) error {
    pack, err := s.GetUserBoostPack(ctx, userID)
    if err != nil {
        return err
    }
    if !pack.IsValid() {
        return fmt.Errorf("boost pack invalid")
    }
    pack.VideosRemaining--
    return s.SaveUserBoostPack(ctx, pack)
}
```

---

## 四、Phase 2: 核心服务层

> **目标**：实现功能检查和配额管理的核心逻辑
> **时间**：2-3 天

### 4.1 功能检查器 (checker.go)

```go
// internal/membership/checker.go
package membership

import (
    "context"
    "fmt"
    "time"
)

// CheckResult 检查结果
type CheckResult struct {
    Allowed bool   `json:"allowed"`
    Reason  string `json:"reason,omitempty"`
    Code    string `json:"code,omitempty"`
    Upgrade string `json:"upgrade,omitempty"`
}

type FeatureChecker struct {
    store MembershipStore
}

func NewFeatureChecker(store MembershipStore) *FeatureChecker {
    return &FeatureChecker{store: store}
}

func (c *FeatureChecker) GetUserMembership(ctx context.Context, userID string) (*UserMembership, error) {
    return c.store.GetUserMembership(ctx, userID)
}

// CanUseFeature 检查用户是否可以使用某功能
func (c *FeatureChecker) CanUseFeature(ctx context.Context, userID, feature string) CheckResult {
    membership, err := c.store.GetUserMembership(ctx, userID)
    if err != nil {
        return CheckResult{Allowed: false, Reason: "获取会员信息失败", Code: "MEMBERSHIP_ERROR"}
    }

    config := membership.GetConfig()

    featureMap := map[string]struct {
        enabled bool
        upgrade Tier
        msg     string
    }{
        "ai_translation":       {config.Features.AITranslation, TierBasic, "AI 字幕翻译是付费功能"},
        "translation_optimize": {config.Features.TranslationOptimize, TierPro, "翻译质量优化是专业版功能"},
        "ai_title_generation":  {config.Features.AITitleGeneration, TierBasic, "AI 标题生成是付费功能"},
        "gemini_video_analysis":{config.Features.GeminiVideoAnalysis, TierPro, "Gemini 视频分析是专业版功能"},
        "auto_upload":          {config.Features.AutoUpload, TierPro, "自动上传是专业版功能"},
        "api_access":           {config.Features.APIAccess, TierEnterprise, "API 访问是企业版功能"},
    }

    if f, ok := featureMap[feature]; ok {
        if !f.enabled {
            return CheckResult{
                Allowed: false, Reason: f.msg,
                Code: "FEATURE_NOT_ALLOWED", Upgrade: string(f.upgrade),
            }
        }
    }

    return CheckResult{Allowed: true}
}

// CanProcessVideo 检查配额
func (c *FeatureChecker) CanProcessVideo(ctx context.Context, userID string) CheckResult {
    membership, _ := c.store.GetUserMembership(ctx, userID)
    config := membership.GetConfig()

    if config.Limits.VideosPerDay == -1 {
        return CheckResult{Allowed: true}
    }

    today := time.Now().Format("2006-01-02")
    used, _ := c.store.GetDailyUsage(ctx, userID, today)

    if used < config.Limits.VideosPerDay {
        return CheckResult{Allowed: true}
    }

    boostPack, _ := c.store.GetUserBoostPack(ctx, userID)
    if boostPack.IsValid() {
        return CheckResult{Allowed: true}
    }

    return CheckResult{
        Allowed: false,
        Reason:  fmt.Sprintf("今日配额已用完 (%d/%d)", used, config.Limits.VideosPerDay),
        Code:    "QUOTA_EXCEEDED",
        Upgrade: c.suggestUpgrade(membership.GetEffectiveTier()),
    }
}

// CanBatchSubmit 检查批量提交
func (c *FeatureChecker) CanBatchSubmit(ctx context.Context, userID string, count int) CheckResult {
    membership, _ := c.store.GetUserMembership(ctx, userID)
    config := membership.GetConfig()

    if count > config.Limits.BatchSize {
        return CheckResult{
            Allowed: false,
            Reason:  fmt.Sprintf("批量提交数量超限 (最多 %d 个)", config.Limits.BatchSize),
            Code:    "BATCH_SIZE_EXCEEDED",
            Upgrade: c.suggestUpgrade(membership.GetEffectiveTier()),
        }
    }
    return CheckResult{Allowed: true}
}

func (c *FeatureChecker) GetUserPriority(ctx context.Context, userID string) int {
    membership, _ := c.store.GetUserMembership(ctx, userID)
    return membership.GetConfig().Priority
}

func (c *FeatureChecker) suggestUpgrade(current Tier) string {
    switch current {
    case TierFree:
        return string(TierBasic)
    case TierBasic:
        return string(TierPro)
    case TierPro:
        return string(TierEnterprise)
    }
    return ""
}
```

### 4.2 配额服务 (quota.go)

```go
// internal/membership/quota.go
package membership

import (
    "context"
    "fmt"
    "time"
)

type QuotaService struct {
    store   MembershipStore
    checker *FeatureChecker
}

func NewQuotaService(store MembershipStore, checker *FeatureChecker) *QuotaService {
    return &QuotaService{store: store, checker: checker}
}

type QuotaInfo struct {
    DailyLimit         int  `json:"daily_limit"`
    DailyUsed          int  `json:"daily_used"`
    DailyRemaining     int  `json:"daily_remaining"`
    BoostPackRemaining int  `json:"boost_pack_remaining"`
    TotalRemaining     int  `json:"total_remaining"`
    IsUnlimited        bool `json:"is_unlimited"`
}

func (s *QuotaService) GetQuotaInfo(ctx context.Context, userID string) (*QuotaInfo, error) {
    membership, _ := s.store.GetUserMembership(ctx, userID)
    config := membership.GetConfig()

    if config.Limits.VideosPerDay == -1 {
        return &QuotaInfo{DailyLimit: -1, IsUnlimited: true}, nil
    }

    today := time.Now().Format("2006-01-02")
    used, _ := s.store.GetDailyUsage(ctx, userID, today)

    dailyRemaining := config.Limits.VideosPerDay - used
    if dailyRemaining < 0 {
        dailyRemaining = 0
    }

    boostPack, _ := s.store.GetUserBoostPack(ctx, userID)
    boostRemaining := 0
    if boostPack.IsValid() {
        boostRemaining = boostPack.VideosRemaining
    }

    return &QuotaInfo{
        DailyLimit:         config.Limits.VideosPerDay,
        DailyUsed:          used,
        DailyRemaining:     dailyRemaining,
        BoostPackRemaining: boostRemaining,
        TotalRemaining:     dailyRemaining + boostRemaining,
    }, nil
}

func (s *QuotaService) ConsumeQuota(ctx context.Context, userID string) error {
    check := s.checker.CanProcessVideo(ctx, userID)
    if !check.Allowed {
        return fmt.Errorf(check.Reason)
    }

    membership, _ := s.store.GetUserMembership(ctx, userID)
    config := membership.GetConfig()

    if config.Limits.VideosPerDay == -1 {
        return nil
    }

    today := time.Now().Format("2006-01-02")
    used, _ := s.store.GetDailyUsage(ctx, userID, today)

    if used < config.Limits.VideosPerDay {
        _, err := s.store.IncrDailyUsage(ctx, userID, today)
        return err
    }

    return s.store.DecrBoostPack(ctx, userID)
}
```

---

## 五、Phase 3: 集成层

> **目标**：将会员服务集成到现有代码中
> **时间**：3-4 天

### 5.1 中间件实现

```go
// internal/handler/middleware/feature_middleware.go
package middleware

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/difyz9/ytb2bili/internal/membership"
)

type FeatureMiddleware struct {
    checker *membership.FeatureChecker
}

func NewFeatureMiddleware(checker *membership.FeatureChecker) *FeatureMiddleware {
    return &FeatureMiddleware{checker: checker}
}

func (m *FeatureMiddleware) RequireQuota() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        if userID == "" {
            userID = "default"
        }

        check := m.checker.CanProcessVideo(c.Request.Context(), userID)
        if !check.Allowed {
            c.JSON(http.StatusForbidden, gin.H{
                "code": check.Code, "message": check.Reason, "upgrade": check.Upgrade,
            })
            c.Abort()
            return
        }
        c.Next()
    }
}

func (m *FeatureMiddleware) RequireFeature(feature string) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        if userID == "" {
            userID = "default"
        }

        check := m.checker.CanUseFeature(c.Request.Context(), userID, feature)
        if !check.Allowed {
            c.JSON(http.StatusForbidden, gin.H{
                "code": check.Code, "message": check.Reason,
                "upgrade": check.Upgrade, "feature": feature,
            })
            c.Abort()
            return
        }
        c.Next()
    }
}

func (m *FeatureMiddleware) InjectMembership() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        if userID == "" {
            userID = "default"
        }

        membership, err := m.checker.GetUserMembership(c.Request.Context(), userID)
        if err == nil {
            c.Set("membership", membership)
            c.Set("tier", string(membership.GetEffectiveTier()))
        }
        c.Next()
    }
}
```

### 5.2 任务处理器集成示例

```go
// translate_subtitle.go 修改
func (t *TranslateSubtitle) Execute(context map[string]interface{}) bool {
    userID, _ := context["user_id"].(string)

    // 检查 AI 翻译权限
    check := t.FeatureChecker.CanUseFeature(t.ctx, userID, "ai_translation")
    if !check.Allowed {
        t.App.Logger.Warnf("用户 %s 无 AI 翻译权限，跳过翻译", userID)
        context["skip_translation"] = true
        return true // 继续后续步骤
    }

    // 继续原有翻译逻辑...
    return t.doTranslation(context)
}

// generate_metadata.go 修改
func (g *GenerateMetadata) Execute(context map[string]interface{}) bool {
    userID, _ := context["user_id"].(string)
    membership, _ := g.FeatureChecker.GetUserMembership(g.ctx, userID)
    config := membership.GetConfig()

    // 免费用户：使用原始元数据
    if !config.Features.AITitleGeneration {
        return g.useOriginalMetadata(context)
    }

    // 专业版+：使用 Gemini 多模态
    if config.Features.GeminiVideoAnalysis && g.App.Config.GeminiConfig.Enabled {
        return g.generateWithGemini(context)
    }

    // 基础版：使用文本 AI
    return g.generateWithTextAI(context)
}
```

### 5.3 依赖注入配置 (main.go)

```go
func main() {
    fx.New(
        // 现有模块...

        // 会员模块
        fx.Provide(
            func(config *types.AppConfig) (*redis.Client, error) {
                if !config.Membership.Enabled {
                    return nil, nil
                }
                return redis.NewClient(&redis.Options{
                    Addr:     config.Membership.Redis.Addr,
                    Password: config.Membership.Redis.Password,
                    DB:       config.Membership.Redis.DB,
                }), nil
            },
            func(client *redis.Client) membership.MembershipStore {
                if client == nil {
                    return membership.NewMockStore()
                }
                return membership.NewRedisMembershipStore(client)
            },
            membership.NewFeatureChecker,
            membership.NewQuotaService,
            membership.NewBoostPackService,
            middleware.NewFeatureMiddleware,
        ),
        fx.Invoke(registerHandlers),
    ).Run()
}
```

---

## 六、Phase 4: 前端集成

> **时间**：2-3 天

### 6.1 React Hook

```tsx
// hooks/useMembership.ts
import { useState, useEffect } from "react";

export function useMembership() {
  const [membership, setMembership] = useState(null);
  const [quota, setQuota] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      fetch("/api/membership/status").then((r) => r.json()),
      fetch("/api/membership/quota").then((r) => r.json()),
    ]).then(([m, q]) => {
      setMembership(m.data);
      setQuota(q.data);
      setLoading(false);
    });
  }, []);

  return { membership, quota, loading };
}

export function useCanUseFeature(feature: string) {
  const { membership } = useMembership();
  if (!membership) return false;

  const featureMap = {
    ai_translation: ["basic", "pro", "enterprise"],
    gemini_video_analysis: ["pro", "enterprise"],
    auto_upload: ["pro", "enterprise"],
    api_access: ["enterprise"],
  };

  return featureMap[feature]?.includes(membership.tier) ?? false;
}
```

### 6.2 功能门控组件

```tsx
// components/FeatureGate.tsx
export function FeatureGate({ feature, children, fallback }) {
  const canUse = useCanUseFeature(feature);

  if (canUse) return children;

  return (
    fallback || (
      <div className="p-4 bg-gray-100 rounded-lg text-center">
        <Lock className="w-8 h-8 mx-auto text-gray-400" />
        <p className="mt-2 text-gray-600">此功能需要升级会员</p>
        <Link href="/pricing" className="mt-2 text-blue-500">
          查看套餐
        </Link>
      </div>
    )
  );
}
```

### 6.3 配额显示组件

```tsx
// components/QuotaIndicator.tsx
export function QuotaIndicator() {
  const { quota, loading } = useMembership();

  if (loading) return <Skeleton />;
  if (quota.is_unlimited) return <span className="text-green-500">无限</span>;

  const percent = (quota.daily_used / quota.daily_limit) * 100;

  return (
    <div className="flex items-center gap-2">
      <div className="w-24 h-2 bg-gray-200 rounded-full">
        <div
          className={`h-full rounded-full ${
            percent > 80 ? "bg-red-500" : "bg-blue-500"
          }`}
          style={{ width: `${Math.min(percent, 100)}%` }}
        />
      </div>
      <span className="text-sm">
        {quota.daily_remaining}/{quota.daily_limit}
      </span>
      {quota.boost_pack_remaining > 0 && (
        <span className="text-orange-500">+{quota.boost_pack_remaining}</span>
      )}
    </div>
  );
}
```

---

## 七、Phase 5: 支付集成

> **时间**：1-2 天

### 7.1 Lemon Squeezy Webhook

```go
// internal/handler/webhook_handler.go
func (h *WebhookHandler) HandleLemonSqueezy(c *gin.Context) {
    // 验证签名
    signature := c.GetHeader("X-Signature")
    body, _ := io.ReadAll(c.Request.Body)

    if !h.verifySignature(body, signature) {
        c.JSON(401, gin.H{"error": "Invalid signature"})
        return
    }

    var event LemonSqueezyEvent
    json.Unmarshal(body, &event)

    switch event.Meta.EventName {
    case "subscription_created", "subscription_updated":
        h.handleSubscription(c.Request.Context(), event)
    case "subscription_cancelled":
        h.handleCancellation(c.Request.Context(), event)
    case "order_created":
        h.handleBoostPackPurchase(c.Request.Context(), event)
    }

    c.JSON(200, gin.H{"received": true})
}
```

---

## 八、测试策略

### 8.1 单元测试

```go
// membership/checker_test.go
func TestCanUseFeature(t *testing.T) {
    tests := []struct {
        name    string
        tier    Tier
        feature string
        want    bool
    }{
        {"free_ai_translation", TierFree, "ai_translation", false},
        {"basic_ai_translation", TierBasic, "ai_translation", true},
        {"basic_gemini", TierBasic, "gemini_video_analysis", false},
        {"pro_gemini", TierPro, "gemini_video_analysis", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            store := NewMockStore()
            store.SetUserTier(testUserID, tt.tier)
            checker := NewFeatureChecker(store)

            result := checker.CanUseFeature(context.Background(), testUserID, tt.feature)
            assert.Equal(t, tt.want, result.Allowed)
        })
    }
}
```

### 8.2 集成测试清单

- [ ] 免费用户每日配额限制
- [ ] 配额用尽后加油包消耗
- [ ] 会员过期降级
- [ ] 功能开关正确生效
- [ ] Webhook 正确处理订阅事件

---

## 九、部署与运维

### 9.1 配置清单

```toml
# config.toml
[membership]
enabled = true

[membership.redis]
addr = "redis:6379"
password = ""
db = 1

[membership.lemon_squeezy]
api_key = "${LEMON_SQUEEZY_API_KEY}"
store_id = "${LEMON_SQUEEZY_STORE_ID}"
webhook_secret = "${LEMON_SQUEEZY_WEBHOOK_SECRET}"
```

### 9.2 监控指标

- 会员转化率
- 配额使用率
- 加油包购买量
- 功能使用分布

---

## 十、扩展性设计

### 10.1 新增功能开关

```go
// 1. 在 Features 结构体添加字段
type Features struct {
    // ...
    NewFeature bool `json:"new_feature"`
}

// 2. 在 DefaultTierConfigs 配置
TierPro: { Features: Features{ NewFeature: true } }

// 3. 在 checker.go 添加检查
featureMap["new_feature"] = struct{...}{config.Features.NewFeature, TierPro, "新功能是专业版功能"}
```

### 10.2 新增会员等级

```go
// 1. 添加常量
const TierPremium Tier = "premium"

// 2. 添加配置
DefaultTierConfigs[TierPremium] = TierConfig{...}

// 3. 更新 suggestUpgrade 逻辑
```

### 10.3 新增加油包类型

```go
DefaultBoostPackConfigs[BoostPackXL] = BoostPackConfig{
    Type: "xl", Name: "超大加油包", Price: 69.9, Videos: 200, ValidDays: 60,
}
```

---

## 附录：快速开始命令

```bash
# 1. 创建目录
mkdir -p internal/membership
mkdir -p internal/handler/middleware

# 2. 创建文件
touch internal/membership/{tier,store,redis_store,checker,quota,boost_pack}.go
touch internal/handler/middleware/{quota,feature}_middleware.go
touch internal/handler/{membership,boost_pack,webhook}_handler.go

# 3. 运行测试
go test ./internal/membership/...

# 4. 启动 Redis
docker run -d -p 6379:6379 redis:alpine
```

---

## Phase 6: 认证系统集成 (已完成)

### 6.1 认证架构

系统采用双层认证架构：

```
┌─────────────────────────────────────────────────────────────┐
│                      认证层 (两层)                           │
├─────────────────────────────────────────────────────────────┤
│  1. 服务器级鉴权: appId + appSecret                          │
│     - 用于识别哪个客户端/服务器在调用 API                      │
│     - 类似 API Key 机制                                      │
│     - 适用于: 浏览器插件、第三方集成、多租户场景               │
├─────────────────────────────────────────────────────────────┤
│  2. 用户级认证: JWT                                          │
│     - 用于识别具体是哪个用户                                  │
│     - 登录/注册后颁发 token                                   │
│     - 携带用户信息 (user_id, 等级等)                          │
└─────────────────────────────────────────────────────────────┘
```

### 6.2 数据库模型

```go
// App 应用/客户端模型 (用于 appId + appSecret 鉴权)
type App struct {
    BaseModel
    AppID       string  `gorm:"uniqueIndex;size:64;not null"`
    AppSecret   string  `gorm:"size:128;not null"`
    Name        string  `gorm:"size:100;not null"`
    Description string  `gorm:"size:500"`
    Status      int     `gorm:"default:1"`        // 1=启用, 0=禁用
    RateLimit   int     `gorm:"default:1000"`     // 每日请求限制
    AllowedIPs  string  `gorm:"type:text"`        // IP白名单
    OwnerID     *uint   `gorm:"index"`            // 所属用户
}

// UserToken 用户 Token 记录
type UserToken struct {
    BaseModel
    UserID       uint      `gorm:"index;not null"`
    TokenHash    string    `gorm:"uniqueIndex;size:64;not null"`
    RefreshToken string    `gorm:"size:256"`
    DeviceInfo   string    `gorm:"size:255"`
    IP           string    `gorm:"size:45"`
    ExpiresAt    time.Time
    IsRevoked    bool      `gorm:"default:false"`
}
```

### 6.3 API 端点

#### 用户认证 API

| 方法 | 路径                    | 说明         | 认证要求 |
| ---- | ----------------------- | ------------ | -------- |
| POST | `/api/v1/user/register` | 用户注册     | 无       |
| POST | `/api/v1/user/login`    | 用户登录     | 无       |
| POST | `/api/v1/user/refresh`  | 刷新 Token   | 无       |
| POST | `/api/v1/user/logout`   | 退出登录     | JWT      |
| GET  | `/api/v1/user/me`       | 获取当前用户 | JWT      |

#### App 管理 API

| 方法   | 路径                                 | 说明          |
| ------ | ------------------------------------ | ------------- |
| POST   | `/api/v1/apps`                       | 创建 App      |
| GET    | `/api/v1/apps`                       | 列出 Apps     |
| GET    | `/api/v1/apps/:id`                   | 获取 App 详情 |
| PUT    | `/api/v1/apps/:id`                   | 更新 App      |
| DELETE | `/api/v1/apps/:id`                   | 删除 App      |
| POST   | `/api/v1/apps/:id/regenerate-secret` | 重新生成密钥  |

### 6.4 认证方式

#### 方式 1: App 简单认证 (appId + appSecret)

```bash
curl -X GET "http://localhost:8096/api/v1/membership/tiers" \
  -H "X-App-Id: app_xxx" \
  -H "X-App-Secret: secret_xxx"
```

#### 方式 2: App 签名认证 (更安全)

```bash
# 签名算法: HMAC-SHA256(appId + timestamp + nonce, appSecret)
curl -X GET "http://localhost:8096/api/v1/membership/tiers" \
  -H "X-App-Id: app_xxx" \
  -H "X-Timestamp: 2025-12-01T22:00:00Z" \
  -H "X-Nonce: random_string" \
  -H "X-Signature: computed_signature"
```

#### 方式 3: JWT 用户认证

```bash
# 登录获取 Token
curl -X POST "http://localhost:8096/api/v1/user/login" \
  -H "Content-Type: application/json" \
  -d '{"username": "testuser", "password": "123456"}'

# 使用 Token 访问 API
curl -X GET "http://localhost:8096/api/v1/user/me" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."
```

### 6.5 JWT Token 结构

```json
{
  "user_id": 5,
  "username": "testuser",
  "tier": "free",
  "app_id": "app_xxx",
  "iss": "bili-up",
  "sub": "testuser",
  "exp": 1764687186,
  "nbf": 1764600786,
  "iat": 1764600786
}
```

### 6.6 与会员系统集成

会员中间件已更新，支持从多种来源获取 user_id：

```go
func (m *MembershipMiddleware) getUserID(c *gin.Context) string {
    // 1. 优先从 JWT 认证中间件设置的 context 获取
    if userID, exists := c.Get("user_id"); exists {
        // ...
    }

    // 2. 兼容旧的 context key
    if userID, exists := c.Get(string(ContextKeyUserID)); exists {
        // ...
    }

    // 3. 尝试从 Header 获取 (用于测试/调试)
    if userID := c.GetHeader("X-User-ID"); userID != "" {
        return userID
    }

    return ""
}
```

### 6.7 文件结构

```
internal/auth/
├── jwt.go           # JWT 服务 (生成/解析 Token)
├── app_auth.go      # App 认证工具 (生成 appId/appSecret, 签名验证)
├── middleware.go    # 认证中间件 (AppAuth, JWTAuth)
└── handler.go       # 认证 API Handler
```

### 6.8 配置说明

JWT 配置 (可在 `config.toml` 中配置):

```toml
[jwt]
secret_key = "your-secret-key-change-in-production"
issuer = "bili-up"
access_expiry = "24h"
refresh_expiry = "168h"  # 7 days
```

### 6.9 安全建议

1. **生产环境必须修改 JWT 密钥**
2. **App Secret 只在创建时返回一次**
3. **建议使用签名认证而非简单密钥认证**
4. **设置 IP 白名单限制 App 访问来源**
5. **定期轮换 App Secret**
6. **Token 黑名单机制已实现，支持主动撤销**
