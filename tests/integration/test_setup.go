package integration

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/difyz9/ytb2bili/pkg/audit"
	"github.com/difyz9/ytb2bili/pkg/crypto"
	"github.com/difyz9/ytb2bili/pkg/store/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestApp 测试应用容器
type TestApp struct {
	DB            *gorm.DB
	EncryptionSvc *crypto.EncryptionService
	AuditSvc      *audit.AuditService
	TempDir       string
	CleanupFunc   func()
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

	// 关闭连接以便清理
	sqlDB, _ := db.DB()

	// 3. 自动迁移表结构
	err = db.AutoMigrate(
		&model.SavedVideo{},
		&model.TaskStep{},
		&model.UserBiliAccount{},
		&model.AuditLog{},
	)
	if err != nil {
		t.Fatalf("数据库迁移失败: %v", err)
	}

	// 4. 初始化加密服务
	os.Setenv("COOKIE_ENCRYPTION_KEY", "test_32_byte_encryption_key_1234")
	encSvc, err := crypto.NewEncryptionService("test_32_byte_encryption_key_1234")
	if err != nil {
		t.Fatalf("初始化加密服务失败: %v", err)
	}

	// 5. 初始化审计服务
	auditSvc := audit.NewAuditService(db)

	// 7. 清理函数
	cleanup := func() {
		auditSvc.Close() // 关闭审计服务
		sqlDB.Close()    // 关闭数据库连接
		os.Unsetenv("COOKIE_ENCRYPTION_KEY")
		os.RemoveAll(tempDir)
	}

	return &TestApp{
		DB:            db,
		EncryptionSvc: encSvc,
		AuditSvc:      auditSvc,
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
func (app *TestApp) CreateTestVideo(t *testing.T) *model.SavedVideo {
	video := &model.SavedVideo{
		VideoID:     fmt.Sprintf("test_%d", time.Now().UnixNano()),
		Title:       "测试视频",
		Description: "这是一个测试视频",
		Status:      "200", // 准备上传
	}
	if err := app.DB.Create(video).Error; err != nil {
		t.Fatalf("创建测试视频失败: %v", err)
	}
	return video
}

// CreateTestAccount 创建测试B站账号
func (app *TestApp) CreateTestAccount(t *testing.T) *model.UserBiliAccount {
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	account := &model.UserBiliAccount{
		BiliMid:   123456789,
		BiliName:  "测试账号",
		Cookies:   "test_cookies",
		ExpiresAt: &expiresAt,
		IsPrimary: true,
	}
	if err := app.DB.Create(account).Error; err != nil {
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

	return int(count)
}

// ClearAuditLogs 清空审计日志
func (app *TestApp) ClearAuditLogs() {
	app.DB.Exec("DELETE FROM cw_audit_logs")
}
