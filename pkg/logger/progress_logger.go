package logger

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// ProgressLogManager 管理进度日志和普通日志的隔离
type ProgressLogManager struct {
	logger     *zap.SugaredLogger
	active     atomic.Int32 // 当前活动的进度会话数量
	lastOutput atomic.Value // 保存上次的进度输出，用于覆盖
}

// Global progress manager instance
var globalProgressManager *ProgressLogManager

// InitProgressLogManager 初始化全局进度日志管理器
func InitProgressLogManager(logger *zap.SugaredLogger) {
	if globalProgressManager == nil {
		globalProgressManager = &ProgressLogManager{
			logger: logger,
		}
		globalProgressManager.lastOutput.Store("")
	}
}

// GetProgressManager 获取全局进度日志管理器
func GetProgressManager() *ProgressLogManager {
	if globalProgressManager == nil {
		// 如果未初始化，使用默认 logger
		logger, _ := NewLogger(false)
		InitProgressLogManager(logger)
	}
	return globalProgressManager
}

// StartProgress 开始一个进度会话
// 返回一个 cleanup 函数，调用时结束进度会话
func (p *ProgressLogManager) StartProgress() func() {
	p.active.Add(1)
	return func() {
		p.active.Add(-1)
		// 清除进度行
		p.clearProgressLine()
	}
}

// IsActive 检查当前是否有活动的进度会话
func (p *ProgressLogManager) IsActive() bool {
	return p.active.Load() > 0
}

// clearProgressLine 清除当前的进度行
func (p *ProgressLogManager) clearProgressLine() {
	last := p.lastOutput.Load().(string)
	if last != "" {
		// 计算需要清除的行数
		lines := strings.Count(last, "\n") + 1
		// 使用 ANSI 转义码清除行
		for i := 0; i < lines; i++ {
			fmt.Print("\033[F\033[K") // 移动到上一行并清除
		}
		p.lastOutput.Store("")
	}
}

// LogWithProgressCheck 智能日志输出
// 如果有活动进度，会特殊处理（缓冲、延迟或简化）
func (p *ProgressLogManager) LogWithProgressCheck(level string, args ...interface{}) {
	if p.IsActive() {
		// 进度活动时，可以选择以下策略之一：

		// 策略1: 直接输出，但在下一行（不覆盖进度）
		// 适用于错误和重要信息
		if level == "error" || level == "warn" {
			p.clearProgressLine()
			p.log(level, args...)
			time.Sleep(50 * time.Millisecond) // 给用户一点时间看到信息
			return
		}

		// 策略2: 完全跳过（debug 级别的日志）
		if level == "debug" {
			return
		}

		// 策略3: 对于 info 级别，简化输出到单行
		if level == "info" {
			// 可选：过滤掉一些不重要的日志
			msg := fmt.Sprint(args...)
			if strings.Contains(msg, "没有待处理") ||
				strings.Contains(msg, "没有待上传") ||
				strings.Contains(msg, "未找到") {
				return // 跳过这些噪音日志
			}
			// 其他 info 日志仍然输出
			p.log(level, args...)
		}
	} else {
		// 没有进度活动时，正常输出
		p.log(level, args...)
	}
}

// LogfWithProgressCheck 格式化日志输出（带进度检查）
func (p *ProgressLogManager) LogfWithProgressCheck(level string, template string, args ...interface{}) {
	if p.IsActive() {
		if level == "error" || level == "warn" {
			p.clearProgressLine()
			p.logf(level, template, args...)
			time.Sleep(50 * time.Millisecond)
			return
		}

		if level == "debug" {
			return
		}

		if level == "info" {
			// 检查是否是需要过滤的日志
			if strings.Contains(template, "没有待处理") ||
				strings.Contains(template, "没有待上传") ||
				strings.Contains(template, "未找到") ||
				strings.Contains(template, "发现 %d 个待重试") {
				return // 跳过定时任务的噪音日志
			}
			p.logf(level, template, args...)
		}
	} else {
		p.logf(level, template, args...)
	}
}

// 内部日志方法
func (p *ProgressLogManager) log(level string, args ...interface{}) {
	switch level {
	case "debug":
		p.logger.Debug(args...)
	case "info":
		p.logger.Info(args...)
	case "warn":
		p.logger.Warn(args...)
	case "error":
		p.logger.Error(args...)
	}
}

func (p *ProgressLogManager) logf(level string, template string, args ...interface{}) {
	switch level {
	case "debug":
		p.logger.Debugf(template, args...)
	case "info":
		p.logger.Infof(template, args...)
	case "warn":
		p.logger.Warnf(template, args...)
	case "error":
		p.logger.Errorf(template, args...)
	}
}

// ProgressHelper 进度显示辅助函数
type ProgressHelper struct {
	manager *ProgressLogManager
	cleanup func()
}

// NewProgressHelper 创建一个新的进度辅助器
func NewProgressHelper() *ProgressHelper {
	mgr := GetProgressManager()
	cleanup := mgr.StartProgress()
	return &ProgressHelper{
		manager: mgr,
		cleanup: cleanup,
	}
}

// UpdateProgress 更新进度（单行覆盖模式）
func (ph *ProgressHelper) UpdateProgress(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	// 使用 ANSI 转义码实现单行更新
	fmt.Printf("\r\033[K%s", msg) // \r 回到行首, \033[K 清除到行尾
	ph.manager.lastOutput.Store(msg)
}

// Close 结束进度显示
func (ph *ProgressHelper) Close() {
	if ph.cleanup != nil {
		// 清除进度行
		ph.manager.clearProgressLine()
		ph.cleanup()
		ph.cleanup = nil
	}
}
