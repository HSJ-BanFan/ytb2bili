package chain_task

import (
	"context"
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
	applogger "github.com/difyz9/ytb2bili/internal/logger"
	"github.com/difyz9/ytb2bili/internal/membership"
	"github.com/difyz9/ytb2bili/pkg/logger"

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

	// 任务取消管理器
	CancelManager *TaskCancelManager

	// 权限服务（可选注入）
	PermissionService *membership.PermissionService
}

// NewUploadScheduler 创建上传调度器实例
func NewUploadScheduler(
	app *core.AppServer,
	task *cron.Cron,
	db *gorm.DB,
	savedVideoService *services.SavedVideoService,
	taskStepService *services.TaskStepService,
	biliAccountService *services.BiliAccountService,
	cancelManager *TaskCancelManager, // 新增参数
) *UploadScheduler {
	return &UploadScheduler{
		App:                app,
		Task:               task,
		Db:                 db,
		SavedVideoService:  savedVideoService,
		TaskStepService:    taskStepService,
		BiliAccountService: biliAccountService,
		logger:             app.Logger,
		CancelManager:      cancelManager, // 初始化取消管理器
	}
}

// SetUp 启动上传调度器
func (s *UploadScheduler) SetUp() {
	// 获取检查间隔配置（默认10秒）
	checkInterval := 10 // 默认值
	if s.App.Config != nil && s.App.Config.DownloadConfig != nil && s.App.Config.DownloadConfig.UploadCheckInterval > 0 {
		checkInterval = s.App.Config.DownloadConfig.UploadCheckInterval
	}

	// 构建 cron 表达式: 每N秒执行一次
	// 格式: */N * * * * *
	cronExpr := fmt.Sprintf("*/%d * * * * *", checkInterval)

	s.Task.AddFunc(cronExpr, func() {
		s.mutex.Lock()
		defer s.mutex.Unlock()

		// 使用智能日志器，自动过滤进度活动期间的噪音日志
		smartLogger := logger.NewSmartLogger(s.logger)

		// 检查是否启用自动上传
		autoUploadEnabled := true
		if s.App.Config != nil && s.App.Config.DownloadConfig != nil {
			autoUploadEnabled = s.App.Config.DownloadConfig.AutoUploadEnabled
		}

		if autoUploadEnabled {
			// 1. 检查是否需要上传视频
			if err := s.uploadNextVideo(); err != nil {
				smartLogger.Errorf("上传视频失败: %v", err)
			}
		}

		// 2. 检查是否需要上传字幕
		if err := s.uploadNextSubtitle(); err != nil {
			smartLogger.Errorf("上传字幕失败: %v", err)
		}
	})

	// 启动时显示待上传字幕的视频数量
	s.showPendingSubtitleInfo()

	// 显示上传模式和延迟信息
	uploadMode := "immediate"
	videoDelay := 0
	if s.App.Config != nil && s.App.Config.DownloadConfig != nil {
		if s.App.Config.DownloadConfig.AutoUploadMode != "" {
			uploadMode = s.App.Config.DownloadConfig.AutoUploadMode
		}
		videoDelay = s.App.Config.DownloadConfig.VideoUploadDelay
	}
	if uploadMode == "delayed" {
		s.logger.Infof("📤 上传模式: 延迟上传 (视频处理完成后 %d 分钟上传)", videoDelay)
	} else {
		s.logger.Infof("📤 上传模式: 立即上传")
	}

	s.logger.Infof("✓ Upload scheduler started, checking every %d seconds", checkInterval)
}

