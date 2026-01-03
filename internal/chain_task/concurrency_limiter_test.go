package chain_task

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// ============================================================================
// Mock PermissionService
// ============================================================================

// MockPermissionService 模拟权限服务
type MockPermissionService struct {
	maxConcurrentTasks map[string]int // userID -> maxConcurrent
	defaultMax         int
	shouldFail         bool
	mu                 sync.RWMutex
}

func NewMockPermissionService(defaultMax int) *MockPermissionService {
	return &MockPermissionService{
		maxConcurrentTasks: make(map[string]int),
		defaultMax:         defaultMax,
	}
}

func (m *MockPermissionService) SetMaxConcurrent(userID string, max int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxConcurrentTasks[userID] = max
}

func (m *MockPermissionService) SetShouldFail(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = fail
}

func (m *MockPermissionService) GetMaxConcurrentTasks(ctx context.Context, userID string) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldFail {
		return 1, assert.AnError
	}

	if max, exists := m.maxConcurrentTasks[userID]; exists {
		return max, nil
	}
	return m.defaultMax, nil
}

// ============================================================================
// 基本功能测试
// ============================================================================

func TestNewConcurrencyLimiter(t *testing.T) {
	mockPerm := NewMockPermissionService(3)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	assert.NotNil(t, limiter)
	assert.NotNil(t, limiter.userConcurrency)
	assert.Empty(t, limiter.userConcurrency)
}

func TestConcurrencyLimiter_TryAcquire_Success(t *testing.T) {
	mockPerm := NewMockPermissionService(3)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	ctx := context.Background()
	ok, current, max, err := limiter.TryAcquire(ctx, 100)

	assert.True(t, ok)
	assert.Equal(t, 1, current)
	assert.Equal(t, 3, max)
	assert.NoError(t, err)
}

func TestConcurrencyLimiter_TryAcquire_ReachLimit(t *testing.T) {
	mockPerm := NewMockPermissionService(2)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	ctx := context.Background()
	userID := uint(100)

	// 获取第一个槽位
	ok1, current1, max1, _ := limiter.TryAcquire(ctx, userID)
	assert.True(t, ok1)
	assert.Equal(t, 1, current1)
	assert.Equal(t, 2, max1)

	// 获取第二个槽位
	ok2, current2, max2, _ := limiter.TryAcquire(ctx, userID)
	assert.True(t, ok2)
	assert.Equal(t, 2, current2)
	assert.Equal(t, 2, max2)

	// 第三个应该失败
	ok3, current3, max3, _ := limiter.TryAcquire(ctx, userID)
	assert.False(t, ok3)
	assert.Equal(t, 2, current3)
	assert.Equal(t, 2, max3)
}

func TestConcurrencyLimiter_TryAcquire_UnlimitedConcurrency(t *testing.T) {
	mockPerm := NewMockPermissionService(-1) // -1 表示无限制
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	ctx := context.Background()
	userID := uint(100)

	// 应该可以获取任意数量的槽位
	for i := 1; i <= 100; i++ {
		ok, current, max, _ := limiter.TryAcquire(ctx, userID)
		assert.True(t, ok, "第 %d 次获取应成功", i)
		assert.Equal(t, i, current)
		assert.Equal(t, -1, max)
	}
}

func TestConcurrencyLimiter_TryAcquire_PermissionError_FallbackToDefault(t *testing.T) {
	mockPerm := NewMockPermissionService(3)
	mockPerm.SetShouldFail(true)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	ctx := context.Background()
	userID := uint(100)

	// 权限服务失败，应该回退到默认值 1
	ok1, _, max1, _ := limiter.TryAcquire(ctx, userID)
	assert.True(t, ok1)
	assert.Equal(t, 1, max1) // 默认值

	// 达到限制
	ok2, _, _, _ := limiter.TryAcquire(ctx, userID)
	assert.False(t, ok2)
}

// ============================================================================
// Release 测试
// ============================================================================

func TestConcurrencyLimiter_Release(t *testing.T) {
	mockPerm := NewMockPermissionService(3)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	ctx := context.Background()
	userID := uint(100)

	// 获取槽位
	limiter.TryAcquire(ctx, userID)
	limiter.TryAcquire(ctx, userID)

	current1, _ := limiter.GetStats(userID)
	assert.Equal(t, 2, current1)

	// 释放一个
	limiter.Release(userID)
	current2, _ := limiter.GetStats(userID)
	assert.Equal(t, 1, current2)

	// 再释放一个
	limiter.Release(userID)
	current3, _ := limiter.GetStats(userID)
	assert.Equal(t, 0, current3)
}

func TestConcurrencyLimiter_Release_MultipleRelease_NoNegative(t *testing.T) {
	mockPerm := NewMockPermissionService(3)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	ctx := context.Background()
	userID := uint(100)

	// 获取一个槽位
	limiter.TryAcquire(ctx, userID)

	// 多次释放
	limiter.Release(userID)
	limiter.Release(userID)
	limiter.Release(userID)

	// 不应该变成负数
	current, _ := limiter.GetStats(userID)
	assert.Equal(t, 0, current)
}

