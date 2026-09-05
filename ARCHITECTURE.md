# ytb2bili 架构设计文档

## 目录

1. [项目概述](#项目概述)
2. [服务架构](#服务架构)
3. [数据结构](#数据结构)
4. [数据库设计](#数据库设计)
5. [API路由设计](#api路由设计)
6. [通信机制](#通信机制)
7. [任务处理流程](#任务处理流程)
8. [目录结构](#目录结构)

---

## 项目概述

**ytb2bili** 是一个多用户的 YouTube 转 Bilibili 视频搬运系统，采用 Go + Next.js 架构，支持：

- 多用户隔离（JWT 认证）
- 自动化视频下载、元数据生成、字幕翻译
- 定时上传到 Bilibili（视频+字幕）
- AI 驱动的元数据生成（DeepSeek/Gemini/OpenAI 兼容）
- 任务链式处理与状态跟踪

---

## 服务架构

### 整体架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                         Frontend (Next.js)                       │
│                    /api/v1/* (REST API)                         │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                    ┌───────────▼───────────┐
                    │   Gin Web Server      │
                    │   (AppServer)         │
                    └───────────┬───────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        │                       │                       │
┌───────▼────────┐    ┌────────▼────────┐    ┌────────▼────────┐
│  Auth Layer    │    │  Handler Layer  │    │ Background Jobs │
│ (JWT + Admin)  │    │  (API Routes)   │    │  (Cron + Chain) │
└───────┬────────┘    └────────┬────────┘    └────────┬────────┘
        │                      │                       │
        │              ┌───────▼────────┐      ┌───────▼────────┐
        │              │  Service Layer │      │ Task Scheduler │
        │              │ (Business Log) │      │  (ChainTask)   │
        │              └───────┬────────┘      └────────┬────────┘
        │                      │                       │
        └──────────────────────┼───────────────────────┘
                               │
                    ┌──────────▼───────────┐
                    │   Data Layer        │
                    │ (GORM + File Store) │
                    └─────────────────────┘
```

### 核心组件

| 组件 | 位置 | 职责 |
|------|------|------|
| **AppServer** | `internal/core/app_server.go` | Gin 服务器，中间件配置，路由注册 |
| **ChainTaskHandler** | `internal/chain_task/chain_task_handler.go` | 准备阶段任务调度器（5秒/次） |
| **UploadScheduler** | `internal/chain_task/upload_scheduler.go` | 上传阶段调度器（10秒/次） |
| **TaskCancelManager** | `internal/chain_task/task_cancel_manager.go` | 任务取消管理 |
| **Services** | `internal/core/services/` | 业务逻辑层 |
| **Handlers** | `internal/handler/` | HTTP 请求处理器 |

### 依赖注入 (Uber FX)

```go
// main.go 中的依赖注入图
fx.New(
    // 配置
    fx.Provide(func() *types.AppConfig { return config }),

    // 日志
    fx.Provide(logger.NewLogger),

    // 数据库
    fx.Provide(store.NewDatabase),

    // 服务层
    fx.Provide(services.NewVideoService),
    fx.Provide(services.NewSavedVideoService),
    fx.Provide(services.NewTaskStepService),
    // ...

    // 生命周期
    fx.Invoke(func(lifecycle fx.Lifecycle) {
        // OnStart, OnStop
    }),
)
```

---

## 数据结构

### 核心数据模型

#### 1. SavedVideo (`pkg/store/model/models.go`)

```go
type SavedVideo struct {
    BaseModel
    UserID                uint      // 用户ID（多租户隔离）
    VideoID               string    // YouTube视频ID
    URL                   string
    Title                 string
    Status                string    // 001-999 状态码
    Description           string
    GeneratedTitle        string    // AI生成标题
    GeneratedDesc         string    // AI生成描述
    GeneratedTags         string    // AI生成标签
    BiliBVID              string    // B站视频ID
    BiliAID               int64     // B站AV号
    Subtitles             string    // 字幕数据(JSON)
    VideoSizeMB           float64
    ProcessingCompletedAt *time.Time
    SubtitleScheduledAt   *time.Time
    FilesCleaned          bool
    DownloadProgress      string
}
```

**状态流转：**
```
001 (待处理) → 002 (处理中) → 200 (准备就绪)
                                → 201 (上传视频)
                                → 299 (视频上传失败)
                                → 300 (等待字幕上传)
                                → 301 (上传字幕中)
                                → 399 (字幕上传失败)
                                → 400 (完成)
                                → 998 (已取消)
                                → 999 (失败)
```

#### 2. TaskStep (`pkg/store/model/task_step.go`)

```go
type TaskStep struct {
    BaseModel
    VideoID    string     // 关联视频ID
    StepName   string     // 步骤名称
    StepOrder  int        // 执行顺序
    Status     string     // pending/running/completed/failed/skipped
    StartTime  *time.Time
    EndTime    *time.Time
    Duration   int64      // 毫秒
    ErrorMsg   string
    ResultData string     // JSON
    RetryCount int
    CanRetry   bool
}
```

**任务步骤：**
1. 获取元数据
2. 下载视频
3. 下载字幕
4. 下载封面
5. 翻译字幕
6. AI增强元数据
7. 确认元数据
8. 上传到Bilibili
9. 上传字幕到Bilibili

#### 3. TBUser (`internal/core/models/tb_user.go`)

```go
type TBUser struct {
    Id               string
    Username         string
    Email            string
    PassWord         string  // bcrypt加密
    NickName         string
    Status   string
    Phone    string
    Avatar   string
    Platform string
    Credit   int64
```

#### 4. UserAIConfig (`pkg/store/model/user_config.go`)

```go
type UserAIConfig struct {
    ID        uint
    UserID    uint
    DeepSeekEnabled bool
    DeepSeekAPIKey  string
    GeminiEnabled   bool
    GeminiAPIKey    string
    GeminiAPIKeys   string  // JSON数组
    OpenAIEnabled   bool
    OpenAIProvider  string
    OpenAIAPIKey    string
    OpenAIBaseURL   string
    BaiduEnabled    bool
    BaiduAppID      string
}
```

#### 5. UserBiliAccount (`pkg/store/model/models.go`)

```go
type UserBiliAccount struct {
    BaseModel
    UserID      uint
    Nickname    string
    AccountID   string  // B站账号ID
    AccessToken string
    RefreshToken string
    ExpiresAt   time.Time
    IsDefault   bool
}
```

---

## 数据库设计

### 表结构

| 表名 | 主键 | 索引 | 说明 |
|------|------|------|------|
| `cw_saved_videos` | `id` | `user_id`, `video_id`, `status` | 视频记录表 |
| `cw_task_steps` | `id` | `video_id`, `step_name`, `status` | 任务步骤表 |
| `cw_user_ai_configs` | `id` | `user_id` (UNIQUE) | 用户AI配置 |
| `cw_user_preferences` | `id` | `user_id` (UNIQUE) | 用户偏好设置 |
| `cw_user_bili_accounts` | `id` | `user_id`, `account_id` | B站账号绑定 |
| `tb_user` | `id` | `user_name`, `email` | 用户表 |
| `cw_apps` | `id` | `app_id` | 应用注册 |

### ER 关系

```
┌─────────────┐         ┌─────────────────┐
│   tb_user   │─1────n─│ cw_saved_videos │
│  (用户)     │         │   (视频)        │
└─────────────┘         └────────┬────────┘
                                │
                                │ 1 ── n
                                ▼
                         ┌──────────────┐
                         │cw_task_steps │
                         │ (任务步骤)   │
                         └──────────────┘

┌─────────────┐         ┌────────────────────┐
│   tb_user   │─1────n─│cw_user_bili_accounts│
│  (用户)     │         │  (B站账号绑定)      │
└─────────────┘         └────────────────────┘

┌─────────────┐         ┌──────────────────┐
│   tb_user   │─1────1─│cw_user_ai_configs│
│  (用户)     │         │ (AI配置)          │
└─────────────┘         └──────────────────┘
```

---

## API路由设计

### 路由分组结构

```
/api/v1
├── /auth                  # JWT认证
│   ├── POST   /register
│   ├── POST   /login
│   ├── POST   /logout
│   ├── GET    /status
│   └── POST   /refresh
│
├── /videos               # 视频管理 (JWT认证)
│   ├── GET    /                 # 获取视频列表
│   ├── GET    /:id              # 获取视频详情
│   ├── DELETE /:id              # 删除视频
│   ├── GET    /:id/files        # 获取视频文件
│   ├── POST   /:id/steps/:stepName/retry  # 重试步骤
│   ├── POST   /:id/upload/video           # 手动上传视频
│   ├── POST   /:id/upload/subtitle        # 手动上传字幕
│   ├── POST   /:id/steps/reset-failed     # 重置失败步骤
│   └── POST   /:id/steps/reset-all        # 重置所有步骤
│
├── /bili-accounts        # B站账号管理 (JWT认证)
│   ├── GET    /                 # 获取账号列表
│   ├── POST   /                 # 添加账号
│   ├── DELETE /:id              # 删除账号
│   └── POST   /:id/set-default  # 设置默认账号
│
├── /upload               # 上传接口 (JWT认证)
│   └── POST   /video            # 上传视频文件
│
├── /category            # 分类接口
│   └── GET    /                 # 获取分类列表
│
├── /config              # 配置接口 (部分需要认证)
│   ├── GET    /                 # 获取公开配置
│   ├── GET    /dynamic          # 获取动态配置 (JWT认证)
│   └── POST   /dynamic          # 更新动态配置 (JWT认证)
│
├── /user/config         # 用户配置 (JWT认证)
│   ├── GET    /ai               # 获取AI配置
│   ├── POST   /ai               # 更新AI配置
│   └── GET    /preference       # 获取偏好设置
│
└── /subtitle            # 字幕接口 (JWT认证)
    ├── POST   /upload           # 上传自定义字幕
    └── POST   /generate         # 生成字幕
```

### 认证方式

| 路由组 | 认证方式 | 说明 |
|--------|----------|------|
| `/auth/*` | 无/可选 | 注册/登录接口 |
| `/videos/*` | JWT 必需 | 用户数据隔离 |
| `/bili-accounts/*` | JWT 必需 | 多账号管理 |
| `/config` | 无/可选 | 公开配置无需登录 |
| `/user/config/*` | JWT 必需 | 用户个人配置 |

---

## 通信机制

### 请求流程

```
┌──────────┐     1. HTTP Request      ┌──────────┐
│ Frontend │ ────────────────────────▶│  Gin     │
│(Next.js) │                          │  Router  │
└──────────┘                          └─────┬────┘
                                           │
                                           │ 2. Middleware Chain
                                           │    - CORS
                                           │    - JWTAuth (可选)
                                           │    - Logging
                                           ▼
                                    ┌──────────┐
                                    │ Handler  │
                                    │ Function │
                                    └─────┬────┘
                                          │
                    ┌─────────────────────┼─────────────────────┐
                    │ 3. Service Call     │                     │
                    ▼                     ▼                     ▼
            ┌───────────┐         ┌───────────┐         ┌───────────┐
            │ Service   │         │ Service   │         │ Service   │
            │ Layer     │         │ Layer     │         │ Layer     │
            └─────┬─────┘         └─────┬─────┘         └─────┬─────┘
                  │                     │                     │
                  └─────────────────────┼─────────────────────┘
                                        │ 4. Database Query
                                        ▼
                                ┌───────────────┐
                                │  GORM / Files │
                                └───────┬───────┘
                                        │ 5. Return Data
                                        ▼
┌──────────┐     6. JSON Response    ┌──────────┐
│ Frontend │ ◀────────────────────── │  Gin     │
└──────────┘                          └──────────┘
```

### 中间件链

```go
// internal/core/app_server.go:setupMiddleware
func (s *AppServer) setupMiddleware() {
    s.Engine.Use(corsMiddleware())
    s.Engine.Use(loggerMiddleware())
    s.Engine.Use(recoveryMiddleware())
}

// JWT 认证中间件 (选择性应用)
func (m *AuthMiddleware) JWTAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        claims, err := m.JWTService.ParseToken(token)
        if err != nil {
            c.JSON(401, gin.H{"message": "未登录"})
            c.Abort()
            return
        }
        // 设置 userID 到 context
        c.Set("userID", claims.UserID)
        c.Next()
    }
}
```

### 响应格式

```json
// 成功响应
{
    "code": 200,
    "message": "success",
    "data": {
        "videos": [...],
        "total": 100,
        "page": 1,
        "limit": 10
    }
}

// 错误响应
{
    "code": 401,
    "message": "未登录"
}
```

---

## 任务处理流程

### 两阶段架构

```
┌─────────────────────────────────────────────────────────────────┐
│                      Phase 1: 准备阶段                            │
│                  (ChainTaskHandler - 5秒/次)                     │
├─────────────────────────────────────────────────────────────────┤
│ 1. 获取元数据   ─────┐                                           │
│ 2. 下载视频     ─────┼───▶ 并行执行 (max 10 workers)              │
│ 3. 下载字幕     ─────┤    - 下载任务: max 3 并发                  │
│ 4. 下载封面     ─────┤    - 全局并发池控制                         │
│ 5. 翻译字幕     ─────┘                                           │
│ 6. AI增强元数据                                                 │
│ 7. 确认元数据                                                   │
│                                                                │
│ Status: 001 → 002 → 200 (准备就绪)                              │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Phase 2: 上传阶段                            │
│                 (UploadScheduler - 10秒/次)                      │
├─────────────────────────────────────────────────────────────────┤
│ 1. 上传视频 (immediate/delayed 模式)                             │
│    - immediate: 立即上传                                         │
│    - delayed: 延迟 N 分钟后上传                                  │
│                                                                │
│ 2. 上传字幕 (视频完成后)                                         │
│    - 默认延迟: 10分钟                                            │
│    - 智能延迟: 根据视频大小计算                                   │
│    - 失败重试: 指数退避 (10/20/40分钟)                           │
│                                                                │
│ Status: 200 → 201 → 300 → 301 → 400 (完成)                      │
└─────────────────────────────────────────────────────────────────┘
```

### 任务链执行

```go
// internal/chain_task/chain_task_handler.go:RunTaskChain
chain := manager.NewTaskChain().
    SetLogger(logger).
    SetVideoID(videoID).
    SetContext(ctx)

// 添加任务（按依赖顺序）
chain.AddTask(fetchMetadataTask)     // 无依赖
chain.AddTask(downloadTask)          // 无依赖
chain.AddTask(subtitleTask)          // 无依赖
chain.AddTask(coverTask)             // 依赖: 获取元数据
chain.AddTask(translateTask)         // 依赖: 下载字幕
chain.AddTask(metadataTask)          // 依赖: 下载视频 + 翻译字幕
chain.AddTask(confirmMetadataTask)   // 依赖: AI增强元数据

// 执行任务链
result := chain.Run(true) // true = 并行执行无依赖任务
```

### 任务取消机制

```go
// internal/chain_task/task_cancel_manager.go
type TaskCancelManager struct {
    cancelFuncs sync.Map // map[uint]context.CancelFunc
}

// 注册任务取消函数
ctx := cancelManager.Register(videoID)
defer cancelManager.Deregister(videoID)

// 在任务中检查取消信号
select {
case <-ctx.Done():
    return false // 任务被取消
default:
    // 继续执行
}
```

### 并发控制

```go
// 全局并发池
workerPool := make(chan struct{}, 10) // 最多10个并发任务

// 下载专用池
downloadWorkerPool := make(chan struct{}, 3) // 最多3个并发下载
```

---

## 目录结构

```
ytb2bili/
├── cmd/                    # 命令行工具
│   └── gen-jwt/           # JWT生成工具
│
├── internal/               # 私有代码
│   ├── auth/              # JWT认证
│   │   ├── jwt.go
│   │   ├── middleware.go
│   │   └── handler.go
│   │
│   ├── chain_task/        # 任务链系统
│   │   ├── chain_task_handler.go      # 准备阶段调度
│   │   ├── upload_scheduler.go        # 上传阶段调度
│   │   ├── task_cancel_manager.go     # 任务取消
│   │   ├── handlers/                 # 任务处理器
│   │   │   ├── fetch_metadata.go
│   │   │   ├── down_load_video.go
│   │   │   ├── generate_subtitles.go
│   │   │   ├── translate_subtitle.go
│   │   │   ├── generate_metadata.go
│   │   │   ├── upload_to_bilibili.go
│   │   │   └── ...
│   │   └── manager/          # 任务链管理
│   │       ├── chain.go
│   │       ├── state.go
│   │       └── progress.go
│   │
│   ├── core/               # 核心组件
│   │   ├── app_server.go   # Gin服务器
│   │   ├── models/         # 数据模型
│   │   ├── services/       # 业务逻辑
│   │   └── types/          # 类型定义
│   │
│   ├── handler/            # HTTP处理器
│   │   ├── auth_handler.go
│   │   ├── video_handler.go
│   │   ├── bili_account_handler.go
│   │   ├── upload_handler.go
│   │   ├── config_handler.go
│   │   ├── user_config_handler.go
│   │   ├── subtitle_handler.go
│   │   └── ...
│   │
│   │
│   ├── logger/             # 日志系统
│   ├── migration/          # 数据库迁移
│   └── web/                # 嵌入的前端资源
│
├── pkg/                    # 公共代码
│   ├── analytics/          # 分析统计
│   ├── cos/                # 腾讯云COS
│   ├── logger/             # 日志工具
│   ├── prompts/            # AI提示词
│   ├── services/           # 公共服务
│   ├── store/              # 数据存储
│   │   ├── model/          # 数据模型
│   │   └── database.go
│   ├── translator/         # 翻译服务
│   └── utils/              # 工具函数
│
├── web/                    # Next.js前端
│   ├── src/
│   │   ├── app/           # App Router
│   │   ├── components/    # React组件
│   │   ├── lib/           # 工具库
│   │   │   └── authFetch.ts  # 带JWT的fetch
│   │   └── pages/         # 页面
│   └── package.json
│
├── config.toml             # 主配置文件
├── main.go                 # 应用入口
├── go.mod
├── go.sum
├── Makefile               # 构建脚本
└── README.md
```

---

## 总结

**ytb2bili** 是一个功能完善的多用户视频搬运系统，具有以下特点：

1. **多用户隔离**: JWT 认证 + 用户级数据隔离
2. **两阶段处理**: 准备阶段（并行）+ 上传阶段（定时）
3. **任务链式执行**: 支持依赖检查、失败重试、取消机制
4. **AI 集成**: 支持多家 AI 服务（DeepSeek/Gemini/OpenAI）
5. **认证与角色**: 普通 JWT 认证，保留 `admin/user` 角色
6. **状态持久化**: 所有任务状态可恢复、可重试

技术栈：
- **后端**: Go + Gin + GORM + Uber FX
- **前端**: Next.js 14 + TypeScript + Tailwind CSS
- **数据库**: SQLite/MySQL/PostgreSQL
- **任务调度**: robfig/cron
- **文件存储**: 腾讯云 COS (可选)
