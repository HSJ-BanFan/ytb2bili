package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/difyz9/bilibili-go-sdk/bilibili"
	"github.com/difyz9/ytb2bili/internal/chain_task/base"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/storage"
	"github.com/difyz9/ytb2bili/pkg/cos"
	"github.com/difyz9/ytb2bili/pkg/utils"
)

// fetchAndSaveMetadata 尝试从 YouTube 获取元数据并保存到数据库
func (t *UploadToBilibili) fetchAndSaveMetadata(videoID string) error {
	t.App.Logger.Infof("🔄 尝试补充获取视频元数据: %s", videoID)

	// 1. 找到 yt-dlp
	var installDir string
	if t.App.Config != nil && t.App.Config.YtDlpPath != "" {
		installDir = t.App.Config.YtDlpPath
	}
	manager := utils.NewYtDlpManager(t.App.Logger, installDir)
	if !manager.IsInstalled() {
		return fmt.Errorf("未找到 yt-dlp")
	}
	ytdlpPath := manager.GetBinaryPath()

	// 2. 构建命令
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	command := []string{
		ytdlpPath,
		"--dump-json",
		"--no-download",
		videoURL,
	}

	// 添加 cookies 支持
	configDir := filepath.Dir(t.App.Config.Path)
	cookiesPath := filepath.Join(configDir, "cookies.txt")
	// 如果配置文件目录下的 cookies.txt 不存在，尝试当前目录
	if _, err := os.Stat(cookiesPath); err != nil {
		cookiesPath = "cookies.txt"
	}
	if _, err := os.Stat(cookiesPath); err == nil {
		absPath, _ := filepath.Abs(cookiesPath)
		command = append(command, "--cookies", absPath)
	}

	// 添加代理
	if t.App.Config != nil && t.App.Config.ProxyConfig != nil && t.App.Config.ProxyConfig.UseProxy && t.App.Config.ProxyConfig.ProxyHost != "" {
		command = append(command, "--proxy", t.App.Config.ProxyConfig.ProxyHost)
	}

	// 3. 执行命令
	cmd := exec.Command(command[0], command[1:]...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("执行 yt-dlp 失败: %v", err)
	}

	// 4. 解析 JSON
	var metadata VideoMetadataInfo
	if err := json.Unmarshal(output, &metadata); err != nil {
		return fmt.Errorf("解析元数据失败: %v", err)
	}

	// 5. 更新数据库
	savedVideo, err := t.SavedVideoService.GetVideoByVideoID(videoID)
	if err != nil {
		return fmt.Errorf("获取视频记录失败: %v", err)
	}

	savedVideo.Title = metadata.Title
	savedVideo.Description = metadata.Description
	// 如果需要，也可以更新其他字段

	if err := t.SavedVideoService.UpdateVideo(savedVideo); err != nil {
		return fmt.Errorf("更新数据库失败: %v", err)
	}

	t.App.Logger.Infof("✅ 成功补充获取并保存元数据: %s", metadata.Title)
	return nil
}

type UploadToBilibili struct {
	base.BaseTask
	App                *core.AppServer
	SavedVideoService  *services.SavedVideoService
	BiliAccountService *services.BiliAccountService
}

