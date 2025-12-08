package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/difyz9/ytb2bili/internal/chain_task/base"
	"github.com/difyz9/ytb2bili/internal/chain_task/manager"
	"github.com/difyz9/ytb2bili/internal/core"
	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/membership"
	"github.com/difyz9/ytb2bili/pkg/cos"
	"github.com/difyz9/ytb2bili/pkg/prompts"
	"gorm.io/gorm"
)

type GenerateMetadata struct {
	base.BaseTask
	App               *core.AppServer
	DeepSeekClient    *DeepSeekClient
	GeminiClient      *GeminiClient
	SavedVideoService *services.SavedVideoService
	AIManager         *services.AIServiceManager
	LastProvider      services.AIProvider
}

func NewGenerateMetadata(name string, app *core.AppServer, stateManager *manager.StateManager, client *cos.CosClient, apiKey string, db *gorm.DB, savedVideoService *services.SavedVideoService) *GenerateMetadata {
	// 创建AI服务管理器
	aiManager := services.NewAIServiceManager(app.Config, app.Logger)

	return &GenerateMetadata{
		BaseTask: base.BaseTask{
			Name:         name,
			StateManager: stateManager,
			Client:       client,
		},
		App:               app,
		DeepSeekClient:    nil, // 不再固化客户端，运行时动态创建
		SavedVideoService: savedVideoService,
		AIManager:         aiManager,
	}
}

// getCurrentAIProvider 获取当前可用的AI服务提供商
func (g *GenerateMetadata) getCurrentAIProvider() (services.AIProvider, error) {
	// 刷新配置
	g.AIManager.RefreshConfig(g.App.Config)

	// 获取首选提供商
	provider, err := g.AIManager.GetPreferredProvider()
	if err != nil {
		return "", fmt.Errorf("没有可用的AI服务: %v", err)
	}

	return provider, nil
}

// getCurrentDeepSeekClient 获取当前的DeepSeek客户端（使用最新配置，兼容旧代码）
func (g *GenerateMetadata) getCurrentDeepSeekClient() (*DeepSeekClient, error) {
	if g.App.Config.DeepSeekTransConfig == nil || !g.App.Config.DeepSeekTransConfig.Enabled {
		return nil, fmt.Errorf("DeepSeek 翻译服务未启用")
	}

	apiKey := g.App.Config.DeepSeekTransConfig.ApiKey
	if apiKey == "" {
		return nil, fmt.Errorf("DeepSeek API Key 未配置")
	}

	return NewDeepSeekClient(apiKey), nil
}

type VideoMetadata struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// checkUserPermission 检查用户是否有 AI 元数据生成权限
func (g *GenerateMetadata) checkUserPermission() bool {
	g.App.Logger.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	g.App.Logger.Info("🔐 开始检查用户会员权限...")

	// 获取视频信息，包含提交用户ID
	savedVideo, err := g.SavedVideoService.GetVideoByVideoID(g.StateManager.VideoID)
	if err != nil {
		g.App.Logger.Warnf("⚠️ 无法获取视频信息: %v，默认允许执行", err)
		return true // 获取失败时默认允许（向后兼容）
	}

	g.App.Logger.Infof("📹 视频ID: %s, 关联用户ID: %d", savedVideo.VideoID, savedVideo.UserID)

	// 如果没有用户ID（旧数据），默认允许
	if savedVideo.UserID == 0 {
		g.App.Logger.Info("ℹ️ 视频没有关联用户ID（旧数据），默认允许执行")
		return true
	}

	userID := strconv.FormatUint(uint64(savedVideo.UserID), 10)
	g.App.Logger.Infof("📋 检查用户 %s 的 AI 功能权限...", userID)

	// 创建会员存储和检查器
	membershipStore := membership.NewDBMembershipStore(g.App.DB)
	checker := membership.NewFeatureChecker(membershipStore)

	// 获取用户会员信息用于日志
	userMembership, err := checker.GetUserMembership(context.Background(), userID)
	if err != nil {
		g.App.Logger.Warnf("⚠️ 获取用户会员信息失败: %v", err)
	} else {
		g.App.Logger.Infof("👤 用户会员信息:")
		g.App.Logger.Infof("   - 用户ID: %s", userMembership.UserID)
		g.App.Logger.Infof("   - 会员等级: %s", userMembership.Tier)
		g.App.Logger.Infof("   - 过期时间: %s", userMembership.ExpiresAt.Format("2006-01-02 15:04:05"))
		g.App.Logger.Infof("   - 是否过期: %v", userMembership.IsExpired())
		g.App.Logger.Infof("   - 有效等级: %s", userMembership.GetEffectiveTier())

		// 获取等级配置
		config := userMembership.GetConfig()
		g.App.Logger.Infof("   - 等级名称: %s", config.Name)
		g.App.Logger.Infof("   - Gemini视频分析权限: %v", config.Features.GeminiVideoAnalysis)
	}

	// 检查 Gemini 视频分析权限
	result := checker.CanUseFeature(context.Background(), userID, "gemini_video_analysis")
	g.App.Logger.Infof("🔍 权限检查结果: Allowed=%v, Reason=%s, Code=%s", result.Allowed, result.Reason, result.Code)

	if !result.Allowed {
		g.App.Logger.Warnf("❌ 用户 %s 没有 Gemini 视频分析权限: %s", userID, result.Reason)
		g.App.Logger.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		return false
	}

	g.App.Logger.Infof("✅ 用户 %s 有 AI 元数据生成权限", userID)
	g.App.Logger.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	return true
}