// uploadNextVideo 上传下一个准备好的视频
// 根据配置的上传模式（immediate/delayed）决定是否立即上传
func (s *UploadScheduler) uploadNextVideo() error {
	now := time.Now()

	// 使用智能日志器过滤噪音日志
	smartLogger := logger.NewSmartLogger(s.logger)

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
		UserID                uint // 用户ID，用于权限检查
		VideoID               string
		Title                 string
		Subtitles             string // 字幕数据，用于判断是否有字幕
		ProcessingCompletedAt *time.Time
		CreatedAt             time.Time
	}

	query := s.Db.Table("cw_saved_videos").
		Select("id, user_id, video_id, title, subtitles, processing_completed_at, created_at").
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
		if uploadMode == "delayed" {
			// 检查是否有正在等待延迟的视频
			var pendingVideo struct {
				VideoID               string
				Title                 string
				ProcessingCompletedAt *time.Time
			}
			s.Db.Table("cw_saved_videos").
				Select("video_id, title, processing_completed_at").
				Where("status = ?", "200").
				Where("deleted_at IS NULL").
				Where("processing_completed_at IS NOT NULL").
				Order("processing_completed_at ASC").
				Limit(1).
				Find(&pendingVideo)

			if pendingVideo.ProcessingCompletedAt != nil {
				uploadAt := pendingVideo.ProcessingCompletedAt.Add(time.Duration(videoUploadDelay) * time.Minute)
				remaining := uploadAt.Sub(now)
				if remaining > 0 {
					smartLogger.Debugf("📤 等待延迟上传: %s (剩余 %s)", pendingVideo.Title, remaining.Round(time.Second))
				}
			}
		} else {
			smartLogger.Debugf("未找到待上传视频 (模式: %s)", uploadMode)
		}
		return nil
	}

	video := videos[0]

	// 检查用户自动上传权限
	if s.PermissionService != nil && video.UserID > 0 {
		userIDStr := fmt.Sprintf("%d", video.UserID)
		canUpload, reason, err := s.PermissionService.CanAutoUpload(context.Background(), userIDStr)
		if err != nil {
			// 使用用户日志助手
			userLogger := applogger.NewUserLogger(s.logger.Desugar(), video.UserID)
			userLogger.Warnm("检查用户上传权限失败",
				map[string]interface{}{
					"video_id": video.VideoID,
					"error":    err,
				})
			// 权限检查失败时继续执行（容错）
		} else if !canUpload {
			// 无自动上传权限，跳过此视频（保持状态 200，不改为 205）
			// 用户可以在"定时上传"页面手动触发上传
			userLogger := applogger.NewUserLogger(s.logger.Desugar(), video.UserID)
			userLogger.TaskLog(video.VideoID, "upload_video", "skipped",
				map[string]interface{}{
					"reason": reason,
					"title":  video.Title,
				})
			return nil
		}
	}

	// 创建用户日志助手
	userLogger := applogger.NewUserLogger(s.logger.Desugar(), video.UserID)

	// 显示调度信息
	if video.ProcessingCompletedAt != nil {
		userLogger.TaskLog(video.VideoID, "upload_video", "started",
			map[string]interface{}{
				"title":        video.Title,
				"completed_at": video.ProcessingCompletedAt.Format("15:04:05"),
			})
	} else {
		userLogger.TaskLog(video.VideoID, "upload_video", "started",
			map[string]interface{}{
				"title": video.Title,
			})
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

	// 重新获取视频信息（因为 executeUploadTask 可能会更新 video 信息，如 SubtitleScheduledAt）
	// 注意：不能直接使用 videos[0]，因为那是旧数据
	var updatedVideo struct {
		ID                  uint
		VideoID             string
		Subtitles           string
		SubtitleScheduledAt *time.Time
	}
	if err := s.Db.Table("cw_saved_videos").Select("id, video_id, subtitles, subtitle_scheduled_at").Where("id = ?", video.ID).First(&updatedVideo).Error; err != nil {
		s.logger.Warnf("⚠️ 重新获取视频信息失败: %v", err)
	} else {
		// 更新本地变量以反映最新状态
		video.Subtitles = updatedVideo.Subtitles
		// 如果上传任务设置了计划时间，也认为是有的
		if updatedVideo.SubtitleScheduledAt != nil {
			// 如果数据库中有计划时间，我们应该尊重它
			// 这里我们不需要手动更新 video.Subtitles，只需要确保下面的 checks 能通过
		}
	}

	// 上传成功，检查是否有字幕数据 OR 是否有计划的字幕上传
	hasSubtitle := video.Subtitles != "" && video.Subtitles != "[]" && video.Subtitles != "null"
	hasScheduledSubtitle := updatedVideo.SubtitleScheduledAt != nil

	if hasSubtitle || hasScheduledSubtitle {
		// 有字幕或已有计划，更新状态为 '300'
		// 每次视频上传成功，都重新计算字幕上传延迟时间
		subtitleScheduledAt := now.Add(time.Duration(subtitleUploadDelay) * time.Minute)
		updates := map[string]interface{}{
			"status":                "300",
			"video_uploaded_at":     now,
			"subtitle_scheduled_at": subtitleScheduledAt,
		}

		userLogger.TaskLog(video.VideoID, "upload_video", "success",
			map[string]interface{}{
				"subtitle_scheduled_at": subtitleScheduledAt.Format("15:04:05"),
				"delay_minutes":         subtitleUploadDelay,
			})

		s.Db.Table("cw_saved_videos").Where("id = ?", video.ID).Updates(updates)
	} else {
		// 无字幕且无计划，直接标记为完成状态 '400'
		s.Db.Table("cw_saved_videos").Where("id = ?", video.ID).Updates(map[string]interface{}{
			"status":            "400",
			"video_uploaded_at": now,
		})
		userLogger.TaskLog(video.VideoID, "upload_video", "completed",
			map[string]interface{}{
				"has_subtitle": false,
			})

		// 将"上传字幕到Bilibili"任务步骤标记为跳过，避免UI显示waiting
		if s.TaskStepService != nil {
			s.TaskStepService.UpdateTaskStepStatus(video.VideoID, "上传字幕到Bilibili", "skipped", "视频没有字幕数据")
		}

		// 触发自动清理
		s.triggerAutoCleanup(video.VideoID)
	}
	return nil
}

