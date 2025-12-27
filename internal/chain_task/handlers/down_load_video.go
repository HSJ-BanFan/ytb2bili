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
	"sync"
	"time"

	"github.com/difyz9/ytb2bili/internal/chain_task/base"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/pkg/cos"
	"github.com/difyz9/ytb2bili/pkg/logger"
	"github.com/difyz9/ytb2bili/pkg/utils"
	"gorm.io/gorm"
)

type DownloadVideo struct {
	base.BaseTask
	App                *core.AppServer
	DB                 *gorm.DB
	SavedVideoService  *services.SavedVideoService
	progressMutex      sync.Mutex
	lastProgressUpdate int64
}

// DownloadProgress 下载进度信息
type DownloadProgress struct {
	Percent    float64   `json:"percent"`    // 下载百分比
	Downloaded string    `json:"downloaded"` // 已下载大小
	Total      string    `json:"total"`      // 总大小
	Speed      string    `json:"speed"`      // 下载速度
	ETA        string    `json:"eta"`        // 剩余时间
	UpdatedAt  time.Time `json:"updated_at"` // 更新时间
}

func NewDownloadVideo(name string, app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient, savedVideoService *services.SavedVideoService) *DownloadVideo {
	return &DownloadVideo{
		BaseTask: base.BaseTask{
			Name:         name,
			StateManager: stateManager,
			Client:       client,
		},
		App:                app,
		SavedVideoService:  savedVideoService,
		lastProgressUpdate: 0,
	}
}

// updateDownloadProgress 更新下载进度到数据库
func (t *DownloadVideo) updateDownloadProgress(progress *DownloadProgress) {
	if t.SavedVideoService == nil {
		return
	}

	t.progressMutex.Lock()
	defer t.progressMutex.Unlock()

	now := time.Now().Unix()
	// 限制更新频率，每3秒最多更新一次数据库
	if now-t.lastProgressUpdate < 3 {
		return
	}
	t.lastProgressUpdate = now

	progressJSON, err := json.Marshal(progress)
	if err != nil {
		t.App.Logger.Debugf("序列化下载进度失败: %v", err)
		return
	}

	savedVideo, err := t.SavedVideoService.GetVideoByVideoID(t.StateManager.VideoID)
	if err != nil {
		t.App.Logger.Debugf("获取视频信息失败，无法更新下载进度: %v", err)
		return
	}

	savedVideo.DownloadProgress = string(progressJSON)
	if err := t.SavedVideoService.UpdateVideo(savedVideo); err != nil {
		t.App.Logger.Debugf("更新下载进度到数据库失败: %v", err)
	}
}

