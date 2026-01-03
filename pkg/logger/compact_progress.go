package logger

import (
	"fmt"
	"strings"
)

// CompactProgressLogger 紧凑型进度日志器，使用单行覆盖模式
type CompactProgressLogger struct {
	videoID    string
	helper     *ProgressHelper
	lastOutput string
}

// NewCompactProgressLogger 创建紧凑进度日志器
func NewCompactProgressLogger(videoID string) *CompactProgressLogger {
	return &CompactProgressLogger{
		videoID: videoID,
		helper:  NewProgressHelper(),
	}
}

// Start 开始进度显示
func (cp *CompactProgressLogger) Start() {
	cp.helper.UpdateProgress("📥 [%s] 初始化下载...", cp.videoID)
}

// UpdateProgress 更新下载进度（aria2c 格式）
func (cp *CompactProgressLogger) UpdateProgress(percent float64, downloaded, total, speed, eta string, connections string) {
	// 生成简洁的进度条
	progressBar := cp.formatProgressBar(percent, 20)

	// 构建紧凑的单行输出
	output := fmt.Sprintf("\r📥 [%s] [%s] %5.1f%% %s/%s %s/s ETA:%s",
		cp.videoID,
		progressBar,
		percent,
		downloaded,
		total,
		speed,
		eta,
	)

	cp.helper.UpdateMessage(output)
}

// UpdateProgressYTDLP 更新下载进度（yt-dlp 格式）
func (cp *CompactProgressLogger) UpdateProgressYTDLP(percent float64, total, speed, eta string) {
	// 生成简洁的进度条
	progressBar := cp.formatProgressBar(percent, 20)

	// 构建紧凑的单行输出
	// 注意：speed 参数已经包含 /s 后缀（如 "4.62MiB/s"）
	output := fmt.Sprintf("\r📥 [%s] [%s] %5.1f%% %s %s ETA:%s",
		cp.videoID,
		progressBar,
		percent,
		total,
		speed,
		eta,
	)

	cp.helper.UpdateMessage(output)
}

// LogMessage 记录普通日志消息（会清除进度行）
func (cp *CompactProgressLogger) LogMessage(message string) {
	cp.helper.manager.clearProgressLine()
	fmt.Println(message)
	// 恢复进度显示（如果需要）
	if cp.helper.manager.lastOutput.Load().(string) != "" {
		lastOutput := cp.helper.manager.lastOutput.Load().(string)
		cp.helper.UpdateMessage(lastOutput)
	}
}

// Complete 完成进度显示
func (cp *CompactProgressLogger) Complete(message string) {
	cp.helper.Close()
	if message != "" {
		fmt.Printf("\r✅ [%s] %s\n", cp.videoID, message)
	}
}

// Error 显示错误信息
func (cp *CompactProgressLogger) Error(message string) {
	cp.helper.Close()
	fmt.Printf("\r❌ [%s] %s\n", cp.videoID, message)
}

// formatProgressBar 生成进度条字符串
func (cp *CompactProgressLogger) formatProgressBar(percent float64, width int) string {
	if width <= 0 {
		width = 20
	}

	filled := int(percent / 100.0 * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return bar
}

// DownloadProgressInfo 下载进度信息结构
type DownloadProgressInfo struct {
	Percent     float64
	Downloaded  string
	Total       string
	Speed       string
	ETA         string
	Connections string
}

// Render 渲染进度条（用于调试）
func (cp *CompactProgressLogger) Render(info DownloadProgressInfo) string {
	bar := cp.formatProgressBar(info.Percent, 20)
	return fmt.Sprintf("[%s] %.1f%% %s/%s %s/s ETA:%s",
		bar, info.Percent, info.Downloaded, info.Total, info.Speed, info.ETA)
}
