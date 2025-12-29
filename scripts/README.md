# 构建脚本

此目录包含项目的构建和部署脚本。

## 📜 脚本列表

### Windows PowerShell 脚本

- `build.ps1` - Windows 主构建脚本
- `fix_and_build.bat` - 修复并构建
- `quick-rebuild.ps1` - 快速重建（仅后端）
- `rebuild.bat` - Windows 批处理重建
- `rebuild-frontend.ps1` - 仅重建前端

### Unix Shell 脚本

- `rebuild.sh` - Linux/macOS 重建脚本

## 🚀 使用方法

### Windows

```powershell
# 完整构建（前端 + 后端)
.\scripts\build.ps1

# 快速重建（仅后端）
.\scripts\quick-rebuild.ps1

# 仅重建前端
.\scripts\rebuild-frontend.ps1
```

### Linux/macOS

```bash
# 重建项目
chmod +x scripts/rebuild.sh
./scripts/rebuild.sh
```

## 📖 也可以使用 Makefile

项目根目录的 `Makefile` 提供了更便捷的命令：

```bash
make build          # 完整构建
make build-api      # 仅后端
make build-web      # 仅前端
make clean          # 清理构建产物
```

## 🔧 脚本说明

| 脚本 | 功能 | 依赖 |
|------|------|------|
| build.ps1 | 完整构建流程 | Node.js, Go |
| quick-rebuild.ps1 | 快速重建 Go | Go |
| rebuild-frontend.ps1 | 前端构建 | Node.js, pnpm |
| rebuild.sh | Unix 完整构建 | Node.js, Go |

---

*最后更新: 2024-12-29*
