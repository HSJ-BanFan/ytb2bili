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
	"github.com/difyz9/ytb2bili/pkg/cos"
	"github.com/difyz9/ytb2bili/pkg/prompts"
	"github.com/google/generative-ai-go/genai"
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

// sanitizeUTF8 清洗文本中的非法 UTF-8 字符
func sanitizeUTF8(text string) string {
	return strings.ToValidUTF8(text, "")
}

// checkUserPermission 检查用户是否有 AI 元数据生成权限
func (g *GenerateMetadata) checkUserPermission() bool {
	// 获取视频信息，包含提交用户ID
	savedVideo, err := g.SavedVideoService.GetVideoByVideoID(g.StateManager.VideoID)
	if err != nil {
		g.App.Logger.Debugf("无法获取视频信息: %v，默认允许执行", err)
		return true // 获取失败时默认允许（向后兼容）
	}

	// 如果没有用户ID（旧数据），默认允许
	if savedVideo.UserID == 0 {
		return true
	}

	userID := strconv.FormatUint(uint64(savedVideo.UserID), 10)

	// 使用 PermissionService 检查权限
	permissionService := services.NewPermissionService(g.App.DB)

	// 获取用户会员信息用于日志
	userMembership, err := permissionService.GetUserMembership(context.Background(), userID)
	if err == nil {
		g.App.Logger.Infof("  │ 用户 %s (%s) - AI元数据权限检查",
			userID, userMembership.GetEffectiveTier())
	}

	// 检查 AI 元数据生成权限（使用 metadata_generation 作为 feature key）
	canUse, reason, err := permissionService.CanUseFeature(context.Background(), userID, "metadata_generation")
	if err != nil {
		g.App.Logger.Warnf("  │ 检查权限失败: %v，默认允许", err)
		return true
	}
	if !canUse {
		g.App.Logger.Warnf("  │ 用户 %s 无 AI 元数据权限: %s", userID, reason)
		return false
	}

	return true
}

func (g *GenerateMetadata) Execute(ctx map[string]interface{}) bool {
	startTime := time.Now()

	// 0. 检查用户会员权限
	if !g.checkUserPermission() {
		g.App.Logger.Warn("  │ ⚠️ 无 AI 权限，跳过 (升级 Pro 可解锁)")
		ctx["skipped"] = "需要 Pro 会员才能使用 AI 生成元数据功能"
		return true
	}

	// 列出工作目录中的文件
	g.logDirectoryContents()
	_ = startTime // 用于后续计时

	// 1. 刷新AI服务管理器配置
	g.AIManager.RefreshConfig(g.App.Config)

	// ⚠️ 元数据生成必须使用 Gemini（多模态视频分析能力）
	// 检查 Gemini 是否已配置（支持单个 ApiKey 或多个 ApiKeys）
	geminiConfigured := g.App.Config.GeminiConfig != nil && g.App.Config.GeminiConfig.Enabled &&
		(g.App.Config.GeminiConfig.ApiKey != "" || len(g.App.Config.GeminiConfig.ApiKeys) > 0)
	if !geminiConfigured {
		g.App.Logger.Warn("  │ Gemini 未配置，使用备选 AI...")
		return g.executeWithFallbackAI(ctx)
	}

	// 使用 Gemini 多模态服务
	g.App.Logger.Infof("  │ 🤖 Gemini: %s (视频分析: %v)",
		g.App.Config.GeminiConfig.Model, g.App.Config.GeminiConfig.AnalyzeVideo)

	// 如果配置了视频分析，尝试使用视频文件
	if g.App.Config.GeminiConfig.AnalyzeVideo {
		if success := g.executeWithGeminiVideo(ctx); success {
			return true
		}
		g.App.Logger.Warn("  │ 视频分析失败，回退到文本模式")
	}

	// 使用 Gemini 处理字幕文本
	if success := g.executeWithGeminiText(ctx); success {
		return true
	}

	// Gemini 失败时，使用备选AI服务
	g.App.Logger.Warn("  │ Gemini 失败，尝试备选 AI...")
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

	// 所有AI服务都不可用，设置跳过状态
	g.App.Logger.Warn("⚠️ 所有AI服务不可用，跳过元数据生成")
	ctx["skipped"] = "AI服务不可用(Gemini地区限制/额度不足/网络错误)"
	return true
}

