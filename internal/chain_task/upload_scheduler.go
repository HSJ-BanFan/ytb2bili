package chain_task

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/difyz9/ytb2bili/internal/chain_task/handlers"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/core/types"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// UploadScheduler 上传调度器
// 负责定时上传视频和字幕到Bilibili
type UploadScheduler struct {
	App                *core.AppServer
	SavedVideoService  *services.SavedVideoService
	TaskStepService    *services.TaskStepService
	BiliAccountService *services.BiliAccountService
	Db                 *gorm.DB
	Task               *cron.Cron
	mutex              sync.Mutex
	logger             *zap.SugaredLogger

	// 上传队列跟踪
	lastVideoUploadTime    time.Time // 最后一次视频上传时间
	lastSubtitleUploadTime time.Time // 最后一次字幕上传时间
}

// NewUploadScheduler 创建上传调度器实例
func NewUploadScheduler(
	app *core.AppServer,
	task *cron.Cron,
	db *gorm.DB,
	savedVideoService *services.SavedVideoService,
	taskStepService *services.TaskStepService,
	biliAccountService *services.BiliAccountService,
) *UploadScheduler {
	return &UploadScheduler{
		App:                app,
		Task:               task,
		Db:                 db,
		SavedVideoService:  savedVideoService,
		TaskStepService:    taskStepService,
		BiliAccountService: biliAccountService,
		logger:             app.Logger,
	}
}

// SetUp 启动上传调度器
func (s *UploadScheduler) SetUp() {
	// 每10秒检查一次是否需要上传（更及时响应）
	s.Task.AddFunc("*/10 * * * * *", func() {
		s.mutex.Lock()
		defer s.mutex.Unlock()

		// 检查是否启用自动上传
		autoUploadEnabled := true
		if s.App.Config != nil && s.App.Config.DownloadConfig != nil {
			autoUploadEnabled = s.App.Config.DownloadConfig.AutoUploadEnabled
		}

		if autoUploadEnabled {
			// 1. 检查是否需要上传视频
			if err := s.uploadNextVideo(); err != nil {
				s.logger.Errorf("上传视频失败: %v", err)
			}
		}

		// 2. 检查是否需要上传字幕
		if err := s.uploadNextSubtitle(); err != nil {
			s.logger.Errorf("上传字幕失败: %v", err)
		}
	})

	// 启动时显示待上传字幕的视频数量
	s.showPendingSubtitleInfo()

	s.logger.Info("✓ Upload scheduler started, checking every 10 seconds")
}

// uploadNextVideo 上传下一个准备好的视频
// 根据配置的上传模式（immediate/delayed）决定是否立即上传
func (s *UploadScheduler) uploadNextVideo() error {
	now := time.Now()

	// 获取配置
	uploadMode := "delayed"
	videoUploadDelay := 10 // 默认10分钟
	subtitleUploadDelay := 10
	if s.App.Config != nil && s.App.Config.DownloadConfig != nil {
		if s.App.Config.DownloadConfig.AutoUploadMode != "" {
			uploadMode = s.App.Config.DownloadConfig.AutoUploadMode
		}
		if s.App.Config.DownloadConfig.VideoUploadDelay > 0 {
			videoUploadDelay = s.App.Config.DownloadConfig.VideoUploadDelay
		}
		if s.App.Config.DownloadConfig.SubtitleUploadDelay > 0 {
			subtitleUploadDelay = s.App.Config.DownloadConfig.SubtitleUploadDelay
		}
	}

	// 查询状态为 '200' (准备就绪) 的视频
	var videos []struct {
		ID                    uint
		VideoID               string
		Title                 string
		ProcessingCompletedAt *time.Time
		CreatedAt             time.Time
	}

	query := s.Db.Table("cw_saved_videos").
		Select("id, video_id, title, processing_completed_at, created_at").
		Where("status = ?", "200").
		Where("deleted_at IS NULL")

	// Query videos
	// ... (existing query logic)

	// 根据上传模式添加时间条件
	if uploadMode == "delayed" {
		// 延迟模式：只查询 processing_completed_at + delay <= now 的视频
		delayDuration := time.Duration(videoUploadDelay) * time.Minute
		query = query.Where("processing_completed_at IS NOT NULL AND processing_completed_at <= ?", now.Add(-delayDuration))
	} else {
		// Log for verification
		// s.logger.Debugf("🔍 Check upload (Mode: %s) - No partial delay", uploadMode)
	}
	// immediate 模式：不添加时间条件，立即上传

	err := query.Order("COALESCE(processing_completed_at, created_at) ASC").
		Limit(1).
		Find(&videos).Error

	if err != nil {
		return fmt.Errorf("查询待上传视频失败: %v", err)
	}

	if len(videos) == 0 {
		if uploadMode != "delayed" {
			s.logger.Debugf("未找到待上传视频 (模式: %s)", uploadMode)
		}
		return nil
	}

	video := videos[0]

	// 显示调度信息
	if video.ProcessingCompletedAt != nil {
		s.logger.Infof("📤 开始上传视频: %s (VideoID: %s, 处理完成于: %s)",
			video.Title, video.VideoID, video.ProcessingCompletedAt.Format("15:04:05"))
	} else {
		s.logger.Infof("📤 开始上传视频: %s (VideoID: %s)", video.Title, video.VideoID)
	}

	// 更新状态为 '201' (上传视频中)
	if err := s.SavedVideoService.UpdateStatus(video.ID, "201"); err != nil {
		return fmt.Errorf("更新视频状态失败: %v", err)
	}

	// 执行上传任务
	if err := s.executeUploadTask(video.VideoID, "上传到Bilibili"); err != nil {
		// 上传失败，更新状态为 '299' (上传失败)
		s.SavedVideoService.UpdateStatus(video.ID, "299")
		return fmt.Errorf("上传视频失败: %v", err)
	}

	// 上传成功，更新状态为 '300' 并记录上传时间和计划字幕上传时间
	subtitleScheduledAt := now.Add(time.Duration(subtitleUploadDelay) * time.Minute)
	s.Db.Table("cw_saved_videos").Where("id = ?", video.ID).Updates(map[string]interface{}{
		"status":                "300",
		"video_uploaded_at":     now,
		"subtitle_scheduled_at": subtitleScheduledAt,
	})

	s.logger.Infof("✅ 视频上传成功: %s, 字幕将在 %s 后上传", video.VideoID, subtitleScheduledAt.Format("15:04:05"))
	return nil
}

