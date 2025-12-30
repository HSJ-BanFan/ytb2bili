package chain_task

import (
	"context"
	"fmt"
	"sync"
)

// TaskCancelManager 管理任务取消功能
// 使用 savedVideo.ID (数据库主键) 作为 key，确保多用户场景下的唯一性
type TaskCancelManager struct {
	mu      sync.RWMutex
	cancels map[string]context.CancelFunc
}

// NewTaskCancelManager 创建新的任务取消管理器
func NewTaskCancelManager() *TaskCancelManager {
	return &TaskCancelManager{
		cancels: make(map[string]context.CancelFunc),
	}
}

// Register 注册一个新的任务，返回可取消的 context
// id: cw_saved_videos 表的主键 ID
// 返回的 context 应该传递给任务链使用
func (m *TaskCancelManager) Register(id uint) context.Context {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%d", id)
	ctx, cancel := context.WithCancel(context.Background())
	m.cancels[key] = cancel

	return ctx
}

// Cancel 取消指定 ID 的任务
// id: cw_saved_videos 表的主键 ID
func (m *TaskCancelManager) Cancel(id uint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%d", id)
	if cancel, exists := m.cancels[key]; exists {
		cancel() // 发送取消信号
		// 注意：不在这里删除，让 Deregister 来清理
	}
}

// Deregister 注销任务，释放资源
// id: cw_saved_videos 表的主键 ID
func (m *TaskCancelManager) Deregister(id uint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%d", id)
	if cancel, exists := m.cancels[key]; exists {
		cancel() // 确保 context 被取消，释放资源
		delete(m.cancels, key)
	}
}

// IsCanceled 检查指定 ID 的任务是否已被取消
// 注意：这个方法主要用于非 context 的检查场景
// 在任务内部应该通过 ctx.Done() 来检查
func (m *TaskCancelManager) IsCanceled(id uint) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%d", id)
	_, exists := m.cancels[key]
	return !exists // 如果不在 map 中，说明已注销或已取消
}

// GetCancelFunc 获取指定 ID 的取消函数（主要用于测试）
func (m *TaskCancelManager) GetCancelFunc(id uint) (context.CancelFunc, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := fmt.Sprintf("%d", id)
	cancel, exists := m.cancels[key]
	return cancel, exists
}

// Count 返回当前注册的任务数量（主要用于监控和测试）
func (m *TaskCancelManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.cancels)
}

// ClearCancel 清除指定 ID 的取消状态（用于任务重试时重置）
// 这将删除旧的 cancel 函数，允许任务被重新注册
func (m *TaskCancelManager) ClearCancel(id uint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%d", id)
	delete(m.cancels, key)
}
