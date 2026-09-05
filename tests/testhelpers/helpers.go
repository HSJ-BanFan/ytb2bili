// Package testhelpers 提供测试辅助函数
// 供所有单元测试和集成测试使用
package testhelpers

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/difyz9/ytb2bili/pkg/audit"
	"github.com/difyz9/ytb2bili/pkg/crypto"
	"github.com/difyz9/ytb2bili/pkg/store/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ============================================================================
// TestContext - 测试上下文容器
// ============================================================================

// TestContext 测试上下文，包含所有测试所需的依赖
type TestContext struct {
	T             *testing.T
	DB            *gorm.DB
	TempDir       string
	EncryptionSvc *crypto.EncryptionService
	AuditSvc      *audit.AuditService
	CleanupFuncs  []func()
}

// Setup 创建新的测试上下文
// 使用方法：
//
//	ctx := testhelpers.Setup(t)
//	defer ctx.Cleanup()
func Setup(t *testing.T) *TestContext {
	t.Helper()

	// 1. 创建临时目录
	tempDir, err := os.MkdirTemp("", "ytb2bili_test_*")
	require.NoError(t, err, "创建临时目录失败")

	// 2. 创建内存数据库
	db := SetupTestDB(t)

	// 3. 初始化加密服务
	os.Setenv("COOKIE_ENCRYPTION_KEY", "test_32_byte_encryption_key_1234")
	encSvc, err := crypto.NewEncryptionService("test_32_byte_encryption_key_1234")
	require.NoError(t, err, "初始化加密服务失败")

	// 4. 初始化审计服务
	auditSvc := audit.NewAuditService(db)

	ctx := &TestContext{
		T:             t,
		DB:            db,
		TempDir:       tempDir,
		EncryptionSvc: encSvc,
		AuditSvc:      auditSvc,
		CleanupFuncs:  make([]func(), 0),
	}

	// 添加默认清理函数
	ctx.CleanupFuncs = append(ctx.CleanupFuncs, func() {
		auditSvc.Close()
		os.Unsetenv("COOKIE_ENCRYPTION_KEY")
		os.RemoveAll(tempDir)
	})

	return ctx
}

// Cleanup 清理测试资源
func (ctx *TestContext) Cleanup() {
	for i := len(ctx.CleanupFuncs) - 1; i >= 0; i-- {
		ctx.CleanupFuncs[i]()
	}
}

// AddCleanup 添加清理函数
func (ctx *TestContext) AddCleanup(fn func()) {
	ctx.CleanupFuncs = append(ctx.CleanupFuncs, fn)
}

// ============================================================================
// 数据库相关
// ============================================================================

// SetupTestDB 创建内存 SQLite 测试数据库
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err, "打开测试数据库失败")

	// 自动迁移所有模型
	err = db.AutoMigrate(
		&model.SavedVideo{},
		&model.TaskStep{},
		&model.UserBiliAccount{},
		&model.AuditLog{},
	)
	require.NoError(t, err, "数据库迁移失败")

	return db
}

// ============================================================================
// 视频相关
// ============================================================================

// CreateTestVideo 创建测试视频
func (ctx *TestContext) CreateTestVideo(opts ...VideoOption) *model.SavedVideo {
	ctx.T.Helper()

	video := &model.SavedVideo{
		VideoID:     "test_" + uuid.New().String()[:8],
		URL:         "https://www.youtube.com/watch?v=test123",
		Title:       "测试视频标题",
		Description: "这是一个测试视频描述",
		Status:      "100", // 初始状态
	}

	// 应用选项
	for _, opt := range opts {
		opt(video)
	}

	err := ctx.DB.Create(video).Error
	require.NoError(ctx.T, err, "创建测试视频失败")

	return video
}

// VideoOption 视频选项函数类型
type VideoOption func(*model.SavedVideo)

// WithVideoID 设置视频ID
func WithVideoID(videoID string) VideoOption {
	return func(v *model.SavedVideo) {
		v.VideoID = videoID
	}
}

// WithStatus 设置视频状态
func WithStatus(status string) VideoOption {
	return func(v *model.SavedVideo) {
		v.Status = status
	}
}

// WithTitle 设置视频标题
func WithTitle(title string) VideoOption {
	return func(v *model.SavedVideo) {
		v.Title = title
	}
}

// WithSubtitles 设置字幕
func WithSubtitles(subtitles string) VideoOption {
	return func(v *model.SavedVideo) {
		v.Subtitles = subtitles
	}
}

// WithBiliInfo 设置B站上传信息
func WithBiliInfo(bvid string, aid int64) VideoOption {
	return func(v *model.SavedVideo) {
		v.BiliBVID = bvid
		v.BiliAID = aid
	}
}

// ============================================================================
// B站账号相关
// ============================================================================

