# 任务重试与错误处理改进方案

## 问题背景

当前系统存在以下问题：
1. **无限重试**：retry_count = 118，说明任务被重试了 118 次
2. **数据残留**：视频删除后，task_steps 记录没有被清理
3. **无效 URL**：不支持平台的 URL 会不断重试，浪费资源

---

## 方案 1：添加最大重试次数限制

### 1.1 扩展状态常量

```go
// pkg/store/model/task_step.go

const (
    StatusPending      = "pending"
    StatusRunning      = "running"
    StatusCompleted    = "completed"
    StatusFailed       = "failed"         // 可重试
    StatusFailedPerm   = "failed_permanent" // 不可重试
    StatusSkipped      = "skipped"
)

// 永久失败的原因
const (
    ReasonPlatformNotSupported = "platform_not_supported" // 平台不支持
    ReasonInvalidURL           = "invalid_url"            // URL 无效
    ReasonVideoNotFound        = "video_not_found"        // 视频已删除
    ReasonMaxRetriesExceeded   = "max_retries_exceeded"   // 超过最大重试次数
    ReasonDownloadFailed       = "download_failed"        // 下载失败（网络/文件）
)
```

### 1.2 配置最大重试次数

```go
// internal/core/types/task_config.go

package types

// TaskRetryConfig 任务重试配置
type TaskRetryConfig struct {
    MaxRetries      int            `json:"max_retries"`       // 最大重试次数
    RetryableErrors []string       `json:"retryable_errors"`  // 可重试的错误关键字
    PermanentErrors []string       `json:"permanent_errors"` // 永久性错误关键字
}

var DefaultRetryConfigs = map[string]TaskRetryConfig{
    "download_video": {
        MaxRetries: 3,
        RetryableErrors: []string{
            "network",
            "timeout",
            "connection reset",
            "temporary failure",
        },
        PermanentErrors: []string{
            "Unsupported URL",
            "video not available",
            "copyright",
            "region locked",
        },
    },
    "generate_subtitles": {
        MaxRetries: 3,
        RetryableErrors: []string{
            "whisper error",
            "audio processing",
        },
        PermanentErrors: []string{
            "no audio track",
            "audio extraction failed",
        },
    },
    "translate_subtitle": {
        MaxRetries: 5,
        RetryableErrors: []string{
            "API timeout",
            "rate limit",
            "service unavailable",
        },
        PermanentErrors: []string{
            "invalid API key",
            "unsupported language",
        },
    },
    "generate_metadata": {
        MaxRetries: 3,
        RetryableErrors: []string{
            "AI service timeout",
        },
        PermanentErrors: []string{
            "content policy violation",
        },
    },
    "upload_video": {
        MaxRetries: 10,
        RetryableErrors: []string{
            "Bilibili API error",
            "upload timeout",
            "network error",
        },
        PermanentErrors: []string{
            "invalid credentials",
            "file format not supported",
        },
    },
    "upload_subtitle": {
        MaxRetries: 10,
        RetryableErrors: []string{
            "Bilibili API error",
            "network error",
        },
        PermanentErrors: []string{
            "video not uploaded",
        },
    },
}

func GetRetryConfig(stepName string) TaskRetryConfig {
    if config, ok := DefaultRetryConfigs[stepName]; ok {
        return config
    }
    // 默认配置
    return TaskRetryConfig{
        MaxRetries:      3,
        RetryableErrors: []string{},
        PermanentErrors: []string{},
    }
}
```

### 1.3 修改任务执行逻辑

```go
// internal/chain_task/chain_task_handler.go

func (h *ChainTaskHandler) executeTask(task Task) error {
    // 1. 获取任务步骤
    taskStep, err := h.TaskStepService.GetOrCreateTaskStep(task.GetVideoID(), task.GetName())
    if err != nil {
        return err
    }

    // 2. 检查是否已达到最大重试次数
    if taskStep.RetryCount > 0 {
        config := types.GetRetryConfig(task.GetName())
        if taskStep.RetryCount >= config.MaxRetries {
            // 标记为永久失败
            h.TaskStepService.UpdateStatus(
                task.GetVideoID(),
                task.GetName(),
                model.StatusFailedPerm,
                fmt.Sprintf("达到最大重试次数 (%d): %s", config.MaxRetries, types.ReasonMaxRetriesExceeded),
            )
            return fmt.Errorf("任务已达到最大重试次数")
        }
    }

    // 3. 执行任务
    err = task.Execute(task.GetContext())

    // 4. 处理执行结果
    if err != nil {
        return h.handleTaskError(task, taskStep, err)
    }

    // 5. 成功：更新状态为 completed
    return h.TaskStepService.UpdateStatus(
        task.GetVideoID(),
        task.GetName(),
        model.StatusCompleted,
        "",
    )
}

func (h *ChainTaskHandler) handleTaskError(task Task, taskStep *model.TaskStep, err error) error {
    config := types.GetRetryConfig(task.GetName())
    errorMsg := err.Error()

    // 检查是否为永久性错误
    for _, permanentErr := range config.PermanentErrors {
        if strings.Contains(strings.ToLower(errorMsg), strings.ToLower(permanentErr)) {
            // 永久性错误：标记为 failed_permanent
            h.TaskStepService.UpdateStatus(
                task.GetVideoID(),
                task.GetName(),
                model.StatusFailedPerm,
                fmt.Sprintf("永久性失败: %s", errorMsg),
            )
            return fmt.Errorf("任务永久性失败: %w", err)
        }
    }

    // 可重试错误：增加重试次数，重置为 pending
    taskStep.RetryCount++
    h.TaskStepService.IncrementRetryCount(task.GetVideoID(), task.GetName())
    h.TaskStepService.UpdateStatus(
        task.GetVideoID(),
        task.GetName(),
        model.StatusPending,
        fmt.Sprintf("重试 %d/%d: %s", taskStep.RetryCount, config.MaxRetries, errorMsg),
    )

    return fmt.Errorf("任务失败，将重试: %w", err)
}
```

