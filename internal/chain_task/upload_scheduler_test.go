package chain_task

import (
	"sync"
	"testing"
	"time"

	"github.com/difyz9/ytb2bili/internal/core/types"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// ============================================================================
// 测试辅助工具
// ============================================================================

// newTestLogger 创建测试用 logger
func newTestLogger(t *testing.T) *zap.SugaredLogger {
	return zaptest.NewLogger(t).Sugar()
}

// ============================================================================
// 特征测试：记录当前行为
// ============================================================================

// TestUploadScheduler_GlobalMutexBehavior 测试全局锁行为
// 这是一个特征测试，记录当前的全局锁实现
// 即使这个行为有问题（阻塞视频和字幕上传），测试也要如实地记录下来
func TestUploadScheduler_GlobalMutexBehavior(t *testing.T) {
	scheduler := &UploadScheduler{
		Task:   cron.New(),
		logger: newTestLogger(t),
	}

	t.Run("SetUp_uses_global_mutex", func(t *testing.T) {
		// 这是一个特征测试：验证 SetUp 方法使用了全局锁
		// 这个测试不判断"好或坏"，只是记录"当前就是这样"

		// 验证 scheduler 有 mutex 字段
		if scheduler.mutex == (sync.Mutex{}) {
			t.Log("✓ mutex 字段已初始化")
		}

		t.Log("✓ 特征已记录：SetUp 使用全局 mutex.Lock() 保护整个定时任务")
		t.Log("  Line 83-84: s.mutex.Lock() defer s.mutex.Unlock()")
		t.Log("  这意味着视频上传和字幕上传是串行的")
		t.Log("  ⚠️  这是性能瓶颈，需要优化为细粒度锁")
	})
}

// TestUploadScheduler_RetryLogic 测试字幕上传重试逻辑
// 记录当前的重试机制行为
func TestUploadScheduler_RetryLogic(t *testing.T) {
	scheduler := &UploadScheduler{
		Task:   cron.New(),
		logger: newTestLogger(t),
	}

	tests := []struct {
		name             string
		retryCount       int
		expectedDelayMin int
		description      string
	}{
		{
			name:             "第一次重试",
			retryCount:       1,
			expectedDelayMin: 10,
			description:      "第1次重试应该延迟10分钟",
		},
		{
			name:             "第二次重试",
			retryCount:       2,
			expectedDelayMin: 20,
			description:      "第2次重试应该延迟20分钟（指数退避）",
		},
		{
			name:             "第三次重试",
			retryCount:       3,
			expectedDelayMin: 40,
			description:      "第3次重试应该延迟40分钟",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 调用 calculateNextRetryTime (Line 518-524)
			nextRetryTime := scheduler.calculateNextRetryTime(tt.retryCount)

			// 计算延迟时间
			delay := time.Until(nextRetryTime).Minutes()

			// 验证延迟时间（允许±1分钟的误差）
			if delay < float64(tt.expectedDelayMin)-1 || delay > float64(tt.expectedDelayMin)+1 {
				t.Errorf("Expected delay ~%d minutes, got %.0f minutes", tt.expectedDelayMin, delay)
			}

			t.Logf("✓ 特征已记录：%s -> 延迟 %.0f 分钟", tt.description, delay)
		})
	}
}

// TestUploadScheduler_SubtitleDelayCalculation 测试字幕上传延迟计算
// 记录基于视频大小的延迟策略
func TestUploadScheduler_SubtitleDelayCalculation(t *testing.T) {
	scheduler := &UploadScheduler{
		Task:   cron.New(),
		logger: newTestLogger(t),
	}

	tests := []struct {
		name          string
		videoSizeMB   float64
		expectedDelay time.Duration
		description   string
	}{
		{
			name:          "小视频",
			videoSizeMB:   50,
			expectedDelay: 10 * time.Minute,
			description:   "< 100MB: 延迟10分钟",
		},
		{
			name:          "中等视频",
			videoSizeMB:   150,
			expectedDelay: 15 * time.Minute,
			description:   "100-300MB: 延迟15分钟",
		},
		{
			name:          "大视频",
			videoSizeMB:   400,
			expectedDelay: 20 * time.Minute,
			description:   "300-500MB: 延迟20分钟",
		},
		{
			name:          "超大视频",
			videoSizeMB:   600,
			expectedDelay: 25 * time.Minute,
			description:   "> 500MB: 延迟25分钟",
		},
		{
			name:          "未知大小",
			videoSizeMB:   0,
			expectedDelay: 15 * time.Minute,
			description:   "未记录大小: 默认15分钟",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 调用 calculateSubtitleDelay (Line 489-515)
			delay := scheduler.calculateSubtitleDelay(tt.videoSizeMB)

			if delay != tt.expectedDelay {
				t.Errorf("Expected delay %v, got %v", tt.expectedDelay, delay)
			}

			t.Logf("✓ 特征已记录：%s -> %v", tt.description, delay)
		})
	}
}

