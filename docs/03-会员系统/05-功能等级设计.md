# YTB2BILI 会员功能分级限制设计方案

> 详细说明不同会员等级之间的功能限制如何实现

## 一、功能限制矩阵

### 1.1 完整功能对照表

| 功能模块       | 功能点                | 免费版 | 基础版 | 专业版 | 企业版 | 限制类型 |
| -------------- | --------------------- | :----: | :----: | :----: | :----: | -------- |
| **配额限制**   | 每日视频处理数        |   5    |   20   |  100   |   ∞    | 数量限制 |
|                | 批量提交数量          |   1    |   5    |   20   |  100   | 数量限制 |
| **字幕功能**   | 基础字幕下载          |   ✅   |   ✅   |   ✅   |   ✅   | 无限制   |
|                | AI 字幕翻译           |   ❌   |   ✅   |   ✅   |   ✅   | 功能开关 |
|                | 翻译质量优化          |   ❌   |   ❌   |   ✅   |   ✅   | 功能开关 |
| **元数据生成** | 原始元数据            |   ✅   |   ✅   |   ✅   |   ✅   | 无限制   |
|                | AI 文本分析生成       |   ❌   |   ✅   |   ✅   |   ✅   | 功能开关 |
|                | **Gemini 多模态生成** |   ❌   |   ❌   |   ✅   |   ✅   | 功能开关 |
| **上传功能**   | 手动上传              |   ✅   |   ✅   |   ✅   |   ✅   | 无限制   |
|                | 自动定时上传          |   ❌   |   ❌   |   ✅   |   ✅   | 功能开关 |
|                | 上传优先级            |   低   |   中   |   高   |  最高  | 优先级   |
| **高级功能**   | API 接口访问          |   ❌   |   ❌   |   ❌   |   ✅   | 功能开关 |
|                | 自定义模板            |   ❌   |   ✅   |   ✅   |   ✅   | 功能开关 |
|                | 数据导出              |   ❌   |   ❌   |   ✅   |   ✅   | 功能开关 |
|                | 团队协作              |   ❌   |   ❌   |   ❌   |   ✅   | 功能开关 |

> **元数据生成说明**：
>
> - **原始元数据**：直接使用 YouTube 原视频的标题、描述
> - **AI 文本分析生成**：基于字幕文本，使用 DeepSeek/OpenAI 生成标题、描述、标签
> - **Gemini 多模态生成**：分析视频画面内容 + 字幕，生成更精准的元数据（专业版特权）

### 1.2 限制类型说明

| 限制类型     | 说明                     | 实现方式                | 示例                                       |
| ------------ | ------------------------ | ----------------------- | ------------------------------------------ |
| **数量限制** | 限制某功能的使用次数     | Redis 计数器 + 配额检查 | 每日视频数、批量提交数                     |
| **功能开关** | 完全启用或禁用某功能     | Feature Flag + 权限检查 | AI 翻译、Gemini 元数据、自动上传、API 访问 |
| **优先级**   | 影响处理顺序，不影响功能 | 任务队列优先级排序      | 处理队列优先级                             |

### 1.3 功能开关详细说明

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         功能开关 (Feature Flags)                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐       │
│  │   AI 字幕翻译    │   │ Gemini多模态生成 │   │   自动定时上传   │       │
│  │  (基础版起)      │   │   (专业版起)     │   │   (专业版起)     │       │
│  └─────────────────┘   └─────────────────┘   └─────────────────┘       │
│                                                                         │
│  ┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐       │
│  │   自定义模板     │   │    数据导出      │   │   API 接口访问   │       │
│  │  (基础版起)      │   │   (专业版起)     │   │   (企业版)       │       │
│  └─────────────────┘   └─────────────────┘   └─────────────────┘       │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**Gemini 多模态元数据生成说明**：

- 免费/基础版：使用文本分析（从字幕提取关键信息）生成标题、描述、标签
- 专业版/企业版：使用 Gemini 多模态分析视频内容，生成更精准的元数据

---

## 二、核心数据结构设计

### 2.1 会员等级定义

```go
// internal/membership/tier.go
package membership

// Tier 会员等级
type Tier string

const (
    TierFree       Tier = "free"
    TierBasic      Tier = "basic"
    TierPro        Tier = "pro"
    TierEnterprise Tier = "enterprise"
)

// TierConfig 等级配置
type TierConfig struct {
    Name     string `json:"name"`
    Priority int    `json:"priority"` // 处理优先级 1-100
    Limits   Limits `json:"limits"`
    Features Features `json:"features"`
}

// Limits 数量限制
type Limits struct {
    VideosPerDay int `json:"videos_per_day"` // 每日视频数，-1 表示无限
    BatchSize    int `json:"batch_size"`     // 批量提交数量
}

// Features 功能开关
type Features struct {
    // 字幕功能
    AITranslation       bool `json:"ai_translation"`        // AI 字幕翻译
    TranslationOptimize bool `json:"translation_optimize"`  // 翻译质量优化

    // 元数据生成
    AITitleGeneration   bool `json:"ai_title_generation"`   // AI 标题生成
    AIDescGeneration    bool `json:"ai_desc_generation"`    // AI 描述生成
    GeminiVideoAnalysis bool `json:"gemini_video_analysis"` // Gemini 视频分析

    // 上传功能
    AutoUpload          bool `json:"auto_upload"`           // 自动定时上传

    // 高级功能
    APIAccess           bool `json:"api_access"`            // API 接口访问
    CustomTemplate      bool `json:"custom_template"`       // 自定义模板
    DataExport          bool `json:"data_export"`           // 数据导出
    TeamCollaboration   bool `json:"team_collaboration"`    // 团队协作
}

// DefaultTierConfigs 默认等级配置
var DefaultTierConfigs = map[Tier]TierConfig{
    TierFree: {
        Name:     "免费版",
        Priority: 10,
        Limits: Limits{
            VideosPerDay: 5,
            BatchSize:    1,
        },
        Features: Features{
            AITranslation:       false,
            TranslationOptimize: false,
            AITitleGeneration:   false,
            AIDescGeneration:    false,
            GeminiVideoAnalysis: false,
            AutoUpload:          false,
            APIAccess:           false,
            CustomTemplate:      false,
            DataExport:          false,
            TeamCollaboration:   false,
        },
    },
    TierBasic: {
        Name:     "基础版",
        Priority: 30,
        Limits: Limits{
            VideosPerDay: 20,
            BatchSize:    5,
        },
        Features: Features{
            AITranslation:       true,
            TranslationOptimize: false,
            AITitleGeneration:   true,
            AIDescGeneration:    true,
            GeminiVideoAnalysis: false,
            AutoUpload:          false,
            APIAccess:           false,
            CustomTemplate:      true,
            DataExport:          false,
            TeamCollaboration:   false,
        },
    },
    TierPro: {
        Name:     "专业版",
        Priority: 70,
        Limits: Limits{
            VideosPerDay: 100,
            BatchSize:    20,
        },
        Features: Features{
            AITranslation:       true,
            TranslationOptimize: true,
            AITitleGeneration:   true,
            AIDescGeneration:    true,
            GeminiVideoAnalysis: true,
            AutoUpload:          true,
            APIAccess:           false,
            CustomTemplate:      true,
            DataExport:          true,
            TeamCollaboration:   false,
        },
    },
    TierEnterprise: {
        Name:     "企业版",
        Priority: 100,
        Limits: Limits{
            VideosPerDay: -1, // 无限
            BatchSize:    100,
        },
        Features: Features{
            AITranslation:       true,
            TranslationOptimize: true,
            AITitleGeneration:   true,
            AIDescGeneration:    true,
            GeminiVideoAnalysis: true,
            AutoUpload:          true,
            APIAccess:           true,
            CustomTemplate:      true,
            DataExport:          true,
            TeamCollaboration:   true,
        },
    },
}
```

