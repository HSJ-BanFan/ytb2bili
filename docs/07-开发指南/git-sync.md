# Git 同步指南

## 📚 仓库信息

- **原始仓库（上游）**: https://github.com/difyz9/ytb2bili.git
- **你的 Fork 仓库**: https://github.com/Meilunova/ytb2bili.git

## 🔧 远程仓库配置

当前已配置的远程仓库：

```bash
# 查看远程仓库
git remote -v

# 输出：
# origin  https://github.com/difyz9/ytb2bili.git (fetch)
# origin  https://github.com/difyz9/ytb2bili.git (push)
# myfork  https://github.com/Meilunova/ytb2bili.git (fetch)
# myfork  https://github.com/Meilunova/ytb2bili.git (push)
```

## 📝 二次开发内容总结

### 1. 核心功能优化

#### ✅ 标题和描述长度限制

- **文件**: `internal/chain_task/handlers/upload_to_bilibili.go`
- **功能**:
  - 标题自动截断（80 字符限制）
  - 描述自动截断（2000 字符限制）
  - AI 生成描述控制在 600-800 字
  - 详细的长度日志输出

#### ✅ 描述内容过滤

- **文件**: `internal/chain_task/handlers/upload_to_bilibili.go`
- **功能**:
  - 过滤 YouTube 默认描述（如"YouTube 自动上传的视频"）
  - 智能判断描述有效性
  - 无有效描述时仅显示原视频链接

#### ✅ 原视频链接自动添加

- **文件**: `internal/chain_task/handlers/upload_to_bilibili.go`
- **功能**:
  - 在描述末尾自动添加原视频链接
  - 包含分隔线和转载声明
  - 智能长度控制

#### ✅ 标题和描述来源配置

- **文件**: `internal/core/types/app_config.go`, `config.toml.example`
- **功能**:
  - `use_original_title`: 选择使用原标题或 AI 标题
  - `use_original_desc`: 选择使用原描述或 AI 描述
  - `custom_desc_template`: 自定义描述模板

#### ✅ 代理配置优化

- **文件**: `internal/chain_task/handlers/down_load_video.go`
- **功能**:
  - 代理连接失败自动回退
  - 支持从 Chrome 读取 cookies
  - 支持 cookies.txt 文件

#### ✅ 删除视频功能

- **文件**:
  - `internal/handler/video_handler.go`
  - `web/src/components/video/VideoActions.tsx`
  - `web/src/components/video/VideoList.tsx`
- **功能**:
  - 前端删除按钮
  - 后端软删除 API
  - 批量删除支持

### 2. 配置文件优化

#### ✅ BilibiliConfig 新增字段

```toml
[BilibiliConfig]
  copyright = 2                # 转载
  source = "YouTube"           # 来源
  no_reprint = 1               # 禁止转载
  use_original_title = true    # 使用原视频标题
  use_original_desc = false    # 使用AI生成描述
  custom_desc_template = ""    # 自定义描述模板
```

### 3. 文档新增

创建了以下详细文档：

- `BILIBILI_CONFIG_GUIDE.md` - B 站配置详细说明
- `TITLE_DESC_USAGE.md` - 标题和描述使用指南
- `DESCRIPTION_LENGTH_FIX.md` - 长度限制修复说明
- `DESCRIPTION_FILTER_GUIDE.md` - 描述过滤优化说明
- `YOUTUBE_COOKIES_GUIDE.md` - YouTube Cookies 配置指南
- `PROXY_FIX.md` - 代理配置修复说明
- `DELETE_VIDEO_API.md` - 删除视频 API 文档
- `DELETE_FEATURE_SUMMARY.md` - 删除功能总结
- `FRONTEND_DELETE_GUIDE.md` - 前端删除功能指南
- `FINAL_SETUP_SUMMARY.md` - 完整配置总结
- `GEMINI_INTEGRATION_GUIDE.md` - Gemini 集成指南（计划中）
- `GEMINI_QUICK_START.md` - Gemini 快速开始（计划中）

## 🚀 同步到你的 Fork 仓库

### 步骤 1：提交所有更改

```bash
# 添加所有修改的文件
git add .

# 提交更改
git commit -m "feat: 二次开发优化

主要改进：
- 添加标题和描述长度自动截断（B站限制）
- 过滤YouTube默认描述
- 自动添加原视频链接
- 支持标题/描述来源配置
- 优化代理和cookies支持
- 添加删除视频功能
- 完善文档和配置示例

详细说明请查看各个 *_GUIDE.md 文件"
```

### 步骤 2：推送到你的 Fork 仓库

