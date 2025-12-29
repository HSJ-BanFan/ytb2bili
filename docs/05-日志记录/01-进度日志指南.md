# 进度日志优化集成指南

## 概述

本指南说明如何使用新的智能进度日志系统来避免日志输出污染进度条。

## 核心组件

### 1. ProgressLogManager (进度日志管理器)
- 位置: `pkg/logger/progress_logger.go`
- 功能: 管理进度会话，智能过滤日志

### 2. SmartLogger (智能日志包装器)
- 位置: `pkg/logger/smart_logger.go`
- 功能: 自动检测进度活动并智能处理日志

### 3. CompactProgressLogger (紧凑进度条)
- 位置: `pkg/logger/compact_progress.go`
- 功能: 单行覆盖式进度显示

## 快速开始

### 步骤 1: 在 main.go 中初始化

在日志模块创建后添加初始化:

```go
// 在 main.go 的 fx.Provide 部分添加
fx.Invoke(func(lg *zap.SugaredLogger) {
    logger.InitProgressLogManager(lg)
}),
```

### 步骤 2: 在定时任务中使用 SmartLogger

替换 `chain_task_handler.go` 中的日志调用:

**之前:**
```go
h.App.Logger.Infof("发现 %d 个待重试的步骤", len(retrySteps))
h.App.Logger.Debug("没有待处理的任务")
```

**之后:**
```go
// 方案 A: 使用 SmartLogger (自动智能过滤)
smartLogger := logger.NewSmartLogger(h.App.Logger)
smartLogger.Infof("发现 %d 个待重试的步骤", len(retrySteps))
smartLogger.Debug("没有待处理的任务")

// 方案 B: 手动检查 (更精细控制)
progressMgr := logger.GetProgressManager()
if !progressMgr.IsActive() {
    h.App.Logger.Infof("发现 %d 个待重试的步骤", len(retrySteps))
}
```

### 步骤 3: 在下载视频时使用紧凑进度条

修改 `handlers/down_load_video.go`:

**之前 (多行进度框):**
```go
t.App.Logger.Info("╭─────────────────────────────────────────────────────────────────╮")
t.App.Logger.Info("│                    📥 视频下载进度                             │")
// ... 8 行输出
```

**之后 (单行紧凑进度):**
```go
import "your-project/pkg/logger"

// 在下载函数开始时
progressLogger := logger.NewCompactProgressLogger(t.StateManager.VideoID)
defer progressLogger.Complete("下载完成")

progressLogger.Start()

// 在进度更新时
progressLogger.UpdateProgressYTDLP(
    percent,    // 95.5
    total,      // "239.12MiB"
    speed,      // "4.62MiB/s"
    eta,        // "00:05"
)
```

## 日志过滤规则

当检测到进度活动时，SmartLogger 会自动:

### 完全跳过 (Debug):
- 所有 `Debug` 级别的日志

### 自动过滤 (Info):
包含以下关键词的日志会被静默:
- "没有待处理"
- "没有待上传"
- "未找到"
- "发现 %d 个待重试" (定时任务噪音)

### 始终显示 (Error/Warn/重要 Info):
- 所有 `Error` 和 `Warn` 级别
- 不在过滤列表中的 `Info` 日志
- 输出前会清除进度行，保证可见性

## 配置选项

### 自定义过滤规则

修改 `progress_logger.go` 中的 `LogfWithProgressCheck` 方法:

```go
if level == "info" {
    // 添加你自己的过滤规则
    if strings.Contains(template, "你的关键词") {
        return // 跳过
    }
    // ...
}
```

### 调整进度条样式

修改 `compact_progress.go` 中的 `formatProgressBar`:

```go
// 使用不同的字符
bar := strings.Repeat("▓", filled) + strings.Repeat("░", width-filled)

// 或使用 Unicode
bar := strings.Repeat("●", filled) + strings.Repeat("○", width-filled)
```

## 完整示例

### 示例 1: 定时任务集成

```go
// chain_task_handler.go
func (h *ChainTaskHandler) processRetrySteps() {
    smartLogger := logger.NewSmartLogger(h.App.Logger)

    retrySteps, err := h.getRetrySteps()
    if err != nil {
        smartLogger.Errorf("查询重试步骤失败: %v", err)
        return
    }

    if len(retrySteps) > 0 {
        // 如果没有进度活动，才输出
        smartLogger.Infof("发现 %d 个待重试的步骤", len(retrySteps))
        // 处理重试...
    }
}
```

### 示例 2: 下载进度集成

```go
// handlers/down_load_video.go
func (t *DownloadVideo) executeDownload(...) bool {
    progressLogger := logger.NewCompactProgressLogger(t.StateManager.VideoID)
    defer progressLogger.Complete("视频下载成功")

    progressLogger.Start()
    progressLogger.UpdateProgressYTDLP(0, total, "0B/s", "计算中")

    // 执行下载...
    go func() {
        scanner := bufio.NewScanner(stdout)
        for scanner.Scan() {
            // 解析进度
            if percent, total, speed, eta := parseProgress(scanner.Text()); percent >= 0 {
                progressLogger.UpdateProgressYTDLP(percent, total, speed, eta)
            }
        }
    }()

    // 等待完成...
}
```

### 示例 3: 手动管理进度会话

```go
func someLongRunningOperation(logger *zap.SugaredLogger) {
    progressMgr := logger.GetProgressManager()

    // 开始进度会话
    cleanup := progressMgr.StartProgress()
    defer cleanup()

    // 在进度会话期间，其他日志会被智能过滤
    for i := 0; i < 100; i++ {
        fmt.Printf("\r进度: %d%%", i)
        time.Sleep(100 * time.Millisecond)
    }

    // 会话结束，自动恢复日志
}
```

## 性能考虑

- **原子操作**: 使用 `atomic.Int32` 保证线程安全，无锁竞争
- **内存占用**: 每个进度会话仅占用 4 字节 (int32)
- **CPU 开销**: 字符串匹配和过滤 < 1μs
- **适用场景**: 适合高并发场景，支持多个同时进行的进度显示

## 故障排查

### 进度条没有显示
- 确保 `InitProgressLogManager` 在 main.go 中被调用
- 检查终端是否支持 ANSI 转义码

### 日志仍然被污染
- 确保使用了 `SmartLogger` 包装器
- 检查日志关键词是否在过滤列表中

### 性能下降
- 使用 `Debug` 级别来完全静默非必要日志
- 减少进度更新频率 (例如每 3 秒更新一次)

## 迁移检查清单

- [ ] 在 main.go 中添加 `InitProgressLogManager`
- [ ] 在定时任务中使用 `SmartLogger`
- [ ] 在下载视频中替换为 `CompactProgressLogger`
- [ ] 测试并发场景下的日志输出
- [ ] 验证 ANSI 转义码在目标终端的兼容性