// uploadNextSubtitle 上传下一个待上传字幕的视频
// 使用智能延迟策略：根据视频大小计算延迟时间
func (s *UploadScheduler) uploadNextSubtitle() error {
	// 使用智能日志器过滤噪音日志
	smartLogger := logger.NewSmartLogger(s.logger)

	// 查询状态为 '300' (视频已上传，待上传字幕) 且已到达计划上传时间的视频
	var videos []struct {
		ID                    uint
		UserID                uint // 用户ID
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
		Select("id, user_id, video_id, title, video_size_mb, subtitle_scheduled_at, subtitle_upload_retries, updated_at, created_at").
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
		// 检查是否有待上传但还没到时间的字幕
		var pendingCount int64
		s.Db.Table("cw_saved_videos").
			Where("status = ?", "300").
			Where("deleted_at IS NULL").
			Where("subtitle_upload_retries < ?", 3).
			Count(&pendingCount)

		if pendingCount > 0 {
			// 查询下一个计划时间
			var nextVideo struct {
				SubtitleScheduledAt *time.Time
			}
			s.Db.Table("cw_saved_videos").
				Select("subtitle_scheduled_at").
				Where("status = ?", "300").
				Where("deleted_at IS NULL").
				Where("subtitle_upload_retries < ?", 3).
				Order("COALESCE(subtitle_scheduled_at, updated_at) ASC").
				Limit(1).
				Find(&nextVideo)

			if nextVideo.SubtitleScheduledAt != nil && nextVideo.SubtitleScheduledAt.After(now) {
				remaining := nextVideo.SubtitleScheduledAt.Sub(now).Round(time.Second)
				smartLogger.Debugf("有 %d 个待上传字幕，下一个将在 %s 后上传", pendingCount, remaining)
			} else {
				smartLogger.Debug("没有待上传字幕的视频")
			}
		} else {
			smartLogger.Debug("没有待上传字幕的视频")
		}
		return nil
	}

	video := videos[0]

	// 创建用户日志助手
	userLogger := applogger.NewUserLogger(s.logger.Desugar(), video.UserID)

	// 显示调度信息
	if video.SubtitleScheduledAt != nil {
		userLogger.TaskLog(video.VideoID, "upload_subtitle", "started",
			map[string]interface{}{
				"title":        video.Title,
				"scheduled_at": video.SubtitleScheduledAt.Format("15:04:05"),
				"retry":        video.SubtitleUploadRetries,
			})
	} else {
		userLogger.TaskLog(video.VideoID, "upload_subtitle", "started",
			map[string]interface{}{
				"title": video.Title,
				"retry": video.SubtitleUploadRetries,
			})
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
			userLogger.TaskLog(video.VideoID, "upload_subtitle", "failed",
				map[string]interface{}{
					"reason": "max_retries_exceeded",
					"error":  err.Error(),
				})
		} else {
			userLogger.TaskLog(video.VideoID, "upload_subtitle", "retry",
				map[string]interface{}{
					"next_retry": nextRetryTime.Format("15:04:05"),
					"retry":      retryCount,
					"error":      err.Error(),
				})
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

	userLogger.TaskLog(video.VideoID, "upload_subtitle", "success", map[string]interface{}{})

	// 触发自动清理（如果启用）
	s.triggerAutoCleanup(video.VideoID)

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

	// 检查是否已被取消
	if s.CancelManager != nil {
		// 使用 IsCanceled 检查任务是否已被注销
		if canceled, exists := s.CancelManager.GetCancelFunc(savedVideo.ID); !exists || canceled == nil {
			s.logger.Infof("⛔ 任务已被取消，跳过上传: VideoID=%s", videoID)
			return fmt.Errorf("任务已被用户取消")
		}
	}

	// 获取当前目录
	currentDir, err := filepath.Abs(s.App.Config.FileUpDir)
	if err != nil {
		return fmt.Errorf("获取文件上传目录失败: %v", err)
	}

	// 创建状态管理器
	stateManager := manager.NewStateManager(savedVideo.ID, savedVideo.UserID, savedVideo.VideoID, currentDir, savedVideo.CreatedAt)

	// 更新步骤状态为运行中
	if err := s.TaskStepService.UpdateTaskStepStatus(videoID, taskName, "running"); err != nil {
		s.logger.Errorf("更新任务步骤状态失败: %v", err)
	}

	// 创建任务链
	chain := manager.NewTaskChain()
	chain.SetLogger(s.logger)
	chain.SetVideoID(videoID)

	// 预填充 CompletedTasks，跳过依赖检查
	// UploadScheduler 调度的任务已经通过状态检查（status=200）确保前置条件满足
	chain.SetCompletedTasks([]string{
		"获取元数据",
		"下载视频",
		"下载字幕",
		"下载封面",
		"翻译字幕",
		"AI增强元数据",
		"确认元数据",
	})

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

	// 执行上传任务
	err := s.executeUploadTask(videoID, taskName)
	if err != nil {
		return err
	}

	// 上传成功后的处理
	s.logger.Infof("✅ 手动上传视频成功: %s", videoID)

	// 获取视频信息以决定下一步
	savedVideo, getErr := s.SavedVideoService.GetVideoByVideoID(videoID)
	if getErr != nil {
		s.logger.Warnf("获取视频信息失败: %v", getErr)
		return nil // 上传已成功，不返回错误
	}

	// 检查是否有字幕需要上传
	hasSubtitle := savedVideo.Subtitles != "" && savedVideo.Subtitles != "[]" && savedVideo.Subtitles != "null"

	if taskType == "video" {
		if hasSubtitle {
			// 有字幕，更新状态为等待字幕上传
			subtitleDelay := 5 // 默认 5 分钟
			if s.App.Config != nil && s.App.Config.DownloadConfig != nil && s.App.Config.DownloadConfig.SubtitleUploadDelay > 0 {
				subtitleDelay = s.App.Config.DownloadConfig.SubtitleUploadDelay
			}
			subtitleScheduledAt := time.Now().Add(time.Duration(subtitleDelay) * time.Minute)

			s.Db.Table("cw_saved_videos").Where("id = ?", savedVideo.ID).Updates(map[string]interface{}{
				"status":                "300",
				"video_uploaded_at":     time.Now(),
				"subtitle_scheduled_at": subtitleScheduledAt,
			})
			s.logger.Infof("📝 有字幕待上传，计划时间: %s", subtitleScheduledAt.Format("15:04:05"))
		} else {
			// 无字幕，直接标记为完成并触发清理
			s.Db.Table("cw_saved_videos").Where("id = ?", savedVideo.ID).Updates(map[string]interface{}{
				"status":            "400",
				"video_uploaded_at": time.Now(),
			})
			s.logger.Infof("✅ 视频无字幕，已标记为完成")
			// 触发自动清理
			s.triggerAutoCleanup(videoID)
		}
	} else if taskType == "subtitle" {
		// 字幕上传成功，标记为完成
		s.Db.Table("cw_saved_videos").Where("id = ?", savedVideo.ID).Updates(map[string]interface{}{
			"status":                "400",
			"subtitle_upload_error": "",
		})
		s.logger.Infof("✅ 字幕上传成功，已标记为完成")
		// 触发自动清理
		s.triggerAutoCleanup(videoID)
	}

	return nil
}

// findCoverImage 在指定目录中查找封面图片
// 优先查找 cover.webp，然后是 YouTube 缩略图，最后查找任意图片
func (s *UploadScheduler) findCoverImage(dir string) string {
	// 优先级列表（包含 webp 格式）
	priorityFiles := []string{
		"cover.webp", // yt-dlp 下载的封面
		"cover.jpg",
		"cover.png",
		"maxresdefault.jpg", // YouTube 高清缩略图
		"maxresdefault.webp",
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

	// 如果都没找到，查找任意图片文件（包括 webp）
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := filepath.Ext(name)
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
			return filepath.Join(dir, name)
		}
	}

	return ""
}

