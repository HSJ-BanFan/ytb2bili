# Week 2 剩余改进项实施指南

**文档日期**: 2026-01-01
**状态**: 待实施
**预计完成时间**: 2-3小时

---

## 📋 实施清单

- [x] P2-3: 移除日志中的明文密码 ✅ 已完成
- [x] 视频删除审计日志 ✅ 已完成
- [ ] 任务重试审计日志
- [ ] 视频上传审计日志
- [ ] Cookie刷新审计日志
- [ ] 创建集成测试框架
- [ ] 加密服务集成测试
- [ ] 审计日志集成测试
- [ ] 备份服务集成测试

---

## 第一部分：补充审计日志（剩余3项）

### 1️⃣ 任务重试审计日志

#### 📍 修改文件
`internal/handler/video_handler.go`

#### 🎯 目标
在用户重试失败任务时记录审计日志，用于追溯用户的操作行为。

#### 🔧 实施步骤

**步骤1**: 找到`retryTaskStep`函数

在`video_handler.go`中搜索：
```go
func (h *VideoHandler) retryTaskStep(c *gin.Context)
```

**步骤2**: 在函数开头添加审计日志常量

在文件顶部的import后添加：
```go
const (
	ActionRetryTask = "retry_task_step"
	ActionResetFailed = "reset_failed_steps"
	ActionResetAll = "reset_all_steps"
	ActionManualUpload = "manual_upload"
)
```

**步骤3**: 修改`retryTaskStep`函数

找到以下代码位置（大约在第540行）：
```go
func (h *VideoHandler) retryTaskStep(c *gin.Context) {
	idStr := c.Param("id")
	stepName := c.Param("stepName")

	// 获取用户ID
	userID, exists := auth.GetUserID(c)
	if !exists || userID == 0 {
		c.JSON(http.StatusUnauthorized, VideoListResponse{
			Code:    401,
			Message: "未登录",
		})
		return
	}

	// ... 查询视频逻辑 ...

	// 重试任务
	err := h.TaskStepService.RetryTaskStep(savedVideo.ID, stepName)
	if err != nil {
		// 错误处理
	}

	// ✅ 在成功响应前添加审计日志
	username := getUsername(c)
	h.AuditService.LogSuccess(
		userID,
		username,
		ActionRetryTask,          // action
		"task_step",              // resource
		fmt.Sprintf("%s:%s", savedVideo.VideoID, stepName), // resource_id
		c.ClientIP(),
		c.Request.UserAgent(),
		fmt.Sprintf("重试任务成功: %s -> %s", savedVideo.Title, stepName),
	)

	c.JSON(http.StatusOK, VideoListResponse{
		Code:    200,
		Message: "任务重试成功",
		// ...
	})
}
```

**步骤4**: 同样为其他任务操作添加审计日志

**resetAllFailedSteps函数**（大约在第610行）：
```go
// ✅ 在成功响应前添加
username := getUsername(c)
h.AuditService.LogSuccess(
	userID,
	username,
	ActionResetFailed,
	"video",
	savedVideo.VideoID,
	c.ClientIP(),
	c.Request.UserAgent(),
	fmt.Sprintf("重置失败任务: %s", savedVideo.Title),
)
```

**resetAllSteps函数**（大约在第640行）：
```go
// ✅ 在成功响应前添加
username := getUsername(c)
h.AuditService.LogSuccess(
	userID,
	username,
	ActionResetAll,
	"video",
	savedVideo.VideoID,
	c.ClientIP(),
	c.Request.UserAgent(),
	fmt.Sprintf("重置所有任务: %s", savedVideo.Title),
)
```

**manualUploadVideo函数**（大约在第670行）：
```go
// ✅ 在上传开始前添加
username := getUsername(c)
h.AuditService.LogSuccess(
	userID,
	username,
	ActionManualUpload,
	"video",
	savedVideo.VideoID,
	c.ClientIP(),
	c.Request.UserAgent(),
	fmt.Sprintf("手动上传视频: %s", savedVideo.Title),
)
```

**manualUploadSubtitle函数**（大约在第700行）：
```go
// ✅ 在上传开始前添加
username := getUsername(c)
h.AuditService.LogSuccess(
	userID,
	username,
	"manual_upload_subtitle",
	"video",
	savedVideo.VideoID,
	c.ClientIP(),
	c.Request.UserAgent(),
	fmt.Sprintf("手动上传字幕: %s", savedVideo.Title),
)
```

#### ✅ 验证方法

