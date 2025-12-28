# 会员功能限制与账号隔离完整实施方案

## 📋 问题回顾

| # | 问题 | 状态 | 严重性 |
|---|------|------|--------|
| 1 | 账号隔离 bug - 视频被上传到所有用户的 B 站账号 | ✅ 已修复 | 🔴 严重 |
| 2 | 功能限制缺失 - 免费用户可使用自动上传和并发处理 | ⏳ 待实施 | 🟡 中等 |

---

## ⚠️ 你的方案缺失的关键点

### 缺失点 1：权限控制的多个层次

**你的方案只提到了 `upload_scheduler.go`**，但权限控制需要在多个层次实现：

| 层次 | 位置 | 作用 | 你的方案 |
|------|------|------|----------|
| **API 层** | `video_handler.go` | 拦截未授权的 API 请求 | ❌ 缺失 |
| **调度器层** | `upload_scheduler.go` | 自动上传权限检查 | ✅ 有 |
| **任务执行层** | `chain_task_handler.go` | 并发控制 | ✅ 有 |
| **前端层** | `VideoActions.tsx` | UI 隐藏/禁用 | ❌ 缺失 |

### 缺失点 2：自动上传的多个入口

**自动上传不只是 `upload_scheduler.go`**：

```go
// 入口 1: 自动调度器（你的方案已覆盖）
upload_scheduler.go: uploadNextVideo()

// 入口 2: 手动上传 API（❌ 你没提到）
video_handler.go: manualUploadVideo()

// 入口 3: 字幕上传调度器（❌ 你没提到）
upload_scheduler.go: uploadNextSubtitle()
```

### 缺失点 3：并发控制的具体实现

**你提到"实现 per-user 并发限制"，但没有说明**：
- 如何追踪每个用户的活跃任务数？
- 任务完成/失败时如何释放槽位？
- 跨服务器的并发控制如何实现？

### 缺失点 4：配额系统的集成

**自动上传是否消耗配额？**：
- 如果不消耗，免费用户可能绕过配额限制
- 如果消耗，需要明确时机（任务开始时 vs 上传成功时）

---

## ✅ 完整实施方案

### 第 1 步：扩展会员配置

```go
// internal/membership/tier.go

// Limits 配额限制
type Limits struct {
    VideosPerDay      int `json:"videos_per_day"`       // -1 表示无限
    BatchSize         int `json:"batch_size"`           // 批量提交数
    MaxConcurrentTasks int `json:"max_concurrent_tasks"` // 最大并发任务数 (-1=无限)
}

// DefaultTierConfigs 默认等级配置
var DefaultTierConfigs = map[Tier]TierConfig{
    TierFree: {
        Limits: Limits{
            VideosPerDay:       5,
            BatchSize:          1,
            MaxConcurrentTasks: 1, // ✅ 新增：串行处理
        },
        Features: Features{
            AutoUpload: false, // ✅ 确认：免费用户不能自动上传
        },
    },
    TierBasic: {
        Limits: Limits{
            VideosPerDay:       20,
            BatchSize:          5,
            MaxConcurrentTasks: 2, // ✅ 新增：最多 2 个并发
        },
        Features: Features{
            AutoUpload: true, // ✅ Basic 用户可以自动上传
        },
    },
    TierPro: {
        Limits: Limits{
            VideosPerDay:       100,
            BatchSize:          20,
            MaxConcurrentTasks: 5, // ✅ 新增：最多 5 个并发
        },
        Features: Features{
            AutoUpload: true,
        },
    },
    TierEnterprise: {
        Limits: Limits{
            VideosPerDay:       -1, // 无限
            BatchSize:          100,
            MaxConcurrentTasks: -1, // ✅ 新增：无限制
        },
        Features: Features{
            AutoUpload: true,
        },
    },
}
```

### 第 2 步：创建权限检查服务

