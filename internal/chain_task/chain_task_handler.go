package chain_task

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/difyz9/ytb2bili/internal/chain_task/handlers"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	models2 "github.com/difyz9/ytb2bili/internal/core/models"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/core/types"
	applogger "github.com/difyz9/ytb2bili/internal/logger"
	"github.com/difyz9/ytb2bili/pkg/logger"
	"github.com/difyz9/ytb2bili/pkg/store/model"

	"sync"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"gorm.io/gorm"
)

// ChainTaskHandler 任务链执行器的实现
type ChainTaskHandler struct {
	App *core.AppServer

	SavedVideoService  *services.SavedVideoService
	TaskStepService    *services.TaskStepService
	BiliAccountService *services.BiliAccountService

	// 并发控制
	workerPool         chan struct{} // 准备阶段并发控制
	downloadWorkerPool chan struct{} // 下载专用并发池（限制并发下载数）
	inFlightTasks      sync.Map      // 防止重复调度 map[videoID]bool
	maxWorkers         int           // 最大并发数
	maxDownloads       int           // 最大并发下载数

	Task  *cron.Cron
	Db    *gorm.DB
	mutex sync.Mutex

	// 任务取消管理器
	CancelManager *TaskCancelManager

	// 用户级并发控制（可选注入）
	ConcurrencyLimiter *ConcurrencyLimiter
}

func NewChainTaskHandler(app *core.AppServer, task *cron.Cron, db *gorm.DB, savedVideoService *services.SavedVideoService, taskStepService *services.TaskStepService, biliAccountService *services.BiliAccountService) *ChainTaskHandler {
	// 从配置读取最大并发数，默认为 10
	maxWorkers := 10
	if app.Config != nil && app.Config.DownloadConfig != nil && app.Config.DownloadConfig.MaxConcurrentTasks > 0 {
		maxWorkers = app.Config.DownloadConfig.MaxConcurrentTasks
	}

	// 从配置读取最大并发下载数，默认为 3
	maxDownloads := 3
	if app.Config != nil && app.Config.DownloadConfig != nil && app.Config.DownloadConfig.MaxConcurrentDownloads > 0 {
		maxDownloads = app.Config.DownloadConfig.MaxConcurrentDownloads
	}

	return &ChainTaskHandler{
		App:                app,
		Task:               task,
		Db:                 db,
		SavedVideoService:  savedVideoService,
		TaskStepService:    taskStepService,
		BiliAccountService: biliAccountService,
		workerPool:         make(chan struct{}, maxWorkers),
		downloadWorkerPool: make(chan struct{}, maxDownloads),
		maxWorkers:         maxWorkers,
		maxDownloads:       maxDownloads,
		mutex:              sync.Mutex{},
		CancelManager:      NewTaskCancelManager(), // 初始化取消管理器
	}
}

