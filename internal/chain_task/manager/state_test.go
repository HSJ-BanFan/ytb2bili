package manager

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// NewStateManager 测试
// ============================================================================

// TestNewStateManager_BasicCreation 测试基本的 StateManager 创建
func TestNewStateManager_BasicCreation(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()

	// 创建 StateManager
	createTime := time.Date(2026, 1, 3, 12, 0, 0, 0, time.Local)
	sm := NewStateManager(1, 100, "test_video_123", tempDir, createTime)

	// 验证基本字段
	assert.Equal(t, uint(1), sm.Id)
	assert.Equal(t, uint(100), sm.UserID)
	assert.Equal(t, "test_video_123", sm.VideoID)
	assert.Equal(t, tempDir, sm.ProjectRoot)

	// 验证目录结构
	expectedDir := filepath.Join(tempDir, "user_100", "2026-01-03", "test_video_123")
	assert.Equal(t, expectedDir, sm.CurrentDir)

	// 验证目录已创建
	assert.DirExists(t, sm.CurrentDir)
}

// TestNewStateManager_UserIsolation 测试用户目录隔离
func TestNewStateManager_UserIsolation(t *testing.T) {
	tempDir := t.TempDir()
	createTime := time.Now()

	// 创建两个不同用户的 StateManager
	sm1 := NewStateManager(1, 100, "video_1", tempDir, createTime)
	sm2 := NewStateManager(2, 200, "video_2", tempDir, createTime)

	// 验证目录不同
	assert.NotEqual(t, sm1.CurrentDir, sm2.CurrentDir)
	assert.Contains(t, sm1.CurrentDir, "user_100")
	assert.Contains(t, sm2.CurrentDir, "user_200")

	// 验证两个目录都存在
	assert.DirExists(t, sm1.CurrentDir)
	assert.DirExists(t, sm2.CurrentDir)
}

// TestNewStateManager_DateBasedDirectory 测试基于日期的目录结构
func TestNewStateManager_DateBasedDirectory(t *testing.T) {
	tempDir := t.TempDir()

	// 使用不同日期创建
	date1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	date2 := time.Date(2026, 1, 2, 0, 0, 0, 0, time.Local)

	sm1 := NewStateManager(1, 100, "video_a", tempDir, date1)
	sm2 := NewStateManager(2, 100, "video_b", tempDir, date2)

	// 同一用户，不同日期应该在不同目录
	assert.Contains(t, sm1.CurrentDir, "2026-01-01")
	assert.Contains(t, sm2.CurrentDir, "2026-01-02")
	assert.NotEqual(t, sm1.CurrentDir, sm2.CurrentDir)
}

// TestNewStateManager_FilePathGeneration 测试文件路径生成
func TestNewStateManager_FilePathGeneration(t *testing.T) {
	tempDir := t.TempDir()
	createTime := time.Now()
	videoID := "abc123"

	sm := NewStateManager(1, 100, videoID, tempDir, createTime)

	// 验证视频文件路径
	assert.Equal(t, filepath.Join(sm.CurrentDir, videoID+".mp4"), sm.InputVideoPath)
	assert.Equal(t, filepath.Join(sm.CurrentDir, videoID+"out.mp4"), sm.OutVideoPath)
	assert.Equal(t, filepath.Join(sm.CurrentDir, videoID+".mp3"), sm.OriginalMP3)

	// 验证字幕文件路径
	assert.Equal(t, filepath.Join(sm.CurrentDir, "en.srt"), sm.OriginalSRT)
	assert.Equal(t, filepath.Join(sm.CurrentDir, "zh.srt"), sm.TranslateSRT)
	assert.Equal(t, filepath.Join(sm.CurrentDir, "en.json"), sm.OriginalJSON)
	assert.Equal(t, filepath.Join(sm.CurrentDir, "zh.json"), sm.TranslateJSON)

	// 验证封面路径
	assert.Equal(t, filepath.Join(sm.CurrentDir, "cover.jpg"), sm.ImageCover)
}

// ============================================================================
// Cache 测试
// ============================================================================

// TestStateManager_CacheBasicOperations 测试缓存的基本操作
func TestStateManager_CacheBasicOperations(t *testing.T) {
	tempDir := t.TempDir()
	sm := NewStateManager(1, 100, "video_1", tempDir, time.Now())

	// 测试设置和获取
	sm.SetCache("key1", "value1")
	sm.SetCache("key2", 42)
	sm.SetCache("key3", map[string]string{"a": "b"})

	// 验证获取
	val1, ok1 := sm.GetCache("key1")
	assert.True(t, ok1)
	assert.Equal(t, "value1", val1)

	val2, ok2 := sm.GetCache("key2")
	assert.True(t, ok2)
	assert.Equal(t, 42, val2)

	val3, ok3 := sm.GetCache("key3")
	assert.True(t, ok3)
	assert.Equal(t, map[string]string{"a": "b"}, val3)

	// 测试不存在的键
	_, ok := sm.GetCache("nonexistent")
	assert.False(t, ok)
}

// TestStateManager_CacheOverwrite 测试缓存覆盖
func TestStateManager_CacheOverwrite(t *testing.T) {
	tempDir := t.TempDir()
	sm := NewStateManager(1, 100, "video_1", tempDir, time.Now())

	// 设置初始值
	sm.SetCache("key", "original")
	val, _ := sm.GetCache("key")
	assert.Equal(t, "original", val)

	// 覆盖
	sm.SetCache("key", "updated")
	val, _ = sm.GetCache("key")
	assert.Equal(t, "updated", val)
}

