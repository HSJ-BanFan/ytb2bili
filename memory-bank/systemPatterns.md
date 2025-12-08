# σ₂: System Patterns
*v1.0 | Created: 2025-12-07 | Updated: 2025-12-07*
*Π: 🚧INITIALIZING | Ω: 🔍R*

## 🏛️ Architecture Overview

系统采用分层架构，前后端分离设计。后端使用 Go + Gin 框架，前端使用 Next.js 15+。

```
┌─────────────────────────────────────────────────────────────┐
│                         Frontend                             │
│                   (Next.js 15 + React 18)                    │
└─────────────────────────┬───────────────────────────────────┘
                          │ HTTP API
┌─────────────────────────▼───────────────────────────────────┐
│                      HTTP Handler Layer                      │
│                      (Gin Framework)                         │
├─────────────────────────────────────────────────────────────┤
│                      Service Layer                           │
│         (Video Service / Task Step Service / etc.)           │
├─────────────────────────────────────────────────────────────┤
│                   Chain Task Engine                          │
│    (Task Handlers / Upload Scheduler / State Manager)        │
├─────────────────────────────────────────────────────────────┤
│                      Storage Layer                           │
│           (GORM + MySQL/SQLite + COS Storage)                │
└─────────────────────────────────────────────────────────────┘
```

## 🔧 Key Components

### 1. Chain Task Engine (任务链引擎)
- **chain_task_handler.go** - 任务链执行器
- **upload_scheduler.go** - 上传调度器
- **handlers/** - 具体任务处理器

### 2. Core Business Layer (核心业务层)
- **app_server.go** - HTTP 服务器配置
- **models/** - 数据模型
- **services/** - 业务服务层

### 3. External Services (外部服务)
- **Whisper AI** - 语音识别
- **百度翻译/DeepSeek** - 翻译服务
- **Bilibili SDK** - 视频上传
- **腾讯云 COS** - 对象存储

## 🔄 Design Patterns

### 责任链模式 (Chain of Responsibility)
用于视频处理流程，每个步骤独立执行，可单独重试。

### 工厂模式 (Factory Pattern)
用于创建不同的翻译器实例（百度/DeepSeek）。

### 依赖注入 (Dependency Injection)
使用 Uber FX 进行声明式依赖管理。

### 调度器模式 (Scheduler Pattern)
使用 Robfig Cron 进行定时任务调度。

## 📐 Design Decisions

| 决策 | 选择 | 理由 |
|------|------|------|
| Web框架 | Gin | 高性能、社区活跃 |
| ORM | GORM v2 | 功能完整、多数据库支持 |
| 依赖注入 | Uber FX | 声明式、可测试性好 |
| 前端框架 | Next.js 15 | SSG支持、TypeScript原生支持 |
| 定时任务 | Robfig Cron | 精确到秒级调度 |
