# 多用户日志系统实现示例

## 📊 日志输出对比

### 修改前（难以区分用户）

```
2025/12/28 19:26:50 📤 开始上传视频: Awesome Video (VideoID: abc123)
2025/12/28 19:26:52 📝 开始上传字幕: Another Video (VideoID: def456)
2025/12/28 19:26:54 ✅ 视频上传成功: abc123
2025/12/28 19:26:55 🚀 开始执行任务链: xyz789
2025/12/28 19:26:56 ✅ 字幕上传成功: def456
```

**问题**：
- ❌ 无法区分哪些操作属于哪个用户
- ❌ 难以追踪单个用户的完整流程
- ❌ 调试时需要手动查找用户ID

### 修改后（清晰显示用户上下文）

```
2025/12/28 19:26:50 [user_6] 📤 upload_video started video_id=abc123 title="Awesome Video"
2025/12/28 19:26:52 [user_12] 📝 upload_subtitle started video_id=def456 title="Another Video"
2025/12/28 19:26:54 [user_6] ✅ upload_video success video_id=abc123 subtitle_scheduled_at="20:35:00"
2025/12/28 19:26:55 [user_12] 🚀 schedule started video_id=xyz789 title="My Video" mode="concurrent"
2025/12/28 19:26:56 [user_12] ✅ upload_subtitle success video_id=def456
```

**优点**：
- ✅ 每行日志都显示用户ID前缀 `[user_N]`
- ✅ 使用 emoji 直观区分操作类型
- ✅ 结构化字段便于过滤和搜索
- ✅ 易于追踪单个用户的完整流程

---

## 🔧 实现方式

### 1. 后台任务使用 UserLogHelper

**适用场景**：cron 定时任务、后台调度器（没有 `gin.Context`）

**示例**：`internal/chain_task/upload_scheduler.go`

```go
import (
    applogger "github.com/difyz9/ytb2bili/internal/logger"
)

func (s *UploadScheduler) uploadNextVideo() error {
    // ... 查询视频 ...

    video := videos[0]

    // 创建用户日志助手
    userLogger := applogger.NewUserLogger(s.logger.Desugar(), video.UserID)

    // 记录任务开始
    userLogger.TaskLog(video.VideoID, "upload_video", "started",
        map[string]interface{}{
            "title":        video.Title,
            "completed_at": video.ProcessingCompletedAt.Format("15:04:05"),
        })

    // 执行上传逻辑
    if err := s.executeUploadTask(video.VideoID, "上传到Bilibili"); err != nil {
        // 记录失败
        userLogger.TaskLog(video.VideoID, "upload_video", "failed",
            map[string]interface{}{
                "error": err.Error(),
            })
        return err
    }

    // 记录成功
    userLogger.TaskLog(video.VideoID, "upload_video", "success",
        map[string]interface{}{
            "subtitle_scheduled_at": subtitleScheduledAt.Format("15:04:05"),
        })

    return nil
}
```

**输出**：
```
[user_6] 📤 upload_video started video_id=abc123 title="Awesome Video" completed_at="19:26:50"
[user_6] ✅ upload_video success video_id=abc123 subtitle_scheduled_at="20:35:00"
```

---

### 2. 任务链执行使用 UserLogHelper

**适用场景**：任务链处理器、并发任务执行

**示例**：`internal/chain_task/chain_task_handler.go`

```go
import (
    applogger "github.com/difyz9/ytb2bili/internal/logger"
)

func (h *ChainTaskHandler) RunTaskChain(video models2.TbVideo) {
    // 创建用户日志助手
    userLogger := applogger.NewUserLogger(h.App.Logger.Desugar(), video.UserID)

    // 记录任务链开始
    userLogger.TaskLog(video.VideoId, "task_chain", "started",
        map[string]interface{}{
            "title": video.Title,
            "url":   video.URL,
        })

    // 执行任务链
    startTime := time.Now()
    result := chain.Run(true)
    duration := time.Since(startTime)

    // 检查执行结果
    success := len(chain.FailedTasks) == 0

    if success {
        // 记录成功
        userLogger.TaskLog(video.VideoId, "task_chain", "completed",
            map[string]interface{}{
                "duration_seconds": duration.Seconds(),
                "failed_tasks":     len(chain.FailedTasks),
            })
    } else {
        // 记录失败
        userLogger.TaskLog(video.VideoId, "task_chain", "failed",
            map[string]interface{}{
                "duration_seconds": duration.Seconds(),
                "failed_tasks":     len(chain.FailedTasks),
            })
    }
}
```

**输出**：
```
[user_12] 🚀 task_chain started video_id=xyz789 title="My Video" url="https://youtube.com/watch?v=xyz789"
[user_12] ✅ task_chain completed video_id=xyz789 duration_seconds=45.2 failed_tasks=0
```

---

### 3. 并发任务调度使用 UserLogHelper

**适用场景**：并发调度、worker pool 管理

**示例**：`internal/chain_task/chain_task_handler.go` - SetUp 函数

