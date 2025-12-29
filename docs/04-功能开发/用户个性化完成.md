# 用户个性化配置系统实施完成

## 📋 实施概览

本文档记录了用户个性化配置系统的完整实施，包括系统保护、角色权限、用户配置和配置优先级逻辑。

---

## ✅ 已完成的功能

### 1. 系统保护机制（Phase 1）

#### 1.1 磁盘空间检查
**文件**: `pkg/utils/system_monitor.go`

```go
monitor := utils.NewSystemMonitor(1.0) // 保留1GB
if !monitor.IsDiskSpaceEnough("/data/media") {
    log.Warn("磁盘空间不足，暂停任务")
}
```

**功能**:
- 检查指定目录的可用磁盘空间
- 防止磁盘满导致系统崩溃
- 支持自定义最小保留空间（GB）

#### 1.2 超时控制
**文件**: `pkg/utils/timeout_helper.go`

```go
timeoutHelper := utils.NewTimeoutHelper(2 * time.Hour)
err := timeoutHelper.RunWithTimeout(func(ctx context.Context) error {
    return downloadVideo(ctx)
})
```

**功能**:
- 为长时间运行的任务设置超时
- 防止任务永久挂起
- 预定义各阶段任务的默认超时时间

#### 1.3 安全JWT密钥生成
**文件**: `cmd/gen-jwt/main.go`

**使用方法**:
```bash
go run cmd/gen-jwt/main.go
```

**输出**: 512位安全随机密钥，用于生产环境的JWT签名

---

### 2. 角色权限系统（Phase 2）

#### 2.1 用户模型更新
**文件**: `pkg/store/model/models.go`

**新增字段**:
```go
Role string `gorm:"size:20;default:user;index" json:"role"` // 用户角色: admin/user
```

**关键设计决策**:
- **Role** (admin/user): 控制**权限**（能否访问系统设置）
- **MembershipTier** (free/basic/pro): 控制**配额**（任务数量、并发数）
- 两者**解耦**，独立管理

#### 2.2 认证中间件更新
**文件**: `internal/auth/middleware.go`

**更新内容**:
- `JWTAuth()` 中间件现在自动从数据库加载用户角色
- 角色信息存储在 `gin.Context` 中，key为 `ContextKeyUserRole`
- `OptionalJWTAuth()` 同样支持角色加载

**代码示例**:
```go
// 自动加载角色（无需手动调用）
c.Set(ContextKeyUserRole, userRole)

// 在handler中获取角色
role := auth.GetCurrentUserRole(c) // "admin" 或 "user"
if auth.IsAdmin(c) {
    // 管理员逻辑
}
```

#### 2.3 管理员中间件
**文件**: `internal/auth/admin_middleware.go`

**核心函数**:
- `RequireAdmin()`: 要求管理员权限，否则返回403
- `GetCurrentUserRole(c)`: 获取当前用户角色
- `IsAdmin(c)`: 判断是否为管理员

**使用示例**:
```go
adminGroup := router.Group("")
adminGroup.Use(auth.RequireAdmin())
{
    adminGroup.PUT("/config/deepseek", updateConfig)
}
```

#### 2.4 自动迁移
**文件**: `pkg/store/migrate.go`

**功能**:
- 启动时自动运行数据库迁移
- 检查 `role` 字段是否存在，不存在则添加
- 自动将第一个用户设为管理员
- 创建 `cw_user_ai_configs` 和 `cw_user_preferences` 表

**迁移流程**:
```go
// main.go 中自动执行
fx.Invoke(func(db *gorm.DB, logger *zap.SugaredLogger) error {
    logger.Info("Running database migrations...")
    return store.MigrateDatabase(db)
})
```

---

### 3. 用户个性化配置（Phase 3）

#### 3.1 用户AI配置模型
**文件**: `pkg/store/model/user_config.go`

