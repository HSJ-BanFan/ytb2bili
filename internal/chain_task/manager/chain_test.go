package manager

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// ============================================================================
// Mock Task 实现
// ============================================================================

// MockTask 用于测试的模拟任务
type MockTask struct {
	name           string
	shouldSucceed  bool
	shouldSetSkip  bool // 是否设置 skipped 标记（软跳过）
	executeCalled  bool
	executeCount   int
	executeMu      sync.Mutex
	executeDelay   time.Duration
	panicOnExecute bool
	panicMsg       string
}

func NewMockTask(name string, shouldSucceed bool) *MockTask {
	return &MockTask{
		name:          name,
		shouldSucceed: shouldSucceed,
	}
}

func (m *MockTask) GetName() string {
	return m.name
}

func (m *MockTask) Execute(ctx map[string]interface{}) bool {
	m.executeMu.Lock()
	m.executeCalled = true
	m.executeCount++
	m.executeMu.Unlock()

	if m.panicOnExecute {
		panic(m.panicMsg)
	}

	if m.executeDelay > 0 {
		time.Sleep(m.executeDelay)
	}

	// 检查是否被取消
	if ctxObj, exists := ctx["__ctx__"]; exists {
		if cancelCtx, ok := ctxObj.(context.Context); ok {
			select {
			case <-cancelCtx.Done():
				ctx["error"] = "任务被取消"
				return false
			default:
			}
		}
	}

	if m.shouldSetSkip {
		ctx["skipped"] = "任务被软跳过"
	}

	if !m.shouldSucceed {
		ctx["error"] = "mock task failed"
	}

	return m.shouldSucceed
}

func (m *MockTask) InsertTask() error {
	return nil // 测试中不需要实际插入数据库
}

func (m *MockTask) UpdateStatus(status, message string) error {
	return nil // 测试中不需要实际更新数据库
}

// WasExecuted 检查任务是否被执行
func (m *MockTask) WasExecuted() bool {
	m.executeMu.Lock()
	defer m.executeMu.Unlock()
	return m.executeCalled
}

// ExecuteCount 获取执行次数
func (m *MockTask) ExecuteCount() int {
	m.executeMu.Lock()
	defer m.executeMu.Unlock()
	return m.executeCount
}

// WithDelay 设置执行延迟
func (m *MockTask) WithDelay(d time.Duration) *MockTask {
	m.executeDelay = d
	return m
}

// WithPanic 设置执行时触发 panic
func (m *MockTask) WithPanic(msg string) *MockTask {
	m.panicOnExecute = true
	m.panicMsg = msg
	return m
}

// WithSoftSkip 设置软跳过标记
func (m *MockTask) WithSoftSkip() *MockTask {
	m.shouldSetSkip = true
	return m
}

// ============================================================================
// TaskChain 创建和基本操作测试
// ============================================================================

func TestNewTaskChain(t *testing.T) {
	chain := NewTaskChain()

	assert.NotNil(t, chain)
	assert.NotNil(t, chain.Tasks)
	assert.NotNil(t, chain.Context)
	assert.NotNil(t, chain.CompletedTasks)
	assert.NotNil(t, chain.FailedTasks)
	assert.NotNil(t, chain.SkippedTasks)
	assert.NotNil(t, chain.SoftSkippedTasks)
	assert.Empty(t, chain.Tasks)
}

func TestTaskChain_SetLogger(t *testing.T) {
	chain := NewTaskChain()
	logger := zaptest.NewLogger(t).Sugar()

	result := chain.SetLogger(logger)

	assert.Same(t, chain, result) // 链式调用返回自身
	assert.NotNil(t, chain.Logger)
}

func TestTaskChain_SetVideoID(t *testing.T) {
	chain := NewTaskChain()

	result := chain.SetVideoID("test_video_123")

	assert.Same(t, chain, result)
	assert.Equal(t, "test_video_123", chain.VideoID)
}

