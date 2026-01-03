package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/difyz9/ytb2bili/pkg/store/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ============================================================================
// 测试辅助函数
// ============================================================================

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	// 每个测试使用独立的内存数据库实例
	dbName := fmt.Sprintf("file:test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// 自动迁移
	err = db.AutoMigrate(&model.SavedVideo{}, &model.User{})
	require.NoError(t, err)

	return db
}

func createTestVideo(t *testing.T, db *gorm.DB, userID uint, videoID, status string) *model.SavedVideo {
	t.Helper()

	video := &model.SavedVideo{
		UserID:  userID,
		VideoID: videoID,
		Title:   "Test Video - " + videoID,
		Status:  status,
		URL:     "https://youtube.com/watch?v=" + videoID,
	}
	err := db.Create(video).Error
	require.NoError(t, err)
	return video
}

func createTestUser(t *testing.T, db *gorm.DB) *model.User {
	t.Helper()

	email := fmt.Sprintf("test_%d@example.com", time.Now().UnixNano())
	user := &model.User{
		Username:       "test_user",
		Email:          email,
		Password:       "hashed_password",
		Role:           "user",
		Status:         1,
		EmailVerified:  true,
		MembershipTier: "free",
	}
	err := db.Create(user).Error
	require.NoError(t, err)
	return user
}

// ============================================================================
// SavedVideoService 创建测试
// ============================================================================

func TestNewSavedVideoService(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	assert.NotNil(t, service)
	assert.Equal(t, db, service.DB)
}

// ============================================================================
// 基本 CRUD 操作测试
// ============================================================================

func TestSavedVideoService_CreateVideo(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	video := &model.SavedVideo{
		UserID:  1,
		VideoID: "abc123",
		Title:   "Test Video",
		Status:  "001",
		URL:     "https://youtube.com/watch?v=abc123",
	}

	err := service.CreateVideo(video)

	assert.NoError(t, err)
	assert.NotZero(t, video.ID)
}

func TestSavedVideoService_GetVideoByID(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	// 创建测试数据
	created := createTestVideo(t, db, 1, "test_video_id", "001")

	// 获取
	video, err := service.GetVideoByID(created.ID)

	assert.NoError(t, err)
	assert.NotNil(t, video)
	assert.Equal(t, created.ID, video.ID)
	assert.Equal(t, "test_video_id", video.VideoID)
}

func TestSavedVideoService_GetVideoByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	video, err := service.GetVideoByID(99999)

	assert.Error(t, err)
	assert.Nil(t, video)
}

func TestSavedVideoService_GetVideoByVideoID(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	created := createTestVideo(t, db, 1, "unique_video_id", "001")

	video, err := service.GetVideoByVideoID("unique_video_id")

	assert.NoError(t, err)
	assert.NotNil(t, video)
	assert.Equal(t, created.ID, video.ID)
}

func TestSavedVideoService_GetVideoByVideoID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	video, err := service.GetVideoByVideoID("nonexistent_video_id")

	assert.Error(t, err)
	assert.Nil(t, video)
}

func TestSavedVideoService_UpdateVideo(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	video := createTestVideo(t, db, 1, "update_test", "001")

	// 修改
	video.Title = "Updated Title"
	video.Status = "100"
	err := service.UpdateVideo(video)

	assert.NoError(t, err)

	// 验证
	updated, _ := service.GetVideoByID(video.ID)
	assert.Equal(t, "Updated Title", updated.Title)
	assert.Equal(t, "100", updated.Status)
}

func TestSavedVideoService_UpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	video := createTestVideo(t, db, 1, "status_test", "001")

	err := service.UpdateStatus(video.ID, "200")

	assert.NoError(t, err)

	// 验证
	updated, _ := service.GetVideoByID(video.ID)
	assert.Equal(t, "200", updated.Status)
}

func TestSavedVideoService_DeleteVideo(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	video := createTestVideo(t, db, 1, "delete_test", "001")

	err := service.DeleteVideo(video.ID)

	assert.NoError(t, err)

	// 验证已删除（软删除）
	deleted, err := service.GetVideoByID(video.ID)
	assert.Error(t, err)
	assert.Nil(t, deleted)
}

// ============================================================================
// 列表和分页测试
// ============================================================================

func TestSavedVideoService_GetPendingVideos(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	// 创建测试数据
	// 状态为 001 且有字幕 - 应该返回
	video1 := createTestVideo(t, db, 1, "pending_1", "001")
	db.Model(video1).Update("subtitles", "some_subtitle.srt")

	// 状态为 001 但无字幕 - 不应返回
	createTestVideo(t, db, 1, "pending_2", "001")

	// 状态不是 001 - 不应返回
	video3 := createTestVideo(t, db, 1, "not_pending", "200")
	db.Model(video3).Update("subtitles", "other.srt")

	videos, err := service.GetPendingVideos(10)

	assert.NoError(t, err)
	assert.Len(t, videos, 1)
	assert.Equal(t, "pending_1", videos[0].VideoID)
}