// executeWithAIManager 使用AI服务管理器生成元数据（首选方式）
func (g *GenerateMetadata) executeWithAIManager(ctx map[string]interface{}) bool {
	g.App.Logger.Info("🔄 使用AI服务管理器生成元数据...")

	// 1. 检查字幕文件（优先中文，其次英文）
	srtPath := g.findSubtitleForMetadata()
	if srtPath == "" {
		reason := "未找到字幕文件，无法进行AI增强"
		g.App.Logger.Warnf("⚠️ %s", reason)
		ctx["skipped"] = reason
		ctx["video_title"] = g.StateManager.VideoID
		ctx["video_description"] = "包含字幕的视频"
		return true
	}

	// 2. 读取字幕内容
	srtContent, err := os.ReadFile(srtPath)
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
4. 语言：必须使用简体中文 (Simplified Chinese)

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
		reason := fmt.Sprintf("DeepSeek 服务不可用: %v", err)
		g.App.Logger.Warnf("⚠️ %s", reason)
		context["skipped"] = reason
		context["video_title"] = g.StateManager.VideoID
		context["video_description"] = "包含字幕的视频"
		return true
	}

	g.App.Logger.Infof("🔑 DeepSeek 客户端创建成功")
	// 更新当前使用的客户端
	g.DeepSeekClient = client

	// 1. 检查字幕文件（优先中文，其次英文）
	srtPath := g.findSubtitleForMetadata()
	if srtPath == "" {
		reason := "未找到字幕文件，无法进行AI增强"
		g.App.Logger.Warnf("⚠️ %s", reason)
		context["skipped"] = reason
		context["video_title"] = g.StateManager.VideoID
		context["video_description"] = "包含字幕的视频"
		return true
	}

	// 2. 读取字幕内容
	srtContent, err := os.ReadFile(srtPath)
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
4. 必须使用简体中文 (Simplified Chinese)
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

// findSubtitleForMetadata 查找用于生成元数据的字幕文件（优先中文，其次英文）
func (g *GenerateMetadata) findSubtitleForMetadata() string {
	currentDir := g.StateManager.CurrentDir
	videoID := g.StateManager.VideoID

	// 优先级列表
	priorities := []string{
		filepath.Join(currentDir, "zh.srt"),                               // 翻译后的中文字幕
		filepath.Join(currentDir, fmt.Sprintf("%s.zh-Hans.srt", videoID)), // YouTube 简体中文
		filepath.Join(currentDir, fmt.Sprintf("%s.zh-CN.srt", videoID)),   // YouTube 中文
		filepath.Join(currentDir, "en.srt"),                               // 英文字幕
		filepath.Join(currentDir, fmt.Sprintf("%s.en.srt", videoID)),      // YouTube 英文
	}

	for _, path := range priorities {
		if _, err := os.Stat(path); err == nil {
			g.App.Logger.Infof("🔍 找到字幕文件: %s", filepath.Base(path))
			return path
		}
	}

	// 查找任意 .srt 文件作为后备
	entries, err := os.ReadDir(currentDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".srt") {
				path := filepath.Join(currentDir, entry.Name())
				g.App.Logger.Infof("🔍 使用后备字幕文件: %s", entry.Name())
				return path
			}
		}
	}

	// 未找到字幕，输出诊断日志
	g.App.Logger.Warnf("❌ 未找到字幕文件，目录: %s", currentDir)
	if err == nil && len(entries) > 0 {
		var fileList []string
		for _, e := range entries {
			if !e.IsDir() {
				fileList = append(fileList, e.Name())
			}
		}
		g.App.Logger.Warnf("   目录中有 %d 个文件: %v", len(fileList), fileList)
		g.App.Logger.Warnf("   期望文件: zh.srt, en.srt, %s.zh-Hans.srt, %s.en.srt", videoID, videoID)
	} else if err != nil {
		g.App.Logger.Warnf("   读取目录失败: %v", err)
	}

	return ""
}