```go
// internal/membership/permission_service.go

package membership

import (
    "context"
    "fmt"
)

// PermissionService 权限检查服务
type PermissionService struct {
    store   MembershipStore
    checker *FeatureChecker
}

func NewPermissionService(store MembershipStore, checker *FeatureChecker) *PermissionService {
    return &PermissionService{
        store:   store,
        checker: checker,
    }
}

// CanAutoUpload 检查用户是否可以自动上传
func (s *PermissionService) CanAutoUpload(ctx context.Context, userID string) (bool, error) {
    membership, err := s.store.GetUserMembership(ctx, userID)
    if err != nil {
        return false, fmt.Errorf("获取会员信息失败: %w", err)
    }

    config := membership.GetConfig()
    return config.Features.AutoUpload, nil
}

// GetMaxConcurrentTasks 获取用户最大并发任务数
func (s *PermissionService) GetMaxConcurrentTasks(ctx context.Context, userID string) (int, error) {
    membership, err := s.store.GetUserMembership(ctx, userID)
    if err != nil {
        return 1, fmt.Errorf("获取会员信息失败: %w", err)
    }

    config := membership.GetConfig()
    return config.Limits.MaxConcurrentTasks, nil
}

// CheckUploadPermission 检查上传权限（综合检查）
func (s *PermissionService) CheckUploadPermission(ctx context.Context, userID string, uploadType string) error {
    // 1. 检查功能权限
    canUpload, err := s.CanAutoUpload(ctx, userID)
    if err != nil {
        return err
    }

    if !canUpload {
        return fmt.Errorf("自动上传是 Pro 会员功能，请升级会员")
    }

    // 2. 检查配额（可选，如果上传需要消耗配额）
    // quotaService := ...
    // if err := quotaService.ConsumeQuota(ctx, userID); err != nil {
    //     return err
    // }

    return nil
}
```

### 第 3 步：修改上传调度器（添加权限检查）

```go
// internal/chain_task/upload_scheduler.go

type UploadScheduler struct {
    App                *core.AppServer
    SavedVideoService  *services.SavedVideoService
    TaskStepService    *services.TaskStepService
    BiliAccountService *services.BiliAccountService
    Db                 *gorm.DB
    Task               *cron.Cron
    mutex              sync.Mutex
    logger             *zap.SugaredLogger

    // ✅ 新增：权限服务
    PermissionService *membership.PermissionService
}

// ✅ 修改：uploadNextVideo 添加权限检查
func (s *UploadScheduler) uploadNextVideo() error {
    // ... 现有代码 ...

    for _, video := range videos {
        // ✅ 新增：获取用户 ID
        userIDStr := fmt.Sprintf("%d", video.ID) // 假设 ID 是用户 ID
        // 实际应该从 video 获取 user_id
        // userID := s.getUserIDFromVideo(video.VideoID)

        // ✅ 新增：检查自动上传权限
        canUpload, err := s.PermissionService.CanAutoUpload(context.Background(), userIDStr)
        if err != nil {
            s.logger.Errorf("检查上传权限失败: %v", err)
            continue
        }

        if !canUpload {
            s.logger.Infof("用户 %s 无自动上传权限，跳过视频 %s", userIDStr, video.VideoID)
            continue
        }

        // 执行上传...
        s.executeUploadTask(video.VideoID, "上传到Bilibili")
    }

    return nil
}

// ✅ 修改：ExecuteManualUpload 添加权限检查
func (s *UploadScheduler) ExecuteManualUpload(videoID, taskType string) error {
    // 1. 获取视频信息
    video, err := s.SavedVideoService.GetVideoByVideoID(videoID)
    if err != nil {
        return fmt.Errorf("视频不存在: %w", err)
    }

    // 2. ✅ 新增：检查上传权限
    userIDStr := fmt.Sprintf("%d", video.UserID)
    canUpload, err := s.PermissionService.CanAutoUpload(context.Background(), userIDStr)
    if err != nil {
        return err
    }

    if !canUpload {
        return fmt.Errorf("自动上传是 Pro 会员功能，请升级会员")
    }

    // 3. 执行上传
    var taskName string
    switch taskType {
    case "video":
        taskName = "上传到Bilibili"
    case "subtitle":
        taskName = "上传字幕到Bilibili"
    default:
        return fmt.Errorf("未知的任务类型: %s", taskType)
    }

    return s.executeUploadTask(videoID, taskName)
}
```

### 第 4 步：实现并发控制