```go
go func(t models2.TbVideo, uid uint) {
    defer func() {
        <-h.workerPool
        h.inFlightTasks.Delete(t.VideoId)
        // 释放用户级并发槽位
        if h.ConcurrencyLimiter != nil && uid > 0 {
            h.ConcurrencyLimiter.Release(uid)
        }
    }()

    // 创建用户日志助手
    userLogger := applogger.NewUserLogger(h.App.Logger.Desugar(), uid)
    userLogger.TaskLog(t.VideoId, "schedule", "started",
        map[string]interface{}{
            "title": t.Title,
            "mode":  "concurrent",
        })

    h.RunTaskChain(t)

    userLogger.TaskLog(t.VideoId, "schedule", "completed",
        map[string]interface{}{
            "title": t.Title,
            "mode":  "concurrent",
        })
}(*task, userID)
```

**输出**：
```
[user_6] 🚀 schedule started video_id=abc123 title="Video 1" mode="concurrent"
[user_12] 🚀 schedule started video_id=def456 title="Video 2" mode="concurrent"
[user_6] ✅ schedule completed video_id=abc123 title="Video 1" mode="concurrent"
[user_12] ✅ schedule completed video_id=def456 title="Video 2" mode="concurrent"
```

---

## 📝 日志方法详解

### TaskLog 方法（推荐）

**用途**：记录任务执行的关键节点

**签名**：
```go
func (h *UserLogHelper) TaskLog(
    videoID string,       // 视频ID
    action string,        // 操作类型（download, upload_video, upload_subtitle, translate, etc.）
    status string,        // 状态（started, success, failed, retry, skipped）
    fields map[string]interface{}, // 额外字段
)
```

**自动添加的 emoji**：
- `download`: 📥
- `subtitle`: 📝
- `translate`: 🌐
- `metadata`: ✨
- `upload_video`: 📤
- `upload_subtitle`: 📄
- `retry`: 🔄
- `success`: ✅
- `failed`: ❌
- `pending`: ⏳

**示例**：
```go
// 下载视频
userLogger.TaskLog(videoID, "download", "started",
    map[string]interface{}{
        "url": "https://youtube.com/watch?v=abc123",
        "platform": "YouTube",
    })

// 字幕翻译
userLogger.TaskLog(videoID, "translate", "completed",
    map[string]interface{}{
        "source_lang": "en",
        "target_lang": "zh",
        "segments": 150,
    })

// 上传失败
userLogger.TaskLog(videoID, "upload_video", "failed",
    map[string]interface{}{
        "error": "rate limit exceeded",
        "retry_after": "3600s",
    })
```

### Warnw 方法（警告日志）

**用途**：记录非致命错误或警告

**示例**：
```go
userLogger.Warnw("检查用户上传权限失败",
    map[string]interface{}{
        "video_id": videoID,
        "error": err.Error(),
    })
```

**输出**：
```
[user_6] ⚠️ 检查用户上传权限失败 video_id=abc123 error="permission denied"
```

### Errorw 方法（错误日志）

**用途**：记录严重错误

**示例**：
```go
userLogger.Errorw("初始化任务步骤失败",
    map[string]interface{}{
        "video_id": videoID,
        "error": err.Error(),
    })
```

**输出**：
```
[user_6] ❌ 初始化任务步骤失败 video_id=abc123 error="database connection lost"
```

---

## 🎯 完整工作流示例

### 场景：用户提交视频 URL → 下载 → 上传

**用户 A（ID=6）提交视频**：
```
[user_6] 🚀 schedule started video_id=abc123 title="My Video" mode="concurrent"
[user_6] 🚀 task_chain started video_id=abc123 title="My Video" url="https://youtube.com/watch?v=abc123"
[user_6] 📥 download started video_id=abc123 url="https://youtube.com/watch?v=abc123"
[user_6] ✅ download success video_id=abc123 duration_seconds=120.5 file_size="256MB"
[user_6] 📝 subtitle started video_id=abc123 source_lang="en"
[user_6] ✅ subtitle success video_id=abc123 segments=200
[user_6] 🌐 translate started video_id=abc123 source_lang="en" target_lang="zh"
[user_6] ✅ translate success video_id=abc123 segments=200
[user_6] ✨ metadata started video_id=abc123
[user_6] ✅ metadata success video_id=abc123
[user_6] ✅ task_chain completed video_id=abc123 duration_seconds=245.3 failed_tasks=0
[user_6] ✅ schedule completed video_id=abc123 title="My Video" mode="concurrent"
[user_6] 📤 upload_video started video_id=abc123 title="My Video"
[user_6] ✅ upload_video success video_id=abc123 subtitle_scheduled_at="20:35:00"
[user_6] 📄 upload_subtitle started video_id=abc123
[user_6] ✅ upload_subtitle success video_id=abc123
```

**用户 B（ID=12）同时提交另一个视频**：
```
[user_12] 🚀 schedule started video_id=def456 title="Another Video" mode="concurrent"
[user_12] 🚀 task_chain started video_id=def456 title="Another Video" url="https://youtube.com/watch?v=def456"
[user_12] 📥 download started video_id=def456 url="https://youtube.com/watch?v=def456"
[user_12] ✅ download success video_id=def456 duration_seconds=95.2 file_size="128MB"
[user_12] ✅ task_chain completed video_id=def456 duration_seconds=180.1 failed_tasks=0
[user_12] ✅ schedule completed video_id=def456 title="Another Video" mode="concurrent"
[user_12] 📤 upload_video started video_id=def456 title="Another Video"
[user_12] ✅ upload_video success video_id=def456
```