// 视频分析策略常量
const (
	// 视频大小阈值 (MB)
	SmallVideoThresholdMB     = 50   // 小视频：完整分析，快速处理
	MediumVideoThresholdMB    = 150  // 中等视频：完整分析，适度超时
	LargeVideoThresholdMB     = 300  // 大视频：推荐关键帧分析
	KeyframeAnalysisThreshold = 300  // 超过此大小自动使用关键帧分析
	MaxGeminiUploadSizeMB     = 2000 // Gemini 最大上传大小 (MB)
	MaxGeminiRetries          = 1    // 最大重试次数（减少无效等待）

	// 视频时长阈值 (分钟) - 用于智能策略选择
	ShortVideoDurationMin  = 10 // 短视频
	MediumVideoDurationMin = 30 // 中等视频
	LongVideoDurationMin   = 60 // 长视频

	// 处理时间预估系数
	GeminiProcessingRatioSmall  = 0.5 // 小视频：处理时间约为视频时长的 0.5 倍
	GeminiProcessingRatioMedium = 1.0 // 中等视频：处理时间约为视频时长
	GeminiProcessingRatioLarge  = 2.0 // 大视频：处理时间约为视频时长的 2 倍
)

// VideoAnalysisStrategy 视频分析策略
type VideoAnalysisStrategy struct {
	Mode           string  // 分析模式: "full", "keyframe", "text"
	SizeCategory   string  // 大小分类
	TimeoutSeconds int     // 超时时间
	EstimatedTime  float64 // 预估处理时间(秒)
	Recommendation string  // 推荐说明
}

// selectAnalysisStrategy 智能选择分析策略
func (g *GenerateMetadata) selectAnalysisStrategy(fileSizeMB float64) VideoAnalysisStrategy {
	baseTimeout := g.App.Config.GeminiConfig.Timeout
	if baseTimeout <= 0 {
		baseTimeout = 300
	}

	// 预估视频时长（假设平均码率 5Mbps = 0.625 MB/s）
	estimatedDurationMin := fileSizeMB / (0.625 * 60)

	var strategy VideoAnalysisStrategy

	switch {
	case fileSizeMB <= SmallVideoThresholdMB:
		// 小视频：完整分析，快速处理
		strategy = VideoAnalysisStrategy{
			Mode:           "full",
			SizeCategory:   "小视频",
			TimeoutSeconds: baseTimeout,
			EstimatedTime:  estimatedDurationMin * 60 * GeminiProcessingRatioSmall,
			Recommendation: "完整视频分析（快速模式）",
		}

	case fileSizeMB <= MediumVideoThresholdMB:
		// 中等视频：完整分析，适度超时
		strategy = VideoAnalysisStrategy{
			Mode:           "full",
			SizeCategory:   "中等视频",
			TimeoutSeconds: int(float64(baseTimeout) * 1.5),
			EstimatedTime:  estimatedDurationMin * 60 * GeminiProcessingRatioMedium,
			Recommendation: "完整视频分析（标准模式）",
		}

	case fileSizeMB <= KeyframeAnalysisThreshold:
		// 大视频但未超阈值：给用户选择，默认关键帧
		estimatedFullTime := estimatedDurationMin * 60 * GeminiProcessingRatioLarge
		if estimatedFullTime > float64(baseTimeout*2) {
			// 预估时间过长，推荐关键帧分析
			strategy = VideoAnalysisStrategy{
				Mode:           "keyframe",
				SizeCategory:   "大视频",
				TimeoutSeconds: baseTimeout,
				EstimatedTime:  30, // 关键帧分析通常很快
				Recommendation: "关键帧分析（推荐，避免超时）",
			}
		} else {
			strategy = VideoAnalysisStrategy{
				Mode:           "full",
				SizeCategory:   "大视频",
				TimeoutSeconds: int(float64(baseTimeout) * 2),
				EstimatedTime:  estimatedFullTime,
				Recommendation: "完整视频分析（增强超时）",
			}
		}

	case fileSizeMB <= MaxGeminiUploadSizeMB:
		// 超大视频：强制关键帧分析
		strategy = VideoAnalysisStrategy{
			Mode:           "keyframe",
			SizeCategory:   "超大视频",
			TimeoutSeconds: baseTimeout,
			EstimatedTime:  30,
			Recommendation: "关键帧分析（视频过大，避免处理超时）",
		}

	default:
		// 巨型视频：只能用关键帧
		strategy = VideoAnalysisStrategy{
			Mode:           "keyframe",
			SizeCategory:   "巨型视频",
			TimeoutSeconds: baseTimeout,
			EstimatedTime:  30,
			Recommendation: "关键帧分析（超出上传限制）",
		}
	}

	// 限制最大超时时间
	if strategy.TimeoutSeconds > 900 { // 最大15分钟
		strategy.TimeoutSeconds = 900
	}

	return strategy
}