func TestConcurrencyLimiter_Release_NonExistentUser(t *testing.T) {
	mockPerm := NewMockPermissionService(3)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	// 释放不存在的用户不应 panic
	require.NotPanics(t, func() {
		limiter.Release(999)
	})
}

// ============================================================================
// 用户隔离测试
// ============================================================================

func TestConcurrencyLimiter_UserIsolation(t *testing.T) {
	mockPerm := NewMockPermissionService(2)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	ctx := context.Background()
	userA := uint(100)
	userB := uint(200)

	// UserA 获取 2 个槽位（达到上限）
	limiter.TryAcquire(ctx, userA)
	limiter.TryAcquire(ctx, userA)

	// UserA 无法再获取
	okA, _, _, _ := limiter.TryAcquire(ctx, userA)
	assert.False(t, okA)

	// UserB 应该不受影响
	okB, _, _, _ := limiter.TryAcquire(ctx, userB)
	assert.True(t, okB)

	// 验证统计
	currentA, _ := limiter.GetStats(userA)
	currentB, _ := limiter.GetStats(userB)
	assert.Equal(t, 2, currentA)
	assert.Equal(t, 1, currentB)
}

func TestConcurrencyLimiter_DifferentUserLimits(t *testing.T) {
	mockPerm := NewMockPermissionService(1)
	// 设置不同用户不同限制
	mockPerm.SetMaxConcurrent("100", 1)  // Free tier
	mockPerm.SetMaxConcurrent("200", 3)  // Pro tier
	mockPerm.SetMaxConcurrent("300", -1) // Enterprise tier (无限)

	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)
	ctx := context.Background()

	// Free user: 只能获取 1 个
	okFree1, _, _, _ := limiter.TryAcquire(ctx, 100)
	okFree2, _, _, _ := limiter.TryAcquire(ctx, 100)
	assert.True(t, okFree1)
	assert.False(t, okFree2)

	// Pro user: 可以获取 3 个
	for i := 1; i <= 3; i++ {
		ok, _, _, _ := limiter.TryAcquire(ctx, 200)
		assert.True(t, ok, "Pro user should get slot %d", i)
	}
	okPro4, _, _, _ := limiter.TryAcquire(ctx, 200)
	assert.False(t, okPro4)

	// Enterprise user: 无限
	for i := 1; i <= 10; i++ {
		ok, _, _, _ := limiter.TryAcquire(ctx, 300)
		assert.True(t, ok, "Enterprise user should get slot %d", i)
	}
}

// ============================================================================
// GetStats 测试
// ============================================================================

func TestConcurrencyLimiter_GetStats(t *testing.T) {
	mockPerm := NewMockPermissionService(5)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	ctx := context.Background()
	userID := uint(100)

	// 初始状态
	current, max := limiter.GetStats(userID)
	assert.Equal(t, 0, current)
	assert.Equal(t, 5, max)

	// 获取几个槽位后
	limiter.TryAcquire(ctx, userID)
	limiter.TryAcquire(ctx, userID)

	current, max = limiter.GetStats(userID)
	assert.Equal(t, 2, current)
	assert.Equal(t, 5, max)
}

func TestConcurrencyLimiter_GetAllStats(t *testing.T) {
	mockPerm := NewMockPermissionService(3)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	ctx := context.Background()

	// 多个用户获取槽位
	limiter.TryAcquire(ctx, 100)
	limiter.TryAcquire(ctx, 100)
	limiter.TryAcquire(ctx, 200)
	limiter.TryAcquire(ctx, 300)
	limiter.TryAcquire(ctx, 300)
	limiter.TryAcquire(ctx, 300)

	stats := limiter.GetAllStats()

	assert.Equal(t, 2, stats[100])
	assert.Equal(t, 1, stats[200])
	assert.Equal(t, 3, stats[300])
	assert.Len(t, stats, 3)
}

// ============================================================================
// Reset 和 ForceRelease 测试
// ============================================================================

func TestConcurrencyLimiter_Reset(t *testing.T) {
	mockPerm := NewMockPermissionService(3)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	ctx := context.Background()

	// 多个用户获取槽位
	limiter.TryAcquire(ctx, 100)
	limiter.TryAcquire(ctx, 200)
	limiter.TryAcquire(ctx, 300)

	stats := limiter.GetAllStats()
	assert.Len(t, stats, 3)

	// 重置
	limiter.Reset()

	stats = limiter.GetAllStats()
	assert.Empty(t, stats)
}

func TestConcurrencyLimiter_Reset_Empty(t *testing.T) {
	mockPerm := NewMockPermissionService(3)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	// 重置空的 limiter 不应 panic
	require.NotPanics(t, func() {
		limiter.Reset()
	})
}

