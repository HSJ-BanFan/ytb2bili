package handlers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/difyz9/bilibili-go-sdk/bilibili"
	"github.com/difyz9/ytb2bili/internal/chain_task/base"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/storage"
	"github.com/difyz9/ytb2bili/pkg/cos"
)

type UploadSubtitleToBilibili struct {
	base.BaseTask
	App                *core.AppServer
	SavedVideoService  *services.SavedVideoService
	BiliAccountService *services.BiliAccountService
}

func NewUploadSubtitleToBilibili(name string, app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient, savedVideoService *services.SavedVideoService, biliAccountService *services.BiliAccountService) *UploadSubtitleToBilibili {
	return &UploadSubtitleToBilibili{
		BaseTask: base.BaseTask{
			Name:         name,
			StateManager: stateManager,
			Client:       client,
		},
		App:                app,
		SavedVideoService:  savedVideoService,
		BiliAccountService: biliAccountService,
	}
}

func (t *UploadSubtitleToBilibili) Execute(context map[string]interface{}) bool {
	t.App.Logger.Info("========================================")
	t.App.Logger.Info("开始上传字幕到 Bilibili")
	t.App.Logger.Info("========================================")

	// 1. 检查是否有BVID（视频已上传成功）
	bvid, exists := context["bili_bvid"].(string)
	if !exists || bvid == "" {
		// 尝试从数据库获取BVID
		savedVideo, err := t.SavedVideoService.GetVideoByVideoID(t.StateManager.VideoID)
		if err != nil || savedVideo.BiliBVID == "" {
			t.App.Logger.Warn("⚠️  没有找到BVID，跳过字幕上传")
			return true // 不算失败，只是跳过
		}
		bvid = savedVideo.BiliBVID
	}

	t.App.Logger.Infof("📺 视频BVID: %s", bvid)

	// 2. 获取登录信息（优先使用新的多账号系统）
	loginInfo, err := t.getLoginInfo()
	if err != nil {
		t.App.Logger.Errorf("❌ 没有有效的 Bilibili 登录信息，无法上传字幕: %v", err)
		context["error"] = "未登录 Bilibili"
		return false
	}

	// 3. 查找字幕文件
	subtitleFiles := t.findSubtitleFiles()
	if len(subtitleFiles) == 0 {
		t.App.Logger.Warn("⚠️  未找到字幕文件，跳过字幕上传")
		return true // 不算失败，只是跳过
	}

	// 4. 创建自定义字幕上传器（包含 lan_doc 参数支持）
	uploader := NewCustomSubtitleUploader(loginInfo)

	// 5. 上传字幕文件
	uploadedCount := 0
	for _, subtitleFile := range subtitleFiles {
		t.App.Logger.Infof("📝 正在上传字幕: %s", filepath.Base(subtitleFile.Path))

		err := uploader.UploadSubtitleWithLanDoc(bvid, subtitleFile.Path, subtitleFile.Language)
		if err != nil {
			// 79001: 当前语言已上传生效的字幕文件 (视为已存在，不算失败)
			if strings.Contains(err.Error(), "79001") || strings.Contains(err.Error(), "已上传生效") {
				t.App.Logger.Infof("ℹ️ 字幕已存在: %s (%s)", filepath.Base(subtitleFile.Path), subtitleFile.Language)
			} else {
				t.App.Logger.Errorf("❌ 上传字幕失败 %s: %v", subtitleFile.Path, err)
			}
			// 继续上传其他字幕文件，不因为一个失败就停止
			continue
		}

		t.App.Logger.Infof("✅ 字幕上传成功: %s (%s)", filepath.Base(subtitleFile.Path), subtitleFile.Language)
		uploadedCount++
	}

	// 6. 记录结果
	if uploadedCount > 0 {
		t.App.Logger.Info("========================================")
		t.App.Logger.Infof("✅ 字幕上传完成！成功上传 %d 个字幕文件", uploadedCount)
		t.App.Logger.Infof("  视频链接: https://www.bilibili.com/video/%s", bvid)
		t.App.Logger.Info("========================================")

		context["subtitle_upload_count"] = uploadedCount
		return true
	} else {
		t.App.Logger.Error("❌ 没有成功上传任何字幕文件")
		context["error"] = "字幕上传失败"
		return false
	}
}

// SubtitleFileInfo 字幕文件信息
type SubtitleFileInfo struct {
	Path     string
	Language string
}