```go
// internal/chain_task/concurrency_limiter.go

package chain_task

import (
    "context"
    "sync"
    "time"

    "github.com/difyz9/ytb2bili/internal/membership"
)

// ConcurrencyLimiter 并发控制器（per-user）
type ConcurrencyLimiter struct {
    // userConcurrency[user] = current count
    userConcurrency map[uint]int
    userMutex       map[uint]*sync.Mutex
    globalMutex     sync.RWMutex

    permissionService *membership.PermissionService
    logger           *zap.SugaredLogger
}

func NewConcurrencyLimiter(
    permService *membership.PermissionService,
    logger *zap.SugaredLogger,
) *ConcurrencyLimiter {
    return &ConcurrencyLimiter{
        userConcurrency:    make(map[uint]int),
        userMutex:          make(map[uint]*sync.Mutex),
        permissionService:  permService,
        logger:             logger,
    }
}

// TryAcquire 尝试获取执行槽位
func (c *ConcurrencyLimiter) TryAcquire(ctx context.Context, userID uint) (bool, error) {
    // 1. 获取用户最大并发数
    userIDStr := fmt.Sprintf("%d", userID)
    maxConcurrent, err := c.permissionService.GetMaxConcurrentTasks(ctx, userIDStr)
    if err != nil {
        return false, err
    }

    // -1 表示无限
    if maxConcurrent == -1 {
        return true, nil
    }

    // 2. 获取用户锁
    c.globalMutex.Lock()
    userMutex, exists := c.userMutex[userID]
    if !exists {
        userMutex = &sync.Mutex{}
        c.userMutex[userID] = userMutex
    }
    c.globalMutex.Unlock()

    userMutex.Lock()
    defer userMutex.Unlock()

    // 3. 检查当前并发数
    c.globalMutex.Lock()
    defer c.globalMutex.Unlock()

    current := c.userConcurrency[userID]
    if current >= maxConcurrent {
        c.logger.Infof("用户 %d 并发任务已达上限 (%d/%d)",
            userID, current, maxConcurrent)
        return false, nil
    }

    // 4. 增加并发数
    c.userConcurrency[userID]++
    c.logger.Infof("用户 %d 获取执行槽位 (%d/%d)",
        userID, c.userConcurrency[userID], maxConcurrent)

    return true, nil
}

// Release 释放执行槽位
func (c *ConcurrencyLimiter) Release(userID uint) {
    c.globalMutex.Lock()
    defer c.globalMutex.Unlock()

    if current, exists := c.userConcurrency[userID]; exists {
        if current > 0 {
            c.userConcurrency[userID]--
            c.logger.Infof("用户 %d 释放执行槽位 (%d remaining)",
                userID, c.userConcurrency[userID])
        }
    }
}

// GetStats 获取并发统计
func (c *ConcurrencyLimiter) GetStats(userID uint) (current, max int) {
    c.globalMutex.RLock()
    defer c.globalMutex.RUnlock()

    current = c.userConcurrency[userID]

    ctx := context.Background()
    userIDStr := fmt.Sprintf("%d", userID)
    max, _ = c.permissionService.GetMaxConcurrentTasks(ctx, userIDStr)

    return
}
```

### 第 5 步：集成到任务链处理器

```go
// internal/chain_task/chain_task_handler.go

type ChainTaskHandler struct {
    App                *core.AppServer
    // ... 现有字段 ...

    // ✅ 新增：并发控制器
    ConcurrencyLimiter *ConcurrencyLimiter
}

func (h *ChainTaskHandler) SetUp() {
    // ... 现有代码 ...

    // 启动任务消费者（准备阶段）
    for i := 0; i < h.MaxWorkers; i++ {
        go h.worker()
    }
}

// ✅ 修改：worker 函数添加并发控制
func (h *ChainTaskHandler) worker() {
    for videoID := range h.taskQueue {
        // 1. 获取视频信息
        video, err := h.SavedVideoService.GetVideoByVideoID(videoID)
        if err != nil {
            h.App.Logger.Errorf("获取视频失败: %v", err)
            continue
        }

        // 2. ✅ 新增：尝试获取执行槽位
        acquired, err := h.ConcurrencyLimiter.TryAcquire(context.Background(), video.UserID)
        if err != nil {
            h.App.Logger.Errorf("检查并发权限失败: %v", err)
            // 重试：放回队列
            go func() {
                time.Sleep(10 * time.Second)
                h.taskQueue <- videoID
            }()
            continue
        }

        if !acquired {
            h.App.Logger.Infof("用户 %d 并发任务已达上限，等待中...", video.UserID)
            // 重试：延迟后放回队列
            go func() {
                time.Sleep(30 * time.Second)
                h.taskQueue <- videoID
            }()
            continue
        }

        // 3. 执行任务
        go func() {
            defer func() {
                // ✅ 释放执行槽位
                h.ConcurrencyLimiter.Release(video.UserID)
            }()

            if err := h.ProcessVideo(videoID); err != nil {
                h.App.Logger.Errorf("处理视频失败: %v", err)
            }
        }()
    }
}
```