// triggerAutoCleanup 触发自动清理（如果启用）
// 清理模式由配置决定：immediate=立即清理，delayed=延迟清理（默认60分钟）
func (s *UploadScheduler) triggerAutoCleanup(videoID string) {
	// 检查是否启用自动清理
	if s.App.Config == nil || s.App.Config.DownloadConfig == nil {
		return
	}

	config := s.App.Config.DownloadConfig
	if !config.AutoCleanupEnabled {
		return
	}

	// 获取清理模式和延迟时间
	cleanupMode := config.AutoCleanupMode
	if cleanupMode == "" {
		cleanupMode = "delayed" // 默认延迟清理
	}
	cleanupDelay := config.AutoCleanupDelay
	if cleanupDelay <= 0 {
		cleanupDelay = 60 // 默认60分钟
	}

	s.logger.Debugf("🔧 清理配置: mode=%s, delay=%d分钟", cleanupMode, cleanupDelay)

	if cleanupMode == "immediate" {
		// 立即清理（用户可在config.toml中配置）
		s.logger.Infof("🧹 自动清理: 立即清理 - VideoID: %s", videoID)
		go s.cleanupVideoFiles(videoID)
	} else {
		// 延迟清理（默认）
		s.logger.Infof("🧹 自动清理: 将在%d分钟后清理 - VideoID: %s", cleanupDelay, videoID)
		go func(vid string, delay int) {
			time.Sleep(time.Duration(delay) * time.Minute)
			s.cleanupVideoFiles(vid)
		}(videoID, cleanupDelay)
	}
}