func TestTaskChain_SetContext(t *testing.T) {
	chain := NewTaskChain()
	ctx := context.Background()

	result := chain.SetContext(ctx)

	assert.Same(t, chain, result)
	assert.NotNil(t, chain.Ctx)
}

func TestTaskChain_SetCompletedTasks(t *testing.T) {
	chain := NewTaskChain()

	result := chain.SetCompletedTasks([]string{"任务A", "任务B", "任务C"})

	assert.Same(t, chain, result)
	assert.True(t, chain.CompletedTasks["任务A"])
	assert.True(t, chain.CompletedTasks["任务B"])
	assert.True(t, chain.CompletedTasks["任务C"])
	assert.False(t, chain.CompletedTasks["任务D"])
}

func TestTaskChain_AddTask(t *testing.T) {
	chain := NewTaskChain()
	task := NewMockTask("测试任务", true)

	result := chain.AddTask(task)

	assert.Same(t, chain, result)
	assert.Len(t, chain.Tasks, 1)
	assert.Equal(t, task, chain.Tasks[0])
}

func TestTaskChain_ChainedSetters(t *testing.T) {
	logger := zaptest.NewLogger(t).Sugar()
	ctx := context.Background()

	chain := NewTaskChain().
		SetLogger(logger).
		SetVideoID("abc123").
		SetContext(ctx).
		SetCompletedTasks([]string{"任务A"}).
		AddTask(NewMockTask("任务B", true))

	assert.NotNil(t, chain.Logger)
	assert.Equal(t, "abc123", chain.VideoID)
	assert.NotNil(t, chain.Ctx)
	assert.True(t, chain.CompletedTasks["任务A"])
	assert.Len(t, chain.Tasks, 1)
}

// ============================================================================
// 任务执行测试
// ============================================================================

func TestTaskChain_Run_SingleTask_Success(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	task := NewMockTask("测试任务", true)
	chain.AddTask(task)

	result := chain.Run(true)

	assert.True(t, task.WasExecuted())
	assert.True(t, chain.CompletedTasks["测试任务"])
	assert.Empty(t, chain.FailedTasks)
	assert.Nil(t, result["error"])
}

func TestTaskChain_Run_SingleTask_Failure(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	task := NewMockTask("失败任务", false)
	chain.AddTask(task)

	result := chain.Run(true)

	assert.True(t, task.WasExecuted())
	assert.True(t, chain.FailedTasks["失败任务"])
	assert.Empty(t, chain.CompletedTasks)
	assert.NotNil(t, result["error"])
}

func TestTaskChain_Run_MultipleTask_AllSuccess(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	task1 := NewMockTask("任务1", true)
	task2 := NewMockTask("任务2", true)
	task3 := NewMockTask("任务3", true)

	chain.AddTask(task1).AddTask(task2).AddTask(task3)

	chain.Run(true)

	assert.True(t, task1.WasExecuted())
	assert.True(t, task2.WasExecuted())
	assert.True(t, task3.WasExecuted())
	assert.Len(t, chain.CompletedTasks, 3)
	assert.Empty(t, chain.FailedTasks)
}

// ============================================================================
// 依赖检查测试
// ============================================================================