// executeWithGeminiVideo 使用 Gemini 分析视频文件生成元数据
func (g *GenerateMetadata) executeWithGeminiVideo(taskContext map[string]interface{}) bool {
	totalStartTime := time.Now()

	// ═══════════════════════════════════════════════════════════════
	g.App.Logger.Info("")
	g.App.Logger.Info("╔══════════════════════════════════════════════════════════════╗")
	g.App.Logger.Info("║           🎬 Gemini 视频分析                                 ║")
	g.App.Logger.Info("╚══════════════════════════════════════════════════════════════╝")

	// ───────────────────────────────────────────────────────────────
	// 阶段1: 查找视频文件
	// ───────────────────────────────────────────────────────────────
	g.App.Logger.Info("")
	g.App.Logger.Info("📁 [1/6] 查找视频文件")

	videoFiles := g.findVideoFiles()
	if len(videoFiles) == 0 {
		g.App.Logger.Error("   ✗ 未找到视频文件")
		return false
	}
	videoPath := videoFiles[0]

	fileInfo, err := os.Stat(videoPath)
	if err != nil {
		g.App.Logger.Errorf("   ✗ 无法获取文件信息: %v", err)
		return false
	}
	fileSizeMB := float64(fileInfo.Size()) / 1024 / 1024
	g.App.Logger.Infof("   ✓ %s (%.1f MB)", filepath.Base(videoPath), fileSizeMB)

	// ───────────────────────────────────────────────────────────────
	// 阶段2: 智能策略选择
	// ───────────────────────────────────────────────────────────────
	g.App.Logger.Info("")
	g.App.Logger.Info("🎯 [2/6] 分析策略选择")

	strategy := g.selectAnalysisStrategy(fileSizeMB)

	g.App.Logger.Infof("   分类: %s", strategy.SizeCategory)
	g.App.Logger.Infof("   策略: %s", strategy.Recommendation)
	g.App.Logger.Infof("   预估: %.0f秒 | 超时: %d秒", strategy.EstimatedTime, strategy.TimeoutSeconds)

	// 根据策略执行不同的分析模式
	if strategy.Mode == "keyframe" {
		g.App.Logger.Info("   → 使用关键帧分析模式（更快更稳定）")
		return g.executeWithKeyframeAnalysis(taskContext, videoPath, fileSizeMB)
	}

	// 完整视频分析
	g.App.Logger.Info("   → 使用完整视频分析模式")

	// 执行视频分析（带重试）
	for retry := 0; retry <= MaxGeminiRetries; retry++ {
		if retry > 0 {
			g.App.Logger.Info("")
			g.App.Logger.Warnf("🔄 重试 %d/%d...", retry, MaxGeminiRetries)
			// 重试时轮换 API Key
			if g.App.Config.GeminiConfig.GetApiKeysCount() > 1 {
				g.App.Config.GeminiConfig.RotateApiKey()
				g.App.Logger.Info("   已轮换 API Key")
			}
			// 重试时切换到关键帧模式（避免再次超时）
			g.App.Logger.Info("   → 切换到关键帧分析模式")
			return g.executeWithKeyframeAnalysis(taskContext, videoPath, fileSizeMB)
		}

		success := g.executeGeminiVideoAnalysis(taskContext, videoPath, fileSizeMB, strategy.TimeoutSeconds, retry)
		if success {
			totalDuration := time.Since(totalStartTime)
			g.App.Logger.Info("")
			g.App.Logger.Infof("✅ 视频分析完成！总耗时: %.1f秒", totalDuration.Seconds())
			return true
		}
	}

	// 所有重试都失败，尝试关键帧分析作为后备
	g.App.Logger.Warn("")
	g.App.Logger.Warn("⚠️ 完整分析失败，切换到关键帧分析...")
	return g.executeWithKeyframeAnalysis(taskContext, videoPath, fileSizeMB)
}

