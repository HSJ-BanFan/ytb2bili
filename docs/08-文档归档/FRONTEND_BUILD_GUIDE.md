# 前端构建架构说明

## 📐 架构原理

### 文件流程
```
web/src/ (源码)
    ↓ npm run build
internal/web/bili-up-web/ (构建输出)
    ↓ go build
ytb2bili.exe (嵌入的静态文件)
    ↓ 运行时
http://localhost:8096/ (提供前端页面)
```

### 关键配置

**Next.js 配置** (`web/next.config.js:13`)
```javascript
distDir: '../internal/web/bili-up-web'
```
→ 构建产物直接输出到 Go embed 读取的目录，**不需要手动复制**

**Go Embed** (`internal/web/static.go:11`)
```go
//go:embed bili-up-web/*
var staticFiles embed.FS
```
→ 将前端静态文件编译进二进制，**运行时从内存读取**

## 🚀 快速开始

### 方式 1：完整构建（推荐初次使用）
```powershell
.\build.ps1
```
→ 构建前端 + 后端

### 方式 2：修改了前端代码
```powershell
# 步骤 1: 重新构建前端
.\rebuild-frontend.ps1

# 步骤 2: 重新构建 Go 程序
.\quick-rebuild.ps1
```

### 方式 3：只修改了 Go 代码
```powershell
.\quick-rebuild.ps1
```
→ 仅构建后端（使用已有的前端文件）

## 📝 常见场景

| 场景 | 命令 |
|------|------|
| 完整构建 | `.\build.ps1` |
| 修改了 `web/src/*` | `.\rebuild-frontend.ps1` + `.\quick-rebuild.ps1` |
| 修改了 `internal/*` | `.\quick-rebuild.ps1` |
| 同时改了前后端 | `.\build.ps1` |

## ⚠️ 常见错误

### 错误 1：修改前端后没生效
**原因**：只运行了 `go build`，没有重新构建前端

**解决**：
```powershell
.\rebuild-frontend.ps1
.\quick-rebuild.ps1
```

### 错误 2：前端显示旧版本
**原因**：浏览器缓存

**解决**：
- 按 `Ctrl + F5` 强制刷新
- 或打开开发者工具 → 勾选 "Disable cache"

### 错误 3：找不到构建产物
**原因**：Next.js 构建失败

**解决**：检查 `web/next.config.js` 中的 `distDir` 配置是否为 `'../internal/web/bili-up-web'`

## 🔍 验证构建

### 检查前端是否正确构建
```powershell
Test-Path "internal\web\bili-up-web\index.html"
```
应该返回 `True`

### 检查 Go 是否正确嵌入
```powershell
.\ytb2bili.exe
# 访问 http://localhost:8096/
# 应该能看到前端页面
```

## 📂 目录结构

```
ytb2bili/
├── web/                        # 前端源码
│   ├── src/                    # React 组件
│   ├── next.config.js          # Next.js 配置 ⭐
│   └── package.json
│
├── internal/
│   └── web/
│       ├── static.go           # Go embed 配置 ⭐
│       └── bili-up-web/        # 构建输出（不要手动修改）⭐
│           ├── index.html
│           ├── _next/
│           └── ...
│
├── build.ps1                   # 完整构建脚本
├── quick-rebuild.ps1           # 快速重建（仅 Go）
├── rebuild-frontend.ps1        # 重建前端
└── ytb2bili.exe                # 最终二进制文件
```

## 💡 开发技巧

### 1. 使用 watch 模式（前端）
```powershell
cd web
npm run dev
```
→ 前端运行在 `http://localhost:3000`，热更新即时生效

### 2. 使用 Make（需要 Git Bash）
```bash
make build              # 完整构建
make quick-build        # 快速构建（仅 Go）
make build-web          # 仅构建前端
```

### 3. 跳过前端构建（调试 Go 代码时）
```powershell
.\build.ps1 -SkipFrontend
```

## 🎯 为什么需要这样设计？

### 好处
- ✅ 单一二进制文件，部署方便
- ✅ 前后端统一端口，避免 CORS
- ✅ 静态文件嵌入内存，读取速度快

### 代价
- ❌ 修改前端后必须重新构建 Go 程序
- ❌ 开发时需要等待构建完成

### 解决方案
开发时使用 `npm run dev` 独立运行前端，只有发布时才构建嵌入版本。
