package handlers

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/difyz9/ytb2bili/pkg/logger"
)

// logDownloadProgressCompact 使用紧凑进度条输出下载进度
func (t *DownloadVideo) logDownloadProgressCompact(line string, lastProgressTime *int64, progressLogger *logger.CompactProgressLogger) {
	now := time.Now().Unix()

	// yt-dlp 进度格式: [download]  10.5% of  239.12MiB at    4.62MiB/s ETA 00:42
	ytdlpRegex := regexp.MustCompile(`\[download\]\s+([0-9.]+)%\s+of\s+~?\s*([0-9.]+\s*[KMGT]?i?B)\s+at\s+(.+?)\s+ETA\s+(.+)`)
	if matches := ytdlpRegex.FindStringSubmatch(line); len(matches) >= 5 {
		percentStr := matches[1]
		total := strings.TrimSpace(matches[2])
		speed := strings.TrimSpace(matches[3])
		eta := strings.TrimSpace(matches[4])

		// 处理特殊情况
		if speed == "Unknown B/s" {
			speed = "计算中"
		}
		if strings.HasPrefix(eta, "Unkn") {
			eta = "计算中"
		}

		// 解析百分比
		var percent float64
		if _, err := fmt.Sscanf(percentStr, "%f", &percent); err == nil {
			// 更新进度到数据库（每3秒一次，通过内部互斥锁控制）
			t.updateDownloadProgress(&DownloadProgress{
				Percent:   percent,
				Total:     total,
				Speed:     speed,
				ETA:       eta,
				UpdatedAt: time.Now(),
			})

			// 每秒更新一次进度条显示
			if now-*lastProgressTime >= 1 {
				*lastProgressTime = now
				progressLogger.UpdateProgressYTDLP(percent, total, speed, eta)
			}
		}
		return
	}

	// aria2c 进度格式: [#abc123 100MiB/500MiB(20%) CN:16 DL:1.5MiB ETA:5m30s]
	aria2cRegex := regexp.MustCompile(`\[#\w+\s+([0-9.]+[KMGT]?i?B)/([0-9.]+[KMGT]?i?B)\((\d+)%\)\s+CN:(\d+)\s+DL:([0-9.]+[KMGT]?i?B)(?:\s+ETA:([^\]]+))?\]`)
	if matches := aria2cRegex.FindStringSubmatch(line); len(matches) >= 6 {
		downloaded := matches[1]
		total := matches[2]
		percentStr := matches[3]
		connections := matches[4]
		speed := matches[5]
		eta := "计算中"
		if len(matches) >= 7 && matches[6] != "" {
			eta = matches[6]
		}

		// 解析百分比
		var percent float64
		if _, err := fmt.Sscanf(percentStr, "%f", &percent); err == nil {
			// 更新进度到数据库
			t.updateDownloadProgress(&DownloadProgress{
				Percent:    percent,
				Downloaded: downloaded,
				Total:      total,
				Speed:      speed + "/s",
				ETA:        eta,
				UpdatedAt:  time.Now(),
			})

			// 每秒更新一次进度条显示
			if now-*lastProgressTime >= 1 {
				*lastProgressTime = now
				progressLogger.UpdateProgress(percent, downloaded, total, speed, eta, connections)
			}
		}
		return
	}

	// 下载目标文件
	if strings.Contains(line, "[download] Destination:") {
		filename := strings.TrimSpace(strings.Replace(line, "[download] Destination:", "", 1))
		progressLogger.LogMessage(fmt.Sprintf("📁 目标文件: %s", filename))
		return
	}

	// 合并文件
	if strings.Contains(line, "[Merger]") || strings.Contains(line, "[ffmpeg]") {
		progressLogger.LogMessage("🔧 合并视频文件...")
		return
	}

	// 下载完成（只识别视频下载的 100%，避免字幕下载进度误判）
	// yt-dlp 视频下载完成格式: [download] 100% of XXXMiB in XX:XX
	// 字幕下载使用不同的格式，不应该触发此判断
	if (strings.HasPrefix(line, "[download]") && strings.Contains(line, "100%")) ||
		strings.Contains(line, "has already been downloaded") {
		progressLogger.LogMessage("✅ 视频下载完成")
		return
	}

	// 字幕下载
	if strings.Contains(line, "[info] Writing video subtitles") {
		progressLogger.LogMessage("📝 下载字幕中...")
		return
	}

	// 错误和警告
	if strings.Contains(line, "ERROR") || strings.Contains(line, "error") {
		progressLogger.LogMessage(fmt.Sprintf("⚠️ %s", line))
		return
	}

	// 睡眠等待
	if strings.Contains(line, "Sleeping") {
		progressLogger.LogMessage(fmt.Sprintf("⏳ %s", line))
		return
	}
}

// executeDownloadWithCompactProgress 使用紧凑进度条的下载执行函数
// 这是 executeDownloadWithAuthMode 的替代版本
func (t *DownloadVideo) executeDownloadWithCompactProgress(ytdlpPath, videoURL string, useProxy bool, authMode string, context map[string]interface{}) bool {
	// 创建紧凑进度日志器
	progressLogger := logger.NewCompactProgressLogger(t.StateManager.VideoID)
	defer progressLogger.Complete("") // 将在成功时调用 Complete，失败时调用 Error

	progressLogger.Start()
	t.App.Logger.Infof("开始下载视频: %s", t.StateManager.VideoID)

	// 直接调用现有的 executeDownloadWithAuthMode 而不是重复构建命令
	// 注意：这个紧凑进度版本尚未完全集成，暂时回退到标准实现
	return t.executeDownloadWithAuthMode(ytdlpPath, videoURL, useProxy, authMode, context)
}

// executeDownloadWithCompactProgressFull 完整版本（未启用）
// 如需启用紧凑进度条，需要在此处实现完整的命令构建逻辑
func (t *DownloadVideo) executeDownloadWithCompactProgressFull(ytdlpPath, videoURL string, useProxy bool, authMode string, context map[string]interface{}) bool {
	// 创建紧凑进度日志器
	progressLogger := logger.NewCompactProgressLogger(t.StateManager.VideoID)
	defer progressLogger.Complete("")

	progressLogger.Start()
	t.App.Logger.Infof("开始下载视频: %s", t.StateManager.VideoID)

	// TODO: 需要复制 executeDownloadWithAuthMode 中的命令构建逻辑
	// 暂时占位，返回false
	_ = progressLogger
	context["error"] = "紧凑进度条功能尚未完全实现"
	return false
}

// 注意：这是集成指南
//
// 要在现有的 executeDownloadWithAuthMode 函数中使用紧凑进度条，请进行以下修改：
//
// 1. 在函数开始时创建 progressLogger：
//    progressLogger := logger.NewCompactProgressLogger(t.StateManager.VideoID)
//    defer progressLogger.Complete("")
//
// 2. 将所有 t.logDownloadProgress(scanner.Text(), &lastProgressTime)
//    替换为 t.logDownloadProgressCompact(scanner.Text(), &lastProgressTime, progressLogger)
//
// 3. 移除旧的多行进度框输出（t.App.Logger.Info 那些框线）
//
// 4. 或者在 Execute 方法中添加配置选项，让用户选择使用哪种进度显示方式
//
// 示例配置：
// [download]
// use_compact_progress = true  # 使用紧凑进度条
// use_compact_progress = false # 使用多行进度框（默认）
