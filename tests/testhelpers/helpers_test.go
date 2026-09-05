package testhelpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSetup 测试 Setup 函数
func TestSetup(t *testing.T) {
	ctx := Setup(t)
	defer ctx.Cleanup()

	assert.NotNil(t, ctx.DB)
	assert.NotNil(t, ctx.EncryptionSvc)
	assert.NotNil(t, ctx.AuditSvc)
	assert.NotEmpty(t, ctx.TempDir)
}

// TestCreateTestVideo 测试视频创建
func TestCreateTestVideo(t *testing.T) {
	ctx := Setup(t)
	defer ctx.Cleanup()

	// 创建默认视频
	video := ctx.CreateTestVideo()
	assert.NotZero(t, video.ID)
	assert.Contains(t, video.VideoID, "test_")
	assert.Equal(t, "100", video.Status)

	// 创建自定义视频
	video2 := ctx.CreateTestVideo(
		WithVideoID("custom_video_123"),
		WithStatus("200"),
		WithTitle("自定义标题"),
	)
	assert.Equal(t, "custom_video_123", video2.VideoID)
	assert.Equal(t, "200", video2.Status)
	assert.Equal(t, "自定义标题", video2.Title)
}

// TestCreateTestBiliAccount 测试B站账号创建
func TestCreateTestBiliAccount(t *testing.T) {
	ctx := Setup(t)
	defer ctx.Cleanup()

	// 创建默认B站账号
	account := ctx.CreateTestBiliAccount()
	assert.NotZero(t, account.ID)
	assert.True(t, account.IsEnabled)
	assert.True(t, account.IsPrimary)

	// 创建自定义B站账号
	account2 := ctx.CreateTestBiliAccount(
		WithBiliMid(987654321),
		WithBiliName("自定义账号"),
		WithPrimary(false),
	)
	assert.Equal(t, int64(987654321), account2.BiliMid)
	assert.Equal(t, "自定义账号", account2.BiliName)
	assert.False(t, account2.IsPrimary)
}

// TestCreateTestVideoDir 测试目录创建
func TestCreateTestVideoDir(t *testing.T) {
	ctx := Setup(t)
	defer ctx.Cleanup()

	videoDir := ctx.CreateTestVideoDir("test_video_123")

	// 验证目录存在
	assert.DirExists(t, videoDir)

	// 创建测试文件
	filePath := ctx.CreateTestFile(videoDir, "test.txt", "hello world")
	assert.FileExists(t, filePath)
}

// TestAssertVideoStatus 测试状态断言
func TestAssertVideoStatus(t *testing.T) {
	ctx := Setup(t)
	defer ctx.Cleanup()

	video := ctx.CreateTestVideo(WithStatus("200"))

	// 验证状态
	ctx.AssertVideoStatus(video.VideoID, "200")
}

// TestClearAllData 测试数据清理
func TestClearAllData(t *testing.T) {
	ctx := Setup(t)
	defer ctx.Cleanup()

	// 创建一些数据
	ctx.CreateTestVideo()
	ctx.CreateTestBiliAccount()

	// 清理所有数据
	ctx.ClearAllData()

	// 验证数据已清空
	var count int64

	ctx.DB.Table("cw_saved_videos").Count(&count)
	assert.Equal(t, int64(0), count)
}