// findSubtitleFiles 查找字幕文件
func (t *UploadSubtitleToBilibili) findSubtitleFiles() []SubtitleFileInfo {
	var subtitleFiles []SubtitleFileInfo

	// 1. 检查标准命名的字幕文件
	subtitleFilesToCheck := []struct {
		filename string
		language string
	}{
		{"zh.srt", "zh-Hans"},           // 中文简体
		{"zh_optimized.srt", "zh-Hans"}, // 中文简体 (备用)
		{"en.srt", "en"},                // 英文
	}

	for _, item := range subtitleFilesToCheck {
		fullPath := filepath.Join(t.StateManager.CurrentDir, item.filename)
		if _, err := os.Stat(fullPath); err == nil {
			subtitleFiles = append(subtitleFiles, SubtitleFileInfo{
				Path:     fullPath,
				Language: item.language,
			})
			t.App.Logger.Infof("🎯 找到字幕文件: %s (%s)", item.filename, item.language)
		}
	}

	// 2. 如果没找到标准命名，搜索 VideoID.语言.srt 格式的文件
	if len(subtitleFiles) == 0 {
		t.App.Logger.Info("📂 未找到标准命名字幕，搜索 VideoID.语言.srt 格式...")
		subtitleFiles = t.findSubtitleFilesByPattern()
	}

	return subtitleFiles
}

// findSubtitleFilesByPattern 按模式搜索字幕文件 (VideoID.语言.srt)
func (t *UploadSubtitleToBilibili) findSubtitleFilesByPattern() []SubtitleFileInfo {
	var subtitleFiles []SubtitleFileInfo

	// 语言映射表（映射到 B站 API 接受的语言代码）
	// B站字幕 API 使用 zh-Hans 格式
	languageMap := map[string]string{
		"zh-Hans": "zh-Hans", // 中文简体
		"zh-Hant": "zh-Hant", // 中文繁体
		"zh":      "zh-Hans", // 中文默认简体
		"zh-CN":   "zh-Hans", // 中文简体
		"zh-TW":   "zh-Hant", // 中文繁体
		"en":      "en",      // 英文
		"ja":      "ja",      // 日文
		"ko":      "ko",      // 韩文
	}

	// 优先级顺序：中文 > 英文 > 其他
	priorityOrder := []string{"zh-Hans", "zh-Hant", "zh", "zh-CN", "zh-TW", "en", "ja", "ko"}

	entries, err := os.ReadDir(t.StateManager.CurrentDir)
	if err != nil {
		return subtitleFiles
	}

	// 收集所有字幕文件
	foundFiles := make(map[string]string) // language -> path

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// 匹配 VideoID.语言.srt 格式
		if filepath.Ext(name) != ".srt" {
			continue
		}

		// 检查是否包含语言标识
		for langCode, biliLang := range languageMap {
			pattern := "." + langCode + ".srt"
			if len(name) > len(pattern) && name[len(name)-len(pattern):] == pattern {
				fullPath := filepath.Join(t.StateManager.CurrentDir, name)
				foundFiles[biliLang] = fullPath
				t.App.Logger.Infof("🎯 找到字幕文件: %s (%s)", name, biliLang)
				break
			}
		}
	}

	// 按优先级添加字幕文件
	for _, lang := range priorityOrder {
		if path, exists := foundFiles[lang]; exists {
			subtitleFiles = append(subtitleFiles, SubtitleFileInfo{
				Path:     path,
				Language: lang,
			})
		}
	}

	return subtitleFiles
}

// getLoginInfo 获取 Bilibili 登录信息
// 优先级：多账号系统 > 旧单账号存储
func (t *UploadSubtitleToBilibili) getLoginInfo() (*bilibili.LoginInfo, error) {
	// 1. 优先从多账号系统获取
	if t.BiliAccountService != nil {
		loginInfo, err := t.BiliAccountService.GetGlobalLoginInfo()
		if err == nil && loginInfo != nil {
			t.App.Logger.Info("✓ 使用多账号系统的登录信息")
			return loginInfo, nil
		}
	}

	// 2. 回退到旧的单账号存储
	loginStore := storage.GetDefaultStore()
	if loginStore.IsValid() {
		loginInfo, err := loginStore.Load()
		if err == nil {
			t.App.Logger.Info("✓ 使用旧版单账号存储的登录信息")
			return loginInfo, nil
		}
	}

	return nil, fmt.Errorf("没有可用的登录信息")
}