```bash
# 1. 重新编译
go build -o ytb2bili.exe .

# 2. 运行应用
./ytb2bili.exe

# 3. 使用API重试任务
curl -X POST http://localhost:8096/api/v1/videos/abc123/steps/generate_metadata/retry \
  -H "Authorization: Bearer <your_token>"

# 4. 查询审计日志
mysql -u root -p bili_up -e "
SELECT * FROM cw_audit_logs
WHERE action = 'retry_task_step'
ORDER BY created_at DESC
LIMIT 5;
"

# 预期输出：
# | user_id | action           | resource   | resource_id | message                    |
# |---------|------------------|------------|-------------|----------------------------|
# | 123     | retry_task_step  | task_step  | abc123:...  | 重试任务成功: 视频标题     |
```

---

### 2️⃣ 视频上传审计日志

#### 📍 修改文件
`internal/chain_task/handlers/upload_to_bilibili.go`

#### 🎯 目标
在视频成功上传到B站时记录审计日志。

#### 🔧 实施步骤

**步骤1**: 在文件顶部import中添加audit包
```go
import (
	// ... 现有imports
	"github.com/difyz9/ytb2bili/pkg/audit"
)
```

**步骤2**: 在`UploadToBilibili`结构体中添加AuditService字段

找到：
```go
type UploadToBilibili struct {
	base.BaseTask
	App                *core.AppServer
	DB                 *gorm.DB
	SavedVideoService  *services.SavedVideoService
	BiliAccountService *services.BiliAccountService
	// ...
}
```

修改为：
```go
type UploadToBilibili struct {
	base.BaseTask
	App                *core.AppServer
	DB                 *gorm.DB
	SavedVideoService  *services.SavedVideoService
	BiliAccountService *services.BiliAccountService
	AuditService       *audit.AuditService // ✅ 添加审计服务
	// ...
}
```

**步骤3**: 找到上传成功的位置并添加审计日志

在`Execute`函数中，找到上传成功的逻辑（大约在第400行）：
```go
// 上传成功
if result != nil && result.Code == 0 {
	bvid := result.Data.Bvid

	// ✅ 添加审计日志
	videoID := context["video_id"].(string)
	userID := context["user_id"].(uint)
	username := context["username"].(string)

	h.AuditService.LogSuccess(
		userID,
		username,
		"upload_video",     // action
		"video",            // resource
		videoID,            // resource_id
		"",                 // IP (后台任务无IP)
		"",                 // UserAgent (后台任务无UA)
		fmt.Sprintf("视频上传成功B站: %s -> %s", videoID, bvid),
	)

	// 更新数据库
	savedVideo.Bvid = bvid
	savedVideo.Status = 300 // 已上传
	// ...
}
```

**步骤4**: 修改NewUploadToBilibili构造函数

找到：
```go
func NewUploadToBilibili(app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient, savedVideoService *services.SavedVideoService, biliAccountService *services.BiliAccountService) *UploadToBilibili {
```

修改为：
```go
func NewUploadToBilibili(app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient, savedVideoService *services.SavedVideoService, biliAccountService *services.BiliAccountService, auditService *audit.AuditService) *UploadToBilibili {
	return &UploadToBilibili{
		BaseTask:           base.BaseTask{Name: "UploadToBilibili", StateManager: stateManager, Client: client},
		App:                app,
		SavedVideoService:  savedVideoService,
		BiliAccountService: biliAccountService,
		AuditService:       auditService, // ✅ 添加
	}
}
```

**步骤5**: 更新main.go中的依赖注入

找到NewUploadToBilibili的调用（大约在第210行）：
```go
fx.Provide(handler.NewUploadToBilibili),
```

需要修改为在fx.Invoke中手动创建（如果需要注入auditService），或者在fx.Provide前添加auditService参数。

**简化方案**：在chain_task初始化时注入
```go
// 在 registerHandlers 函数中
uploadHandler := handler.NewUploadToBilibili(server, stateManager, cosClient, savedVideoService, biliAccountService, auditService)
```

#### ✅ 验证方法

```bash
# 1. 编译并运行
./ytb2bili.exe

# 2. 上传一个视频
# 等待上传完成

# 3. 查询审计日志
mysql -u root -p bili_up -e "
SELECT created_at, action, resource_id, message
FROM cw_audit_logs
WHERE action = 'upload_video'
ORDER BY created_at DESC
LIMIT 5;
"
```

---

### 3️⃣ Cookie刷新审计日志

#### 📍 修改文件
`internal/handler/bili_account_handler.go`

#### 🎯 目标
在用户刷新B站Cookie时记录审计日志。

#### 🔧 实施步骤

**步骤1**: 在审计常量中添加新操作类型

在`pkg/audit/audit_service.go`中添加：
```go
const (
	// ... 现有常量
	ActionRefreshBiliCookies = "refresh_bili_cookies"
)
```

**步骤2**: 添加刷新Cookie的审计日志

