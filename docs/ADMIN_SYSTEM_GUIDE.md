# 管理员系统和用户配置实施指南

## 🎯 概述

本文档介绍如何实施以下功能：
1. ✅ 系统保护机制（磁盘检查、超时控制、安全JWT）
2. ✅ 管理员角色系统（admin/user）
3. ✅ 用户个性化配置（AI keys、个人偏好）

---

## 📋 实施步骤

### 步骤1：生成安全的JWT密钥 🔐

**目的**：替换默认的不安全密钥

```bash
# 进入项目目录
cd E:\githubitem\ytb2bili

# 生成JWT密钥
go run cmd/gen-jwt/main.go

# 输出示例：
# 🔑 JWT 密钥生成工具
# ✅ 安全的 JWT 密钥已生成:
# ═══════════════════════════════════════
# 8F3A2B9C7D1E6F4A5B8C2D9E7F1A3B6C9D2E5F8A1B4C7D0E3F6A9C2B5E8F1A4D7E0F3A6B9E2F5A8B1E4D7E0F3A6B9
# ═══════════════════════════════════════
```

**将生成的密钥添加到 config.toml**：

```toml
[auth]
  jwt_secret = "8F3A2B9C7D1E6F4A5B8C2D9E7F1A3B6C9D2E5F8A1B4C7D0E3F6A9C2B5E8F1A4D7E0F3A6B9"
  session_secret = "8F3A2B9C7D1E6F4A5B8C2D9E7F1A3B6C9D2E5F8A1B4C7D0E3F6A9C2B5E8F1A4D7E0F3A6B9"
  jwt_expiration = 24
```

---

### 步骤2：数据库迁移 - 添加角色字段 📦

**创建迁移工具**：

创建文件 `cmd/migrate/main.go`（代码见 `cmd/migrate/main.go`）

**运行迁移**：

```bash
# 方法1：创建临时迁移程序
cat > migrate_now.go << 'EOF'
package main

import (
    "fmt"
    "github.com/difyz9/ytb2bili/cmd/migrate"
    "github.com/difyz9/ytb2bili/internal/core"
)

func main() {
    app := core.NewAppServer()
    if err := app.InitDB(); err != nil {
        panic(err)
    }
    if err := migration.MigrateAll(app.DB); err != nil {
        panic(err)
    }
    fmt.Println("✅ 迁移完成!")
}
EOF

# 运行迁移
go run migrate_now.go

# 清理临时文件
rm migrate_now.go
```

**验证迁移**：

```bash
# 检查数据库
sqlite3 bili_up.db "SELECT id, username, role FROM cw_users LIMIT 5;"

# 预期输出：
# 1|adminuser|admin
# 2|testuser|user
```

**手动设置管理员**（可选）：

```sql
-- 将指定用户设为管理员
UPDATE cw_users SET role = 'admin' WHERE username = 'your_username';

-- 查看所有用户及其角色
SELECT id, username, role, membership_tier FROM cw_users;
```

---

### 步骤3：应用管理员中间件到配置API 🔑

**修改 `internal/handler/config_handler.go`**：

```go
import (
    "github.com/difyz9/ytb2bili/internal/auth"
    // ... 其他导入
)

// RegisterRoutes 注册配置相关路由
func (h *ConfigHandler) RegisterRoutes(server *core.AppServer) {
    api := server.Engine.Group("/api/v1")

    // 添加管理员权限检查中间件
    api.Use(auth.LoadUserRole(server.DB))

    config := api.Group("/config")
    {
        // 公开端点（所有用户可访问）
        config.GET("/ai-services/status", h.getAIServicesStatus)

        // 管理员端点（需要admin角色）
        configAdmin := config.Group("")
        configAdmin.Use(auth.RequireAdmin())
        {
            configAdmin.GET("/deepseek", h.getDeepSeekConfig)
            configAdmin.PUT("/deepseek", h.updateDeepSeekConfig)
            configAdmin.GET("/proxy", h.getProxyConfig)
            configAdmin.PUT("/proxy", h.updateProxyConfig)
            configAdmin.GET("/openai-compatible", h.getOpenAICompatibleConfig)
            configAdmin.PUT("/openai-compatible", h.updateOpenAICompatibleConfig)
            configAdmin.POST("/openai-compatible/test", h.testOpenAICompatibleAPI)
            configAdmin.GET("/openai-compatible/providers", h.getOpenAICompatibleProviders)
            configAdmin.GET("/download", h.getDownloadConfig)
            configAdmin.PUT("/download", h.updateDownloadConfig)
        }
    }
}
```

**说明**：
- `LoadUserRole` 从数据库加载用户角色到 context
- `RequireAdmin` 检查用户角色是否为 admin
- 非 admin 用户访问会返回 403 权限不足

---

### 步骤4：前端权限控制 🎨

**修改 `web/src/app/settings/page.tsx`**：

