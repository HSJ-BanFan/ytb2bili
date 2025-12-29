# 多用户日志系统改进方案

## 🎯 目标

**问题**：当前日志无法区分不同用户的任务，难以调试

**解决方案**：为所有日志添加用户上下文信息（用户ID、用户名、会员等级）

---

## ✅ 实现方案

### 核心组件：ContextLogger

**位置**：`internal/logger/context_logger.go`

**功能**：
- 从 gin.Context 自动提取用户信息
- 在日志前缀中显示用户信息
- 使用 emoji 区分会员等级
- 支持结构化字段

---

## 📊 日志格式对比

### 当前日志（❌ 难以区分）

```
2025/12/28 19:26:50 debug 发现 3 个待重试的步骤
2025/12/28 19:26:50 debug 未找到待上传视频 (模式: immediate)
2025/12/28 19:26:50 [getAccounts] 获取用户账号列表, userID=6
2025/12/28 19:26:55 [getAccounts] 获取用户账号列表, userID=6
```

**问题**：
- ❌ 不知道是哪个用户在执行任务
- ❌ 多个用户ID混合在一起
- ❌ 难以追踪单个用户的完整流程

### 改进后日志（✅ 清晰可读）

```
2025/12/28 19:26:50 [test_free@6 🆓] 发现 3 个待重试的步骤
2025/12/28 19:26:50 [mei@12 💎] 未找到待上传视频 (模式: immediate)
2025/12/28 19:26:50 [test_free@6 🆓] 📤 开始上传: video_id=abc123, action=upload_video
2025/12/28 19:26:55 [mei@12 💎] 📤 开始上传: video_id=def456, action=upload_video
```

**优点**：
- ✅ 清晰显示用户名和ID
- ✅ emoji 直观显示会员等级
- ✅ 每行日志都有上下文
- ✅ 易于过滤和搜索

---

## 🚀 使用方法

### 方法 1：使用 ContextLogger（推荐）

```go
import "github.com/difyz9/ytb2bili/internal/logger"

func (h *UploadScheduler) uploadNextVideo() error {
    // ... 业务逻辑 ...

    // 使用 ContextLogger
    logger.GetContextLogger(c).Infof(
        c,
        "开始上传视频: video_id=%s, 状态=%s",
        video.VideoID,
        video.Status,
    )
}
```

**输出**：
```
[test_free@6 🆓] 开始上传视频: video_id=abc123, 状态=200
```

### 方法 2：使用带字段的日志

```go
logger.GetContextLogger(c).WithFields(c, "视频上传成功", map[string]interface{}{
    "video_id": video.VideoID,
    "bili_bvid": video.BiliBVID,
    "duration": time.Since(startTime).Seconds(),
})
```

**输出**：
```
[test_free@6 🆓] 视频上传成功 {"video_id": "abc123", "bili_bvid": "BV1xx...", "duration": 123.45}
```

### 方法 3：使用任务专用日志

```go
logger.GetContextLogger(c).TaskLog(c,
    video.VideoID,  // 视频ID
    "upload_video", // 动作
    "success",      // 状态
    map[string]interface{}{
        "bili_bvid": result.BVID,
        "duration": time.Since(start).Seconds(),
    },
)
```

**输出**：
```
[test_free@6 🆓] 📤 upload_video success video_id=abc123 bili_bvid=BV1xx...
```

---

## 🎨 日志示例

### 场景 1：视频下载

```go
logger.GetContextLogger(c).TaskLog(c, videoID, "download", "started", map[string]interface{}{
    "url": url,
    "platform": "YouTube",
})
```

**输出**：
```
[test_free@6 🆓] 📥 download started video_id=abc123 url=https://youtube.com/... platform=YouTube
```

### 场景 2：字幕翻译

```go
logger.GetContextLogger(c).TaskLog(c, videoID, "translate", "completed", map[string]interface{}{
    "source_lang": "en",
    "target_lang": "zh",
    "segments": 150,
})
```

**输出**：
```
[mei@12 💎] 🌐 translate completed video_id=def456 source_lang=en target_lang=zh segments=150
```

### 场景 3：错误处理

```go
logger.GetContextLogger(c).Errorf(c, "上传失败: video_id=%s, err=%v", videoID, err)
```

**输出**：
```
[test_free@6 🆓] 上传失败: video_id=abc123, err=rate limit exceeded
```

---

## 🔧 集成到现有代码

### 修改 main.go 添加日志中间件

```go
import "github.com/difyz9/ytb2bili/internal/logger"

func main() {
    // ... 现有代码 ...

    // 设置用户日志中间件
    logger.SetupUserLogging(server.Engine, app.Logger)

    // ... 现有代码 ...
}
```

### 示例：upload_scheduler.go

**修改前**：
```go
func (s *UploadScheduler) uploadNextVideo() error {
    s.logger.Infof("开始上传视频: %s", video.Title)
    // ...
}
```

**修改后**：
```go
func (s *UploadScheduler) uploadNextVideo(c context.Context) error {
    // 添加 context.Context 参数
    logger.GetContextLogger(c).Infof(
        c, // ← gin.Context
        "开始上传视频: %s",
        video.Title,
    )
    // ...
}
```

**问题**：当前 `uploadNextVideo` 没有 `gin.Context` 参数。

### 解决方案：传递 gin.Context

```go
// 在 cron 调用中创建临时 context
func (s *UploadScheduler) SetUp() {
    s.Task.AddFunc(cronExpr, func() {
        s.uploadNextVideo() // ← 需要传递 context
    })
}
```

**方案 1：传递 userID**