### 2.2 用户会员状态

```go
// internal/membership/user_membership.go
package membership

import "time"

// UserMembership 用户会员状态
type UserMembership struct {
    UserID          string    `json:"user_id"`
    DeviceID        string    `json:"device_id"`
    Tier            Tier      `json:"tier"`
    ExpiresAt       *time.Time `json:"expires_at"`       // nil 表示永不过期
    BoostPackVideos int       `json:"boost_pack_videos"` // 加油包剩余额度
    BoostPackExpire *time.Time `json:"boost_pack_expire"`
}

// IsExpired 检查会员是否过期
func (m *UserMembership) IsExpired() bool {
    if m.ExpiresAt == nil {
        return false
    }
    return time.Now().After(*m.ExpiresAt)
}

// GetEffectiveTier 获取有效等级（考虑过期）
func (m *UserMembership) GetEffectiveTier() Tier {
    if m.IsExpired() {
        return TierFree
    }
    return m.Tier
}

// GetConfig 获取当前等级配置
func (m *UserMembership) GetConfig() TierConfig {
    return DefaultTierConfigs[m.GetEffectiveTier()]
}
```

---

## 三、功能限制检查服务

### 3.1 核心检查服务

```go
// internal/membership/feature_checker.go
package membership

import (
    "context"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

// FeatureChecker 功能检查器
type FeatureChecker struct {
    redis           *redis.Client
    membershipStore MembershipStore // 会员数据存储接口
}

// NewFeatureChecker 创建功能检查器
func NewFeatureChecker(redisClient *redis.Client, store MembershipStore) *FeatureChecker {
    return &FeatureChecker{
        redis:           redisClient,
        membershipStore: store,
    }
}

// CheckResult 检查结果
type CheckResult struct {
    Allowed bool   `json:"allowed"`
    Reason  string `json:"reason,omitempty"`
    Upgrade string `json:"upgrade,omitempty"` // 建议升级到的等级
}

// CanUseFeature 检查用户是否可以使用某功能
func (c *FeatureChecker) CanUseFeature(ctx context.Context, userID string, feature string) CheckResult {
    membership, err := c.membershipStore.GetUserMembership(ctx, userID)
    if err != nil {
        return CheckResult{Allowed: false, Reason: "获取会员信息失败"}
    }

    config := membership.GetConfig()

    switch feature {
    case "ai_translation":
        if !config.Features.AITranslation {
            return CheckResult{
                Allowed: false,
                Reason:  "AI 字幕翻译是付费功能",
                Upgrade: string(TierBasic),
            }
        }
    case "translation_optimize":
        if !config.Features.TranslationOptimize {
            return CheckResult{
                Allowed: false,
                Reason:  "翻译质量优化是专业版功能",
                Upgrade: string(TierPro),
            }
        }
    case "gemini_video_analysis":
        if !config.Features.GeminiVideoAnalysis {
            return CheckResult{
                Allowed: false,
                Reason:  "Gemini 视频分析是专业版功能",
                Upgrade: string(TierPro),
            }
        }
    case "auto_upload":
        if !config.Features.AutoUpload {
            return CheckResult{
                Allowed: false,
                Reason:  "自动定时上传是专业版功能",
                Upgrade: string(TierPro),
            }
        }
    case "api_access":
        if !config.Features.APIAccess {
            return CheckResult{
                Allowed: false,
                Reason:  "API 接口访问是企业版功能",
                Upgrade: string(TierEnterprise),
            }
        }
    case "custom_template":
        if !config.Features.CustomTemplate {
            return CheckResult{
                Allowed: false,
                Reason:  "自定义模板是付费功能",
                Upgrade: string(TierBasic),
            }
        }
    case "data_export":
        if !config.Features.DataExport {
            return CheckResult{
                Allowed: false,
                Reason:  "数据导出是专业版功能",
                Upgrade: string(TierPro),
            }
        }
    default:
        // 未知功能默认允许
        return CheckResult{Allowed: true}
    }

    return CheckResult{Allowed: true}
}

// CanProcessVideo 检查是否可以处理视频（配额检查）
func (c *FeatureChecker) CanProcessVideo(ctx context.Context, userID string) CheckResult {
    membership, err := c.membershipStore.GetUserMembership(ctx, userID)
    if err != nil {
        return CheckResult{Allowed: false, Reason: "获取会员信息失败"}
    }

    config := membership.GetConfig()

    // 无限配额
    if config.Limits.VideosPerDay == -1 {
        return CheckResult{Allowed: true}
    }

    // 获取今日使用量
    used, err := c.getDailyUsage(ctx, userID)
    if err != nil {
        return CheckResult{Allowed: false, Reason: "获取使用量失败"}
    }

    // 检查每日配额
    remaining := config.Limits.VideosPerDay - used
    if remaining > 0 {
        return CheckResult{Allowed: true}
    }

    // 检查加油包
    if membership.BoostPackVideos > 0 &&
       (membership.BoostPackExpire == nil || time.Now().Before(*membership.BoostPackExpire)) {
        return CheckResult{Allowed: true}
    }

    // 配额用完
    return CheckResult{
        Allowed: false,
        Reason:  fmt.Sprintf("今日配额已用完（%d/%d）", used, config.Limits.VideosPerDay),
        Upgrade: c.suggestUpgrade(membership.GetEffectiveTier()),
    }
}

// CanBatchSubmit 检查批量提交数量
func (c *FeatureChecker) CanBatchSubmit(ctx context.Context, userID string, count int) CheckResult {
    membership, err := c.membershipStore.GetUserMembership(ctx, userID)
    if err != nil {
        return CheckResult{Allowed: false, Reason: "获取会员信息失败"}
    }

    config := membership.GetConfig()

    if count > config.Limits.BatchSize {
        return CheckResult{
            Allowed: false,
            Reason:  fmt.Sprintf("批量提交数量超出限制（最多 %d 个）", config.Limits.BatchSize),
            Upgrade: c.suggestUpgrade(membership.GetEffectiveTier()),
        }
    }

    return CheckResult{Allowed: true}
}

// GetUserPriority 获取用户处理优先级
func (c *FeatureChecker) GetUserPriority(ctx context.Context, userID string) int {
    membership, err := c.membershipStore.GetUserMembership(ctx, userID)
    if err != nil {
        return 10 // 默认最低优先级
    }
    return membership.GetConfig().Priority
}

// getDailyUsage 获取今日使用量
func (c *FeatureChecker) getDailyUsage(ctx context.Context, userID string) (int, error) {
    today := time.Now().Format("2006-01-02")
    key := fmt.Sprintf("ytb2bili:usage:%s:%s", userID, today)

    val, err := c.redis.Get(ctx, key).Int()
    if err == redis.Nil {
        return 0, nil
    }
    return val, err
}

// suggestUpgrade 建议升级等级
func (c *FeatureChecker) suggestUpgrade(current Tier) string {
    switch current {
    case TierFree:
        return string(TierBasic)
    case TierBasic:
        return string(TierPro)
    case TierPro:
        return string(TierEnterprise)
    default:
        return ""
    }
}
```