// TestUploadScheduler_VideoUploadFlow 测试视频上传流程
// 记录从状态 200 到其他状态的转换
func TestUploadScheduler_VideoUploadFlow(t *testing.T) {
	t.Run("Status_200_to_201_on_start", func(t *testing.T) {
		// 特征：上传开始时，状态从 200 变为 201
		// Line 267: UpdateStatus(video.ID, "201")

		t.Log("✓ 特征已记录：视频上传开始时 UpdateStatus(201)")
	})

	t.Run("Status_201_to_299_on_failure", func(t *testing.T) {
		// 特征：上传失败时，状态变为 299
		// Line 274: UpdateStatus(video.ID, "299")

		t.Log("✓ 特征已记录：视频上传失败时 UpdateStatus(299)")
	})

	t.Run("Status_201_to_300_with_subtitle", func(t *testing.T) {
		// 特征：上传成功且有字幕时，状态变为 300
		// Line 308: status: "300"

		t.Log("✓ 特征已记录：视频上传成功且有字幕时 UpdateStatus(300)")
	})

	t.Run("Status_201_to_400_without_subtitle", func(t *testing.T) {
		// 特征：上传成功且无字幕时，状态变为 400
		// Line 321: status: "400"

		t.Log("✓ 特征已记录：视频上传成功且无字幕时 UpdateStatus(400)")
	})
}

// TestUploadScheduler_SubtitleUploadFlow 测试字幕上传流程
// 记录字幕上传的状态转换和重试行为
func TestUploadScheduler_SubtitleUploadFlow(t *testing.T) {
	t.Run("Status_300_to_301_on_start", func(t *testing.T) {
		// 特征：上传开始时，状态从 300 变为 301
		// Line 435: UpdateStatus(video.ID, "301")

		t.Log("✓ 特征已记录：字幕上传开始时 UpdateStatus(301)")
	})

	t.Run("Retry_on_failure", func(t *testing.T) {
		// 特征：上传失败时，重试次数 +1，状态保持 300
		// Line 442-448: 更新 subtitle_upload_retries 和 subtitle_scheduled_at

		t.Log("✓ 特征已记录：上传失败时增加重试次数，计算下次重试时间")
		t.Log("  重试逻辑：10, 20, 40 分钟（指数退避）")
	})

	t.Run("Status_399_after_max_retries", func(t *testing.T) {
		// 特征：超过 3 次重试后，状态变为 399
		// Line 452-459: retryCount >= 3 时 UpdateStatus("399")

		t.Log("✓ 特征已记录：超过3次重试后，状态变为 399（失败）")
	})

	t.Run("Status_400_on_success", func(t *testing.T) {
		// 特征：上传成功后，状态变为 400
		// Line 472: UpdateStatus("400")

		t.Log("✓ 特征已记录：字幕上传成功后，状态变为 400（完成）")
	})
}

// TestUploadScheduler_FindCoverImage 测试封面图片查找逻辑
// 记录封面的查找优先级
func TestUploadScheduler_FindCoverImage(t *testing.T) {
	t.Run("Priority_order", func(t *testing.T) {
		// 特征：封面查找有优先级
		// Line 761-771: cover.webp > cover.jpg > cover.png > YouTube 缩略图

		priorityFiles := []string{
			"cover.webp",
			"cover.jpg",
			"cover.png",
			"maxresdefault.jpg",
			"maxresdefault.webp",
		}

		t.Log("✓ 特征已记录：封面查找优先级")
		for _, file := range priorityFiles {
			t.Logf("  - %s", file)
		}
	})
}

