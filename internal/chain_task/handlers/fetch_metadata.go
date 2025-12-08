package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/difyz9/ytb2bili/internal/chain_task/base"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/pkg/cos"
	"github.com/difyz9/ytb2bili/pkg/utils"
)

// FetchMetadata 获取视频元数据任务
type FetchMetadata struct {
	base.BaseTask
	App               *core.AppServer
	SavedVideoService *services.SavedVideoService
}

// NewFetchMetadata 创建获取元数据任务
func NewFetchMetadata(name string, app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient, savedVideoService *services.SavedVideoService) *FetchMetadata {
	return &FetchMetadata{
		BaseTask: base.BaseTask{
			Name:         name,
			StateManager: stateManager,
			Client:       client,
		},
		App:               app,
		SavedVideoService: savedVideoService,
	}
}

// Execute 执行获取元数据任务
func (t *FetchMetadata) Execute(context map[string]interface{}) bool {
	videoID := t.StateManager.VideoID
	t.App.Logger.Infof("📋 开始获取视频元数据: %s", videoID)

	// 1. 找到 yt-dlp
	var installDir string
	if t.App.Config != nil && t.App.Config.YtDlpPath != "" {
		installDir = t.App.Config.YtDlpPath
	}
	ytdlpManager := utils.NewYtDlpManager(t.App.Logger, installDir)
	if !ytdlpManager.IsInstalled() {
		errMsg := "未找到 yt-dlp，无法获取元数据"
		t.App.Logger.Error("❌ " + errMsg)
		context["error"] = errMsg
		return false
	}
	ytdlpPath := ytdlpManager.GetBinaryPath()

	// 2. 构建命令
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	command := []string{
		ytdlpPath,
		"--dump-json",
		"--no-download",
		videoURL,
	}

	// 添加 cookies 支持
	cookiesPath := t.findCookiesFile()
	if cookiesPath != "" {
		command = append(command, "--cookies", cookiesPath)
		t.App.Logger.Debugf("🍪 使用 Cookies 文件: %s", cookiesPath)
	}

	// 添加代理
	if t.App.Config != nil && t.App.Config.ProxyConfig != nil && t.App.Config.ProxyConfig.UseProxy && t.App.Config.ProxyConfig.ProxyHost != "" {
		command = append(command, "--proxy", t.App.Config.ProxyConfig.ProxyHost)
		t.App.Logger.Debugf("📡 使用代理: %s", t.App.Config.ProxyConfig.ProxyHost)
	}

	t.App.Logger.Debugf("执行命令: %v", command)

	// 3. 执行命令
	cmd := exec.Command(command[0], command[1:]...)
	output, err := cmd.Output()
	if err != nil {
		errMsg := fmt.Sprintf("执行 yt-dlp 获取元数据失败: %v", err)
		t.App.Logger.Error("❌ " + errMsg)
		context["error"] = errMsg
		return false
	}

	// 4. 解析 JSON
	var metadata VideoMetadataInfo
	if err := json.Unmarshal(output, &metadata); err != nil {
		errMsg := fmt.Sprintf("解析元数据 JSON 失败: %v", err)
		t.App.Logger.Error("❌ " + errMsg)
		context["error"] = errMsg
		return false
	}

	// 5. 更新数据库
	savedVideo, err := t.SavedVideoService.GetVideoByVideoID(videoID)
	if err != nil {
		errMsg := fmt.Sprintf("获取视频记录失败: %v", err)
		t.App.Logger.Error("❌ " + errMsg)
		context["error"] = errMsg
		return false
	}

	// 更新视频信息
	savedVideo.Title = metadata.Title
	savedVideo.Description = metadata.Description
	// 可以添加更多字段更新

	if err := t.SavedVideoService.UpdateVideo(savedVideo); err != nil {
		errMsg := fmt.Sprintf("更新数据库失败: %v", err)
		t.App.Logger.Error("❌ " + errMsg)
		context["error"] = errMsg
		return false
	}

	// 6. 将元数据存入 context 供后续任务使用
	context["metadata"] = &metadata
	context["video_title"] = metadata.Title
	context["video_description"] = metadata.Description
	context["video_duration"] = metadata.Duration
	context["video_uploader"] = metadata.Uploader

	t.App.Logger.Infof("✅ 成功获取元数据: %s", metadata.Title)
	t.App.Logger.Infof("   时长: %d秒, 上传者: %s", metadata.Duration, metadata.Uploader)

	return true
}

// findCookiesFile 查找 cookies 文件
func (t *FetchMetadata) findCookiesFile() string {
	// 优先使用配置文件目录下的 cookies.txt
	if t.App.Config != nil && t.App.Config.Path != "" {
		configDir := filepath.Dir(t.App.Config.Path)
		cookiesPath := filepath.Join(configDir, "cookies.txt")
		if _, err := os.Stat(cookiesPath); err == nil {
			absPath, _ := filepath.Abs(cookiesPath)
			return absPath
		}
	}

	// 尝试当前目录
	if _, err := os.Stat("cookies.txt"); err == nil {
		absPath, _ := filepath.Abs("cookies.txt")
		return absPath
	}

	return ""
}