// executeGeminiVideoAnalysis 执行单次 Gemini 视频分析
func (g *GenerateMetadata) executeGeminiVideoAnalysis(taskContext map[string]interface{}, videoPath string, fileSizeMB float64, timeoutSeconds int, retryCount int) bool {
	// ───────────────────────────────────────────────────────────────
	// 阶段3: 创建 Gemini 客户端
	// ───────────────────────────────────────────────────────────────
	g.App.Logger.Info("")
	g.App.Logger.Info("🔧 [3/6] 创建 Gemini 客户端")

	apiKey := g.App.Config.GeminiConfig.GetCurrentApiKey()
	keyCount := g.App.Config.GeminiConfig.GetApiKeysCount()
	keyIndex := g.App.Config.GeminiConfig.CurrentKeyIndex + 1

	// 隐藏 API Key 中间部分
	maskedKey := apiKey
	if len(apiKey) > 10 {
		maskedKey = apiKey[:4] + "****" + apiKey[len(apiKey)-4:]
	}
	g.App.Logger.Infof("   Key: %s (%d/%d) | 模型: %s | 超时: %ds",
		maskedKey, keyIndex, keyCount, g.App.Config.GeminiConfig.Model, timeoutSeconds)

	client, err := NewGeminiClient(
		apiKey,
		g.App.Config.GeminiConfig.Model,
		timeoutSeconds,
		g.App.Config.GeminiConfig.MaxTokens,
	)
	if err != nil {
		g.App.Logger.Errorf("   ✗ 创建失败: %v", err)
		return false
	}
	defer client.Close()
	g.App.Logger.Info("   ✓ 客户端就绪")

	// 创建带超时的 context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// ───────────────────────────────────────────────────────────────
	// 阶段4: 上传视频到 Gemini
	// ───────────────────────────────────────────────────────────────
	g.App.Logger.Info("")
	g.App.Logger.Info("⏫ [4/6] 上传视频到 Gemini")
	estimatedUploadTime := fileSizeMB / 5.0
	g.App.Logger.Infof("   文件: %.1f MB | 预估: %.0f秒", fileSizeMB, estimatedUploadTime)

	uploadStartTime := time.Now()
	uploadedFile, err := client.UploadFile(ctx, videoPath, filepath.Base(videoPath))
	uploadDuration := time.Since(uploadStartTime)

	if err != nil {
		g.App.Logger.Errorf("   ✗ 上传失败 (%.1fs): %v", uploadDuration.Seconds(), err)
		if strings.Contains(err.Error(), "context deadline exceeded") {
			g.App.Logger.Error("   原因: 上传超时，建议使用关键帧分析模式")
		}
		return false
	}

	actualSpeed := fileSizeMB / uploadDuration.Seconds()
	g.App.Logger.Infof("   ✓ 上传完成 (%.1fs, %.1fMB/s)", uploadDuration.Seconds(), actualSpeed)

	// ───────────────────────────────────────────────────────────────
	// 阶段5: 等待 Gemini 处理视频 (带进度显示)
	// ───────────────────────────────────────────────────────────────
	g.App.Logger.Info("")
	g.App.Logger.Info("⏳ [5/6] 等待 Gemini 处理视频")

	processStartTime := time.Now()
	err = g.waitForFileProcessingWithProgress(ctx, client, uploadedFile, timeoutSeconds)
	processDuration := time.Since(processStartTime)

	if err != nil {
		g.App.Logger.Errorf("   ✗ 处理失败 (%.1fs): %v", processDuration.Seconds(), err)
		if strings.Contains(err.Error(), "context deadline exceeded") {
			g.App.Logger.Error("   原因: 处理超时，建议使用关键帧分析模式")
		}
		return false
	}
	g.App.Logger.Infof("   ✓ 处理完成 (%.1fs)", processDuration.Seconds())

	// ───────────────────────────────────────────────────────────────
	// 阶段6: 调用 AI 生成元数据
	// ───────────────────────────────────────────────────────────────
	g.App.Logger.Info("")
	g.App.Logger.Info("🤖 [6/6] AI 生成元数据")

	generateStartTime := time.Now()
	metadata, err := client.GenerateMetadataFromVideo(ctx, uploadedFile)
	generateDuration := time.Since(generateStartTime)

	if err != nil {
		g.App.Logger.Errorf("   ✗ 生成失败 (%.1fs): %v", generateDuration.Seconds(), err)
		return false
	}
	g.App.Logger.Infof("   ✓ 生成完成 (%.1fs)", generateDuration.Seconds())

	// 结果预览
	g.App.Logger.Info("")
	g.App.Logger.Info("📋 生成结果:")
	g.App.Logger.Infof("   标题: %s", metadata.Title)
	g.App.Logger.Infof("   标签: %v", metadata.Tags)

	return g.saveMetadataResults(metadata, taskContext)
}