func (g *GenerateMetadata) Execute(ctx map[string]interface{}) bool {
	g.App.Logger.Info("========================================")
	g.App.Logger.Infof("开始生成视频标题和描述: VideoID=%s", g.StateManager.VideoID)
	g.App.Logger.Infof("📁 工作目录: %s", g.StateManager.CurrentDir)
	g.App.Logger.Info("========================================")

	// 0. 检查用户会员权限
	if !g.checkUserPermission() {
		g.App.Logger.Warn("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		g.App.Logger.Warn("⚠️ 用户没有 AI 元数据生成权限，跳过此步骤")
		g.App.Logger.Warn("💡 升级到 Pro 会员可解锁 Gemini 视频分析功能")
		g.App.Logger.Warn("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		// 设置跳过标记，让 TaskStepWrapper 设置正确的状态
		ctx["skipped"] = "需要 Pro 会员才能使用 AI 生成元数据功能，请升级会员"
		// 返回 true 表示步骤完成（跳过），不阻塞后续任务
		return true
	}

	// 列出工作目录中的文件，帮助调试
	g.logDirectoryContents()

	// 1. 刷新AI服务管理器配置
	g.AIManager.RefreshConfig(g.App.Config)

	// ⚠️ 元数据生成必须使用 Gemini（多模态视频分析能力）
	// 检查 Gemini 是否已配置（支持单个 ApiKey 或多个 ApiKeys）
	geminiConfigured := g.App.Config.GeminiConfig != nil && g.App.Config.GeminiConfig.Enabled &&
		(g.App.Config.GeminiConfig.ApiKey != "" || len(g.App.Config.GeminiConfig.ApiKeys) > 0)
	if !geminiConfigured {
		g.App.Logger.Error("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		g.App.Logger.Error("❌ 元数据生成需要配置 Gemini 服务！")
		g.App.Logger.Error("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		g.App.Logger.Warn("💡 Gemini 具有多模态视频分析能力，是生成高质量元数据的最佳选择")
		g.App.Logger.Warn("💡 请在设置页面配置 Gemini API Key 并启用")
		g.App.Logger.Warn("💡 配置路径: 设置 → AI 大模型 → Gemini 原生多模态")

		// 尝试使用备选方案（用户首选AI或DeepSeek）生成基础元数据
		g.App.Logger.Info("🔄 尝试使用备选AI服务生成基础元数据...")
		return g.executeWithFallbackAI(ctx)
	}

	// 1. 首选：使用 Gemini 多模态服务生成元数据
	g.App.Logger.Info("🤖 使用 Gemini 多模态服务生成元数据")
	g.App.Logger.Infof("📋 Gemini 配置: Model=%s, Timeout=%ds, AnalyzeVideo=%v",
		g.App.Config.GeminiConfig.Model,
		g.App.Config.GeminiConfig.Timeout,
		g.App.Config.GeminiConfig.AnalyzeVideo)

	// 如果配置了视频分析，尝试使用视频文件
	if g.App.Config.GeminiConfig.AnalyzeVideo {
		g.App.Logger.Info("🎬 尝试 Gemini 视频分析模式...")
		if success := g.executeWithGeminiVideo(ctx); success {
			return true
		}
		g.App.Logger.Warn("⚠️ Gemini 视频分析失败，回退到文本模式")
	}

	// 使用 Gemini 处理字幕文本
	g.App.Logger.Info("📝 尝试 Gemini 文本分析模式...")
	if success := g.executeWithGeminiText(ctx); success {
		return true
	}

	// 2. Gemini 失败时，使用备选AI服务
	g.App.Logger.Warn("⚠️ Gemini 分析失败，尝试备选AI服务...")
	return g.executeWithFallbackAI(ctx)
}

// executeWithFallbackAI 使用备选AI服务生成元数据（当Gemini不可用时）
func (g *GenerateMetadata) executeWithFallbackAI(ctx map[string]interface{}) bool {
	// 尝试用户首选的AI服务
	if g.AIManager.IsOpenAICompatibleEnabled() {
		provider, _ := g.getCurrentAIProvider()
		g.LastProvider = provider
		status := g.AIManager.GetStatus(provider)
		g.App.Logger.Infof("🔄 使用备选AI服务: %s (模型: %s)", status.Name, status.Model)

		if success := g.executeWithAIManager(ctx); success {
			return true
		}
		g.App.Logger.Warn("⚠️ 备选AI服务失败...")
	}

	// 最后尝试 DeepSeek
	if g.AIManager.IsDeepSeekEnabled() {
		g.App.Logger.Info("🔄 尝试 DeepSeek 模式...")
		return g.executeWithDeepSeek(ctx)
	}

	g.App.Logger.Error("❌ 所有AI服务都不可用，无法生成元数据")
	return false
}

// executeWithAIManager 使用AI服务管理器生成元数据（首选方式）
func (g *GenerateMetadata) executeWithAIManager(ctx map[string]interface{}) bool {
	g.App.Logger.Info("🔄 使用AI服务管理器生成元数据...")

	// 1. 检查中文字幕文件是否存在
	zhSRTPath := filepath.Join(g.StateManager.CurrentDir, "zh.srt")
	g.App.Logger.Infof("🔍 检查中文字幕文件: %s", zhSRTPath)
	if _, err := os.Stat(zhSRTPath); os.IsNotExist(err) {
		g.App.Logger.Warnf("⚠️ 中文字幕文件不存在: %s", zhSRTPath)
		g.App.Logger.Warn("⚠️ 请确保字幕翻译步骤已成功完成，使用默认标题和描述")
		ctx["video_title"] = g.StateManager.VideoID
		ctx["video_description"] = "包含字幕的视频"
		return true
	}
	g.App.Logger.Infof("✓ 找到中文字幕文件: %s", zhSRTPath)

	// 2. 读取中文字幕内容
	srtContent, err := os.ReadFile(zhSRTPath)
	if err != nil {
		g.App.Logger.Errorf("❌ 读取中文字幕文件失败: %v", err)
		ctx["error"] = "读取翻译字幕失败，请确保字幕翻译步骤已完成"
		return false
	}

	// 3. 解析字幕提取文本
	subtitleText := g.extractTextFromSRT(string(srtContent))
	if subtitleText == "" {
		g.App.Logger.Warn("⚠️ 字幕内容为空，使用默认标题和描述")
		ctx["video_title"] = g.StateManager.VideoID
		ctx["video_description"] = "包含字幕的视频"
		return true
	}

	g.App.Logger.Infof("📝 提取到字幕文本，总长度: %d 字符", len(subtitleText))

	// 4. 截取前1000字符用于生成标题和描述
	maxLength := 1000
	if len(subtitleText) > maxLength {
		subtitleText = subtitleText[:maxLength] + "..."
	}

	// 5. 使用AI服务管理器生成元数据
	g.App.Logger.Info("🤖 调用AI服务生成标题和描述...")
	metadata, err := g.generateMetadataWithAIManager(subtitleText)
	if err != nil {
		g.App.Logger.Errorf("❌ AI服务生成元数据失败: %v", err)
		return false // 返回false让调用者尝试备选服务
	}

	// 6. 验证标题长度（Bilibili限制80字符）
	if len([]rune(metadata.Title)) > 80 {
		runes := []rune(metadata.Title)
		metadata.Title = string(runes[:77]) + "..."
		g.App.Logger.Warnf("⚠️ 标题过长，已截断为80字符")
	}

	// 7. 保存到 context
	ctx["video_title"] = metadata.Title
	ctx["video_description"] = metadata.Description
	ctx["video_tags"] = metadata.Tags

	// 8. 保存到 meta.json 文件
	g.App.Logger.Info("💾 保存元数据到 meta.json 文件...")
	if err := g.saveMetadataToFile(metadata); err != nil {
		g.App.Logger.Errorf("❌ 保存 meta.json 文件失败: %v", err)
	} else {
		g.App.Logger.Info("✅ meta.json 文件已保存")
	}

	// 9. 保存到数据库
	g.App.Logger.Info("💾 保存生成的元数据到数据库...")
	savedVideo, err := g.SavedVideoService.GetVideoByVideoID(g.StateManager.VideoID)
	if err != nil {
		g.App.Logger.Errorf("❌ 获取视频记录失败: %v", err)
	} else {
		savedVideo.GeneratedTitle = metadata.Title
		savedVideo.GeneratedDesc = metadata.Description
		if len(metadata.Tags) > 0 {
			tagsJSON, _ := json.Marshal(metadata.Tags)
			savedVideo.GeneratedTags = string(tagsJSON)
		}
		if err := g.SavedVideoService.UpdateVideo(savedVideo); err != nil {
			g.App.Logger.Errorf("❌ 更新视频记录失败: %v", err)
		} else {
			g.App.Logger.Info("✅ 数据库记录已更新")
		}
	}

	g.App.Logger.Infof("✓ 生成标题: %s", metadata.Title)
	g.App.Logger.Infof("✓ 生成描述: %s", truncateString(metadata.Description, 100))
	g.App.Logger.Infof("✓ 生成标签: %v", metadata.Tags)
	g.App.Logger.Info("========================================")

	return true
}

// generateMetadataWithAIManager 使用AI服务管理器生成元数据
func (g *GenerateMetadata) generateMetadataWithAIManager(subtitleText string) (*VideoMetadata, error) {
	// 从提示词管理器获取提示词
	promptManager := prompts.GetGlobalManager()
	promptTemplate, err := promptManager.GetPrompt(prompts.PromptMetadataFallback)
	if err != nil {
		g.App.Logger.Warnf("获取提示词失败，使用默认值: %v", err)
	}

	var systemPrompt string
	if promptTemplate != nil {
		systemPrompt = promptTemplate.System
	} else {
		systemPrompt = `你是一个专业的视频内容分析师，擅长为Bilibili视频生成吸引人的标题和描述。

请根据提供的字幕内容，生成：
1. 标题：简洁有力，能吸引观众点击，不超过80个字符
2. 描述：详细介绍视频内容，包含关键信息，适合SEO
3. 标签：5-10个相关标签，用于视频分类和搜索

请以JSON格式返回，格式如下：
{
  "title": "视频标题",
  "description": "视频描述",
  "tags": ["标签1", "标签2", "标签3"]
}

注意：只返回JSON，不要添加其他内容`
	}

	// 渲染用户提示词
	userPrompt, err := promptManager.RenderUserPrompt(prompts.PromptMetadataFallback, &prompts.PromptParams{
		SubtitleText: subtitleText,
	})
	if err != nil || userPrompt == "" {
		userPrompt = fmt.Sprintf("请根据以下字幕内容生成视频元数据：\n\n%s", subtitleText)
	}

	// 使用AI服务管理器调用
	response, provider, err := g.AIManager.ChatCompletion(systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("AI服务调用失败: %v", err)
	}

	// 记录实际使用的提供商
	if provider != g.LastProvider {
		g.App.Logger.Infof("🔄 AI服务已切换: %s -> %s", g.LastProvider, provider)
		g.LastProvider = provider
	}

	// 解析JSON响应
	var metadata VideoMetadata
	cleanResponse := strings.TrimSpace(response)
	cleanResponse = strings.TrimPrefix(cleanResponse, "```json")
	cleanResponse = strings.TrimPrefix(cleanResponse, "```")
	cleanResponse = strings.TrimSuffix(cleanResponse, "```")
	cleanResponse = strings.TrimSpace(cleanResponse)

	if err := json.Unmarshal([]byte(cleanResponse), &metadata); err != nil {
		return nil, fmt.Errorf("解析AI响应失败: %v, 原始响应: %s", err, response)
	}

	return &metadata, nil
}

// executeWithDeepSeek 使用 DeepSeek 生成元数据
func (g *GenerateMetadata) executeWithDeepSeek(context map[string]interface{}) bool {
	g.App.Logger.Info("🔄 使用 DeepSeek 生成元数据...")

	// 0. 动态获取最新的DeepSeek客户端
	client, err := g.getCurrentDeepSeekClient()
	if err != nil {
		g.App.Logger.Errorf("❌ 获取 DeepSeek 客户端失败: %v", err)
		g.App.Logger.Warn("⚠️ 使用默认标题和描述")
		// 使用默认值而不是失败
		context["video_title"] = g.StateManager.VideoID
		context["video_description"] = "包含字幕的视频"
		return true
	}

	g.App.Logger.Infof("🔑 DeepSeek 客户端创建成功")
	// 更新当前使用的客户端
	g.DeepSeekClient = client

	// 1. 检查中文字幕文件是否存在
	zhSRTPath := filepath.Join(g.StateManager.CurrentDir, "zh.srt")
	g.App.Logger.Infof("🔍 检查中文字幕文件: %s", zhSRTPath)
	if _, err := os.Stat(zhSRTPath); os.IsNotExist(err) {
		g.App.Logger.Warnf("⚠️ 中文字幕文件不存在: %s", zhSRTPath)
		g.App.Logger.Warn("⚠️ 请确保字幕翻译步骤已成功完成，使用默认标题和描述")
		// 使用默认值
		context["video_title"] = g.StateManager.VideoID
		context["video_description"] = fmt.Sprintf("包含字幕的视频")
		return true // 没有字幕文件不算失败
	}
	g.App.Logger.Infof("✓ 找到中文字幕文件: %s", zhSRTPath)

	// 2. 读取中文字幕内容
	srtContent, err := os.ReadFile(zhSRTPath)
	if err != nil {
		g.App.Logger.Errorf("❌ 读取中文字幕文件失败: %v", err)
		context["error"] = "读取翻译字幕失败，请确保字幕翻译步骤已完成"
		return false
	}

	// 3. 解析字幕提取文本
	subtitleText := g.extractTextFromSRT(string(srtContent))
	if subtitleText == "" {
		g.App.Logger.Warn("⚠️  字幕内容为空，使用默认标题和描述")
		context["video_title"] = g.StateManager.VideoID
		context["video_description"] = fmt.Sprintf("包含字幕的视频")
		return true
	}

	g.App.Logger.Infof("📝 提取到字幕文本，总长度: %d 字符", len(subtitleText))

	// 4. 截取前1000字符用于生成标题和描述（避免token过多）
	maxLength := 1000
	if len(subtitleText) > maxLength {
		subtitleText = subtitleText[:maxLength] + "..."
	}

	// 5. 调用 DeepSeek API 生成标题和描述
	g.App.Logger.Info("🤖 调用 DeepSeek API 生成标题和描述...")
	metadata, err := g.generateMetadataFromDeepSeek(subtitleText)
	if err != nil {
		g.App.Logger.Errorf("❌ 生成标题和描述失败: %v", err)
		g.App.Logger.Warn("⚠️  将使用默认标题和描述，不影响视频上传")
		// 使用默认值
		context["video_title"] = g.StateManager.VideoID
		context["video_description"] = fmt.Sprintf("包含字幕的视频")
		return true // API调用失败不算整个任务失败
	}

	// 6. 验证标题长度（Bilibili限制80字符）
	if len([]rune(metadata.Title)) > 80 {
		runes := []rune(metadata.Title)
		metadata.Title = string(runes[:77]) + "..."
		g.App.Logger.Warnf("⚠️  标题过长，已截断为80字符")
	}

	// 7. 保存到 context
	context["video_title"] = metadata.Title
	context["video_description"] = metadata.Description
	context["video_tags"] = metadata.Tags

	// 8. 保存到 meta.json 文件
	g.App.Logger.Info("💾 保存元数据到 meta.json 文件...")
	if err := g.saveMetadataToFile(metadata); err != nil {
		g.App.Logger.Errorf("❌ 保存 meta.json 文件失败: %v", err)
		// 不影响任务继续执行
	} else {
		g.App.Logger.Info("✅ meta.json 文件已保存")
	}

	// 9. 保存到数据库
	g.App.Logger.Info("💾 保存生成的元数据到数据库...")
	savedVideo, err := g.SavedVideoService.GetVideoByVideoID(g.StateManager.VideoID)
	if err != nil {
		g.App.Logger.Errorf("❌ 获取视频记录失败: %v", err)
		// 不影响任务继续执行
	} else {
		// 更新生成的元数据
		savedVideo.GeneratedTitle = metadata.Title
		savedVideo.GeneratedDesc = metadata.Description
		savedVideo.GeneratedTags = strings.Join(metadata.Tags, ",")

		if err := g.SavedVideoService.UpdateVideo(savedVideo); err != nil {
			g.App.Logger.Errorf("❌ 保存元数据到数据库失败: %v", err)
		} else {
			g.App.Logger.Info("✅ 元数据已保存到数据库")
		}
	}

	// 10. 输出生成结果
	g.App.Logger.Info("========================================")
	g.App.Logger.Info("✅ 视频元数据生成成功！")
	g.App.Logger.Infof("📌 标题: %s", metadata.Title)
	g.App.Logger.Infof("📝 描述: %s", g.truncateString(metadata.Description, 100))
	g.App.Logger.Infof("🏷️  标签: %v", metadata.Tags)
	g.App.Logger.Info("========================================")

	return true
}

// extractTextFromSRT 从SRT内容中提取纯文本
func (g *GenerateMetadata) extractTextFromSRT(srtContent string) string {
	lines := strings.Split(srtContent, "\n")
	var textLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过空行、序号行、时间码行
		if line == "" || isNumber(line) || strings.Contains(line, "-->") {
			continue
		}
		textLines = append(textLines, line)
	}

	return strings.Join(textLines, " ")
}

// isNumber 检查字符串是否为数字
func isNumber(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// generateMetadataFromDeepSeek 调用 DeepSeek API 生成标题和描述
func (g *GenerateMetadata) generateMetadataFromDeepSeek(subtitleText string) (*VideoMetadata, error) {
	prompt := fmt.Sprintf(`请根据以下视频字幕内容，生成一个吸引人的视频标题、详细描述和3-5个相关标签。

字幕内容：
%s

要求：
1. 标题要简洁有力，严格控制在30个字以内（B站限制80字，但建议30字以内更易读），能够准确概括视频主题，吸引观众点击
2. 描述要详细但不要过长，严格控制在600-800字以内，包含视频的主要内容和亮点（注意：B站简介限制2000字，需要预留约200字给原视频链接和分隔线）
3. 标签要准确反映视频内容，3-5个即可
4. 必须使用中文
5. 输出格式必须是JSON，格式如下：
{
  "title": "视频标题",
  "description": "视频描述",
  "tags": ["标签1", "标签2", "标签3"]
}

请直接返回JSON格式的结果，不要包含任何其他说明文字。`, subtitleText)

	// 使用 DeepSeekClient 调用 API
	content, usage, err := g.DeepSeekClient.ChatCompletionWithUsage("你是一个专业的视频内容分析助手，擅长根据视频字幕生成吸引人的标题和描述。", prompt)
	if err != nil {
		return nil, fmt.Errorf("调用 DeepSeek API 失败: %v", err)
	}

	g.App.Logger.Debugf("DeepSeek 原始返回: %s", content)

	// 提取JSON部分（可能包含在代码块中）
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
	}
	content = strings.TrimSpace(content)

	// 解析JSON
	var metadata VideoMetadata
	if err := json.Unmarshal([]byte(content), &metadata); err != nil {
		return nil, fmt.Errorf("解析元数据JSON失败: %v, 内容: %s", err, content)
	}

	// 验证必填字段
	if metadata.Title == "" {
		return nil, fmt.Errorf("生成的标题为空")
	}

	// Token使用情况
	if usage != nil {
		g.App.Logger.Infof("💰 Token使用: 输入=%d, 输出=%d, 总计=%d",
			usage.PromptTokens,
			usage.CompletionTokens,
			usage.TotalTokens)
	}

	return &metadata, nil
}

// saveMetadataToFile 保存元数据到 meta.json 文件
func (g *GenerateMetadata) saveMetadataToFile(metadata *VideoMetadata) error {
	// 构建文件路径
	metaFilePath := filepath.Join(g.StateManager.CurrentDir, "meta.json")

	// 创建一个包含更多信息的元数据结构
	fileMetadata := map[string]interface{}{
		"video_id":     g.StateManager.VideoID,
		"title":        metadata.Title,
		"description":  metadata.Description,
		"tags":         metadata.Tags,
		"generated_at": time.Now().Format("2006-01-02 15:04:05"),
	}

	// 转换为格式化的JSON
	jsonData, err := json.MarshalIndent(fileMetadata, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化元数据失败: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(metaFilePath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入meta.json文件失败: %v", err)
	}

	g.App.Logger.Infof("📁 meta.json 文件已保存: %s", metaFilePath)
	return nil
}

// truncateString 截断字符串用于日志显示
func (g *GenerateMetadata) truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// 视频大小阈值常量
const (
	SmallVideoThresholdMB     = 50   // 小视频阈值 (MB)
	MediumVideoThresholdMB    = 200  // 中等视频阈值 (MB)
	LargeVideoThresholdMB     = 500  // 大视频阈值 (MB)
	MaxGeminiUploadSizeMB     = 2000 // Gemini 最大上传大小 (MB)，实际限制约 2GB
	MaxGeminiRetries          = 2    // 最大重试次数
)

// executeWithGeminiVideo 使用 Gemini 分析视频文件生成元数据
func (g *GenerateMetadata) executeWithGeminiVideo(taskContext map[string]interface{}) bool {
	totalStartTime := time.Now()

	// ═══════════════════════════════════════════════════════════════
	g.App.Logger.Info("╔══════════════════════════════════════════════════════════════╗")
	g.App.Logger.Info("║           🎬 Gemini 视频分析流程开始                         ║")
	g.App.Logger.Info("╚══════════════════════════════════════════════════════════════╝")

	// ┌─────────────────────────────────────────────────────────────┐
	// │ 阶段1: 查找视频文件                                        │
	// └─────────────────────────────────────────────────────────────┘
	g.App.Logger.Info("")
	g.App.Logger.Info("┌─────────────────────────────────────────────────────────────┐")
	g.App.Logger.Info("│ � 阶段1: 查找视频文件                                      │")
	g.App.Logger.Info("└─────────────────────────────────────────────────────────────┘")
	g.App.Logger.Infof("   目录: %s", g.StateManager.CurrentDir)

	videoFiles := g.findVideoFiles()
	if len(videoFiles) == 0 {
		g.App.Logger.Error("   ✗ 未找到视频文件")
		g.App.Logger.Warn("   支持的格式: .mp4, .flv, .mkv, .webm, .avi, .mov")
		return false
	}
	videoPath := videoFiles[0]

	// 获取视频文件大小
	fileInfo, err := os.Stat(videoPath)
	if err != nil {
		g.App.Logger.Errorf("   ✗ 无法获取文件信息: %v", err)
		return false
	}
	fileSizeMB := float64(fileInfo.Size()) / 1024 / 1024
	g.App.Logger.Infof("   ✓ 找到视频: %s", filepath.Base(videoPath))
	g.App.Logger.Infof("   ✓ 文件大小: %.2f MB", fileSizeMB)

	// ┌─────────────────────────────────────────────────────────────┐
	// │ 阶段2: 分析策略选择                                        │
	// └─────────────────────────────────────────────────────────────┘
	g.App.Logger.Info("")
	g.App.Logger.Info("┌─────────────────────────────────────────────────────────────┐")
	g.App.Logger.Info("│ 🎯 阶段2: 分析策略选择                                      │")
	g.App.Logger.Info("└─────────────────────────────────────────────────────────────┘")

	// 判断视频大小类别
	var sizeCategory string
	var strategy string
	var timeoutMultiplier float64 = 1.0

	switch {
	case fileSizeMB <= SmallVideoThresholdMB:
		sizeCategory = "小视频"
		strategy = "完整视频分析"
		timeoutMultiplier = 1.0
	case fileSizeMB <= MediumVideoThresholdMB:
		sizeCategory = "中等视频"
		strategy = "完整视频分析（增强超时）"
		timeoutMultiplier = 2.0
	case fileSizeMB <= LargeVideoThresholdMB:
		sizeCategory = "大视频"
		strategy = "完整视频分析（长超时）"
		timeoutMultiplier = 3.0
	case fileSizeMB <= MaxGeminiUploadSizeMB:
		sizeCategory = "超大视频"
		strategy = "完整视频分析（最大超时）"
		timeoutMultiplier = 5.0
	default:
		sizeCategory = "巨型视频"
		strategy = "⚠️ 视频过大，自动降级到关键帧分析"
		g.App.Logger.Infof("   视频分类: %s (%.2f MB)", sizeCategory, fileSizeMB)
		g.App.Logger.Infof("   选择策略: %s", strategy)
		g.App.Logger.Warnf("   ⚠️ 视频超过 Gemini 上传限制 (%d MB)，将使用关键帧分析", MaxGeminiUploadSizeMB)
		return g.executeWithKeyframeAnalysis(taskContext, videoPath, fileSizeMB)
	}

	g.App.Logger.Infof("   视频分类: %s (阈值: 小≤%dMB, 中≤%dMB, 大≤%dMB)",
		sizeCategory, SmallVideoThresholdMB, MediumVideoThresholdMB, LargeVideoThresholdMB)
	g.App.Logger.Infof("   选择策略: 【%s】", strategy)

	// 计算超时时间
	baseTimeout := g.App.Config.GeminiConfig.Timeout
	adjustedTimeout := int(float64(baseTimeout) * timeoutMultiplier)
	if adjustedTimeout > 1800 { // 最大30分钟
		adjustedTimeout = 1800
	}
	g.App.Logger.Infof("   超时设置: %d秒 (基础%d秒 × %.1f倍率)", adjustedTimeout, baseTimeout, timeoutMultiplier)
	g.App.Logger.Infof("   最大重试: %d次", MaxGeminiRetries)

	// 执行视频分析（带重试）
	for retry := 0; retry <= MaxGeminiRetries; retry++ {
		if retry > 0 {
			g.App.Logger.Info("")
			g.App.Logger.Warnf("🔄 第 %d 次重试...", retry)
			// 重试时轮换 API Key
			if g.App.Config.GeminiConfig.GetApiKeysCount() > 1 {
				g.App.Config.GeminiConfig.RotateApiKey()
				g.App.Logger.Info("   已轮换到下一个 API Key")
			}
			// 重试时增加超时
			adjustedTimeout = int(float64(adjustedTimeout) * 1.5)
			if adjustedTimeout > 1800 {
				adjustedTimeout = 1800
			}
			g.App.Logger.Infof("   调整超时: %d秒", adjustedTimeout)
		}

		success := g.executeGeminiVideoAnalysis(taskContext, videoPath, fileSizeMB, adjustedTimeout, retry)
		if success {
			totalDuration := time.Since(totalStartTime)
			g.App.Logger.Info("")
			g.App.Logger.Info("╔══════════════════════════════════════════════════════════════╗")
			g.App.Logger.Infof("║ ✅ Gemini 视频分析完成！总耗时: %.1f秒                      ║", totalDuration.Seconds())
			g.App.Logger.Info("╚══════════════════════════════════════════════════════════════╝")
			return true
		}
	}

	// 所有重试都失败，尝试关键帧分析作为后备
	g.App.Logger.Warn("")
	g.App.Logger.Warn("⚠️ 视频分析失败，尝试关键帧分析作为后备方案...")
	return g.executeWithKeyframeAnalysis(taskContext, videoPath, fileSizeMB)
}

// executeGeminiVideoAnalysis 执行单次 Gemini 视频分析
func (g *GenerateMetadata) executeGeminiVideoAnalysis(taskContext map[string]interface{}, videoPath string, fileSizeMB float64, timeoutSeconds int, retryCount int) bool {
	// ┌─────────────────────────────────────────────────────────────┐
	// │ 阶段3: 创建 Gemini 客户端                                   │
	// └─────────────────────────────────────────────────────────────┘
	g.App.Logger.Info("")
	if retryCount == 0 {
		g.App.Logger.Info("┌─────────────────────────────────────────────────────────────┐")
		g.App.Logger.Info("│ � 阶段3: 创建 Gemini 客户端                                │")
		g.App.Logger.Info("└─────────────────────────────────────────────────────────────┘")
	}

	apiKey := g.App.Config.GeminiConfig.GetCurrentApiKey()
	keyCount := g.App.Config.GeminiConfig.GetApiKeysCount()
	keyIndex := g.App.Config.GeminiConfig.CurrentKeyIndex + 1

	// 隐藏 API Key 中间部分
	maskedKey := apiKey
	if len(apiKey) > 10 {
		maskedKey = apiKey[:4] + "****" + apiKey[len(apiKey)-4:]
	}
	g.App.Logger.Infof("   API Key: %s (%d/%d)", maskedKey, keyIndex, keyCount)
	g.App.Logger.Infof("   模型: %s", g.App.Config.GeminiConfig.Model)
	g.App.Logger.Infof("   超时: %d秒", timeoutSeconds)

	client, err := NewGeminiClient(
		apiKey,
		g.App.Config.GeminiConfig.Model,
		timeoutSeconds,
		g.App.Config.GeminiConfig.MaxTokens,
	)
	if err != nil {
		g.App.Logger.Errorf("   ✗ 创建客户端失败: %v", err)
		return false
	}
	defer client.Close()
	g.App.Logger.Info("   ✓ 客户端创建成功")

	// 创建带超时的 context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// ┌─────────────────────────────────────────────────────────────┐
	// │ 阶段4: 上传视频到 Gemini                                    │
	// └─────────────────────────────────────────────────────────────┘
	g.App.Logger.Info("")
	g.App.Logger.Info("┌─────────────────────────────────────────────────────────────┐")
	g.App.Logger.Info("│ ⏫ 阶段4: 上传视频到 Gemini                                  │")
	g.App.Logger.Info("└─────────────────────────────────────────────────────────────┘")
	g.App.Logger.Infof("   文件: %s (%.2f MB)", filepath.Base(videoPath), fileSizeMB)

	// 估算上传时间
	estimatedUploadTime := fileSizeMB / 5.0 // 假设 5MB/s 上传速度
	g.App.Logger.Infof("   预估上传时间: %.0f秒 (按5MB/s估算)", estimatedUploadTime)
	g.App.Logger.Info("   状态: 上传中...")

	uploadStartTime := time.Now()
	uploadedFile, err := client.UploadFile(ctx, videoPath, filepath.Base(videoPath))
	uploadDuration := time.Since(uploadStartTime)

	if err != nil {
		g.App.Logger.Errorf("   ✗ 上传失败 (耗时 %.1f秒): %v", uploadDuration.Seconds(), err)
		if strings.Contains(err.Error(), "context deadline exceeded") {
			g.App.Logger.Error("   原因: 上传超时")
			g.App.Logger.Errorf("   建议: 增加 config.toml 中的 GeminiConfig.Timeout 值 (当前: %d秒)", timeoutSeconds)
		}
		return false
	}

	actualSpeed := fileSizeMB / uploadDuration.Seconds()
	g.App.Logger.Infof("   ✓ 上传成功 (耗时 %.1f秒, 速度 %.1fMB/s)", uploadDuration.Seconds(), actualSpeed)
	g.App.Logger.Infof("   文件ID: %s", uploadedFile.Name)

	// ┌─────────────────────────────────────────────────────────────┐
	// │ 阶段5: 等待 Gemini 处理视频                                 │
	// └─────────────────────────────────────────────────────────────┘
	g.App.Logger.Info("")
	g.App.Logger.Info("┌─────────────────────────────────────────────────────────────┐")
	g.App.Logger.Info("│ ⏳ 阶段5: 等待 Gemini 处理视频                               │")
	g.App.Logger.Info("└─────────────────────────────────────────────────────────────┘")
	g.App.Logger.Info("   状态: 处理中（视频越长处理时间越久）...")

	processStartTime := time.Now()
	err = client.WaitForFileProcessing(ctx, uploadedFile)
	processDuration := time.Since(processStartTime)

	if err != nil {
		g.App.Logger.Errorf("   ✗ 处理失败 (耗时 %.1f秒): %v", processDuration.Seconds(), err)
		if strings.Contains(err.Error(), "context deadline exceeded") {
			g.App.Logger.Error("   原因: 处理超时")
			g.App.Logger.Errorf("   建议: 视频较长，建议增加超时时间或使用关键帧分析模式")
		}
		return false
	}
	g.App.Logger.Infof("   ✓ 处理完成 (耗时 %.1f秒)", processDuration.Seconds())

	// ┌─────────────────────────────────────────────────────────────┐
	// │ 阶段6: 调用 AI 生成元数据                                   │
	// └─────────────────────────────────────────────────────────────┘
	g.App.Logger.Info("")
	g.App.Logger.Info("┌─────────────────────────────────────────────────────────────┐")
	g.App.Logger.Info("│ 🤖 阶段6: 调用 AI 生成元数据                                 │")
	g.App.Logger.Info("└─────────────────────────────────────────────────────────────┘")
	g.App.Logger.Info("   状态: AI 正在分析视频内容...")

	generateStartTime := time.Now()
	metadata, err := client.GenerateMetadataFromVideo(ctx, uploadedFile)
	generateDuration := time.Since(generateStartTime)

	if err != nil {
		g.App.Logger.Errorf("   ✗ 生成失败 (耗时 %.1f秒): %v", generateDuration.Seconds(), err)
		if strings.Contains(err.Error(), "context deadline exceeded") {
			g.App.Logger.Error("   原因: 生成超时")
		}
		return false
	}
	g.App.Logger.Infof("   ✓ 生成完成 (耗时 %.1f秒)", generateDuration.Seconds())

	// ┌─────────────────────────────────────────────────────────────┐
	// │ 生成结果预览                                                │
	// └─────────────────────────────────────────────────────────────┘
	g.App.Logger.Info("")
	g.App.Logger.Info("┌─────────────────────────────────────────────────────────────┐")
	g.App.Logger.Info("│ 📋 生成结果预览                                              │")
	g.App.Logger.Info("└─────────────────────────────────────────────────────────────┘")
	g.App.Logger.Infof("   标题: %s", metadata.Title)
	if len(metadata.Description) > 80 {
		g.App.Logger.Infof("   描述: %s...", metadata.Description[:80])
	} else {
		g.App.Logger.Infof("   描述: %s", metadata.Description)
	}
	g.App.Logger.Infof("   标签: %v", metadata.Tags)

	// 保存结果
	return g.saveMetadataResults(metadata, taskContext)
}

// executeWithKeyframeAnalysis 使用关键帧分析（适用于超大视频）
func (g *GenerateMetadata) executeWithKeyframeAnalysis(taskContext map[string]interface{}, videoPath string, fileSizeMB float64) bool {
	g.App.Logger.Info("")
	g.App.Logger.Info("┌─────────────────────────────────────────────────────────────┐")
	g.App.Logger.Info("│ 🖼️  关键帧分析模式（适用于超大视频）                         │")
	g.App.Logger.Info("└─────────────────────────────────────────────────────────────┘")
	g.App.Logger.Infof("   视频大小: %.2f MB (超过 %d MB 限制)", fileSizeMB, MaxGeminiUploadSizeMB)
	g.App.Logger.Info("   策略: 使用视频缩略图 + 字幕文本进行分析")

	// 1. 查找缩略图
	thumbnailPath := g.findBestThumbnail()
	if thumbnailPath == "" {
		g.App.Logger.Warn("   ⚠️ 未找到缩略图，回退到纯文本分析")
		return g.executeWithGeminiText(taskContext)
	}
	g.App.Logger.Infof("   ✓ 找到缩略图: %s", filepath.Base(thumbnailPath))

	// 2. 读取字幕文本（如果有）
	var subtitleText string
	zhSRTPath := filepath.Join(g.StateManager.CurrentDir, "zh.srt")
	if content, err := os.ReadFile(zhSRTPath); err == nil {
		subtitleText = g.extractTextFromSRT(string(content))
		if len(subtitleText) > 2000 {
			subtitleText = subtitleText[:2000] + "..."
		}
		g.App.Logger.Infof("   ✓ 找到字幕文本: %d 字符", len(subtitleText))
	} else {
		g.App.Logger.Info("   ℹ️ 无字幕文本，仅使用图片分析")
	}

	// 3. 创建 Gemini 客户端
	apiKey := g.App.Config.GeminiConfig.GetCurrentApiKey()
	client, err := NewGeminiClient(
		apiKey,
		g.App.Config.GeminiConfig.Model,
		g.App.Config.GeminiConfig.Timeout,
		g.App.Config.GeminiConfig.MaxTokens,
	)
	if err != nil {
		g.App.Logger.Errorf("   ✗ 创建 Gemini 客户端失败: %v", err)
		return false
	}
	defer client.Close()

	// 4. 使用图片+文本生成元数据
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(g.App.Config.GeminiConfig.Timeout)*time.Second)
	defer cancel()

	g.App.Logger.Info("   状态: 分析缩略图和字幕...")
	metadata, err := client.GenerateMetadataFromImage(ctx, thumbnailPath, subtitleText)
	if err != nil {
		g.App.Logger.Errorf("   ✗ 关键帧分析失败: %v", err)
		// 最后回退到纯文本分析
		if subtitleText != "" {
			g.App.Logger.Info("   🔄 回退到纯文本分析...")
			return g.executeWithGeminiText(taskContext)
		}
		return false
	}

	g.App.Logger.Info("   ✓ 关键帧分析完成")
	g.App.Logger.Infof("   标题: %s", metadata.Title)

	return g.saveMetadataResults(metadata, taskContext)
}

// findBestThumbnail 查找最佳缩略图
func (g *GenerateMetadata) findBestThumbnail() string {
	// 按优先级查找缩略图
	thumbnailNames := []string{
		"maxresdefault.jpg",
		"sddefault.jpg",
		"hqdefault.jpg",
		"mqdefault.jpg",
		"default.jpg",
		"thumbnail.jpg",
		"cover.jpg",
	}

	for _, name := range thumbnailNames {
		path := filepath.Join(g.StateManager.CurrentDir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// 查找任意 jpg/png 图片
	entries, err := os.ReadDir(g.StateManager.CurrentDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
			return filepath.Join(g.StateManager.CurrentDir, entry.Name())
		}
	}

	return ""
}

// executeWithGeminiText 使用 Gemini 分析字幕文本生成元数据
func (g *GenerateMetadata) executeWithGeminiText(taskContext map[string]interface{}) bool {
	g.App.Logger.Info("📝 使用 Gemini 分析字幕文本...")

	// 1. 检查中文字幕文件
	zhSRTPath := filepath.Join(g.StateManager.CurrentDir, "zh.srt")
	g.App.Logger.Infof("🔍 检查中文字幕文件: %s", zhSRTPath)
	if _, err := os.Stat(zhSRTPath); os.IsNotExist(err) {
		g.App.Logger.Warnf("⚠️ 中文字幕文件不存在: %s", zhSRTPath)
		g.App.Logger.Warn("⚠️ 请确保字幕翻译步骤已成功完成")
		return false
	}

	// 2. 读取字幕内容
	srtContent, err := os.ReadFile(zhSRTPath)
	if err != nil {
		g.App.Logger.Errorf("❌ 读取字幕文件失败: %v", err)
		return false
	}

	// 3. 提取文本
	subtitleText := g.extractTextFromSRT(string(srtContent))
	if subtitleText == "" {
		g.App.Logger.Warn("⚠️ 字幕内容为空")
		return false
	}

	g.App.Logger.Infof("📝 提取到字幕文本，总长度: %d 字符", len(subtitleText))

	// 4. 截取文本（避免token过多）
	maxLength := 2000
	if len(subtitleText) > maxLength {
		subtitleText = subtitleText[:maxLength] + "..."
	}

	// 5. 创建 Gemini 客户端（使用轮询 API Key）
	apiKey := g.App.Config.GeminiConfig.GetCurrentApiKey()
	keyCount := g.App.Config.GeminiConfig.GetApiKeysCount()
	keyIndex := g.App.Config.GeminiConfig.CurrentKeyIndex + 1
	g.App.Logger.Infof("🔧 创建 Gemini 客户端 (API Key %d/%d)...", keyIndex, keyCount)

	client, err := NewGeminiClient(
		apiKey,
		g.App.Config.GeminiConfig.Model,
		g.App.Config.GeminiConfig.Timeout,
		g.App.Config.GeminiConfig.MaxTokens,
	)
	if err != nil {
		g.App.Logger.Errorf("❌ 创建 Gemini 客户端失败: %v", err)
		// 尝试轮换到下一个 API Key
		if keyCount > 1 {
			g.App.Config.GeminiConfig.RotateApiKey()
			g.App.Logger.Infof("🔄 轮换到下一个 API Key...")
		}
		return false
	}
	defer client.Close()

	// 6. 生成元数据
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(g.App.Config.GeminiConfig.Timeout)*time.Second)
	defer cancel()

	g.App.Logger.Info("🤖 调用 Gemini 生成元数据...")
	metadata, err := client.GenerateMetadataFromText(ctx, subtitleText)
	if err != nil {
		g.App.Logger.Errorf("❌ 生成元数据失败: %v", err)
		return false
	}

	// 7. 保存结果
	return g.saveMetadataResults(metadata, taskContext)
}

// saveMetadataResults 保存元数据结果到context和数据库
func (g *GenerateMetadata) saveMetadataResults(metadata *VideoMetadata, taskContext map[string]interface{}) bool {
	// 1. 验证标题长度
	if len([]rune(metadata.Title)) > 80 {
		runes := []rune(metadata.Title)
		metadata.Title = string(runes[:77]) + "..."
		g.App.Logger.Warnf("⚠️ 标题过长，已截断为80字符")
	}

	// 2. 保存到 context
	taskContext["video_title"] = metadata.Title
	taskContext["video_description"] = metadata.Description
	taskContext["video_tags"] = metadata.Tags

	// 3. 保存到 meta.json 文件
	g.App.Logger.Info("💾 保存元数据到 meta.json 文件...")
	if err := g.saveMetadataToFile(metadata); err != nil {
		g.App.Logger.Errorf("❌ 保存 meta.json 文件失败: %v", err)
	} else {
		g.App.Logger.Info("✅ meta.json 文件已保存")
	}

	// 4. 保存到数据库
	g.App.Logger.Info("💾 保存生成的元数据到数据库...")
	savedVideo, err := g.SavedVideoService.GetVideoByVideoID(g.StateManager.VideoID)
	if err != nil {
		g.App.Logger.Errorf("❌ 获取视频记录失败: %v", err)
	} else {
		savedVideo.GeneratedTitle = metadata.Title
		savedVideo.GeneratedDesc = metadata.Description
		savedVideo.GeneratedTags = strings.Join(metadata.Tags, ",")

		if err := g.SavedVideoService.UpdateVideo(savedVideo); err != nil {
			g.App.Logger.Errorf("❌ 保存元数据到数据库失败: %v", err)
		} else {
			g.App.Logger.Info("✅ 元数据已保存到数据库")
		}
	}

	// 5. 输出生成结果
	g.App.Logger.Info("========================================")
	g.App.Logger.Info("✅ 视频元数据生成成功！")
	g.App.Logger.Infof("📌 标题: %s", metadata.Title)
	g.App.Logger.Infof("📝 描述: %s", g.truncateString(metadata.Description, 100))
	g.App.Logger.Infof("🏷️ 标签: %v", metadata.Tags)
	g.App.Logger.Info("========================================")

	return true
}

// findVideoFiles 查找视频文件
func (g *GenerateMetadata) findVideoFiles() []string {
	var videoFiles []string
	videoExtensions := []string{".mp4", ".flv", ".mkv", ".webm", ".avi", ".mov"}

	files, err := os.ReadDir(g.StateManager.CurrentDir)
	if err != nil {
		g.App.Logger.Errorf("读取目录失败: %v", err)
		return videoFiles
	}

	g.App.Logger.Debugf("🔍 扫描目录: %s, 共 %d 个文件/文件夹", g.StateManager.CurrentDir, len(files))

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(file.Name()))
		for _, videoExt := range videoExtensions {
			if ext == videoExt {
				fullPath := filepath.Join(g.StateManager.CurrentDir, file.Name())
				videoFiles = append(videoFiles, fullPath)
				g.App.Logger.Debugf("✓ 找到视频文件: %s", file.Name())
				break
			}
		}
	}

	if len(videoFiles) == 0 {
		g.App.Logger.Debugf("⚠️ 目录中未找到视频文件")
	} else {
		g.App.Logger.Debugf("📹 共找到 %d 个视频文件", len(videoFiles))
	}

	return videoFiles
}

// logDirectoryContents 记录目录内容，帮助调试
func (g *GenerateMetadata) logDirectoryContents() {
	files, err := os.ReadDir(g.StateManager.CurrentDir)
	if err != nil {
		g.App.Logger.Errorf("❌ 无法读取工作目录: %v", err)
		return
	}

	g.App.Logger.Infof("📂 工作目录文件列表 (%d 个):", len(files))
	for _, file := range files {
		if file.IsDir() {
			g.App.Logger.Infof("   📁 [目录] %s", file.Name())
		} else {
			if info, err := file.Info(); err == nil {
				sizeMB := float64(info.Size()) / 1024 / 1024
				if sizeMB >= 1 {
					g.App.Logger.Infof("   📄 %s (%.2f MB)", file.Name(), sizeMB)
				} else {
					sizeKB := float64(info.Size()) / 1024
					g.App.Logger.Infof("   📄 %s (%.2f KB)", file.Name(), sizeKB)
				}
			} else {
				g.App.Logger.Infof("   📄 %s", file.Name())
			}
		}
	}
}