```tsx
import { useAuth } from '@/hooks/useAuth';

export default function SettingsPage() {
  const { user } = useAuth();
  const isAdmin = user?.role === 'admin'; // 检查是否为管理员

  return (
    <AppLayout userName={user?.name} onLogout={handleLogout}>
      <div className="max-w-4xl mx-auto">
        <h1 className="text-3xl font-bold mb-6">系统设置</h1>

        {/* AI模型设置 - 仅管理员可见 */}
        {isAdmin && (
          <div className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">AI 模型配置</h2>
            <AIModelSettings />
          </div>
        )}

        {/* 通用设置 - 仅管理员可见 */}
        {isAdmin && (
          <div className="mb-8">
            <h2 className="text-2xl font-semibold mb-4">通用设置</h2>
            <GeneralSettings />
          </div>
        )}

        {/* 账号管理 - 所有用户可见 */}
        <div className="mb-8">
          <h2 className="text-2xl font-semibold mb-4">B站账号管理</h2>
          <AccountSettings />
        </div>

        {/* 非管理员提示 */}
        {!isAdmin && (
          <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-6">
            <h3 className="text-lg font-semibold text-yellow-900 mb-2">
              仅管理员可访问系统设置
            </h3>
            <p className="text-sm text-yellow-800">
              如需修改AI配置或系统设置，请联系管理员。
            </p>
          </div>
        )}
      </div>
    </AppLayout>
  );
}
```

---

### 步骤5：测试管理员功能 ✅

1. **测试普通用户访问**：
```bash
# 以普通用户身份登录
# 访问 http://localhost:8096/settings
# 应该看不到 AI 模型配置和通用设置

# 尝试访问配置API（应该返回403）
curl -H "Authorization: Bearer <user_token>" \
  http://localhost:8096/api/v1/config/deepseek
```

2. **测试管理员访问**：
```bash
# 以管理员身份登录
# 访问 http://localhost:8096/settings
# 应该能看到所有配置项

# 访问配置API（应该返回配置）
curl -H "Authorization: Bearer <admin_token>" \
  http://localhost:8096/api/v1/config/deepseek
```

---

## 📊 系统保护机制使用

### 磁盘空间检查

**在任务调度器中使用**：

```go
import "github.com/difyz9/ytb2bili/pkg/utils"

func (h *ChainTaskHandler) SetUp() {
    // 创建系统监控器
    monitor := utils.NewSystemMonitor(1.0) // 至少保留1GB

    // 在任务调度前检查磁盘空间
    if !monitor.IsDiskSpaceEnough(h.App.Config.FileUpDir) {
        h.App.Logger.Warn("⚠️ 磁盘空间不足，暂停任务调度")
        return
    }

    // ... 继续调度任务
}
```

### 超时控制

**在任务中使用超时**：

```go
import "github.com/difyz9/ytb2bili/pkg/utils"

func (t *DownloadVideoTask) Execute(ctx map[string]interface{}) bool {
    // 创建超时控制助手
    timeoutHelper := utils.NewTimeoutHelper(2 * time.Hour)

    // 在超时时间内执行任务
    err := timeoutHelper.RunWithTimeout(func(ctx context.Context) error {
        return t.downloadWithContext(ctx)
    })

    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            ctx["error"] = "任务超时（超过2小时）"
            return false
        }
        ctx["error"] = err.Error()
        return false
    }

    return true
}
```

---

## 🎯 下一步：阶段3（用户个性化配置）

完成阶段2后，您将拥有：

✅ **安全的JWT密钥**
✅ **管理员角色系统**
✅ **配置API权限保护**
✅ **前端权限控制**

**下一阶段**将实施：
- 用户个人AI配置表（`user_ai_configs`）
- 用户个人偏好表（`user_preferences`）
- 任务执行优先级逻辑（用户Key > 系统Key > 降级）

---

## 🐛 常见问题

### Q1: 迁移后普通用户无法登录？

**A**: 检查JWT中间件是否正确加载用户角色：

```go
// 在 main.go 中确保中间件顺序正确
api.Use(authMiddleware.JWTAuth())      // 1. JWT认证
api.Use(auth.LoadUserRole(db))          // 2. 加载用户角色
```

### Q2: 修改角色后需要重新登录吗？

**A**: 是的，角色信息存储在JWT token中，修改后需要重新登录。

### Q3: 如何添加更多管理员？

**方法1：直接修改数据库**
```sql
UPDATE cw_users SET role = 'admin' WHERE username = 'target_user';
```

**方法2：创建管理CLI工具**
```bash
./ytb2bili.exe set-admin --username target_user
```

### Q4: 磁盘空间检查不准确？

**A**: 当前实现是简化版本，生产环境建议使用 `github.com/shirou/gopsutil` 库：

```bash
go get github.com/shirou/gopsutil/disk
```

---

## ✅ 检查清单

- [ ] 生成并配置JWT密钥
- [ ] 运行数据库迁移
- [ ] 验证用户角色已设置
- [ ] 应用管理员中间件到配置API
- [ ] 更新前端权限控制
- [ ] 测试管理员和普通用户访问
- [ ] 测试磁盘空间检查
- [ ] 测试任务超时控制

---

## 📚 相关文档

- `docs/MULTI_USER_LOGGING.md` - 多用户日志系统
- `docs/LOGGING_IMPLEMENTATION_EXAMPLES.md` - 日志实施示例
- `pkg/utils/system_monitor.go` - 系统监控实现
- `pkg/utils/timeout_helper.go` - 超时控制实现
- `internal/auth/admin_middleware.go` - 管理员中间件