func NewUploadToBilibili(name string, app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient, savedVideoService *services.SavedVideoService, biliAccountService *services.BiliAccountService) *UploadToBilibili {
	return &UploadToBilibili{
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

func (t *UploadToBilibili) Execute(context map[string]interface{}) bool {
	t.App.Logger.Info("========================================")
	t.App.Logger.Info("开始上传视频到 Bilibili")
	t.App.Logger.Info("========================================")

	// 1. 查找下载的视频文件
	videoFiles := t.findVideoFiles()
	if len(videoFiles) == 0 {
		errMsg := "未找到视频文件"
		t.App.Logger.Error("❌ " + errMsg)
		context["error"] = errMsg
		return false
	}

	videoPath := videoFiles[0]
	t.App.Logger.Infof("📹 找到视频文件: %s", filepath.Base(videoPath))

	// 2. 获取所有启用的账号（企业版多账号上传）
	accounts := t.getAllEnabledAccounts()
	if len(accounts) == 0 {
		t.App.Logger.Error("❌ 没有可用的B站账号")
		context["error"] = "没有可用的B站账号"
		return false
	}

	t.App.Logger.Infof("📋 找到 %d 个启用的B站账号，开始多账号上传...", len(accounts))
	for i, acc := range accounts {
		t.App.Logger.Infof("   [%d] %s (MID: %d)", i+1, acc.Name, acc.Mid)
	}

	// 3. 并行上传到所有账号
	type uploadResult struct {
		AccountName string
		Mid         int64
		Success     bool
		BVID        string
		AID         int64
		Error       string
	}

	results := make([]uploadResult, len(accounts))
	var wg sync.WaitGroup

	for i, acc := range accounts {
		wg.Add(1)
		go func(idx int, account *storage.BiliAccount) {
			defer wg.Done()

			result := uploadResult{
				AccountName: account.Name,
				Mid:         account.Mid,
			}

			t.App.Logger.Infof("⏫ [%s] 开始上传视频...", account.Name)

			// 上传到该账号
			bvid, aid, err := t.uploadToAccount(account, videoPath, context)
			if err != nil {
				result.Success = false
				result.Error = err.Error()
				t.App.Logger.Errorf("❌ [%s] 上传失败: %v", account.Name, err)
			} else {
				result.Success = true
				result.BVID = bvid
				result.AID = aid
				t.App.Logger.Infof("✅ [%s] 上传成功! BVID: %s", account.Name, bvid)
			}

			results[idx] = result
		}(i, acc)
	}

	wg.Wait()

	// 4. 统计上传结果
	successCount := 0
	var firstBVID string
	var firstAID int64
	var allBVIDs []string

	// 构建前端可用的上传结果
	type AccountUploadResult struct {
		AccountName string `json:"account_name"`
		Mid         int64  `json:"mid"`
		Success     bool   `json:"success"`
		BVID        string `json:"bvid,omitempty"`
		AID         int64  `json:"aid,omitempty"`
		Error       string `json:"error,omitempty"`
		VideoURL    string `json:"video_url,omitempty"`
	}
	uploadResults := make([]AccountUploadResult, 0, len(results))

	t.App.Logger.Info("")
	t.App.Logger.Info("╔══════════════════════════════════════════════════════════════╗")
	t.App.Logger.Info("║                    📊 多账号上传结果                          ║")
	t.App.Logger.Info("╠══════════════════════════════════════════════════════════════╣")

	for _, r := range results {
		result := AccountUploadResult{
			AccountName: r.AccountName,
			Mid:         r.Mid,
			Success:     r.Success,
			BVID:        r.BVID,
			AID:         r.AID,
			Error:       r.Error,
		}
		if r.Success {
			successCount++
			if firstBVID == "" {
				firstBVID = r.BVID
				firstAID = r.AID
			}
			allBVIDs = append(allBVIDs, r.BVID)
			result.VideoURL = fmt.Sprintf("https://www.bilibili.com/video/%s", r.BVID)
			t.App.Logger.Infof("║  ✅ %s (MID: %d) - BVID: %s", r.AccountName, r.Mid, r.BVID)
		} else {
			t.App.Logger.Infof("║  ❌ %s (MID: %d) - 错误: %s", r.AccountName, r.Mid, r.Error)
		}
		uploadResults = append(uploadResults, result)
	}

	t.App.Logger.Infof("╠══════════════════════════════════════════════════════════════╣")
	t.App.Logger.Infof("║  成功: %d/%d 个账号", successCount, len(accounts))
	t.App.Logger.Info("╚══════════════════════════════════════════════════════════════╝")

	if successCount > 0 {
		savedVideo, err := t.SavedVideoService.GetVideoByVideoID(t.StateManager.VideoID)
		if err == nil {
			savedVideo.BiliBVID = firstBVID
			savedVideo.BiliAID = firstAID
			// 保存所有BVID到扩展字段（JSON格式）
			if len(allBVIDs) > 1 {
				bvidsJSON, _ := json.Marshal(allBVIDs)
				savedVideo.BiliMultiBVIDs = string(bvidsJSON)
			}

			// 设置字幕计划上传时间（根据视频大小智能计算延迟）
			videoSizeMB := t.getVideoSizeMB()
			savedVideo.VideoSizeMB = videoSizeMB

			// 检查是否有字幕
			hasSubtitle := savedVideo.Subtitles != "" && savedVideo.Subtitles != "[]" && savedVideo.Subtitles != "null"

			if hasSubtitle {
				subtitleDelay := t.calculateSubtitleDelay(videoSizeMB)
				scheduledTime := time.Now().Add(subtitleDelay)
				savedVideo.SubtitleScheduledAt = &scheduledTime
				savedVideo.SubtitleUploadRetries = 0
				savedVideo.SubtitleUploadError = ""
			}

			if err := t.SavedVideoService.UpdateVideo(savedVideo); err != nil {
				t.App.Logger.Errorf("❌ 保存上传结果到数据库失败: %v", err)
			} else {
				t.App.Logger.Info("✅ 上传结果已保存到数据库")
				if hasSubtitle {
					t.App.Logger.Infof("🕒 字幕将在 %s 后上传 (计划时间: %s)",
						t.calculateSubtitleDelay(videoSizeMB).Round(time.Minute),
						savedVideo.SubtitleScheduledAt.Format("15:04:05"))
				} else {
					t.App.Logger.Info("✓ 视频无字幕，不计划字幕上传")
				}
			}
		}

		context["bili_bvid"] = firstBVID
		context["bili_aid"] = firstAID
		context["bili_all_bvids"] = allBVIDs
	}

	// 保存上传结果到context，供任务步骤记录使用
	// 直接存储对象，避免双重JSON序列化
	context["result_data"] = map[string]interface{}{
		"total_accounts":  len(accounts),
		"success_count":   successCount,
		"primary_bvid":    firstBVID,
		"primary_aid":     firstAID,
		"account_results": uploadResults,
	}

	// 至少有一个账号上传成功就算成功
	if successCount > 0 {
		return true
	}

	// 所有账号都失败，收集错误信息
	var errors []string
	for _, r := range results {
		if !r.Success && r.Error != "" {
			errors = append(errors, fmt.Sprintf("[%s] %s", r.AccountName, r.Error))
		}
	}
	if len(errors) > 0 {
		context["error"] = fmt.Sprintf("所有账号上传失败: %s", strings.Join(errors, "; "))
	} else {
		context["error"] = "所有账号上传失败"
	}
	return false
}

// uploadToAccount 上传视频到指定账号
func (t *UploadToBilibili) uploadToAccount(account *storage.BiliAccount, videoPath string, context map[string]interface{}) (string, int64, error) {
	if account.LoginInfo == nil {
		return "", 0, fmt.Errorf("账号 %s 没有登录信息", account.Name)
	}

	// 获取视频文件信息
	fileInfo, err := os.Stat(videoPath)
	if err != nil {
		return "", 0, fmt.Errorf("获取视频文件信息失败: %v", err)
	}
	fileSizeMB := float64(fileInfo.Size()) / 1024 / 1024
	t.App.Logger.Infof("📦 视频文件: %s (%.2f MB)", filepath.Base(videoPath), fileSizeMB)

	// 预估上传时间（假设平均上传速度 2MB/s）
	estimatedSeconds := int(fileSizeMB / 2)
	if estimatedSeconds < 10 {
		estimatedSeconds = 10
	}
	t.App.Logger.Infof("⏳ 预计上传时间: %d 秒 (取决于网络速度)", estimatedSeconds)

	// 创建上传客户端
	uploadClient := bilibili.NewUploadClient(account.LoginInfo)

	// 上传视频文件
	t.App.Logger.Info("🚀 开始上传视频到 B站...")
	startTime := time.Now()
	video, err := uploadClient.UploadVideo(videoPath)
	uploadDuration := time.Since(startTime)
	if err != nil {
		return "", 0, fmt.Errorf("上传视频失败: %v", err)
	}

	// 计算实际上传速度
	actualSpeedMBps := fileSizeMB / uploadDuration.Seconds()
	t.App.Logger.Infof("✅ 视频上传完成! 耗时: %s, 平均速度: %.2f MB/s", uploadDuration.Round(time.Second), actualSpeedMBps)

	// 准备投稿信息（传入当前账号用于封面上传）
	studio := t.buildStudioInfo(video, context, account)

	// 提交视频
	result, err := uploadClient.SubmitVideo(studio)
	if err != nil {
		return "", 0, fmt.Errorf("提交视频失败: %v", err)
	}

	if result.Code != 0 {
		return "", 0, fmt.Errorf("提交失败: code=%d, message=%s", result.Code, result.Message)
	}

	// 解析BVID和AID
	var bvid string
	var aid int64
	if result.Data != nil {
		if dataMap, ok := result.Data.(map[string]interface{}); ok {
			if bvidVal, exists := dataMap["bvid"]; exists {
				if bvidStr, ok := bvidVal.(string); ok {
					bvid = bvidStr
				}
			}
			if aidVal, exists := dataMap["aid"]; exists {
				if aidFloat, ok := aidVal.(float64); ok {
					aid = int64(aidFloat)
				}
			}
		}
	}

	// 注意：字幕上传已改为延迟上传机制
	// 视频上传成功后会设置 SubtitleScheduledAt，由调度器在审核通过后自动上传
	// 不再在此处立即上传字幕，避免权限不足错误

	return bvid, aid, nil
}

// getAllEnabledAccounts 获取当前视频所属用户的所有启用B站账号
// ⚠️ 重要：确保账号隔离，只返回当前用户绑定的账号
func (t *UploadToBilibili) getAllEnabledAccounts() []*storage.BiliAccount {
	var accounts []*storage.BiliAccount

	// 1. 获取当前视频所属的用户ID
	userID := t.StateManager.UserID
	if userID == 0 && t.StateManager.VideoID != "" {
		// 如果 StateManager 没有 UserID，从数据库查询
		savedVideo, err := t.SavedVideoService.GetVideoByVideoID(t.StateManager.VideoID)
		if err == nil && savedVideo.UserID > 0 {
			userID = savedVideo.UserID
		}
	}

	if userID == 0 {
		t.App.Logger.Warn("⚠️ 无法确定视频所属用户，将不使用任何账号")
		return accounts
	}

	t.App.Logger.Infof("🔍 获取用户 %d 的 B 站账号...", userID)

	// 2. 从数据库获取该用户的所有启用账号
	if t.BiliAccountService != nil {
		dbAccounts, err := t.BiliAccountService.GetEnabledAccountsForUser(userID)
		if err == nil && len(dbAccounts) > 0 {
			t.App.Logger.Infof("📦 从数据库获取到用户 %d 的 %d 个启用账号", userID, len(dbAccounts))
			for _, dbAccount := range dbAccounts {
				// 解析 cookies 中的 LoginInfo
				var loginInfo *bilibili.LoginInfo
				if dbAccount.Cookies != "" {
					if err := json.Unmarshal([]byte(dbAccount.Cookies), &loginInfo); err != nil {
						t.App.Logger.Warnf("解析账号 %s 的登录信息失败: %v", dbAccount.BiliName, err)
						continue
					}
				}
				if loginInfo == nil {
					t.App.Logger.Warnf("账号 %s 没有有效的登录信息", dbAccount.BiliName)
					continue
				}
				account := &storage.BiliAccount{
					ID:        fmt.Sprintf("%d", dbAccount.BiliMid),
					Mid:       dbAccount.BiliMid,
					Name:      dbAccount.BiliName,
					Face:      dbAccount.BiliFace,
					IsEnabled: dbAccount.IsEnabled,
					IsPrimary: dbAccount.IsPrimary,
					LoginInfo: loginInfo,
				}
				accounts = append(accounts, account)
			}
			if len(accounts) > 0 {
				return accounts
			}
		} else if err != nil {
			t.App.Logger.Warnf("获取用户 %d 的账号失败: %v", userID, err)
		} else {
			t.App.Logger.Warnf("用户 %d 没有绑定任何 B 站账号", userID)
		}
	}

	// 3. 如果用户没有绑定账号，返回空列表（不再回退到全局账号）
	t.App.Logger.Warn("⚠️ 用户没有绑定有效的 B 站账号，无法上传")
	return accounts
}

// getLoginInfo 获取登录信息（支持多用户）
func (t *UploadToBilibili) getLoginInfo(context map[string]interface{}) (*bilibili.LoginInfo, error) {
	t.App.Logger.Info("🔍 开始获取B站登录信息...")
	t.App.Logger.Infof("   BiliAccountService 是否存在: %v", t.BiliAccountService != nil)

	// 1. 尝试从 context 获取用户ID
	var userID uint
	if uid, ok := context["user_id"]; ok {
		switch v := uid.(type) {
		case uint:
			userID = v
		case int:
			userID = uint(v)
		case float64:
			userID = uint(v)
		}
	}
	t.App.Logger.Infof("   从 context 获取的用户ID: %d", userID)

	// 2. 如果有用户ID且有账号服务，尝试获取用户的B站账号
	if userID > 0 && t.BiliAccountService != nil {
		loginInfo, account, err := t.BiliAccountService.GetLoginInfoForUser(userID)
		if err == nil {
			t.App.Logger.Infof("✓ 使用用户 %d 的B站账号: %s (MID: %d)", userID, account.BiliName, account.BiliMid)
			// 更新最后使用时间
			t.BiliAccountService.UpdateLastUsed(account.ID)
			return loginInfo, nil
		}
		t.App.Logger.Warnf("⚠️ 获取用户 %d 的B站账号失败: %v，尝试使用全局账号", userID, err)
	}

	// 3. 尝试从视频记录获取用户ID
	if userID == 0 && t.StateManager != nil && t.StateManager.VideoID != "" {
		t.App.Logger.Infof("   尝试从视频记录获取用户ID, VideoID: %s", t.StateManager.VideoID)
		savedVideo, err := t.SavedVideoService.GetVideoByVideoID(t.StateManager.VideoID)
		if err == nil && savedVideo.UserID > 0 {
			userID = savedVideo.UserID
			t.App.Logger.Infof("   从视频记录获取的用户ID: %d", userID)
			if t.BiliAccountService != nil {
				loginInfo, account, err := t.BiliAccountService.GetLoginInfoForUser(userID)
				if err == nil {
					t.App.Logger.Infof("✓ 使用视频所属用户 %d 的B站账号: %s", userID, account.BiliName)
					t.BiliAccountService.UpdateLastUsed(account.ID)
					return loginInfo, nil
				}
				t.App.Logger.Warnf("   获取用户 %d 的B站账号失败: %v", userID, err)
			}
		} else if err != nil {
			t.App.Logger.Warnf("   获取视频记录失败: %v", err)
		}
	}

	// 4. 回退到全局账号服务
	t.App.Logger.Info("   尝试获取全局B站账号...")
	if t.BiliAccountService != nil {
		loginInfo, err := t.BiliAccountService.GetGlobalLoginInfo()
		if err == nil {
			t.App.Logger.Infof("✓ 使用全局B站账号 (MID: %d)", loginInfo.TokenInfo.Mid)
			return loginInfo, nil
		}
		t.App.Logger.Warnf("   获取全局账号失败: %v", err)
	}

	// 5. 最后回退到旧的单账号存储
	t.App.Logger.Info("   尝试使用旧版单账号存储...")
	loginStore := storage.GetDefaultStore()
	if !loginStore.IsValid() {
		t.App.Logger.Error("   旧版存储无效")
		return nil, fmt.Errorf("没有有效的 Bilibili 登录信息，请先绑定B站账号")
	}

	loginInfo, err := loginStore.Load()
	if err != nil {
		t.App.Logger.Errorf("   加载旧版存储失败: %v", err)
		return nil, fmt.Errorf("加载登录信息失败: %v", err)
	}

	t.App.Logger.Infof("✓ 使用旧版单账号存储的登录信息 (MID: %d)", loginInfo.TokenInfo.Mid)
	return loginInfo, nil
}

// findVideoFiles 查找下载目录中的视频文件
func (t *UploadToBilibili) findVideoFiles() []string {
	var videoFiles []string
	videoExtensions := []string{".mp4", ".flv", ".mkv", ".webm", ".avi", ".mov"}

	files, err := os.ReadDir(t.StateManager.CurrentDir)
	if err != nil {
		t.App.Logger.Errorf("读取目录失败: %v", err)
		return videoFiles
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(file.Name()))
		for _, videoExt := range videoExtensions {
			if ext == videoExt {
				fullPath := filepath.Join(t.StateManager.CurrentDir, file.Name())
				videoFiles = append(videoFiles, fullPath)
				break
			}
		}
	}

	return videoFiles
}

// buildStudioInfo 构建投稿信息
// account 参数用于上传封面时使用正确的账号登录信息
func (t *UploadToBilibili) buildStudioInfo(video *bilibili.Video, context map[string]interface{}, account *storage.BiliAccount) *bilibili.Studio {
	// 默认值
	title := t.StateManager.VideoID
	desc := "自动上传的视频"
	tags := "视频"
	coverURL := "" // 封面URL

	// 从数据库查询视频的标题和描述信息
	savedVideo, err := t.SavedVideoService.GetVideoByVideoID(t.StateManager.VideoID)
	if err != nil {
		t.App.Logger.Warnf("⚠️ 无法从数据库获取视频信息: %v，将使用默认值", err)
	} else {
		// 如果标题为空，尝试补充获取元数据
		if savedVideo.Title == "" {
			if err := t.fetchAndSaveMetadata(t.StateManager.VideoID); err == nil {
				// 重新获取
				savedVideo, _ = t.SavedVideoService.GetVideoByVideoID(t.StateManager.VideoID)
			} else {
				t.App.Logger.Warnf("⚠️ 补充获取元数据失败: %v", err)
			}
		}

		// 清理标题中的标签（#hashtag）
		cleanTitle := func(title string) string {
			// 使用正则表达式移除 #标签
			re := regexp.MustCompile(`\s*#[^\s#]+`)
			cleaned := re.ReplaceAllString(title, "")
			// 清理多余的空格
			cleaned = strings.TrimSpace(cleaned)
			// 将多个连续空格替换为单个空格
			re2 := regexp.MustCompile(`\s+`)
			cleaned = re2.ReplaceAllString(cleaned, " ")
			return cleaned
		}

		// 根据配置选择标题来源
		biliConfig := t.App.Config.BilibiliConfig

		// 优先使用 upload_* 字段（由"确认元数据"任务设置）
		if savedVideo.UploadTitle != "" {
			title = savedVideo.UploadTitle
			sourceLabel := ""
			switch savedVideo.MetadataSource {
			case "ai_generated":
				sourceLabel = "✨ AI生成"
			case "user_edited":
				sourceLabel = "📝 用户编辑"
			default:
				sourceLabel = "📹 原始"
			}
			t.App.Logger.Infof("✓ 使用预设标题 [%s]: %s", sourceLabel, title)
		} else if biliConfig != nil && biliConfig.CustomTitleTemplate != "" {
			// 回退到旧逻辑：使用自定义标题模板（兼容未执行"确认元数据"的情况）
			title = biliConfig.CustomTitleTemplate
			// 清理原标题中的标签
			cleanedOriginalTitle := cleanTitle(savedVideo.Title)
			title = strings.ReplaceAll(title, "{original_title}", cleanedOriginalTitle)
			title = strings.ReplaceAll(title, "{ai_title}", savedVideo.GeneratedTitle)
			t.App.Logger.Infof("✓ 使用自定义标题模板: %s", title)
		} else if biliConfig != nil && !biliConfig.UseOriginalTitle {
			// 配置为使用AI生成标题
			if savedVideo.GeneratedTitle != "" {
				title = savedVideo.GeneratedTitle
				t.App.Logger.Infof("✓ 使用AI生成的标题: %s", title)
			} else if savedVideo.Title != "" {
				title = cleanTitle(savedVideo.Title)
				t.App.Logger.Infof("✓ AI标题不存在，回退使用原始标题（已清理标签）: %s", title)
			}
		} else {
			// 默认使用原始标题（YouTube原标题）
			if savedVideo.Title != "" {
				title = cleanTitle(savedVideo.Title)
				t.App.Logger.Infof("✓ 使用YouTube原始标题（已清理标签）: %s", title)
			} else if savedVideo.GeneratedTitle != "" {
				title = savedVideo.GeneratedTitle
				t.App.Logger.Infof("✓ 原始标题不存在，回退使用AI标题: %s", title)
			}
		}

		// B站标题长度限制（80个字符）
		const maxTitleLength = 80
		titleRunes := []rune(title)
		if len(titleRunes) > maxTitleLength {
			title = string(titleRunes[:maxTitleLength])
			t.App.Logger.Warnf("⚠️ 标题过长，已截断至 %d 字符: %s", maxTitleLength, title)
		}
		t.App.Logger.Infof("📝 标题长度: %d/%d 字符", len([]rune(title)), maxTitleLength)

		// 过滤无效的描述（YouTube的默认描述）
		isValidDescription := func(desc string) bool {
			if desc == "" {
				return false
			}
			// 过滤YouTube的默认描述
			invalidDescriptions := []string{
				"YouTube",
				"自动上传的视频",
				"Uploaded by",
				"Auto-generated",
			}
			for _, invalid := range invalidDescriptions {
				if strings.Contains(desc, invalid) && len(desc) < 50 {
					return false
				}
			}
			return true
		}

		// 根据配置选择描述来源
		// 优先使用 upload_desc 字段（由"确认元数据"任务设置）
		if savedVideo.UploadDesc != "" {
			desc = savedVideo.UploadDesc
			sourceLabel := ""
			switch savedVideo.MetadataSource {
			case "ai_generated":
				sourceLabel = "✨ AI生成"
			case "user_edited":
				sourceLabel = "📝 用户编辑"
			default:
				sourceLabel = "📹 原始"
			}
			t.App.Logger.Infof("✓ 使用预设描述 [%s]", sourceLabel)
		} else if biliConfig != nil && biliConfig.CustomDescTemplate != "" {
			// 回退到旧逻辑：使用自定义模板（兼容未执行"确认元数据"的情况）
			desc = biliConfig.CustomDescTemplate
			desc = strings.ReplaceAll(desc, "{original_desc}", savedVideo.Description)
			desc = strings.ReplaceAll(desc, "{ai_desc}", savedVideo.GeneratedDesc)
			t.App.Logger.Infof("✓ 使用自定义描述模板")
		} else if biliConfig != nil && biliConfig.UseOriginalDesc {
			// 配置为使用原始描述
			if isValidDescription(savedVideo.Description) {
				desc = savedVideo.Description
				t.App.Logger.Infof("✓ 使用YouTube原始描述")
			} else if savedVideo.GeneratedDesc != "" {
				desc = savedVideo.GeneratedDesc
				t.App.Logger.Infof("✓ 原始描述无效，回退使用AI描述")
			} else {
				desc = ""
				t.App.Logger.Info("✓ 无有效描述，仅使用原视频链接")
			}
		} else {
			// 默认使用AI生成的描述 + 原视频简介
			aiIntro := ""
			originalDesc := ""

			// 获取AI生成的精炼介绍（100字以内）
			if savedVideo.GeneratedDesc != "" {
				aiIntro = savedVideo.GeneratedDesc
				t.App.Logger.Infof("✓ AI生成的精炼介绍: %s", aiIntro)
			}

			// 获取原视频简介
			if isValidDescription(savedVideo.Description) {
				originalDesc = savedVideo.Description
				t.App.Logger.Infof("✓ 原视频简介长度: %d 字符", len([]rune(originalDesc)))
			}

			// 拼接描述：AI介绍 + 分隔线 + 原视频简介
			if aiIntro != "" && originalDesc != "" {
				desc = fmt.Sprintf("%s\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n📄 原视频简介：\n%s", aiIntro, originalDesc)
				t.App.Logger.Info("✓ 使用AI介绍 + 原视频简介")
			} else if aiIntro != "" {
				desc = aiIntro
				t.App.Logger.Info("✓ 仅使用AI介绍")
			} else if originalDesc != "" {
				desc = originalDesc
				t.App.Logger.Info("✓ 仅使用原视频简介")
			} else {
				desc = ""
				t.App.Logger.Info("✓ 无有效描述，仅使用原视频链接")
			}
		}

		// 使用标签（优先使用 upload_tags）
		if savedVideo.UploadTags != "" {
			tags = savedVideo.UploadTags
			t.App.Logger.Infof("✓ 使用预设标签: %s", tags)
		} else if savedVideo.GeneratedTags != "" {
			tags = savedVideo.GeneratedTags
			t.App.Logger.Infof("✓ 使用AI生成的标签: %s", tags)
		}

		// B站简介字数限制（2000字）
		const maxDescLength = 2000

		// 在描述末尾添加原视频链接
		linkSuffix := ""
		if savedVideo.URL != "" {
			linkSuffix = fmt.Sprintf("\n\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n📺 原视频链接：%s\n🔄 本视频为转载内容，仅供学习交流使用", savedVideo.URL)
		}

		// 计算链接后缀的长度（字符数）
		linkSuffixLength := len([]rune(linkSuffix))
		t.App.Logger.Infof("🔗 原视频链接后缀长度: %d 字符", linkSuffixLength)

		// 预先截断描述，确保有足够空间给链接
		descRunes := []rune(desc)
		originalDescLength := len(descRunes)
		t.App.Logger.Infof("📄 原始描述长度: %d 字符", originalDescLength)

		// 计算可用的描述长度（留20个字符的安全缓冲）
		maxAllowedDescLength := maxDescLength - linkSuffixLength - 20
		if maxAllowedDescLength < 0 {
			maxAllowedDescLength = 0
		}

		// 如果描述超过可用长度，截断它
		if len(descRunes) > maxAllowedDescLength {
			if maxAllowedDescLength > 3 {
				desc = string(descRunes[:maxAllowedDescLength]) + "..."
				t.App.Logger.Warnf("⚠️ 描述过长，已截断至 %d 字符（原长度: %d）", maxAllowedDescLength, originalDescLength)
			} else {
				desc = ""
				t.App.Logger.Warn("⚠️ 空间不足，已清空描述内容，仅保留原视频链接")
			}
		}

		// 添加链接后缀
		if linkSuffix != "" {
			desc += linkSuffix
			t.App.Logger.Infof("✓ 已添加原视频链接到描述")
		}

		// 最终检查长度
		finalDescLength := len([]rune(desc))
		t.App.Logger.Infof("📝 最终描述长度: %d/%d 字符", finalDescLength, maxDescLength)

		// 最后的安全检查，如果还是超长，强制截断
		if finalDescLength > maxDescLength {
			desc = string([]rune(desc)[:maxDescLength])
			t.App.Logger.Errorf("❌ 描述仍然超长！强制截断至 %d 字符", maxDescLength)
		}
	}

	// 从 context 获取下载的封面图片并上传作为封面
	t.App.Logger.Infof("📸 检查 context 中的封面路径...")
	t.App.Logger.Infof("📸 context keys: %v", func() []string {
		keys := make([]string, 0, len(context))
		for k := range context {
			keys = append(keys, k)
		}
		return keys
	}())
	if coverImagePath, ok := context["cover_image_path"].(string); ok && coverImagePath != "" {
		t.App.Logger.Infof("📸 找到封面图片: %s (完整路径: %s)", filepath.Base(coverImagePath), coverImagePath)

		// 使用当前账号的登录信息上传封面
		var loginInfo *bilibili.LoginInfo
		if account != nil && account.LoginInfo != nil {
			loginInfo = account.LoginInfo
			t.App.Logger.Infof("📸 使用账号 %s 上传封面", account.Name)
		} else {
			// 回退到旧存储
			loginStore := storage.GetDefaultStore()
			var err error
			loginInfo, err = loginStore.Load()
			if err != nil {
				t.App.Logger.Errorf("❌ 获取登录信息失败，无法上传封面: %v", err)
			}
		}

		if loginInfo != nil {
			uploadClient := bilibili.NewUploadClient(loginInfo)
			uploadedCoverURL, err := uploadClient.UploadCover(coverImagePath)
			if err != nil {
				t.App.Logger.Errorf("❌ 上传封面失败: %v", err)
			} else {
				coverURL = uploadedCoverURL
				t.App.Logger.Infof("✓ 封面上传成功: %s", coverURL)
			}
		}
	} else {
		t.App.Logger.Warn("⚠️ context 中没有 cover_image_path，将不设置封面")
	}

	// 检查是否有中文字幕
	zhSRTPath := filepath.Join(t.StateManager.CurrentDir, "zh.srt")
	hasZhSubtitle := false
	if _, err := os.Stat(zhSRTPath); err == nil {
		hasZhSubtitle = true
		t.App.Logger.Info("✓ 检测到中文字幕文件")
	}

	// 更新video对象的Title为翻译后的标题
	video.Title = title
	t.App.Logger.Infof("✓ 设置视频Title为: %s", title)

	// 读取配置
	copyright := 1 // 默认自制
	noReprint := 1 // 默认禁止转载
	source := ""
	tid := 122                   // 默认分区
	dynamic := "发布了新视频！"         // 默认动态
	openElec := 0                // 默认关闭充电
	selectionReserve := int64(0) // 默认不参与活动
	upSelectionReply := 0        // 默认不展示推荐评论
	upCloseReply := 0            // 默认开启评论
	upCloseReward := 0           // 默认开启打赏

	if t.App.Config.BilibiliConfig != nil {
		if t.App.Config.BilibiliConfig.Copyright > 0 {
			copyright = t.App.Config.BilibiliConfig.Copyright
		}
		noReprint = t.App.Config.BilibiliConfig.NoReprint
		source = t.App.Config.BilibiliConfig.Source

		// 读取新增配置
		if t.App.Config.BilibiliConfig.Tid > 0 {
			tid = t.App.Config.BilibiliConfig.Tid
		}
		if t.App.Config.BilibiliConfig.Dynamic != "" {
			dynamic = t.App.Config.BilibiliConfig.Dynamic
		}
		openElec = t.App.Config.BilibiliConfig.OpenElec
		selectionReserve = t.App.Config.BilibiliConfig.SelectionReserve
		upSelectionReply = t.App.Config.BilibiliConfig.UpSelectionReply
		upCloseReply = t.App.Config.BilibiliConfig.UpCloseReply
		upCloseReward = t.App.Config.BilibiliConfig.UpCloseReward
	}

	// 如果是转载且没有提供来源，使用视频URL作为来源
	if copyright == 2 && source == "" {
		if savedVideo != nil {
			source = savedVideo.URL
		} else {
			// 如果无法获取URL，构建一个默认的YouTube URL
			source = fmt.Sprintf("https://www.youtube.com/watch?v=%s", t.StateManager.VideoID)
		}
	}

	studio := &bilibili.Studio{
		Copyright:     copyright,
		Title:         t.truncateTitle(title, 80), // B站标题最长80字符
		Desc:          desc,
		Tag:           tags,
		Tid:           tid,
		Cover:         coverURL, // 使用上传的封面URL
		Dynamic:       dynamic,
		OpenSubtitle:  hasZhSubtitle, // 如果有中文字幕则开启
		Interactive:   0,
		Dolby:         0,
		LosslessMusic: 0,
		NoReprint:     noReprint,
		OpenElec:      openElec,
		Videos: []bilibili.Video{
			*video,
		},
		Source: source,
	}

	// 记录暂不支持的高级配置（需要SDK更新）
	if selectionReserve > 0 {
		t.App.Logger.Warnf("⚠️ 参与活动功能(selection_reserve=%d)暂不被SDK支持，已忽略", selectionReserve)
	}
	if upSelectionReply > 0 {
		t.App.Logger.Warnf("⚠️ 推荐评论功能(up_selection_reply=%d)暂不被SDK支持，已忽略", upSelectionReply)
	}
	if upCloseReply > 0 {
		t.App.Logger.Warnf("⚠️ 关闭评论功能(up_close_reply=%d)暂不被SDK支持，已忽略", upCloseReply)
	}
	if upCloseReward > 0 {
		t.App.Logger.Warnf("⚠️ 关闭打赏功能(up_close_reward=%d)暂不被SDK支持，已忽略", upCloseReward)
	}

	t.App.Logger.Infof("📋 投稿信息:")
	t.App.Logger.Infof("  标题: %s", studio.Title)
	t.App.Logger.Infof("  简介: %s", t.truncateString(studio.Desc, 100))
	t.App.Logger.Infof("  标签: %s", studio.Tag)
	t.App.Logger.Infof("  分区: %d", studio.Tid)
	t.App.Logger.Infof("  封面: %s", studio.Cover)
	t.App.Logger.Infof("  字幕: %v", studio.OpenSubtitle)
	t.App.Logger.Infof("  类型: %d (1=自制, 2=转载)", studio.Copyright)
	if studio.Copyright == 2 {
		t.App.Logger.Infof("  来源: %s", studio.Source)
	}

	return studio
}

// truncateString 截断字符串用于日志显示
func (t *UploadToBilibili) truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// truncateTitle 截断标题到指定长度
func (t *UploadToBilibili) truncateTitle(title string, maxLen int) string {
	runes := []rune(title)
	if len(runes) <= maxLen {
		return title
	}
	return string(runes[:maxLen-3]) + "..."
}

// getUserFriendlyError 将技术错误转换为用户友好的错误信息
func (t *UploadToBilibili) getUserFriendlyError(err error, operation string) string {
	errorStr := err.Error()

	// 网络相关错误
	if strings.Contains(errorStr, "broken pipe") || strings.Contains(errorStr, "connection reset") {
		return fmt.Sprintf("%s失败：网络连接中断，请检查网络状态后重试", operation)
	}

	if strings.Contains(errorStr, "timeout") || strings.Contains(errorStr, "deadline exceeded") {
		return fmt.Sprintf("%s失败：网络超时，请稍后重试", operation)
	}

	if strings.Contains(errorStr, "connection refused") {
		return fmt.Sprintf("%s失败：无法连接到B站服务器，请检查网络连接", operation)
	}

	if strings.Contains(errorStr, "no such host") || strings.Contains(errorStr, "dns") {
		return fmt.Sprintf("%s失败：网络域名解析失败，请检查网络设置", operation)
	}

	// 文件相关错误
	if strings.Contains(errorStr, "no such file") || strings.Contains(errorStr, "file not found") {
		return fmt.Sprintf("%s失败：找不到视频文件，请确认文件已正确下载", operation)
	}

	if strings.Contains(errorStr, "permission denied") {
		return fmt.Sprintf("%s失败：文件访问权限不足", operation)
	}

	if strings.Contains(errorStr, "file too large") {
		return fmt.Sprintf("%s失败：文件过大，超出B站上传限制", operation)
	}

	// B站API相关错误
	if strings.Contains(errorStr, "401") || strings.Contains(errorStr, "unauthorized") {
		return fmt.Sprintf("%s失败：登录状态已过期，请重新登录", operation)
	}

	if strings.Contains(errorStr, "403") || strings.Contains(errorStr, "forbidden") {
		return fmt.Sprintf("%s失败：账号权限不足或被限制", operation)
	}

	if strings.Contains(errorStr, "429") || strings.Contains(errorStr, "rate limit") {
		return fmt.Sprintf("%s失败：操作频率过快，请稍后再试", operation)
	}

	if strings.Contains(errorStr, "500") || strings.Contains(errorStr, "internal server error") {
		return fmt.Sprintf("%s失败：B站服务器临时异常，请稍后重试", operation)
	}

	if strings.Contains(errorStr, "upload chunks") {
		return fmt.Sprintf("%s失败：视频分片上传中断，可能是网络不稳定导致，请重试", operation)
	}

	// 通用错误处理
	if strings.Contains(errorStr, "failed to") {
		return fmt.Sprintf("%s失败：操作执行失败，请稍后重试", operation)
	}

	// 如果是未知错误，返回简化的错误信息
	return fmt.Sprintf("%s失败：发生未知错误，请重试或联系技术支持", operation)
}

// uploadSubtitles 上传字幕到 B站
// 支持上传中文字幕(zh.srt)和英文字幕(en.srt)
func (t *UploadToBilibili) uploadSubtitles(account *storage.BiliAccount, bvid string) {
	t.App.Logger.Info("📝 开始上传字幕...")

	// 创建字幕上传器
	client := bilibili.NewClient()
	subtitleUploader := bilibili.NewSubtitleUploader(client, account.LoginInfo)

	// 查找字幕文件
	zhSRTPath := filepath.Join(t.StateManager.CurrentDir, "zh.srt")
	enSRTPath := filepath.Join(t.StateManager.CurrentDir, "en.srt")

	subtitlesUploaded := 0

	// 上传中文字幕
	if _, err := os.Stat(zhSRTPath); err == nil {
		t.App.Logger.Infof("  │ 上传中文字幕: zh.srt")
		if err := subtitleUploader.UploadSubtitle(bvid, zhSRTPath, "zh-Hans"); err != nil {
			t.App.Logger.Warnf("  │ ⚠️ 中文字幕上传失败: %v", err)
		} else {
			t.App.Logger.Info("  │ ✓ 中文字幕上传成功")
			subtitlesUploaded++
		}
	}

	// 上传英文字幕
	if _, err := os.Stat(enSRTPath); err == nil {
		t.App.Logger.Infof("  │ 上传英文字幕: en.srt")
		if err := subtitleUploader.UploadSubtitle(bvid, enSRTPath, "en"); err != nil {
			t.App.Logger.Warnf("  │ ⚠️ 英文字幕上传失败: %v", err)
		} else {
			t.App.Logger.Info("  │ ✓ 英文字幕上传成功")
			subtitlesUploaded++
		}
	}

	if subtitlesUploaded > 0 {
		t.App.Logger.Infof("📝 字幕上传完成: %d 个字幕已上传", subtitlesUploaded)
	} else {
		t.App.Logger.Info("📝 没有找到可上传的字幕文件")
	}
}

// getVideoSizeMB 获取当前视频文件大小（MB）
func (t *UploadToBilibili) getVideoSizeMB() float64 {
	videoPath := t.findVideoFile()
	if videoPath == "" {
		return 0
	}

	fileInfo, err := os.Stat(videoPath)
	if err != nil {
		return 0
	}

	return float64(fileInfo.Size()) / 1024 / 1024
}

// calculateSubtitleDelay 根据配置或视频大小计算字幕上传延迟时间
// 优先使用配置文件中的设置，未配置时根据视频大小智能计算
func (t *UploadToBilibili) calculateSubtitleDelay(videoSizeMB float64) time.Duration {
	// 1. 优先检查配置文件
	if t.App.Config != nil && t.App.Config.DownloadConfig != nil && t.App.Config.DownloadConfig.SubtitleUploadDelay > 0 {
		delay := time.Duration(t.App.Config.DownloadConfig.SubtitleUploadDelay) * time.Minute
		t.App.Logger.Infof("🕒 使用配置文件的字幕上传延迟: %d 分钟", t.App.Config.DownloadConfig.SubtitleUploadDelay)
		return delay
	}

	// 2. 如果未配置，根据视频大小设置延迟时间（B站审核时间与视频大小正相关）
	// 小视频 (<100MB): 10分钟
	// 中等视频 (100-300MB): 15分钟
	// 大视频 (300-500MB): 20分钟
	// 超大视频 (>500MB): 25分钟
	switch {
	case videoSizeMB <= 0:
		// 未获取到视频大小，使用默认值
		return 15 * time.Minute
	case videoSizeMB < 100:
		return 10 * time.Minute
	case videoSizeMB < 300:
		return 15 * time.Minute
	case videoSizeMB < 500:
		return 20 * time.Minute
	default:
		return 25 * time.Minute
	}
}

// findVideoFile 查找视频文件
func (t *UploadToBilibili) findVideoFile() string {
	videoExtensions := []string{".mp4", ".mkv", ".avi", ".mov", ".flv", ".webm"}

	entries, err := os.ReadDir(t.StateManager.CurrentDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		for _, videoExt := range videoExtensions {
			if ext == videoExt {
				return filepath.Join(t.StateManager.CurrentDir, entry.Name())
			}
		}
	}

	return ""
}
