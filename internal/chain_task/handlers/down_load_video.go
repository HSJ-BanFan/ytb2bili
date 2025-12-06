package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/difyz9/ytb2bili/internal/chain_task/base"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/pkg/cos"
	"github.com/difyz9/ytb2bili/pkg/utils"
	"gorm.io/gorm"
)

type DownloadVideo struct {
	base.BaseTask
	App               *core.AppServer
	DB                *gorm.DB
	SavedVideoService *services.SavedVideoService
}

func NewDownloadVideo(name string, app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient, savedVideoService *services.SavedVideoService) *DownloadVideo {
	return &DownloadVideo{
		BaseTask: base.BaseTask{
			Name:         name,
			StateManager: stateManager,
			Client:       client,
		},
		App:               app,
		SavedVideoService: savedVideoService,
	}
}

// findYtDlp 查找系统中的 yt-dlp 可执行文件
func (t *DownloadVideo) findYtDlp() (string, error) {
	// 从配置中获取安装目录
	var installDir string
	if t.App.Config != nil && t.App.Config.YtDlpPath != "" {
		installDir = t.App.Config.YtDlpPath
	}

	// 创建 yt-dlp 管理器
	manager := utils.NewYtDlpManager(t.App.Logger, installDir)

	// 检查是否已安装
	if manager.IsInstalled() {
		path := manager.GetBinaryPath()
		t.App.Logger.Debugf("找到 yt-dlp: %s", path)
		return path, nil
	}

	return "", fmt.Errorf("未找到 yt-dlp，请确保已正确安装")
}

// getVideoURL 根据 VideoID 构建完整的视频 URL
func (t *DownloadVideo) getVideoURL() string {
	videoID := t.StateManager.VideoID

	// 如果已经是完整 URL，直接返回
	if strings.HasPrefix(videoID, "http://") || strings.HasPrefix(videoID, "https://") {
		return videoID
	}

	// YouTube 短 ID 格式
	if len(videoID) == 11 && !strings.Contains(videoID, "/") {
		return fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	}

	// Bilibili BV 号
	if strings.HasPrefix(videoID, "BV") {
		return fmt.Sprintf("https://www.bilibili.com/video/%s", videoID)
	}

	// 默认作为 YouTube ID 处理
	return fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
}

func (t *DownloadVideo) Execute(context map[string]interface{}) bool {
	t.App.Logger.Info("========================================")
	t.App.Logger.Info("DownloadVideo Handler Version: with-cookies-support-v3") // 版本标记
	t.App.Logger.Infof("开始下载视频: %s", t.StateManager.VideoID)
	t.App.Logger.Info("========================================")

	// 1. 查找 yt-dlp 可执行文件
	ytdlpPath, err := t.findYtDlp()
	if err != nil {
		t.App.Logger.Errorf("❌ %v", err)
		context["error"] = err.Error()
		return false
	}

	// 2. 确保下载目录存在
	if err := os.MkdirAll(t.StateManager.CurrentDir, 0755); err != nil {
		t.App.Logger.Errorf("❌ 创建下载目录失败: %v", err)
		context["error"] = err.Error()
		return false
	}

	// 3. 尝试下载（先用代理，失败后不用代理重试）
	videoURL := t.getVideoURL()
	useProxy := t.App.Config != nil && t.App.Config.ProxyConfig != nil &&
		t.App.Config.ProxyConfig.UseProxy && t.App.Config.ProxyConfig.ProxyHost != ""

	// 第一次尝试：使用代理（如果配置了）
	if useProxy {
		t.App.Logger.Info("🔄 尝试使用代理下载...")
		if t.executeDownload(ytdlpPath, videoURL, true, context) {
			return true
		}
		t.App.Logger.Warn("⚠️ 代理下载失败，尝试不使用代理重试...")
	}

	// 第二次尝试：不使用代理
	t.App.Logger.Info("🔄 尝试不使用代理下载...")
	return t.executeDownload(ytdlpPath, videoURL, false, context)
}