### 3.2 配额消耗服务

```go
// internal/membership/quota_consumer.go
package membership

import (
    "context"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

// QuotaConsumer 配额消耗器
type QuotaConsumer struct {
    redis           *redis.Client
    membershipStore MembershipStore
}

// ConsumeQuota 消耗配额（处理视频后调用）
func (c *QuotaConsumer) ConsumeQuota(ctx context.Context, userID string) error {
    membership, err := c.membershipStore.GetUserMembership(ctx, userID)
    if err != nil {
        return err
    }

    config := membership.GetConfig()

    // 无限配额不需要消耗
    if config.Limits.VideosPerDay == -1 {
        return nil
    }

    // 获取今日使用量
    today := time.Now().Format("2006-01-02")
    key := fmt.Sprintf("ytb2bili:usage:%s:%s", userID, today)

    used, _ := c.redis.Get(ctx, key).Int()

    // 优先消耗每日配额
    if used < config.Limits.VideosPerDay {
        pipe := c.redis.Pipeline()
        pipe.Incr(ctx, key)
        pipe.Expire(ctx, key, 48*time.Hour) // 保留2天
        _, err = pipe.Exec(ctx)
        return err
    }

    // 消耗加油包
    if membership.BoostPackVideos > 0 {
        return c.membershipStore.DecrementBoostPack(ctx, userID)
    }

    return fmt.Errorf("no quota available")
}
```

---

## 四、功能限制集成点

### 4.1 需要修改的文件清单

| 文件                                                 | 修改内容                 | 限制功能             |
| ---------------------------------------------------- | ------------------------ | -------------------- |
| `internal/handler/video_handler.go`                  | 添加视频提交前的配额检查 | 每日视频数、批量提交 |
| `internal/chain_task/handlers/translate_subtitle.go` | 添加 AI 翻译功能检查     | AI 字幕翻译          |
| `internal/chain_task/handlers/generate_metadata.go`  | 添加 Gemini 分析检查     | Gemini 视频分析      |
| `internal/chain_task/upload_scheduler.go`            | 添加自动上传检查         | 自动定时上传         |
| `internal/chain_task/chain_task_handler.go`          | 添加优先级排序           | 处理优先级           |
| `internal/handler/middleware/`                       | 新增权限检查中间件       | 全局功能检查         |

### 4.2 视频提交限制集成

```go
// internal/handler/video_handler.go 修改示例

// submitVideo 提交视频处理
func (h *VideoHandler) submitVideo(c *gin.Context) {
    userID := c.GetString("user_id")

    var req struct {
        URLs []string `json:"urls" binding:"required"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "参数错误"})
        return
    }

    // 1. 检查批量提交限制
    batchCheck := h.FeatureChecker.CanBatchSubmit(c.Request.Context(), userID, len(req.URLs))
    if !batchCheck.Allowed {
        c.JSON(403, gin.H{
            "code":    "BATCH_LIMIT_EXCEEDED",
            "message": batchCheck.Reason,
            "upgrade": batchCheck.Upgrade,
        })
        return
    }

    // 2. 检查每日配额
    for i := 0; i < len(req.URLs); i++ {
        quotaCheck := h.FeatureChecker.CanProcessVideo(c.Request.Context(), userID)
        if !quotaCheck.Allowed {
            c.JSON(403, gin.H{
                "code":    "QUOTA_EXCEEDED",
                "message": quotaCheck.Reason,
                "upgrade": quotaCheck.Upgrade,
                "processed": i, // 已处理数量
            })
            return
        }
    }

    // 3. 继续处理...
}
```

### 4.3 AI 翻译功能限制集成

```go
// internal/chain_task/handlers/translate_subtitle.go 修改示例