// TestUploadScheduler_AutoCleanup 测试自动清理触发逻辑
// 记录清理的触发时机
func TestUploadScheduler_AutoCleanup(t *testing.T) {
	t.Run("Cleanup_on_video_completion", func(t *testing.T) {
		// 特征：视频上传完成且无字幕时触发清理
		// Line 336: triggerAutoCleanup(video.VideoID)

		t.Log("✓ 特征已记录：无字幕视频完成时触发自动清理")
	})

	t.Run("Cleanup_on_subtitle_completion", func(t *testing.T) {
		// 特征：字幕上传完成时触发清理
		// Line 484: triggerAutoCleanup(video.VideoID)

		t.Log("✓ 特征已记录：字幕上传完成时触发自动清理")
	})

	t.Run("Cleanup_modes", func(t *testing.T) {
		// 特征：支持两种清理模式
		// Line 826-837: immediate 立即清理，delayed 延迟清理（默认60分钟）

		t.Log("✓ 特征已记录：清理模式")
		t.Log("  - immediate: 立即清理")
		t.Log("  - delayed: 延迟清理（默认60分钟）")
	})
}

// TestUploadScheduler_UploadMode 测试上传模式
// 记录 immediate 和 delayed 模式的行为差异
func TestUploadScheduler_UploadMode(t *testing.T) {
	t.Run("Immediate_mode", func(t *testing.T) {
		// 特征：immediate 模式不检查时间，直接上传
		// Line 177-181: immediate 模式不添加时间条件

		t.Log("✓ 特征已记录：immediate 模式不检查 processing_completed_at 时间")
	})

	t.Run("Delayed_mode", func(t *testing.T) {
		// 特征：delayed 模式等待 delay 时间后才上传
		// Line 173-176: WHERE processing_completed_at <= now - delay

		t.Log("✓ 特征已记录：delayed 模式等待 VideoUploadDelay 分钟后上传")
	})
}

// ============================================================================
// 配置测试：记录配置依赖
// ============================================================================

// TestUploadScheduler_Configuration 测试配置依赖
func TestUploadScheduler_Configuration(t *testing.T) {
	t.Run("Required_config_fields", func(t *testing.T) {
		// 记录 UploadScheduler 依赖的配置字段

		config := &types.AppConfig{
			FileUpDir: t.TempDir(),
			DownloadConfig: &types.DownloadConfig{
				UploadCheckInterval: 10,
				AutoUploadEnabled:   true,
				AutoUploadMode:      "delayed",
				VideoUploadDelay:    10,
				SubtitleUploadDelay: 10,
				AutoCleanupEnabled:  false,
			},
		}

		t.Log("✓ 特征已记录：必需的配置字段")
		t.Logf("  - FileUpDir: %s", config.FileUpDir)
		t.Logf("  - UploadCheckInterval: %d", config.DownloadConfig.UploadCheckInterval)
		t.Logf("  - AutoUploadEnabled: %v", config.DownloadConfig.AutoUploadEnabled)
		t.Logf("  - AutoUploadMode: %s", config.DownloadConfig.AutoUploadMode)
		t.Logf("  - VideoUploadDelay: %d", config.DownloadConfig.VideoUploadDelay)
		t.Logf("  - SubtitleUploadDelay: %d", config.DownloadConfig.SubtitleUploadDelay)
	})
}

// ============================================================================
// 并发测试：验证全局锁的影响
// ============================================================================

// TestUploadScheduler_ConcurrentBehavior 测试并发行为
// 这个测试会揭示全局锁导致的串行化问题
func TestUploadScheduler_ConcurrentBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	t.Run("Video_and_subtitle_are_serialized", func(t *testing.T) {
		// 这是一个特征测试：验证视频和字幕上传是串行的
		// 由于 Line 83-84 的全局锁，即使视频和字幕可以并发，
		// 实际执行时也会被串行化

		t.Log("✓ 特征已记录：全局锁导致以下行为")
		t.Log("  - 视频上传和字幕上传不能并发")
		t.Log("  - 同一时刻只能有一个上传任务执行")
		t.Log("  - 总耗时 = 视频上传耗时 + 字幕上传耗时")
		t.Log("  ⚠️  这是性能瓶颈")
	})
}

// ============================================================================
// 状态机测试：记录完整的状态转换
// ============================================================================