// findAria2c 查找系统中的 aria2c 可执行文件
func (t *DownloadVideo) findAria2c() string {
	// 首先检查配置中是否指定了路径
	if t.App.Config != nil && t.App.Config.DownloadConfig != nil &&
		t.App.Config.DownloadConfig.Aria2cPath != "" {
		if _, err := os.Stat(t.App.Config.DownloadConfig.Aria2cPath); err == nil {
			return t.App.Config.DownloadConfig.Aria2cPath
		}
	}

	// 尝试从 PATH 查找
	if path, err := exec.LookPath("aria2c"); err == nil {
		return path
	}
	// Windows 常见安装位置
	windowsPaths := []string{
		"C:\\Program Files\\aria2\\aria2c.exe",
		"C:\\aria2\\aria2c.exe",
	}
	for _, p := range windowsPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// getDownloadConfig 获取下载配置，使用默认值填充未设置的字段
func (t *DownloadVideo) getDownloadConfig() (concurrentFragments int, aria2cConnections int, httpChunkSize string) {
	concurrentFragments = 8
	aria2cConnections = 16
	httpChunkSize = "10M"

	if t.App.Config != nil && t.App.Config.DownloadConfig != nil {
		cfg := t.App.Config.DownloadConfig
		if cfg.ConcurrentFragments > 0 {
			concurrentFragments = cfg.ConcurrentFragments
		}
		if cfg.Aria2cConnections > 0 {
			aria2cConnections = cfg.Aria2cConnections
		}
		if cfg.HttpChunkSize != "" {
			httpChunkSize = cfg.HttpChunkSize
		}
	}
	return
}

// executeDownload 执行实际的下载操作
func (t *DownloadVideo) executeDownload(ytdlpPath, videoURL string, useProxy bool, context map[string]interface{}) bool {
	// 构建下载命令
	command := []string{
		ytdlpPath,
		"-P", t.StateManager.CurrentDir,
		"-o", "%(id)s.%(ext)s",
		"--merge-output-format", "mp4",
	}

	// 获取下载配置
	concurrentFragments, aria2cConnections, httpChunkSize := t.getDownloadConfig()

	// 检查是否启用 aria2c（默认启用）
	useAria2c := true
	if t.App.Config != nil && t.App.Config.DownloadConfig != nil {
		useAria2c = t.App.Config.DownloadConfig.UseAria2c
	}

	// 获取代理配置
	proxyHost := ""
	if useProxy && t.App.Config != nil && t.App.Config.ProxyConfig != nil &&
		t.App.Config.ProxyConfig.UseProxy && t.App.Config.ProxyConfig.ProxyHost != "" {
		proxyHost = t.App.Config.ProxyConfig.ProxyHost
	}

	// 检查是否有 aria2c 可用，用于多线程下载加速
	// 注意：当使用代理时，aria2c 可能会遇到 403 错误，因此使用代理时优先使用 yt-dlp 内置下载器
	aria2cPath := t.findAria2c()
	if useAria2c && aria2cPath != "" && proxyHost == "" {
		// 不使用代理时，aria2c 可以正常工作
		t.App.Logger.Infof("🚀 检测到 aria2c，启用多线程下载加速 (连接数: %d)", aria2cConnections)
		aria2cArgs := fmt.Sprintf("aria2c:-x %d -s %d -k 1M --file-allocation=none --async-dns=false --check-certificate=false", aria2cConnections, aria2cConnections)
		command = append(command,
			"--downloader", "aria2c",
			"--downloader-args", aria2cArgs,
		)
	} else {
		if proxyHost != "" {
			t.App.Logger.Info("ℹ️ 使用代理时，采用 yt-dlp 内置并发下载器（避免 aria2c 403 错误）")
		} else if !useAria2c {
			t.App.Logger.Info("ℹ️ aria2c 已在配置中禁用，使用 yt-dlp 内置下载器")
		} else {
			t.App.Logger.Warn("⚠️ 未检测到 aria2c，使用 yt-dlp 内置下载器")
			t.App.Logger.Info("💡 建议安装 aria2c 以启用多线程下载加速: https://aria2.github.io/")
		}
		// 使用 yt-dlp 内置的并发分片下载
		t.App.Logger.Infof("📊 使用并发分片数: %d, HTTP分块大小: %s", concurrentFragments, httpChunkSize)
		command = append(command,
			"--concurrent-fragments", fmt.Sprintf("%d", concurrentFragments),
			"--buffer-size", "16K",
			"--http-chunk-size", httpChunkSize,
		)
	}

	// 检查是否存在 cookies.txt
	configDir := filepath.Dir(t.App.Config.Path)
	cookiesPath := filepath.Join(configDir, "cookies.txt")

	// 如果配置文件目录下的 cookies.txt 不存在，尝试当前目录
	if _, err := os.Stat(cookiesPath); err != nil {
		cookiesPath = "cookies.txt"
	}

	if _, err := os.Stat(cookiesPath); err == nil {
		absPath, _ := filepath.Abs(cookiesPath)
		command = append(command, "--cookies", absPath)
		t.App.Logger.Infof("🍪 使用 Cookies 文件: %s", absPath)
	} else {
		// 如果没有 cookies 文件，尝试从浏览器读取（Chrome 优先）
		t.App.Logger.Info("🍪 未找到 cookies 文件，尝试从浏览器读取...")
		command = append(command, "--cookies-from-browser", "chrome")
		t.App.Logger.Info("🍪 将从 Chrome 浏览器读取 cookies")
		t.App.Logger.Warn("⚠️ 未找到 cookies.txt，可能会遇到 'Sign in to confirm you're not a bot' 错误")
	}

	// 添加代理配置（如果需要）
	if useProxy && t.App.Config != nil && t.App.Config.ProxyConfig != nil &&
		t.App.Config.ProxyConfig.UseProxy && t.App.Config.ProxyConfig.ProxyHost != "" {
		command = append(command, "--proxy", t.App.Config.ProxyConfig.ProxyHost)
		t.App.Logger.Infof("📡 使用代理: %s", t.App.Config.ProxyConfig.ProxyHost)
	} else if !useProxy {
		t.App.Logger.Info("🌐 不使用代理")
	}

	// 添加视频标识符和URL
	command = append(command, "--", t.StateManager.VideoID)
	command = append(command, videoURL)

	t.App.Logger.Infof("执行命令: %s", strings.Join(command, " "))
	t.App.Logger.Infof("下载目录: %s", t.StateManager.CurrentDir)
	t.App.Logger.Infof("视频URL: %s", videoURL)

	// 创建命令并设置输出管道
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = t.StateManager.CurrentDir

	// 捕获标准输出和标准错误
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.App.Logger.Errorf("❌ 创建标准输出管道失败: %v", err)
		context["error"] = err.Error()
		return false
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.App.Logger.Errorf("❌ 创建标准错误管道失败: %v", err)
		context["error"] = err.Error()
		return false
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		t.App.Logger.Errorf("❌ 启动下载命令失败: %v", err)
		context["error"] = err.Error()
		return false
	}

	// 收集错误输出
	var errorOutput strings.Builder
	var lastOutput strings.Builder
	var lastProgressTime int64

	// 实时读取输出并收集错误信息
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			t.logDownloadProgress(line, &lastProgressTime)
			lastOutput.WriteString(line + "\n")
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			t.logDownloadProgress(line, &lastProgressTime)
			errorOutput.WriteString(line + "\n")
			lastOutput.WriteString(line + "\n")
		}
	}()

	// 等待命令完成
	if err := cmd.Wait(); err != nil {
		// 构建详细的错误信息
		errorMsg := fmt.Sprintf("下载失败: %v", err)

		// 添加错误输出的最后几行
		if errorOutput.Len() > 0 {
			lines := strings.Split(strings.TrimSpace(errorOutput.String()), "\n")
			if len(lines) > 0 {
				// 取最后5行错误信息
				startIdx := len(lines) - 5
				if startIdx < 0 {
					startIdx = 0
				}
				relevantErrors := strings.Join(lines[startIdx:], "\n")
				errorMsg += "\n\n详细错误:\n" + relevantErrors
			}
		}

		// 检查常见错误并给出建议
		if strings.Contains(errorOutput.String(), "Sign in to confirm") ||
			strings.Contains(errorOutput.String(), "not a bot") {
			errorMsg += "\n\n💡 建议: 需要 cookies.txt 文件来绕过机器人验证"
			errorMsg += "\n   请参考文档: docs/setup/cookies-setup.md"
		} else if strings.Contains(errorOutput.String(), "HTTP Error 403") {
			errorMsg += "\n\n💡 建议: 访问被拒绝，可能需要配置代理或更新 cookies"
		} else if strings.Contains(errorOutput.String(), "HTTP Error 404") {
			errorMsg += "\n\n💡 建议: 视频不存在或已被删除"
		} else if strings.Contains(errorOutput.String(), "Private video") {
			errorMsg += "\n\n💡 建议: 这是私有视频，无法下载"
		} else if strings.Contains(errorOutput.String(), "Video unavailable") {
			errorMsg += "\n\n💡 建议: 视频不可用，可能已被删除或设为私有"
		}

		t.App.Logger.Error("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.App.Logger.Errorf("❌ 视频下载失败")
		t.App.Logger.Errorf("📹 视频ID: %s", t.StateManager.VideoID)
		t.App.Logger.Errorf("🔗 视频URL: %s", videoURL)
		t.App.Logger.Error("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		t.App.Logger.Error(errorMsg)
		t.App.Logger.Error("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		context["error"] = errorMsg
		return false
	}

	// 10. 验证下载的文件
	downloadedFile := t.findDownloadedFile()
	if downloadedFile == "" {
		errMsg := "下载完成但未找到视频文件"
		t.App.Logger.Error("❌ " + errMsg)
		context["error"] = errMsg
		return false
	}

	// 11. 保存文件信息到 context
	context["downloaded_file"] = downloadedFile
	t.App.Logger.Infof("✓ 视频下载成功: %s", downloadedFile)

	// 12. 获取视频元数据（标题、描述等）
	t.App.Logger.Info("📋 获取视频元数据...")
	metadata, err := t.getVideoMetadata(ytdlpPath)
	if err != nil {
		t.App.Logger.Warnf("⚠️ 获取视频元数据失败: %v，将使用默认值", err)
	} else {
		context["original_title"] = metadata.Title
		context["original_description"] = metadata.Description
		t.App.Logger.Infof("✓ 原始标题: %s", metadata.Title)
		if metadata.Description != "" {
			t.App.Logger.Infof("✓ 原始描述: %s", t.truncateString(metadata.Description, 100))
		}

		// 保存到数据库
		if t.SavedVideoService != nil {
			savedVideo, err := t.SavedVideoService.GetVideoByVideoID(t.StateManager.VideoID)
			if err == nil {
				savedVideo.Title = metadata.Title
				savedVideo.Description = metadata.Description
				if err := t.SavedVideoService.UpdateVideo(savedVideo); err != nil {
					t.App.Logger.Errorf("❌ 保存原始元数据到数据库失败: %v", err)
				} else {
					t.App.Logger.Info("✅ 原始元数据已保存到数据库")
				}
			}
		}
	}

	t.App.Logger.Info("========================================")

	return true
}

// logOutput 实时输出日志
func (t *DownloadVideo) logOutput(reader io.Reader, level string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// 解析进度信息
		if strings.Contains(line, "[download]") {
			if strings.Contains(line, "Destination:") {
				t.App.Logger.Infof("📥 %s", line)
			} else if strings.Contains(line, "%") {
				// 进度信息，使用 Debug 级别避免日志过多
				t.App.Logger.Debugf("⏳ %s", line)
			} else {
				t.App.Logger.Infof("📥 %s", line)
			}
		} else if strings.Contains(line, "[ffmpeg]") {
			t.App.Logger.Infof("🔄 %s", line)
		} else {
			if level == "ERROR" {
				t.App.Logger.Warnf("⚠️  %s", line)
			} else {
				t.App.Logger.Debugf("%s", line)
			}
		}
	}
}

// findDownloadedFile 查找下载的视频文件
func (t *DownloadVideo) findDownloadedFile() string {
	// 查找目录下的 mp4 文件
	files, err := filepath.Glob(filepath.Join(t.StateManager.CurrentDir, "*.mp4"))
	if err != nil || len(files) == 0 {
		// 尝试查找其他视频格式
		for _, ext := range []string{"*.webm", "*.mkv", "*.flv"} {
			files, err = filepath.Glob(filepath.Join(t.StateManager.CurrentDir, ext))
			if err == nil && len(files) > 0 {
				break
			}
		}
	}

	if len(files) > 0 {
		// 返回最新的文件
		latestFile := files[0]
		latestTime := int64(0)

		for _, file := range files {
			info, err := os.Stat(file)
			if err != nil {
				continue
			}
			if info.ModTime().Unix() > latestTime {
				latestTime = info.ModTime().Unix()
				latestFile = file
			}
		}

		return latestFile
	}

	return ""
}

// VideoMetadataInfo 视频元数据信息
type VideoMetadataInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Uploader    string `json:"uploader"`
	Duration    int    `json:"duration"`
}