func (t *TranslateSubtitle) Execute(context map[string]interface{}) bool {
    userID := context["user_id"].(string)

    // 检查 AI 翻译功能权限
    check := t.FeatureChecker.CanUseFeature(t.ctx, userID, "ai_translation")
    if !check.Allowed {
        t.App.Logger.Warnf("用户 %s 无 AI 翻译权限: %s", userID, check.Reason)

        // 免费用户：跳过翻译步骤，使用原始字幕
        context["skip_translation"] = true
        context["skip_reason"] = check.Reason
        return true // 继续后续步骤
    }

    // 检查翻译质量优化功能
    optimizeCheck := t.FeatureChecker.CanUseFeature(t.ctx, userID, "translation_optimize")
    if optimizeCheck.Allowed {
        t.EnableOptimization = true
    }

    // 继续原有翻译逻辑...
}
```

### 4.4 Gemini 多模态元数据生成限制

**核心逻辑**：根据用户等级决定使用哪种方式生成视频元数据（标题、描述、标签）

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    元数据生成策略 (按用户等级)                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  免费版用户                                                              │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │ 使用原始 YouTube 元数据（标题、描述）                              │   │
│  │ 不调用任何 AI 服务                                                │   │
│  │ 标签：从原视频提取或使用默认标签                                   │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  基础版用户                                                              │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │ 使用 DeepSeek/OpenAI 文本分析                                     │   │
│  │ 输入：字幕文本 + 原视频信息                                        │   │
│  │ 输出：AI 生成的标题、描述、标签                                    │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  专业版/企业版用户                                                       │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │ 使用 Gemini 多模态分析                                            │   │
│  │ 输入：视频文件 + 字幕文本 + 原视频信息                             │   │
│  │ 输出：基于视频内容的精准标题、描述、标签                            │   │
│  │ 优势：能理解视频画面内容，生成更精准的元数据                        │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

```go
// internal/chain_task/handlers/generate_metadata.go 修改示例

func (g *GenerateMetadata) Execute(context map[string]interface{}) bool {
    userID := context["user_id"].(string)

    // 获取用户会员配置
    membership, _ := g.FeatureChecker.GetUserMembership(g.ctx, userID)
    config := membership.GetConfig()

    // 策略1: 免费用户 - 使用原始元数据
    if !config.Features.AITitleGeneration {
        g.App.Logger.Info("免费用户：使用原始 YouTube 元数据")
        return g.useOriginalMetadata(context)
    }

    // 策略2: 专业版+ 且 Gemini 已配置 - 使用 Gemini 多模态
    if config.Features.GeminiVideoAnalysis && g.App.Config.GeminiConfig.Enabled {
        g.App.Logger.Info("专业版用户：使用 Gemini 多模态分析生成元数据")
        return g.generateWithGemini(context)
    }

    // 策略3: 基础版 或 Gemini 未配置 - 使用文本 AI 分析
    g.App.Logger.Info("基础版用户：使用文本 AI 分析生成元数据")
    return g.generateWithTextAI(context)
}

// useOriginalMetadata 使用原始 YouTube 元数据（免费用户）
func (g *GenerateMetadata) useOriginalMetadata(context map[string]interface{}) bool {
    videoID := context["video_id"].(string)

    // 从数据库获取原始元数据
    video, err := g.SavedVideoService.GetByVideoID(videoID)
    if err != nil {
        return false
    }

    // 直接使用原始标题和描述
    metadata := &VideoMetadata{
        Title:       video.OriginalTitle,
        Description: video.OriginalDescription,
        Tags:        g.extractDefaultTags(video),
    }

    return g.saveMetadata(context, metadata)
}

// generateWithTextAI 使用文本 AI 生成元数据（基础版）
func (g *GenerateMetadata) generateWithTextAI(context map[string]interface{}) bool {
    // 读取字幕文本
    subtitleText := g.readSubtitleText(context)

    // 使用 DeepSeek/OpenAI 分析
    prompt := fmt.Sprintf(`
基于以下字幕内容，生成适合 Bilibili 的视频元数据：

字幕内容：
%s

请生成：
1. 标题（20字以内，吸引人）
2. 描述（200字以内）
3. 标签（5-10个，逗号分隔）

输出 JSON 格式：{"title":"","description":"","tags":[]}
`, subtitleText)

    result, err := g.AIManager.GenerateText(g.ctx, prompt)
    if err != nil {
        g.App.Logger.Errorf("文本 AI 生成失败: %v", err)
        return g.useOriginalMetadata(context) // 降级
    }

    return g.parseAndSaveMetadata(context, result)
}

// generateWithGemini 使用 Gemini 多模态生成元数据（专业版+）
func (g *GenerateMetadata) generateWithGemini(context map[string]interface{}) bool {
    videoPath := context["video_path"].(string)
    subtitleText := g.readSubtitleText(context)

    // Gemini 多模态分析
    prompt := fmt.Sprintf(`
分析这个视频内容，结合字幕生成适合 Bilibili 的元数据：

字幕参考：
%s

请基于视频画面和字幕内容生成：
1. 标题（20字以内，准确描述视频主题）
2. 描述（200字以内，包含视频亮点）
3. 标签（5-10个，精准匹配内容）

输出 JSON 格式：{"title":"","description":"","tags":[]}
`, subtitleText)

    result, err := g.GeminiClient.AnalyzeVideoWithPrompt(g.ctx, videoPath, prompt)
    if err != nil {
        g.App.Logger.Warnf("Gemini 分析失败，降级到文本分析: %v", err)
        return g.generateWithTextAI(context) // 降级到文本分析
    }

    return g.parseAndSaveMetadata(context, result)
}
```

**降级策略**：

| 场景                | 降级方案           |
| ------------------- | ------------------ |
| Gemini API 调用失败 | 降级到文本 AI 分析 |
| 文本 AI 调用失败    | 降级到原始元数据   |
| 所有方案都失败      | 使用默认模板       |

### 4.5 自动上传限制

```go
// internal/chain_task/upload_scheduler.go 修改示例

func (s *UploadScheduler) shouldAutoUpload(video *model.SavedVideo) bool {
    userID := video.UserID

    // 检查自动上传权限
    check := s.FeatureChecker.CanUseFeature(context.Background(), userID, "auto_upload")
    if !check.Allowed {
        s.App.Logger.Debugf("用户 %s 无自动上传权限，跳过", userID)
        return false
    }

    return true
}