// uploadNextSubtitle 上传下一个待上传字幕的视频
// 使用智能延迟策略：根据视频大小计算延迟时间
func (s *UploadScheduler) uploadNextSubtitle() error {
	// 查询状态为 '300' (视频已上传，待上传字幕) 且已到达计划上传时间的视频
	var videos []struct {
		ID                    uint
		VideoID               string
		Title                 string
		VideoSizeMB           float64
		SubtitleScheduledAt   *time.Time
		SubtitleUploadRetries int
		UpdatedAt             time.Time
		CreatedAt             time.Time
	}

	now := time.Now()

	// 查询已到达计划上传时间的视频，或者没有设置计划时间但已过去1小时的视频
	err := s.Db.Table("cw_saved_videos").
		Select("id, video_id, title, video_size_mb, subtitle_scheduled_at, subtitle_upload_retries, updated_at, created_at").
		Where("status = ?", "300").
		Where("deleted_at IS NULL").
		Where("subtitle_upload_retries < ?", 3). // 最多重试3次
		Where("(subtitle_scheduled_at IS NOT NULL AND subtitle_scheduled_at <= ?) OR (subtitle_scheduled_at IS NULL AND updated_at <= ?)",
			now, now.Add(-time.Hour)).
		Order("COALESCE(subtitle_scheduled_at, updated_at) ASC").
		Limit(1).
		Find(&videos).Error

	if err != nil {
		return fmt.Errorf("查询待上传字幕的视频失败: %v", err)
	}

	if len(videos) == 0 {
		s.logger.Debug("没有待上传字幕的视频")
		return nil
	}

	video := videos[0]

	// 显示调度信息
	if video.SubtitleScheduledAt != nil {
		s.logger.Infof("📝 开始上传字幕: %s (VideoID: %s, 计划时间: %s, 重试: %d/3)",
			video.Title, video.VideoID, video.SubtitleScheduledAt.Format("15:04:05"), video.SubtitleUploadRetries)
	} else {
		s.logger.Infof("📝 开始上传字幕: %s (VideoID: %s, 重试: %d/3)",
			video.Title, video.VideoID, video.SubtitleUploadRetries)
	}

	// 更新状态为 '301' (上传字幕中)
	if err := s.SavedVideoService.UpdateStatus(video.ID, "301"); err != nil {
		return fmt.Errorf("更新视频状态失败: %v", err)
	}

	// 执行上传字幕任务
	if err := s.executeUploadTask(video.VideoID, "上传字幕到Bilibili"); err != nil {
		// 上传失败，增加重试次数并设置下次重试时间
		retryCount := video.SubtitleUploadRetries + 1
		nextRetryTime := s.calculateNextRetryTime(retryCount)

		s.Db.Table("cw_saved_videos").Where("id = ?", video.ID).Updates(map[string]interface{}{
			"status":                  "300", // 保持待上传状态
			"subtitle_upload_retries": retryCount,
			"subtitle_scheduled_at":   nextRetryTime,
			"subtitle_upload_error":   err.Error(),
		})

		if retryCount >= 3 {
			// 超过最大重试次数，标记为失败
			s.SavedVideoService.UpdateStatus(video.ID, "399")
			s.logger.Errorf("✗ 字幕上传失败 (已达最大重试次数): %s", video.VideoID)
		} else {
			s.logger.Warnf("⚠️ 字幕上传失败，将在 %s 重试 (%d/3): %v",
				nextRetryTime.Format("15:04:05"), retryCount, err)
		}
		return fmt.Errorf("上传字幕失败: %v", err)
	}

	// 上传成功，更新状态为 '400' (全部完成)
	if err := s.SavedVideoService.UpdateStatus(video.ID, "400"); err != nil {
		return fmt.Errorf("更新视频状态失败: %v", err)
	}

	// 清除错误信息
	s.Db.Table("cw_saved_videos").Where("id = ?", video.ID).Updates(map[string]interface{}{
		"subtitle_upload_error": "",
	})

	s.logger.Infof("✅ 字幕上传成功: %s", video.VideoID)
	return nil
}