// getVideoMetadata 使用 yt-dlp 获取视频元数据（带代理回退）
func (t *DownloadVideo) getVideoMetadata(ytdlpPath string) (*VideoMetadataInfo, error) {
	videoURL := t.getVideoURL()

	// 构建基础命令参数
	args := []string{"--dump-json", "--no-download"}

	// 添加 cookies 支持
	configDir := filepath.Dir(t.App.Config.Path)
	cookiesPath := filepath.Join(configDir, "cookies.txt")
	if _, err := os.Stat(cookiesPath); err != nil {
		cookiesPath = "cookies.txt"
	}

	if _, err := os.Stat(cookiesPath); err == nil {
		absPath, _ := filepath.Abs(cookiesPath)
		args = append(args, "--cookies", absPath)
		t.App.Logger.Debugf("🍪 使用 Cookies 文件获取元数据: %s", absPath)
	} else {
		// 从浏览器读取 cookies
		args = append(args, "--cookies-from-browser", "chrome")
		t.App.Logger.Debug("🍪 从 Chrome 浏览器读取 cookies 获取元数据")
	}

	// 尝试使用代理
	useProxy := t.App.Config != nil && t.App.Config.ProxyConfig != nil &&
		t.App.Config.ProxyConfig.UseProxy && t.App.Config.ProxyConfig.ProxyHost != ""

	if useProxy {
		args = append(args, "--proxy", t.App.Config.ProxyConfig.ProxyHost)
		t.App.Logger.Debugf("📡 使用代理获取元数据: %s", t.App.Config.ProxyConfig.ProxyHost)
	}

	args = append(args, videoURL)

	// 第一次尝试（可能带代理）
	cmd := exec.Command(ytdlpPath, args...)
	output, err := cmd.Output()

	// 如果使用代理失败，尝试不使用代理
	if err != nil && useProxy {
		t.App.Logger.Warnf("⚠️ 使用代理获取元数据失败，尝试不使用代理...")
		argsNoProxy := []string{"--dump-json", "--no-download", videoURL}
		cmd = exec.Command(ytdlpPath, argsNoProxy...)
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("获取元数据失败: %v", err)
		}
		t.App.Logger.Info("✓ 不使用代理成功获取元数据")
	} else if err != nil {
		return nil, fmt.Errorf("获取元数据失败: %v", err)
	}

	var metadata VideoMetadataInfo
	if err := json.Unmarshal(output, &metadata); err != nil {
		return nil, fmt.Errorf("解析元数据失败: %v", err)
	}

	return &metadata, nil
}

