package services

import (
	"encoding/json"
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

func setupTaskStepTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := fmt.Sprintf("file:test_taskstep_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	err = db.AutoMigrate(&model.TaskStep{}, &model.SavedVideo{}, &model.User{})
	require.NoError(t, err)

	return db
}

func createTaskStepTestVideo(t *testing.T, db *gorm.DB, userID uint, videoID string) *model.SavedVideo {
	t.Helper()

	video := &model.SavedVideo{
		UserID:  userID,
		VideoID: videoID,
		Title:   "Test Video - " + videoID,
		Status:  "001",
		URL:     "https://youtube.com/watch?v=" + videoID,
	}
	err := db.Create(video).Error
	require.NoError(t, err)
	return video
}

func createTestTaskStep(t *testing.T, db *gorm.DB, videoID, stepName, status string, order int) *model.TaskStep {
	t.Helper()

	step := &model.TaskStep{
		VideoID:   videoID,
		StepName:  stepName,
		StepOrder: order,
		Status:    status,
		CanRetry:  true,
	}
	err := db.Create(step).Error
	require.NoError(t, err)
	return step
}

// ============================================================================
// TaskStepService 创建测试
// ============================================================================

func TestNewTaskStepService(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	assert.NotNil(t, service)
	assert.Equal(t, db, service.DB)
}

// ============================================================================
// InitTaskSteps 测试
// ============================================================================

func TestTaskStepService_InitTaskSteps(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	videoID := "init_test_video"

	err := service.InitTaskSteps(videoID)
	assert.NoError(t, err)

	// 验证创建了 9 个步骤
	steps, err := service.GetTaskStepsByVideoID(videoID)
	assert.NoError(t, err)
	assert.Len(t, steps, 9)

	// 验证步骤顺序
	expectedSteps := []string{
		"获取元数据", "下载视频", "下载字幕", "下载封面", "翻译字幕",
		"AI增强元数据", "确认元数据", "上传到Bilibili", "上传字幕到Bilibili",
	}
	for i, step := range steps {
		assert.Equal(t, expectedSteps[i], step.StepName)
		assert.Equal(t, i+1, step.StepOrder)
		assert.Equal(t, model.TaskStepStatusWaiting, step.Status)
	}
}

func TestTaskStepService_InitTaskSteps_Idempotent(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	videoID := "idempotent_test"

	// 第一次初始化
	err := service.InitTaskSteps(videoID)
	assert.NoError(t, err)

	steps1, _ := service.GetTaskStepsByVideoID(videoID)

	// 第二次初始化（应该跳过已存在的步骤）
	err = service.InitTaskSteps(videoID)
	assert.NoError(t, err)

	steps2, _ := service.GetTaskStepsByVideoID(videoID)

	// 步骤数量应该不变
	assert.Len(t, steps1, 9)
	assert.Len(t, steps2, 9)
}

// ============================================================================
// GetTaskStepsByVideoID 测试
// ============================================================================

func TestTaskStepService_GetTaskStepsByVideoID(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	videoID := "get_steps_test"

	// 创建一些步骤
	createTestTaskStep(t, db, videoID, "步骤1", model.TaskStepStatusPending, 1)
	createTestTaskStep(t, db, videoID, "步骤2", model.TaskStepStatusCompleted, 2)
	createTestTaskStep(t, db, videoID, "步骤3", model.TaskStepStatusFailed, 3)

	steps, err := service.GetTaskStepsByVideoID(videoID)

	assert.NoError(t, err)
	assert.Len(t, steps, 3)
	// 验证按顺序返回
	assert.Equal(t, "步骤1", steps[0].StepName)
	assert.Equal(t, "步骤2", steps[1].StepName)
	assert.Equal(t, "步骤3", steps[2].StepName)
}

func TestTaskStepService_GetTaskStepsByVideoID_Empty(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	steps, err := service.GetTaskStepsByVideoID("nonexistent_video")

	assert.NoError(t, err)
	assert.Empty(t, steps)
}

// ============================================================================
// GetTaskStepsByVideoIDForUser 测试
// ============================================================================

func TestTaskStepService_GetTaskStepsByVideoIDForUser(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	// 创建用户 1 的视频
	video := createTaskStepTestVideo(t, db, 1, "user1_video")
	service.InitTaskSteps(video.VideoID)

	// 用户 1 访问自己的视频 - 成功
	steps, err := service.GetTaskStepsByVideoIDForUser(video.VideoID, 1)
	assert.NoError(t, err)
	assert.Len(t, steps, 9)

	// 用户 2 访问用户 1 的视频 - 失败
	steps, err = service.GetTaskStepsByVideoIDForUser(video.VideoID, 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "视频不存在或无权访问")
	assert.Nil(t, steps)

	// userID=0 可以访问所有
	steps, err = service.GetTaskStepsByVideoIDForUser(video.VideoID, 0)
	assert.NoError(t, err)
	assert.Len(t, steps, 9)
}

// ============================================================================
// UpdateTaskStepStatus 测试
// ============================================================================

func TestTaskStepService_UpdateTaskStepStatus_Running(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	videoID := "status_running_test"
	step := createTestTaskStep(t, db, videoID, "测试步骤", model.TaskStepStatusPending, 1)

	err := service.UpdateTaskStepStatus(videoID, step.StepName, model.TaskStepStatusRunning)
	assert.NoError(t, err)

	// 验证状态更新
	updated, _ := service.GetTaskStepByName(videoID, step.StepName)
	assert.Equal(t, model.TaskStepStatusRunning, updated.Status)
	assert.NotNil(t, updated.StartTime) // 开始时间应被设置
}

func TestTaskStepService_UpdateTaskStepStatus_Completed(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	videoID := "status_completed_test"
	step := createTestTaskStep(t, db, videoID, "测试步骤", model.TaskStepStatusRunning, 1)
	// 模拟开始时间
	startTime := time.Now().Add(-5 * time.Second)
	db.Model(step).Update("start_time", startTime)

	err := service.UpdateTaskStepStatus(videoID, step.StepName, model.TaskStepStatusCompleted)
	assert.NoError(t, err)

	updated, _ := service.GetTaskStepByName(videoID, step.StepName)
	assert.Equal(t, model.TaskStepStatusCompleted, updated.Status)
	assert.NotNil(t, updated.EndTime)
	assert.Greater(t, updated.Duration, int64(0)) // 时长应被计算
}

func TestTaskStepService_UpdateTaskStepStatus_Failed_WithError(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	videoID := "status_failed_test"
	step := createTestTaskStep(t, db, videoID, "测试步骤", model.TaskStepStatusRunning, 1)

	err := service.UpdateTaskStepStatus(videoID, step.StepName, model.TaskStepStatusFailed, "下载失败：网络超时")
	assert.NoError(t, err)

	updated, _ := service.GetTaskStepByName(videoID, step.StepName)
	assert.Equal(t, model.TaskStepStatusFailed, updated.Status)
	assert.Equal(t, "下载失败：网络超时", updated.ErrorMsg)
}

// ============================================================================
// UpdateTaskStepResult 测试
// ============================================================================

func TestTaskStepService_UpdateTaskStepResult(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	videoID := "result_test"
	step := createTestTaskStep(t, db, videoID, "测试步骤", model.TaskStepStatusCompleted, 1)

	resultData := map[string]interface{}{
		"file_path": "/tmp/video.mp4",
		"size":      12345678,
	}

	err := service.UpdateTaskStepResult(videoID, step.StepName, resultData)
	assert.NoError(t, err)

	updated, _ := service.GetTaskStepByName(videoID, step.StepName)
	assert.NotEmpty(t, updated.ResultData)

	// 验证 JSON 结构
	var parsed map[string]interface{}
	json.Unmarshal([]byte(updated.ResultData), &parsed)
	assert.Equal(t, "/tmp/video.mp4", parsed["file_path"])
}

// ============================================================================
// ResetTaskStep 测试
// ============================================================================

func TestTaskStepService_ResetTaskStep(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	videoID := "reset_test"
	step := createTestTaskStep(t, db, videoID, "测试步骤", model.TaskStepStatusFailed, 1)
	// 设置一些数据
	now := time.Now()
	db.Model(step).Updates(map[string]interface{}{
		"start_time":  now,
		"end_time":    now,
		"duration":    5000,
		"error_msg":   "some error",
		"result_data": `{"key":"value"}`,
	})

	err := service.ResetTaskStep(videoID, step.StepName)
	assert.NoError(t, err)

	updated, _ := service.GetTaskStepByName(videoID, step.StepName)
	assert.Equal(t, model.TaskStepStatusPending, updated.Status)
	assert.Nil(t, updated.StartTime)
	assert.Nil(t, updated.EndTime)
	assert.Equal(t, int64(0), updated.Duration)
	assert.Empty(t, updated.ErrorMsg)
	assert.Empty(t, updated.ResultData)
}

// ============================================================================
// GetTaskStepByName 测试
// ============================================================================

func TestTaskStepService_GetTaskStepByName(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	videoID := "get_by_name_test"
	createTestTaskStep(t, db, videoID, "下载视频", model.TaskStepStatusCompleted, 1)

	step, err := service.GetTaskStepByName(videoID, "下载视频")

	assert.NoError(t, err)
	assert.NotNil(t, step)
	assert.Equal(t, "下载视频", step.StepName)
}

func TestTaskStepService_GetTaskStepByName_NotFound(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	step, err := service.GetTaskStepByName("nonexistent", "不存在的步骤")

	assert.Error(t, err)
	assert.Nil(t, step)
}

// ============================================================================
// GetTaskProgress 测试
// ============================================================================

func TestTaskStepService_GetTaskProgress(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	videoID := "progress_test"
	createTestTaskStep(t, db, videoID, "步骤1", model.TaskStepStatusCompleted, 1)
	createTestTaskStep(t, db, videoID, "步骤2", model.TaskStepStatusCompleted, 2)
	createTestTaskStep(t, db, videoID, "步骤3", model.TaskStepStatusRunning, 3)
	createTestTaskStep(t, db, videoID, "步骤4", model.TaskStepStatusPending, 4)
	createTestTaskStep(t, db, videoID, "步骤5", model.TaskStepStatusFailed, 5)

	progress, err := service.GetTaskProgress(videoID)

	assert.NoError(t, err)
	assert.Equal(t, 5, progress["total_steps"])
	assert.Equal(t, 2, progress["completed_steps"])
	assert.Equal(t, 1, progress["failed_steps"])
	assert.Equal(t, "步骤3", progress["current_step"])
	assert.Equal(t, 40, progress["progress_percent"]) // 2/5 = 40%
}

func TestTaskStepService_GetTaskProgress_Empty(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	progress, err := service.GetTaskProgress("empty_video")

	assert.NoError(t, err)
	assert.Equal(t, 0, progress["total_steps"])
	assert.Equal(t, 0, progress["progress_percent"])
}

// ============================================================================
// ResetAllRunningTasks 测试
// ============================================================================

func TestTaskStepService_ResetAllRunningTasks(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	// 创建运行中的任务步骤
	createTestTaskStep(t, db, "video1", "步骤1", model.TaskStepStatusRunning, 1)
	createTestTaskStep(t, db, "video2", "步骤2", model.TaskStepStatusRunning, 1)
	createTestTaskStep(t, db, "video3", "步骤3", model.TaskStepStatusCompleted, 1) // 不应被重置

	// 创建处理中的视频
	video1 := createTaskStepTestVideo(t, db, 1, "video1")
	db.Model(video1).Update("status", "002")
	video2 := createTaskStepTestVideo(t, db, 1, "video2")
	db.Model(video2).Update("status", "002")
	video3 := createTaskStepTestVideo(t, db, 1, "video3")
	db.Model(video3).Update("status", "200") // 不应被重置

	err := service.ResetAllRunningTasks()
	assert.NoError(t, err)

	// 验证任务步骤被重置
	step1, _ := service.GetTaskStepByName("video1", "步骤1")
	step2, _ := service.GetTaskStepByName("video2", "步骤2")
	step3, _ := service.GetTaskStepByName("video3", "步骤3")

	assert.Equal(t, model.TaskStepStatusPending, step1.Status)
	assert.Equal(t, model.TaskStepStatusPending, step2.Status)
	assert.Equal(t, model.TaskStepStatusCompleted, step3.Status) // 未变

	// 验证视频状态被重置
	var v1, v2, v3 model.SavedVideo
	db.Where("video_id = ?", "video1").First(&v1)
	db.Where("video_id = ?", "video2").First(&v2)
	db.Where("video_id = ?", "video3").First(&v3)

	assert.Equal(t, "001", v1.Status)
	assert.Equal(t, "001", v2.Status)
	assert.Equal(t, "200", v3.Status) // 未变
}

// ============================================================================
// GetPendingSteps 测试
// ============================================================================

func TestTaskStepService_GetPendingSteps(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	// 创建有效的视频和步骤
	createTaskStepTestVideo(t, db, 1, "pending_video")
	createTestTaskStep(t, db, "pending_video", "待执行步骤", model.TaskStepStatusPending, 1)
	createTestTaskStep(t, db, "pending_video", "已完成步骤", model.TaskStepStatusCompleted, 2)

	steps, err := service.GetPendingSteps()

	assert.NoError(t, err)
	assert.Len(t, steps, 1)
	assert.Equal(t, "待执行步骤", steps[0].StepName)
}

// ============================================================================
// DeleteTaskStepsByVideoID 测试
// ============================================================================

func TestTaskStepService_DeleteTaskStepsByVideoID(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	videoID := "delete_steps_test"
	createTestTaskStep(t, db, videoID, "步骤1", model.TaskStepStatusCompleted, 1)
	createTestTaskStep(t, db, videoID, "步骤2", model.TaskStepStatusCompleted, 2)

	err := service.DeleteTaskStepsByVideoID(videoID)
	assert.NoError(t, err)

	// 验证被软删除
	steps, _ := service.GetTaskStepsByVideoID(videoID)
	assert.Empty(t, steps)
}

// ============================================================================
// ResetFailedSteps 测试
// ============================================================================

func TestTaskStepService_ResetFailedSteps(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	videoID := "reset_failed_test"
	createTestTaskStep(t, db, videoID, "失败步骤1", model.TaskStepStatusFailed, 1)
	createTestTaskStep(t, db, videoID, "失败步骤2", model.TaskStepStatusFailed, 2)
	createTestTaskStep(t, db, videoID, "跳过步骤", model.TaskStepStatusSkipped, 3)
	createTestTaskStep(t, db, videoID, "已完成步骤", model.TaskStepStatusCompleted, 4)

	count, err := service.ResetFailedSteps(videoID)

	assert.NoError(t, err)
	assert.Equal(t, int64(3), count) // 2个失败 + 1个跳过

	// 验证状态
	step1, _ := service.GetTaskStepByName(videoID, "失败步骤1")
	step2, _ := service.GetTaskStepByName(videoID, "失败步骤2")
	step3, _ := service.GetTaskStepByName(videoID, "跳过步骤")
	step4, _ := service.GetTaskStepByName(videoID, "已完成步骤")

	assert.Equal(t, model.TaskStepStatusPending, step1.Status)
	assert.Equal(t, model.TaskStepStatusPending, step2.Status)
	assert.Equal(t, model.TaskStepStatusPending, step3.Status)
	assert.Equal(t, model.TaskStepStatusCompleted, step4.Status) // 未变
}

// ============================================================================
// GetFailedOrSkippedSteps 测试
// ============================================================================

func TestTaskStepService_GetFailedOrSkippedSteps(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	videoID := "failed_skipped_test"
	createTestTaskStep(t, db, videoID, "失败步骤", model.TaskStepStatusFailed, 1)
	createTestTaskStep(t, db, videoID, "跳过步骤", model.TaskStepStatusSkipped, 2)
	createTestTaskStep(t, db, videoID, "已完成步骤", model.TaskStepStatusCompleted, 3)
	createTestTaskStep(t, db, videoID, "待执行步骤", model.TaskStepStatusPending, 4)

	steps, err := service.GetFailedOrSkippedSteps(videoID)

	assert.NoError(t, err)
	assert.Len(t, steps, 2)
	assert.Equal(t, "失败步骤", steps[0].StepName)
	assert.Equal(t, "跳过步骤", steps[1].StepName)
}

// ============================================================================
// GetCompletedStepNames 测试
// ============================================================================

func TestTaskStepService_GetCompletedStepNames(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	videoID := "completed_names_test"
	createTestTaskStep(t, db, videoID, "下载视频", model.TaskStepStatusCompleted, 1)
	createTestTaskStep(t, db, videoID, "下载字幕", model.TaskStepStatusCompleted, 2)
	createTestTaskStep(t, db, videoID, "上传视频", model.TaskStepStatusRunning, 3)

	names, err := service.GetCompletedStepNames(videoID)

	assert.NoError(t, err)
	assert.Len(t, names, 2)
	assert.Contains(t, names, "下载视频")
	assert.Contains(t, names, "下载字幕")
}

// ============================================================================
// GetRunningStepNames 测试
// ============================================================================

func TestTaskStepService_GetRunningStepNames(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	videoID := "running_names_test"
	createTestTaskStep(t, db, videoID, "下载视频", model.TaskStepStatusCompleted, 1)
	createTestTaskStep(t, db, videoID, "上传视频", model.TaskStepStatusRunning, 2)
	createTestTaskStep(t, db, videoID, "上传字幕", model.TaskStepStatusRunning, 3)

	names, err := service.GetRunningStepNames(videoID)

	assert.NoError(t, err)
	assert.Len(t, names, 2)
	assert.Contains(t, names, "上传视频")
	assert.Contains(t, names, "上传字幕")
}

// ============================================================================
// EnsureSubtitleUploadStep 测试
// ============================================================================

func TestTaskStepService_EnsureSubtitleUploadStep(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	videoID := "ensure_subtitle_test"

	// 第一次应该创建步骤
	err := service.EnsureSubtitleUploadStep(videoID)
	assert.NoError(t, err)

	step, _ := service.GetTaskStepByName(videoID, "上传字幕到Bilibili")
	assert.NotNil(t, step)
	assert.Equal(t, 8, step.StepOrder)

	// 第二次调用应该跳过（幂等）
	err = service.EnsureSubtitleUploadStep(videoID)
	assert.NoError(t, err)

	// 仍然只有一个步骤
	var count int64
	db.Model(&model.TaskStep{}).Where("video_id = ? AND step_name = ?", videoID, "上传字幕到Bilibili").Count(&count)
	assert.Equal(t, int64(1), count)
}

// ============================================================================
// 边界情况测试
// ============================================================================

func TestTaskStepService_UpdateStatus_NonExistentStep(t *testing.T) {
	db := setupTaskStepTestDB(t)
	service := NewTaskStepService(db)

	// 更新不存在的步骤不应报错（只是 RowsAffected=0）
	err := service.UpdateTaskStepStatus("nonexistent", "不存在", model.TaskStepStatusCompleted)
	assert.NoError(t, err)
}

// ============================================================================
// 性能基准测试
// ============================================================================

func BenchmarkTaskStepService_InitTaskSteps(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	db.AutoMigrate(&model.TaskStep{})
	service := NewTaskStepService(db)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.InitTaskSteps(fmt.Sprintf("bench_video_%d", i))
	}
}

func BenchmarkTaskStepService_GetTaskStepsByVideoID(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	db.AutoMigrate(&model.TaskStep{})
	service := NewTaskStepService(db)

	// 预先创建数据
	service.InitTaskSteps("bench_video")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.GetTaskStepsByVideoID("bench_video")
	}
}

func BenchmarkTaskStepService_UpdateTaskStepStatus(b *testing.B) {
	db, _ := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	db.AutoMigrate(&model.TaskStep{})
	service := NewTaskStepService(db)
	service.InitTaskSteps("bench_video")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.UpdateTaskStepStatus("bench_video", "下载视频", model.TaskStepStatusCompleted)
	}
}