---

## 🔍 日志搜索和过滤

### 搜索特定用户的所有操作

```bash
# 搜索用户 6 的所有日志
grep "user_6" logs.txt

# 搜索用户 6 的所有上传操作
grep "user_6.*upload" logs.txt

# 搜索用户 6 的失败操作
grep "user_6.*failed" logs.txt
```

### 使用 jq 过滤 JSON 日志

```bash
# 提取特定用户的所有日志
cat logs.json | jq 'select(.user_id == 6)'

# 统计每个用户的任务数量
cat logs.json | jq -r '.user_id' | sort | uniq -c

# 查看某个视频的完整流程
cat logs.json | jq 'select(.video_id == "abc123")'
```

---

## 📊 监控和分析

### 统计用户活动

```bash
# 统计每个用户的视频下载数
grep "download success" logs.txt | grep -oP 'user_\d+' | sort | uniq -c

# 统计每个用户的上传成功率
grep "upload_video" logs.txt | grep -oP 'user_\d+.*\K(success|failed)' | sort | uniq -c
```

### 错误分析

```bash
# 查找所有失败的操作
grep "failed" logs.txt

# 查找重试的操作
grep "retry" logs.txt

# 查找跳过的操作（权限不足）
grep "skipped" logs.txt
```

---

## ✅ 实施检查清单

- [x] 创建 `internal/logger/user_logger.go`
- [x] 创建 `internal/logger/context_logger.go`
- [x] 修改 `upload_scheduler.go` 添加用户上下文
- [x] 修改 `chain_task_handler.go` 添加用户上下文
- [x] 创建日志示例文档
- [ ] 重新构建后端
- [ ] 测试多用户场景日志输出
- [ ] 验证日志格式正确性

---

## 🚀 构建和测试

### 1. 构建后端

```bash
cd E:\githubitem\ytb2bili
go build -o ytb2bili.exe .
```

### 2. 运行并测试

```bash
# 启动服务器
./ytb2bili.exe

# 提交测试视频（用户 A）
curl -X POST http://localhost:8096/api/v1/videos \
  -H "Authorization: Bearer <user_a_token>" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://www.youtube.com/watch?v=test1"}'

# 提交测试视频（用户 B）
curl -X POST http://localhost:8096/api/v1/videos \
  -H "Authorization: Bearer <user_b_token>" \
  -H "Content-Type: application/json" \
  -d '{"url": "https://www.youtube.com/watch?v=test2"}'

# 查看控制台输出，验证日志格式
```

### 3. 预期日志输出

```
[user_6] 🚀 schedule started video_id=test1 title="Test Video 1" mode="concurrent"
[user_12] 🚀 schedule started video_id=test2 title="Test Video 2" mode="concurrent"
[user_6] 🚀 task_chain started video_id=test1 title="Test Video 1" url="https://www.youtube.com/watch?v=test1"
[user_12] 🚀 task_chain started video_id=test2 title="Test Video 2" url="https://www.youtube.com/watch?v=test2"
...
```

---

## 💡 最佳实践

### 1. 始终包含 videoID

**❌ 不好**：
```go
userLogger.Infow("开始下载视频", map[string]interface{}{})
```

**✅ 好**：
```go
userLogger.TaskLog(videoID, "download", "started", map[string]interface{}{
    "url": url,
})
```

### 2. 使用结构化字段

**❌ 不好**：
```go
userLogger.Infof("上传失败: %s, 错误: %s", videoID, err)
```

**✅ 好**：
```go
userLogger.TaskLog(videoID, "upload_video", "failed", map[string]interface{}{
    "error": err.Error(),
})
```

### 3. 记录关键指标

**✅ 推荐**：
```go
userLogger.TaskLog(videoID, "download", "success", map[string]interface{}{
    "duration_seconds": duration.Seconds(),
    "file_size_mb": fileSizeMB,
    "download_speed_mb_s": speed,
})
```

### 4. 保持一致的 action 名称

**推荐使用的 action**：
- `schedule` - 任务调度
- `task_chain` - 任务链执行
- `download` - 下载视频
- `subtitle` - 字幕生成
- `translate` - 字幕翻译
- `metadata` - 元数据生成
- `upload_video` - 视频上传
- `upload_subtitle` - 字幕上传

---

## 🎉 总结

通过实施多用户日志系统，现在可以：

1. ✅ 清晰区分不同用户的操作
2. ✅ 追踪单个用户的完整流程
3. ✅ 快速定位问题和调试
4. ✅ 统计用户活动和行为
5. ✅ 监控系统性能和错误

**日志格式**：`[user_N] emoji action status video_id=xxx ...`

**关键改进**：所有后台任务都使用 `UserLogHelper` 添加用户上下文前缀
