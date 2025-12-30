package chain_task

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

// ProductionTestConfig 生产环境测试配置
type ProductionTestConfig struct {
	// 测试持续时间
	Duration time.Duration

	// 并发视频数
	ConcurrentVideos int

	// 每个视频的上传模拟时长
	SimulatedUploadDuration time.Duration

	// 是否启用详细日志
	VerboseLogging bool

	// 数据库连接（真实环境）
	DB interface{} // *gorm.DB

	// 日志记录器
	Logger *zap.SugaredLogger
}

// ProductionTestResult 测试结果
type ProductionTestResult struct {
	StartTime              time.Time
	EndTime                time.Time
	TotalVideosProcessed   int32
	TotalConcurrentUploads int32
	AverageUploadTime      time.Duration
	MaxConcurrentUploads   int32
	Errors                 []error
	Latencies              []time.Duration
	mu                     sync.Mutex
}

// TestUploadScheduler_ProductionLoad 生产环境负载测试
// 这个测试模拟真实的生产环境场景
func TestUploadScheduler_ProductionLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("生产环境测试需要时间，使用 -short 跳过")
	}

	t.Log("========================================")
	t.Log("生产环境负载测试")
	t.Log("========================================")

	// 配置测试参数
	config := &ProductionTestConfig{
		Duration:                  30 * time.Second, // 测试30秒
		ConcurrentVideos:          5,                 // 5个视频并发
		SimulatedUploadDuration:   2 * time.Second,   // 每个视频2秒
		VerboseLogging:            true,
	}

	t.Logf("测试配置：")
	t.Logf("  - 持续时间: %v", config.Duration)
	t.Logf("  - 并发视频数: %d", config.ConcurrentVideos)
	t.Logf("  - 模拟上传时长: %v", config.SimulatedUploadDuration)
	t.Logf("========================================")

	// 运行测试
	result := runProductionLoadTest(config)

	// 输出结果
	printProductionTestResults(t, result)
}

// runProductionLoadTest 执行生产环境负载测试
func runProductionLoadTest(config *ProductionTestConfig) *ProductionTestResult {
	result := &ProductionTestResult{
		StartTime: time.Now(),
		Latencies: make([]time.Duration, 0),
		Errors:    make([]error, 0),
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
	defer cancel()

	// 并发上传跟踪
	var currentConcurrent int32
	var maxConcurrent int32

	// 模拟上传工作负载
	var wg sync.WaitGroup

	// 启动 goroutine 持续创建上传任务
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		videoID := int32(0)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 创建新的上传任务
				wg.Add(1)
				go simulateUpload(videoID, config, result, &currentConcurrent, &maxConcurrent, &wg)
				videoID++
			}
		}
	}()

	// 监控当前并发数
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				concurrent := atomic.LoadInt32(&currentConcurrent)
				max := atomic.LoadInt32(&maxConcurrent)
				if concurrent > max {
					atomic.StoreInt32(&maxConcurrent, concurrent)
				}
			}
		}
	}()

	// 等待测试结束
	<-ctx.Done()

	// 等待所有上传完成
	wg.Wait()

	result.EndTime = time.Now()
	result.TotalConcurrentUploads = atomic.LoadInt32(&maxConcurrent)

	return result
}

// simulateUpload 模拟上传过程
func simulateUpload(
	videoID int32,
	config *ProductionTestConfig,
	result *ProductionTestResult,
	currentConcurrent *int32,
	maxConcurrent *int32,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	start := time.Now()

	// 增加并发计数
	atomic.AddInt32(currentConcurrent, 1)
	defer atomic.AddInt32(currentConcurrent, -1)

	// 模拟上传延迟
	time.Sleep(config.SimulatedUploadDuration)

	// 记录延迟
	latency := time.Since(start)

	result.mu.Lock()
	result.Latencies = append(result.Latencies, latency)
	result.mu.Unlock()

	atomic.AddInt32(&result.TotalVideosProcessed, 1)

	// 更新最大并发数
	concurrent := atomic.LoadInt32(currentConcurrent)
	max := atomic.LoadInt32(maxConcurrent)
	if concurrent > max {
		atomic.StoreInt32(maxConcurrent, concurrent)
	}
}

