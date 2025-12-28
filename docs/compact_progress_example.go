//go:build ignore

// 示例：如何修改下载视频处理器以使用紧凑进度条
// 文件: internal/chain_task/handlers/down_load_video.go

package handlers

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"your-project/pkg/logger" // 导入新的日志包
)

// executeDownloadWithCompactProgress 使用紧凑进度条的下载实现
func (t *DownloadVideo) executeDownloadWithCompactProgress(ytdlpPath, videoURL string, useProxy bool, context map[string]interface{}) bool {
	// 创建紧凑进度日志器
	progressLogger := logger.NewCompactProgressLogger(t.StateManager.VideoID)
	defer progressLogger.Complete("") // 将在成功时调用 Complete，失败时调用 Error

	progressLogger.Start()
	t.App.Logger.Infof("开始下载视频: %s", t.StateManager.VideoID)

	// 构建下载命令
	cmdArgs := []string{
		ytdlpPath,
		"-P", t.StateManager.CurrentDir,
		"-o", "%(id)s.%(ext)s",
		// ... 其他参数 ...
		videoURL,
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		progressLogger.Error(fmt.Sprintf("创建输出管道失败: %v", err))
		context["error"] = err.Error()
		return false
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		progressLogger.Error(fmt.Sprintf("创建错误管道失败: %v", err))
		context["error"] = err.Error()
		return false
	}

	// 用于跟踪上次的进度时间
	var lastProgressTime int64
	var lastPercent float64

	// 启动命令
	if err := cmd.Start(); err != nil {
		progressLogger.Error(fmt.Sprintf("启动下载命令失败: %v", err))
		context["error"] = err.Error()
		return false
	}

	// 处理标准输出
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			t.processDownloadLine(line, progressLogger, &lastProgressTime, &lastPercent)
		}
	}()

	// 处理标准错误
	go func() {
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			t.processDownloadLine(line, progressLogger, &lastProgressTime, &lastPercent)
		}
	}()

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		progressLogger.Error(fmt.Sprintf("下载失败: %v", err))
		context["error"] = err.Error()
		return false
	}

	// 验证下载的文件
	downloadedFile := t.findDownloadedFile()
	if downloadedFile == "" {
		progressLogger.Error("下载完成但未找到视频文件")
		context["error"] = "下载完成但未找到视频文件"
		return false
	}

	context["downloaded_file"] = downloadedFile
	progressLogger.Complete("下载成功")
	return true
}

// processDownloadLine 处理下载输出行
func (t *DownloadVideo) processDownloadLine(
	line string,
	progressLogger *logger.CompactProgressLogger,
	lastProgressTime *int64,
	lastPercent *float64,
) {
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
			// 更新到数据库（每3秒）
			if now-*lastProgressTime >= 3 && percent != *lastPercent {
				*lastProgressTime = now
				*lastPercent = percent

				t.updateDownloadProgress(&DownloadProgress{
					Percent:   percent,
					Total:     total,
					Speed:     speed,
					ETA:       eta,
					UpdatedAt: time.Now(),
				})
			}

			// 每秒更新一次进度条显示（或每 5% 变化）
			if now-*lastProgressTime >= 1 || percent-*lastPercent >= 5.0 {
				progressLogger.UpdateProgressYTDLP(percent, total, speed, eta)
				*lastProgressTime = now
				*lastPercent = percent
			}
		}
		return
	}

	// 下载目标文件
	if strings.Contains(line, "[download] Destination:") {
		filename := strings.TrimSpace(strings.Replace(line, "[download] Destination:", "", 1))
		t.App.Logger.Infof("📁 目标文件: %s", filename)
		return
	}

	// 合并文件
	if strings.Contains(line, "[Merger]") || strings.Contains(line, "[ffmpeg]") {
		progressLogger.LogMessage("🔧 合并视频文件...")
		return
	}

	// 下载完成
	if strings.Contains(line, "100%") || strings.Contains(line, "has already been downloaded") {
		progressLogger.LogMessage("✅ 视频下载完成")
		return
	}

	// 字幕下载
	if strings.Contains(line, "[info] Writing video subtitles") {
		progressLogger.LogMessage("📝 下载字幕中...")
		return
	}
}

// ============================================================================
// 在现有代码中的集成点说明：
// ============================================================================

/*
集成步骤:

1. 在文件开头添加导入:
   import "your-project/pkg/logger"

2. 在 DownloadVideo 结构体中可以添加可选字段:
   type DownloadVideo struct {
       // ... 现有字段 ...
       useCompactProgress bool // 是否使用紧凑进度条
   }

3. 在 Execute 方法中根据配置选择进度显示方式:
   func (t *DownloadVideo) Execute(context map[string]interface{}) bool {
       if t.useCompactProgress {
           return t.executeDownloadWithCompactProgress(...)
       } else {
           return t.executeDownload(...) // 原有的实现
       }
   }

4. 或者在配置文件中添加选项:
   [download]
   use_compact_progress = true

5. 渐进式迁移策略:
   - 第一阶段: 同时支持两种模式，通过配置切换
   - 第二阶段: 默认使用紧凑模式，保留旧模式作为 fallback
   - 第三阶段: 完全切换到紧凑模式，移除旧代码

性能对比:
   旧模式 (多行框):
   - 每次更新输出 8 行
   - 8 次 Logger.Info() 调用
   - 终端滚动频繁

   新模式 (单行紧凑):
   - 每次更新输出 1 行
   - 1 次 fmt.Printf() 调用
   - 使用 ANSI \r 覆盖，无滚动
   - 减少 87.5% 的输出量

兼容性:
   - 需要 ANSI 转义码支持
   - Windows 10+ CMD/PowerShell: ✅ 支持
   - Windows Terminal: ✅ 支持
   - Linux/macOS Terminal: ✅ 支持
   - CI/CD 环境: ⚠️ 可能需要禁用 (--no-interactive)
*/