// truncateString 截断字符串用于日志显示
func (t *DownloadVideo) truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// logDownloadProgress 解析并输出下载进度
func (t *DownloadVideo) logDownloadProgress(line string, lastProgressTime *int64) {
	now := time.Now().Unix()

	// aria2c 进度格式: [#abc123 100MiB/500MiB(20%) CN:16 DL:1.5MiB ETA:5m30s]
	aria2cRegex := regexp.MustCompile(`\[#\w+\s+([0-9.]+[KMGT]?i?B)/([0-9.]+[KMGT]?i?B)\((\d+)%\)\s+CN:(\d+)\s+DL:([0-9.]+[KMGT]?i?B)(?:\s+ETA:([^\]]+))?\]`)
	if matches := aria2cRegex.FindStringSubmatch(line); len(matches) >= 6 {
		// 每3秒输出一次进度，避免日志过多
		if now-*lastProgressTime >= 3 {
			*lastProgressTime = now
			downloaded := matches[1]
			total := matches[2]
			percent := matches[3]
			connections := matches[4]
			speed := matches[5]
			eta := "计算中"
			if len(matches) >= 7 && matches[6] != "" {
				eta = matches[6]
			}
			t.App.Logger.Infof("📥 下载进度: %s/%s (%s%%) | 速度: %s/s | 连接数: %s | 剩余时间: %s",
				downloaded, total, percent, speed, connections, eta)
		}
		return
	}

	// yt-dlp 进度格式: [download]  10.5% of 500.00MiB at 1.50MiB/s ETA 05:30
	ytdlpRegex := regexp.MustCompile(`\[download\]\s+([0-9.]+)%\s+of\s+~?([0-9.]+[KMGT]?i?B)\s+at\s+([0-9.]+[KMGT]?i?B/s)(?:\s+ETA\s+([0-9:]+))?`)
	if matches := ytdlpRegex.FindStringSubmatch(line); len(matches) >= 4 {
		if now-*lastProgressTime >= 3 {
			*lastProgressTime = now
			percent := matches[1]
			total := matches[2]
			speed := matches[3]
			eta := "计算中"
			if len(matches) >= 5 && matches[4] != "" {
				eta = matches[4]
			}
			t.App.Logger.Infof("📥 下载进度: %s%% of %s | 速度: %s | 剩余时间: %s",
				percent, total, speed, eta)
		}
		return
	}

	// 下载目标文件
	if strings.Contains(line, "[download] Destination:") {
		t.App.Logger.Infof("📁 %s", line)
		return
	}

	// 恢复下载
	if strings.Contains(line, "Resuming download") {
		t.App.Logger.Infof("🔄 %s", line)
		return
	}

	// 合并文件
	if strings.Contains(line, "[Merger]") || strings.Contains(line, "[ffmpeg]") {
		t.App.Logger.Infof("🔧 %s", line)
		return
	}

	// 下载完成
	if strings.Contains(line, "100%") || strings.Contains(line, "has already been downloaded") {
		t.App.Logger.Infof("✅ %s", line)
		return
	}

	// 睡眠等待
	if strings.Contains(line, "Sleeping") {
		t.App.Logger.Infof("⏳ %s", line)
		return
	}

	// 错误和警告
	if strings.Contains(line, "ERROR") || strings.Contains(line, "error") {
		t.App.Logger.Errorf("❌ %s", line)
		return
	}
	if strings.Contains(line, "WARNING") || strings.Contains(line, "warning") {
		t.App.Logger.Warnf("⚠️ %s", line)
		return
	}

	// 其他信息使用 Debug 级别
	t.App.Logger.Debug(line)
}