// calculateSubtitleDelay 根据视频大小计算字幕上传延迟时间
// B站视频审核时间与视频大小正相关
func (s *UploadScheduler) calculateSubtitleDelay(videoSizeMB float64) time.Duration {
	// 根据视频大小设置延迟时间
	// 小视频 (<100MB): 10分钟
	// 中等视频 (100-300MB): 15分钟
	// 大视频 (300-500MB): 20分钟
	// 超大视频 (>500MB): 25分钟
	var delay time.Duration

	switch {
	case videoSizeMB <= 0:
		// 未记录视频大小，使用默认值
		delay = 15 * time.Minute
	case videoSizeMB < 100:
		delay = 10 * time.Minute
	case videoSizeMB < 300:
		delay = 15 * time.Minute
	case videoSizeMB < 500:
		delay = 20 * time.Minute
	default:
		delay = 25 * time.Minute
	}

	s.logger.Infof("🕒 字幕上传延迟计算: 视频%.1fMB -> 延迟%d分钟", videoSizeMB, int(delay.Minutes()))
	return delay
}

// calculateNextRetryTime 计算下次重试时间 (指数退避)
func (s *UploadScheduler) calculateNextRetryTime(retryCount int) time.Time {
	// 第1次重试: 10分钟后
	// 第2次重试: 20分钟后
	// 第3次重试: 40分钟后
	delayMinutes := 10 * (1 << (retryCount - 1)) // 10, 20, 40
	return time.Now().Add(time.Duration(delayMinutes) * time.Minute)
}

// showPendingSubtitleInfo 显示待上传字幕的视频信息
func (s *UploadScheduler) showPendingSubtitleInfo() {
	var count int64
	now := time.Now()

	// 统计待上传字幕的视频数量
	s.Db.Table("cw_saved_videos").
		Where("status = ?", "300").
		Where("deleted_at IS NULL").
		Where("subtitle_upload_retries < ?", 3).
		Count(&count)

	if count == 0 {
		s.logger.Info("📝 当前没有待上传字幕的视频")
		return
	}

	// 查询最近的一个待上传视频
	var video struct {
		VideoID             string
		Title               string
		VideoSizeMB         float64
		SubtitleScheduledAt *time.Time
	}

	s.Db.Table("cw_saved_videos").
		Select("video_id, title, video_size_mb, subtitle_scheduled_at").
		Where("status = ?", "300").
		Where("deleted_at IS NULL").
		Where("subtitle_upload_retries < ?", 3).
		Order("COALESCE(subtitle_scheduled_at, updated_at) ASC").
		Limit(1).
		Find(&video)

	if video.SubtitleScheduledAt != nil {
		remaining := video.SubtitleScheduledAt.Sub(now)
		if remaining > 0 {
			s.logger.Infof("📝 待上传字幕: %d 个视频, 下一个: %s (%.1fMB), 将在 %s 后上传",
				count, video.Title, video.VideoSizeMB, remaining.Round(time.Minute))
		} else {
			s.logger.Infof("📝 待上传字幕: %d 个视频, 下一个: %s (%.1fMB), 已到达计划时间",
				count, video.Title, video.VideoSizeMB)
		}
	} else {
		s.logger.Infof("📝 待上传字幕: %d 个视频, 下一个: %s (%.1fMB)",
			count, video.Title, video.VideoSizeMB)
	}
}