```go
func (s *UploadScheduler) uploadNextVideoWithUser(userID uint) error {
    // 使用 userID 创建日志
    prefix := fmt.Sprintf("[user_%d]", userID)
    s.logger.Infof("%s 开始上传视频: %s", prefix, video.Title)
}
```

**方案 2：使用结构化日志字段**

```go
func (s *UploadScheduler) uploadNextVideo() error {
    s.logger.Infow("开始上传视频",
        zap.Uint("user_id", video.UserID),
        zap.String("video_id", video.VideoID),
        zap.String("title", video.Title),
    )
}
```

---

## 💡 推荐方案：结构化日志

对于**后台任务**（没有 gin.Context），使用结构化日志字段：

### 修改 SavedVideoService

```go
func (s *SavedVideoService) CreateVideoWithLog(video *model.SavedVideo) error {
    s.Logger.Infow("创建视频记录",
        zap.Uint("user_id", video.UserID),
        zap.String("video_id", video.VideoID),
        zap.String("url", video.URL),
        zap.String("tier", getTierFromDB(video.UserID)),
    )
    return s.CreateVideo(video)
}
```

**输出**：
```json
{
  "level": "info",
  "ts": "2025-12-28T19:26:50.000+08:00",
  "caller": "saved_video_service.go:65",
  "msg": "创建视频记录",
  "user_id": 6,
  "video_id": "abc123",
  "url": "https://youtube.com/...",
  "tier": "free"
}
```

---

## 🎨 日志过滤和搜索

### 搜索特定用户的日志

```bash
# 搜索用户 test_free 的所有日志
grep "test_free@6" logs.txt

# 搜索特定用户的所有任务
grep "mei@12" logs.txt | grep "video_id=abc123"

# 搜索所有 Pro 用户的操作
grep "💎" logs.txt
```

### 使用 jq 过滤 JSON 日志

```bash
# 提取特定用户的所有日志
cat logs.txt | jq 'select(.user_id == 6)'

# 统计每个用户的任务数量
cat logs.txt | jq -r '.user_id' | sort | uniq -c
```

---

## 📋 实施步骤

### 阶段 1：结构化日志字段（立即实施）

1. **修改关键业务逻辑**，添加字段：
   - `user_id`
   - `video_id`
   - `action`
   - `status`

2. **示例**：
   ```go
   s.logger.Infow("处理视频",
       zap.Uint("user_id", userID),
       zap.String("video_id", videoID),
       zap.String("action", "download"),
   )
   ```

### 阶段 2：日志格式化（短期）

1. **在日志前缀中添加用户信息**：
   ```go
   prefix := fmt.Sprintf("[user_%d]", userID)
   s.logger.Infof("%s 开始处理", prefix)
   ```

2. **或者使用 helper 函数**：
   ```go
   func logWithUser(logger *zap.Logger, userID uint, msg string) {
       username := getUsernameFromDB(userID)
       tier := getTierFromDB(userID)
       logger.Infof(fmt.Sprintf("[%s@%d %s] %s", username, userID, tier, msg))
   }
   ```

### 阶段 3：集成 ContextLogger（长期）

1. **修改 handler 签名**，传递 `gin.Context`
2. **使用 ContextLogger** 记录日志
3. **在 main.go 中注册日志中间件**

---

## 📊 日志优先级和类型

### 任务开始/结束

```go
logger.GetContextLogger(c).TaskLog(c, videoID, "download", "started", fields)
logger.GetContextLogger(c).TaskLog(c, videoID, "download", "completed", fields)
```

### 重要状态变化

```go
logger.GetContextLogger(c).Infof(c, "状态变更: %s -> %s", oldStatus, newStatus)
```

### 错误和警告

```go
logger.GetContextLogger(c).Errorf(c, "任务失败: %v", err)
logger.GetContextLogger(c).Warnf(c, "重试第 %d 次", retryCount)
```

---

## 🎯 效果对比

### 修改前

```
2025/12/28 19:26:50 发现 3 个待重试的步骤
2025/12/28 19:26:50 未找到待上传视频 (模式: immediate)
2025/12/28 19:26:50 [getAccounts] 获取用户账号列表, userID=6
```

### 修改后

```
[test_free@6 🆓] 🔁 发现 3 个待重试的步骤
[test_free@6 🆓] 🔍 未找到待上传视频 (模式: immediate)
[mei@12 💎] 📤 开始上传: video_id=xyz789
```

---

## 🚀 快速开始

### 方案 1：最小改动（推荐）

**在关键位置添加字段**：

```go
// chain_task_handler.go
func (h *ChainTaskHandler) RunTaskChain(t models2.TbVideo) {
    h.App.Logger.Infow("开始执行任务链",
        zap.Uint("user_id", t.UserID),
        zap.String("video_id", t.VideoId),
        zap.String("tier", getUserTier(t.UserID)),
    )
}
```

### 方案 2：使用 ContextLogger

```go
// 需要 gin.Context 参数
func (h *VideoHandler) manualUploadVideo(c *gin.Context) {
    logger.GetContextLogger(c).Infof(c, "手动上传视频: %s", videoID)
}
```

---

## ✅ 总结

| 特性 | 当前日志 | 改进后 |
|------|---------|--------|
| **用户标识** | ❌ 无或分散 | ✅ 统一前缀 [用户@ID 等级] |
| **可读性** | ❌ 难以区分 | ✅ emoji + 颜色 |
| **可搜索性** | ❌ 混乱 | ✅ 可过滤 |
| **上下文** | ❌ 缺失 | ✅ 完整 |
| **结构化** | ❌ 文本 | ✅ JSON 字段 |

**建议**：先实施结构化日志字段（阶段1），再逐步迁移到 ContextLogger（阶段3）