### 第 6 步：API 层权限检查

```go
// internal/handler/video_handler.go

type VideoHandler struct {
    BaseHandler
    SavedVideoService *services.SavedVideoService
    TaskStepService   *services.TaskStepService
    UploadScheduler   interface {
        ExecuteManualUpload(videoID, taskType string) error
    }

    // ✅ 新增：权限服务
    PermissionService *membership.PermissionService
}

// ✅ 修改：manualUploadVideo 添加权限检查
func (h *VideoHandler) manualUploadVideo(c *gin.Context) {
    videoID := c.Param("id")

    // 1. 获取用户 ID
    userID, exists := auth.GetUserID(c)
    if !exists || userID == 0 {
        c.JSON(401, gin.H{"message": "未登录"})
        return
    }

    // 2. ✅ 新增：检查上传权限
    userIDStr := fmt.Sprintf("%d", userID)
    canUpload, err := h.PermissionService.CanAutoUpload(c.Request.Context(), userIDStr)
    if err != nil {
        c.JSON(500, gin.H{"message": "检查权限失败"})
        return
    }

    if !canUpload {
        c.JSON(403, gin.H{
            "message": "自动上传是 Pro 会员功能，请升级会员",
            "code":    "FEATURE_NOT_ALLOWED",
            "upgrade": "pro",
        })
        return
    }

    // 3. 执行上传
    if err := h.UploadScheduler.ExecuteManualUpload(videoID, "video"); err != nil {
        c.JSON(500, gin.H{"message": err.Error()})
        return
    }

    c.JSON(200, gin.H{"message": "上传任务已提交"})
}
```

### 第 7 步：前端 UI 控制

```typescript
// web/src/components/video/VideoActions.tsx

import { useAuth } from '@/hooks/useAuth';

interface VideoActionsProps {
  video: Video;
}

export default function VideoActions({ video }: VideoActionsProps) {
  const { user, canUseFeature } = useAuth();

  const handleUploadVideo = async () => {
    // ✅ 检查自动上传权限
    if (!canUseFeature('auto_upload')) {
      alert('❌ 自动上传是 Pro 会员功能\n\n请升级到 Pro 版本以使用此功能');
      return;
    }

    // ... 执行上传
  };

  return (
    <div>
      {/* ✅ 根据权限显示/隐藏按钮 */}
      {canUseFeature('auto_upload') ? (
        <button onClick={handleUploadVideo}>
          上传到B站
        </button>
      ) : (
        <button
          onClick={() => alert('请升级到 Pro 版本')}
          disabled
          className="opacity-50 cursor-not-allowed"
        >
          上传到B站 (Pro专属)
        </button>
      )}
    </div>
  );
}
```

### 第 8 步：main.go 中注册服务

```go
// main.go

func registerHandlers(
    server *core.AppServer,
    logger *zap.SugaredLogger,
    // ... 现有参数
    membershipStore membership.MembershipStore,
) {
    // ✅ 新增：创建权限服务
    checker := membership.NewFeatureChecker(membershipStore)
    permissionService := membership.NewPermissionService(membershipStore, checker)

    // ✅ 新增：创建并发控制器
    concurrencyLimiter := chain_task.NewConcurrencyLimiter(permissionService, logger)

    // ... 现有代码 ...

    // 上传调度器（注入权限服务）
    uploadScheduler := chain_task.NewUploadScheduler(
        server,
        task,
        db,
        savedVideoService,
        taskStepService,
        biliAccountService,
    )
    uploadScheduler.PermissionService = permissionService // ✅ 注入

    // 任务链处理器（注入并发控制器）
    chainTaskHandler := chain_task.NewChainTaskHandler(
        app,
        db,
        savedVideoService,
        taskStepService,
        uploadScheduler,
    )
    chainTaskHandler.ConcurrencyLimiter = concurrencyLimiter // ✅ 注入

    // 视频 Handler（注入权限服务）
    videoHandler := handler.NewVideoHandler(server, savedVideoService, taskStepService)
    videoHandler.SetUploadScheduler(uploadScheduler)
    videoHandler.PermissionService = permissionService // ✅ 注入
    videoHandler.RegisterRoutes(videoGroup)
}
```