// SetUp 启动任务消费者
func (h *ChainTaskHandler) SetUp() {
	// 应用启动时重置所有"运行中"的任务步骤
	h.resetRunningTasksOnStartup()

	h.App.Logger.Infof("✓ 任务调度器启动，准备阶段最大并发: %d，下载最大并发: %d", h.maxWorkers, h.maxDownloads)

	// 添加定时任务（每5秒扫描一次）
	h.Task.AddFunc("*/5 * * * * *", func() {
		// 使用智能日志器，自动过滤进度活动期间的噪音日志
		smartLogger := logger.NewSmartLogger(h.App.Logger)

		// 0. 重置失败步骤为 pending（修复竞态条件）
		// 将 can_retry=true 的 failed 步骤转换为 pending，以便调度器能检测并重试
		if err := h.TaskStepService.ResetFailedStepsToPending(); err != nil {
			smartLogger.Errorf("重置失败步骤为 pending 出错: %v", err)
		}

		// 1. 优先处理重试的任务步骤（排除上传步骤）
		retrySteps, err := h.getRetrySteps()
		if err != nil {
			smartLogger.Errorf("查询重试步骤失败: %v", err)
		} else if len(retrySteps) > 0 {
			smartLogger.Debugf("发现 %d 个待重试的步骤", len(retrySteps))

			for _, step := range retrySteps {
				// 跳过上传相关步骤（由 UploadScheduler 处理）
				if step.StepName == "上传到Bilibili" || step.StepName == "上传字幕到Bilibili" {
					continue
				}

				videoID := step.VideoID
				stepName := step.StepName

				// 检查是否已在执行中
				if _, loaded := h.inFlightTasks.LoadOrStore(videoID+":"+stepName, true); loaded {
					continue
				}

				// 尝试获取 worker slot（下载任务使用专用池）
				isDownloadStep := stepName == "下载视频"
				var pool chan struct{}
				if isDownloadStep {
					pool = h.downloadWorkerPool
				} else {
					pool = h.workerPool
				}

				select {
				case pool <- struct{}{}:
					// 获取到 slot，启动 goroutine 执行
					go func(vID, sName string, workerPool chan struct{}) {
						defer func() {
							<-workerPool
							h.inFlightTasks.Delete(vID + ":" + sName)
						}()

						h.App.Logger.Infof("🔄 [并发] 开始重试步骤: %s - %s", vID, sName)
						if err := h.RunSingleTaskStep(vID, sName); err != nil {
							h.App.Logger.Errorf("重试步骤失败: %v", err)
						}
					}(videoID, stepName, pool)
				default:
					// worker pool 已满，跳过本次调度
					h.inFlightTasks.Delete(videoID + ":" + stepName)
				}
			}
		}

		// 2. 处理新的视频任务
		pendingTasks, err := h.getPendingTasks()
		if err != nil {
			smartLogger.Errorf("查询待处理任务失败: %v", err)
			return
		}

		if len(pendingTasks) == 0 {
			smartLogger.Debug("没有待处理的任务")
			return
		}

		// 遍历所有待处理任务，尝试并发执行
		for _, task := range pendingTasks {
			videoID := task.VideoId
			userID := task.UserID

			// 检查是否已在执行中（内存级快速检查）
			if _, loaded := h.inFlightTasks.LoadOrStore(videoID, true); loaded {
				continue
			}

			// ═══════════════════════════════════════════════════════════════
			// 关键修复：使用数据库原子操作防止竞态条件
			// 只有成功将状态从 "001" 更新为 "002" 的进程才能继续执行
			// ═══════════════════════════════════════════════════════════════
			result := h.Db.Model(&model.SavedVideo{}).
				Where("id = ? AND status != ?", task.Id, "002").
				Update("status", "002")

			if result.Error != nil {
				smartLogger.Errorf("原子更新任务状态失败: %v", result.Error)
				h.inFlightTasks.Delete(videoID)
				continue
			}

			if result.RowsAffected == 0 {
				// 状态不是 "001"，说明已被其他调度器处理，跳过
				h.inFlightTasks.Delete(videoID)
				continue
			}

			// 检查用户级并发限制
			if h.ConcurrencyLimiter != nil && userID > 0 {
				acquired, current, max, _ := h.ConcurrencyLimiter.TryAcquire(context.Background(), userID)
				if !acquired {
					smartLogger.Debugf("⏳ 用户 %d 并发已达上限 (%d/%d)，任务 %s 等待下次调度", userID, current, max, videoID)
					// 回滚状态
					h.updateSavedVideoStatus(task.Id, "001")
					h.inFlightTasks.Delete(videoID)
					continue
				}
			}

			// 尝试获取 worker slot
			select {
			case h.workerPool <- struct{}{}:
				// 获取到 slot，启动 goroutine 执行
				go func(t models2.TbVideo, uid uint) {
					defer func() {
						<-h.workerPool
						h.inFlightTasks.Delete(t.VideoId)
						// 释放用户级并发槽位
						if h.ConcurrencyLimiter != nil && uid > 0 {
							h.ConcurrencyLimiter.Release(uid)
						}
					}()

					// 创建用户日志助手
					userLogger := applogger.NewUserLogger(h.App.Logger.Desugar(), uid)
					userLogger.TaskLog(t.VideoId, "schedule", "started",
						map[string]interface{}{
							"title": t.Title,
							"mode":  "concurrent",
						})
					h.RunTaskChain(t)
					userLogger.TaskLog(t.VideoId, "schedule", "completed",
						map[string]interface{}{
							"title": t.Title,
							"mode":  "concurrent",
						})
				}(*task, userID)
			default:
				// worker pool 已满，回滚状态并跳过本次调度
				h.updateSavedVideoStatus(task.Id, "001")
				h.inFlightTasks.Delete(videoID)
				// ⚠️ 重要：释放已获取的并发槽位，否则会槽位泄漏
				if h.ConcurrencyLimiter != nil && userID > 0 {
					h.ConcurrencyLimiter.Release(userID)
				}
				smartLogger.Debugf("Worker pool 已满，任务 %s 回滚状态并等待下次调度", videoID)
			}
		}
	})

	// 启动 cron 调度器
	h.Task.Start()
	h.App.Logger.Info("✓ Cron scheduler started, checking for tasks every 5 seconds")
}