// clearDownloadProgress 清除下载进度（下载完成或失败时调用）
func (t *DownloadVideo) clearDownloadProgress() {
	if t.SavedVideoService == nil {
		return
	}

	savedVideo, err := t.SavedVideoService.GetVideoByVideoID(t.StateManager.VideoID)
	if err != nil {
		return
	}

	savedVideo.DownloadProgress = ""
	_ = t.SavedVideoService.UpdateVideo(savedVideo)
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

func (t *DownloadVideo) Execute(ctx map[string]interface{}) bool {
	t.App.Logger.Info("========================================")
	t.App.Logger.Info("DownloadVideo Handler Version: with-progress-tracking-v6") // 版本标记
	t.App.Logger.Infof("开始下载视频: %s", t.StateManager.VideoID)
	t.App.Logger.Info("========================================")

	// 确保下载完成后清除进度
	defer t.clearDownloadProgress()

	// 0. 检查视频文件是否已存在且完整
	if existingVideo := t.checkExistingVideo(); existingVideo != "" {
		t.App.Logger.Infof("✅ 视频文件已存在: %s", existingVideo)
		t.App.Logger.Info("✅ 跳过下载，直接标记为完成")
		ctx["video_path"] = existingVideo
		return true
	}

	// 0.5 清理旧的临时下载文件，避免文件系统冲突
	if cleaned := t.cleanPartFiles(); cleaned > 0 {
		t.App.Logger.Infof("🧹 已清理 %d 个临时下载文件", cleaned)
	}

	// 1. 查找 yt-dlp 可执行文件
	ytdlpPath, err := t.findYtDlp()
	if err != nil {
		t.App.Logger.Errorf("❌ %v", err)
		ctx["error"] = err.Error()
		return false
	}

	// 2. 确保下载目录存在
	if err := os.MkdirAll(t.StateManager.CurrentDir, 0755); err != nil {
		t.App.Logger.Errorf("❌ 创建下载目录失败: %v", err)
		ctx["error"] = err.Error()
		return false
	}

	// 3. 尝试下载（多层回退策略）
	videoURL := t.getVideoURL()
	useProxy := t.App.Config != nil && t.App.Config.ProxyConfig != nil &&
		t.App.Config.ProxyConfig.UseProxy && t.App.Config.ProxyConfig.ProxyHost != ""

	// 检查 cookies 文件
	hasCookiesFile := t.findCookiesFile() != ""
	hasBackupCookies := t.findBackupCookiesFile() != ""

	// 定义下载尝试策略
	type downloadAttempt struct {
		useProxy    bool
		authMode    string // "cookies_file", "cookies_backup", "browser"
		description string
	}

	attempts := []downloadAttempt{}

	// 策略优先级：cookies.txt > cookies_backup.txt > 浏览器cookies
	// 每种认证方式都有 代理/无代理 两个变体

	// 主 cookies.txt 策略
	if hasCookiesFile {
		if useProxy {
			attempts = append(attempts, downloadAttempt{true, "cookies_file", "代理 + cookies.txt"})
		}
		attempts = append(attempts, downloadAttempt{false, "cookies_file", "无代理 + cookies.txt"})
	}

	// 备用 cookies_backup.txt 策略
	if hasBackupCookies {
		if useProxy {
			attempts = append(attempts, downloadAttempt{true, "cookies_backup", "代理 + cookies_backup.txt"})
		}
		attempts = append(attempts, downloadAttempt{false, "cookies_backup", "无代理 + cookies_backup.txt"})
	}

	// 浏览器 cookies 策略（最后的回退）
	if useProxy {
		attempts = append(attempts, downloadAttempt{true, "browser", "代理 + 浏览器cookies"})
	}
	attempts = append(attempts, downloadAttempt{false, "browser", "无代理 + 浏览器cookies"})

	for i, attempt := range attempts {
		t.App.Logger.Infof("🔄 尝试策略 [%d/%d]: %s", i+1, len(attempts), attempt.description)
		if t.executeDownloadWithAuthMode(ytdlpPath, videoURL, attempt.useProxy, attempt.authMode, ctx) {
			return true
		}
		if i < len(attempts)-1 {
			t.App.Logger.Warn("⚠️ 下载失败，尝试下一策略...")
		}
	}

	return false
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

// findCookiesFile 查找 cookies.txt 文件
func (t *DownloadVideo) findCookiesFile() string {
	possiblePaths := []string{}

	// 1. 配置文件目录
	if t.App.Config != nil && t.App.Config.Path != "" {
		configDir := filepath.Dir(t.App.Config.Path)
		possiblePaths = append(possiblePaths, filepath.Join(configDir, "cookies.txt"))
	}

	// 2. 可执行文件目录
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		possiblePaths = append(possiblePaths, filepath.Join(exeDir, "cookies.txt"))
	}

	// 3. 当前工作目录
	if cwd, err := os.Getwd(); err == nil {
		possiblePaths = append(possiblePaths, filepath.Join(cwd, "cookies.txt"))
	}

	// 4. 项目根目录
	if t.StateManager != nil && t.StateManager.CurrentDir != "" {
		projectRoot := t.StateManager.CurrentDir
		for i := 0; i < 4; i++ {
			projectRoot = filepath.Dir(projectRoot)
		}
		possiblePaths = append(possiblePaths, filepath.Join(projectRoot, "cookies.txt"))
	}

	// 5. 相对路径
	possiblePaths = append(possiblePaths, "cookies.txt")

	// 查找第一个存在的 cookies 文件
	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			absPath, _ := filepath.Abs(p)
			return absPath
		}
	}
	return ""
}

