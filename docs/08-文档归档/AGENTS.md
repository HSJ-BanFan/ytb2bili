# AGENTS.md

> 本文件为 AI 编程代理提供项目上下文、开发规范和操作指南。

## 📋 项目概览

**YTB2BILI** 是一个 YouTube 到 Bilibili 的自动化视频转载系统，支持：
- 从 YouTube/TikTok 等平台下载视频
- Whisper AI 自动生成字幕
- 百度翻译/DeepSeek AI 翻译字幕
- AI 生成符合 B站规范的标题、描述、标签
- 定时上传视频和字幕到 Bilibili

**技术栈**：Go 1.24+ (Gin + GORM + FX) + Next.js 15 (TypeScript + Tailwind)

---

## 🛠️ 开发环境

```bash
# 开发模式 (热重载)
make dev

# 一键构建 (前端+后端)
make build

# 仅构建后端
make build-api

# 启动服务
./bili-up-api-server
# 或
./ytb2bili.exe
```

**默认端口**: `8096`  
**健康检查**: `GET http://localhost:8096/health`

---

## 📁 项目结构

```
ytb2bili/
├── main.go                 # 应用入口 + FX 依赖注入
├── config.toml             # 配置文件
├── internal/               # 内部业务逻辑
│   ├── chain_task/         # 任务链处理引擎
│   │   ├── handlers/       # 具体任务处理器 (字幕生成、翻译、上传等)
│   │   └── manager/        # 任务链管理
│   ├── core/               # 核心业务层
│   │   ├── models/         # 数据模型 (GORM)
│   │   ├── services/       # 业务服务层
│   │   └── types/          # 类型定义和接口
│   ├── handler/            # HTTP 请求处理器 (Gin handlers)
│   └── storage/            # 存储抽象层
├── pkg/                    # 可重用组件库
│   ├── cos/                # 腾讯云 COS 客户端
│   ├── translator/         # 翻译服务 (百度/DeepSeek)
│   ├── logger/             # 日志组件 (Zap)
│   └── utils/              # 工具函数
└── web/                    # Next.js 前端 (编译后嵌入 Go 二进制)
```

---

## 📝 代码规范

### Go 后端
- 遵循 Go 标准项目布局 (`internal/`, `pkg/`, `cmd/`)
- 使用 Uber FX 进行依赖注入
- 日志使用 `pkg/logger` (基于 Zap)
- 数据库操作使用 GORM v2
- HTTP 处理器放在 `internal/handler/`

### API 响应格式
```json
{
  "code": 200,
  "message": "success",
  "data": { ... }
}
```

### 错误码规范
- `200`: 成功
- `400`: 请求参数错误
- `401`: 未授权
- `500`: 服务器内部错误

### Next.js 前端
- 使用 App Router (`app/` 目录)
- 组件放在 `components/`
- API 调用使用 Axios
- 样式使用 Tailwind CSS

---

## 🧪 测试指南

```bash
# 运行单元测试
make test
# 或
go test -v ./...

# 代码规范检查
make lint

# 代码格式化
make fmt
```

### 手动测试流程
```bash
# 1. 检查服务健康
curl http://localhost:8096/health

# 2. 检查登录状态
curl http://localhost:8096/api/v1/auth/status

# 3. 获取视频列表
curl http://localhost:8096/api/v1/videos
```

---

## 🔄 任务处理流程

视频处理分为 **4步准备阶段** + **定时上传阶段**：

1. **字幕生成** (`generate_subtitles.go`) - Whisper AI
2. **封面下载** (`download_img_handler.go`) - 下载并上传到 COS
3. **字幕翻译** (`translate_subtitle.go`) - 百度翻译/DeepSeek
4. **元数据生成** (`generate_metadata.go`) - AI 生成标题描述标签
5. **视频上传** (`upload_to_bilibili.go`) - 每小时1个
6. **字幕上传** (`upload_subtitle_to_bilibili.go`) - 视频上传后1小时

---

## 📊 数据库

**支持**: MySQL 8.0+ / PostgreSQL 15+ / SQLite

### 核心表
- `tb_videos`: 视频主表
- `task_steps`: 任务步骤表
- `tb_users`: 用户表
- `tb_bilibili_accounts`: B站账号表

### 视频状态码
- `001`: 待处理
- `002`: 处理中
- `200`: 准备上传
- `300`: 视频已上传
- `400`: 完成
- `999`: 失败

---

## 🚀 PR 规范

1. 提交前确保通过 `make lint` 和 `make test`
2. 重要功能变更需更新 `README.md`
3. 新增 API 需在 `README.md` 的 API 文档部分补充说明
4. 数据库变更需提供迁移脚本或在 `pkg/store/migrate.go` 中更新

---

## ⚠️ 注意事项

- **B站上传频率**: 每小时最多上传1个视频，避免被限制
- **翻译 API**: 需要配置百度翻译或 DeepSeek API 密钥
- **云存储**: 封面和视频文件存储在腾讯云 COS
- **yt-dlp**: 需要可访问 YouTube 的网络环境