**UserAIConfig 表结构**:
```go
type UserAIConfig struct {
    ID        uint      `gorm:"primarykey"`
    UserID    uint      `gorm:"uniqueIndex;not null"`

    // DeepSeek 配置
    DeepSeekEnabled bool   `json:"deepseek_enabled"`
    DeepSeekAPIKey  string `json:"deepseek_api_key,omitempty"`
    DeepSeekModel   string `json:"deepseek_model"`

    // Gemini 配置
    GeminiEnabled   bool    `json:"gemini_enabled"`
    GeminiAPIKey    string  `json:"gemini_api_key,omitempty"`
    GeminiAPIKeys   string  `json:"gemini_api_keys,omitempty"` // JSON数组
    GeminiModel     string  `json:"gemini_model"`
    GeminiTimeout   int     `json:"gemini_timeout"`
    GeminiMaxTokens int     `json:"gemini_max_tokens"`

    // OpenAI 兼容配置
    OpenAIEnabled   bool    `json:"openai_enabled"`
    OpenAIProvider  string  `json:"openai_provider,omitempty"`
    OpenAIAPIKey    string  `json:"openai_api_key,omitempty"`
    OpenAIBaseURL   string  `json:"openai_base_url,omitempty"`
    OpenAIModel     string  `json:"openai_model"`
    OpenAITimeout   int     `json:"openai_timeout"`
    OpenAIMaxTokens int     `json:"openai_max_tokens"`

    // Baidu 配置
    BaiduEnabled bool   `json:"baidu_enabled"`
    BaiduAppID  string `json:"baidu_app_id,omitempty"`
    BaiduSecret string `json:"baidu_secret,omitempty"`
}
```

#### 3.2 用户偏好模型
**UserPreference 表结构**:
```go
type UserPreference struct {
    ID        uint      `gorm:"primarykey"`
    UserID    uint      `gorm:"uniqueIndex;not null"`

    // 通知设置
    EmailNotificationsEnabled bool   `json:"email_notifications_enabled"`
    NotificationEmail        string `json:"notification_email,omitempty"`

    // 任务默认设置
    DefaultAutoUpload    bool `json:"default_auto_upload"`
    DefaultUploadDelay   int  `json:"default_upload_delay"`
    DefaultSubtitleDelay int  `json:"default_subtitle_delay"`
    DefaultCopyright     int  `json:"default_copyright"`
    DefaultSource        string `json:"default_source"`
    DefaultTid           int  `json:"default_tid"`

    // 界面设置
    Theme        string `json:"theme"`         // light/dark
    Language     string `json:"language"`       // zh/en
    ItemsPerPage int    `json:"items_per_page"`
    ShowAdvanced bool   `json:"show_advanced"`

    // 隐私设置
    EnableAnalytics bool `json:"enable_analytics"`
}
```

#### 3.3 用户配置服务
**文件**: `internal/core/services/user_config_service.go`

**核心方法**:
```go
// 获取或创建用户AI配置（如果不存在则使用默认值）
func (s *UserConfigService) GetOrCreateUserAIConfig(userID uint) (*model.UserAIConfig, error)

// 更新用户AI配置
func (s *UserConfigService) UpdateUserAIConfig(userID uint, config *model.UserAIConfig) error

// 获取或创建用户偏好
func (s *UserConfigService) GetOrCreateUserPreference(userID uint) (*model.UserPreference, error)

// 更新用户偏好
func (s *UserConfigService) UpdateUserPreference(userID uint, pref *model.UserPreference) error

// 检查用户是否配置了特定AI服务
func (s *UserConfigService) HasConfiguredAI(userID uint, provider string) bool
```

#### 3.4 用户配置API
**文件**: `internal/handler/user_config_handler.go`

**API端点**:
```
GET  /api/v1/user/config/ai           - 获取用户AI配置
PUT  /api/v1/user/config/ai           - 更新用户AI配置
GET  /api/v1/user/config/preferences  - 获取用户偏好
PUT  /api/v1/user/config/preferences  - 更新用户偏好
```

**所有端点都需要JWT认证**

---

### 4. 配置优先级逻辑（Phase 3 核心）

#### 4.1 AI配置解析器
**文件**: `internal/core/services/ai_config_resolver.go`

**优先级策略**:
```
用户配置 → 系统配置 → 优雅降级（返回空配置）
```

**核心功能**:
```go
// 获取DeepSeek配置
func (r *AIConfigResolver) GetDeepSeekConfig(userID uint) *ResolvedAIConfig

// 获取Gemini配置
func (r *AIConfigResolver) GetGeminiConfig(userID uint) *ResolvedAIConfig

// 获取OpenAI兼容配置
func (r *AIConfigResolver) GetOpenAICompatibleConfig(userID uint) *ResolvedAIConfig

// 获取百度翻译配置
func (r *AIConfigResolver) GetBaiduConfig(userID uint) *ResolvedAIConfig

// 获取首选翻译服务（自动选择最佳可用服务）
func (r *AIConfigResolver) GetPrimaryTranslationConfig(userID uint) *ResolvedAIConfig

// 获取元数据生成配置（固定使用Gemini）
func (r *AIConfigResolver) GetMetadataConfig(userID uint) *ResolvedAIConfig
```