---

## 方案 2：级联删除任务步骤

### 2.1 方案 A：使用数据库外键（推荐）

```go
// pkg/store/model/task_step.go

type TaskStep struct {
    BaseModel
    VideoID    string `gorm:"type:varchar(100);not null;index" json:"video_id"`

    // 添加外键约束
    SavedVideoID *uint      `gorm:"index" json:"saved_video_id"`           // 关联 SavedVideo.ID
    SavedVideo   *SavedVideo `gorm:"foreignKey:SavedVideoID;constraint:OnDelete:CASCADE" json:"-"`

    StepName   string     `gorm:"type:varchar(100);not null" json:"step_name"`
    // ... 其他字段
}
```

**优点**：
- ✅ 数据库自动处理级联删除
- ✅ 不需要修改删除逻辑
- ✅ 数据一致性由数据库保证

**缺点**：
- ⚠️ 需要数据迁移
- ⚠️ 需要修改现有的 task_steps 表结构

### 2.2 方案 B：应用层手动删除（简单快速）

```go
// internal/core/services/saved_video_service.go

// DeleteVideoForUser 删除视频（带用户隔离和级联删除）
func (s *SavedVideoService) DeleteVideoForUser(id, userID uint) error {
    // 开启事务
    tx := s.DB.Begin()
    defer func() {
        if r := recover(); r != nil {
            tx.Rollback()
        }
    }()

    // 1. 获取视频信息（用于删除关联的 task_steps）
    var video model.SavedVideo
    err := tx.Where("id = ? AND user_id = ?", id, userID).First(&video).Error
    if err != nil {
        tx.Rollback()
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return errors.New("视频不存在或无权删除")
        }
        return err
    }

    // 2. 删除关联的任务步骤
    if err := tx.Where("video_id = ?", video.VideoID).Delete(&model.TaskStep{}).Error; err != nil {
        tx.Rollback()
        return fmt.Errorf("删除任务步骤失败: %w", err)
    }

    // 3. 删除视频记录
    if err := tx.Delete(&video).Error; err != nil {
        tx.Rollback()
        return fmt.Errorf("删除视频失败: %w", err)
    }

    // 提交事务
    return tx.Commit().Error
}
```

**推荐**：短期使用方案 B，长期迁移到方案 A

---

## 方案 3：优化 URL 验证（改进版）

### ❌ 不推荐：URL 白名单

```javascript
// ❌ 这样做不好
const SUPPORTED_PLATFORMS = [
    'youtube.com',
    'tiktok.com',
    'twitter.com',
    // ... 1000+ 个平台
];

function validateURL(url) {
    const hostname = new URL(url).hostname;
    return SUPPORTED_PLATFORMS.some(platform =>
        hostname.includes(platform)
    );
}
```

**问题**：
- 需要维护 1000+ 平台列表
- URL 格式变化频繁
- 新平台无法立即支持

### ✅ 推荐：快速失败 + 错误提示

#### 3.1 前端基本格式验证

```typescript
// web/src/lib/urlValidator.ts

export function validateURLFormat(url: string): { valid: boolean; error?: string } {
    try {
        // 基本格式检查
        const urlObj = new URL(url);

        // 必须是 HTTP/HTTPS
        if (!['http:', 'https:'].includes(urlObj.protocol)) {
            return { valid: false, error: '仅支持 HTTP/HTTPS 协议' };
        }

        // 基本格式检查
        if (!urlObj.hostname) {
            return { valid: false, error: 'URL 格式无效' };
        }

        // 过滤明显的无效 URL
        if (urlObj.hostname === 'localhost' || urlObj.hostname === '127.0.0.1') {
            return { valid: false, error: '不支持本地地址' };
        }

        return { valid: true };
    } catch (e) {
        return { valid: false, error: 'URL 格式无效' };
    }
}
```

#### 3.2 后端检测平台支持