```bash
# 推送到你的 fork 仓库的 main 分支
git push myfork main

# 如果需要强制推送（谨慎使用）
# git push myfork main --force
```

### 步骤 3：创建新分支（推荐）

如果你想保持更清晰的版本管理：

```bash
# 创建并切换到新分支
git checkout -b feature/custom-improvements

# 推送到你的 fork 仓库
git push myfork feature/custom-improvements
```

## 🔄 保持与上游同步

如果原始仓库有更新，你可以这样同步：

```bash
# 1. 获取上游更新
git fetch origin

# 2. 切换到 main 分支
git checkout main

# 3. 合并上游更新
git merge origin/main

# 4. 解决冲突（如果有）
# 手动编辑冲突文件，然后：
git add .
git commit -m "merge: 合并上游更新"

# 5. 推送到你的 fork
git push myfork main
```

## 📋 提交前检查清单

- [ ] 已更新 `.gitignore`，排除不必要的文件
- [ ] 已删除敏感信息（如 `cookies.txt`、`config.toml`）
- [ ] 已测试所有功能正常工作
- [ ] 已添加必要的文档说明
- [ ] 提交信息清晰明确

## ⚠️ 注意事项

### 不要提交的文件

以下文件已在 `.gitignore` 中排除，不会被提交：

- `ytb2bili.exe` - 编译后的可执行文件
- `bili-up-api-server.exe` - 编译后的可执行文件
- `yt-dlp.exe` - YouTube 下载工具
- `cookies.txt` - 浏览器 cookies（包含敏感信息）
- `config.toml` - 配置文件（包含 API 密钥等敏感信息）
- `*.db` - 数据库文件
- `*.backup` - 备份文件
- `test_delete.html` - 测试文件

### 保留的示例文件

以下示例文件会被提交，供其他用户参考：

- `config.toml.example` - 配置文件示例
- 各种 `*_GUIDE.md` - 文档指南

## 🎯 推荐工作流程

### 方案 1：直接推送到 main 分支

适合个人项目，快速同步：

```bash
git add .
git commit -m "feat: 二次开发优化"
git push myfork main
```

### 方案 2：使用功能分支（推荐）

适合团队协作或保持清晰的版本历史：

```bash
# 创建功能分支
git checkout -b feature/improvements

# 提交更改
git add .
git commit -m "feat: 添加长度限制和描述过滤"

# 推送到 fork
git push myfork feature/improvements

# 在 GitHub 上创建 Pull Request（可选）
```

### 方案 3：保持多个分支

```bash
# 主分支跟随上游
git checkout main
git pull origin main
git push myfork main

# 开发分支包含你的改进
git checkout -b dev
git merge main
# 添加你的改进
git push myfork dev
```

## 📖 后续维护

### 定期同步上游

```bash
# 每周或每月执行一次
git fetch origin
git checkout main
git merge origin/main
git push myfork main
```

### 标记重要版本

```bash
# 创建版本标签
git tag -a v1.0-custom -m "第一个自定义版本"
git push myfork v1.0-custom
```

## 💡 最佳实践

1. **提交信息规范**

   - `feat:` 新功能
   - `fix:` 修复 bug
   - `docs:` 文档更新
   - `refactor:` 代码重构
   - `test:` 测试相关

2. **分支命名规范**

   - `feature/功能名` - 新功能
   - `fix/问题名` - bug 修复
   - `docs/文档名` - 文档更新

3. **定期备份**
   - 推送到远程仓库
   - 创建版本标签
   - 导出重要配置

## 🔗 相关链接

- 原始仓库: https://github.com/difyz9/ytb2bili
- 你的 Fork: https://github.com/Meilunova/ytb2bili
- Git 文档: https://git-scm.com/doc

## ❓ 常见问题

### Q: 如何处理冲突？

A: 如果合并时出现冲突：

```bash
# 1. 查看冲突文件
git status

# 2. 手动编辑冲突文件，保留需要的内容

# 3. 标记为已解决
git add <冲突文件>

# 4. 完成合并
git commit
```

### Q: 如何撤销错误的提交？

A: 如果还没推送：

```bash
# 撤销最后一次提交，保留更改
git reset --soft HEAD~1

# 撤销最后一次提交，丢弃更改
git reset --hard HEAD~1
```

### Q: 如何查看提交历史？

A:

```bash
# 查看提交历史
git log --oneline --graph --all

# 查看某个文件的修改历史
git log --follow <文件名>
```

## 🎉 完成！

现在你可以将你的二次开发成果同步到你的 GitHub 仓库了！