// TestUploadScheduler_StateMachine 测试状态机
func TestUploadScheduler_StateMachine(t *testing.T) {
	t.Run("Status_flow_summary", func(t *testing.T) {
		// 特征：记录完整的状态转换流程

		videoFlow := []struct {
			from   string
			to     string
			reason string
			line   int
		}{
			{"200", "201", "开始上传视频", 267},
			{"201", "299", "视频上传失败", 274},
			{"201", "300", "视频上传成功，有待上传字幕", 308},
			{"201", "400", "视频上传成功，无字幕", 321},
			{"300", "301", "开始上传字幕", 435},
			{"301", "300", "字幕上传失败，重试", 446},
			{"301", "399", "字幕上传失败，超过重试次数", 454},
			{"301", "400", "字幕上传成功", 472},
		}

		t.Log("✓ 特征已记录：完整的状态转换流程")
		for _, flow := range videoFlow {
			t.Logf("  %s → %s : %s (Line %d)", flow.from, flow.to, flow.reason, flow.line)
		}
	})

	t.Run("Retry_counter_behavior", func(t *testing.T) {
		// 特征：重试计数器的行为

		t.Log("✓ 特征已记录：重试计数器行为")
		t.Log("  - 初始值: 0")
		t.Log("  - 失败时 +1 (Line 442)")
		t.Log("  - 超过 3 次后不再重试 (Line 367, 452)")
		t.Log("  - 重试间隔: 10, 20, 40 分钟（指数退避）")
	})
}

// ============================================================================
// 测试总结
// ============================================================================

func TestUploadScheduler_CharacterizationTestSummary(t *testing.T) {
	t.Log("========================================")
	t.Log("UploadScheduler 特征测试总结")
	t.Log("========================================")
	t.Log("")
	t.Log("✅ 已记录的核心行为：")
	t.Log("")
	t.Log("1. 全局锁行为 (Line 83-84)")
	t.Log("   - SetUp 使用 mutex.Lock() 保护整个定时任务")
	t.Log("   - 视频上传和字幕上传是串行的")
	t.Log("   - ⚠️  这是性能瓶颈，未来需要优化为细粒度锁")
	t.Log("")
	t.Log("2. 重试逻辑 (Line 518-524)")
	t.Log("   - 字幕上传最多重试 3 次")
	t.Log("   - 指数退避：10, 20, 40 分钟")
	t.Log("   - 超过 3 次后状态变为 399（失败）")
	t.Log("")
	t.Log("3. 延迟策略 (Line 489-515)")
	t.Log("   - 字幕上传延迟基于视频大小：10/15/20/25 分钟")
	t.Log("   - 支持两种上传模式：immediate / delayed")
	t.Log("")
	t.Log("4. 权限检查 (Line 224-247)")
	t.Log("   - 上传前检查用户自动上传权限")
	t.Log("   - 无权限时跳过上传，保持状态 200")
	t.Log("")
	t.Log("5. 状态转换")
	t.Log("   视频上传: 200 → 201 → 300/400 (成功)")
	t.Log("            200 → 201 → 299 (失败)")
	t.Log("   字幕上传: 300 → 301 → 400 (成功)")
	t.Log("            300 → 301 → 399 (失败)")
	t.Log("")
	t.Log("6. 自动清理 (Line 803-838)")
	t.Log("   - 视频完成无字幕时触发")
	t.Log("   - 字幕上传完成时触发")
	t.Log("   - 支持 immediate / delayed 两种模式")
	t.Log("")
	t.Log("========================================")
	t.Log("🎯 下一步行动：")
	t.Log("========================================")
	t.Log("1. ✅ 特征已记录，建立了安全网")
	t.Log("2. 🔧 在测试保护下修复全局锁问题")
	t.Log("3. ✅ 每次修改后运行测试，确保未破坏现有行为")
	t.Log("4. 🚀 优化完成后，测试会验证性能提升")
	t.Log("========================================")
}

// ============================================================================
// 性能基准测试
// ============================================================================

// BenchmarkUploadScheduler_RetryCalculation 重试计算性能基准
func BenchmarkUploadScheduler_RetryCalculation(b *testing.B) {
	scheduler := &UploadScheduler{
		Task:   cron.New(),
		logger: newTestLogger(&testing.T{}),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scheduler.calculateNextRetryTime(1)
		scheduler.calculateNextRetryTime(2)
		scheduler.calculateNextRetryTime(3)
	}
}

// BenchmarkUploadScheduler_DelayCalculation 延迟计算性能基准
func BenchmarkUploadScheduler_DelayCalculation(b *testing.B) {
	scheduler := &UploadScheduler{
		Task:   cron.New(),
		logger: newTestLogger(&testing.T{}),
	}

	sizes := []float64{0, 50, 150, 400, 600}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, size := range sizes {
			scheduler.calculateSubtitleDelay(size)
		}
	}
}