// waitForFileProcessingWithProgress 带进度显示的文件处理等待
func (g *GenerateMetadata) waitForFileProcessingWithProgress(ctx context.Context, client *GeminiClient, file *genai.File, timeoutSeconds int) error {
	lastLogTime := time.Now()

	callback := func(elapsed time.Duration, state string) {
		// 每10秒输出一次进度
		if time.Since(lastLogTime) >= 10*time.Second {
			remaining := timeoutSeconds - int(elapsed.Seconds())
			if remaining < 0 {
				remaining = 0
			}
			g.App.Logger.Infof("   … 处理中: %.0fs/剩余%ds", elapsed.Seconds(), remaining)
			lastLogTime = time.Now()
		}
	}

	return client.WaitForFileProcessingWithCallback(ctx, file, callback)
}

// executeWithKeyframeAnalysis 使用关键帧分析（适用于大视频，更快更稳定）
func (g *GenerateMetadata) executeWithKeyframeAnalysis(taskContext map[string]interface{}, videoPath string, fileSizeMB float64) bool {
	startTime := time.Now()

	g.App.Logger.Info("")
	g.App.Logger.Info("🖼️  关键帧分析模式（快速稳定）")
	g.App.Logger.Infof("   视频: %.1f MB | 策略: 缩略图 + 字幕文本", fileSizeMB)

	imagePaths := g.findThumbnails(4)
	if len(imagePaths) == 0 {
		g.App.Logger.Warn("   ⚠️ 未找到缩略图")
	} else {
		g.App.Logger.Infof("   ✓ 缩略图: %s", filepath.Base(imagePaths[0]))
		if len(imagePaths) > 1 {
			g.App.Logger.Infof("   ✓ 额外图片: %d", len(imagePaths)-1)
		}
	}

	// 2. 读取字幕文本（如果有）
	var subtitleText string
	zhSRTPath := filepath.Join(g.StateManager.CurrentDir, "zh.srt")
	if content, err := os.ReadFile(zhSRTPath); err == nil {
		subtitleText = g.extractTextFromSRT(string(content))
		if len(subtitleText) > 3000 {
			subtitleText = subtitleText[:3000] + "..."
		}
		g.App.Logger.Infof("   ✓ 字幕: %d 字符", len(subtitleText))
	} else {
		g.App.Logger.Info("   ℹ️ 无字幕，仅用图片分析")
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
		g.App.Logger.Errorf("   ✗ 创建客户端失败: %v", err)
		return false
	}
	defer client.Close()

	// 4. 使用图片+文本生成元数据
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(g.App.Config.GeminiConfig.Timeout)*time.Second)
	defer cancel()

	contextText := g.buildGeminiKeyframeContextText(taskContext, subtitleText)

	if len(imagePaths) == 0 {
		if strings.TrimSpace(contextText) == "" {
			return false
		}
		g.App.Logger.Info("   … AI 分析中（文本模式）...")
		metadata, err := client.GenerateMetadataFromText(ctx, contextText)
		if err != nil {
			g.App.Logger.Errorf("   ✗ 分析失败: %v", err)
			return false
		}
		duration := time.Since(startTime)
		g.App.Logger.Infof("   ✓ 完成 (%.1fs)", duration.Seconds())
		g.App.Logger.Infof("   标题: %s", metadata.Title)
		g.App.Logger.Infof("   标签: %v", metadata.Tags)
		return g.saveMetadataResults(metadata, taskContext)
	}

	g.App.Logger.Info("   … AI 分析中...")
	metadata, err := client.GenerateMetadataFromImages(ctx, imagePaths, contextText)
	if err != nil {
		g.App.Logger.Errorf("   ✗ 分析失败: %v", err)
		if strings.TrimSpace(contextText) != "" {
			g.App.Logger.Info("   → 回退到文本模式")
			metadata, err = client.GenerateMetadataFromText(ctx, contextText)
			if err != nil {
				g.App.Logger.Errorf("   ✗ 文本回退失败: %v", err)
				return false
			}
		} else {
			return false
		}
	}

	duration := time.Since(startTime)
	g.App.Logger.Infof("   ✓ 完成 (%.1fs)", duration.Seconds())
	g.App.Logger.Infof("   标题: %s", metadata.Title)
	g.App.Logger.Infof("   标签: %v", metadata.Tags)

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

