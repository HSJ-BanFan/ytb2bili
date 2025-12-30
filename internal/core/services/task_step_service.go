package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/difyz9/ytb2bili/pkg/store/model"

	"gorm.io/gorm"
)

// TaskStepService 任务步骤服务
type TaskStepService struct {
	DB *gorm.DB
}

// NewTaskStepService 创建任务步骤服务实例
func NewTaskStepService(db *gorm.DB) *TaskStepService {
	return &TaskStepService{
		DB: db,
	}
}

// InitTaskSteps 初始化视频的任务步骤
func (s *TaskStepService) InitTaskSteps(videoID string) error {
	// 定义标准任务步骤（按执行顺序）
	// ┌────┬──────────────────┬────────────────────┬──────┬────────────┐
	// │序号│      任务名      │        依赖        │ 必需 │  失败处理  │
	// ├────┼──────────────────┼────────────────────┼──────┼────────────┤
	// │ 1  │ 获取元数据       │        无          │  是  │  终止链    │
	// │ 2  │ 下载视频         │        无          │  是  │  终止链    │
	// │ 3  │ 下载字幕         │        无          │  否  │  继续执行  │
	// │ 4  │ 下载封面         │    获取元数据      │  是  │  终止链    │
	// │ 5  │ 翻译字幕         │      下载字幕      │  否  │    跳过    │
	// │ 6  │ AI增强元数据     │ 下载视频,翻译字幕  │  否  │    跳过    │
	// │ 7  │ 确认元数据       │   AI增强元数据     │  否  │  使用默认  │
	// │ 8  │ 上传到Bilibili   │    确认元数据      │  否  │    -       │
	// │ 9  │ 上传字幕到B站    │   上传到Bilibili   │  否  │  延迟重试  │
	// └────┴──────────────────┴────────────────────┴──────┴────────────┘
	steps := []struct {
		Name     string
		Order    int
		CanRetry bool
	}{
		{"获取元数据", 1, true},
		{"下载视频", 2, true},
		{"下载字幕", 3, true},
		{"下载封面", 4, true},
		{"翻译字幕", 5, true},
		{"AI增强元数据", 6, true},
		{"确认元数据", 7, true},
		{"上传到Bilibili", 8, true},
		{"上传字幕到Bilibili", 9, true},
	}

	// 检查是否已经初始化过
	var existingSteps []model.TaskStep
	if err := s.DB.Where("video_id = ?", videoID).Find(&existingSteps).Error; err != nil {
		return err
	}

	// 创建已存在步骤的映射
	existingStepNames := make(map[string]bool)
	for _, step := range existingSteps {
		existingStepNames[step.StepName] = true
	}

	// 创建任务步骤记录（只创建不存在的步骤）
	for _, step := range steps {
		if existingStepNames[step.Name] {
			continue // 步骤已存在，跳过
		}

		taskStep := &model.TaskStep{
			VideoID:   videoID,
			StepName:  step.Name,
			StepOrder: step.Order,
			Status:    model.TaskStepStatusWaiting, // 使用 waiting 状态，防止调度器抢先执行
			CanRetry:  step.CanRetry,
		}

		if err := s.DB.Create(taskStep).Error; err != nil {
			return err
		}
	}

	return nil
}

// GetTaskStepsByVideoID 根据视频ID获取任务步骤列表
func (s *TaskStepService) GetTaskStepsByVideoID(videoID string) ([]model.TaskStep, error) {
	var steps []model.TaskStep
	err := s.DB.Where("video_id = ?", videoID).
		Order("step_order ASC").
		Find(&steps).Error
	return steps, err
}

// GetTaskStepsByVideoIDForUser 根据视频ID获取任务步骤列表（带用户隔离）
// 先校验视频归属，再返回任务步骤
func (s *TaskStepService) GetTaskStepsByVideoIDForUser(videoID string, userID uint) ([]model.TaskStep, error) {
	// 先校验视频归属
	var video model.SavedVideo
	query := s.DB.Where("video_id = ?", videoID)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.First(&video).Error; err != nil {
		return nil, errors.New("视频不存在或无权访问")
	}
	// 返回任务步骤
	return s.GetTaskStepsByVideoID(videoID)
}