// getNextVideoToUpload 获取下一个待上传视频（按优先级排序）
func (s *UploadScheduler) getNextVideoToUpload() (*model.SavedVideo, error) {
    videos, err := s.SavedVideoService.GetPendingUploadVideos()
    if err != nil {
        return nil, err
    }

    // 按用户优先级排序
    sort.Slice(videos, func(i, j int) bool {
        priorityI := s.FeatureChecker.GetUserPriority(context.Background(), videos[i].UserID)
        priorityJ := s.FeatureChecker.GetUserPriority(context.Background(), videos[j].UserID)
        return priorityI > priorityJ
    })

    // 过滤无自动上传权限的视频
    for _, video := range videos {
        if s.shouldAutoUpload(video) {
            return video, nil
        }
    }

    return nil, nil
}
```

---

## 五、权限检查中间件

### 5.1 Gin 中间件实现

```go
// internal/handler/middleware/feature_middleware.go
package middleware

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/difyz9/ytb2bili/internal/membership"
)

// FeatureMiddleware 功能权限中间件
type FeatureMiddleware struct {
    checker *membership.FeatureChecker
}

func NewFeatureMiddleware(checker *membership.FeatureChecker) *FeatureMiddleware {
    return &FeatureMiddleware{checker: checker}
}

// RequireFeature 要求特定功能权限
func (m *FeatureMiddleware) RequireFeature(feature string) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        if userID == "" {
            c.JSON(http.StatusUnauthorized, gin.H{
                "code":    "UNAUTHORIZED",
                "message": "未登录",
            })
            c.Abort()
            return
        }

        check := m.checker.CanUseFeature(c.Request.Context(), userID, feature)
        if !check.Allowed {
            c.JSON(http.StatusForbidden, gin.H{
                "code":    "FEATURE_NOT_ALLOWED",
                "message": check.Reason,
                "upgrade": check.Upgrade,
                "feature": feature,
            })
            c.Abort()
            return
        }

        c.Next()
    }
}

// RequireQuota 要求配额
func (m *FeatureMiddleware) RequireQuota() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        if userID == "" {
            c.JSON(http.StatusUnauthorized, gin.H{
                "code":    "UNAUTHORIZED",
                "message": "未登录",
            })
            c.Abort()
            return
        }

        check := m.checker.CanProcessVideo(c.Request.Context(), userID)
        if !check.Allowed {
            c.JSON(http.StatusForbidden, gin.H{
                "code":    "QUOTA_EXCEEDED",
                "message": check.Reason,
                "upgrade": check.Upgrade,
            })
            c.Abort()
            return
        }

        c.Next()
    }
}

// InjectUserTier 注入用户等级信息到上下文
func (m *FeatureMiddleware) InjectUserTier() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        if userID != "" {
            membership, _ := m.checker.GetUserMembership(c.Request.Context(), userID)
            if membership != nil {
                c.Set("user_tier", membership.GetEffectiveTier())
                c.Set("user_config", membership.GetConfig())
            }
        }
        c.Next()
    }
}
```

### 5.2 路由注册示例

```go
// internal/handler/routes.go

func RegisterRoutes(r *gin.Engine, handlers *Handlers, featureMiddleware *middleware.FeatureMiddleware) {
    api := r.Group("/api/v1")

    // 公开接口
    api.GET("/plans", handlers.Membership.GetPlans)

    // 需要登录的接口
    auth := api.Group("")
    auth.Use(middleware.AuthRequired())
    auth.Use(featureMiddleware.InjectUserTier())
    {
        // 视频相关 - 需要配额
        videos := auth.Group("/videos")
        videos.Use(featureMiddleware.RequireQuota())
        {
            videos.POST("", handlers.Video.SubmitVideo)
        }

        // API 接口 - 需要企业版
        apiRoutes := auth.Group("/api")
        apiRoutes.Use(featureMiddleware.RequireFeature("api_access"))
        {
            apiRoutes.POST("/batch", handlers.API.BatchProcess)
        }

        // 数据导出 - 需要专业版
        export := auth.Group("/export")
        export.Use(featureMiddleware.RequireFeature("data_export"))
        {
            export.GET("/videos", handlers.Export.ExportVideos)
        }
    }
}
```

---

## 六、前端集成

### 6.1 用户状态 Hook

```typescript
// src/hooks/useMembership.ts
import { useState, useEffect, createContext, useContext } from "react";

interface MembershipState {
  tier: "free" | "basic" | "pro" | "enterprise";
  tierName: string;
  features: {
    aiTranslation: boolean;
    translationOptimize: boolean;
    geminiVideoAnalysis: boolean;
    autoUpload: boolean;
    apiAccess: boolean;
    customTemplate: boolean;
    dataExport: boolean;
  };
  limits: {
    videosPerDay: number;
    batchSize: number;
  };
  usage: {
    dailyUsed: number;
    dailyRemaining: number;
    boostPackBalance: number;
  };
  expiresAt: string | null;
}

const MembershipContext = createContext<MembershipState | null>(null);

export function useMembership() {
  return useContext(MembershipContext);
}

export function useCanUseFeature(feature: keyof MembershipState["features"]) {
  const membership = useMembership();
  if (!membership) return false;
  return membership.features[feature];
}

export function useQuotaRemaining() {
  const membership = useMembership();
  if (!membership) return 0;
  return membership.usage.dailyRemaining + membership.usage.boostPackBalance;
}
```

### 6.2 功能锁定组件

```tsx
// src/components/FeatureGate.tsx
"use client";

import { useMembership, useCanUseFeature } from "@/hooks/useMembership";
import { Lock, Sparkles } from "lucide-react";
import Link from "next/link";

interface FeatureGateProps {
  feature: string;
  children: React.ReactNode;
  fallback?: React.ReactNode;
  showUpgrade?: boolean;
}

export function FeatureGate({
  feature,
  children,
  fallback,
  showUpgrade = true,
}: FeatureGateProps) {
  const canUse = useCanUseFeature(feature as any);
  const membership = useMembership();

  if (canUse) {
    return <>{children}</>;
  }

  if (fallback) {
    return <>{fallback}</>;
  }

  // 默认锁定 UI
  return (
    <div className="relative">
      {/* 模糊的原内容 */}
      <div className="opacity-50 blur-sm pointer-events-none">{children}</div>

      {/* 锁定遮罩 */}
      <div className="absolute inset-0 flex items-center justify-center bg-gray-900/10 rounded-lg">
        <div className="text-center p-4">
          <Lock className="w-8 h-8 mx-auto mb-2 text-gray-400" />
          <p className="text-sm text-gray-600 mb-2">
            {getFeatureDescription(feature)}
          </p>
          {showUpgrade && (
            <Link
              href="/pricing"
              className="inline-flex items-center gap-1 text-sm text-purple-600 hover:text-purple-700"
            >
              <Sparkles className="w-4 h-4" />
              升级解锁
            </Link>
          )}
        </div>
      </div>
    </div>
  );
}

