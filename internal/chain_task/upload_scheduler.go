package chain_task

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/difyz9/ytb2bili/internal/chain_task/handlers"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/core/types"
	applogger "github.com/difyz9/ytb2bili/internal/logger"
	"github.com/difyz9/ytb2bili/internal/membership"
	"github.com/difyz9/ytb2bili/pkg/audit"
	"github.com/difyz9/ytb2bili/pkg/logger"
	"github.com/difyz9/ytb2bili/pkg/store/model"

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
	AuditService       *audit.AuditService // 审计服务
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
	auditService *audit.AuditService, // 审计服务
	cancelManager *TaskCancelManager,
) *UploadScheduler {
	return &UploadScheduler{
		App:                app,
		Task:               task,
		Db:                 db,
		SavedVideoService:  savedVideoService,
		TaskStepService:    taskStepService,
		BiliAccountService: biliAccountService,
		AuditService:       auditService,
		logger:             app.Logger,
		CancelManager:      cancelManager,
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
		// 使用智能日志器，自动过滤进度活动期间的噪音日志
		smartLogger := logger.NewSmartLogger(s.logger)

		// 检查是否启用自动上传
		autoUploadEnabled := true
		enableFineGrainedLock := true // 默认启用细粒度锁
		if s.App.Config != nil && s.App.Config.DownloadConfig != nil {
			autoUploadEnabled = s.App.Config.DownloadConfig.AutoUploadEnabled
			enableFineGrainedLock = s.App.Config.DownloadConfig.EnableFineGrainedLock
		}

		if enableFineGrainedLock {
			// 新逻辑：细粒度事务锁（不同 videoID 可以并发）
			if autoUploadEnabled {
				// 1. 检查是否需要上传视频（使用细粒度事务锁，不阻塞其他 videoID）
				if err := s.uploadNextVideoWithLock(); err != nil {
					smartLogger.Errorf("上传视频失败: %v", err)
				}
			}

			// 2. 检查是否需要上传字幕（使用细粒度事务锁，不阻塞其他 videoID）
			if err := s.uploadNextSubtitleWithLock(); err != nil {
				smartLogger.Errorf("上传字幕失败: %v", err)
			}
		} else {
			// 旧逻辑：全局锁（兼容模式，可通过配置回滚）
			s.mutex.Lock()
			defer s.mutex.Unlock()

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

	// ═══════════════════════════════════════════════════════════════
	// 自动清理定时任务配置
	// ═══════════════════════════════════════════════════════════════
	if s.App.Config != nil && s.App.Config.DownloadConfig != nil && s.App.Config.DownloadConfig.AutoCleanupEnabled {
		// 任务1: 每分钟检查并执行到期的清理任务（即时响应）
		// 处理由 triggerAutoCleanup() 调度的延迟清理（30分钟后）
		s.Task.AddFunc("0 * * * * *", func() {
			s.executeScheduledCleanup()
		})

		// 任务2: 每天凌晨2点扫描并标记待清理的旧任务（批量清理）
		// 根据配置的天数，将满足条件的任务标记为 scheduled
		s.Task.AddFunc("0 0 2 * * *", func() {
			s.markFilesForCleanup()
		})

		cleanupDelayDays := s.App.Config.DownloadConfig.AutoCleanupDelay / (24 * 60) // 分钟转天
		if cleanupDelayDays < 1 {
			cleanupDelayDays = 1
		}
		softDeleteMode := "软删除"
		if !s.App.Config.DownloadConfig.SoftDelete {
			softDeleteMode = "永久删除"
		}
		s.logger.Infof("🗑️ 自动清理已启用: 每分钟检查 + %d 天批量%s", cleanupDelayDays, softDeleteMode)

		// 启动时扫描并清理那些应该被清理但因重启而丢失的任务
		go s.recoverPendingCleanups()
	}
}

// uploadNextVideo 上传下一个准备好的视频（旧版本：无锁）
// 根据配置的上传模式（immediate/delayed）决定是否立即上传
// 注意：这个函数不使用锁，仅用于兼容模式（全局锁在调用处控制）
func (s *UploadScheduler) uploadNextVideo() error {
	// 旧逻辑：保持不变，用于兼容模式
	return s.uploadNextVideoImpl(false)
}

// uploadNextVideoWithLock 上传下一个准备好的视频（新版本：使用事务锁）
// 根据配置的上传模式（immediate/delayed）决定是否立即上传
func (s *UploadScheduler) uploadNextVideoWithLock() error {
	return s.uploadNextVideoImpl(true)
}

// uploadNextVideoImpl 上传下一个准备好的视频（实现）
func (s *UploadScheduler) uploadNextVideoImpl(useTransactionLock bool) error {
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

	// 根据配置决定是否使用事务锁
	var tx *gorm.DB
	if useTransactionLock {
		// 使用事务 + 行锁，防止并发时多个 goroutine 获取到同一个视频
		// 这是细粒度锁，不同 videoID 可以并发处理
		tx = s.Db.Begin()
		defer tx.Rollback()
	} else {
		// 不使用事务（兼容模式）
		tx = s.Db
	}

	// 查询状态为 '200' (准备就绪) 的视频，使用行锁
	var videos []struct {
		ID                    uint
		UserID                uint // 用户ID，用于权限检查
		VideoID               string
		Title                 string
		Subtitles             string // 字幕数据，用于判断是否有字幕
		ProcessingCompletedAt *time.Time
		CreatedAt             time.Time
	}

	query := tx.Table("cw_saved_videos").
		Select("id, user_id, video_id, title, subtitles, processing_completed_at, created_at").
		Where("status = ?", "200").
		Where("deleted_at IS NULL")

	// 根据上传模式添加时间条件
	if uploadMode == "delayed" {
		// 延迟模式：只查询 processing_completed_at + delay <= now 的视频
		delayDuration := time.Duration(videoUploadDelay) * time.Minute
		query = query.Where("processing_completed_at IS NOT NULL AND processing_completed_at <= ?", now.Add(-delayDuration))
	}

	// 使用 FOR UPDATE 行锁，防止其他事务获取同一行
	err := query.Order("COALESCE(processing_completed_at, created_at) ASC").
		Limit(1).
		Find(&videos).Error

	if err != nil {
		return fmt.Errorf("查询待上传视频失败: %v", err)
	}

	if len(videos) == 0 {
		if useTransactionLock {
			tx.Rollback()
		}
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

	// 检查用户自动上传权限（在事务内，如果无权限则回滚）
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
			// 无自动上传权限，回滚事务，跳过此视频（保持状态 200）
			// 用户可以在"定时上传"页面手动触发上传
			userLogger := applogger.NewUserLogger(s.logger.Desugar(), video.UserID)
			userLogger.TaskLog(video.VideoID, "upload_video", "skipped",
				map[string]interface{}{
					"reason": reason,
					"title":  video.Title,
				})
			if useTransactionLock {
				tx.Rollback()
			}
			return nil
		}
	}

	// 立即更新状态为 '201' (上传视频中)，防止其他 goroutine 获取到同一个视频
	if err := tx.Model(&model.SavedVideo{}).Where("id = ?", video.ID).Update("status", "201").Error; err != nil {
		return fmt.Errorf("更新视频状态失败: %v", err)
	}

	// 提交事务，释放行锁
	if useTransactionLock {
		if err := tx.Commit().Error; err != nil {
			return fmt.Errorf("提交事务失败: %v", err)
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

// uploadNextSubtitle 上传下一个待上传字幕的视频（旧版本：无锁）
// 使用智能延迟策略：根据视频大小计算延迟时间
// 注意：这个函数不使用锁，仅用于兼容模式（全局锁在调用处控制）
func (s *UploadScheduler) uploadNextSubtitle() error {
	// 旧逻辑：保持不变，用于兼容模式
	return s.uploadNextSubtitleImpl(false)
}

// uploadNextSubtitleWithLock 上传下一个待上传字幕的视频（新版本：使用事务锁）
// 使用智能延迟策略：根据视频大小计算延迟时间
func (s *UploadScheduler) uploadNextSubtitleWithLock() error {
	return s.uploadNextSubtitleImpl(true)
}

// uploadNextSubtitleImpl 上传下一个待上传字幕的视频（实现）
func (s *UploadScheduler) uploadNextSubtitleImpl(useTransactionLock bool) error {
	// 使用智能日志器过滤噪音日志
	smartLogger := logger.NewSmartLogger(s.logger)

	// 根据配置决定是否使用事务锁
	var tx *gorm.DB
	if useTransactionLock {
		// 使用事务 + 行锁，防止并发时多个 goroutine 获取到同一个视频
		tx = s.Db.Begin()
		defer tx.Rollback()
	} else {
		// 不使用事务（兼容模式）
		tx = s.Db
	}

	now := time.Now()

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

	// 查询已到达计划上传时间的视频，或者没有设置计划时间但已过去1小时的视频
	err := tx.Table("cw_saved_videos").
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
		if useTransactionLock {
			tx.Rollback()
		}
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

	// 立即更新状态为 '301' (上传字幕中)，防止其他 goroutine 获取到同一个视频
	if err := tx.Model(&model.SavedVideo{}).Where("id = ?", video.ID).Update("status", "301").Error; err != nil {
		return fmt.Errorf("更新视频状态失败: %v", err)
	}

	// 提交事务，释放行锁
	if useTransactionLock {
		if err := tx.Commit().Error; err != nil {
			return fmt.Errorf("提交事务失败: %v", err)
		}
	}

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

	// 执行上传字幕任务
	if err := s.executeUploadTask(video.VideoID, "上传字幕到Bilibili"); err != nil {
		errMsg := err.Error()

		// 检查是否是"审核中"状态的特殊错误
		isUnderReview := strings.Contains(errMsg, "ERR_VIDEO_UNDER_REVIEW")

		// 上传失败，增加重试次数并设置下次重试时间
		retryCount := video.SubtitleUploadRetries + 1

		// 根据错误类型计算重试延迟
		var nextRetryTime time.Time
		var maxRetries int

		if isUnderReview {
			// 审核中的视频：使用较长的固定延迟（10分钟），不使用指数退避
			// 最大重试次数设置为30次（5小时），足以覆盖大多数审核场景
			maxRetries = 30
			nextRetryTime = time.Now().Add(10 * time.Minute)
			s.logger.Infof("⏳ 视频审核中，将在10分钟后重试 (第%d次)", retryCount)
		} else {
			// 其他错误：使用指数退避策略，最大3次重试
			maxRetries = 3
			nextRetryTime = s.calculateNextRetryTime(retryCount)
		}

		s.Db.Table("cw_saved_videos").Where("id = ?", video.ID).Updates(map[string]interface{}{
			"status":                  "300", // 保持待上传状态
			"subtitle_upload_retries": retryCount,
			"subtitle_scheduled_at":   nextRetryTime,
			"subtitle_upload_error":   errMsg,
		})

		if retryCount >= maxRetries {
			// 超过最大重试次数，标记为失败
			s.SavedVideoService.UpdateStatus(video.ID, "399")
			userLogger.TaskLog(video.VideoID, "upload_subtitle", "failed",
				map[string]interface{}{
					"reason":      "max_retries_exceeded",
					"error":       errMsg,
					"retry_count": retryCount,
				})
			return fmt.Errorf("超过最大重试次数(%d次): %v", maxRetries, err)
		} else {
			userLogger.TaskLog(video.VideoID, "upload_subtitle", "retry",
				map[string]interface{}{
					"next_retry": nextRetryTime.Format("15:04:05"),
					"retry":      retryCount,
					"error":      errMsg,
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

	// 注册任务到取消管理器，获取可取消的 context
	var ctx context.Context
	if s.CancelManager != nil {
		// 注册任务
		runID := "upload_" + taskName
		var allowed bool
		ctx, allowed = s.CancelManager.Register(savedVideo.ID, runID)
		if !allowed {
			return fmt.Errorf("任务被取消")
		}
		defer s.CancelManager.Deregister(savedVideo.ID, runID)
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
	if ctx != nil {
		chain.SetContext(ctx)
	}

	// 预填充 CompletedTasks，跳过依赖检查
	// UploadScheduler 调度的任务已经通过状态检查（status=200）确保前置条件满足
	// 预填充 CompletedTasks，跳过依赖检查
	// UploadScheduler 调度的任务已经通过状态检查（status=200）确保前置条件满足
	completedTasks := []string{
		"获取元数据",
		"下载视频",
		"下载字幕",
		"下载封面",
		"翻译字幕",
		"AI增强元数据",
		"确认元数据",
	}

	// 如果是上传字幕任务，需要标记"上传到Bilibili"为已完成
	if taskName == "上传字幕到Bilibili" {
		completedTasks = append(completedTasks, "上传到Bilibili")
	}

	chain.SetCompletedTasks(completedTasks)

	var task types.Task

	// 根据任务名称创建对应的任务
	switch taskName {
	case "上传到Bilibili":
		task = handlers.NewUploadToBilibili("上传到Bilibili", s.App, stateManager, s.App.CosClient, s.SavedVideoService, s.BiliAccountService, s.AuditService)
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
//
// 调用时机（已确保在字幕上传完成后）：
// - 视频上传完成且无字幕
// - 字幕上传成功
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

	// 获取视频信息
	savedVideo, err := s.SavedVideoService.GetVideoByVideoID(videoID)
	if err != nil {
		s.logger.Warnf("⚠️ 获取视频信息失败，无法调度清理: %v", err)
		return
	}

	if cleanupMode == "immediate" {
		// 立即清理（用户可在config.toml中配置）
		s.logger.Infof("🧹 自动清理: 立即清理 - VideoID: %s", videoID)
		// 使用数据库原子操作，避免重复清理
		result := s.Db.Model(&model.SavedVideo{}).
			Where("id = ? AND files_cleaned = ?", savedVideo.ID, false).
			Updates(map[string]interface{}{
				"files_cleanup_status":       "scheduled",
				"files_cleanup_scheduled_at": time.Now(), // 立即执行
			})

		if result.Error != nil {
			s.logger.Errorf("❌ 调度立即清理失败: %v", result.Error)
			return
		}

		if result.RowsAffected > 0 {
			s.logger.Infof("✅ 已调度立即清理: %s", videoID)
			// 触发清理执行（异步）
			go s.cleanupVideoFiles(videoID)
		}
	} else {
		// ═══════════════════════════════════════════════════════════════
		// 延迟清理（持久化到数据库，重启后仍能执行）
		// ═══════════════════════════════════════════════════════════════
		scheduledTime := time.Now().Add(time.Duration(cleanupDelay) * time.Minute)
		s.logger.Infof("🧹 自动清理: 将在%d分钟后清理 (%s) - VideoID: %s",
			cleanupDelay, scheduledTime.Format("15:04:05"), videoID)

		// 使用数据库原子操作，避免重复调度
		result := s.Db.Model(&model.SavedVideo{}).
			Where("id = ? AND files_cleaned = ?", savedVideo.ID, false).
			Updates(map[string]interface{}{
				"files_cleanup_status":       "scheduled",
				"files_cleanup_scheduled_at": scheduledTime,
			})

		if result.Error != nil {
			s.logger.Errorf("❌ 调度延迟清理失败: %v", result.Error)
			return
		}

		if result.RowsAffected == 0 {
			s.logger.Debugf("📂 任务已被调度或已清理，跳过: %s", videoID)
		} else {
			s.logger.Infof("✅ 已调度延迟清理: %s (将于 %s 执行)", videoID, scheduledTime.Format("15:04:05"))
		}
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
	now := time.Now()
	s.Db.Table("cw_saved_videos").Where("video_id = ?", videoID).Updates(map[string]interface{}{
		"files_cleaned":              true,
		"files_cleaned_at":           now,
		"files_cleanup_status":       "completed",
		"files_cleanup_scheduled_at": nil, // 清空预定时间
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

// recoverPendingCleanups 恢复因重启而丢失的待清理任务
// 在启动时扫描 status=400 且 files_cleaned=false 的视频，并触发清理
func (s *UploadScheduler) recoverPendingCleanups() {
	// 等待2秒让系统完全启动
	time.Sleep(2 * time.Second)

	s.logger.Info("🔍 启动时扫描：检查因重启而丢失的待清理任务...")

	// 获取清理延迟配置
	cleanupDelay := 30 // 默认30分钟
	if s.App.Config != nil && s.App.Config.DownloadConfig != nil && s.App.Config.DownloadConfig.AutoCleanupDelay > 0 {
		cleanupDelay = s.App.Config.DownloadConfig.AutoCleanupDelay
	}

	// 计算截止时间：如果视频完成时间超过配置的延迟时间，说明应该被清理了
	cutoffTime := time.Now().Add(-time.Duration(cleanupDelay) * time.Minute)

	// 查找符合条件的视频：
	// 1. status = 400（已完成）
	// 2. files_cleaned = false（未清理）
	// 3. updated_at < cutoffTime（完成时间已超过延迟时间）
	var videos []struct {
		VideoID   string
		Title     string
		UpdatedAt time.Time
	}

	err := s.Db.Table("cw_saved_videos").
		Select("video_id, title, updated_at").
		Where("status = ?", "400").
		Where("files_cleaned = ?", false).
		Where("deleted_at IS NULL").
		Where("updated_at < ?", cutoffTime).
		Find(&videos).Error

	if err != nil {
		s.logger.Errorf("❌ 查询待清理视频失败: %v", err)
		return
	}

	if len(videos) == 0 {
		s.logger.Info("✅ 启动时扫描完成：没有需要恢复清理的任务")
		return
	}

	s.logger.Infof("🧹 发现 %d 个因重启而遗漏的待清理任务，开始处理...", len(videos))

	// 对每个视频触发清理
	for _, video := range videos {
		s.logger.Infof("🧹 恢复清理任务: %s (%s)", video.Title, video.VideoID)
		s.cleanupVideoFiles(video.VideoID)
	}

	s.logger.Infof("✅ 启动时清理恢复完成：已处理 %d 个任务", len(videos))
}

// markFilesForCleanup 标记符合条件的任务文件为待清理状态
// 每天凌晨2点执行，将满足保留期的任务标记为 scheduled 状态
func (s *UploadScheduler) markFilesForCleanup() {
	s.logger.Info("🔍 开始扫描待清理的任务文件...")

	// 获取清理延迟配置（分钟转天）
	cleanupDelayDays := 7 // 默认7天
	if s.App.Config != nil && s.App.Config.DownloadConfig != nil {
		delayMinutes := s.App.Config.DownloadConfig.AutoCleanupDelay
		if delayMinutes > 0 {
			cleanupDelayDays = delayMinutes / (24 * 60)
			if cleanupDelayDays < 1 {
				cleanupDelayDays = 1
			}
		}
	}

	// 计算截止时间（N天前完成的任务）
	cutoffTime := time.Now().AddDate(0, 0, -cleanupDelayDays)
	scheduledTime := time.Now().Add(24 * time.Hour) // 24小时后执行清理

	// 查找符合条件的记录：
	// 1. 状态为 400（已上传完成）
	// 2. files_cleaned = false（尚未清理）
	// 3. files_cleanup_status = 'pending'（未标记）
	// 4. updated_at 早于截止时间
	result := s.Db.Table("cw_saved_videos").
		Where("status = ?", "400").
		Where("files_cleaned = ?", false).
		Where("(files_cleanup_status = ? OR files_cleanup_status IS NULL)", "pending").
		Where("updated_at < ?", cutoffTime).
		Updates(map[string]interface{}{
			"files_cleanup_status":       "scheduled",
			"files_cleanup_scheduled_at": scheduledTime,
		})

	if result.Error != nil {
		s.logger.Errorf("❌ 标记待清理任务失败: %v", result.Error)
		return
	}

	if result.RowsAffected > 0 {
		s.logger.Infof("✅ 已标记 %d 个任务文件为待清理（将于明天凌晨3点执行）", result.RowsAffected)
	} else {
		s.logger.Debug("📂 没有需要标记清理的任务")
	}
}

// executeScheduledCleanup 执行已计划的清理任务
// 每天凌晨3点执行，处理 scheduled 状态的任务
func (s *UploadScheduler) executeScheduledCleanup() {
	s.logger.Info("🗑️ 开始执行计划清理任务...")

	// 查找所有标记为 scheduled 且计划时间已到的记录
	var videos []model.SavedVideo
	err := s.Db.Where("files_cleanup_status = ?", "scheduled").
		Where("files_cleanup_scheduled_at <= ?", time.Now()).
		Find(&videos).Error

	if err != nil {
		s.logger.Errorf("❌ 查询待清理任务失败: %v", err)
		return
	}

	if len(videos) == 0 {
		s.logger.Debug("📂 没有需要执行清理的任务")
		return
	}

	s.logger.Infof("📋 发现 %d 个待清理任务", len(videos))

	// 获取软删除配置
	softDelete := true
	cleanupDir := ""
	if s.App.Config != nil && s.App.Config.DownloadConfig != nil {
		softDelete = s.App.Config.DownloadConfig.SoftDelete
		cleanupDir = s.App.Config.DownloadConfig.CleanupDir
	}

	// 如果未配置回收站目录，使用默认目录
	if cleanupDir == "" && s.App.Config != nil {
		cleanupDir = filepath.Join(s.App.Config.FileUpDir, ".cleanup")
	}

	// 确保回收站目录存在（仅软删除模式）
	if softDelete && cleanupDir != "" {
		if err := os.MkdirAll(cleanupDir, 0755); err != nil {
			s.logger.Errorf("❌ 创建回收站目录失败: %v", err)
			return
		}
	}

	cleanedCount := 0
	for _, video := range videos {
		if s.softCleanupVideoFiles(video.VideoID, softDelete, cleanupDir) {
			cleanedCount++
			// 更新数据库状态
			s.Db.Table("cw_saved_videos").Where("video_id = ?", video.VideoID).Updates(map[string]interface{}{
				"files_cleanup_status": "completed",
				"files_cleaned":        true,
				"files_cleaned_at":     time.Now(),
			})
		}
	}

	s.logger.Infof("✅ 清理完成: 成功清理 %d/%d 个任务", cleanedCount, len(videos))
}

// softCleanupVideoFiles 清理指定视频的文件（带软删除选项）
// softDelete=true 时移动到回收站，否则永久删除
func (s *UploadScheduler) softCleanupVideoFiles(videoID string, softDelete bool, cleanupDir string) bool {
	// 获取视频信息以确定目录
	video, err := s.SavedVideoService.GetVideoByVideoID(videoID)
	if err != nil {
		s.logger.Warnf("⚠️ 获取视频信息失败: %s - %v", videoID, err)
		return false
	}

	// 构建视频目录路径（与现有方法一致）
	baseDir, err := filepath.Abs(s.App.Config.FileUpDir)
	if err != nil {
		s.logger.Warnf("⚠️ 获取文件上传目录失败: %v", err)
		return false
	}
	userDir := fmt.Sprintf("user_%d", video.UserID)
	dateDir := video.CreatedAt.Local().Format("2006-01-02")
	videoDir := filepath.Join(baseDir, userDir, dateDir, videoID)

	// 检查目录是否存在
	if _, err := os.Stat(videoDir); os.IsNotExist(err) {
		s.logger.Debugf("📂 视频目录不存在，跳过: %s", videoDir)
		return true // 目录不存在也算清理成功
	}

	// 统计文件
	var fileCount int
	var totalSize int64
	entries, err := os.ReadDir(videoDir)
	if err != nil {
		s.logger.Warnf("⚠️ 读取目录失败: %v", err)
		return false
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			if info, err := entry.Info(); err == nil {
				fileCount++
				totalSize += info.Size()
			}
		}
	}

	if softDelete && cleanupDir != "" {
		// 软删除：移动到回收站
		destDir := filepath.Join(cleanupDir, time.Now().Format("2006-01-02"), videoID)
		if err := os.MkdirAll(filepath.Dir(destDir), 0755); err != nil {
			s.logger.Errorf("❌ 创建回收站子目录失败: %v", err)
			return false
		}

		if err := os.Rename(videoDir, destDir); err != nil {
			s.logger.Errorf("❌ 移动到回收站失败: %v", err)
			return false
		}

		s.logger.Infof("♻️ 已移动到回收站 %s: %d 个文件, %.2f MB", videoID, fileCount, float64(totalSize)/(1024*1024))
	} else {
		// 永久删除
		if err := os.RemoveAll(videoDir); err != nil {
			s.logger.Errorf("❌ 永久删除失败: %v", err)
			return false
		}

		s.logger.Infof("🗑️ 已永久删除 %s: %d 个文件, %.2f MB", videoID, fileCount, float64(totalSize)/(1024*1024))
	}

	return true
}