func (g *GenerateMetadata) findThumbnails(maxCount int) []string {
	if maxCount <= 0 {
		return nil
	}

	thumbnailNames := []string{
		"maxresdefault.jpg",
		"sddefault.jpg",
		"hqdefault.jpg",
		"mqdefault.jpg",
		"default.jpg",
		"thumbnail.jpg",
		"cover.jpg",
	}

	seen := make(map[string]struct{}, maxCount)
	result := make([]string, 0, maxCount)
	add := func(path string) {
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}

	for _, name := range thumbnailNames {
		path := filepath.Join(g.StateManager.CurrentDir, name)
		if _, err := os.Stat(path); err == nil {
			add(path)
			if len(result) >= maxCount {
				return result
			}
		}
	}

	entries, err := os.ReadDir(g.StateManager.CurrentDir)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
			add(filepath.Join(g.StateManager.CurrentDir, entry.Name()))
			if len(result) >= maxCount {
				return result
			}
		}
	}

	return result
}

func (g *GenerateMetadata) buildGeminiKeyframeContextText(taskContext map[string]interface{}, subtitleText string) string {
	parts := make([]string, 0, 6)
	if strings.TrimSpace(subtitleText) != "" {
		parts = append(parts, g.truncateString(subtitleText, 2500))
	}

	if v, ok := taskContext["original_title"].(string); ok {
		v = strings.TrimSpace(v)
		if v != "" {
			parts = append(parts, "原始标题: "+g.truncateString(v, 200))
		}
	}
	if v, ok := taskContext["original_description"].(string); ok {
		v = strings.TrimSpace(v)
		if v != "" {
			parts = append(parts, "原始简介: "+g.truncateString(v, 1200))
		}
	}

	if v, ok := taskContext["video_uploader"].(string); ok {
		v = strings.TrimSpace(v)
		if v != "" {
			parts = append(parts, "上传者: "+g.truncateString(v, 200))
		}
	}
	if v, ok := taskContext["video_duration"].(int); ok && v > 0 {
		parts = append(parts, fmt.Sprintf("时长: %d秒", v))
	} else if v, ok := taskContext["video_duration"].(float64); ok && v > 0 {
		parts = append(parts, fmt.Sprintf("时长: %.0f秒", v))
	} else if v, ok := taskContext["video_duration"].(string); ok {
		v = strings.TrimSpace(v)
		if v != "" {
			parts = append(parts, "时长: "+g.truncateString(v, 50))
		}
	}

	if g.SavedVideoService != nil {
		if savedVideo, err := g.SavedVideoService.GetVideoByVideoID(g.StateManager.VideoID); err == nil {
			title := strings.TrimSpace(savedVideo.Title)
			if title != "" {
				parts = append(parts, "数据库标题: "+g.truncateString(title, 200))
			}
			desc := strings.TrimSpace(savedVideo.Description)
			if desc != "" {
				parts = append(parts, "数据库简介: "+g.truncateString(desc, 1200))
			}
		}
	}

	text := strings.TrimSpace(strings.Join(parts, "\n"))
	if text == "" {
		return ""
	}
	return g.truncateString(text, 3000)
}