// UpdateTaskStepStatus 更新任务步骤状态
func (s *TaskStepService) UpdateTaskStepStatus(videoID, stepName, status string, errorMsg ...string) error {
	updates := map[string]interface{}{
		"status": status,
	}

	// 设置时间
	now := time.Now()
	if status == model.TaskStepStatusRunning {
		updates["start_time"] = &now
	} else if status == model.TaskStepStatusCompleted || status == model.TaskStepStatusFailed {
		updates["end_time"] = &now

		// 计算执行时长
		var step model.TaskStep
		if err := s.DB.Where("video_id = ? AND step_name = ?", videoID, stepName).First(&step).Error; err == nil {
			if step.StartTime != nil {
				duration := now.Sub(*step.StartTime).Milliseconds()
				updates["duration"] = duration
			}
		}
	}

	// 设置错误信息
	if len(errorMsg) > 0 && errorMsg[0] != "" {
		updates["error_msg"] = errorMsg[0]
	}

	return s.DB.Model(&model.TaskStep{}).
		Where("video_id = ? AND step_name = ?", videoID, stepName).
		Updates(updates).Error
}

// UpdateTaskStepResult 更新任务步骤执行结果
func (s *TaskStepService) UpdateTaskStepResult(videoID, stepName string, resultData interface{}) error {
	var jsonData string
	if resultData != nil {
		if jsonBytes, err := json.Marshal(resultData); err == nil {
			jsonData = string(jsonBytes)
		}
	}

	return s.DB.Model(&model.TaskStep{}).
		Where("video_id = ? AND step_name = ?", videoID, stepName).
		Update("result_data", jsonData).Error
}

// ResetTaskStep 重置任务步骤（用于重新执行）
func (s *TaskStepService) ResetTaskStep(videoID, stepName string) error {
	updates := map[string]interface{}{
		"status":      model.TaskStepStatusPending,
		"start_time":  nil,
		"end_time":    nil,
		"duration":    0,
		"error_msg":   "",
		"result_data": "",
	}

	return s.DB.Model(&model.TaskStep{}).
		Where("video_id = ? AND step_name = ?", videoID, stepName).
		Updates(updates).Error
}

// GetTaskStepByName 根据视频ID和步骤名称获取特定步骤
func (s *TaskStepService) GetTaskStepByName(videoID, stepName string) (*model.TaskStep, error) {
	var step model.TaskStep
	err := s.DB.Where("video_id = ? AND step_name = ?", videoID, stepName).First(&step).Error
	if err != nil {
		return nil, err
	}
	return &step, nil
}

// GetTaskProgress 获取任务进度信息
func (s *TaskStepService) GetTaskProgress(videoID string) (map[string]interface{}, error) {
	var steps []model.TaskStep
	if err := s.DB.Where("video_id = ?", videoID).Order("step_order ASC").Find(&steps).Error; err != nil {
		return nil, err
	}

	totalSteps := len(steps)
	completedSteps := 0
	failedSteps := 0
	currentStep := ""

	for _, step := range steps {
		switch step.Status {
		case model.TaskStepStatusCompleted:
			completedSteps++
		case model.TaskStepStatusFailed:
			failedSteps++
		case model.TaskStepStatusRunning:
			currentStep = step.StepName
		}
	}

	progress := map[string]interface{}{
		"total_steps":      totalSteps,
		"completed_steps":  completedSteps,
		"failed_steps":     failedSteps,
		"current_step":     currentStep,
		"progress_percent": 0,
	}

	if totalSteps > 0 {
		progress["progress_percent"] = (completedSteps * 100) / totalSteps
	}

	return progress, nil
}