// findBackupCookiesFile 查找备用 cookies 文件
func (t *DownloadVideo) findBackupCookiesFile() string {
	possiblePaths := []string{}

	// 当前工作目录
	if cwd, err := os.Getwd(); err == nil {
		possiblePaths = append(possiblePaths, filepath.Join(cwd, "cookies_backup.txt"))
	}

	// 配置文件目录
	if t.App.Config != nil && t.App.Config.Path != "" {
		configDir := filepath.Dir(t.App.Config.Path)
		possiblePaths = append(possiblePaths, filepath.Join(configDir, "cookies_backup.txt"))
	}

	// 相对路径
	possiblePaths = append(possiblePaths, "cookies_backup.txt")

	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			absPath, _ := filepath.Abs(p)
			return absPath
		}
	}
	return ""
}

// executeDownloadWithAuthMode 使用指定的认证模式执行下载
// authMode: "oauth2", "cookies_file", "browser"
func (t *DownloadVideo) executeDownloadWithAuthMode(ytdlpPath, videoURL string, useProxy bool, authMode string, context map[string]interface{}) bool {
	// 创建紧凑进度日志器
	progressLogger := logger.NewCompactProgressLogger(t.StateManager.VideoID)
	defer progressLogger.Complete("")

	progressLogger.Start()

	// 构建下载命令
	command := []string{
		ytdlpPath,
		"-P", t.StateManager.CurrentDir,
		"-o", "%(id)s.%(ext)s",
		// 强制使用 H.264(avc1) + AAC 格式，确保 B站兼容
		// 优先级：1080p H.264 > 720p H.264 > best H.264 > best
		"-f", "bestvideo[vcodec^=avc1][height<=1080]+bestaudio[acodec^=mp4a]/bestvideo[vcodec^=avc1]+bestaudio[acodec^=mp4a]/best[vcodec^=avc1]/best",
		"--merge-output-format", "mp4",
		"--retries", "10",
		"--fragment-retries", "10",
		"--socket-timeout", "30",
		"--file-access-retries", "5",
		"--extractor-retries", "5",
		"--write-subs",
		"--write-auto-subs",
		"--sub-langs", "en,zh-Hans",
		"--sub-format", "srt/best",
		"--convert-subs", "srt",
		"--no-abort-on-error",
	}

	// 获取下载配置
	concurrentFragments, aria2cConnections, httpChunkSize := t.getDownloadConfig()

	// 检查是否启用 aria2c
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

	// aria2c 相关配置
	aria2cPath := t.findAria2c()
	aria2cWithProxy := false
	if t.App.Config != nil && t.App.Config.DownloadConfig != nil {
		aria2cWithProxy = t.App.Config.DownloadConfig.Aria2cWithProxy
	}

	canUseAria2c := useAria2c && aria2cPath != "" && (proxyHost == "" || aria2cWithProxy)

	if canUseAria2c {
		t.App.Logger.Infof("🚀 检测到 aria2c，启用多线程下载加速 (连接数: %d)", aria2cConnections)
		aria2cArgs := fmt.Sprintf("aria2c:-x %d -s %d -k 1M --file-allocation=none --async-dns=false --check-certificate=false --auto-file-renaming=false --allow-overwrite=true --max-tries=5 --retry-wait=3", aria2cConnections, aria2cConnections)
		if proxyHost != "" {
			aria2cArgs += fmt.Sprintf(" --all-proxy=%s", proxyHost)
			t.App.Logger.Infof("📡 aria2c 使用代理: %s", proxyHost)
		}
		command = append(command,
			"--downloader", "aria2c",
			"--downloader-args", aria2cArgs,
		)
	} else {
		t.App.Logger.Infof("📊 使用并发分片数: %d, HTTP分块大小: %s", concurrentFragments, httpChunkSize)
		command = append(command,
			"--concurrent-fragments", fmt.Sprintf("%d", concurrentFragments),
			"--buffer-size", "16K",
			"--http-chunk-size", httpChunkSize,
		)
	}

	// 根据 authMode 添加认证参数
	switch authMode {
	case "cookies_file":
		// 使用 cookies.txt 文件
		cookiesPath := t.findCookiesFile()
		if cookiesPath != "" {
			command = append(command, "--cookies", cookiesPath)
			t.App.Logger.Infof("🍪 使用 Cookies 文件: %s", cookiesPath)
		}
	case "cookies_backup":
		// 使用备用 cookies_backup.txt 文件
		backupPath := t.findBackupCookiesFile()
		if backupPath != "" {
			command = append(command, "--cookies", backupPath)
			t.App.Logger.Infof("🍪 使用备用 Cookies 文件: %s", backupPath)
		}
	case "browser":
		// 使用浏览器提取 cookies
		browserName := "chrome"
		if t.App.Config != nil && t.App.Config.DownloadConfig != nil &&
			t.App.Config.DownloadConfig.CookiesFromBrowser != "" {
			browserName = t.App.Config.DownloadConfig.CookiesFromBrowser
		}
		if browserName != "none" && browserName != "disabled" {
			command = append(command, "--cookies-from-browser", browserName)
			t.App.Logger.Infof("🍪 从 %s 浏览器读取 cookies", browserName)
		}
	}

	// 添加代理配置
	if useProxy && proxyHost != "" {
		command = append(command, "--proxy", proxyHost)
		t.App.Logger.Infof("📡 使用代理: %s", proxyHost)
	} else if !useProxy {
		t.App.Logger.Info("🌐 不使用代理")
	}

	// 添加视频标识符和URL
	command = append(command, "--", t.StateManager.VideoID)
	command = append(command, videoURL)

	t.App.Logger.Infof("执行命令: %s", strings.Join(command, " "))

	// 执行下载命令
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = t.StateManager.CurrentDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		context["error"] = err.Error()
		return false
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		context["error"] = err.Error()
		return false
	}

	if err := cmd.Start(); err != nil {
		context["error"] = err.Error()
		return false
	}

	// 收集输出
	var errorOutput strings.Builder
	var lastProgressTime int64

	// 自定义分割函数，同时处理 \n 和 \r（yt-dlp 分片下载使用 \r 覆盖输出）
	splitFunc := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		// 查找 \n 或 \r
		for i := 0; i < len(data); i++ {
			if data[i] == '\n' || data[i] == '\r' {
				return i + 1, data[0:i], nil
			}
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		// 增加缓冲区大小到1MB，防止aria2c输出过大导致缓冲区溢出
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		scanner.Split(splitFunc)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			t.logDownloadProgressCompact(line, &lastProgressTime, progressLogger)
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		// 增加缓冲区大小到1MB
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		scanner.Split(splitFunc)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			t.logDownloadProgressCompact(line, &lastProgressTime, progressLogger)
			errorOutput.WriteString(line + "\n")
		}
	}()

	if err := cmd.Wait(); err != nil {
		errorMsg := fmt.Sprintf("下载失败: %v", err)
		if errorOutput.Len() > 0 {
			errorMsg += "\n\n详细错误:\n" + errorOutput.String()
		}
		progressLogger.Error(errorMsg)
		context["error"] = errorMsg
		return false
	}

	// 验证下载的文件
	downloadedFile := t.findDownloadedFile()
	if downloadedFile == "" {
		context["error"] = "下载完成但未找到视频文件"
		return false
	}

	context["downloaded_file"] = downloadedFile
	progressLogger.Complete("视频下载成功: " + downloadedFile)
	return true
}