在`bili_account_handler.go`中找到Cookie刷新的handler（如果存在），或者添加新的handler：
```go
// refreshCookies 刷新B站账号Cookie
func (h *BiliAccountHandler) refreshCookies(c *gin.Context) {
	userID := h.getUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		return
	}

	// 解析请求
	var req struct {
		Mid int64 `json:"mid" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	// 获取账号
	account, err := h.BiliAccountService.GetByID(req.Mid)
	if err != nil {
		h.AuditService.LogFailure(userID, "", ActionRefreshBiliCookies, "bili_account",
			fmt.Sprintf("%d", req.Mid), c.ClientIP(), c.Request.UserAgent(), "账号不存在")
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "账号不存在"})
		return
	}

	// 刷新Cookie逻辑
	// ... 刷新代码 ...

	// ✅ 记录审计日志
	if err == nil {
		h.AuditService.LogSuccess(userID, "", ActionRefreshBiliCookies, "bili_account",
			fmt.Sprintf("%d", account.Mid), c.ClientIP(), c.Request.UserAgent(),
			fmt.Sprintf("刷新Cookie成功: %s", account.BiliName))
	} else {
		h.AuditService.LogFailure(userID, "", ActionRefreshBiliCookies, "bili_account",
			fmt.Sprintf("%d", account.Mid), c.ClientIP(), c.Request.UserAgent(),
			fmt.Sprintf("刷新Cookie失败: %v", err))
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "刷新成功"})
}
```

**步骤3**: 注册新路由（如果需要）

在`RegisterRoutes`函数中添加：
```go
biliAccounts.POST("/refresh-cookies", h.refreshCookies)
```

#### ✅ 验证方法

```bash
# 调用刷新Cookie API
curl -X POST http://localhost:8096/api/v1/bilibili/accounts/refresh-cookies \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"mid": 123456789}'

# 查询审计日志
mysql -u root -p bili_up -e "
SELECT created_at, action, message
FROM cw_audit_logs
WHERE action = 'refresh_bili_cookies'
ORDER BY created_at DESC;
"
```

---

## 第二部分：集成测试框架

### 4️⃣ 创建集成测试框架

#### 📍 新建目录结构
```
tests/
├── integration/
│   ├── test_setup.go       # 测试初始化
│   ├── encryption_test.go  # 加密服务测试
│   ├── audit_log_test.go   # 审计日志测试
│   ├── backup_test.go      # 备份服务测试
│   └── helpers.go          # 测试辅助函数
└── README.md               # 测试文档
```

#### 🔧 实施步骤

**步骤1**: 创建测试初始化文件

创建 `tests/integration/test_setup.go`：
```go
package integration

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/core/types"
	"github.com/difyz9/ytb2bili/pkg/audit"
	"github.com/difyz9/ytb2bili/pkg/crypto"
	"github.com/difyz9/ytb2bili/pkg/logger"
	"github.com/difyz9/ytb2bili/pkg/store"
	"github.com/difyz9/ytb2bili/pkg/store/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestApp 测试应用容器
type TestApp struct {
	DB              *gorm.DB
	AppServer       *core.AppServer
	Config          *types.AppConfig
	EncryptionSvc   *crypto.EncryptionService
	AuditSvc        *audit.AuditService
	SavedVideoSvc   *services.SavedVideoService
	AccountSvc      *services.BiliAccountService
	TempDir         string
	CleanupFunc     func()
}

// SetupTestApp 初始化测试应用
func SetupTestApp(t *testing.T) *TestApp {
	// 1. 创建临时目录
	tempDir, err := os.MkdirTemp("", "ytb2bili_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	// 2. 创建内存数据库
	dbFile := fmt.Sprintf("%s/test.db", tempDir)
	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // 静默模式
	})
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}

	// 3. 自动迁移表结构
	err = db.AutoMigrate(
		&model.User{},
		&model.SavedVideo{},
		&model.TaskStep{},
		&model.App{},
		&model.UserToken{},
		&model.UserBiliAccount{},
		&model.UserAIConfig{},
		&model.UserPreference{},
		&model.EmailVerification{},
		&model.AuditLog{},
	)
	if err != nil {
		t.Fatalf("数据库迁移失败: %v", err)
	}

	// 4. 初始化配置
	config := &types.AppConfig{
		Listen:      ":8096",
		Environment: "test",
		Debug:       false,
		FileUpDir:   tempDir,
		Database: types.DatabaseConfig{
			Type:     "sqlite",
			Database: dbFile,
		},
		Security: types.SecurityConfig{
			CookieExpireDays:  30,
			CookieWarningDays: 3,
		},
	}

	// 5. 初始化加密服务
	encSvc, err := crypto.NewEncryptionService([]byte("test_32_byte_encryption_key_1234"))
	if err != nil {
		t.Fatalf("初始化加密服务失败: %v", err)
	}

	// 6. 初始化审计服务
	auditSvc := audit.NewAuditService(db)

	// 7. 初始化AppServer
	appServer := core.NewAppServer(config, nil)
	appServer.Logger, _ = logger.NewLogger(false)

	// 8. 初始化服务
	savedVideoSvc := services.NewSavedVideoService(db, appServer.Logger)
	accountSvc := services.NewBiliAccountService(db, appServer.Logger)

	// 9. 创建测试用户
	user := &model.User{
		Username:      "test_user",
		Email:         "test@example.com",
		Password:      "$2a$10$vIysWJwYXHJECRrret5pAeuwpzjwzVeXDDLPbWJrzng7Xx6oRS6sK", // 123456
		Role:          "user",
		Status:        1,
		EmailVerified:  true,
	}
	db.Create(user)

	// 10. 清理函数
	cleanup := func() {
		os.RemoveAll(tempDir)
		auditSvc.Close() // 关闭审计服务
	}

	return &TestApp{
		DB:            db,
		AppServer:     appServer,
		Config:        config,
		EncryptionSvc: encSvc,
		AuditSvc:      auditSvc,
		SavedVideoSvc: savedVideoSvc,
		AccountSvc:    accountSvc,
		TempDir:       tempDir,
		CleanupFunc:   cleanup,
	}
}