// TestStateManager_CacheConcurrency 测试缓存并发安全
func TestStateManager_CacheConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	sm := NewStateManager(1, 100, "video_1", tempDir, time.Now())

	var wg sync.WaitGroup
	numGoroutines := 100

	// 并发写入
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sm.SetCache("shared_key", n)
		}(i)
	}

	// 并发读取
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sm.GetCache("shared_key")
		}()
	}

	wg.Wait()

	// 验证最终值存在（具体值不确定，但不应该 panic）
	_, ok := sm.GetCache("shared_key")
	assert.True(t, ok)
}

// ============================================================================
// GetCurrentDateYYYYMMDD 测试
// ============================================================================

// TestGetCurrentDateYYYYMMDD 测试日期格式化
func TestGetCurrentDateYYYYMMDD(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "标准日期",
			input:    time.Date(2026, 1, 3, 12, 30, 0, 0, time.Local),
			expected: "2026-01-03",
		},
		{
			name:     "月份补零",
			input:    time.Date(2026, 5, 1, 0, 0, 0, 0, time.Local),
			expected: "2026-05-01",
		},
		{
			name:     "日期补零",
			input:    time.Date(2026, 12, 9, 0, 0, 0, 0, time.Local),
			expected: "2026-12-09",
		},
		{
			name:     "年初",
			input:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local),
			expected: "2026-01-01",
		},
		{
			name:     "年末",
			input:    time.Date(2026, 12, 31, 23, 59, 59, 0, time.Local),
			expected: "2026-12-31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCurrentDateYYYYMMDD(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// 边界条件测试
// ============================================================================

// TestNewStateManager_EmptyVideoID 测试空视频ID
func TestNewStateManager_EmptyVideoID(t *testing.T) {
	tempDir := t.TempDir()
	createTime := time.Now()

	// 即使视频ID为空，也应该能创建（虽然不推荐）
	sm := NewStateManager(1, 100, "", tempDir, createTime)

	assert.NotNil(t, sm)
	assert.Equal(t, "", sm.VideoID)
	// 路径会以 / 结尾，这是一个边界情况
	assert.DirExists(t, sm.CurrentDir)
}

// TestNewStateManager_SpecialCharactersInVideoID 测试视频ID中的特殊字符
func TestNewStateManager_SpecialCharactersInVideoID(t *testing.T) {
	tempDir := t.TempDir()
	createTime := time.Now()

	// YouTube 视频ID可能包含 - 和 _
	videoID := "abc-123_XYZ"
	sm := NewStateManager(1, 100, videoID, tempDir, createTime)

	assert.Equal(t, videoID, sm.VideoID)
	assert.Contains(t, sm.InputVideoPath, videoID+".mp4")
}

// TestNewStateManager_ZeroUserID 测试用户ID为0的情况
func TestNewStateManager_ZeroUserID(t *testing.T) {
	tempDir := t.TempDir()
	createTime := time.Now()

	sm := NewStateManager(1, 0, "video_1", tempDir, createTime)

	// 用户ID为0时，目录应该是 user_0
	assert.Contains(t, sm.CurrentDir, "user_0")
	assert.DirExists(t, sm.CurrentDir)
}

// TestNewStateManager_DirectoryAlreadyExists 测试目录已存在的情况
func TestNewStateManager_DirectoryAlreadyExists(t *testing.T) {
	tempDir := t.TempDir()
	createTime := time.Now()
	videoID := "existing_video"

	// 预先创建目录
	dateStr := GetCurrentDateYYYYMMDD(createTime)
	preExistingDir := filepath.Join(tempDir, "user_100", dateStr, videoID)
	err := os.MkdirAll(preExistingDir, 0755)
	require.NoError(t, err)

	// 在已存在的目录上创建 StateManager（不应报错）
	sm := NewStateManager(1, 100, videoID, tempDir, createTime)

	assert.Equal(t, preExistingDir, sm.CurrentDir)
	assert.DirExists(t, sm.CurrentDir)
}

// ============================================================================
// 性能基准测试
// ============================================================================

// BenchmarkNewStateManager 基准测试：创建 StateManager
func BenchmarkNewStateManager(b *testing.B) {
	tempDir := b.TempDir()
	createTime := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewStateManager(uint(i), 100, "video_bench", tempDir, createTime)
	}
}

// BenchmarkStateManager_SetCache 基准测试：缓存写入
func BenchmarkStateManager_SetCache(b *testing.B) {
	tempDir := b.TempDir()
	sm := NewStateManager(1, 100, "video_1", tempDir, time.Now())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.SetCache("key", i)
	}
}

// BenchmarkStateManager_GetCache 基准测试：缓存读取
func BenchmarkStateManager_GetCache(b *testing.B) {
	tempDir := b.TempDir()
	sm := NewStateManager(1, 100, "video_1", tempDir, time.Now())
	sm.SetCache("key", "value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.GetCache("key")
	}
}

// BenchmarkGetCurrentDateYYYYMMDD 基准测试：日期格式化
func BenchmarkGetCurrentDateYYYYMMDD(b *testing.B) {
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetCurrentDateYYYYMMDD(now)
	}
}