// ResetAllRunningTasks 重置所有运行中的任务
func (s *TaskStepService) ResetAllRunningTasks() error {
	// 开始事务
	tx := s.DB.Begin()
	// 确保事务在任何情况下都能正确回滚（包括 panic）
	// 注意：如果 Commit() 成功，后续的 Rollback() 是 no-op
	defer tx.Rollback()

	// 重置所有状态为 Running 的任务步骤为 Pending
	result := tx.Model(&model.TaskStep{}).
		Where("status = ?", model.TaskStepStatusRunning).
		Update("status", model.TaskStepStatusPending)

	if result.Error != nil {
		return fmt.Errorf("failed to reset running task steps: %v", result.Error)
	}

	taskStepsAffected := result.RowsAffected

	// 重置相关视频的状态
	// 将状态为 "002"(处理中) 的视频重置为 "001"(待处理)
	videoResult := tx.Model(&model.SavedVideo{}).
		Where("status = ?", "002").
		Update("status", "001")

	if videoResult.Error != nil {
		return fmt.Errorf("failed to reset running video status: %v", videoResult.Error)
	}

	videosAffected := videoResult.RowsAffected

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	log.Printf("Reset %d running task steps and %d videos (from processing to pending status)", taskStepsAffected, videosAffected)
	return nil
}

// GetPendingSteps 获取所有状态为pending的任务步骤
func (s *TaskStepService) GetPendingSteps() ([]*model.TaskStep, error) {
	var steps []*model.TaskStep

	// 使用 JOIN 查询，只获取未删除视频的待处理步骤
	result := s.DB.Table("cw_task_steps").
		Select("cw_task_steps.*").
		Joins("INNER JOIN cw_saved_videos ON cw_task_steps.video_id = cw_saved_videos.video_id").
		Where("cw_task_steps.status = ?", model.TaskStepStatusPending).
		Where("cw_task_steps.deleted_at IS NULL").
		Where("cw_saved_videos.deleted_at IS NULL").
		Order("cw_task_steps.created_at ASC").
		Find(&steps)

	if result.Error != nil {
		return nil, fmt.Errorf("查询待重试步骤失败: %v", result.Error)
	}

	return steps, nil
}

// ResetFailedStepsToPending 将可重试的失败步骤重置为 pending
// 这样调度器就能检测到这些步骤并进行重试
// 注意：超过最大重试次数(10次)的步骤将被标记为永久失败，不再重试
func (s *TaskStepService) ResetFailedStepsToPending() error {
	now := time.Now()
	const maxRetryCount = 10 // 最大重试次数

	// 1. 将超过最大重试次数的步骤标记为永久失败
	permanentFailResult := s.DB.Table("cw_task_steps").
		Where("status = ?", model.TaskStepStatusFailed).
		Where("can_retry = ?", true).
		Where("retry_count >= ?", maxRetryCount).
		Updates(map[string]interface{}{
			"status":     "failed_permanent",
			"can_retry":  false,
			"updated_at": now,
			"error":      fmt.Sprintf("超过最大重试次数(%d次)，已停止重试", maxRetryCount),
		})

	if permanentFailResult.Error != nil {
		return fmt.Errorf("标记永久失败步骤失败: %w", permanentFailResult.Error)
	}

	if permanentFailResult.RowsAffected > 0 {
		log.Printf("⛔ 已将 %d 个步骤标记为永久失败（超过最大重试次数）", permanentFailResult.RowsAffected)
	}

	// 2. 重置可重试的失败步骤（未超过最大重试次数）
	result := s.DB.Table("cw_task_steps").
		Where("status = ?", model.TaskStepStatusFailed).
		Where("can_retry = ?", true).
		Where("retry_count < ?", maxRetryCount).
		Updates(map[string]interface{}{
			"status":      model.TaskStepStatusPending,
			"updated_at":  now,
			"retry_count": gorm.Expr("retry_count + 1"),
		})

	if result.Error != nil {
		return fmt.Errorf("重置失败步骤为 pending 失败: %w", result.Error)
	}

	// 只有实际有重置的步骤时才记录日志
	if result.RowsAffected > 0 {
		log.Printf("✓ 重置 %d 个失败步骤为 pending 状态", result.RowsAffected)
	}

	return nil
}

