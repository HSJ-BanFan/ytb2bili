# σ₃: Technical Context
*v1.0 | Created: 2025-12-07 | Updated: 2025-12-07*
*Π: 🚧INITIALIZING | Ω: 🔍R*

## 🛠️ Technology Stack

### 🖥️ Backend
- **语言**: Go 1.24+
- **Web 框架**: Gin (高性能HTTP框架)
- **ORM**: GORM v2 (支持多数据库)
- **数据库**: MySQL 8.0+ / PostgreSQL 15+ / SQLite
- **文件存储**: 腾讯云 COS
- **依赖注入**: Uber FX
- **定时任务**: Robfig Cron v3
- **日志**: Zap + Lumberjack

### 🌐 Frontend
- **框架**: Next.js 15+ (App Router)
- **语言**: TypeScript 5.x
- **UI 库**: React 18 + Tailwind CSS 3.x
- **图标**: Lucide React
- **HTTP 客户端**: Axios

### 🔗 External Services
- **yt-dlp** - 多平台视频下载
- **Whisper AI** - 语音识别和字幕生成
- **百度翻译 API** - 机器翻译服务
- **DeepSeek AI** - AI翻译和内容生成
- **Bilibili SDK** - 视频上传和账号凭证管理
- **腾讯云 COS** - 对象存储服务

## 📦 Dependencies

### 核心依赖 (go.mod)
```
github.com/gin-gonic/gin
gorm.io/gorm
go.uber.org/fx
github.com/robfig/cron/v3
go.uber.org/zap
```

### 前端依赖 (package.json)
```
next
react
tailwindcss
axios
lucide-react
```

## 🔧 Development Environment

### 必需工具
- Go 1.24+
- Node.js 18+
- Make (构建工具)
- yt-dlp (视频下载)

### 可选工具
- Air (热重载开发)
- Docker (容器化部署)

## 🗄️ Database Schema

### 核心表
- **tb_videos** - 视频主表
- **task_steps** - 任务步骤表
- **tb_users** - 用户表

### 视频状态码
| 状态码 | 含义 |
|--------|------|
| 001 | 待处理 |
| 002 | 处理中 |
| 200 | 准备上传 |
| 300 | 视频已上传 |
| 400 | 完成 |
| 999 | 失败 |

## ⚙️ Configuration

### 主配置文件: config.toml
- 服务监听端口
- 数据库连接信息
- 腾讯云COS配置
- 翻译服务配置（可动态配置）
