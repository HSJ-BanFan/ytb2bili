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
	// 每5分钟检查一次是否需要上传
	s.Task.AddFunc("*/5 * * * *", func() {
		s.mutex.Lock()
		defer s.mutex.Unlock()

		now := time.Now()

		// 1. 检查是否需要上传视频（每小时一次）
		if now.Sub(s.lastVideoUploadTime) >= time.Hour {
			s.logger.Info("🔍 检查待上传的视频...")
			if err := s.uploadNextVideo(); err != nil {
				s.logger.Errorf("上传视频失败: %v", err)
			} else {
				s.lastVideoUploadTime = now
			}
		}

		// 2. 检查是否需要上传字幕（视频上传1小时后）
		if now.Sub(s.lastSubtitleUploadTime) >= time.Hour {
			s.logger.Info("🔍 检查待上传字幕的视频...")
			if err := s.uploadNextSubtitle(); err != nil {
				s.logger.Errorf("上传字幕失败: %v", err)
			} else {
				s.lastSubtitleUploadTime = now
			}
		}
	})

	s.logger.Info("✓ Upload scheduler started, checking every 5 minutes")
}

// uploadNextVideo 上传下一个准备好的视频
func (s *UploadScheduler) uploadNextVideo() error {
	// 查询状态为 '200' (准备就绪) 的视频
	var videos []struct {
		ID        uint
		VideoID   string
		Title     string
		CreatedAt time.Time
	}

	err := s.Db.Table("cw_saved_videos").
		Select("id, video_id, title, created_at").
		Where("status = ?", "200").
		Where("deleted_at IS NULL").
		Order("created_at ASC").
		Limit(1).
		Find(&videos).Error

	if err != nil {
		return fmt.Errorf("查询待上传视频失败: %v", err)
	}

	if len(videos) == 0 {
		s.logger.Debug("没有待上传的视频")
		return nil
	}

	video := videos[0]
	s.logger.Infof("📤 开始上传视频: %s (VideoID: %s)", video.Title, video.VideoID)

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

	// 上传成功，更新状态为 '300' (视频已上传，待上传字幕)
	if err := s.SavedVideoService.UpdateStatus(video.ID, "300"); err != nil {
		return fmt.Errorf("更新视频状态失败: %v", err)
	}

	s.logger.Infof("✅ 视频上传成功: %s", video.VideoID)
	return nil
}

// uploadNextSubtitle 上传下一个待上传字幕的视频
func (s *UploadScheduler) uploadNextSubtitle() error {
	// 查询状态为 '300' (视频已上传，待上传字幕) 且上传时间超过1小时的视频
	var videos []struct {
		ID        uint
		VideoID   string
		Title     string
		UpdatedAt time.Time
		CreatedAt time.Time
	}

	oneHourAgo := time.Now().Add(-time.Hour)

	err := s.Db.Table("cw_saved_videos").
		Select("id, video_id, title, updated_at, created_at").
		Where("status = ? AND updated_at <= ?", "300", oneHourAgo).
		Where("deleted_at IS NULL").
		Order("updated_at ASC").
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
	s.logger.Infof("📝 开始上传字幕: %s (VideoID: %s)", video.Title, video.VideoID)

	// 更新状态为 '301' (上传字幕中)
	if err := s.SavedVideoService.UpdateStatus(video.ID, "301"); err != nil {
		return fmt.Errorf("更新视频状态失败: %v", err)
	}

	// 执行上传字幕任务
	if err := s.executeUploadTask(video.VideoID, "上传字幕到Bilibili"); err != nil {
		// 上传失败，更新状态为 '399' (字幕上传失败)
		s.SavedVideoService.UpdateStatus(video.ID, "399")
		return fmt.Errorf("上传字幕失败: %v", err)
	}

	// 上传成功，更新状态为 '400' (全部完成)
	if err := s.SavedVideoService.UpdateStatus(video.ID, "400"); err != nil {
		return fmt.Errorf("更新视频状态失败: %v", err)
	}

	s.logger.Infof("✅ 字幕上传成功: %s", video.VideoID)
	return nil
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
		task = handlers.NewUploadSubtitleToBilibili("上传字幕到Bilibili", s.App, stateManager, s.App.CosClient, s.SavedVideoService)
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