func TestTaskChain_checkDependencies_NoDependencies(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	// "获取元数据" 没有依赖
	ok, reason := chain.checkDependencies("获取元数据")

	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestTaskChain_checkDependencies_UnknownTask(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	// 未知任务应返回 true（无依赖）
	ok, reason := chain.checkDependencies("未知任务")

	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestTaskChain_checkDependencies_DependencySatisfied(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	// "翻译字幕" 依赖 "下载字幕"
	chain.CompletedTasks["下载字幕"] = true

	ok, reason := chain.checkDependencies("翻译字幕")

	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestTaskChain_checkDependencies_DependencyNotExecuted(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	// "翻译字幕" 依赖 "下载字幕"，但 "下载字幕" 未执行
	ok, reason := chain.checkDependencies("翻译字幕")

	assert.False(t, ok)
	assert.Contains(t, reason, "下载字幕")
	assert.Contains(t, reason, "未执行")
}

func TestTaskChain_checkDependencies_DependencyFailed(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	// "翻译字幕" 依赖 "下载字幕"，但 "下载字幕" 失败
	chain.FailedTasks["下载字幕"] = true

	ok, reason := chain.checkDependencies("翻译字幕")

	assert.False(t, ok)
	assert.Contains(t, reason, "下载字幕")
	assert.Contains(t, reason, "失败")
}

func TestTaskChain_checkDependencies_DependencySkipped(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	// "翻译字幕" 依赖 "下载字幕"，但 "下载字幕" 被硬跳过
	chain.SkippedTasks["下载字幕"] = true

	ok, reason := chain.checkDependencies("翻译字幕")

	assert.False(t, ok)
	assert.Contains(t, reason, "下载字幕")
	assert.Contains(t, reason, "跳过")
}

func TestTaskChain_checkDependencies_DependencySoftSkipped(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	// "翻译字幕" 依赖 "下载字幕"
	// "下载字幕" 软跳过（执行成功但无需处理）应满足依赖
	chain.SoftSkippedTasks["下载字幕"] = true

	ok, reason := chain.checkDependencies("翻译字幕")

	assert.True(t, ok)
	assert.Empty(t, reason)
}

// ============================================================================
// 软跳过和硬跳过测试
// ============================================================================

func TestTaskChain_Run_SoftSkip(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	// 使用自定义任务名（不在 TaskConfigs 中），避免依赖检查
	task := NewMockTask("自定义软跳过任务", true).WithSoftSkip()
	chain.AddTask(task)

	chain.Run(true)

	assert.True(t, task.WasExecuted())
	assert.True(t, chain.SoftSkippedTasks["自定义软跳过任务"])
	assert.False(t, chain.CompletedTasks["自定义软跳过任务"])
	assert.False(t, chain.SkippedTasks["自定义软跳过任务"])
}

func TestTaskChain_Run_SoftSkipDoesNotBlockDependents(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	// "下载字幕" 软跳过，"翻译字幕" 依赖它，应该仍能执行
	task1 := NewMockTask("下载字幕", true).WithSoftSkip()
	task2 := NewMockTask("翻译字幕", true)

	chain.AddTask(task1).AddTask(task2)

	chain.Run(true)

	assert.True(t, task1.WasExecuted())
	assert.True(t, task2.WasExecuted())
	assert.True(t, chain.SoftSkippedTasks["下载字幕"])
	assert.True(t, chain.CompletedTasks["翻译字幕"])
}

func TestTaskChain_Run_HardSkipBlocksDependents(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	// 模拟 "下载字幕" 失败，"翻译字幕" 依赖它
	// 注意：翻译字幕在 TaskConfigs 中依赖下载字幕
	task1 := NewMockTask("下载字幕", false)
	task2 := NewMockTask("翻译字幕", true)

	chain.AddTask(task1).AddTask(task2)

	chain.Run(true)

	assert.True(t, task1.WasExecuted())
	// 翻译字幕不会被执行，因为依赖失败
	assert.False(t, task2.WasExecuted())
	assert.True(t, chain.FailedTasks["下载字幕"])
	assert.True(t, chain.SkippedTasks["翻译字幕"])
}

// ============================================================================
// 必需任务测试
// ============================================================================

func TestTaskChain_Run_RequiredTaskFailure_StopsChain(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	// "下载视频" 是必需任务
	task1 := NewMockTask("下载视频", false)
	task2 := NewMockTask("其他任务", true)

	chain.AddTask(task1).AddTask(task2)

	chain.Run(true) // stopOnRequiredFailure = true

	assert.True(t, task1.WasExecuted())
	assert.False(t, task2.WasExecuted()) // 链被终止，此任务未执行
}

func TestTaskChain_Run_RequiredTaskFailure_ContinuesIfNotStopOnFailure(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	// "下载视频" 是必需任务
	task1 := NewMockTask("下载视频", false)
	task2 := NewMockTask("其他任务", true)

	chain.AddTask(task1).AddTask(task2)

	chain.Run(false) // stopOnRequiredFailure = false

	assert.True(t, task1.WasExecuted())
	assert.True(t, task2.WasExecuted()) // 链继续执行
}

func TestTaskChain_Run_OptionalTaskFailure_DoesNotStopChain(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	// "下载字幕" 不是必需任务
	task1 := NewMockTask("下载字幕", false)
	task2 := NewMockTask("其他任务", true)

	chain.AddTask(task1).AddTask(task2)

	chain.Run(true)

	assert.True(t, task1.WasExecuted())
	assert.True(t, task2.WasExecuted()) // 可选任务失败不会终止链
}

// ============================================================================
// 取消上下文测试
// ============================================================================

func TestTaskChain_Run_Cancellation(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	ctx, cancel := context.WithCancel(context.Background())
	chain.SetContext(ctx)

	// 第一个任务执行时取消
	task1 := NewMockTask("长时间任务", true).WithDelay(100 * time.Millisecond)
	task2 := NewMockTask("后续任务", true)

	chain.AddTask(task1).AddTask(task2)

	// 在后台取消
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result := chain.Run(true)

	// 取消后的行为取决于时机，至少不应 panic
	assert.NotNil(t, result)
}

func TestTaskChain_Run_CancelledBeforeStart(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消
	chain.SetContext(ctx)

	task := NewMockTask("测试任务", true)
	chain.AddTask(task)

	result := chain.Run(true)

	// 链应该被取消
	assert.Equal(t, true, result["canceled"])
	assert.False(t, task.WasExecuted())
}

// ============================================================================
// Panic 恢复测试
// ============================================================================

func TestTaskChain_Run_PanicRecovery(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	task1 := NewMockTask("会 panic 的任务", true).WithPanic("测试 panic")
	task2 := NewMockTask("后续任务", true)

	chain.AddTask(task1).AddTask(task2)

	// 不应该 panic
	require.NotPanics(t, func() {
		chain.Run(true)
	})

	assert.True(t, chain.FailedTasks["会 panic 的任务"])
	assert.True(t, task2.WasExecuted()) // 后续任务仍应执行
}

// ============================================================================
// RunWithContext 测试
// ============================================================================

func TestTaskChain_RunWithContext(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	task := NewMockTask("测试任务", true)
	chain.AddTask(task)

	initialContext := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}

	result := chain.RunWithContext(initialContext)

	// 验证初始 context 被合并
	assert.Equal(t, "value1", chain.Context["key1"])
	assert.Equal(t, 42, chain.Context["key2"])
	assert.True(t, task.WasExecuted())
	assert.NotNil(t, result)
}

// ============================================================================
// TaskConfigs 测试
// ============================================================================

func TestTaskConfigs_RequiredTasks(t *testing.T) {
	requiredTasks := []string{"获取元数据", "下载视频", "下载封面"}

	for _, taskName := range requiredTasks {
		config, exists := TaskConfigs[taskName]
		assert.True(t, exists, "任务 %s 应该存在于配置中", taskName)
		assert.True(t, config.Required, "任务 %s 应该是必需的", taskName)
	}
}

func TestTaskConfigs_OptionalTasks(t *testing.T) {
	optionalTasks := []string{"下载字幕", "翻译字幕", "AI增强元数据", "确认元数据", "上传到Bilibili", "上传字幕到Bilibili"}

	for _, taskName := range optionalTasks {
		config, exists := TaskConfigs[taskName]
		assert.True(t, exists, "任务 %s 应该存在于配置中", taskName)
		assert.False(t, config.Required, "任务 %s 不应该是必需的", taskName)
	}
}

func TestTaskConfigs_Dependencies(t *testing.T) {
	tests := []struct {
		taskName     string
		dependencies []string
	}{
		{"获取元数据", nil},
		{"下载视频", nil},
		{"下载字幕", nil},
		{"下载封面", []string{"获取元数据"}},
		{"翻译字幕", []string{"下载字幕"}},
		{"AI增强元数据", []string{"下载视频"}},
		{"确认元数据", []string{"AI增强元数据"}},
		{"上传到Bilibili", []string{"确认元数据"}},
		{"上传字幕到Bilibili", []string{"上传到Bilibili"}},
	}

	for _, tt := range tests {
		t.Run(tt.taskName, func(t *testing.T) {
			config, exists := TaskConfigs[tt.taskName]
			require.True(t, exists)

			if tt.dependencies == nil {
				assert.Nil(t, config.Dependencies)
			} else {
				assert.Equal(t, tt.dependencies, config.Dependencies)
			}
		})
	}
}

// ============================================================================
// 状态跟踪测试
// ============================================================================

func TestTaskChain_StateTracking(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	successTask := NewMockTask("成功任务", true)
	failTask := NewMockTask("失败任务", false)
	softSkipTask := NewMockTask("软跳过任务", true).WithSoftSkip()

	chain.AddTask(successTask).AddTask(failTask).AddTask(softSkipTask)

	chain.Run(false)

	assert.True(t, chain.CompletedTasks["成功任务"])
	assert.True(t, chain.FailedTasks["失败任务"])
	assert.True(t, chain.SoftSkippedTasks["软跳过任务"])
}

// ============================================================================
// 边界情况测试
// ============================================================================

func TestTaskChain_Run_EmptyChain(t *testing.T) {
	chain := NewTaskChain()
	chain.SetLogger(zaptest.NewLogger(t).Sugar())

	// 空链不应 panic
	require.NotPanics(t, func() {
		result := chain.Run(true)
		assert.NotNil(t, result)
	})
}

func TestTaskChain_Run_NilLogger(t *testing.T) {
	chain := NewTaskChain()
	// 不设置 Logger

	task := NewMockTask("测试任务", true)
	chain.AddTask(task)

	// 不应 panic
	require.NotPanics(t, func() {
		chain.Run(true)
	})

	assert.True(t, task.WasExecuted())
}

// ============================================================================
// 并发安全测试
// ============================================================================

func TestTaskChain_ConcurrentExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过并发测试")
	}

	// 多个 TaskChain 并发执行，不应相互影响
	var wg sync.WaitGroup
	numChains := 10

	for i := 0; i < numChains; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			chain := NewTaskChain()
			chain.SetVideoID("video_" + string(rune('A'+id)))

			task := NewMockTask("任务", true).WithDelay(10 * time.Millisecond)
			chain.AddTask(task)

			result := chain.Run(true)
			assert.NotNil(t, result)
		}(i)
	}

	wg.Wait()
}

// ============================================================================
// 性能基准测试
// ============================================================================

func BenchmarkNewTaskChain(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewTaskChain()
	}
}

func BenchmarkTaskChain_AddTask(b *testing.B) {
	chain := NewTaskChain()
	task := NewMockTask("测试任务", true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chain.AddTask(task)
	}
}

func BenchmarkTaskChain_checkDependencies(b *testing.B) {
	chain := NewTaskChain()
	chain.CompletedTasks["下载字幕"] = true

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chain.checkDependencies("翻译字幕")
	}
}

func BenchmarkTaskChain_Run_SingleTask(b *testing.B) {
	for i := 0; i < b.N; i++ {
		chain := NewTaskChain()
		task := NewMockTask("测试任务", true)
		chain.AddTask(task)
		chain.Run(true)
	}
}