// DeleteTaskStepsByVideoID 删除指定视频的所有任务步骤（软删除）
func (s *TaskStepService) DeleteTaskStepsByVideoID(videoID string) error {
	result := s.DB.Where("video_id = ?", videoID).Delete(&model.TaskStep{})
	if result.Error != nil {
		return fmt.Errorf("删除任务步骤失败: %v", result.Error)
	}
	return nil
}

// ResetFailedSteps 重置指定视频的所有失败/跳过步骤为待执行状态
// 返回被重置的步骤数量
func (s *TaskStepService) ResetFailedSteps(videoID string) (int64, error) {
	updates := map[string]interface{}{
		"status":     model.TaskStepStatusPending,
		"start_time": nil,
		"end_time":   nil,
		"duration":   0,
		"error_msg":  "",
	}

	result := s.DB.Model(&model.TaskStep{}).
		Where("video_id = ? AND status IN ?", videoID, []string{model.TaskStepStatusFailed, model.TaskStepStatusSkipped}).
		Updates(updates)

	if result.Error != nil {
		return 0, fmt.Errorf("重置失败步骤失败: %v", result.Error)
	}

	return result.RowsAffected, nil
}

// GetFailedOrSkippedSteps 获取指定视频的所有失败或跳过的步骤
func (s *TaskStepService) GetFailedOrSkippedSteps(videoID string) ([]model.TaskStep, error) {
	var steps []model.TaskStep
	err := s.DB.Where("video_id = ? AND status IN ?", videoID, []string{model.TaskStepStatusFailed, model.TaskStepStatusSkipped}).
		Order("step_order ASC").
		Find(&steps).Error
	return steps, err
}

// GetCompletedStepNames 获取指定视频已完成的任务步骤名称列表
func (s *TaskStepService) GetCompletedStepNames(videoID string) ([]string, error) {
	var stepNames []string
	err := s.DB.Model(&model.TaskStep{}).
		Where("video_id = ? AND status = ?", videoID, model.TaskStepStatusCompleted).
		Pluck("step_name", &stepNames).Error
	return stepNames, err
}

// EnsureSubtitleUploadStep 确保视频有"上传字幕到Bilibili"步骤
// 用于为已存在的视频补充缺失的步骤
func (s *TaskStepService) EnsureSubtitleUploadStep(videoID string) error {
	// 检查步骤是否已存在
	var count int64
	if err := s.DB.Model(&model.TaskStep{}).
		Where("video_id = ? AND step_name = ?", videoID, "上传字幕到Bilibili").
		Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		return nil // 步骤已存在
	}

	// 创建步骤
	taskStep := &model.TaskStep{
		VideoID:   videoID,
		StepName:  "上传字幕到Bilibili",
		StepOrder: 8,
		Status:    model.TaskStepStatusPending,
		CanRetry:  true,
	}

	return s.DB.Create(taskStep).Error
}

// MigrateAllVideosSubtitleStep 为所有视频添加"上传字幕到Bilibili"步骤
func (s *TaskStepService) MigrateAllVideosSubtitleStep() (int, error) {
	// 查询所有没有"上传字幕到Bilibili"步骤的视频
	var videoIDs []string
	err := s.DB.Model(&model.TaskStep{}).
		Select("DISTINCT video_id").
		Where("video_id NOT IN (?)",
			s.DB.Model(&model.TaskStep{}).
				Select("video_id").
				Where("step_name = ?", "上传字幕到Bilibili")).
		Pluck("video_id", &videoIDs).Error

	if err != nil {
		return 0, err
	}

	count := 0
	for _, videoID := range videoIDs {
		if err := s.EnsureSubtitleUploadStep(videoID); err != nil {
			log.Printf("为视频 %s 添加字幕上传步骤失败: %v", videoID, err)
			continue
		}
		count++
	}

	return count, nil
}