// Cleanup 清理测试环境
func (app *TestApp) Cleanup() {
	if app.CleanupFunc != nil {
		app.CleanupFunc()
	}
}

// CreateTestVideo 创建测试视频
func (app *TestApp) CreateTestVideo(t *testing.T, userID uint) *model.SavedVideo {
	video := &model.SavedVideo{
		UserID:      userID,
		VideoID:     fmt.Sprintf("test_%d", time.Now().UnixNano()),
		Title:       "测试视频",
		Description: "这是一个测试视频",
		Status:      200, // 准备上传
	}
	if err := app.DB.Create(video).Error; err != nil {
		t.Fatalf("创建测试视频失败: %v", err)
	}
	return video
}

// CreateTestAccount 创建测试B站账号
func (app *TestApp) CreateTestAccount(t *testing.T, userID uint) *model.UserBiliAccount {
	account := &model.UserBiliAccount{
		UserID:      userID,
		BiliMid:     123456789,
		BiliName:    "测试账号",
		Cookies:     "test_cookies",
		ExpiresAt:   time.Now().Add(30 * 24 * time.Hour),
		IsPrimary:   true,
	}
	if err := app.AccountSvc.Create(account); err != nil {
		t.Fatalf("创建测试账号失败: %v", err)
	}
	return account
}

