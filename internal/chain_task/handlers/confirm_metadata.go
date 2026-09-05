package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/difyz9/ytb2bili/internal/chain_task/base"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/core/types"
	"github.com/difyz9/ytb2bili/pkg/cos"
	"github.com/difyz9/ytb2bili/pkg/store/model"
)

// 避免与其他文件的 truncateString 冲突
var _ = truncateStrForConfirm

// ConfirmMetadata 确认元数据任务
// 职责：根据配置决定最终上传使用的元数据（原始 vs AI生成），并支持模板合成
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
	biliConfig := t.getBiliUploadConfig()

	// 3. 决定元数据来源
	metadataSource := "original" // 默认使用原始数据
	aiAttempted := false
	fallbackReason := ""

	// 检查是否有 AI 生成的数据
	hasAIData := savedVideo.GeneratedTitle != "" ||
		savedVideo.GeneratedDesc != "" ||
		savedVideo.GeneratedTags != ""

	// 补救措施：如果 DB 中没有 AI 数据，尝试从 meta.json 读取
	// 这解决了某些情况下 DB 更新延迟或失败导致 AI 数据丢失的问题
	if !hasAIData {
		metaPath := filepath.Join(t.StateManager.CurrentDir, "meta.json")
		if _, err := os.Stat(metaPath); err == nil {
			t.App.Logger.Info("  🔄 DB中缺失AI数据，尝试从 meta.json 恢复...")
			if content, err := os.ReadFile(metaPath); err == nil {
				var metaData map[string]interface{}
				if err := json.Unmarshal(content, &metaData); err == nil {
					// 恢复数据到对象（暂不存回DB，稍后统一保存）
					if title, ok := metaData["title"].(string); ok && title != "" {
						savedVideo.GeneratedTitle = title
						hasAIData = true
					}
					if desc, ok := metaData["description"].(string); ok && desc != "" {
						savedVideo.GeneratedDesc = desc
						hasAIData = true
					}
					if tags, ok := metaData["tags"].([]interface{}); ok {
						var tagStrs []string
						for _, tag := range tags {
							if s, ok := tag.(string); ok {
								tagStrs = append(tagStrs, s)
							}
						}
						if len(tagStrs) > 0 {
							savedVideo.GeneratedTags = strings.Join(tagStrs, ",")
							hasAIData = true
						}
					}
					if hasAIData {
						t.App.Logger.Infof("  ✅ 成功从 meta.json 恢复 AI 数据: %s", savedVideo.GeneratedTitle)
					}
				}
			}
		}
	}

	// 检查配置是否使用原始标题
	useOriginalTitle := true
	customTitleTemplate := ""
	customDescTemplate := ""

	if biliConfig != nil {
		// 优先尝试直接断言为结构体指针
		if config, ok := biliConfig.(*types.BilibiliConfig); ok {
			useOriginalTitle = config.UseOriginalTitle
			customTitleTemplate = config.CustomTitleTemplate
			customDescTemplate = config.CustomDescTemplate
		} else if config, ok := biliConfig.(map[string]interface{}); ok {
			// 回退逻辑：尝试处理 map 类型（兼容某些动态解析场景）
			if v, exists := config["UseOriginalTitle"]; exists {
				if boolVal, ok := v.(bool); ok {
					useOriginalTitle = boolVal
				}
			}
			if v, exists := config["CustomTitleTemplate"]; exists {
				if strVal, ok := v.(string); ok {
					customTitleTemplate = strVal
				}
			}
			if v, exists := config["CustomDescTemplate"]; exists {
				if strVal, ok := v.(string); ok {
					customDescTemplate = strVal
				}
			}
		}
	}

	// 优先检查是否配置了模板
	if customTitleTemplate != "" || customDescTemplate != "" {
		metadataSource = "template_mixture"
		t.App.Logger.Info("  ✓ 检测到自定义模板，将使用模板混合模式")
		if hasAIData {
			aiAttempted = true
		}
	} else if hasAIData {
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
	t.fillUploadMetadata(savedVideo, metadataSource, customTitleTemplate, customDescTemplate)

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
		"metadata_source": metadataSource,
		"ai_attempted":    aiAttempted,
		"has_ai_data":     hasAIData,
		"title_source":    t.getTitleSourceString(metadataSource),
		"original_title":  savedVideo.Title,
		"generated_title": savedVideo.GeneratedTitle,
		"upload_title":    savedVideo.UploadTitle,
	}

	if fallbackReason != "" {
		resultData["fallback_reason"] = fallbackReason
	}

	ctx["result_data"] = resultData

	// 8. 输出日志
	t.App.Logger.Infof("✅ 元数据已确认")
	t.App.Logger.Infof("   来源: %s", resultData["title_source"])
	t.App.Logger.Infof("   标题: %s", truncateStrForConfirm(savedVideo.UploadTitle, 50))
	if customTitleTemplate != "" {
		t.App.Logger.Infof("   标题模板: %s", customTitleTemplate)
	}
	t.App.Logger.Infof("   AI尝试: %v", aiAttempted)

	return true
}

// fillUploadMetadata 填充最终上传元数据
func (t *ConfirmMetadata) fillUploadMetadata(savedVideo *model.SavedVideo, source string, titleTemplate, descTemplate string) {
	switch source {
	case "template_mixture":
		// 使用模板混合
		if titleTemplate != "" {
			savedVideo.UploadTitle = t.applyTemplate(titleTemplate, savedVideo)
		} else {
			// 如果没有标题模板，回退到智能选择（优先AI）
			savedVideo.UploadTitle = coalesce(savedVideo.GeneratedTitle, savedVideo.Title, "")
		}

		if descTemplate != "" {
			savedVideo.UploadDesc = t.applyTemplate(descTemplate, savedVideo)
		} else {
			// 如果没有描述模板，回退到智能选择（优先AI）
			savedVideo.UploadDesc = coalesce(savedVideo.GeneratedDesc, savedVideo.Description, "")
		}

		// 标签优先使用AI生成的，如果没有则为空
		savedVideo.UploadTags = savedVideo.GeneratedTags

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

// applyTemplate 应用模板替换变量
func (t *ConfirmMetadata) applyTemplate(template string, video *model.SavedVideo) string {
	result := template

	// 基础变量
	result = strings.ReplaceAll(result, "{original_title}", video.Title)
	// 如果 AI 标题为空，使用空字符串替换，避免显示 "{ai_title}"
	result = strings.ReplaceAll(result, "{ai_title}", video.GeneratedTitle)

	result = strings.ReplaceAll(result, "{original_desc}", video.Description)
	result = strings.ReplaceAll(result, "{ai_desc}", video.GeneratedDesc)

	// 清理可能产生的多余空格或分隔符（简单的优化）
	result = strings.TrimSpace(result)

	return result
}

// getBiliUploadConfig 获取B站上传配置
func (t *ConfirmMetadata) getBiliUploadConfig() interface{} {
	if t.App.Config == nil || t.App.Config.BilibiliConfig == nil {
		return nil
	}
	return t.App.Config.BilibiliConfig
}

// getTitleSourceString 获取标题来源的可读字符串
func (t *ConfirmMetadata) getTitleSourceString(source string) string {
	switch source {
	case "template_mixture":
		return "🧩 模板混合"
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
