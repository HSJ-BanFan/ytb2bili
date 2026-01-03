package testhelpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetup 测试 Setup 函数
func TestSetup(t *testing.T) {
	ctx := Setup(t)
	defer ctx.Cleanup()

	assert.NotNil(t, ctx.DB)
	assert.NotNil(t, ctx.EncryptionSvc)
	assert.NotNil(t, ctx.AuditSvc)
	assert.NotNil(t, ctx.JWTService)
	assert.NotEmpty(t, ctx.TempDir)
}

// TestCreateTestUser 测试用户创建
func TestCreateTestUser(t *testing.T) {
	ctx := Setup(t)
	defer ctx.Cleanup()

	// 创建默认用户
	user1 := ctx.CreateTestUser()
	assert.NotZero(t, user1.ID)
	assert.Contains(t, user1.Username, "test_user_")
	assert.Equal(t, "user", user1.Role)
	assert.Equal(t, "free", user1.MembershipTier)

	// 创建自定义用户
	user2 := ctx.CreateTestUser(
		WithUsername("custom_user"),
		WithRole("admin"),
		WithMembershipTier("pro"),
	)
	assert.Equal(t, "custom_user", user2.Username)
	assert.Equal(t, "admin", user2.Role)
	assert.Equal(t, "pro", user2.MembershipTier)

	// 验证两个用户ID不同
	assert.NotEqual(t, user1.ID, user2.ID)
}

// TestCreateTestVideo 测试视频创建
func TestCreateTestVideo(t *testing.T) {
	ctx := Setup(t)
	defer ctx.Cleanup()

	user := ctx.CreateTestUser()

	// 创建默认视频
	video := ctx.CreateTestVideo(user.ID)
	assert.NotZero(t, video.ID)
	assert.Equal(t, user.ID, video.UserID)
	assert.Contains(t, video.VideoID, "test_")
	assert.Equal(t, "100", video.Status)

	// 创建自定义视频
	video2 := ctx.CreateTestVideo(user.ID,
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

	user := ctx.CreateTestUser()

	// 创建默认B站账号
	account := ctx.CreateTestBiliAccount(user.ID)
	assert.NotZero(t, account.ID)
	assert.Equal(t, user.ID, account.UserID)
	assert.True(t, account.IsEnabled)
	assert.True(t, account.IsPrimary)

	// 创建自定义B站账号
	account2 := ctx.CreateTestBiliAccount(user.ID,
		WithBiliMid(987654321),
		WithBiliName("自定义账号"),
		WithPrimary(false),
	)
	assert.Equal(t, int64(987654321), account2.BiliMid)
	assert.Equal(t, "自定义账号", account2.BiliName)
	assert.False(t, account2.IsPrimary)
}

// TestGenerateTestToken 测试 JWT Token 生成
func TestGenerateTestToken(t *testing.T) {
	ctx := Setup(t)
	defer ctx.Cleanup()

	user := ctx.CreateTestUser()

	// 生成 Token
	token := ctx.GenerateTestToken(user)
	assert.NotEmpty(t, token)

	// 解析 Token 验证
	claims, err := ctx.JWTService.ParseToken(token)
	require.NoError(t, err)
	assert.Equal(t, user.ID, claims.UserID)
	assert.Equal(t, user.Username, claims.Username)
}

// TestUserIsolation 测试用户隔离场景
func TestUserIsolation(t *testing.T) {
	ctx := Setup(t)
	defer ctx.Cleanup()

	// 创建两个用户
	userA := ctx.CreateTestUser(WithUsername("user_a"))
	userB := ctx.CreateTestUser(WithUsername("user_b"))

	// 创建 userA 的视频
	videoA := ctx.CreateTestVideo(userA.ID, WithVideoID("video_a_123"))

	// 验证视频属于 userA
	assert.Equal(t, userA.ID, videoA.UserID)

	// 验证 userB 不应该能访问 userA 的视频
	// （这个断言在实际 Service 层测试中会更有意义）
	assert.NotEqual(t, userB.ID, videoA.UserID)
}

// TestCreateTestVideoDir 测试目录创建
func TestCreateTestVideoDir(t *testing.T) {
	ctx := Setup(t)
	defer ctx.Cleanup()

	user := ctx.CreateTestUser()
	videoDir := ctx.CreateTestVideoDir(user.ID, "test_video_123")

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

	user := ctx.CreateTestUser()
	video := ctx.CreateTestVideo(user.ID, WithStatus("200"))

	// 验证状态
	ctx.AssertVideoStatus(video.VideoID, "200")
}

// TestClearAllData 测试数据清理
func TestClearAllData(t *testing.T) {
	ctx := Setup(t)
	defer ctx.Cleanup()

	// 创建一些数据
	user := ctx.CreateTestUser()
	ctx.CreateTestVideo(user.ID)
	ctx.CreateTestBiliAccount(user.ID)

	// 清理所有数据
	ctx.ClearAllData()

	// 验证数据已清空
	var count int64
	ctx.DB.Table("cw_users").Count(&count)
	assert.Equal(t, int64(0), count)

	ctx.DB.Table("cw_saved_videos").Count(&count)
	assert.Equal(t, int64(0), count)
}