func TestConcurrencyLimiter_ForceRelease(t *testing.T) {
	mockPerm := NewMockPermissionService(3)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	ctx := context.Background()
	userID := uint(100)

	// 获取多个槽位
	limiter.TryAcquire(ctx, userID)
	limiter.TryAcquire(ctx, userID)
	limiter.TryAcquire(ctx, userID)

	current, _ := limiter.GetStats(userID)
	assert.Equal(t, 3, current)

	// 强制释放
	limiter.ForceRelease(userID)

	current, _ = limiter.GetStats(userID)
	assert.Equal(t, 0, current)
}

func TestConcurrencyLimiter_ForceRelease_NonExistent(t *testing.T) {
	mockPerm := NewMockPermissionService(3)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	// 强制释放不存在的用户不应 panic
	require.NotPanics(t, func() {
		limiter.ForceRelease(999)
	})
}

// ============================================================================
// 并发安全测试
// ============================================================================

func TestConcurrencyLimiter_ConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过并发测试")
	}

	mockPerm := NewMockPermissionService(100) // 高限制避免阻塞
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	ctx := context.Background()
	userID := uint(100)
	numGoroutines := 50
	var wg sync.WaitGroup

	var successCount int32

	// 并发获取
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _, _, _ := limiter.TryAcquire(ctx, userID)
			if ok {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	// 所有获取都应成功（限制足够高）
	assert.Equal(t, int32(numGoroutines), successCount)

	// 并发释放
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			limiter.Release(userID)
		}()
	}

	wg.Wait()

	current, _ := limiter.GetStats(userID)
	assert.Equal(t, 0, current)
}

func TestConcurrencyLimiter_ConcurrentAcquireRelease(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过并发测试")
	}

	mockPerm := NewMockPermissionService(5)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	ctx := context.Background()
	userID := uint(100)
	numOperations := 100
	var wg sync.WaitGroup

	// 并发获取和释放
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				limiter.TryAcquire(ctx, userID)
			} else {
				limiter.Release(userID)
			}
		}(i)
	}

	wg.Wait()

	// 最终状态应该是非负的
	current, _ := limiter.GetStats(userID)
	assert.GreaterOrEqual(t, current, 0)
}

func TestConcurrencyLimiter_ConcurrentMultipleUsers(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过并发测试")
	}

	mockPerm := NewMockPermissionService(10)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	ctx := context.Background()
	numUsers := 10
	numOpsPerUser := 20
	var wg sync.WaitGroup

	// 多用户并发操作
	for userID := uint(1); userID <= uint(numUsers); userID++ {
		for i := 0; i < numOpsPerUser; i++ {
			wg.Add(1)
			go func(uid uint, op int) {
				defer wg.Done()
				if op%3 == 0 {
					limiter.TryAcquire(ctx, uid)
				} else if op%3 == 1 {
					limiter.Release(uid)
				} else {
					limiter.GetStats(uid)
				}
			}(userID, i)
		}
	}

	wg.Wait()

	// 不应 panic，统计应正常
	stats := limiter.GetAllStats()
	for _, v := range stats {
		assert.GreaterOrEqual(t, v, 0)
	}
}

// ============================================================================
// 边界情况测试
// ============================================================================

func TestConcurrencyLimiter_ZeroLimit(t *testing.T) {
	mockPerm := NewMockPermissionService(0)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	ctx := context.Background()

	// 限制为 0 就无法获取任何槽位
	ok, _, max, _ := limiter.TryAcquire(ctx, 100)
	assert.False(t, ok)
	assert.Equal(t, 0, max)
}

func TestConcurrencyLimiter_AcquireAfterRelease(t *testing.T) {
	mockPerm := NewMockPermissionService(1)
	logger := zaptest.NewLogger(t).Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	ctx := context.Background()
	userID := uint(100)

	// 获取（达到上限）
	ok1, _, _, _ := limiter.TryAcquire(ctx, userID)
	assert.True(t, ok1)

	// 无法再获取
	ok2, _, _, _ := limiter.TryAcquire(ctx, userID)
	assert.False(t, ok2)

	// 释放
	limiter.Release(userID)

	// 应该能再次获取
	ok3, _, _, _ := limiter.TryAcquire(ctx, userID)
	assert.True(t, ok3)
}

// ============================================================================
// 性能基准测试
// ============================================================================

func BenchmarkConcurrencyLimiter_TryAcquire(b *testing.B) {
	mockPerm := NewMockPermissionService(-1)
	logger := zap.NewNop().Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.TryAcquire(ctx, 100)
	}
}

func BenchmarkConcurrencyLimiter_Release(b *testing.B) {
	mockPerm := NewMockPermissionService(-1)
	logger := zap.NewNop().Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	// 预先获取大量槽位
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		limiter.TryAcquire(ctx, 100)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Release(100)
	}
}

func BenchmarkConcurrencyLimiter_GetStats(b *testing.B) {
	mockPerm := NewMockPermissionService(10)
	logger := zap.NewNop().Sugar()
	limiter := NewConcurrencyLimiter(mockPerm, logger)

	// 预设数据
	limiter.userConcurrency[100] = 5
	limiter.userConcurrency[200] = 3
	limiter.userConcurrency[300] = 7

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.GetStats(100)
	}
}