function getFeatureDescription(feature: string): string {
  const descriptions: Record<string, string> = {
    ai_translation: "升级到基础版解锁 AI 字幕翻译",
    translation_optimize: "升级到专业版解锁翻译质量优化",
    gemini_video_analysis: "升级到专业版解锁 Gemini 视频分析",
    auto_upload: "升级到专业版解锁自动定时上传",
    api_access: "升级到企业版解锁 API 接口访问",
    custom_template: "升级到基础版解锁自定义模板",
    data_export: "升级到专业版解锁数据导出",
  };
  return descriptions[feature] || "此功能需要升级会员";
}
```

### 6.3 配额显示组件

```tsx
// src/components/QuotaIndicator.tsx
"use client";

import { useMembership, useQuotaRemaining } from "@/hooks/useMembership";
import { AlertTriangle } from "lucide-react";

export function QuotaIndicator() {
  const membership = useMembership();
  const remaining = useQuotaRemaining();

  if (!membership) return null;

  const { dailyUsed, dailyRemaining } = membership.usage;
  const { videosPerDay } = membership.limits;
  const percent = videosPerDay > 0 ? (dailyUsed / videosPerDay) * 100 : 0;

  return (
    <div className="flex items-center gap-3">
      {/* 进度条 */}
      <div className="flex-1 h-2 bg-gray-100 rounded-full overflow-hidden">
        <div
          className={`h-full transition-all ${
            percent >= 90
              ? "bg-red-500"
              : percent >= 70
              ? "bg-yellow-500"
              : "bg-green-500"
          }`}
          style={{ width: `${Math.min(percent, 100)}%` }}
        />
      </div>

      {/* 数字 */}
      <span className="text-sm text-gray-600 whitespace-nowrap">
        {dailyUsed}/{videosPerDay === -1 ? "∞" : videosPerDay}
      </span>

      {/* 警告 */}
      {remaining <= 2 && remaining > 0 && (
        <AlertTriangle className="w-4 h-4 text-yellow-500" />
      )}
    </div>
  );
}
```

---

## 七、实施步骤

### Phase 1: 基础设施 (2-3 天)

1. [ ] 创建 `internal/membership/` 目录结构
2. [ ] 实现 `TierConfig` 和 `Features` 数据结构
3. [ ] 实现 `FeatureChecker` 核心服务
4. [ ] 配置 Redis 连接（可使用 Upstash）

### Phase 2: 后端集成 (3-4 天)

1. [ ] 实现 `FeatureMiddleware` 中间件
2. [ ] 修改 `video_handler.go` 添加配额检查
3. [ ] 修改 `translate_subtitle.go` 添加 AI 翻译检查
4. [ ] 修改 `generate_metadata.go` 添加 Gemini 检查
5. [ ] 修改 `upload_scheduler.go` 添加自动上传检查

### Phase 3: 前端集成 (2-3 天)

1. [ ] 实现 `useMembership` Hook
2. [ ] 实现 `FeatureGate` 组件
3. [ ] 实现 `QuotaIndicator` 组件
4. [ ] 在各功能页面添加权限检查

### Phase 4: 测试验证 (2 天)

1. [ ] 单元测试：各等级功能权限
2. [ ] 集成测试：完整处理流程
3. [ ] 边界测试：配额耗尽、过期等场景

---

## 八、注意事项

### 8.1 降级策略

当用户无权限使用某功能时，应提供合理的降级方案：

| 功能        | 降级方案               |
| ----------- | ---------------------- |
| AI 翻译     | 跳过翻译，保留原始字幕 |
| Gemini 分析 | 使用文本分析替代       |
| 自动上传    | 提示用户手动上传       |
| 批量提交    | 限制为单个提交         |

### 8.2 缓存策略

- 用户会员状态缓存 5 分钟
- 每日使用量实时更新
- 功能配置可长期缓存（配置变更时清除）

### 8.3 错误处理

所有权限检查失败应返回统一格式：

```json
{
  "code": "FEATURE_NOT_ALLOWED",
  "message": "AI 字幕翻译是付费功能",
  "upgrade": "basic",
  "feature": "ai_translation"
}
```

前端根据 `upgrade` 字段显示对应的升级引导。

### 8.4 并发安全

- 配额消耗使用 Redis INCR 原子操作
- 加油包扣减使用乐观锁或 Redis 事务
- 避免超卖问题

---

## 九、加油包方案设计

### 9.1 加油包定义

加油包是一次性购买的额外配额，用于补充每日限额不足的情况。

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           加油包方案                                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                │
│   │  小加油包    │    │  中加油包    │    │  大加油包    │                │
│   │             │    │             │    │             │                │
│   │   ¥6.9     │    │   ¥19.9    │    │   ¥39.9    │                │
│   │  +10 视频   │    │  +30 视频   │    │  +80 视频   │                │
│   │  7天有效    │    │  15天有效   │    │  30天有效   │                │
│   └─────────────┘    └─────────────┘    └─────────────┘                │
│                                                                         │
│   特点：                                                                 │
│   • 可叠加购买（额度累加，有效期取最长）                                   │
│   • 优先消耗每日配额，不足时消耗加油包                                     │
│   • 过期后剩余额度清零                                                   │
│   • 所有会员等级均可购买                                                  │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 9.2 加油包配置

```go
// internal/membership/boost_pack.go
package membership

import "time"

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
    Videos      int           `json:"videos"`       // 视频额度
    ValidDays   int           `json:"valid_days"`   // 有效天数
    Description string        `json:"description"`
}

// DefaultBoostPackConfigs 默认加油包配置
var DefaultBoostPackConfigs = map[BoostPackType]BoostPackConfig{
    BoostPackSmall: {
        Type:        BoostPackSmall,
        Name:        "小加油包",
        Price:       6.9,
        Videos:      10,
        ValidDays:   7,
        Description: "适合偶尔需要额外处理几个视频",
    },
    BoostPackMedium: {
        Type:        BoostPackMedium,
        Name:        "中加油包",
        Price:       19.9,
        Videos:      30,
        ValidDays:   15,
        Description: "适合短期项目或活动期间使用",
    },
    BoostPackLarge: {
        Type:        BoostPackLarge,
        Name:        "大加油包",
        Price:       39.9,
        Videos:      80,
        ValidDays:   30,
        Description: "超值大容量，适合重度用户",
    },
}

