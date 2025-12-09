package manager

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ProgressTracker 任务进度追踪器
type ProgressTracker struct {
	mu             sync.Mutex
	logger         *zap.SugaredLogger
	taskName       string
	videoID        string
	startTime      time.Time
	currentStep    string
	currentPct     int
	totalSteps     int
	completedSteps int
	lastUpdate     time.Time
	updateInterval time.Duration
}

// NewProgressTracker 创建进度追踪器
func NewProgressTracker(logger *zap.SugaredLogger, taskName, videoID string) *ProgressTracker {
	return &ProgressTracker{
		logger:         logger,
		taskName:       taskName,
		videoID:        videoID,
		startTime:      time.Now(),
		updateInterval: 5 * time.Second, // 最小更新间隔
	}
}

// SetTotalSteps 设置总步骤数
func (p *ProgressTracker) SetTotalSteps(total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.totalSteps = total
}

// StartStep 开始一个步骤
func (p *ProgressTracker) StartStep(stepName string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.currentStep = stepName
	p.logger.Infof("  ▶ %s", stepName)
}

// CompleteStep 完成一个步骤
func (p *ProgressTracker) CompleteStep(stepName string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completedSteps++
	elapsed := time.Since(p.startTime).Round(time.Second)
	if p.totalSteps > 0 {
		pct := (p.completedSteps * 100) / p.totalSteps
		p.logger.Infof("  ✓ %s (%d/%d, %d%%, 已用时 %v)", stepName, p.completedSteps, p.totalSteps, pct, elapsed)
	} else {
		p.logger.Infof("  ✓ %s (已用时 %v)", stepName, elapsed)
	}
}

// UpdateProgress 更新进度百分比（带节流，避免刷屏）
func (p *ProgressTracker) UpdateProgress(pct int, message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	// 只在进度变化且超过更新间隔时输出
	if pct != p.currentPct && (pct == 100 || now.Sub(p.lastUpdate) >= p.updateInterval) {
		p.currentPct = pct
		p.lastUpdate = now
		elapsed := time.Since(p.startTime).Round(time.Second)
		p.logger.Infof("  📊 [%s] %d%% - %s (已用时 %v)", p.taskName, pct, message, elapsed)
	}
}

// LogInfo 输出信息日志
func (p *ProgressTracker) LogInfo(format string, args ...interface{}) {
	p.logger.Infof("  │ "+format, args...)
}

// LogWarn 输出警告日志
func (p *ProgressTracker) LogWarn(format string, args ...interface{}) {
	p.logger.Warnf("  ⚠ "+format, args...)
}

// LogError 输出错误日志
func (p *ProgressTracker) LogError(format string, args ...interface{}) {
	p.logger.Errorf("  ✗ "+format, args...)
}

// Finish 完成任务
func (p *ProgressTracker) Finish(success bool, summary string) {
	elapsed := time.Since(p.startTime).Round(time.Millisecond)
	if success {
		p.logger.Infof("  ✅ %s 完成 - %s (总耗时 %v)", p.taskName, summary, elapsed)
	} else {
		p.logger.Errorf("  ❌ %s 失败 - %s (总耗时 %v)", p.taskName, summary, elapsed)
	}
}

// ProgressBar 生成进度条字符串
func ProgressBar(pct int, width int) string {
	if width <= 0 {
		width = 20
	}
	filled := (pct * width) / 100
	empty := width - filled

	bar := ""
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := 0; i < empty; i++ {
		bar += "░"
	}
	return fmt.Sprintf("[%s] %3d%%", bar, pct)
}

// FormatDuration 格式化时间
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1f秒", d.Seconds())
	} else if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		return fmt.Sprintf("%d分%d秒", mins, secs)
	} else {
		hours := int(d.Hours())
		mins := int(d.Minutes()) % 60
		return fmt.Sprintf("%d小时%d分", hours, mins)
	}
}

// FormatBytes 格式化字节大小
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