func TestSavedVideoService_ListVideos(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	// 创建测试数据
	for i := 0; i < 15; i++ {
		status := "001"
		if i%3 == 0 {
			status = "200"
		}
		createTestVideo(t, db, 1, fmt.Sprintf("list_video_%d", i), status)
	}

	// 测试分页
	videos, total, err := service.ListVideos(1, 10, "")

	assert.NoError(t, err)
	assert.Len(t, videos, 10)
	assert.Equal(t, int64(15), total)

	// 测试状态筛选
	videos, total, err = service.ListVideos(1, 10, "200")

	assert.NoError(t, err)
	assert.Equal(t, int64(5), total) // 0, 3, 6, 9, 12
}

func TestSavedVideoService_GetVideosPaginated(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	// 创建 5 个视频
	for i := 0; i < 5; i++ {
		createTestVideo(t, db, 1, fmt.Sprintf("paginated_%d", i), "001")
	}

	// 第一页
	videos, total, err := service.GetVideosPaginated(0, 3)

	assert.NoError(t, err)
	assert.Len(t, videos, 3)
	assert.Equal(t, 5, total)

	// 第二页
	videos, total, err = service.GetVideosPaginated(3, 3)

	assert.NoError(t, err)
	assert.Len(t, videos, 2)
	assert.Equal(t, 5, total)
}

func TestSavedVideoService_GetVideosByPlaylistID(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	// 创建属于同一播放列表的视频
	v1 := createTestVideo(t, db, 1, "playlist_1", "001")
	db.Model(v1).Update("playlist_id", "PL123")

	v2 := createTestVideo(t, db, 1, "playlist_2", "001")
	db.Model(v2).Update("playlist_id", "PL123")

	v3 := createTestVideo(t, db, 1, "other_playlist", "001")
	db.Model(v3).Update("playlist_id", "PL456")

	videos, err := service.GetVideosByPlaylistID("PL123")

	assert.NoError(t, err)
	assert.Len(t, videos, 2)
}

// ============================================================================
// 批量操作测试
// ============================================================================

func TestSavedVideoService_UpdateVideoStatus_Batch(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	// 创建多个视频
	v1 := createTestVideo(t, db, 1, "batch_1", "001")
	v2 := createTestVideo(t, db, 1, "batch_2", "001")
	v3 := createTestVideo(t, db, 1, "batch_3", "001")

	// 批量更新
	err := service.UpdateVideoStatus([]uint{v1.ID, v2.ID}, "200")

	assert.NoError(t, err)

	// 验证
	updated1, _ := service.GetVideoByID(v1.ID)
	updated2, _ := service.GetVideoByID(v2.ID)
	updated3, _ := service.GetVideoByID(v3.ID)

	assert.Equal(t, "200", updated1.Status)
	assert.Equal(t, "200", updated2.Status)
	assert.Equal(t, "001", updated3.Status) // 未更新
}

func TestSavedVideoService_UpdateVideoFields(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	video := createTestVideo(t, db, 1, "fields_test", "001")

	fields := map[string]interface{}{
		"title":  "New Title",
		"status": "300",
	}
	err := service.UpdateVideoFields(video.ID, fields)

	assert.NoError(t, err)

	// 验证
	updated, _ := service.GetVideoByID(video.ID)
	assert.Equal(t, "New Title", updated.Title)
	assert.Equal(t, "300", updated.Status)
}

// ============================================================================
// 用户隔离测试
// ============================================================================

func TestSavedVideoService_GetVideoByIDForUser(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	// 用户 1 的视频
	v1 := createTestVideo(t, db, 1, "user1_video", "001")
	// 用户 2 的视频
	v2 := createTestVideo(t, db, 2, "user2_video", "001")

	// 用户 1 访问自己的视频 - 成功
	video, err := service.GetVideoByIDForUser(v1.ID, 1)
	assert.NoError(t, err)
	assert.NotNil(t, video)

	// 用户 1 访问用户 2 的视频 - 失败
	video, err = service.GetVideoByIDForUser(v2.ID, 1)
	assert.Error(t, err)
	assert.Nil(t, video)

	// userID=0 时可以访问所有
	video, err = service.GetVideoByIDForUser(v2.ID, 0)
	assert.NoError(t, err)
	assert.NotNil(t, video)
}

func TestSavedVideoService_GetVideoByVideoIDForUser(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	// 用户 1 的视频
	createTestVideo(t, db, 1, "user1_only", "001")
	// 用户 2 的视频
	createTestVideo(t, db, 2, "user2_only", "001")

	// 用户 1 访问自己的视频 - 成功
	video, err := service.GetVideoByVideoIDForUser("user1_only", 1)
	assert.NoError(t, err)
	assert.NotNil(t, video)

	// 用户 1 访问用户 2 的视频 - 失败
	video, err = service.GetVideoByVideoIDForUser("user2_only", 1)
	assert.Error(t, err)
	assert.Nil(t, video)

	// userID=0 时可以访问所有
	video, err = service.GetVideoByVideoIDForUser("user2_only", 0)
	assert.NoError(t, err)
	assert.NotNil(t, video)
}