// UserBoostPack 用户加油包状态
type UserBoostPack struct {
    UserID          string    `json:"user_id"`
    VideosRemaining int       `json:"videos_remaining"` // 剩余视频额度
    ExpiresAt       time.Time `json:"expires_at"`       // 过期时间
    LastPurchaseAt  time.Time `json:"last_purchase_at"` // 最后购买时间
}

// IsValid 检查加油包是否有效
func (b *UserBoostPack) IsValid() bool {
    return b.VideosRemaining > 0 && time.Now().Before(b.ExpiresAt)
}
```

### 9.3 加油包购买逻辑

```go
// internal/membership/boost_pack_service.go
package membership

import (
    "context"
    "fmt"
    "time"
)

// BoostPackService 加油包服务
type BoostPackService struct {
    store MembershipStore
    redis *redis.Client
}

// PurchaseBoostPack 购买加油包
func (s *BoostPackService) PurchaseBoostPack(ctx context.Context, userID string, packType BoostPackType) error {
    config, ok := DefaultBoostPackConfigs[packType]
    if !ok {
        return fmt.Errorf("invalid boost pack type: %s", packType)
    }

    // 获取当前加油包状态
    current, err := s.store.GetUserBoostPack(ctx, userID)
    if err != nil {
        // 不存在则创建新的
        current = &UserBoostPack{
            UserID:          userID,
            VideosRemaining: 0,
            ExpiresAt:       time.Now(),
        }
    }

    // 计算新的额度和过期时间
    newVideos := current.VideosRemaining + config.Videos
    newExpiry := s.calculateNewExpiry(current.ExpiresAt, config.ValidDays)

    // 更新加油包
    updated := &UserBoostPack{
        UserID:          userID,
        VideosRemaining: newVideos,
        ExpiresAt:       newExpiry,
        LastPurchaseAt:  time.Now(),
    }

    if err := s.store.SaveUserBoostPack(ctx, updated); err != nil {
        return err
    }

    // 清除缓存
    s.redis.Del(ctx, fmt.Sprintf("ytb2bili:boost:%s", userID))
    s.redis.Del(ctx, fmt.Sprintf("ytb2bili:quota:%s", userID))

    return nil
}

// calculateNewExpiry 计算新的过期时间（叠加逻辑）
func (s *BoostPackService) calculateNewExpiry(currentExpiry time.Time, addDays int) time.Time {
    now := time.Now()

    // 如果当前已过期，从现在开始计算
    if currentExpiry.Before(now) {
        return now.AddDate(0, 0, addDays)
    }

    // 否则在当前过期时间基础上延长
    return currentExpiry.AddDate(0, 0, addDays)
}

// ConsumeBoostPack 消耗加油包额度
func (s *BoostPackService) ConsumeBoostPack(ctx context.Context, userID string) error {
    pack, err := s.store.GetUserBoostPack(ctx, userID)
    if err != nil {
        return fmt.Errorf("no boost pack found")
    }

    if !pack.IsValid() {
        return fmt.Errorf("boost pack expired or depleted")
    }

    pack.VideosRemaining--

    if err := s.store.SaveUserBoostPack(ctx, pack); err != nil {
        return err
    }

    // 清除缓存
    s.redis.Del(ctx, fmt.Sprintf("ytb2bili:boost:%s", userID))

    return nil
}

// GetBoostPackStatus 获取加油包状态
func (s *BoostPackService) GetBoostPackStatus(ctx context.Context, userID string) (*UserBoostPack, error) {
    pack, err := s.store.GetUserBoostPack(ctx, userID)
    if err != nil {
        return &UserBoostPack{
            UserID:          userID,
            VideosRemaining: 0,
            ExpiresAt:       time.Time{},
        }, nil
    }

    // 检查是否过期
    if !pack.IsValid() {
        pack.VideosRemaining = 0
    }

    return pack, nil
}
```

### 9.4 配额消耗优先级

```go
// ConsumeQuota 消耗配额（优先每日配额，不足时消耗加油包）
func (c *QuotaConsumer) ConsumeQuota(ctx context.Context, userID string) error {
    membership, err := c.membershipStore.GetUserMembership(ctx, userID)
    if err != nil {
        return err
    }

    config := membership.GetConfig()

    // 企业版无限配额
    if config.Limits.VideosPerDay == -1 {
        return nil
    }

    // 获取今日使用量
    today := time.Now().Format("2006-01-02")
    key := fmt.Sprintf("ytb2bili:usage:%s:%s", userID, today)
    used, _ := c.redis.Get(ctx, key).Int()

    // 优先级1: 消耗每日配额
    if used < config.Limits.VideosPerDay {
        pipe := c.redis.Pipeline()
        pipe.Incr(ctx, key)
        pipe.Expire(ctx, key, 48*time.Hour)
        _, err = pipe.Exec(ctx)
        return err
    }

    // 优先级2: 消耗加油包
    boostPack, err := c.boostPackService.GetBoostPackStatus(ctx, userID)
    if err == nil && boostPack.IsValid() {
        return c.boostPackService.ConsumeBoostPack(ctx, userID)
    }

    return fmt.Errorf("no quota available")
}
```

### 9.5 加油包 API

```go
// internal/handler/boost_pack_handler.go
package handler

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "github.com/difyz9/ytb2bili/internal/membership"
)

type BoostPackHandler struct {
    service *membership.BoostPackService
}

// GetBoostPacks 获取加油包列表
func (h *BoostPackHandler) GetBoostPacks(c *gin.Context) {
    packs := make([]membership.BoostPackConfig, 0)
    for _, config := range membership.DefaultBoostPackConfigs {
        packs = append(packs, config)
    }

    c.JSON(http.StatusOK, gin.H{
        "code":    200,
        "message": "success",
        "data":    packs,
    })
}

// GetMyBoostPack 获取我的加油包状态
func (h *BoostPackHandler) GetMyBoostPack(c *gin.Context) {
    userID := c.GetString("user_id")

    pack, err := h.service.GetBoostPackStatus(c.Request.Context(), userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "code":    500,
            "message": "获取加油包状态失败",
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "code":    200,
        "message": "success",
        "data": gin.H{
            "videos_remaining": pack.VideosRemaining,
            "expires_at":       pack.ExpiresAt,
            "is_valid":         pack.IsValid(),
        },
    })
}