// cleanupVideoFiles 清理指定视频的所有媒体文件
func (s *UploadScheduler) cleanupVideoFiles(videoID string) {
	s.logger.Infof("🧹 开始清理视频文件: %s", videoID)

	// 获取视频信息以确定目录
	savedVideo, err := s.SavedVideoService.GetVideoByVideoID(videoID)
	if err != nil {
		s.logger.Warnf("⚠️ 获取视频信息失败，无法清理: %v", err)
		return
	}

	// 构建视频目录路径
	baseDir, err := filepath.Abs(s.App.Config.FileUpDir)
	if err != nil {
		s.logger.Warnf("⚠️ 获取文件上传目录失败: %v", err)
		return
	}

	// 视频目录格式: data/media/user_{id}/YYYY-MM-DD/videoID
	userDir := fmt.Sprintf("user_%d", savedVideo.UserID)
	// 注意：使用 Local() 确保日期与创建目录时一致
	dateStr := savedVideo.CreatedAt.Local().Format("2006-01-02")

	// 如果目录不存在，尝试查找其他可能的日期目录（处理时区问题）
	// 但首先尝试标准的构建路径
	videoDir := filepath.Join(baseDir, userDir, dateStr, videoID)

	// 增加调试日志
	s.logger.Infof("🧹 目标清理路径: %s", videoDir)

	// 检查目录是否存在
	if _, err := os.Stat(videoDir); os.IsNotExist(err) {
		s.logger.Debugf("📂 视频目录不存在，无需清理: %s", videoDir)
		return
	}

	// 统计文件
	var fileCount int
	var totalSize int64

	entries, err := os.ReadDir(videoDir)
	if err != nil {
		s.logger.Warnf("⚠️ 读取目录失败: %v", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			if info, err := entry.Info(); err == nil {
				fileCount++
				totalSize += info.Size()
			}
		}
	}

	// 删除目录及其所有内容
	if err := os.RemoveAll(videoDir); err != nil {
		s.logger.Errorf("❌ 清理视频目录失败: %v", err)
		return
	}

	// 更新数据库记录（标记为已清理）
	s.Db.Table("cw_saved_videos").Where("video_id = ?", videoID).Updates(map[string]interface{}{
		"files_cleaned":    true,
		"files_cleaned_at": time.Now(),
	})

	s.logger.Infof("✅ 已清理视频 %s: 删除 %d 个文件, 释放 %.2f MB",
		videoID, fileCount, float64(totalSize)/(1024*1024))

	// 尝试清理空的父目录 (日期目录)
	dateDirPath := filepath.Dir(videoDir)
	if entries, err := os.ReadDir(dateDirPath); err == nil && len(entries) == 0 {
		if err := os.Remove(dateDirPath); err == nil {
			s.logger.Debugf("🧹 已清理空日期目录: %s", dateDirPath)

			// 尝试清理空的父目录 (用户目录)
			userDirPath := filepath.Dir(dateDirPath)
			if entries, err := os.ReadDir(userDirPath); err == nil && len(entries) == 0 {
				if err := os.Remove(userDirPath); err == nil {
					s.logger.Debugf("🧹 已清理空用户目录: %s", userDirPath)
				}
			}
		}
	}
}