func TestSavedVideoService_GetVideosPaginatedForUser(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	// 创建用户 1 的 3 个视频
	for i := 0; i < 3; i++ {
		createTestVideo(t, db, 1, fmt.Sprintf("user1_video_%d", i), "001")
	}
	// 创建用户 2 的 2 个视频
	for i := 0; i < 2; i++ {
		createTestVideo(t, db, 2, fmt.Sprintf("user2_video_%d", i), "001")
	}

	// 用户 1 只能看到自己的 3 个
	videos, total, err := service.GetVideosPaginatedForUser(0, 10, 1)
	assert.NoError(t, err)
	assert.Len(t, videos, 3)
	assert.Equal(t, 3, total)

	// 用户 2 只能看到自己的 2 个
	videos, total, err = service.GetVideosPaginatedForUser(0, 10, 2)
	assert.NoError(t, err)
	assert.Len(t, videos, 2)
	assert.Equal(t, 2, total)

	// userID=0 可以看到所有 5 个
	videos, total, err = service.GetVideosPaginatedForUser(0, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, videos, 5)
	assert.Equal(t, 5, total)
}

func TestSavedVideoService_DeleteVideoForUser(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	// 用户 1 的视频
	v1 := createTestVideo(t, db, 1, "user1_delete", "001")
	// 用户 2 的视频
	v2 := createTestVideo(t, db, 2, "user2_delete", "001")

	// 用户 1 删除自己的视频 - 成功
	err := service.DeleteVideoForUser(v1.ID, 1)
	assert.NoError(t, err)

	// 用户 1 尝试删除用户 2 的视频 - 失败
	err = service.DeleteVideoForUser(v2.ID, 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "视频不存在或无权删除")

	// 验证用户 2 的视频仍然存在
	video, err := service.GetVideoByIDForUser(v2.ID, 2)
	assert.NoError(t, err)
	assert.NotNil(t, video)
}

// ============================================================================
// 别名方法测试
// ============================================================================

func TestSavedVideoService_GetByID(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	created := createTestVideo(t, db, 1, "alias_test", "001")

	// GetByID 应该与 GetVideoByID 行为一致
	video, err := service.GetByID(created.ID)

	assert.NoError(t, err)
	assert.NotNil(t, video)
	assert.Equal(t, created.ID, video.ID)
}

// ============================================================================
// 边界情况测试
// ============================================================================

func TestSavedVideoService_EmptyDatabase(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	// 空数据库的各种查询
	videos, err := service.GetPendingVideos(10)
	assert.NoError(t, err)
	assert.Empty(t, videos)

	videos, total, err := service.ListVideos(1, 10, "")
	assert.NoError(t, err)
	assert.Empty(t, videos)
	assert.Equal(t, int64(0), total)

	videos, totalInt, err := service.GetVideosPaginated(0, 10)
	assert.NoError(t, err)
	assert.Empty(t, videos)
	assert.Equal(t, 0, totalInt)
}

func TestSavedVideoService_LargeOffset(t *testing.T) {
	db := setupTestDB(t)
	service := NewSavedVideoService(db)

	// 创建一些视频
	for i := 0; i < 5; i++ {
		createTestVideo(t, db, 1, fmt.Sprintf("offset_%d", i), "001")
	}

	// 超出范围的 offset
	videos, total, err := service.GetVideosPaginated(100, 10)

	assert.NoError(t, err)
	assert.Empty(t, videos)
	assert.Equal(t, 5, total)
}

// ============================================================================
// 性能基准测试
// ============================================================================

func BenchmarkSavedVideoService_CreateVideo(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	db.AutoMigrate(&model.SavedVideo{})
	service := NewSavedVideoService(db)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		video := &model.SavedVideo{
			UserID:  1,
			VideoID: fmt.Sprintf("bench_%d", i),
			Title:   "Benchmark Video",
			Status:  "001",
			URL:     "https://youtube.com/watch?v=bench",
		}
		service.CreateVideo(video)
	}
}

func BenchmarkSavedVideoService_GetVideoByID(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	db.AutoMigrate(&model.SavedVideo{})

	// 预先创建一些数据
	for i := 0; i < 100; i++ {
		db.Create(&model.SavedVideo{
			UserID:  1,
			VideoID: fmt.Sprintf("bench_%d", i),
			Title:   "Benchmark Video",
			Status:  "001",
			URL:     "https://youtube.com/watch?v=bench",
		})
	}

	service := NewSavedVideoService(db)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.GetVideoByID(uint(i%100 + 1))
	}
}

func BenchmarkSavedVideoService_ListVideos(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	db.AutoMigrate(&model.SavedVideo{})

	// 预先创建数据
	for i := 0; i < 1000; i++ {
		db.Create(&model.SavedVideo{
			UserID:  1,
			VideoID: fmt.Sprintf("bench_%d", i),
			Title:   "Benchmark Video",
			Status:  "001",
			URL:     "https://youtube.com/watch?v=bench",
		})
	}

	service := NewSavedVideoService(db)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.ListVideos(1, 20, "")
	}
}
