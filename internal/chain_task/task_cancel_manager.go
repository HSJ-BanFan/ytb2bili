package chain_task

import (
	"context"
	"fmt"
	"sync"
)

// TaskCancelManager 管理任务取消功能
// 重构版：支持同一 ID 下的多个并发任务（runID）
type TaskCancelManager struct {
	mu           sync.RWMutex
	activeTasks  map[string]map[string]context.CancelFunc // ID -> RunID -> CancelFunc
	cancelStates map[string]bool                          // ID -> IsCanceled (持久化取消状态)
}

// NewTaskCancelManager 创建新的任务取消管理器
func NewTaskCancelManager() *TaskCancelManager {
	return &TaskCancelManager{
		activeTasks:  make(map[string]map[string]context.CancelFunc),
		cancelStates: make(map[string]bool),
	}
}

// Register 注册一个新的任务，返回可取消的 context
// id: cw_saved_videos 表的主键 ID
// runID: 任务执行的唯一标识（例如 "step_下载视频" 或 "chain_full"）
func (m *TaskCancelManager) Register(id uint, runID string) (context.Context, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%d", id)

	// 1. 检查是否已被标记为取消
	if m.cancelStates[key] {
		// 如果已取消，返回一个已取消的 context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // 立即取消
		return ctx, false
	}

	// 2. 初始化内部 map
	if m.activeTasks[key] == nil {
		m.activeTasks[key] = make(map[string]context.CancelFunc)
	}

	// 3. 创建并存储新的 cancel func
	// 注意：如果 runID 已存在，旧的会被覆盖（通常不应发生，除非重复调用）
	ctx, cancel := context.WithCancel(context.Background())
	m.activeTasks[key][runID] = cancel

	return ctx, true
}

// Cancel 取消指定 ID 的所有任务，并标记为已取消状态
func (m *TaskCancelManager) Cancel(id uint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%d", id)

	// 1. 设置取消状态
	m.cancelStates[key] = true

	// 2. 取消所有活动的任务
	if tasks, exists := m.activeTasks[key]; exists {
		for _, cancel := range tasks {
			cancel()
		}
		// 也可以选择在这里清空 activeTasks，但为了安全起见保留直到 Deregister
	}
}

// Deregister 注销特定任务（任务完成时调用）
func (m *TaskCancelManager) Deregister(id uint, runID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%d", id)
	if tasks, exists := m.activeTasks[key]; exists {
		if cancel, ok := tasks[runID]; ok {
			cancel() // 确保释放资源
			delete(tasks, runID)
		}
		// 如果该 ID 下没有活动任务了，可以清理 map 入口，但保留 cancelStates
		if len(tasks) == 0 {
			delete(m.activeTasks, key)
		}
	}
}

// IsCanceled 检查指定 ID 是否处于取消状态
func (m *TaskCancelManager) IsCanceled(id uint) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%d", id)
	return m.cancelStates[key]
}

// ClearCancel 清除取消状态（允许新任务重新开始）
func (m *TaskCancelManager) ClearCancel(id uint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%d", id)
	delete(m.cancelStates, key)
	// 注意：不清除 activeTasks，因为可能有任务还在运行（虽然状态已重置）
}

// CountActive 返回指定 ID 的活动任务数
func (m *TaskCancelManager) CountActive(id uint) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%d", id)
	if tasks, exists := m.activeTasks[key]; exists {
		return len(tasks)
	}
	return 0
}