**ResolvedAIConfig 结构**:
```go
type ResolvedAIConfig struct {
    Provider      string  // deepseek, gemini, openai, baidu
    Enabled       bool    // 是否可用
    APIKey        string  // 单个API密钥
    APIKeys       []string // 多个API密钥（用于轮询）
    Model         string  // 模型名称
    BaseURL       string  // API地址（OpenAI兼容）
    Timeout       int     // 超时时间（秒）
    MaxTokens     int     // 最大token数
    UsesUserConfig bool   // 是否使用了用户配置
}
```

**使用示例**:
```go
resolver := services.NewAIConfigResolver(db, logger, userConfigService, appConfig)

// 获取用户的Gemini配置
config := resolver.GetGeminiConfig(userID)

if !config.Enabled {
    // 无可用配置，优雅降级
    logger.Warn("Gemini未配置，跳过视频分析")
    return nil
}

if config.UsesUserConfig {
    logger.Infof("使用用户配置的Gemini API Key")
} else {
    logger.Infof("使用系统配置的Gemini API Key")
}

// 使用配置调用API
client := genai.NewClient(ctx, option.WithAPIKey(config.APIKey))
```

---

### 5. 系统配置权限控制（已完成）

**文件**: `internal/handler/config_handler.go`

**权限设计**:
- **GET端点** (读取): 所有登录用户可访问
- **PUT/POST端点** (修改): 仅管理员可访问

**实现代码**:
```go
api := server.Engine.Group("/api/v1")
api.Use(auth.LoadUserRole(h.db))  // 加载用户角色

config := api.Group("/config")
{
    // 公开端点（所有登录用户可读取）
    config.GET("/deepseek", h.getDeepSeekConfig)
    config.GET("/gemini", h.getGeminiConfig)
    config.GET("/openai-compatible", h.getOpenAICompatibleConfig)
    // ... 更多GET端点

    // 管理员端点（仅管理员可修改）
    adminConfig := config.Group("")
    adminConfig.Use(auth.RequireAdmin())
    {
        adminConfig.PUT("/deepseek", h.updateDeepSeekConfig)
        adminConfig.PUT("/gemini", h.updateGeminiConfig)
        adminConfig.PUT("/openai-compatible", h.updateOpenAICompatibleConfig)
        // ... 更多PUT/POST端点
    }
}
```

**前端访问控制**:
- 普通用户: 可以**查看**系统设置，但修改按钮被禁用
- 管理员: 可以**查看和修改**系统设置

---

## 🎯 下一步：前端实施

虽然后端已完全实施，但前端仍需更新以支持新功能：

### 1. 用户设置页面
创建 `web/src/app/user-settings/page.tsx`:

```tsx
// 用户个人设置页面（所有用户可访问）
// - AI密钥配置
// - 个人偏好设置
```

### 2. 系统设置页面权限控制
更新 `web/src/app/settings/page.tsx`:

```tsx
const { user } = useAuth();
const isAdmin = user?.role === 'admin';

return (
  <div>
    {/* 仅管理员可见 */}
    {isAdmin && <AIModelSettings />}
    {isAdmin && <GeneralSettings />}

    {/* 所有用户可见 */}
    <AccountSettings />

    {/* 非管理员提示 */}
    {!isAdmin && <ReadOnlyIndicator />}
  </div>
)
```

### 3. API调用封装
创建 `web/src/lib/userConfig.ts`:

```typescript
export const userConfigAPI = {
  getAIConfig: () => authFetch('/api/v1/user/config/ai'),
  updateAIConfig: (data) => authFetch('/api/v1/user/config/ai', { method: 'PUT', body: data }),
  getPreferences: () => authFetch('/api/v1/user/config/preferences'),
  updatePreferences: (data) => authFetch('/api/v1/user/config/preferences', { method: 'PUT', body: data }),
}
```

---

## 📊 数据库表结构

### cw_users (已更新)
```sql
ALTER TABLE cw_users ADD COLUMN role VARCHAR(20) DEFAULT 'user';
CREATE INDEX idx_users_role ON cw_users(role);
```