// PurchaseBoostPack 购买加油包（支付成功后回调）
func (h *BoostPackHandler) PurchaseBoostPack(c *gin.Context) {
    userID := c.GetString("user_id")

    var req struct {
        PackType string `json:"pack_type" binding:"required"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "code":    400,
            "message": "参数错误",
        })
        return
    }

    packType := membership.BoostPackType(req.PackType)
    if err := h.service.PurchaseBoostPack(c.Request.Context(), userID, packType); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "code":    500,
            "message": err.Error(),
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "code":    200,
        "message": "购买成功",
    })
}
```

### 9.6 前端加油包组件

```tsx
// src/components/BoostPackCard.tsx
"use client";

import { Zap } from "lucide-react";

interface BoostPackProps {
  type: "small" | "medium" | "large";
  name: string;
  price: number;
  videos: number;
  validDays: number;
  description: string;
  onPurchase: () => void;
}

export function BoostPackCard({
  type,
  name,
  price,
  videos,
  validDays,
  description,
  onPurchase,
}: BoostPackProps) {
  const colors = {
    small: "from-green-400 to-green-600",
    medium: "from-blue-400 to-blue-600",
    large: "from-purple-400 to-purple-600",
  };

  return (
    <div className="relative bg-white rounded-2xl shadow-lg overflow-hidden hover:shadow-xl transition-shadow">
      {/* 顶部渐变条 */}
      <div className={`h-2 bg-gradient-to-r ${colors[type]}`} />

      <div className="p-6">
        {/* 图标和名称 */}
        <div className="flex items-center gap-3 mb-4">
          <div className={`p-2 rounded-lg bg-gradient-to-r ${colors[type]}`}>
            <Zap className="w-5 h-5 text-white" />
          </div>
          <h3 className="text-lg font-semibold">{name}</h3>
        </div>

        {/* 价格 */}
        <div className="mb-4">
          <span className="text-3xl font-bold">¥{price}</span>
        </div>

        {/* 额度信息 */}
        <div className="space-y-2 mb-6">
          <div className="flex items-center justify-between text-sm">
            <span className="text-gray-500">视频额度</span>
            <span className="font-medium">+{videos} 个</span>
          </div>
          <div className="flex items-center justify-between text-sm">
            <span className="text-gray-500">有效期</span>
            <span className="font-medium">{validDays} 天</span>
          </div>
          <div className="flex items-center justify-between text-sm">
            <span className="text-gray-500">单价</span>
            <span className="font-medium text-green-600">
              ¥{(price / videos).toFixed(2)}/个
            </span>
          </div>
        </div>

        {/* 描述 */}
        <p className="text-sm text-gray-500 mb-6">{description}</p>

        {/* 购买按钮 */}
        <button
          onClick={onPurchase}
          className={`w-full py-3 rounded-lg text-white font-medium bg-gradient-to-r ${colors[type]} hover:opacity-90 transition-opacity`}
        >
          立即购买
        </button>
      </div>
    </div>
  );
}
```

### 9.7 加油包状态显示

```tsx
// src/components/BoostPackStatus.tsx
"use client";

import { Zap, Clock } from "lucide-react";
import { formatDistanceToNow } from "date-fns";
import { zhCN } from "date-fns/locale";

interface BoostPackStatusProps {
  videosRemaining: number;
  expiresAt: string | null;
}

export function BoostPackStatus({
  videosRemaining,
  expiresAt,
}: BoostPackStatusProps) {
  if (videosRemaining <= 0 || !expiresAt) {
    return (
      <div className="flex items-center gap-2 text-gray-400">
        <Zap className="w-4 h-4" />
        <span className="text-sm">无加油包</span>
      </div>
    );
  }

  const expireDate = new Date(expiresAt);
  const isExpiringSoon =
    expireDate.getTime() - Date.now() < 3 * 24 * 60 * 60 * 1000; // 3天内

  return (
    <div className="flex items-center gap-4 p-3 bg-orange-50 rounded-lg">
      <div className="flex items-center gap-2">
        <Zap className="w-5 h-5 text-orange-500" />
        <span className="font-medium">{videosRemaining}</span>
        <span className="text-sm text-gray-500">个视频</span>
      </div>

      <div
        className={`flex items-center gap-1 text-sm ${
          isExpiringSoon ? "text-red-500" : "text-gray-500"
        }`}
      >
        <Clock className="w-4 h-4" />
        <span>
          {formatDistanceToNow(expireDate, { addSuffix: true, locale: zhCN })}
          到期
        </span>
      </div>
    </div>
  );
}
```

---

## 十、总结

### 10.1 核心架构

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         会员功能限制系统架构                              │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│   ┌─────────────┐     ┌─────────────┐     ┌─────────────┐              │
│   │  用户请求    │────▶│  中间件检查  │────▶│  功能检查器  │              │
│   └─────────────┘     └─────────────┘     └──────┬──────┘              │
│                                                   │                     │
│                       ┌───────────────────────────┼───────────────┐    │
│                       │                           │               │    │
│                       ▼                           ▼               ▼    │
│               ┌─────────────┐           ┌─────────────┐   ┌──────────┐│
│               │  会员等级    │           │  每日配额    │   │ 加油包   ││
│               │  TierConfig │           │  Redis计数  │   │ 额度     ││
│               └─────────────┘           └─────────────┘   └──────────┘│
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 10.2 实施优先级

| 优先级 | 功能              | 工作量 |
| :----: | ----------------- | :----: |
|   P0   | 每日配额限制      |  2 天  |
|   P0   | AI 翻译功能开关   |  1 天  |
|   P1   | Gemini 多模态开关 |  1 天  |
|   P1   | 加油包购买        |  2 天  |
|   P2   | 自动上传开关      |  1 天  |
|   P2   | 优先级队列        |  1 天  |
|   P3   | API 访问控制      |  1 天  |

### 10.3 关键成功指标

- 功能限制准确率 100%
- 配额计算无超卖
- 降级策略覆盖所有场景
- 用户体验流畅（无明显延迟）

### 10.4 风险与应对

| 风险       | 应对措施                  |
| ---------- | ------------------------- |
| Redis 故障 | 本地缓存兜底 + 宽松策略   |
| 配额超卖   | 原子操作 + 乐观锁         |
| 功能误判   | 完善单元测试 + 灰度发布   |
| 用户投诉   | 清晰的升级引导 + 客服支持 |
