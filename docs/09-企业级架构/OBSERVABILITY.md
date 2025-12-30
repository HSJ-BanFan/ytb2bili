# ytb2bili 可观测性 (Observability) 开发文档

> **版本**: v1.0
> **日期**: 2025-12-29
> **目标读者**: 架构师、高级开发人员、DevOps 工程师、SRE 工程师

---

## 📋 目录

1. [执行摘要](#执行摘要)
2. [可观测性三大支柱](#可观测性三大支柱)
3. [当前日志系统分析](#当前日志系统分析)
4. [指标监控现状](#指标监控现状)
5. [链路追踪缺失](#链路追踪缺失)
6. [企业级改造方案](#企业级改造方案)
7. [告警体系设计](#告警体系设计)
8. [实施路线图](#实施路线图)
9. [最佳实践建议](#最佳实践建议)

---

## 执行摘要

### 🎯 当前状态

ytb2bili 是一个**基础可观测性**的视频转存系统，具备良好的日志分级，但距离企业级"5分钟定位问题"要求有明显差距：

| 可观测性支柱 | 当前状态 | 企业级要求 | 差距 |
|------------|---------|-----------|------|
| **Logging (日志)** | ✅ Zap结构化日志 | ✅ 集中式日志检索 | **中等** |
| **Metrics (指标)** | ❌ 无标准指标 | ✅ Prometheus仪表盘 | **严重** |
| **Tracing (追踪)** | ❌ 无链路追踪 | ✅ 分布式追踪 | **严重** |
| **Alerting (告警)** | ❌ 无主动告警 | ✅ 多渠道告警 | **严重** |
| **Dashboard (仪表盘)** | ❌ 无可视化 | ✅ Grafana监控面板 | **严重** |

### 📊 总体评估

**日志完整性**: **7/10** - 有结构化日志，但缺少集中式管理
**指标可见性**: **2/10** - 几乎无指标暴露
**链路可视化**: **0/10** - 完全缺失
**告警及时性**: **1/10** - 仅依赖用户反馈

**结论**: 系统具备基础日志记录能力，但**缺少完整的可观测性体系**，无法在5分钟内定位故障根因。

---

## 可观测性三大支柱

### 📊 维度对比图

```
┌─────────────────────────────────────────────────────────────────────┐
│                        可观测性成熟度模型                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Level 1 (基础):  🟢 🟢 🟢 ⚪ ⚪  ✅ 部分达成                         │
│  Level 2 (可观测): 🟢 🟢 ⚪ ⚪ ⚪  ❌ 未达成                          │
│  Level 3 (可操作): 🟢 ⚪ ⚪ ⚪ ⚪  ❌ 未达成                          │
│  Level 4 (自动化): ⚪ ⚪ ⚪ ⚪ ⚪  ❌ 未达成                          │
│                                                                     │
│  当前状态: Level 1 → Level 2                                        │
│  目标状态: Level 4 (完全可观测 + 自动化运维)                        │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 🔍 三大支柱定义

#### 1. **Logging (日志)** - "发生了什么？"

**当前实现**:
```
pkg/logger/logger.go:13
├── Zap 结构化日志 ✅
├── Lumberjack 日志轮转 ✅
├── 分级日志 (Debug/Info/Warn/Error) ✅
└── 本地文件存储 (logs/app.log) ⚠️
```

**企业级要求**:
- ✅ 结构化日志 (已实现)
- ❌ 集中式收集 (需 ELK/Loki)
- ❌ 全文检索 (需 Kibana/Grafana)
- ❌ 日志关联分析 (需 TraceID)
- ❌ 日志采样 (高流量场景)

#### 2. **Metrics (指标)** - "发生了多少？"

**当前实现**:
```
❌ 无 Prometheus 指标暴露
❌ 无 Grafana 仪表盘
❌ 无实时监控面板
⚠️  仅依赖数据库查询统计
```

**企业级要求**:
- ✅ RED 指标 (Rate/Errors/Duration)
- ✅ USE 指标 (Utilization/Saturation/Errors)
- ✅ 业务指标 (任务成功率、用户活跃度)
- ✅ 系统指标 (CPU/Memory/Disk/Network)

#### 3. **Tracing (追踪)** - "在哪里发生？"

**当前实现**:
```
❌ 无 OpenTelemetry 集成
❌ 无分布式追踪 (Jaeger/Tempo)
❌ 无请求链路可视化
❌ 无跨服务调用追踪
```

**企业级要求**:
- ✅ TraceID 传递 (全链路)
- ✅ Span 采集 (HTTP/DB/External API)
- ✅ 调用拓扑图
- ✅ 性能瓶颈定位

---

## 当前日志系统分析

### ✅ 强项

#### 1. **结构化日志** (Zap)

**位置**: `pkg/logger/logger.go`

```go
// ✅ 优点: 结构化字段，易于机器解析
encoderConfig := zapcore.EncoderConfig{
    TimeKey:        "time",
    LevelKey:       "level",
    NameKey:        "logger",
    CallerKey:      "caller",  // 自动记录调用位置
    MessageKey:     "msg",
    StacktraceKey:  "stacktrace",
}

// ✅ 输出示例 (JSON格式)
{
  "time": "2025-12-29T10:30:45Z",
  "level": "info",
  "caller": "chain_task_handler.go:142",
  "msg": "🔄 [并发] 开始重试步骤: abc123 - 下载视频",
  "video_id": "abc123",
  "step_name": "下载视频",
  "user_id": "1001"
}
```

#### 2. **日志轮转** (Lumberjack)

**位置**: `pkg/logger/logger.go:51-58`

```go
// ✅ 自动日志管理
lumberJackLogger := &lumberjack.Logger{
    Filename:   "logs/app.log",
    MaxSize:    10,    // 单文件最大 10MB
    MaxBackups: 5,     // 保留最近 5 个备份
    MaxAge:     30,    // 保留 30 天
    Compress:   true,  // 自动压缩旧日志
}
```

**问题**:
- ⚠️ **单机文件**: 多实例无法汇总
- ⚠️ **无索引**: 搜索需 grep 遍历全文件
- ⚠️ **磁盘风险**: 日志过多可能占满磁盘

#### 3. **上下文日志** (ContextLogger)

**位置**: `internal/logger/context_logger.go`

```go
// ✅ 自动注入用户上下文
func (l *ContextLogger) TaskLog(c *gin.Context, videoID, action, status string, fields map[string]interface{}) {
    user := getUserContext(c)  // 提取 userID, username, tier

    allFields := append(allFields,
        zap.String("user_id", user.UserID),
        zap.String("username", user.Username),
        zap.String("tier", user.Tier),
        zap.String("video_id", videoID),
        zap.String("action", action),
        zap.String("status", status),
    )
}

// ✅ 输出示例
{
  "user_id": "1001",
  "username": "john_doe",
  "tier": "pro",
  "video_id": "abc123",
  "action": "download_video",
  "status": "completed",
  "duration_ms": 45230
}
```

#### 4. **智能日志过滤** (SmartLogger)

**位置**: `pkg/logger/smart_logger.go`

```go
// ✅ 自动过滤进度噪音日志
type SmartLogger struct {
    baseLogger *zap.SugaredLogger
    lastProgressTime sync.Map  // 跟踪上次输出时间
}

// ✅ 30秒内同一进度只记录一次
func (l *SmartLogger) Infof(msg string, args ...interface{}) {
    if l.shouldSuppressProgress(msg) {
        return  // 跳过频繁的进度日志
    }
    l.baseLogger.Infof(msg, args...)
}
```

### ❌ 短板

#### 1. **无集中式日志管理**

**问题场景**:
```
场景1: 生产环境故障排查
  用户反馈: "我的视频上传失败"
  开发人员: 需登录服务器 → SSH → cd logs → grep app.log "user_1001"
  耗时: 10-30 分钟
  问题: 无法跨服务器查询，无法实时告警
```

**企业级方案**:
```
应用服务器 ──> Loki/Fluentd ──> Loki Cluster ──> Grafana
     │                                               │
     └───────────────────────────────────────────────┘
                    全文搜索 + 可视化
```

#### 2. **无关联标识**

**当前问题**:
```json
// ❌ 日志1: API请求日志
{"time": "2025-12-29T10:30:00Z", "msg": "POST /api/v1/videos", "user_id": "1001"}

// ❌ 日志2: 任务执行日志 (无法关联到上面的请求)
{"time": "2025-12-29T10:30:05Z", "msg": "开始下载视频", "video_id": "abc123"}

// ❌ 日志3: 错误日志 (无法关联到前面的操作)
{"time": "2025-12-29T10:31:00Z", "level": "error", "msg": "上传失败"}
```

**企业级方案**:
```json
// ✅ 所有日志共享 TraceID
{"trace_id": "req-abc123-def456", "msg": "POST /api/v1/videos", ...}
{"trace_id": "req-abc123-def456", "msg": "开始下载视频", "span_id": "span-1", ...}
{"trace_id": "req-abc123-def456", "msg": "上传失败", "span_id": "span-2", ...}

// ✅ 在 Grafana/Loki 一键查询:
// {trace_id="req-abc123-def456"} → 看到完整调用链
```

#### 3. **无敏感信息脱敏**

**风险示例**:
```go
// ❌ 当前代码可能记录敏感信息
logger.Infof("Bilibili API Response: %s", response)  // 可能包含 Cookie
logger.Infof("User login: %s", credentials)          // 密码泄露风险
```

**企业级方案**:
```go
// ✅ 自动脱敏
logger.Infof("Bilibili API Response: %s", sanitize(response))
// 输出: "Bilibili API Response: {cookies: [REDACTED]}"

// ✅ 结构化脱敏
type SafeUser struct {
    Username string `json:"username"`
    Password string `json:"-"`  // 永不记录
}
```

---

## 指标监控现状

### ❌ 当前缺失的指标

#### 1. **RED 指标** (关键业务指标)

| 指标 | 定义 | 当前状态 | 影响 |
|------|------|---------|------|
| **Rate (请求速率)** | 每秒处理任务数 | ❌ 无监控 | 无法感知流量突增 |
| **Errors (错误率)** | 失败任务占比 | ⚠️ 仅查DB | 无法实时告警 |
| **Duration (延迟)** | 任务处理耗时 | ⚠️ 仅存DB | 无法性能优化 |

**现状代码** (`internal/core/services/task_step_service.go`):
```go
// ❌ 仅存储到数据库，无实时指标
type TaskStep struct {
    StartTime  *time.Time
    EndTime    *time.Time
    Duration   int64      // 毫秒
    ErrorMsg   string
}

// 查询统计需手动执行SQL
SELECT AVG(duration) FROM cw_task_steps WHERE status = 'completed';
```

**企业级方案** (Prometheus):
```go
// ✅ 实时暴露指标
var taskDuration = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "task_duration_seconds",
        Help: "任务处理时长分布",
        Buckets: prometheus.ExponentialBuckets(1, 2, 10), // 1s, 2s, 4s, ...
    },
    []string{"step_name", "status"},
)

// ✅ 记录指标
func (h *Handler) ExecuteTask() {
    start := time.Now()
    defer func() {
        duration := time.Since(start).Seconds()
        taskDuration.WithLabelValues("download_video", "success").Observe(duration)
    }()

    // ... 执行任务 ...
}

// ✅ Grafana 实时查询 P99 延迟
// histogram_quantile(0.99, task_duration_seconds)
```

#### 2. **USE 指标** (系统资源指标)

| 资源 | 指标 | 当前状态 | 企业级要求 |
|------|------|---------|-----------|
| **CPU** | 使用率/核数 | ⚠️ 需手动 `top` | ✅ node_exporter |
| **Memory** | 使用量/GC次数 | ⚠️ 需手动查看 | ✅ Go runtime metrics |
| **Disk** | IOPS/使用率 | ⚠️ 有 SystemMonitor | ✅ 持久化监控 |
| **Network** | 带宽/连接数 | ❌ 无监控 | ✅ 实时告警 |

**现有代码** (`pkg/utils/system_monitor.go`):
```go
// ⚠️ 仅为命令行工具，未暴露为指标
func (sm *SystemMonitor) GetMemoryUsage() (allocMB, allocGB, sysGB uint64) {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    return m.Alloc / 1024 / 1024, ...
}

// ❌ 仅在启动时打印，无法持续监控
sm.PrintSystemInfo()
```

**企业级方案**:
```go
// ✅ 暴露为 Prometheus 指标
import "github.com/prometheus/client_golang/prometheus/promauto"

var memoryUsage = promauto.NewGaugeVec(
    prometheus.GaugeOpts{
        Name: "go_memory_usage_bytes",
        Help: "Go 内存使用量",
    },
    []string{"type"}, // heap/stack/sys
)

// ✅ 定时采集 (每10秒)
go func() {
    ticker := time.NewTicker(10 * time.Second)
    for range ticker.C {
        var m runtime.MemStats
        runtime.ReadMemStats(&m)
        memoryUsage.WithLabelValues("heap").Set(float64(m.HeapAlloc))
        memoryUsage.WithLabelValues("sys").Set(float64(m.Sys))
    }
}()
```

#### 3. **业务指标**

**关键业务指标**:
```
❌ 活跃用户数 (DAU/MAU)
❌ 任务队列积压数
❌ 视频上传成功率
❌ Bilibili API 调用次数/配额
❌ 存储空间使用趋势
⚠️  部分数据在 Analytics 表 (需SQL查询)
```

**现状代码** (`pkg/analytics/client.go`):
```go
// ⚠️ 事件数据发送到自定义分析服务 (非Prometheus)
func (c *Client) TrackAPIRequest(ctx context.Context, endpoint, method, userID, deviceID string, statusCode int, duration time.Duration) error {
    // 发送到 go-analysis-server
    // ❌ 无法与 Prometheus/Grafana 集成
    // ❌ 无法设置告警规则
}
```

---

## 链路追踪缺失

### 🔴 问题场景

**场景**: 用户反馈 "视频上传失败"

**当前排查流程** (无链路追踪):
```
1. 查看 API 日志
   grep "user_1001" logs/app.log
   → 找到: POST /api/v1/videos (10:30:00)

2. 查看任务日志
   grep "video_abc123" logs/app.log
   → 找到多个日志片段，时间分散

3. 查看 Bilibili API 调用
   grep "Bilibili Upload" logs/app.log
   → 找到错误: (10:35:20) HTTP 400

4. 问题:
   - 无法看到完整调用链
   - 无法定位是哪个步骤出错
   - 无法计算总耗时
   - 多个服务器日志无法关联
```

**企业级方案** (有链路追踪):
```
1. 在 Grafana Tempo 查询 TraceID: "req-abc123"

2. 看到 Waterfall 视图:
   ┌────────────────────────────────────────────────────────┐
   │ POST /api/v1/videos          [0ms ────────── 45ms]      │
   │ ├─ 创建任务记录               [1ms ──── 5ms]            │
   │ ├─ 调用 yt-dlp               [6ms ─────────── 40000ms] │
   │ │  └─ 下载视频                 [10ms ──────── 39900ms] │
   │ ├─ 生成字幕                   [40010ms ───── 45000ms]   │
   │ └─ 上传到 Bilibili           [45010ms ──── 50000ms]    │
   │    └─ HTTP 400 Error (10:35:00)                        │
   └────────────────────────────────────────────────────────┘

3. 一目了然:
   - 总耗时: 50秒
   - 瓶颈: yt-dlp 下载占 80% 时间
   - 根因: Bilibili API 返回 400 (标题过长)
```

### 🔧 技术实现

**OpenTelemetry + Jaeger/Tempo 架构**:

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Frontend   │────▶│   ytb2bili   │────▶│  Bilibili    │
│   (Browser)  │     │   (Backend)  │     │     API      │
└──────────────┘     └──────┬───────┘     └──────────────┘
                             │
                             ▼
                    ┌────────────────┐
                    │ OpenTelemetry  │
                    │    SDK (Go)    │
                    └────────┬───────┘
                             │
                             ▼
                    ┌────────────────┐
                    │   Jaeger/Tempo │
                    │   (Collector)  │
                    └────────┬───────┘
                             │
                             ▼
                    ┌────────────────┐
                    │   Grafana      │
                    │   (Dashboard)  │
                    └────────────────┘
```

**代码改造示例**:

```go
// 1. 添加依赖
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

// 2. 初始化 Tracer
func main() {
    // 初始化 OpenTelemetry
    tp, err := initTracer()
    if err != nil {
        log.Fatal(err)
    }
    defer func() {
        if err := tp.Shutdown(context.Background()); err != nil {
            log.Printf("Error shutting down tracer provider: %v", err)
        }
    }()
    otel.SetTracerProvider(tp)
}

// 3. 在关键函数中添加 Span
func (h *Handler) DownloadVideo(ctx context.Context, videoID string) error {
    ctx, span := otel.Tracer("worker").Start(ctx, "DownloadVideo")
    defer span.End()

    // 添加属性
    span.SetAttributes(
        attribute.String("video_id", videoID),
        attribute.String("user_id", getUserID(ctx)),
    )

    // 调用子函数 (自动创建 child span)
    if err := h.callYtDlp(ctx, videoID); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return err
    }

    span.SetStatus(codes.Ok, "Download completed")
    return nil
}

// 4. HTTP 中间件自动追踪
func TracingMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx, span := otel.Tracer("http").Start(
            c.Request.Context(),
            c.Request.URL.Path,
            trace.WithAttributes(
                attribute.String("method", c.Request.Method),
                attribute.String("path", c.Request.URL.Path),
            ),
        )
        defer span.End()

        // 注入 TraceID 到响应头
        c.Writer.Header().Set("X-Trace-ID", span.SpanContext().TraceID().String())

        c.Next()
    }
}
```

---

## 企业级改造方案

### 🎯 改造目标

1. **5分钟定位问题**: 从故障发生到根因分析 < 5分钟
2. **主动告警**: 关键指标异常自动通知
3. **可视化监控**: 一目了然的仪表盘
4. **可追溯**: 完整的调用链路追踪

---

### 📐 推荐架构

```
┌───────────────────────────────────────────────────────────────────────┐
│                           应用层                                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                │
│  │ ytb2bili-1   │  │ ytb2bili-2   │  │ ytb2bili-N   │                │
│  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────┐ │                │
│  │ │OpenTel   │ │  │ │OpenTel   │ │  │ │OpenTel   │ │                │
│  │ │emetry SDK│ │  │ │emetry SDK│ │  │ │emetry SDK│ │                │
│  │ └────┬─────┘ │  │ └────┬─────┘ │  │ └────┬─────┘ │                │
│  │      │       │  │      │       │  │      │       │                │
│  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────┐ │                │
│  │ │Prometheus│ │  │ │Prometheus│ │  │ │Prometheus│ │                │
│  │ │Exporter  │ │  │ │Exporter  │ │  │ │Exporter  │ │                │
│  │ └──────────┘ │  │ └──────────┘ │  │ └──────────┘ │                │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘                │
│         │                 │                 │                         │
└─────────┼─────────────────┼─────────────────┼─────────────────────────┘
          │                 │                 │
          ▼                 ▼                 ▼
┌───────────────────────────────────────────────────────────────────────┐
│                         可观测性平台                                    │
│  ┌────────────────┐    ┌────────────────┐    ┌────────────────┐       │
│  │   Prometheus   │    │     Loki       │    │  Jaeger/Tempo  │       │
│  │   (指标存储)   │    │   (日志存储)    │    │   (链路存储)   │       │
│  └────────┬───────┘    └────────┬───────┘    └────────┬───────┘       │
│           │                     │                     │               │
│           └─────────────────────┼─────────────────────┘               │
│                                 ▼                                     │
│                    ┌────────────────────┐                             │
│                    │     Grafana        │                             │
│                    │   (可视化面板)      │                             │
│                    └────────────────────┘                             │
└───────────────────────────────────────────────────────────────────────┘
          │                     │                     │
          ▼                     ▼                     ▼
┌───────────────────────────────────────────────────────────────────────┐
│                         告警层                                         │
│  ┌────────────────┐    ┌────────────────┐    ┌────────────────┐       │
│  │ AlertManager   │───▶│  Email/Slack   │    │    PagerDuty   │       │
│  └────────────────┘    └────────────────┘    └────────────────┘       │
└───────────────────────────────────────────────────────────────────────┘
```

---

### 🔧 方案1: 集中式日志管理 (Loki)

#### 为什么选择 Loki?

- **轻量级**: 相比 ELK 节省 80% 存储成本
- **Grafana 原生集成**: 与指标共用 Grafana
- **Label 索引**: 快速检索
- **LogQL**: 类似 PromQL 的查询语言

#### 实施步骤

**Step 1: 部署 Loki**

```bash
# 使用 Docker Compose
version: '3'
services:
  loki:
    image: grafana/loki:latest
    ports:
      - "3100:3100"
    command: -config.file=/etc/loki/local-config.yaml
    volumes:
      - ./loki-config.yaml:/etc/loki/local-config.yaml

  promtail:
    image: grafana/promtail:latest
    volumes:
      - ./promtail-config.yaml:/etc/promtail/config.yml
      - /var/log:/var/log:ro
      - ./logs:/app/logs:ro
    depends_on:
      - loki
```

**Step 2: 配置 Promtail (日志采集)**

```yaml
# promtail-config.yaml
server:
  http_listen_port: 9080

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://loki:3100/loki/api/v1/push

scrape_configs:
  - job_name: ytb2bili
    static_configs:
      - targets:
          - localhost
        labels:
          app: ytb2bili
          env: production
          host: ${HOSTNAME}

    pipeline_stages:
      # 解析 JSON 日志
      - json:
          expressions:
            time: time
            level: level
            msg: msg
            user_id: user_id
            video_id: video_id
            trace_id: trace_id

      # 添加 TraceID 标签
      - labels:
          trace_id:

      # 解析时间
      - timestamp:
          source: time
          format: RFC3339
```

**Step 3: 修改应用日志 (添加 TraceID)**

```go
// pkg/logger/trace_logger.go
package logger

import (
    "context"

    "go.opentelemetry.io/otel/trace"
    "go.uber.org/zap"
)

// TraceLogger 带链路追踪的日志器
type TraceLogger struct {
    base *zap.SugaredLogger
    tracer trace.Tracer
}

func (l *TraceLogger) InfoWithContext(ctx context.Context, msg string, fields ...zap.Field) {
    // 提取 TraceID
    span := trace.SpanFromContext(ctx)
    if span.SpanContext().IsValid() {
        traceID := span.SpanContext().TraceID().String()
        fields = append(fields, zap.String("trace_id", traceID))
    }

    l.base.Infow(msg, fields...)
}
```

**Step 4: Grafana 查询**

```logql
# 查询特定用户的所有日志
{app="ytb2bili", user_id="1001"}

# 查询特定 TraceID 的完整调用链
{app="ytb2bili"} |= "req-abc123"

# 统计错误率
count_over_time({app="ytb2bili", level="error"}[5m])

# 查询慢任务 (>10秒)
{app="ytb2bili"} |= "duration_ms" | json | duration_ms > 10000
```

---

### 🔧 方案2: Prometheus 指标监控

#### 关键指标定义

```go
// pkg/metrics/metrics.go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // 1. 业务指标
    TaskProcessedTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "task_processed_total",
            Help: "任务处理总数",
        },
        []string{"step_name", "status"}, // status: success/failed
    )

    TaskDurationSeconds = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "task_duration_seconds",
            Help:    "任务处理时长(秒)",
            Buckets: prometheus.ExponentialBuckets(1, 2, 13), // 1s ~ 4096s
        },
        []string{"step_name"},
    )

    TaskQueueBacklog = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "task_queue_backlog",
            Help: "任务队列积压数",
        },
    )

    // 2. API 指标
    HTTPRequestTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "HTTP请求总数",
        },
        []string{"method", "endpoint", "status"},
    )

    HTTPRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP请求时长",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "endpoint"},
    )

    // 3. 系统指标
    DatabaseConnections = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "db_connections_active",
            Help: "数据库活跃连接数",
        },
    )

    BilibiliAPICallsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "bilibili_api_calls_total",
            Help: "Bilibili API调用总数",
        },
        []string{"endpoint", "status"},
    )
)

// RecordTaskSuccess 记录任务成功
func RecordTaskSuccess(stepName string, duration float64) {
    TaskProcessedTotal.WithLabelValues(stepName, "success").Inc()
    TaskDurationSeconds.WithLabelValues(stepName).Observe(duration)
}

// RecordTaskFailure 记录任务失败
func RecordTaskFailure(stepName string, err error) {
    TaskProcessedTotal.WithLabelValues(stepName, "failed").Inc()
}
```

#### 集成到现有代码

```go
// internal/chain_task/chain_task_handler.go
import "github.com/difyz9/ytb2bili/pkg/metrics"

func (h *ChainTaskHandler) RunTaskChain(videoID string, userID uint) error {
    startTime := time.Now()

    // 1. 更新队列积压指标
    pendingTasks, _ := h.getPendingTasks()
    metrics.TaskQueueBacklog.Set(float64(len(pendingTasks)))

    // 2. 执行任务链
    err := h.executeChain(videoID, userID)

    // 3. 记录指标
    duration := time.Since(startTime).Seconds()
    if err != nil {
        metrics.RecordTaskFailure("task_chain", err)
    } else {
        metrics.RecordTaskSuccess("task_chain", duration)
    }

    return err
}
```

#### Prometheus 配置

```yaml
# prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'ytb2bili'
    static_configs:
      - targets: ['localhost:8096']
    metrics_path: '/metrics'
```

#### 暴露 Metrics 端点

```go
// internal/core/app_server.go
import (
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *AppServer) setupMetrics() {
    // 暴露 /metrics 端点
    s.Engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
}
```

---

### 🔧 方案3: 分布式追踪 (Jaeger/Tempo)

#### 部署 Jaeger

```bash
docker run -d \
  --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 4317:4317 \
  -p 4318:4318 \
  -p 16686:16686 \
  jaegertracing/all-in-one:latest
```

#### 集成 OpenTelemetry

```go
// pkg/tracing/tracer.go
package tracing

import (
    "context"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/sdk/resource"
    tracesdk "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func InitTracer(serviceName, jaegerEndpoint string) (*tracesdk.TracerProvider, error) {
    // 创建 Jaeger Exporter
    exp, err := jaeger.New(jaeger.WithCollectorEndpoint(
        jaeger.WithEndpoint(jaegerEndpoint),
    ))
    if err != nil {
        return nil, err
    }

    // 创建 TracerProvider
    tp := tracesdk.NewTracerProvider(
        tracesdk.WithBatcher(exp),
        tracesdk.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String(serviceName),
        )),
    )

    otel.SetTracerProvider(tp)
    return tp, nil
}
```

#### 在 Handler 中使用

```go
// internal/handler/video_handler.go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

type VideoHandler struct {
    tracer trace.Tracer
}

func (h *VideoHandler) CreateVideo(c *gin.Context) {
    // 创建 Span
    ctx, span := h.tracer.Start(
        c.Request.Context(),
        "CreateVideo",
        trace.WithAttributes(
            attribute.String("user_id", getUserID(c)),
        ),
    )
    defer span.End()

    // 调用 Service
    video, err := h.service.CreateVideo(ctx, req)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    // 返回 TraceID 给前端
    c.JSON(200, gin.H{
        "video_id": video.ID,
        "trace_id": span.SpanContext().TraceID().String(),
    })
}
```

---

### 🔧 方案4: 告警系统 (AlertManager)

#### 告警规则

```yaml
# alerting_rules.yml
groups:
  - name: ytb2bili_alerts
    interval: 30s
    rules:
      # 1. 任务积压告警
      - alert: HighTaskBacklog
        expr: task_queue_backlog > 1000
        for: 5m
        labels:
          severity: warning
          team: backend
        annotations:
          summary: "任务队列积压过多"
          description: "当前积压 {{ $value }} 个任务，超过阈值 1000"

      # 2. 任务失败率告警
      - alert: HighTaskFailureRate
        expr: |
          rate(task_processed_total{status="failed"}[5m]) /
          rate(task_processed_total[5m]) > 0.1
        for: 10m
        labels:
          severity: critical
        annotations:
          summary: "任务失败率过高"
          description: "过去10分钟失败率: {{ $value | humanizePercentage }}"

      # 3. P99 延迟告警
      - alert: HighTaskLatency
        expr: |
          histogram_quantile(0.99,
            sum(rate(task_duration_seconds_bucket[5m])) by (step_name, le)
          ) > 300
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "任务处理延迟过高"
          description: "{{ $labels.step_name }} P99延迟: {{ $value }}s"

      # 4. 应用宕机告警
      - alert: ApplicationDown
        expr: up{job="ytb2bili"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "应用实例宕机"
          description: "{{ $labels.instance }} 已下线超过1分钟"

      # 5. 数据库连接满
      - alert: DatabaseConnectionPoolFull
        expr: db_connections_active / db_connections_max > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "数据库连接池接近满"
          description: "连接使用率: {{ $value | humanizePercentage }}"

      # 6. Bilibili API 错误率
      - alert: BilibiliAPIHighErrorRate
        expr: |
          rate(bilibili_api_calls_total{status=~"5.."}[5m]) /
          rate(bilibili_api_calls_total[5m]) > 0.05
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Bilibili API 错误率过高"
          description: "5xx错误率: {{ $value | humanizePercentage }}"
```

#### AlertManager 配置

```yaml
# alertmanager.yml
global:
  resolve_timeout: 5m

# 告警路由
route:
  group_by: ['alertname', 'severity']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 12h
  receiver: 'default'

  routes:
    # Critical 告警立即发送
    - match:
        severity: critical
      receiver: 'pagerduty'
      continue: true

    # Warning 告警聚合发送
    - match:
        severity: warning
      receiver: 'slack'
      group_wait: 5m

# 接收器配置
receivers:
  - name: 'default'
    webhook_configs:
      - url: 'http://localhost:5001/webhook'

  - name: 'slack'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/YOUR/WEBHOOK/URL'
        channel: '#alerts'
        title: '{{ .GroupLabels.alertname }}'
        text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'

  - name: 'pagerduty'
    pagerduty_configs:
      - service_key: 'YOUR_PAGERDUTY_KEY'
```

---

## 告警体系设计

### 📊 告警分级

| 级别 | 响应时间 | 通知渠道 | 示例场景 |
|------|---------|---------|---------|
| **P0 - Critical** | <5分钟 | PagerDuty/电话 | 应用宕机、数据库不可用 |
| **P1 - High** | <15分钟 | Slack/短信 | 任务失败率>10%、API错误率>5% |
| **P2 - Medium** | <1小时 | Slack/邮件 | 队列积压>1000、P99延迟>5分钟 |
| **P3 - Low** | <1天 | 邮件 | 磁盘使用>80%、日志异常 |

### 🎯 告警最佳实践

#### 1. **避免告警疲劳**

```yaml
# ❌ 错误: 告警过于敏感
- alert: TooManyErrors
  expr: rate(errors_total[1m]) > 0  # 任何错误都告警

# ✅ 正确: 设置持续时间和阈值
- alert: HighErrorRate
  expr: rate(errors_total[5m]) > 0.1  # 10%错误率
  for: 10m  # 持续10分钟才告警
```

#### 2. **告警聚合**

```yaml
# ✅ 将相关告警分组
route:
  group_by: ['cluster', 'service']  # 同一服务的告警合并
  group_wait: 30s  # 30秒内收集所有相关告警
```

#### 3. **告警抑制**

```yaml
# ✅ 主告警触发时，抑制次要告警
inhibit_rules:
  - source_match:
      severity: 'critical'
    target_match:
      severity: 'warning'
    equal: ['alertname', 'instance']
```

---

## 实施路线图

### 🎯 第一阶段: 基础可观测性 (2-3周)

**目标**: 从0到1，建立基本监控能力

**任务清单**:

#### Week 1: 日志集中化
- [ ] 部署 Loki + Promtail
- [ ] 配置日志采集规则
- [ ] Grafana 添加 Loki 数据源
- [ ] 创建基础日志查询面板

**验收标准**:
- ✅ 所有应用日志实时采集到 Loki
- ✅ 可通过 Grafana 查询任意时间段日志
- ✅ 支持按 user_id、video_id、trace_id 过滤

#### Week 2: 核心指标暴露
- [ ] 集成 Prometheus Client Go
- [ ] 定义核心业务指标 (TaskProcessed, TaskDuration)
- [ ] 暴露 /metrics 端点
- [ ] 部署 Prometheus 采集指标
- [ ] 创建 Grafana 基础仪表盘

**验收标准**:
- ✅ Prometheus 可采集到 /metrics 数据
- ✅ Grafana 显示任务处理速率、延迟分布
- ✅ 可查询实时队列积压数

#### Week 3: 基础告警
- [ ] 部署 AlertManager
- [ ] 配置 Slack/Webhook 告警
- [ ] 创建关键告警规则 (应用宕机、高错误率)
- [ ] 测试告警触发和通知

**验收标准**:
- ✅ 应用宕机时 1分钟内收到告警
- ✅ 任务失败率>10% 时收到 Slack 通知
- ✅ 告警包含足够上下文信息

---

### 🎯 第二阶段: 完善可观测性 (3-4周)

**目标**: 达到企业级"5分钟定位问题"标准

**任务清单**:

#### Week 4: 分布式追踪
- [ ] 集成 OpenTelemetry SDK
- [ ] 部署 Jaeger/Tempo
- [ ] 在关键路径添加 Span (API、任务链、外部调用)
- [ ] 配置 TraceID 注入 (HTTP响应头、日志关联)

**验收标准**:
- ✅ 每个 API 请求有唯一 TraceID
- ✅ Jaeger 可查看完整调用链
- ✅ 日志可通过 TraceID 关联查询

#### Week 5: 高级指标
- [ ] 添加 RED 指标 (Rate/Errors/Duration)
- [ ] 添加 USE 指标 (CPU/Memory/Disk)
- [ ] 添加业务指标 (用户活跃度、存储趋势)
- [ ] 创建 SLO/SLI 仪表盘

**验收标准**:
- ✅ Grafana 显示完整 RED/USE 指标
- ✅ 可查询 P50/P95/P99 延迟
- ✅ 可视化 SLO 达成情况

#### Week 6-7: 告警优化
- [ ] 优化告警规则 (避免告警疲劳)
- [ ] 配置告警路由和分级
- [ ] 实现 On-call 值班表
- [ ] 编写告警响应手册 (Runbook)

**验收标准**:
- ✅ 告警准确率 > 95%
- ✅ P0 告警响应时间 < 5分钟
- ✅ 每个告警有对应 Runbook

---

### 🎯 第三阶段: 智能运维 (2-3周)

**目标**: 从被动告警到主动预测

**任务清单**:

#### Week 8: 异常检测
- [ ] 集成机器学习异常检测
- [ ] 配置智能告警 (动态阈值)
- [ ] 实现容量预测

#### Week 9: 自动化恢复
- [ ] 实现自动重启 (健康检查失败)
- [ ] 实现自动扩容 (队列积压)
- [ ] 实现自动降级 (非核心功能)

#### Week 10: 运维仪表盘
- [ ] 创建综合运维大屏
- [ ] 集成日志/指标/追踪
- [ ] 实现根因分析建议

---

## 最佳实践建议

### 📝 日志最佳实践

#### 1. **结构化字段**

```go
// ❌ 不好: 非结构化日志
logger.Info("User john_doe uploaded video abc123 of size 1024MB")

// ✅ 好: 结构化日志
logger.Infow("Video uploaded",
    zap.String("user_id", "john_doe"),
    zap.String("video_id", "abc123"),
    zap.Int64("size_mb", 1024),
    zap.String("status", "success"),
)
```

#### 2. **统一 TraceID**

```go
// ✅ 中间件注入 TraceID
func TracingMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        traceID := uuid.New().String()
        c.Set("trace_id", traceID)
        c.Writer.Header().Set("X-Trace-ID", traceID)
        c.Next()
    }
}

// ✅ 日志记录器自动使用
logger.Infow("Processing request",
    zap.String("trace_id", c.GetString("trace_id")),
    ...
)
```

#### 3. **错误日志带堆栈**

```go
// ✅ 记录完整错误堆栈
if err != nil {
    logger.Errorw("Failed to download video",
        zap.String("video_id", videoID),
        zap.Error(err),  // 自动记录堆栈
        zap.String("stack", string(debug.Stack())),
    )
}
```

---

### 📊 指标最佳实践

#### 1. **使用 Histogram 而非 Summary**

```go
// ❌ 不推荐: Summary (无法聚合)
taskDuration := prometheus.NewSummaryVec(
    prometheus.SummaryOpts{
        Name: "task_duration_seconds",
    },
    []string{"step_name"},
)

// ✅ 推荐: Histogram (支持分位数计算)
taskDuration := prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "task_duration_seconds",
        Buckets: prometheus.ExponentialBuckets(1, 2, 10),
    },
    []string{"step_name"},
)
```

#### 2. **标签基数控制**

```go
// ❌ 不好: 高基数标签 (video_id 可能成千上万)
taskTotal.WithLabelValues(videoID, "success").Inc()

// ✅ 好: 低基数标签
taskTotal.WithLabelValues("download_video", "success").Inc()
```

---

### 🔍 追踪最佳实践

#### 1. **Span 命名规范**

```go
// ❌ 不好: 含糊的名称
span := tracer.Start(ctx, "process")

// ✅ 好: 描述性名称
span := tracer.Start(ctx, "DownloadVideo")
span := tracer.Start(ctx, "BilibiliAPI.UploadVideo")
```

#### 2. **添加关键属性**

```go
span.SetAttributes(
    attribute.String("user_id", userID),
    attribute.String("video_id", videoID),
    attribute.Int64("file_size_bytes", size),
    attribute.String("bili_account", accountName),
)
```

---

## 成本与收益分析

### 💰 成本估算

#### 基础设施成本 (月度)

| 组件 | 规格 | 单价 | 数量 | 小计 |
|------|------|------|------|------|
| **Grafana** | 4核8G | ¥200/月 | 1台 | ¥200 |
| **Prometheus** | 2核4G | ¥100/月 | 1台 | ¥100 |
| **Loki** | 2核4G | ¥100/月 | 1台 | ¥100 |
| **Jaeger/Tempo** | 2核4G | ¥100/月 | 1台 | ¥100 |
| **存储** | 500GB SSD | ¥300/月 | 1套 | ¥300 |
| **合计** | - | - | - | **¥800/月** |

> **年度成本**: ¥9,600 (相比 SCALABILITY_AND_HA.md 的 ¥1,370/月 低 40%)

#### 开发成本

| 任务 | 工时 | 人天 | 单价 | 小计 |
|------|------|------|------|------|
| 基础监控搭建 | 5天 | 5 | ¥1500/天 | ¥7,500 |
| 指标定义开发 | 5天 | 5 | ¥1500/天 | ¥7,500 |
| 追踪集成 | 10天 | 10 | ¥1500/天 | ¥15,000 |
| 告警配置 | 3天 | 3 | ¥1000/天 | ¥3,000 |
| 文档编写 | 2天 | 2 | ¥1000/天 | ¥2,000 |
| **合计** | - | - | - | **¥35,000** |

> **一次性投入**: ¥35,000 (相比分布式改造的 ¥49,000 低 30%)

---

### 📈 收益分析

#### 效率提升

| 场景 | 改造前 | 改造后 | 提升 |
|------|--------|--------|------|
| **故障定位** | 30-60分钟 (grep日志) | <5分钟 (Grafana查询) | **10倍** |
| **问题发现** | 用户反馈 (被动) | 告警通知 (主动) | **提前** |
| **性能分析** | 猜测瓶颈 | 可视化调用链 | **精准** |
| **容量规划** | 凭经验 | 基于指标预测 | **科学** |

#### MTTR (平均修复时间) 对比

```
改造前 MTTR: 45分钟
├─ 发现问题: 20分钟 (用户反馈)
├─ 定位根因: 20分钟 (grep日志)
└─ 修复验证: 5分钟

改造后 MTTR: 8分钟
├─ 发现问题: 1分钟 (告警)
├─ 定位根因: 5分钟 (Grafana+Traces)
└─ 修复验证: 2分钟

MTTR 降低: 82% 🔥
```

---

### 💡 ROI 分析

**场景1: 减少宕机损失**
- 假设: 宕机1小时损失 ¥10,000 (用户流失+声誉)
- 改造前: 月均宕机4小时 (累计 ¥40,000)
- 改造后: 月均宕机0.5小时 (累计 ¥5,000)
- **节省**: ¥35,000/月 > ¥35,000 一次性投入 → **首月回本**

**场景2: 提升开发效率**
- 假设: 每个故障排查耗时减少 30分钟
- 团队: 5人，每人每周排查1次故障
- 节省: 5人 × 4次/月 × 0.5小时 = 10小时/月
- 按 ¥500/小时计算: **节省 ¥5,000/月**

**投资回报期**: **约2个月**

---

## 总结与建议

### ✅ 现有优势

1. **结构化日志**: Zap 生态完善
2. **日志轮转**: Lumberjack 自动管理
3. **上下文日志**: ContextLogger 支持用户追踪
4. **任务状态持久化**: TaskStep 表记录执行历史

### ⚠️ 主要差距

1. **无集中式日志管理**: 多服务器无法汇总
2. **无实时指标**: 依赖数据库查询统计
3. **无链路追踪**: 无法可视化调用链
4. **无主动告警**: 被动响应用户反馈

### 🎯 实施建议

**对个人开发者/小团队**:
1. ✅ **优先实施第一阶段** (日志+基础指标)
2. ✅ **使用 Docker Compose 快速部署** Loki+Prometheus+Grafana
3. ✅ **配置关键告警** (应用宕机、高错误率)
4. ⚠️ **暂缓分布式追踪** (除非服务拆分)

**对企业/大规模应用**:
1. ✅ **完整三阶段实施** (日志+指标+追踪)
2. ✅ **使用托管服务** (云 Loki/CloudWatch)
3. ✅ **建立 On-call 机制** (7×24值班)
4. ✅ **持续优化告警** (避免告警疲劳)

---

## 📚 参考资源

**技术文档**:
- [Grafana Loki 文档](https://grafana.com/docs/loki/latest/)
- [Prometheus 最佳实践](https://prometheus.io/docs/practices/)
- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/instrumentation/go/)
- [Jaeger 文档](https://www.jaegertracing.io/docs/)

**架构案例**:
- [Google SRE Book](https://sre.google/sre-book/table-of-contents/)
- [Observability Maturity Model](https://www.cncf.io/blog/2023/04/27/observability-maturity-model/)

**告警设计**:
- [Prometheus Alerting Guide](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/)
- [AlertManager Configuration](https://prometheus.io/docs/alerting/latest/configuration/)

---

**文档维护**: 请随着可观测性演进及时更新本文档
**反馈渠道**: 提交 Issue 或 PR 到 GitHub 仓库

---

*最后更新: 2025-12-29*