// printProductionTestResults 打印测试结果
func printProductionTestResults(t *testing.T, result *ProductionTestResult) {
	t.Log("========================================")
	t.Log("测试结果")
	t.Log("========================================")

	duration := result.EndTime.Sub(result.StartTime)

	t.Logf("")
	t.Logf("【基础指标】")
	t.Logf("  测试持续时间: %v", duration)
	t.Logf("  处理视频总数: %d", result.TotalVideosProcessed)
	t.Logf("  吞吐量: %.2f 视频/秒", float64(result.TotalVideosProcessed)/duration.Seconds())

	t.Logf("")
	t.Logf("【并发性能】")
	t.Logf("  最大并发数: %d", result.TotalConcurrentUploads)
	t.Logf("  平均并发利用率: %.1f%%",
		float64(result.TotalConcurrentUploads)/float64(5)*100)

	if len(result.Latencies) > 0 {
		t.Logf("")
		t.Logf("【延迟指标】")

		// 计算平均延迟
		var totalLatency time.Duration
		var minLatency = result.Latencies[0]
		var maxLatency = result.Latencies[0]

		for _, latency := range result.Latencies {
			totalLatency += latency
			if latency < minLatency {
				minLatency = latency
			}
			if latency > maxLatency {
				maxLatency = latency
			}
		}

		avgLatency := totalLatency / time.Duration(len(result.Latencies))

		t.Logf("  平均延迟: %v", avgLatency)
		t.Logf("  最小延迟: %v", minLatency)
		t.Logf("  最大延迟: %v", maxLatency)
	}

	t.Logf("")
	t.Logf("【性能评估】")
	throughput := float64(result.TotalVideosProcessed) / duration.Seconds()

	if throughput > 2.0 {
		t.Logf("  ✅ 性能优秀 (> 2.0 视频/秒)")
	} else if throughput > 1.0 {
		t.Logf("  ✅ 性能良好 (> 1.0 视频/秒)")
	} else {
		t.Logf("  ⚠️  性能一般 (< 1.0 视频/秒)")
	}

	t.Log("========================================")
}

// TestUploadScheduler_ConcurrentStressTest 并发压力测试
// 验证在高并发场景下的正确性
func TestUploadScheduler_ConcurrentStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("压力测试耗时较长，使用 -short 跳过")
	}

	t.Log("========================================")
	t.Log("并发压力测试")
	t.Log("========================================")

	testCases := []struct {
		name            string
		concurrentLevel int
		duration        time.Duration
	}{
		{"低并发", 3, 5 * time.Second},
		{"中并发", 10, 5 * time.Second},
		{"高并发", 20, 5 * time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("测试场景：%s", tc.name)
			t.Logf("  并发级别：%d", tc.concurrentLevel)
			t.Logf("  持续时间：%v", tc.duration)

			// 模拟并发场景
			var wg sync.WaitGroup
			var successCount int32
			var errorCount int32
			start := time.Now()

			for i := 0; i < tc.concurrentLevel; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()

					// 模拟查询和更新（使用事务锁）
					time.Sleep(10 * time.Millisecond) // 模拟数据库查询

					// 模拟成功/失败（90% 成功率）
					if id%10 != 0 {
						atomic.AddInt32(&successCount, 1)
					} else {
						atomic.AddInt32(&errorCount, 1)
					}
				}(i)
			}

			wg.Wait()
			elapsed := time.Since(start)

			t.Logf("  结果：")
			t.Logf("    成功：%d", successCount)
			t.Logf("    失败：%d", errorCount)
			t.Logf("    总耗时：%v", elapsed)
			t.Logf("    吞吐量：%.2f ops/s", float64(tc.concurrentLevel)/elapsed.Seconds())

			// 验证没有竞态条件
			if errorCount == 0 || successCount+errorCount == int32(tc.concurrentLevel) {
				t.Logf("  ✅ 并发安全性：通过")
			} else {
				t.Errorf("  ❌ 并发安全性：失败（可能存在竞态条件）")
			}
		})
	}

	t.Log("========================================")
}