// resetRunningTasksOnStartup 应用启动时重置所有"运行中"的任务步骤
func (h *ChainTaskHandler) resetRunningTasksOnStartup() {
	h.App.Logger.Info("🔄 正在重置应用重启前的运行中任务...")

	// 重置所有"运行中"状态的任务步骤为"待执行"
	err := h.TaskStepService.ResetAllRunningTasks()
	if err != nil {
		h.App.Logger.Errorf("❌ 重置运行中任务失败: %v", err)
		return
	}

	h.App.Logger.Info("✅ 已重置所有运行中的任务步骤，它们将在下次调度时重新执行")
}

// getPendingTasks 获取状态为 '001' 的待处理任务（从 SavedVideo 表查询）
func (h *ChainTaskHandler) getPendingTasks() ([]*models2.TbVideo, error) {
	// 使用 SavedVideoService 查询状态为 '001' 的任务
	savedVideos, err := h.SavedVideoService.GetPendingVideos(10)
	if err != nil {
		return nil, err
	}

	// 将 SavedVideo 转换为 TbVideo 格式
	var tasks []*models2.TbVideo
	for _, sv := range savedVideos {
		task := &models2.TbVideo{
			Id:        sv.ID,
			URL:       sv.URL,
			UserID:    sv.UserID,
			Title:     sv.Title,
			VideoId:   sv.VideoID,
			Status:    sv.Status,
			CreatedAt: sv.CreatedAt,
			UpdatedAt: sv.UpdatedAt,
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// getRetrySteps 获取状态为 'pending' 的重试步骤
func (h *ChainTaskHandler) getRetrySteps() ([]*model.TaskStep, error) {
	return h.TaskStepService.GetPendingSteps()
}
func (h *ChainTaskHandler) RunTaskChain(video models2.TbVideo) {

	currentDir, err := filepath.Abs(h.App.Config.FileUpDir)
	if err != nil {
		h.App.Logger.Errorf("获取文件上传目录失败: %v", err)
		// 任务失败，更新状态为失败
		if updateErr := h.SavedVideoService.UpdateStatus(video.Id, "999"); updateErr != nil {
			h.App.Logger.Errorf("更新任务状态为失败时出错: %v", updateErr)
		}
		return
	}

	// 注册任务到取消管理器（使用数据库主键 ID + 任务类型标识）
	// 全链任务使用固定标识 "chain_full"
	runID := "chain_full"
	// 关键修复：每次启动新任务链前，清除可能残留的"已取消"状态
	// 防止之前删除的任务重新提交时被误判为已取消
	h.CancelManager.ClearCancel(video.Id)

	ctx, allowed := h.CancelManager.Register(video.Id, runID)
	if !allowed {
		h.App.Logger.Warnf("⛔ 任务链启动被拒绝（任务已被取消）: VideoID=%s", video.VideoId)
		// 更新状态为取消（998）
		if err := h.updateSavedVideoStatus(video.Id, "998"); err != nil {
			h.App.Logger.Errorf("更新任务状态为取消时出错: %v", err)
		}
		return
	}
	defer h.CancelManager.Deregister(video.Id, runID)

	// 创建用户日志助手
	userLogger := applogger.NewUserLogger(h.App.Logger.Desugar(), video.UserID)
	userLogger.TaskLog(video.VideoId, "task_chain", "started",
		map[string]interface{}{
			"title": video.Title,
			"url":   video.URL,
		})

	// 初始化任务步骤
	if err := h.TaskStepService.InitTaskSteps(video.VideoId); err != nil {
		userLogger.Errorm("初始化任务步骤失败",
			map[string]interface{}{
				"error": err.Error(),
			})
	}

	stateManager := manager.NewStateManager(video.Id, video.UserID, video.VideoId, currentDir, video.CreatedAt)

	// 创建任务链并设置日志、视频ID和取消上下文
	chain := manager.NewTaskChain().
		SetLogger(h.App.Logger).
		SetVideoID(video.VideoId).
		SetContext(ctx) // 设置取消上下文

	// ═══════════════════════════════════════════════════════════════
	// 任务流程（按依赖关系组织，支持并行执行）:
	// ┌────┬─────────────┬─────────────────────┬──────┬────────────┐
	// │序号│   任务名    │        依赖         │ 必需 │  失败处理  │
	// ├────┼─────────────┼─────────────────────┼──────┼────────────┤
	// │ 1  │ 获取元数据  │        无           │  是  │  终止链    │
	// ├────┼─────────────┼─────────────────────┼──────┼────────────┤
	// │ 2a │ 下载封面    │    获取元数据       │  是  │  终止链    │
	// │ 2b │ 下载视频    │    获取元数据       │  否  │  继续执行  │
	// │ 2c │ AI增强元数据│    获取元数据       │  否  │    跳过    │
	// ├────┼─────────────┼─────────────────────┼──────┼────────────┤
	// │ 3  │ 下载字幕    │      下载视频       │  否  │  继续执行  │
	// ├────┼─────────────┼─────────────────────┼──────┼────────────┤
	// │ 4  │ 翻译字幕    │      下载字幕       │  否  │    跳过    │
	// ├────┼─────────────┼─────────────────────┼──────┼────────────┤
	// │ 5  │ 确认元数据  │ AI增强+翻译字幕     │  是  │  终止链    │
	// └────┴─────────────┴─────────────────────┴──────┴────────────┘
	//
	// 并行说明：
	// - Group 2a, 2b, 2c 可以并行执行（都只依赖 Group 1）
	// - AI增强元数据可以和下载并行，节省总时间
	// - 确认元数据等待 AI 和翻译都完成后才执行
	//
	// 注意: 上传任务由 UploadScheduler 定时执行
	// ═══════════════════════════════════════════════════════════════

	// ═══════════════════════════════════════════════════════════════
	// 任务执行顺序（必须按依赖关系排列）：
	// 1. 获取元数据 (无依赖)
	// 2. 下载视频   (无依赖)
	// 3. 下载字幕   (无依赖)
	// 4. 下载封面   (依赖: 获取元数据)
	// 5. 翻译字幕   (依赖: 下载字幕)
	// 6. AI增强元数据 (依赖: 下载视频)
	//    说明: 优先使用中文字幕(zh.srt)，如不存在则使用英文字幕(en.srt)
	//    不强制依赖翻译字幕，即使用户禁用翻译也能正常工作
	// 7. 确认元数据   (依赖: AI增强元数据)
	// ═══════════════════════════════════════════════════════════════

	// Group 1: 基础任务（无依赖或仅依赖获取元数据）
	fetchMetadataTask := handlers.NewFetchMetadata("获取元数据", h.App, stateManager, h.App.CosClient, h.SavedVideoService)
	chain.AddTask(h.wrapTaskWithStepTracking(fetchMetadataTask, video.VideoId))

	downloadTask := handlers.NewDownloadVideo("下载视频", h.App, stateManager, h.App.CosClient, h.SavedVideoService)
	chain.AddTask(h.wrapTaskWithStepTracking(downloadTask, video.VideoId))

	subtitleTask := handlers.NewGenerateSubtitles("下载字幕", h.App, stateManager, h.App.CosClient, h.SavedVideoService)
	chain.AddTask(h.wrapTaskWithStepTracking(subtitleTask, video.VideoId))

	coverTask := handlers.NewDownloadImgHandler("下载封面", h.App, stateManager, h.App.CosClient)
	chain.AddTask(h.wrapTaskWithStepTracking(coverTask, video.VideoId))

	// Group 2: 翻译字幕（依赖下载字幕）
	translateTask := handlers.NewTranslateSubtitle("翻译字幕", h.App, stateManager, h.App.CosClient, h.Db, "")
	chain.AddTask(h.wrapTaskWithStepTracking(translateTask, video.VideoId))

	// Group 3: AI增强元数据（仅依赖下载视频）
	// 说明: 可以与翻译字幕并行执行，优先使用中文字幕，无中文则用英文字幕
	metadataTask := handlers.NewGenerateMetadata("AI增强元数据", h.App, stateManager, h.App.CosClient, "", h.Db, h.SavedVideoService)
	chain.AddTask(h.wrapTaskWithStepTracking(metadataTask, video.VideoId))

	// Group 4: 确认最终元数据（依赖 AI增强元数据）
	confirmMetadataTask := handlers.NewConfirmMetadata("确认元数据", h.App, stateManager, h.App.CosClient, h.SavedVideoService)
	chain.AddTask(h.wrapTaskWithStepTracking(confirmMetadataTask, video.VideoId))

	// 执行任务链
	startTime := time.Now()
	result := chain.Run(true)
	duration := time.Since(startTime)

	// 检查是否被取消
	if canceled, exists := result["canceled"]; exists && canceled.(bool) {
		userLogger.TaskLog(video.VideoId, "task_chain", "canceled",
			map[string]interface{}{
				"duration_seconds": duration.Seconds(),
				"reason":           "用户删除视频",
			})
		// 更新状态为取消（998）
		if err := h.updateSavedVideoStatus(video.Id, "998"); err != nil {
			h.App.Logger.Errorf("更新任务状态为取消时出错: %v", err)
		}
		return
	}

	// 检查任务链是否成功执行
	// 只有必需任务失败才认为整体失败
	success := len(chain.FailedTasks) == 0 || !h.hasRequiredTaskFailed(chain)

	// 根据执行结果更新任务状态
	if success {
		// 更新状态为 200 并设置处理完成时间（用于延迟上传调度）
		now := time.Now()
		if err := h.Db.Table("cw_saved_videos").Where("id = ?", video.Id).Updates(map[string]interface{}{
			"status":                  "200",
			"processing_completed_at": now,
		}).Error; err != nil {
			h.App.Logger.Errorf("更新任务状态为完成时出错: %v", err)
		}
		userLogger.TaskLog(video.VideoId, "task_chain", "completed",
			map[string]interface{}{
				"duration_seconds": duration.Seconds(),
				"failed_tasks":     len(chain.FailedTasks),
			})
		h.App.Logger.Infof("✅ 任务链完成，视频准备就绪 (处理完成时间: %s)", now.Format("15:04:05"))
	} else {
		if err := h.updateSavedVideoStatus(video.Id, "999"); err != nil {
			h.App.Logger.Errorf("更新任务状态为失败时出错: %v", err)
		}
		userLogger.TaskLog(video.VideoId, "task_chain", "failed",
			map[string]interface{}{
				"duration_seconds": duration.Seconds(),
				"failed_tasks":     len(chain.FailedTasks),
			})
	}

	// 记录最终结果
	_ = result
	_ = duration
}

// hasRequiredTaskFailed 检查是否有必需任务失败
func (h *ChainTaskHandler) hasRequiredTaskFailed(chain *manager.TaskChain) bool {
	for taskName := range chain.FailedTasks {
		if config, exists := manager.TaskConfigs[taskName]; exists && config.Required {
			return true
		}
	}
	return false
}

// RunSingleTaskStep 执行单个任务步骤
func (h *ChainTaskHandler) RunSingleTaskStep(videoID, stepName string) error {
	// 注意：此方法假设调用方已经获得了锁，因此不在这里加锁

	// 获取视频信息
	savedVideo, err := h.SavedVideoService.GetVideoByVideoID(videoID)
	if err != nil {
		return fmt.Errorf("获取视频信息失败: %v", err)
	}

	// 转换为TbVideo格式
	video := models2.TbVideo{
		Id:        savedVideo.ID,
		URL:       savedVideo.URL,
		UserID:    savedVideo.UserID,
		Title:     savedVideo.Title,
		VideoId:   savedVideo.VideoID,
		Status:    savedVideo.Status,
		CreatedAt: savedVideo.CreatedAt,
		UpdatedAt: savedVideo.UpdatedAt,
	}

	// 注册任务到取消管理器（使用数据库主键 ID + 步骤名称）
	// 单步任务使用 "step_" + stepName 作为标识
	runID := "step_" + stepName
	// 同样在单步执行前清除取消状态，确保重试操作能正常执行
	h.CancelManager.ClearCancel(video.Id)

	ctx, allowed := h.CancelManager.Register(video.Id, runID)
	if !allowed {
		h.App.Logger.Warnf("⛔ 任务步骤启动被拒绝（任务已被取消）: %s - %s", video.VideoId, stepName)
		return nil // 已取消，直接返回
	}
	defer h.CancelManager.Deregister(video.Id, runID)

	// 获取当前目录
	currentDir, err := filepath.Abs(h.App.Config.FileUpDir)
	if err != nil {
		return fmt.Errorf("获取文件上传目录失败: %v", err)
	}

	// 创建状态管理器
	stateManager := manager.NewStateManager(video.Id, video.UserID, video.VideoId, currentDir, video.CreatedAt)

	// 重置步骤状态
	if err := h.TaskStepService.ResetTaskStep(videoID, stepName); err != nil {
		h.App.Logger.Errorf("重置任务步骤失败: %v", err)
	}

	// 更新步骤状态为运行中
	if err := h.TaskStepService.UpdateTaskStepStatus(videoID, stepName, "running"); err != nil {
		h.App.Logger.Errorf("更新任务步骤状态失败: %v", err)
	}

	// 获取已完成的任务步骤（用于依赖检查）
	completedSteps, err := h.TaskStepService.GetCompletedStepNames(videoID)
	if err != nil {
		h.App.Logger.Warnf("获取已完成步骤失败: %v", err)
		completedSteps = []string{}
	}

	// 获取正在运行的任务步骤，将它们也视为已完成（避免并发竞态条件）
	// 原因：当单步重试时，可能有其他步骤正在并发执行，如果不将它们计入已完成列表，
	// 会导致依赖检查失败（如：下载封面依赖获取元数据，但获取元数据正在运行中）
	runningSteps, err := h.TaskStepService.GetRunningStepNames(videoID)
	if err == nil && len(runningSteps) > 0 {
		h.App.Logger.Debugf("🔄 检测到 %d 个正在运行的步骤，将它们计入依赖检查", len(runningSteps))
		for _, runningStep := range runningSteps {
			// 如果某个步骤正在运行，且它不是当前要执行的步骤，则视为已完成
			if runningStep != stepName {
				alreadyExists := false
				for _, s := range completedSteps {
					if s == runningStep {
						alreadyExists = true
						break
					}
				}
				if !alreadyExists {
					completedSteps = append(completedSteps, runningStep)
					h.App.Logger.Debugf("🔄 步骤 '%s' 正在运行，依赖检查视为已完成", runningStep)
				}
			}
		}
	}

	// 智能依赖检查：如果视频文件存在，自动将"下载视频"标记为完成（仅用于依赖判断，不更新数据库）
	videoFilePath := filepath.Join(stateManager.CurrentDir, videoID+".mp4")
	if _, err := os.Stat(videoFilePath); err == nil {
		hasDownloadVideo := false
		for _, s := range completedSteps {
			if s == "下载视频" {
				hasDownloadVideo = true
				break
			}
		}
		if !hasDownloadVideo {
			completedSteps = append(completedSteps, "下载视频")
			h.App.Logger.Debugf("📁 检测到视频文件存在，依赖检查视为'下载视频'已完成")
			// 注意：不更新数据库状态，仅用于本次依赖检查
		}
	}

	// 智能依赖检查：如果字幕文件存在，自动将"下载字幕"标记为完成（仅用于依赖判断）
	subtitlePatterns := []string{
		filepath.Join(stateManager.CurrentDir, "zh.srt"),
		filepath.Join(stateManager.CurrentDir, "en.srt"),
		filepath.Join(stateManager.CurrentDir, videoID+".*.srt"),
	}
	hasSubtitleFile := false
	for _, pattern := range subtitlePatterns {
		matches, _ := filepath.Glob(pattern)
		if len(matches) > 0 {
			hasSubtitleFile = true
			break
		}
	}
	if hasSubtitleFile {
		hasDownloadSubtitle := false
		for _, s := range completedSteps {
			if s == "下载字幕" || s == "生成字幕" {
				hasDownloadSubtitle = true
				break
			}
		}
		if !hasDownloadSubtitle {
			completedSteps = append(completedSteps, "下载字幕")
			h.App.Logger.Debugf("📁 检测到字幕文件存在，依赖检查视为'下载字幕'已完成")
		}
	}

	// 智能依赖检查：如果封面文件存在，自动将"下载封面"标记为完成（仅用于依赖判断）
	coverPatterns := []string{
		filepath.Join(stateManager.CurrentDir, "cover.jpg"),
		filepath.Join(stateManager.CurrentDir, "cover.webp"),
		filepath.Join(stateManager.CurrentDir, "cover.png"),
	}
	hasCoverFile := false
	for _, coverPath := range coverPatterns {
		if _, err := os.Stat(coverPath); err == nil {
			hasCoverFile = true
			break
		}
	}
	if hasCoverFile {
		hasDownloadCover := false
		for _, s := range completedSteps {
			if s == "下载封面" {
				hasDownloadCover = true
				break
			}
		}
		if !hasDownloadCover {
			completedSteps = append(completedSteps, "下载封面")
			h.App.Logger.Debugf("📁 检测到封面文件存在，依赖检查视为'下载封面'已完成")
		}
	}

	// 创建单个任务的链，并预填充已完成的任务
	chain := manager.NewTaskChain().
		SetLogger(h.App.Logger).
		SetVideoID(videoID).
		SetContext(ctx). // 设置取消上下文
		SetCompletedTasks(completedSteps)
	var task types.Task

	// 根据步骤名称创建对应的任务
	switch stepName {
	case "获取元数据":
		task = handlers.NewFetchMetadata("获取元数据", h.App, stateManager, h.App.CosClient, h.SavedVideoService)
	case "下载视频":
		task = handlers.NewDownloadVideo("下载视频", h.App, stateManager, h.App.CosClient, h.SavedVideoService)
	case "下载字幕", "生成字幕": // 支持新旧两种名称
		task = handlers.NewGenerateSubtitles("下载字幕", h.App, stateManager, h.App.CosClient, h.SavedVideoService)
	case "下载封面":
		task = handlers.NewDownloadImgHandler("下载封面", h.App, stateManager, h.App.CosClient)
	case "翻译字幕":
		task = handlers.NewTranslateSubtitle("翻译字幕", h.App, stateManager, h.App.CosClient, h.Db, "")
	case "生成元数据", "AI增强元数据": // 支持新旧两种名称
		task = handlers.NewGenerateMetadata("AI增强元数据", h.App, stateManager, h.App.CosClient, "", h.Db, h.SavedVideoService)
	case "确认元数据":
		task = handlers.NewConfirmMetadata("确认元数据", h.App, stateManager, h.App.CosClient, h.SavedVideoService)
	case "上传到Bilibili":
		task = handlers.NewUploadToBilibili("上传到Bilibili", h.App, stateManager, h.App.CosClient, h.SavedVideoService, h.BiliAccountService)
	case "上传字幕到Bilibili":
		task = handlers.NewUploadSubtitleToBilibili("上传字幕到Bilibili", h.App, stateManager, h.App.CosClient, h.SavedVideoService, h.BiliAccountService)
	default:
		return fmt.Errorf("未知的任务步骤: %s", stepName)
	}

	// 添加任务到链
	if task != nil {
		chain.AddTask(task)
	}

	h.App.Logger.Infof("开始执行单个任务步骤: %s (VideoID: %s)", stepName, videoID)

	// 创建带有封面路径的 context
	initialContext := make(map[string]interface{})

	// 如果是上传任务，查找封面文件并设置到 context
	if stepName == "上传到Bilibili" {
		h.App.Logger.Infof("📂 封面查找目录: %s", stateManager.CurrentDir)
		coverPath := h.findCoverImage(stateManager.CurrentDir)
		if coverPath != "" {
			initialContext["cover_image_path"] = coverPath
			h.App.Logger.Infof("📸 找到封面文件: %s", coverPath)
		} else {
			h.App.Logger.Warnf("⚠️ 未找到封面文件，目录: %s", stateManager.CurrentDir)
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
		if err := h.TaskStepService.UpdateTaskStepStatus(videoID, stepName, "completed"); err != nil {
			h.App.Logger.Errorf("更新任务步骤状态失败: %v", err)
		}
		if err := h.TaskStepService.UpdateTaskStepResult(videoID, stepName, result); err != nil {
			h.App.Logger.Errorf("更新任务步骤结果失败: %v", err)
		}
		h.App.Logger.Infof("任务步骤 %s 执行成功", stepName)
	} else {
		if err := h.TaskStepService.UpdateTaskStepStatus(videoID, stepName, "failed", errorMsg); err != nil {
			h.App.Logger.Errorf("更新任务步骤状态失败: %v", err)
		}
		h.App.Logger.Errorf("任务步骤 %s 执行失败: %s", stepName, errorMsg)
		return fmt.Errorf("任务执行失败: %s", errorMsg)
	}

	return nil
}

// wrapTaskWithStepTracking 包装任务以添加步骤跟踪
func (h *ChainTaskHandler) wrapTaskWithStepTracking(task types.Task, videoID string) types.Task {
	return &TaskStepWrapper{
		task:            task,
		videoID:         videoID,
		taskStepService: h.TaskStepService,
		logger:          h.App.Logger,
	}
}

// TaskStepWrapper 任务步骤包装器
type TaskStepWrapper struct {
	task            types.Task
	videoID         string
	taskStepService *services.TaskStepService
	logger          *zap.SugaredLogger
}

func (w *TaskStepWrapper) GetName() string {
	return w.task.GetName()
}

func (w *TaskStepWrapper) InsertTask() error {
	return w.task.InsertTask()
}

func (w *TaskStepWrapper) UpdateStatus(status, message string) error {
	// 直接更新数据库中的步骤状态
	stepName := w.task.GetName()
	return w.taskStepService.UpdateTaskStepStatus(w.videoID, stepName, status, message)
}

func (w *TaskStepWrapper) Execute(context map[string]interface{}) bool {
	stepName := w.task.GetName()

	// 更新步骤状态为运行中
	if err := w.taskStepService.UpdateTaskStepStatus(w.videoID, stepName, "running"); err != nil {
		w.logger.Errorf("更新任务步骤状态失败: %v", err)
	}

	// 执行原始任务
	success := w.task.Execute(context)

	// 更新步骤状态
	if success {
		// 检查是否是跳过状态（权限不足等原因）
		if skipReason, skipped := context["skipped"]; skipped {
			skipMsg := fmt.Sprintf("%v", skipReason)
			if err := w.taskStepService.UpdateTaskStepStatus(w.videoID, stepName, "skipped", skipMsg); err != nil {
				w.logger.Errorf("更新任务步骤状态失败: %v", err)
			}
			// 清除 skipped 标记，避免影响后续任务
			delete(context, "skipped")
		} else {
			if err := w.taskStepService.UpdateTaskStepStatus(w.videoID, stepName, "completed"); err != nil {
				w.logger.Errorf("更新任务步骤状态失败: %v", err)
			}

			// 保存执行结果
			result := map[string]interface{}{}
			for k, v := range context {
				if k != "error" { // 排除错误信息
					result[k] = v
				}
			}
			if err := w.taskStepService.UpdateTaskStepResult(w.videoID, stepName, result); err != nil {
				w.logger.Errorf("更新任务步骤结果失败: %v", err)
			}
		}
	} else {
		errorMsg := ""
		if err, exists := context["error"]; exists {
			errorMsg = fmt.Sprintf("%v", err)
		}

		if err := w.taskStepService.UpdateTaskStepStatus(w.videoID, stepName, "failed", errorMsg); err != nil {
			w.logger.Errorf("更新任务步骤状态失败: %v", err)
		}
	}

	return success
}

// updateSavedVideoStatus 更新 SavedVideo 的状态
func (h *ChainTaskHandler) updateSavedVideoStatus(id uint, status string) error {
	return h.SavedVideoService.UpdateStatus(id, status)
}

// findCoverImage 在指定目录中查找封面图片
// 优先查找 cover.webp，然后是 YouTube 缩略图，最后查找任意图片
func (h *ChainTaskHandler) findCoverImage(dir string) string {
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
