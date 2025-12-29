package handlers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/difyz9/ytb2bili/internal/chain_task/base"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/pkg/cos"
	"github.com/difyz9/ytb2bili/pkg/store/model"
)

// 避免与其他文件的 truncateString 冲突
var _ = truncateStrForConfirm

// ConfirmMetadata 确认元数据任务
// 职责：根据配置决定最终上传使用的元数据（原始 vs AI生成）
type ConfirmMetadata struct {
	base.BaseTask
	App               *core.AppServer
	SavedVideoService *services.SavedVideoService
}

// NewConfirmMetadata 创建确认元数据任务
func NewConfirmMetadata(
	name string,
	app *core.AppServer,
	stateManager *manager.StateManager,
	client *cos.CosClient,
	savedVideoService *services.SavedVideoService,
) *ConfirmMetadata {
	return &ConfirmMetadata{
		BaseTask: base.BaseTask{
			Name:         name,
			StateManager: stateManager,
			Client:       client,
		},
		App:               app,
		SavedVideoService: savedVideoService,
	}
}

// Execute 执行确认元数据任务
func (t *ConfirmMetadata) Execute(ctx map[string]interface{}) bool {
	videoID := t.StateManager.VideoID
	t.App.Logger.Infof("🔍 确认元数据: %s", videoID)

	// 1. 获取视频记录
	savedVideo, err := t.SavedVideoService.GetVideoByVideoID(videoID)
	if err != nil {
		errMsg := fmt.Sprintf("获取视频记录失败: %v", err)
		t.App.Logger.Error("❌ " + errMsg)
		ctx["error"] = errMsg
		return false
	}

	// 2. 获取用户配置
	biliConfig := t.getBiliUploadConfig(savedVideo.UserID)

	// 3. 决定元数据来源
	metadataSource := "original" // 默认使用原始数据
	aiAttempted := false
	fallbackReason := ""

	// 检查是否有 AI 生成的数据
	hasAIData := savedVideo.GeneratedTitle != "" ||
		savedVideo.GeneratedDesc != "" ||
		savedVideo.GeneratedTags != ""

	// 检查配置是否使用原始标题
	useOriginalTitle := true
	if biliConfig != nil {
		// 尝试类型断言访问 UseOriginalTitle 字段
		if config, ok := biliConfig.(map[string]interface{}); ok {
			if v, exists := config["UseOriginalTitle"]; exists {
				if boolVal, ok := v.(bool); ok {
					useOriginalTitle = boolVal
				}
			}
		}
	}

	if hasAIData {
		aiAttempted = true
		// 根据配置决定是否使用 AI 数据
		if !useOriginalTitle {
			// 配置为优先使用 AI 生成
			metadataSource = "ai_generated"
			t.App.Logger.Info("  ✓ 配置为使用 AI 生成元数据")
		} else {
			// 配置为使用原始数据
			metadataSource = "original"
			fallbackReason = "配置为使用原始数据"
			t.App.Logger.Info("  ✓ 配置为使用原始元数据")
		}
	} else {
		// 没有 AI 数据，使用原始
		metadataSource = "original"
		fallbackReason = "AI 未生成数据"
		t.App.Logger.Info("  ⚠️ AI 未生成数据，使用原始元数据")
	}

	// 4. 填充最终元数据
	t.fillUploadMetadata(savedVideo, metadataSource)

	// 5. 保存到数据库
	savedVideo.MetadataSource = metadataSource
	savedVideo.MetadataEditStatus = "auto" // 自动模式

	if err := t.SavedVideoService.UpdateVideo(savedVideo); err != nil {
		errMsg := fmt.Sprintf("更新数据库失败: %v", err)
		t.App.Logger.Error("❌ " + errMsg)
		ctx["error"] = errMsg
		return false
	}

	// 6. 记录结果到 context
	ctx["upload_title"] = savedVideo.UploadTitle
	ctx["upload_description"] = savedVideo.UploadDesc
	ctx["upload_tags"] = savedVideo.UploadTags

	// 7. 构建详细的 result_data
	resultData := map[string]interface{}{
		"metadata_source":  metadataSource,
		"ai_attempted":     aiAttempted,
		"has_ai_data":      hasAIData,
		"title_source":     t.getTitleSourceString(metadataSource),
		"original_title":   savedVideo.Title,
		"generated_title":  savedVideo.GeneratedTitle,
		"upload_title":     savedVideo.UploadTitle,
	}

	if fallbackReason != "" {
		resultData["fallback_reason"] = fallbackReason
	}

	ctx["result_data"] = resultData

	// 8. 输出日志
	t.App.Logger.Infof("✅ 元数据已确认")
	t.App.Logger.Infof("   来源: %s", resultData["title_source"])
	t.App.Logger.Infof("   标题: %s", truncateStrForConfirm(savedVideo.UploadTitle, 50))
	t.App.Logger.Infof("   AI尝试: %v", aiAttempted)

	return true
}

// fillUploadMetadata 填充最终上传元数据
func (t *ConfirmMetadata) fillUploadMetadata(savedVideo *model.SavedVideo, source string) {
	switch source {
	case "ai_generated":
		// 优先使用 AI 生成的数据，不存在时回退到原始
		savedVideo.UploadTitle = coalesce(savedVideo.GeneratedTitle, savedVideo.Title, "")
		savedVideo.UploadDesc = coalesce(savedVideo.GeneratedDesc, savedVideo.Description, "")
		savedVideo.UploadTags = savedVideo.GeneratedTags // 标签可以是空

	case "original":
		// 使用原始数据
		savedVideo.UploadTitle = savedVideo.Title
		savedVideo.UploadDesc = savedVideo.Description
		savedVideo.UploadTags = "" // 原始数据没有标签

	case "user_edited":
		// 用户已编辑，保持不变（通过前端 API 设置）
		if savedVideo.UploadTitle == "" {
			savedVideo.UploadTitle = savedVideo.Title
		}
		if savedVideo.UploadDesc == "" {
			savedVideo.UploadDesc = savedVideo.Description
		}

	default:
		// 默认使用原始
		savedVideo.UploadTitle = savedVideo.Title
		savedVideo.UploadDesc = savedVideo.Description
	}
}

// getBiliUploadConfig 获取B站上传配置
func (t *ConfirmMetadata) getBiliUploadConfig(userID uint) interface{} {
	if t.App.Config == nil || t.App.Config.BilibiliConfig == nil {
		return nil
	}
	return t.App.Config.BilibiliConfig
}

// getTitleSourceString 获取标题来源的可读字符串
func (t *ConfirmMetadata) getTitleSourceString(source string) string {
	switch source {
	case "ai_generated":
		return "✨ AI生成"
	case "user_edited":
		return "📝 用户编辑"
	case "original":
		return "📹 原始"
	default:
		return "未知"
	}
}

// coalesce 返回第一个非空字符串
func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// truncateStrForConfirm 截断字符串（避免与其他文件的函数重名）
func truncateStrForConfirm(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// parseTags 解析标签字符串为数组
func parseTags(tagsStr string) []string {
	if tagsStr == "" {
		return []string{}
	}

	var tags []string
	if err := json.Unmarshal([]byte(tagsStr), &tags); err != nil {
		// 如果不是 JSON，尝试逗号分隔
		tags = strings.Split(tagsStr, ",")
		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}
	}

	// 过滤空标签
	var result []string
	for _, tag := range tags {
		if tag != "" {
			result = append(result, tag)
		}
	}

	return result
}