```go
// internal/chain_task/handlers/down_load_video.go

func (h *DownloadVideoHandler) checkPlatformSupport(url string) error {
    // 1. 基本格式验证
    parsedURL, err := url.Parse(url)
    if err != nil {
        return fmt.Errorf("%s: %w", types.ReasonInvalidURL, err)
    }

    // 2. 调用 yt-dlp 的 --list-extractors 检查支持
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx,
        h.YtDlpPath,
        "--list-extractors",
    )
    output, err := cmd.Output()
    if err != nil {
        // 无法检查，假设支持
        return nil
    }

    // 3. 检查域名是否在支持列表中
    extractors := strings.Split(string(output), "\n")
    hostname := parsedURL.Hostname()

    for _, extractor := range extractors {
        if strings.TrimSpace(extractor) == "" {
            continue
        }
        // 简单匹配：检查主机名是否包含 extractor 名称
        if strings.Contains(hostname, strings.ToLower(extractor)) {
            return nil // 支持该平台
        }
    }

    // 4. 未知平台，尝试实际下载
    // 让 yt-dlp 自己判断是否支持
    return nil
}

func (h *DownloadVideoHandler) Execute(ctx map[string]interface{}) bool {
    videoID := ctx["video_id"].(string)
    videoURL := ctx["url"].(string)

    // 1. 快速平台检测（可选，5秒内返回）
    if err := h.checkPlatformSupport(videoURL); err != nil {
        // 标记为永久失败
        ctx["error"] = fmt.Sprintf("平台不支持: %v", err)
        return false
    }

    // 2. 实际下载（30秒超时）
    downloadCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    cmd := exec.CommandContext(downloadCtx,
        h.YtDlpPath,
        "--print", "filename",
        videoURL,
    )

    output, err := cmd.CombinedOutput()
    if err != nil {
        errMsg := string(output)

        // 3. 检查是否为不支持的 URL
        if strings.Contains(strings.ToLower(errMsg), "unsupported") ||
           strings.Contains(strings.ToLower(errMsg), "no video formats") {
            // 永久性失败
            ctx["error"] = fmt.Sprintf("%s: %s", types.ReasonPlatformNotSupported, errMsg)
            return false
        }

        // 可重试错误
        ctx["error"] = fmt.Sprintf("下载失败: %s", errMsg)
        return false
    }

    // 下载成功...
    return true
}
```

#### 3.3 用户友好的错误提示

```typescript
// web/src/components/video/VideoActions.tsx

async function handleSubmit() {
    // 1. 前端验证
    const validation = validateURLFormat(url);
    if (!validation.valid) {
        alert(`❌ URL 无效: ${validation.error}\n\n支持的示例：\n• YouTube: https://youtube.com/watch?v=xxx\n• TikTok: https://tiktok.com/@user/video/xxx`);
        return;
    }

    // 2. 提交到后端
    try {
        const response = await authFetch('/api/v1/videos', {
            method: 'POST',
            body: JSON.stringify({ url }),
        });

        const data = await response.json();

        if (data.code === 400 && data.message?.includes('平台不支持')) {
            alert(`❌ ${data.message}\n\n💡 提示：\n• yt-dlp 支持大多数主流视频平台\n• 确保视频 URL 可以直接访问\n• 避免使用需要登录的视频链接`);
            return;
        }

        if (data.code === 200) {
            alert('✅ 视频添加成功！');
        }
    } catch (error) {
        alert('❌ 网络错误，请稍后重试');
    }
}
```

---

## 实施步骤

### 阶段 1：紧急修复（1-2天）

1. ✅ 添加 `failed_permanent` 状态
2. ✅ 修改 `DeleteVideoForUser` 支持级联删除
3. ✅ 添加基本重试次数限制（统一设置为 5 次）

### 阶段 2：完善重试策略（3-5天）

1. ✅ 实现任务级别的重试配置
2. ✅ 区分永久性错误和可重试错误
3. ✅ 添加重试原因记录

### 阶段 3：优化用户体验（可选）

1. ✅ 前端 URL 格式验证
2. ✅ 友好的错误提示
3. ✅ 支持的平台列表展示

---

## 测试用例

```go
// internal/core/services/task_step_service_test.go

func TestMaxRetryLimit(t *testing.T) {
    // 1. 创建任务步骤，设置 retry_count = 5
    // 2. 尝试重试
    // 3. 验证状态变为 failed_permanent
}

func TestCascadeDelete(t *testing.T) {
    // 1. 创建视频和关联的任务步骤
    // 2. 删除视频
    // 3. 验证 task_steps 也被删除
}

func TestPlatformDetection(t *testing.T) {
    testCases := []struct {
        url      string
        expected bool
    }{
        {"https://youtube.com/watch?v=xxx", true},
        {"https://tiktok.com/@user/video/xxx", true},
        {"https://invalid-platform.com/video", false},
    }

    for _, tc := range testCases {
        result := checkPlatformSupport(tc.url)
        // 验证结果
    }
}
```

---

## 监控和日志

```go
// 添加重试统计
type RetryStats struct {
    TotalRetries      int            `json:"total_retries"`
    PermanentFailures int            `json:"permanent_failures"`
    ByReason          map[string]int `json:"by_reason"`
}

// 定期输出统计信息
func (s *TaskStepService) LogRetryStats() {
    stats := s.GetRetryStats()
    s.App.Logger.Infof("重试统计: 总重试 %d 次, 永久失败 %d 次",
        stats.TotalRetries,
        stats.PermanentFailures,
    )
}
```
