# 项目根目录文件说明

## 📂 运行时文件（保留在根目录）

以下文件会在运行时生成或需要，应保留在项目根目录：

### 二进制文件

- **ytb2bili.exe** - 主程序编译产物（`make build` 生成）
- **yt-dlp.exe** - 视频下载工具
  - **推荐位置**: 项目根目录（与 ytb2bili.exe 同目录）
  - **查找顺序**:
    1. 项目根目录（默认）
    2. `config.toml` 中 `yt_dlp_path` 指定的路径
    3. 系统标准路径（如 `C:\Program Files\yt-dlp\`）
    4. 系统环境变量 PATH
  - **下载**: 程序启动时会自动检测并下载最新版

> 这些文件由 `make build` 生成，不应提交到 git，但需要在根目录运行

### 配置文件

- **config.toml** - 主配置文件（从 config.toml.example 复制）
- **cookies.txt** - YouTube cookies（用于访问受限制视频）
- **bili_up.db** - SQLite 数据库文件（首次运行自动创建）

> 这些文件包含个人配置和运行时数据，不应提交到 git

### 日志文件

- **server.log** - 服务器运行日志
- **upload_test.log** - 上传测试日志

> 日志文件会在运行时自动生成，不应提交到 git

## 🔧 首次运行准备

### 1. 创建配置文件

```bash
cp config.toml.example config.toml
# 编辑 config.toml 填入你的配置
```

### 2. 构建 Go 程序

```bash
make build
# 或
go build -o ytb2bili.exe .
```

### 3. 运行程序

```bash
./ytb2bili.exe
# 或
make run
```

## 📁 目录结构说明

```
ytb2bili/
├── ytb2bili.exe          # 编译产物（运行时需要）
├── yt-dlp.exe            # 视频下载工具（运行时需要）
├── config.toml           # 配置文件（运行时需要）
├── cookies.txt           # Cookies（运行时需要）
├── bili_up.db            # 数据库（运行时生成）
├── server.log            # 日志（运行时生成）
│
├── cmd/                  # 命令行工具源码
├── internal/             # Go 源代码
├── web/                  # Next.js 前端
├── docs/                 # 文档
├── scripts/              # 构建脚本
├── docker/               # Docker 配置
└── data/                 # 运行时数据目录
    ├── media/            # 下载的视频
    ├── server.log        # 日志（可选移动到这里）
    └── upload_test.log   # 测试日志
```

## ⚙️ Git 配置

这些运行时文件已在 `.gitignore` 中配置，不会提交到 git 仓库：

```gitignore
# 运行时文件（保留在根目录，但不提交）
*.exe
cookies.txt
cookies_backup.txt
*.db
*.log
config.toml
!config.toml.example
```

## 🚀 开发建议

### 提交代码前

```bash
# 清理运行时文件（可选）
git clean -fdX

# 查看将被忽略的文件
git status --ignored
```

### 分发程序时

打包时排除以下文件/目录：
- `.git/`
- `node_modules/`
- `.next/`
- `internal/web/bili-up-web/`
- `*.log`
- `data/media/`（用户的下载内容）

---

*最后更新: 2024-12-29*