// executeWithGeminiText 使用 Gemini 分析字幕文本生成元数据
func (g *GenerateMetadata) executeWithGeminiText(taskContext map[string]interface{}) bool {
	g.App.Logger.Info("📝 使用 Gemini 分析字幕文本...")

	// 1. 检查字幕文件（优先中文，其次英文）
	srtPath := g.findSubtitleForMetadata()
	if srtPath == "" {
		g.App.Logger.Warn("⚠️ 未找到任何字幕文件（中文或英文）")
		return false
	}

	// 2. 读取字幕内容
	srtContent, err := os.ReadFile(srtPath)
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

// logDirectoryContents 记录目录内容（简洁版）
func (g *GenerateMetadata) logDirectoryContents() {
	files, err := os.ReadDir(g.StateManager.CurrentDir)
	if err != nil {
		return
	}

	// 统计文件类型
	var videoFile, subtitleFiles, imageFiles int
	var videoSize int64
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		info, _ := file.Info()
		if strings.HasSuffix(name, ".mp4") || strings.HasSuffix(name, ".webm") {
			videoFile++
			if info != nil {
				videoSize = info.Size()
			}
		} else if strings.HasSuffix(name, ".srt") || strings.HasSuffix(name, ".vtt") {
			subtitleFiles++
		} else if strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".png") {
			imageFiles++
		}
	}

	// 一行输出摘要
	videoSizeMB := float64(videoSize) / 1024 / 1024
	g.App.Logger.Infof("  │ 📂 文件: 视频 %d (%.0fMB), 字幕 %d, 图片 %d",
		videoFile, videoSizeMB, subtitleFiles, imageFiles)
}