// executeDownload 执行实际的下载操作（兼容旧代码）
func (t *DownloadVideo) executeDownload(ytdlpPath, videoURL string, useProxy bool, context map[string]interface{}) bool {
	// 构建下载命令
	command := []string{
		ytdlpPath,
		"-P", t.StateManager.CurrentDir,
		"-o", "%(id)s.%(ext)s",
		// 强制使用 H.264(avc1) + AAC 格式，确保 B站兼容
		"-f", "bestvideo[vcodec^=avc1][height<=1080]+bestaudio[acodec^=mp4a]/bestvideo[vcodec^=avc1]+bestaudio[acodec^=mp4a]/best[vcodec^=avc1]/best",
		"--merge-output-format", "mp4",
		// 网络优化参数
		"--retries", "10", // 重试次数
		"--fragment-retries", "10", // 分片重试次数
		"--socket-timeout", "30", // Socket 超时（秒）
		"--file-access-retries", "5", // 文件访问重试
		"--extractor-retries", "5", // 提取器重试
		// 字幕下载参数（只下载必要的语言，减少冗余文件）
		"--write-subs",              // 下载字幕
		"--write-auto-subs",         // 下载自动生成的字幕
		"--sub-langs", "en,zh-Hans", // 只下载英文和简体中文字幕
		"--sub-format", "srt/best", // 字幕格式优先级
		"--convert-subs", "srt", // 转换为 SRT 格式
		// 错误处理
		"--no-abort-on-error", // 字幕下载失败时不中断视频下载
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
	// 注意：当使用代理时，aria2c 可能会遇到 403 错误，可通过配置 aria2c_with_proxy 强制启用
	aria2cPath := t.findAria2c()
	aria2cWithProxy := false
	if t.App.Config != nil && t.App.Config.DownloadConfig != nil {
		aria2cWithProxy = t.App.Config.DownloadConfig.Aria2cWithProxy
	}

	// 判断是否使用 aria2c：启用 && 已安装 && (无代理 || 配置允许代理时使用)
	canUseAria2c := useAria2c && aria2cPath != "" && (proxyHost == "" || aria2cWithProxy)

	if canUseAria2c {
		t.App.Logger.Infof("🚀 检测到 aria2c，启用多线程下载加速 (连接数: %d)", aria2cConnections)
		aria2cArgs := fmt.Sprintf("aria2c:-x %d -s %d -k 1M --file-allocation=none --async-dns=false --check-certificate=false --auto-file-renaming=false --allow-overwrite=true --max-tries=5 --retry-wait=3", aria2cConnections, aria2cConnections)
		// 如果使用代理，为 aria2c 添加代理参数
		if proxyHost != "" {
			aria2cArgs += fmt.Sprintf(" --all-proxy=%s", proxyHost)
			t.App.Logger.Infof("📡 aria2c 使用代理: %s", proxyHost)
		}
		command = append(command,
			"--downloader", "aria2c",
			"--downloader-args", aria2cArgs,
		)
	} else {
		if proxyHost != "" && !aria2cWithProxy {
			t.App.Logger.Info("ℹ️ 使用代理时，采用 yt-dlp 内置并发下载器（避免 aria2c 403 错误）")
			t.App.Logger.Info("💡 如需使用 aria2c，可在配置中设置 aria2c_with_proxy = true")
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
	// 尝试多个可能的位置
	var cookiesPath string
	possiblePaths := []string{}

	// 1. 配置文件目录
	if t.App.Config != nil && t.App.Config.Path != "" {
		configDir := filepath.Dir(t.App.Config.Path)
		possiblePaths = append(possiblePaths, filepath.Join(configDir, "cookies.txt"))
	}

	// 2. 可执行文件目录
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		possiblePaths = append(possiblePaths, filepath.Join(exeDir, "cookies.txt"))
	}

	// 3. 当前工作目录
	if cwd, err := os.Getwd(); err == nil {
		possiblePaths = append(possiblePaths, filepath.Join(cwd, "cookies.txt"))
	}

	// 4. 项目根目录（StateManager.CurrentDir 的上级目录）
	if t.StateManager != nil && t.StateManager.CurrentDir != "" {
		// 从 data/media/date/videoID 向上找到项目根目录
		projectRoot := t.StateManager.CurrentDir
		for i := 0; i < 4; i++ {
			projectRoot = filepath.Dir(projectRoot)
		}
		possiblePaths = append(possiblePaths, filepath.Join(projectRoot, "cookies.txt"))
	}

	// 5. 相对路径
	possiblePaths = append(possiblePaths, "cookies.txt")

	// 查找第一个存在的 cookies 文件
	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil {
			cookiesPath = p
			break
		}
	}

	if cookiesPath != "" {
		absPath, _ := filepath.Abs(cookiesPath)
		command = append(command, "--cookies", absPath)
		t.App.Logger.Infof("🍪 使用 Cookies 文件: %s", absPath)
	} else {
		// 如果没有 cookies 文件，尝试从浏览器读取
		browserName := "chrome" // 默认浏览器
		if t.App.Config != nil && t.App.Config.DownloadConfig != nil &&
			t.App.Config.DownloadConfig.CookiesFromBrowser != "" {
			browserName = t.App.Config.DownloadConfig.CookiesFromBrowser
		}

		// 检查是否明确禁用浏览器 cookies（设置为 "none" 或 "disabled"）
		if browserName != "none" && browserName != "disabled" {
			t.App.Logger.Warnf("🍪 未找到 cookies 文件，尝试的路径: %v", possiblePaths)
			command = append(command, "--cookies-from-browser", browserName)
			t.App.Logger.Infof("🍪 将从 %s 浏览器读取 cookies", browserName)
		} else {
			t.App.Logger.Warn("⚠️ 未配置 cookies，可能会遇到 'Sign in to confirm you're not a bot' 错误")
		}
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

	// 自定义分割函数，同时处理 \n 和 \r
	splitFunc := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		// 查找 \n 或 \r
		for i := 0; i < len(data); i++ {
			if data[i] == '\n' || data[i] == '\r' {
				return i + 1, data[0:i], nil
			}
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	}

	// 实时读取输出并收集错误信息
	go func() {
		scanner := bufio.NewScanner(stdout)
		// 增加缓冲区大小到1MB，防止aria2c输出过大导致缓冲区溢出
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		scanner.Split(splitFunc)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			t.logDownloadProgress(line, &lastProgressTime)
			lastOutput.WriteString(line + "\n")
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		// 增加缓冲区大小到1MB
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		scanner.Split(splitFunc)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
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
		// 从配置读取浏览器类型
		browserName := "chrome"
		if t.App.Config != nil && t.App.Config.DownloadConfig != nil &&
			t.App.Config.DownloadConfig.CookiesFromBrowser != "" {
			browserName = t.App.Config.DownloadConfig.CookiesFromBrowser
		}
		if browserName != "none" && browserName != "disabled" {
			args = append(args, "--cookies-from-browser", browserName)
			t.App.Logger.Debugf("🍪 从 %s 浏览器读取 cookies 获取元数据", browserName)
		}
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

// formatProgressBar 生成进度条字符串
func formatProgressBar(percent float64, width int) string {
	if width <= 0 {
		width = 30
	}

	filled := int(percent / 100.0 * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("[%s]", bar)
}

// formatBytes 格式化字节大小（添加颜色标记）
func formatBytes(bytesStr string) string {
	return fmt.Sprintf("%-8s", bytesStr)
}

// logDownloadProgress 解析并输出下载进度
func (t *DownloadVideo) logDownloadProgress(line string, lastProgressTime *int64) {
	now := time.Now().Unix()

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
			// 更新进度到数据库（每3秒一次，通过内部互斥锁控制）
			t.updateDownloadProgress(&DownloadProgress{
				Percent:    percent,
				Downloaded: downloaded,
				Total:      total,
				Speed:      speed + "/s",
				ETA:        eta,
				UpdatedAt:  time.Now(),
			})
		}

		// 每3秒输出一次详细进度日志
		if now-*lastProgressTime >= 3 {
			*lastProgressTime = now

			// 生成进度条
			progressBar := formatProgressBar(percent, 30)

			// 格式化的输出
			t.App.Logger.Info("╭─────────────────────────────────────────────────────────────────╮")
			t.App.Logger.Info("│                    📥 视频下载进度                             │")
			t.App.Logger.Info("├─────────────────────────────────────────────────────────────────┤")
			t.App.Logger.Infof("│  视频 ID    │ %s%-44s│", t.StateManager.VideoID, "")
			t.App.Logger.Infof("│  下载进度   │ %s %6s%%                                           │",
				progressBar, percentStr)
			t.App.Logger.Infof("│  已下载     │ %s / %-15s                              │",
				formatBytes(downloaded), total)
			t.App.Logger.Infof("│  下载速度   │ %s/s (连接数: %s)                                │",
				formatBytes(speed), connections)
			t.App.Logger.Infof("│  剩余时间   │ %-50s│", eta)
			t.App.Logger.Info("╰─────────────────────────────────────────────────────────────────╯")
		}
		return
	}

	// yt-dlp 进度格式: [download]  10.5% of  239.12MiB at    4.62MiB/s ETA 00:42
	// 也可能是: [download]   8.3% of  239.12MiB at  Unknown B/s ETA Unknown
	ytdlpRegex := regexp.MustCompile(`\[download\]\s+([0-9.]+)%\s+of\s+~?\s*([0-9.]+\s*[KMGT]?i?B)\s+at\s+(.+?)\s+ETA\s+(.+)`)
	if matches := ytdlpRegex.FindStringSubmatch(line); len(matches) >= 5 {
		percentStr := matches[1]
		total := strings.TrimSpace(matches[2])
		speed := strings.TrimSpace(matches[3])
		eta := strings.TrimSpace(matches[4])
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
				Percent:    percent,
				Downloaded: "",
				Total:      total,
				Speed:      speed,
				ETA:        eta,
				UpdatedAt:  time.Now(),
			})
		}

		// 每3秒输出一次详细进度日志
		if now-*lastProgressTime >= 3 {
			*lastProgressTime = now

			// 生成进度条
			progressBar := formatProgressBar(percent, 30)

			// 格式化的输出
			t.App.Logger.Info("╭─────────────────────────────────────────────────────────────────╮")
			t.App.Logger.Info("│                    📥 视频下载进度                             │")
			t.App.Logger.Info("├─────────────────────────────────────────────────────────────────┤")
			t.App.Logger.Infof("│  视频 ID    │ %s%-44s│", t.StateManager.VideoID, "")
			t.App.Logger.Infof("│  下载进度   │ %s %6s%%                                           │",
				progressBar, percentStr)
			t.App.Logger.Infof("│  文件大小   │ %-51s│", total)
			t.App.Logger.Infof("│  下载速度   │ %-51s│", speed)
			t.App.Logger.Infof("│  剩余时间   │ %-50s│", eta)
			t.App.Logger.Info("╰─────────────────────────────────────────────────────────────────╯")
		}
		return
	}

	// 下载目标文件
	if strings.Contains(line, "[download] Destination:") {
		t.App.Logger.Info("╭─────────────────────────────────────────────────────────────────╮")
		t.App.Logger.Info("│                    📥 开始下载视频                               │")
		t.App.Logger.Info("╰─────────────────────────────────────────────────────────────────╯")
		t.App.Logger.Infof("📁 %s", strings.TrimSpace(strings.Replace(line, "[download] Destination:", "", 1)))
		return
	}

	// 恢复下载
	if strings.Contains(line, "Resuming download") {
		t.App.Logger.Info("╭─────────────────────────────────────────────────────────────────╮")
		t.App.Logger.Info("│                    🔄 恢复下载                                   │")
		t.App.Logger.Info("╰─────────────────────────────────────────────────────────────────╯")
		t.App.Logger.Infof("🔄 %s", line)
		return
	}

	// 合并文件
	if strings.Contains(line, "[Merger]") || strings.Contains(line, "[ffmpeg]") {
		t.App.Logger.Info("╭─────────────────────────────────────────────────────────────────╮")
		t.App.Logger.Info("│                    🔧 合并视频文件                               │")
		t.App.Logger.Info("╰─────────────────────────────────────────────────────────────────╯")
		t.App.Logger.Infof("🔧 %s", line)
		return
	}

	// 下载完成
	if strings.Contains(line, "100%") || strings.Contains(line, "has already been downloaded") {
		t.App.Logger.Info("╭─────────────────────────────────────────────────────────────────╮")
		t.App.Logger.Info("│                    ✅ 下载完成                                   │")
		t.App.Logger.Info("╰─────────────────────────────────────────────────────────────────╯")
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
		t.App.Logger.Error("╭─────────────────────────────────────────────────────────────────╮")
		t.App.Logger.Error("│                    ❌ 下载错误                                   │")
		t.App.Logger.Error("╰─────────────────────────────────────────────────────────────────╯")
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

// checkExistingVideo 检查视频文件是否已存在且完整
// 返回视频文件路径，如果不存在或不完整则返回空字符串
func (t *DownloadVideo) checkExistingVideo() string {
	videoID := t.StateManager.VideoID
	currentDir := t.StateManager.CurrentDir

	// 检查目录是否存在
	if _, err := os.Stat(currentDir); os.IsNotExist(err) {
		return ""
	}

	// 支持的视频格式
	videoExtensions := []string{".mp4", ".mkv", ".webm", ".flv", ".avi", ".mov"}

	// 查找匹配的视频文件
	for _, ext := range videoExtensions {
		// 尝试精确匹配: VideoID.ext
		exactPath := filepath.Join(currentDir, videoID+ext)
		if info, err := os.Stat(exactPath); err == nil && !info.IsDir() {
			// 检查文件大小是否合理（至少 1MB，排除空文件或损坏文件）
			if info.Size() > 1024*1024 {
				t.App.Logger.Infof("📁 找到已存在的视频文件: %s (%.2f MB)", exactPath, float64(info.Size())/(1024*1024))
				return exactPath
			}
		}
	}

	// 使用 glob 模式查找包含 VideoID 的视频文件
	for _, ext := range videoExtensions {
		pattern := filepath.Join(currentDir, "*"+videoID+"*"+ext)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			// 排除 .part 文件（未完成的下载）
			if strings.HasSuffix(match, ".part") {
				continue
			}
			if info, err := os.Stat(match); err == nil && !info.IsDir() {
				// 检查文件大小是否合理
				if info.Size() > 1024*1024 {
					t.App.Logger.Infof("📁 找到已存在的视频文件: %s (%.2f MB)", match, float64(info.Size())/(1024*1024))
					return match
				}
			}
		}
	}

	// 检查是否有未完成的下载（.part 文件）
	partPattern := filepath.Join(currentDir, "*.part")
	if partMatches, _ := filepath.Glob(partPattern); len(partMatches) > 0 {
		t.App.Logger.Infof("📥 发现未完成的下载文件，将继续下载...")
	}

	return ""
}

// cleanPartFiles 清理下载目录中的视频临时文件（.part 和 .aria2）
// 注意：只清理视频相关的临时文件，保留字幕和封面文件
// 返回清理的文件数量
func (t *DownloadVideo) cleanPartFiles() int {
	videoID := t.StateManager.VideoID
	currentDir := t.StateManager.CurrentDir

	// 检查目录是否存在
	if _, err := os.Stat(currentDir); os.IsNotExist(err) {
		return 0
	}

	cleaned := 0

	// 视频文件扩展名（只清理这些类型的 .part 文件）
	videoExtensions := []string{".mp4", ".webm", ".mkv", ".flv", ".avi", ".mov"}

	// 清理与当前视频相关的视频临时文件
	for _, ext := range videoExtensions {
		// 清理 videoID*.ext.part 格式的文件
		partPattern := filepath.Join(currentDir, videoID+"*"+ext+".part")
		partMatches, err := filepath.Glob(partPattern)
		if err == nil {
			for _, match := range partMatches {
				if err := os.Remove(match); err == nil {
					t.App.Logger.Debugf("🧹 清理视频临时文件: %s", filepath.Base(match))
					cleaned++
				}
			}
		}
	}

	// 清理 yt-dlp 格式的分片文件 (videoID.f*.part)
	fragmentPattern := filepath.Join(currentDir, videoID+".f*.part")
	fragmentMatches, err := filepath.Glob(fragmentPattern)
	if err == nil {
		for _, match := range fragmentMatches {
			if err := os.Remove(match); err == nil {
				t.App.Logger.Debugf("🧹 清理视频分片文件: %s", filepath.Base(match))
				cleaned++
			}
		}
	}

	// 同时清理 aria2c 的控制文件 (.aria2)，但只清理视频相关的
	for _, ext := range videoExtensions {
		aria2Pattern := filepath.Join(currentDir, videoID+"*"+ext+".aria2")
		aria2Matches, err := filepath.Glob(aria2Pattern)
		if err == nil {
			for _, match := range aria2Matches {
				if err := os.Remove(match); err == nil {
					t.App.Logger.Debugf("🧹 清理 aria2 控制文件: %s", filepath.Base(match))
					cleaned++
				}
			}
		}
	}

	// 清理分片文件的 aria2 控制文件
	fragmentAria2Pattern := filepath.Join(currentDir, videoID+".f*.aria2")
	fragmentAria2Matches, err := filepath.Glob(fragmentAria2Pattern)
	if err == nil {
		for _, match := range fragmentAria2Matches {
			if err := os.Remove(match); err == nil {
				t.App.Logger.Debugf("🧹 清理分片 aria2 控制文件: %s", filepath.Base(match))
				cleaned++
			}
		}
	}

	// 清理超过 24 小时的孤立视频 .part 文件（不清理字幕 .srt.part）
	for _, ext := range videoExtensions {
		oldPartPattern := filepath.Join(currentDir, "*"+ext+".part")
		oldPartMatches, err := filepath.Glob(oldPartPattern)
		if err == nil {
			for _, match := range oldPartMatches {
				if info, err := os.Stat(match); err == nil {
					// 如果文件超过 24 小时未修改，删除它
					if time.Since(info.ModTime()) > 24*time.Hour {
						if err := os.Remove(match); err == nil {
							t.App.Logger.Debugf("🧹 清理过期视频临时文件: %s", filepath.Base(match))
							cleaned++
						}
					}
				}
			}
		}
	}

	return cleaned
}
