package services

import (
	"encoding/json"
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
	// ┌────┬─────────────┬─────────────┬──────┬────────────┐
	// │序号│   任务名    │    依赖     │ 必需 │  失败处理  │
	// ├────┼─────────────┼─────────────┼──────┼────────────┤
	// │ 1  │ 获取元数据  │     无      │  是  │  终止链    │
	// │ 2  │ 下载视频    │     无      │  否  │  继续执行  │
	// │ 3  │ 生成字幕    │  下载视频   │  否  │    跳过    │
	// │ 4  │ 下载封面    │  获取元数据 │  是  │  终止链    │
	// │ 5  │ 翻译字幕    │  生成字幕   │  否  │    跳过    │
	// │ 6  │ 生成元数据  │  下载视频   │  否  │    跳过    │
	// │ 7  │ 上传到B站   │     无      │  否  │    -       │
	// └────┴─────────────┴─────────────┴──────┴────────────┘
	steps := []struct {
		Name     string
		Order    int
		CanRetry bool
	}{
		{"获取元数据", 1, true},
		{"下载视频", 2, true},
		{"生成字幕", 3, true},
		{"下载封面", 4, true},
		{"翻译字幕", 5, true},
		{"生成元数据", 6, true},
		{"上传到Bilibili", 7, true},
	}

	// 检查是否已经初始化过
	var count int64
	if err := s.DB.Model(&model.TaskStep{}).Where("video_id = ?", videoID).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		return nil // 已经初始化过，跳过
	}

	// 创建任务步骤记录
	for _, step := range steps {
		taskStep := &model.TaskStep{
			VideoID:   videoID,
			StepName:  step.Name,
			StepOrder: step.Order,
			Status:    model.TaskStepStatusPending,
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

	// 重置所有状态为 Running 的任务步骤为 Pending
	result := tx.Model(&model.TaskStep{}).
		Where("status = ?", "Running").
		Update("status", "Pending")

	if result.Error != nil {
		tx.Rollback()
		return fmt.Errorf("failed to reset running task steps: %v", result.Error)
	}

	taskStepsAffected := result.RowsAffected

	// 重置相关视频的状态
	// 将状态为 "002"(处理中) 的视频重置为 "001"(待处理)
	videoResult := tx.Model(&model.SavedVideo{}).
		Where("status = ?", "002").
		Update("status", "001")

	if videoResult.Error != nil {
		tx.Rollback()
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