### cw_user_ai_configs (新增)
```sql
CREATE TABLE cw_user_ai_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL UNIQUE,
    deepseek_enabled BOOLEAN DEFAULT 0,
    deepseek_api_key VARCHAR(255),
    deepseek_model VARCHAR(50) DEFAULT 'deepseek-chat',
    gemini_enabled BOOLEAN DEFAULT 0,
    gemini_api_key VARCHAR(255),
    gemini_api_keys TEXT,
    gemini_model VARCHAR(50) DEFAULT 'gemini-2.0-flash',
    gemini_timeout INTEGER DEFAULT 120,
    gemini_max_tokens INTEGER DEFAULT 8000,
    openai_enabled BOOLEAN DEFAULT 0,
    openai_provider VARCHAR(50),
    openai_api_key VARCHAR(255),
    openai_base_url VARCHAR(255),
    openai_model VARCHAR(50) DEFAULT 'gpt-3.5-turbo',
    openai_timeout INTEGER DEFAULT 60,
    openai_max_tokens INTEGER DEFAULT 4000,
    baidu_enabled BOOLEAN DEFAULT 0,
    baidu_app_id VARCHAR(100),
    baidu_secret VARCHAR(255),
    created_at DATETIME,
    updated_at DATETIME,
    UNIQUE(user_id)
);
```

### cw_user_preferences (新增)
```sql
CREATE TABLE cw_user_preferences (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL UNIQUE,
    email_notifications_enabled BOOLEAN DEFAULT 1,
    notification_email VARCHAR(255),
    default_auto_upload BOOLEAN DEFAULT 1,
    default_upload_delay INTEGER DEFAULT 10,
    default_subtitle_delay INTEGER DEFAULT 10,
    default_copyright INTEGER DEFAULT 2,
    default_source VARCHAR(50) DEFAULT 'YouTube',
    default_tid INTEGER DEFAULT 122,
    theme VARCHAR(20) DEFAULT 'light',
    language VARCHAR(10) DEFAULT 'zh',
    items_per_page INTEGER DEFAULT 20,
    show_advanced BOOLEAN DEFAULT 0,
    enable_analytics BOOLEAN DEFAULT 0,
    created_at DATETIME,
    updated_at DATETIME,
    UNIQUE(user_id)
);
```

---

## 🔒 安全建议

### 1. API密钥加密存储
**当前**: 明文存储在数据库
**建议**: 使用加密存储（如 AES-256）

```go
// 加密
encryptedKey, err := encrypt(userAIConfig.DeepSeekAPIKey, encryptionKey)

// 解密
decryptedKey, err := decrypt(encryptedKey, encryptionKey)
```

### 2. 审计日志
记录管理员的所有配置修改操作：

```go
logger.Infof("管理员 %s 修改了系统配置: %s", adminUsername, changeDetails)
```

### 3. 密钥轮换提醒
提醒用户定期更换API密钥：

```go
if time.Since(config.CreatedAt) > 90*24*time.Hour {
    logger.Warn("API密钥已使用超过90天，建议更换")
}
```

---

## ✅ 测试检查清单

- [ ] 启动应用，确认自动迁移成功执行
- [ ] 第一个用户自动设为管理员
- [ ] 管理员可以修改系统配置
- [ ] 普通用户可以查看但不能修改系统配置
- [ ] 用户可以配置个人AI密钥
- [ ] 用户配置优先于系统配置
- [ ] 当用户无配置时，回退到系统配置
- [ ] 当两者都无配置时，优雅降级（不报错）
- [ ] JWT中间件正确加载用户角色
- [ ] RequireAdmin中间件正确拦截非管理员请求

---

## 📚 相关文档

- `docs/ADMIN_SYSTEM_GUIDE.md` - 管理员系统实施指南
- `pkg/utils/system_monitor.go` - 系统监控实现
- `pkg/utils/timeout_helper.go` - 超时控制实现
- `internal/auth/admin_middleware.go` - 管理员中间件
- `internal/core/services/ai_config_resolver.go` - AI配置解析器

---

## 🎉 总结

用户个性化配置系统已完整实施，包括：

✅ **系统保护**: 磁盘检查、超时控制、安全JWT
✅ **角色权限**: admin/user角色系统，与会员等级解耦
✅ **用户配置**: 每用户独立的AI配置和偏好设置
✅ **配置优先级**: 用户 → 系统 → 优雅降级
✅ **权限控制**: 普通用户只读，管理员可修改
✅ **自动迁移**: 启动时自动运行数据库迁移

**后端实施 100% 完成**，前端仍需集成。