// CreateTestBiliAccount 创建测试B站账号
func (ctx *TestContext) CreateTestBiliAccount(opts ...BiliAccountOption) *model.UserBiliAccount {
	ctx.T.Helper()

	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	account := &model.UserBiliAccount{
		BiliMid:   123456789,
		BiliName:  "测试B站账号",
		Cookies:   "test_cookies_" + uuid.New().String()[:8],
		ExpiresAt: &expiresAt,
		IsEnabled: true,
		IsPrimary: true,
	}

	// 应用选项
	for _, opt := range opts {
		opt(account)
	}

	err := ctx.DB.Create(account).Error
	require.NoError(ctx.T, err, "创建测试B站账号失败")

	return account
}

// BiliAccountOption B站账号选项函数类型
type BiliAccountOption func(*model.UserBiliAccount)

// WithBiliMid 设置B站MID
func WithBiliMid(mid int64) BiliAccountOption {
	return func(a *model.UserBiliAccount) {
		a.BiliMid = mid
	}
}

// WithBiliName 设置B站用户名
func WithBiliName(name string) BiliAccountOption {
	return func(a *model.UserBiliAccount) {
		a.BiliName = name
	}
}

// WithPrimary 设置是否为主账号
func WithPrimary(isPrimary bool) BiliAccountOption {
	return func(a *model.UserBiliAccount) {
		a.IsPrimary = isPrimary
	}
}

// WithEnabled 设置是否启用
func WithEnabled(isEnabled bool) BiliAccountOption {
	return func(a *model.UserBiliAccount) {
		a.IsEnabled = isEnabled
	}
}

// ============================================================================
// TaskStep 相关
// ============================================================================

// CreateTestTaskStep 创建测试任务步骤
func (ctx *TestContext) CreateTestTaskStep(videoID string, stepName string, opts ...TaskStepOption) *model.TaskStep {
	ctx.T.Helper()

	step := &model.TaskStep{
		VideoID:  videoID,
		StepName: stepName,
		Status:   "pending",
	}

	// 应用选项
	for _, opt := range opts {
		opt(step)
	}

	err := ctx.DB.Create(step).Error
	require.NoError(ctx.T, err, "创建测试任务步骤失败")

	return step
}

// TaskStepOption 任务步骤选项函数类型
type TaskStepOption func(*model.TaskStep)

// WithTaskStatus 设置任务状态
func WithTaskStatus(status string) TaskStepOption {
	return func(ts *model.TaskStep) {
		ts.Status = status
	}
}

// ============================================================================
// 文件和目录相关
// ============================================================================

// CreateTestVideoDir 创建测试视频目录结构
// 返回目录路径
func (ctx *TestContext) CreateTestVideoDir(videoID string) string {
	ctx.T.Helper()

	dateStr := time.Now().Format("2006-01-02")
	videoDir := filepath.Join(ctx.TempDir, dateStr, videoID)

	err := os.MkdirAll(videoDir, 0755)
	require.NoError(ctx.T, err, "创建测试视频目录失败")

	return videoDir
}

// CreateTestFile 在指定目录创建测试文件
func (ctx *TestContext) CreateTestFile(dir, filename, content string) string {
	ctx.T.Helper()

	filePath := filepath.Join(dir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(ctx.T, err, "创建测试文件失败")

	return filePath
}

// ============================================================================
// 断言辅助函数
// ============================================================================

// AssertVideoStatus 验证视频状态
func (ctx *TestContext) AssertVideoStatus(videoID string, expectedStatus string) {
	ctx.T.Helper()

	var video model.SavedVideo
	err := ctx.DB.Where("video_id = ?", videoID).First(&video).Error
	require.NoError(ctx.T, err, "查询视频失败")
	require.Equal(ctx.T, expectedStatus, video.Status, "视频状态不正确")
}

// ============================================================================
// 等待和轮询辅助函数
// ============================================================================

// WaitForCondition 等待条件满足（带超时）
func WaitForCondition(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("等待超时: %s", message)
}

// WaitForAuditLog 等待审计日志写入
func (ctx *TestContext) WaitForAuditLog(timeout time.Duration, minCount int) int {
	ctx.T.Helper()

	var count int64
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		ctx.DB.Model(&model.AuditLog{}).Count(&count)
		if int(count) >= minCount {
			return int(count)
		}
		time.Sleep(100 * time.Millisecond)
	}

	return int(count)
}

// ============================================================================
// 清理辅助函数
// ============================================================================

// ClearTable 清空指定表
func (ctx *TestContext) ClearTable(tableName string) {
	ctx.T.Helper()
	ctx.DB.Exec(fmt.Sprintf("DELETE FROM %s", tableName))
}

// ClearAllData 清空所有测试数据
func (ctx *TestContext) ClearAllData() {
	ctx.T.Helper()

	tables := []string{
		"cw_audit_logs",
		"cw_user_bili_accounts",
		"cw_task_steps",
		"cw_saved_videos",
	}

	for _, table := range tables {
		ctx.DB.Exec(fmt.Sprintf("DELETE FROM %s", table))
	}
}