// WaitForAuditLog 等待审计日志写入（带超时）
func (app *TestApp) WaitForAuditLog(t *testing.T, timeout time.Duration) int {
	var count int64
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		app.DB.Model(&model.AuditLog{}).Count(&count)
		if count > 0 {
			return int(count)
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("等待审计日志写入超时")
	return 0
}
```

**步骤2**: 创建测试辅助函数

创建 `tests/integration/helpers.go`：
```go
package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// AssertAuditLog 断言审计日志存在
func AssertAuditLog(t *testing.T, app *TestApp, action string, resourceID string) {
	var log model.AuditLog
	err := app.DB.Where("action = ? AND resource_id = ?", action, resourceID).
		Order("created_at DESC").
		First(&log).Error

	assert.NoError(t, err, "审计日志应该存在")
	assert.Equal(t, action, log.Action)
	assert.Equal(t, resourceID, log.ResourceID)
	assert.True(t, log.Success, "操作应该成功")
}

// GetAuditLogCount 获取审计日志数量
func GetAuditLogCount(t *testing.T, app *TestApp) int64 {
	var count int64
	err := app.DB.Model(&model.AuditLog{}).Count(&count).Error
	assert.NoError(t, err)
	return count
}

// ClearAuditLogs 清空审计日志
func ClearAuditLogs(t *testing.T, app *TestApp) {
	err := app.DB.Exec("DELETE FROM cw_audit_logs").Error
	assert.NoError(t, err)
}
```

**步骤3**: 创建测试文档

创建 `tests/README.md`：
```markdown
# 集成测试文档

## 运行测试

```bash
# 运行所有集成测试
go test -v ./tests/integration/...

# 运行特定测试
go test -v ./tests/integration/... -run TestEncryption

# 运行测试并显示覆盖率
go test -v ./tests/integration/... -cover
```

## 测试结构

- `test_setup.go`: 测试环境初始化
- `helpers.go`: 测试辅助函数
- `*_test.go`: 具体的测试用例

## 注意事项

1. 测试使用内存数据库，不影响生产数据
2. 每个测试独立运行，互不影响
3. 测试完成后自动清理临时文件
```

---

### 5️⃣ 加密服务集成测试

#### 📍 创建文件
`tests/integration/encryption_test.go`

#### 🔧 实施代码

```go
package integration

import (
	"testing"

	"github.com/difyz9/ytb2bili/pkg/crypto"
	"github.com/stretchr/testify/assert"
)

// TestEncryptionService_RoundTrip 测试加密解密往返
func TestEncryptionService_RoundTrip(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// 测试数据
	testData := []string{
		"simple text",
		"中文测试数据",
		"special chars: !@#$%^&*()",
		"",
		"a", // 单字符
	}

	for _, data := range testData {
		t.Run(data, func(t *testing.T) {
			// 加密
			encrypted, err := app.EncryptionSvc.EncryptString(data)
			assert.NoError(t, err, "加密应该成功")
			assert.NotEmpty(t, encrypted, "加密结果不应为空")
			assert.NotEqual(t, data, encrypted, "加密结果应与原文不同")

			// 解密
			decrypted, err := app.EncryptionSvc.DecryptString(encrypted)
			assert.NoError(t, err, "解密应该成功")
			assert.Equal(t, data, decrypted, "解密结果应与原文一致")
		})
	}
}

// TestEncryptionService_InvalidCiphertext 测试无效密文处理
func TestEncryptionService_InvalidCiphertext(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	invalidCiphertexts := []string{
		"",
		"invalid_base64!",
		"too_short",
		"\x00\x01\x02\x03",
	}

	for _, ciphertext := range invalidCiphertexts {
		t.Run(ciphertext, func(t *testing.T) {
			_, err := app.EncryptionSvc.DecryptString(ciphertext)
			assert.Error(t, err, "无效密文应该返回错误")
		})
	}
}

// TestEncryptionService_JSONEncryption 测试JSON加密
func TestEncryptionService_JSONencryption(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// 测试数据结构
	type TestStruct struct {
		Name   string `json:"name"`
		Age    int    `json:"age"`
		Secret string `json:"secret"`
	}

	original := TestStruct{
		Name:   "张三",
		Age:    30,
		Secret: "my_secret_password",
	}

	// 加密
	encrypted, err := app.EncryptionSvc.EncryptJSON(original)
	assert.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	// 解密
	var decrypted TestStruct
	err = app.EncryptionSvc.DecryptJSON(encrypted, &decrypted)
	assert.NoError(t, err)
	assert.Equal(t, original.Name, decrypted.Name)
	assert.Equal(t, original.Age, decrypted.Age)
	assert.Equal(t, original.Secret, decrypted.Secret)
}

// TestEncryptionService_ConcurrentAccess 测试并发访问
func TestEncryptionService_ConcurrentAccess(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	iterations := 100
	done := make(chan bool, iterations)

	for i := 0; i < iterations; i++ {
		go func() {
			data := "concurrent_test_data"
			encrypted, err := app.EncryptionSvc.EncryptString(data)
			assert.NoError(t, err)

			decrypted, err := app.EncryptionSvc.DecryptString(encrypted)
			assert.NoError(t, err)
			assert.Equal(t, data, decrypted)

			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < iterations; i++ {
		<-done
	}
}

// TestEncryptionService_LargeData 测试大数据加密
func TestEncryptionService_LargeData(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// 生成1MB数据
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	// 加密
	encrypted, err := app.EncryptionSvc.Encrypt(largeData)
	assert.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	// 解密
	decrypted, err := app.EncryptionSvc.Decrypt(encrypted)
	assert.NoError(t, err)
	assert.Equal(t, largeData, decrypted)
}
```

#### ✅ 运行测试

```bash
cd E:/githubitem/ytb2bili
go test -v ./tests/integration/... -run TestEncryption

# 预期输出：
# === RUN   TestEncryptionService_RoundTrip
# --- PASS: TestEncryptionService_RoundTrip (0.02s)
#     --- PASS: TestEncryptionService_RoundTrip/simple_text
#     --- PASS: TestEncryptionService_RoundTrip/中文测试数据
# ...
# === RUN   TestEncryptionService_ConcurrentAccess
# --- PASS: TestEncryptionService_ConcurrentAccess (0.05s)
# PASS
```

---

### 6️⃣ 审计日志集成测试

#### 📍 创建文件
`tests/integration/audit_log_test.go`

#### 🔧 实施代码

```go
package integration

import (
	"testing"
	"time"

	"github.com/difyz9/ytb2bili/pkg/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuditLog_BasicLogging 测试基本日志记录
func TestAuditLog_BasicLogging(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// 记录日志
	entry := audit.LogEntry{
		UserID:     1,
		Username:   "test_user",
		Action:     "test_action",
		Resource:   "test_resource",
		ResourceID: "test_123",
		IP:         "192.168.1.1",
		UserAgent:  "test-agent",
		Success:    true,
		Message:    "测试消息",
	}

	app.AuditSvc.Log(entry)

	// 等待异步写入
	time.Sleep(100 * time.Millisecond)

	// 验证日志已写入
	app.WaitForAuditLog(t, 2*time.Second)

	var logs []model.AuditLog
	err := app.DB.Find(&logs).Error
	require.NoError(t, err)
	assert.Greater(t, len(logs), 0, "应该有审计日志")

	// 验证日志内容
	lastLog := logs[len(logs)-1]
	assert.Equal(t, uint(1), lastLog.UserID)
	assert.Equal(t, "test_user", lastLog.Username)
	assert.Equal(t, "test_action", lastLog.Action)
	assert.Equal(t, "test_resource", lastLog.Resource)
	assert.Equal(t, "test_123", lastLog.ResourceID)
	assert.True(t, lastLog.Success)
}

// TestAuditLog_RetryMechanism 测试重试机制
func TestAuditLog_RetryMechanism(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// 关闭数据库连接，模拟故障
	sqlDB, _ := app.DB.DB()
	sqlDB.Close()

	// 记录多条日志（应该缓冲）
	for i := 0; i < 5; i++ {
		app.AuditSvc.LogSuccess(1, "user", "test", "resource", "123", "1.1.1.1", "agent", "message")
	}

	// 重新连接数据库（模拟恢复）
	newDB, err := gorm.Open(sqlite.Open(app.DB.Migrator().(*gorm.DB).Config.DSN), &gorm.Config{})
	require.NoError(t, err)
	app.AuditSvc = audit.NewAuditService(newDB)
	defer newDB.Close()

	// 等待重试和写入
	time.Sleep(5 * time.Second)

	// 验证日志最终写入成功
	var count int64
	newDB.Model(&model.AuditLog{}).Count(&count)
	assert.Greater(t, count, int64(0), "重试后应该成功写入日志")
}

// TestAuditLog_HighConcurrency 测试高并发场景
func TestAuditLog_HighConcurrency(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// 并发记录1000条日志
	numGoroutines := 100
	logsPerGoroutine := 10

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < logsPerGoroutine; j++ {
				app.AuditSvc.LogSuccess(
					uint(id),
					"user",
					"concurrent_test",
					"resource",
					"123",
					"127.0.0.1",
					"test",
					"concurrent message",
				)
			}
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// 等待异步写入完成
	time.Sleep(3 * time.Second)

	// 验证日志数量
	var count int64
	app.DB.Model(&model.AuditLog{}).Count(&count)
	assert.Equal(t, int64(numGoroutines*logsPerGoroutine), count, "应该记录所有日志")
}

// TestAuditLog_QueryByUser 测试按用户查询
func TestAuditLog_QueryByUser(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// 创建测试用户
	user1 := app.CreateTestUser(t)
	user2 := app.CreateTestUser(t)

	// 记录不同用户的日志
	app.AuditSvc.LogSuccess(user1.ID, "user1", "action1", "resource", "1", "1.1.1.1", "agent", "message1")
	app.AuditSvc.LogSuccess(user2.ID, "user2", "action2", "resource", "2", "1.1.1.1", "agent", "message2")
	app.AuditSvc.LogSuccess(user1.ID, "user1", "action3", "resource", "3", "1.1.1.1", "agent", "message3")

	time.Sleep(100 * time.Millisecond)

	// 查询user1的日志
	var user1Logs []model.AuditLog
	err := app.DB.Where("user_id = ?", user1.ID).Find(&user1Logs).Error
	require.NoError(t, err)
	assert.Equal(t, 2, len(user1Logs))

	// 查询user2的日志
	var user2Logs []model.AuditLog
	err = app.DB.Where("user_id = ?", user2.ID).Find(&user2Logs).Error
	require.NoError(t, err)
	assert.Equal(t, 1, len(user2Logs))
}

// TestAuditLog_DetailsStorage 测试Details字段存储
func TestAuditLog_DetailsStorage(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	details := map[string]interface{}{
		"key1": "value1",
		"key2": 12345,
		"key3": true,
		"nested": map[string]interface{}{
			"subkey": "subvalue",
		},
	}

	app.AuditSvc.LogSuccess(
		1, "user", "test", "resource", "123",
		"1.1.1.1", "agent", "message with details",
		audit.WithDetails(details),
	)

	time.Sleep(100 * time.Millisecond)

	// 查询日志
	var log model.AuditLog
	err := app.DB.Where("action = ?", "test").First(&log).Error
	require.NoError(t, err)
	assert.NotEmpty(t, log.Details, "Details字段应该有内容")

	// 验证JSON可以正确解析
	var parsedDetails map[string]interface{}
	err = json.Unmarshal([]byte(log.Details), &parsedDetails)
	require.NoError(t, err)
	assert.Equal(t, "value1", parsedDetails["key1"])
	assert.Equal(t, float64(12345), parsedDetails["key2"]) // JSON数字解析为float64
	assert.Equal(t, true, parsedDetails["key3"])
}
```

---

### 7️⃣ 备份服务集成测试

#### 📍 创建文件
`tests/integration/backup_test.go`

#### 🔧 实施代码

```go
package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/difyz9/ytb2bili/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackupService_CreateBackup 测试创建备份
func TestBackupService_CreateBackup(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// 创建一些测试数据
	user := app.CreateTestUser(t)
	video := app.CreateTestVideo(t, user.ID)

	// 创建备份服务
	backupDir := filepath.Join(app.TempDir, "backups")
	backupSvc := db.NewBackupService(app.DB, backupDir)

	// 执行备份
	err := backupSvc.CreateEncryptedBackup()
	require.NoError(t, err, "备份应该成功")

	// 验证备份文件存在
	files, err := os.ReadDir(backupDir)
	require.NoError(t, err)
	assert.Equal(t, 1, len(files), "应该有一个备份文件")

	backupFile := files[0]
	assert.Contains(t, backupFile.Name(), "backup_", "文件名应该包含backup_前缀")
	assert.Contains(t, backupFile.Name(), ".enc", "文件应该是加密的")

	// 验证备份文件大小
	info, err := backupFile.Info()
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0), "备份文件应该有内容")

	// 验证备份文件可以解密
	backupPath := filepath.Join(backupDir, backupFile.Name())
	encryptedData, err := os.ReadFile(backupPath)
	require.NoError(t, err)

	decryptedData, err := app.EncryptionSvc.Decrypt(encryptedData)
	require.NoError(t, err, "备份文件应该可以解密")
	assert.NotEmpty(t, decryptedData, "解密后应该有数据")
}

// TestBackupService_AutoCleanup 测试自动清理旧备份
func TestBackupService_AutoCleanup(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	backupDir := filepath.Join(app.TempDir, "backups")
	backupSvc := db.NewBackupService(app.DB, backupDir)

	// 创建多个备份
	for i := 0; i < 5; i++ {
		err := backupSvc.CreateEncryptedBackup()
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond) // 确保文件时间戳不同
	}

	// 验证有5个备份文件
	files, err := os.ReadDir(backupDir)
	require.NoError(t, err)
	assert.Equal(t, 5, len(files))

	// 手动修改第一个备份的修改时间为10天前
	files, _ = os.ReadDir(backupDir)
	oldBackupPath := filepath.Join(backupDir, files[0].Name())
	oldTime := time.Now().AddDate(0, 0, -10)
	os.Chtimes(oldBackupPath, oldTime, oldTime)

	// 创建新备份（应该触发清理）
	err = backupSvc.CreateEncryptedBackup()
	require.NoError(t, err)

	// 验证旧备份已被清理
	files, err = os.ReadDir(backupDir)
	require.NoError(t, err)
	// 应该有5个备份：4个新的 + 1个刚创建的，旧的被清理
	assert.LessOrEqual(t, len(files), 5)
}

// TestBackupService_BackupIntegrity 测试备份完整性
func TestBackupService_BackupIntegrity(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// 创建测试数据
	user := app.CreateTestUser(t)
	video := app.CreateTestVideo(t, user.ID)
	account := app.CreateTestAccount(t, user.ID)

	// 创建备份
	backupDir := filepath.Join(app.TempDir, "backups")
	backupSvc := db.NewBackupService(app.DB, backupDir)
	err := backupSvc.CreateEncryptedBackup()
	require.NoError(t, err)

	// 读取备份文件
	files, _ := os.ReadDir(backupDir)
	backupPath := filepath.Join(backupDir, files[0].Name())
	encryptedData, _ := os.ReadFile(backupPath)

	// 解密备份
	decryptedData, err := app.EncryptionSvc.Decrypt(encryptedData)
	require.NoError(t, err)

	// 将解密的数据写入临时文件
	tempDBPath := filepath.Join(app.TempDir, "restored.db")
	err = os.WriteFile(tempDBPath, decryptedData, 0644)
	require.NoError(t, err)

	// 打开恢复的数据库
	restoredDB, err := gorm.Open(sqlite.Open(tempDBPath), &gorm.Config{})
	require.NoError(t, err)
	defer restoredDB.Close()

	// 验证数据完整性
	var userCount int64
	restoredDB.Model(&model.User{}).Count(&userCount)
	assert.GreaterOrEqual(t, userCount, int64(1), "应该至少有一个用户")

	var videoCount int64
	restoredDB.Model(&model.SavedVideo{}).Count(&videoCount)
	assert.GreaterOrEqual(t, videoCount, int64(1), "应该至少有一个视频")

	var accountCount int64
	restoredDB.Model(&model.UserBiliAccount{}).Count(&accountCount)
	assert.GreaterOrEqual(t, accountCount, int64(1), "应该至少有一个账号")
}
```

---

## 第三部分：运行和验证

### 🚀 完整实施步骤

#### 步骤1: 准备工作
```bash
# 1. 安装测试依赖
go get github.com/stretchr/testify/assert
go get github.com/stretchr/testify/require

# 2. 创建测试目录
mkdir -p tests/integration
cd tests/integration
```

#### 步骤2: 创建文件
按照上述代码创建以下文件：
- `test_setup.go`
- `helpers.go`
- `encryption_test.go`
- `audit_log_test.go`
- `backup_test.go`
- `README.md`

#### 步骤3: 修改业务代码
按照第一部分的说明修改业务代码，添加审计日志。

#### 步骤4: 运行测试
```bash
# 运行所有集成测试
go test -v ./tests/integration/...

# 运行特定测试
go test -v ./tests/integration/... -run TestEncryptionService_RoundTrip

# 查看测试覆盖率
go test -v ./tests/integration/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

#### 步骤5: 验证审计日志
```bash
# 启动应用
./ytb2bili.exe

# 执行一些操作（删除视频、重试任务等）

# 查询审计日志
mysql -u root -p bili_up -e "
SELECT created_at, username, action, resource, resource_id, message
FROM cw_audit_logs
ORDER BY created_at DESC
LIMIT 20;
"
```

---

## 📊 预期结果

### 审计日志示例数据
```sql
mysql> SELECT created_at, username, action, resource, resource_id, message FROM cw_audit_logs ORDER BY created_at DESC LIMIT 10;
+---------------------+-----------+-------------------+-----------+-------------+--------------------------------------------------+
| created_at          | username  | action            | resource  | resource_id | message                                          |
+---------------------+-----------+-------------------+-----------+-------------+--------------------------------------------------+
| 2026-01-01 12:34:56 | test_user | delete_video      | video     | Cj4inlWlf4I | 删除视频成功: 测试视频                            |
| 2026-01-01 12:33:45 | test_user | retry_task_step   | task_step | abc:metadata| 重试任务成功: 测试视频 -> generate_metadata    |
| 2026-01-01 12:32:30 | test_user | upload_video      | video     | xyz123      | 视频上传成功B站: xyz123 -> BV123xx45678           |
| 2026-01-01 12:31:15 | test_user | refresh_bili_cookies|bili_account|123456789   | 刷新Cookie成功: 测试账号                        |
+---------------------+-----------+-------------------+-----------+-------------+--------------------------------------------------+
```

### 测试通过示例
```bash
$ go test -v ./tests/integration/...
=== RUN   TestEncryptionService_RoundTrip
=== RUN   TestEncryptionService_JSONEncryption
=== RUN   TestAuditLog_BasicLogging
=== RUN   TestBackupService_CreateBackup
--- PASS: TestEncryptionService_RoundTrip (0.02s)
--- PASS: TestEncryptionService_JSONEncryption (0.03s)
--- PASS: TestAuditLog_BasicLogging (0.15s)
--- PASS: TestBackupService_CreateBackup (0.25s)
PASS
ok      github.com/difyz9/ytb2bili/tests/integration    0.512s
```

---

## 🐛 常见问题排查

### 问题1: 测试编译错误
```
undefined: SetupTestApp
```
**解决**: 确保所有测试文件都在`tests/integration/`目录下，且包名为`integration`

### 问题2: 审计日志未写入
```
查询结果为空
```
**解决**:
1. 检查AuditService是否正确注入
2. 添加`time.Sleep(100 * time.Millisecond)`等待异步写入
3. 检查日志级别是否输出错误信息

### 问题3: 备份文件解密失败
```
decryption failed
```
**解决**:
1. 确认加密服务已正确初始化
2. 检查密钥长度是否为32字节
3. 验证备份文件未被损坏

---

## ✅ 验收标准

### 功能验收
- [ ] 删除视频时记录审计日志
- [ ] 重试任务时记录审计日志
- [ ] 上传视频时记录审计日志
- [ ] 刷新Cookie时记录审计日志
- [ ] 加密服务测试100%通过
- [ ] 审计日志测试100%通过
- [ ] 备份服务测试100%通过

### 性能验收
- [ ] 审计日志写入不影响主流程性能（<10ms）
- [ ] 高并发场景下日志不丢失（1000并发）
- [ ] 备份操作在后台执行，不阻塞启动

### 安全验收
- [ ] 日志中不包含明文密码
- [ ] 备份文件加密存储
- [ ] 审计日志不可篡改

---

## 📚 参考文档

- [Go测试最佳实践](https://golang.org/pkg/testing/)
- [Testify断言库](https://github.com/stretchr/testify)
- [GORM文档](https://gorm.io/docs/)
- [审计日志设计](../docs/week-2-security-setup.md)

---

**文档版本**: v1.0
**最后更新**: 2026-01-01
**维护者**: Claude Code