---

## 🧪 验证方案

### 测试 1：免费用户自动上传限制

```bash
# 1. 使用 test_free 登录
# 2. 提交视频 URL
# 3. 等待处理完成

# 预期结果：
# - 视频停留在状态 200 (准备就绪)
# - 不会自动上传到B站
# - 日志显示：用户 test_free 无自动上传权限
```

### 测试 2：Pro 用户自动上传

```bash
# 1. 将 test_free 升级为 Pro
UPDATE cw_users SET membership_tier = 'pro' WHERE id = <test_free_id>;

# 2. 重启服务器
# 3. 提交新视频

# 预期结果：
# - 视频自动上传到B站
# - 日志显示：检查权限通过
```

### 测试 3：并发限制

```bash
# 1. Free 用户同时提交 3 个视频

# 预期结果（Free 用户，MaxConcurrentTasks=1）：
# - 视频 1：立即开始处理
# - 视频 2：等待视频 1 完成
# - 视频 3：等待视频 2 完成

# Pro 用户（MaxConcurrentTasks=5）：
# - 所有视频同时处理
```

### 测试 4：账号隔离

```bash
# 1. test_free 绑定账号 A
# 2. mei 绑定账号 B
# 3. test_free 提交视频

# 预期结果：
# - 视频只上传到账号 A
# - 不上传到账号 B
```

---

## 📊 权限控制流程图

```
用户触发上传
    │
    ▼
┌─────────────────┐
│  前端 UI 检查    │
│  隐藏/禁用按钮   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  API 层检查      │
│  video_handler  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  权限服务检查    │
│  CanAutoUpload  │
└────────┬────────┘
         │
    ┌────┴────┐
    │         │
   ✅         ❌
    │         │
    ▼         ▼
┌─────────┐  返回 403
│ 调度器  │  "请升级"
└────┬────┘
     │
     ▼
┌─────────────────┐
│  并发控制       │
│  TryAcquire     │
└────────┬────────┘
         │
    ┌────┴────┐
    │         │
   ✅         ❌
    │         │
    ▼         ▼
┌─────────┐  延迟重试
│执行上传  │
└─────────┘
```

---

## 🎯 实施优先级

| 优先级 | 任务 | 文件 | 预计时间 |
|--------|------|------|----------|
| P0 | 扩展 tier.go 配置 | `internal/membership/tier.go` | 30 分钟 |
| P0 | 创建权限服务 | `internal/membership/permission_service.go` | 1 小时 |
| P0 | API 层权限检查 | `internal/handler/video_handler.go` | 30 分钟 |
| P1 | 调度器权限检查 | `internal/chain_task/upload_scheduler.go` | 30 分钟 |
| P1 | 实现并发控制 | `internal/chain_task/concurrency_limiter.go` | 2 小时 |
| P1 | 集成到任务链 | `internal/chain_task/chain_task_handler.go` | 1 小时 |
| P2 | 前端 UI 控制 | `web/src/components/video/VideoActions.tsx` | 1 小时 |
| P2 | 注册服务 | `main.go` | 30 分钟 |

**总计**：约 7-8 小时

---

## ⚠️ 注意事项

1. **数据库迁移**：
   - `user_id` 字段已存在于 `cw_saved_videos` 表，无需迁移
   - 如果没有，需要添加：`ALTER TABLE cw_saved_videos ADD COLUMN user_id INT;`

2. **配额消耗**：
   - 建议在**任务成功后**消耗配额
   - 避免在任务开始时消耗（防止任务失败但配额已扣）

3. **日志记录**：
   - 记录所有权限检查结果
   - 便于调试和审计

4. **错误提示**：
   - 友好的错误消息
   - 明确告知用户如何升级

5. **测试覆盖**：
   - 每个会员等级都要测试
   - 边界条件（并发数 = 0, -1）