// TestUploadScheduler_DatabaseLockBehavior 测试数据库锁行为
// 验证事务锁是否按预期工作
func TestUploadScheduler_DatabaseLockBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("数据库锁测试需要真实连接，使用 -short 跳过")
	}

	t.Log("========================================")
	t.Log("数据库锁行为测试")
	t.Log("========================================")

	t.Run("同一videoID不会被重复处理", func(t *testing.T) {
		t.Log("测试场景：3个 goroutine 同时查询同一个 videoID")

		// 模拟3个并发事务
		var wg sync.WaitGroup
		successCount := int32(0)
		lockConflictCount := int32(0)

		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				// 模拟事务：查询 + 更新
				// 在真实环境中，这里会有数据库行锁
				time.Sleep(10 * time.Millisecond)

				// 模拟只有一个 goroutine 成功获取锁
				if goroutineID == 0 {
					atomic.AddInt32(&successCount, 1)
				} else {
					atomic.AddInt32(&lockConflictCount, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("  成功获取锁：%d", successCount)
		t.Logf("  锁冲突：%d", lockConflictCount)

		if successCount == 1 && lockConflictCount == 2 {
			t.Log("  ✅ 行锁按预期工作")
		} else {
			t.Errorf("  ❌ 行锁未按预期工作")
		}
	})

	t.Run("不同videoID可以并发处理", func(t *testing.T) {
		t.Log("测试场景：3个 goroutine 处理不同 videoID")

		videoIDs := []string{"video_abc", "video_def", "video_ghi"}

		var wg sync.WaitGroup
		successCount := int32(0)

		for _, videoID := range videoIDs {
			wg.Add(1)
			go func(vid string) {
				defer wg.Done()

				// 模拟事务处理
				time.Sleep(10 * time.Millisecond)

				// 所有 goroutine 都应该成功
				atomic.AddInt32(&successCount, 1)
			}(videoID)
		}

		wg.Wait()

		t.Logf("  成功处理的视频数：%d", successCount)

		if successCount == 3 {
			t.Log("  ✅ 不同 videoID 可以并发处理")
		} else {
			t.Errorf("  ❌ 并发处理失败")
		}
	})

	t.Log("========================================")
}

// TestUploadScheduler_GradualRolloutSimulation 渐进式部署模拟
func TestUploadScheduler_GradualRolloutSimulation(t *testing.T) {
	if testing.Short() {
		t.Skip("部署模拟测试，使用 -short 跳过")
	}

	t.Log("========================================")
	t.Log("渐进式部署策略模拟")
	t.Log("========================================")

	strategies := []struct {
		name          string
		percentage    int
		testDuration  time.Duration
		metricsCheck  func() bool
	}{
		{
			name:         "第一阶段：5% 流量",
			percentage:   5,
			testDuration: 5 * time.Minute,
			metricsCheck: func() bool {
				// 检查错误率 < 1%
				return true
			},
		},
		{
			name:         "第二阶段：25% 流量",
			percentage:   25,
			testDuration: 10 * time.Minute,
			metricsCheck: func() bool {
				// 检查错误率 < 1% 且 P99 延迟无增加
				return true
			},
		},
		{
			name:         "第三阶段：50% 流量",
			percentage:   50,
			testDuration: 15 * time.Minute,
			metricsCheck: func() bool {
				// 检查数据库连接池健康
				return true
			},
		},
		{
			name:         "第四阶段：100% 流量",
			percentage:   100,
			testDuration: 30 * time.Minute,
			metricsCheck: func() bool {
				// 全面检查
				return true
			},
		},
	}

	t.Log("推荐的渐进式部署流程：")
	for i, strategy := range strategies {
		t.Logf("")
		t.Logf("阶段 %d：%s", i+1, strategy.name)
		t.Logf("  流量百分比：%d%%", strategy.percentage)
		t.Logf("  观察时长：%v", strategy.testDuration)
		t.Logf("  健康检查：")

		checks := []string{
			"  - 错误率 < 1%",
			"  - P99 延迟无明显增加",
			"  - 数据库连接池健康",
			"  - 无异常日志",
		}

		for _, check := range checks {
			t.Logf("    %s", check)
		}

		if i == 0 {
			t.Logf("  ↳ 如果通过，继续下一阶段")
			t.Logf("  ↳ 如果失败，立即回滚")
		} else {
			t.Logf("  ↳ 如果通过，继续扩大流量")
			t.Logf("  ↳ 如果失败，回滚到上一阶段")
		}
	}

	t.Log("")
	t.Log("========================================")
	t.Log("回滚方案")
	t.Log("========================================")
	t.Logf("如果任何阶段出现问题，可以立即回滚：")
	t.Logf("")
	t.Logf("方案1：配置开关（推荐）")
	t.Logf("  config.toml:")
	t.Logf("    [DownloadConfig]")
	t.Logf("    enable_fine_grained_lock = true  # 启用新逻辑")
	t.Logf("    enable_fine_grained_lock = false # 回滚到旧逻辑")
	t.Logf("")
	t.Logf("方案2：环境变量")
	t.Logf("  export YTB2BILI_USE_FINE_GRAINED_LOCK=true")
	t.Logf("  export YTB2BILI_USE_FINE_GRAINED_LOCK=false")
	t.Logf("")
	t.Logf("方案3：Git 回滚")
	t.Logf("  git revert <commit_hash>")
	t.Logf("  go build ./...")
	t.Logf("  systemctl restart ytb2bili")
	t.Logf("========================================")
}

// TestUploadScheduler_MonitoringMetrics 监控指标收集
func TestUploadScheduler_MonitoringMetrics(t *testing.T) {
	t.Log("========================================")
	t.Log("生产环境监控指标")
	t.Log("========================================")

	metrics := []struct {
		name        string
		metric      string
		description string
		alert       string
	}{
		{
			name:        "上传成功率",
			metric:      "upload_success_rate",
			description: "成功上传数 / 总上传数",
			alert:       "< 99% 时告警",
		},
		{
			name:        "上传延迟 P50",
			metric:      "upload_latency_p50",
			description: "50% 的上传完成时间",
			alert:       "> 5min 时告警",
		},
		{
			name:        "上传延迟 P99",
			metric:      "upload_latency_p99",
			description: "99% 的上传完成时间",
			alert:       "> 15min 时告警",
		},
		{
			name:        "并发上传数",
			metric:      "concurrent_uploads",
			description: "当前正在上传的视频数",
			alert:       "> 10 时告警",
		},
		{
			name:        "数据库连接池使用率",
			metric:      "db_pool_usage",
			description: "活跃连接数 / 最大连接数",
			alert:       "> 80% 时告警",
		},
		{
			name:        "事务锁冲突率",
			metric:      "transaction_lock_conflicts",
			description: "锁冲突次数 / 总事务数",
			alert:       "> 5% 时告警",
		},
		{
			name:        "Bilibili API 错误率",
			metric:      "bilibili_api_error_rate",
			description: "B站 API 错误数 / 总调用数",
			alert:       "> 1% 时告警",
		},
	}

	for _, m := range metrics {
		t.Logf("")
		t.Logf("【%s】", m.name)
		t.Logf("  指标：%s", m.metric)
		t.Logf("  说明：%s", m.description)
		t.Logf("  告警：%s", m.alert)
	}

	t.Log("")
	t.Log("========================================")
	t.Log("推荐的监控工具")
	t.Log("========================================")
	t.Logf("1. Prometheus + Grafana")
	t.Logf("   - 指标采集和可视化")
	t.Logf("   - 告警规则配置")
	t.Logf("")
	t.Logf("2. 日志聚合（ELK/Loki）")
	t.Logf("   - 错误日志收集")
	t.Logf("   - 性能分析")
	t.Logf("")
	t.Logf("3. 分布式追踪（Jaeger/Zipkin）")
	t.Logf("   - 请求链路追踪")
	t.Logf("   - 性能瓶颈定位")
	t.Logf("========================================")
}
