package chain_task

import (
	"context"
	"fmt"
	"sync"

	"github.com/difyz9/ytb2bili/internal/membership"
	"go.uber.org/zap"
)

// ConcurrencyLimiter 并发控制器（per-user）
type ConcurrencyLimiter struct {
	// userConcurrency[userID] = current running count
	userConcurrency map[uint]int
	globalMutex     sync.RWMutex

	permissionService *membership.PermissionService
	logger            *zap.SugaredLogger
}

// NewConcurrencyLimiter 创建并发控制器
func NewConcurrencyLimiter(
	permService *membership.PermissionService,
	logger *zap.SugaredLogger,
) *ConcurrencyLimiter {
	return &ConcurrencyLimiter{
		userConcurrency:   make(map[uint]int),
		permissionService: permService,
		logger:            logger,
	}
}

// TryAcquire 尝试获取执行槽位
// 返回 (是否获取成功, 当前并发数, 最大并发数, 错误)
func (c *ConcurrencyLimiter) TryAcquire(ctx context.Context, userID uint) (bool, int, int, error) {
	// 1. 获取用户最大并发数
	userIDStr := fmt.Sprintf("%d", userID)
	maxConcurrent, err := c.permissionService.GetMaxConcurrentTasks(ctx, userIDStr)
	if err != nil {
		// 权限获取失败，默认限制为 1
		maxConcurrent = 1
		c.logger.Warnf("获取用户 %d 并发限制失败: %v，使用默认值 1", userID, err)
	}

	// -1 表示无限制
	if maxConcurrent == -1 {
		c.globalMutex.Lock()
		c.userConcurrency[userID]++
		current := c.userConcurrency[userID]
		c.globalMutex.Unlock()
		return true, current, -1, nil
	}

	c.globalMutex.Lock()
	defer c.globalMutex.Unlock()

	// 2. 检查当前并发数
	current := c.userConcurrency[userID]
	if current >= maxConcurrent {
		c.logger.Debugf("⏳ 用户 %d 并发任务已达上限 (%d/%d)", userID, current, maxConcurrent)
		return false, current, maxConcurrent, nil
	}

	// 3. 增加并发数
	c.userConcurrency[userID]++
	c.logger.Infof("✅ 用户 %d 获取执行槽位 (%d/%d)", userID, c.userConcurrency[userID], maxConcurrent)

	return true, c.userConcurrency[userID], maxConcurrent, nil
}

// Release 释放执行槽位
func (c *ConcurrencyLimiter) Release(userID uint) {
	c.globalMutex.Lock()
	defer c.globalMutex.Unlock()

	if current, exists := c.userConcurrency[userID]; exists {
		if current > 0 {
			c.userConcurrency[userID]--
			c.logger.Debugf("🔓 用户 %d 释放执行槽位 (%d remaining)", userID, c.userConcurrency[userID])
		}
	}
}

// GetStats 获取用户并发统计
func (c *ConcurrencyLimiter) GetStats(userID uint) (current, max int) {
	c.globalMutex.RLock()
	current = c.userConcurrency[userID]
	c.globalMutex.RUnlock()

	ctx := context.Background()
	userIDStr := fmt.Sprintf("%d", userID)
	max, _ = c.permissionService.GetMaxConcurrentTasks(ctx, userIDStr)

	return
}

// GetAllStats 获取所有用户的并发统计
func (c *ConcurrencyLimiter) GetAllStats() map[uint]int {
	c.globalMutex.RLock()
	defer c.globalMutex.RUnlock()

	stats := make(map[uint]int)
	for k, v := range c.userConcurrency {
		stats[k] = v
	}
	return stats
}

// Reset 重置所有用户的并发计数（服务器启动时调用）
func (c *ConcurrencyLimiter) Reset() {
	c.globalMutex.Lock()
	defer c.globalMutex.Unlock()

	if len(c.userConcurrency) > 0 {
		c.logger.Info("🔄 重置所有用户的并发槽位")
		c.userConcurrency = make(map[uint]int)
	}
}

// ForceRelease 强制释放指定用户的所有槽位（用于修复槽位泄漏）
func (c *ConcurrencyLimiter) ForceRelease(userID uint) {
	c.globalMutex.Lock()
	defer c.globalMutex.Unlock()

	if current, exists := c.userConcurrency[userID]; exists && current > 0 {
		c.logger.Warnf("🔓 强制释放用户 %d 的 %d 个槽位", userID, current)
		c.userConcurrency[userID] = 0
	}
}