// executeUploadTask 执行上传任务
func (s *UploadScheduler) executeUploadTask(videoID, taskName string) error {
	// 获取视频信息
	savedVideo, err := s.SavedVideoService.GetVideoByVideoID(videoID)
	if err != nil {
		return fmt.Errorf("获取视频信息失败: %v", err)
	}

	// 获取当前目录
	currentDir, err := filepath.Abs(s.App.Config.FileUpDir)
	if err != nil {
		return fmt.Errorf("获取文件上传目录失败: %v", err)
	}

	// 创建状态管理器
	stateManager := manager.NewStateManager(savedVideo.ID, savedVideo.VideoID, currentDir, savedVideo.CreatedAt)

	// 更新步骤状态为运行中
	if err := s.TaskStepService.UpdateTaskStepStatus(videoID, taskName, "running"); err != nil {
		s.logger.Errorf("更新任务步骤状态失败: %v", err)
	}

	// 创建任务链
	chain := manager.NewTaskChain()
	var task types.Task

	// 根据任务名称创建对应的任务
	switch taskName {
	case "上传到Bilibili":
		task = handlers.NewUploadToBilibili("上传到Bilibili", s.App, stateManager, s.App.CosClient, s.SavedVideoService, s.BiliAccountService)
	case "上传字幕到Bilibili":
		task = handlers.NewUploadSubtitleToBilibili("上传字幕到Bilibili", s.App, stateManager, s.App.CosClient, s.SavedVideoService, s.BiliAccountService)
	default:
		return fmt.Errorf("未知的任务类型: %s", taskName)
	}

	// 添加任务到链
	chain.AddTask(task)

	s.logger.Infof("开始执行上传任务: %s (VideoID: %s)", taskName, videoID)

	// 创建带有封面路径的 context
	initialContext := make(map[string]interface{})

	// 查找封面文件并设置到 context
	if taskName == "上传到Bilibili" {
		s.logger.Infof("📂 封面查找目录: %s", stateManager.CurrentDir)
		coverPath := s.findCoverImage(stateManager.CurrentDir)
		if coverPath != "" {
			initialContext["cover_image_path"] = coverPath
			s.logger.Infof("📸 找到封面文件: %s (完整路径: %s)", filepath.Base(coverPath), coverPath)
		} else {
			s.logger.Warnf("⚠️ 未找到封面文件，目录: %s", stateManager.CurrentDir)
		}
	}

	// 执行任务（传入初始 context）
	result := chain.RunWithContext(initialContext)

	// 检查执行结果
	success := true
	var errorMsg string
	if errorMsgInterface, exists := result["error"]; exists && errorMsgInterface != nil {
		success = false
		errorMsg = fmt.Sprintf("%v", errorMsgInterface)
	}

	// 更新步骤状态
	if success {
		if err := s.TaskStepService.UpdateTaskStepStatus(videoID, taskName, "completed"); err != nil {
			s.logger.Errorf("更新任务步骤状态失败: %v", err)
		}
		if err := s.TaskStepService.UpdateTaskStepResult(videoID, taskName, result); err != nil {
			s.logger.Errorf("更新任务步骤结果失败: %v", err)
		}
		s.logger.Infof("任务 %s 执行成功", taskName)
		return nil
	} else {
		if err := s.TaskStepService.UpdateTaskStepStatus(videoID, taskName, "failed", errorMsg); err != nil {
			s.logger.Errorf("更新任务步骤状态失败: %v", err)
		}
		s.logger.Errorf("任务 %s 执行失败: %s", taskName, errorMsg)
		return fmt.Errorf("任务执行失败: %s", errorMsg)
	}
}

// ExecuteManualUpload 手动执行上传任务（用于 Web 界面手动触发）
func (s *UploadScheduler) ExecuteManualUpload(videoID, taskType string) error {
	s.logger.Infof("🎯 手动执行上传任务: VideoID=%s, TaskType=%s", videoID, taskType)

	var taskName string
	switch taskType {
	case "video":
		taskName = "上传到Bilibili"
	case "subtitle":
		taskName = "上传字幕到Bilibili"
	default:
		return fmt.Errorf("未知的任务类型: %s", taskType)
	}

	return s.executeUploadTask(videoID, taskName)
}

// findCoverImage 在指定目录中查找封面图片
// 优先查找 maxresdefault.jpg，其次 sddefault.jpg，最后查找任意 jpg 文件
func (s *UploadScheduler) findCoverImage(dir string) string {
	// 优先级列表
	priorityFiles := []string{
		"maxresdefault.jpg",
		"sddefault.jpg",
		"hqdefault.jpg",
		"mqdefault.jpg",
		"default.jpg",
	}

	// 按优先级查找
	for _, filename := range priorityFiles {
		coverPath := filepath.Join(dir, filename)
		if _, err := os.Stat(coverPath); err == nil {
			return coverPath
		}
	}

	// 如果都没找到，查找任意 jpg 文件
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) == ".jpg" || filepath.Ext(name) == ".jpeg" || filepath.Ext(name) == ".png" {
			return filepath.Join(dir, name)
		}
	}

	return ""
}
