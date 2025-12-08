package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/difyz9/ytb2bili/pkg/prompts"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiClient Gemini API 客户端
type GeminiClient struct {
	client    *genai.Client
	model     string
	timeout   time.Duration
	maxTokens int
}

// NewGeminiClient 创建新的 Gemini 客户端
// 注意：Gemini 原生 API 必须使用官方地址，不支持自定义代理（文件上传功能需要）
func NewGeminiClient(apiKey string, model string, timeout int, maxTokens int) (*GeminiClient, error) {
	ctx := context.Background()

	// 使用官方 API 地址（不支持自定义代理，因为文件上传需要官方端点）
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("创建 Gemini 客户端失败: %v", err)
	}

	if model == "" {
		model = "gemini-1.5-pro"
	}

	if timeout <= 0 {
		timeout = 120
	}

	if maxTokens <= 0 {
		maxTokens = 8000
	}

	return &GeminiClient{
		client:    client,
		model:     model,
		timeout:   time.Duration(timeout) * time.Second,
		maxTokens: maxTokens,
	}, nil
}

// Close 关闭客户端
func (g *GeminiClient) Close() error {
	return g.client.Close()
}

// TestConnection 测试 Gemini API 连接和可用性
func (g *GeminiClient) TestConnection(ctx context.Context) error {
	// 使用一个简单的文本生成来测试连接
	model := g.client.GenerativeModel(g.model)
	model.SetMaxOutputTokens(50)
	model.SetTemperature(0.1)

	testPrompt := "请回复：OK"

	resp, err := model.GenerateContent(ctx, genai.Text(testPrompt))
	if err != nil {
		return fmt.Errorf("API 调用失败: %v", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return fmt.Errorf("API 返回空响应")
	}

	return nil
}

// UploadFile 上传文件到 Gemini
func (g *GeminiClient) UploadFile(ctx context.Context, filePath string, displayName string) (*genai.File, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	uploadedFile, err := g.client.UploadFile(ctx, "", file, &genai.UploadFileOptions{
		DisplayName: displayName,
	})
	if err != nil {
		return nil, fmt.Errorf("上传文件失败: %v", err)
	}

	return uploadedFile, nil
}

// WaitForFileProcessing 等待文件处理完成
func (g *GeminiClient) WaitForFileProcessing(ctx context.Context, file *genai.File) error {
	for {
		fileInfo, err := g.client.GetFile(ctx, file.Name)
		if err != nil {
			return fmt.Errorf("获取文件状态失败: %v", err)
		}

		if fileInfo.State == genai.FileStateActive {
			return nil
		}

		if fileInfo.State == genai.FileStateFailed {
			return fmt.Errorf("文件处理失败")
		}

		// 等待一段时间后重试
		time.Sleep(2 * time.Second)
	}
}

// GenerateMetadataFromVideo 从视频生成元数据（标题、描述、标签）
func (g *GeminiClient) GenerateMetadataFromVideo(ctx context.Context, videoFile *genai.File) (*VideoMetadata, error) {
	// 直接使用模型名称，SDK会自动处理
	model := g.client.GenerativeModel(g.model)

	// 设置生成参数
	model.SetMaxOutputTokens(int32(g.maxTokens))
	model.SetTemperature(0.7)

	// 设置安全设置 - 降低过滤级别以处理更多类型的视频内容
	model.SafetySettings = []*genai.SafetySetting{
		{Category: genai.HarmCategoryHarassment, Threshold: genai.HarmBlockNone},
		{Category: genai.HarmCategoryHateSpeech, Threshold: genai.HarmBlockNone},
		{Category: genai.HarmCategorySexuallyExplicit, Threshold: genai.HarmBlockNone},
		{Category: genai.HarmCategoryDangerousContent, Threshold: genai.HarmBlockNone},
	}

	// 从提示词管理器获取提示词
	promptManager := prompts.GetGlobalManager()
	promptTemplate, err := promptManager.GetPrompt(prompts.PromptMetadataVideo)
	if err != nil {
		return nil, fmt.Errorf("获取提示词失败: %v", err)
	}

	// 使用系统提示词作为主提示词（Gemini 不区分 system/user）
	prompt := promptTemplate.System

	resp, err := model.GenerateContent(ctx, genai.FileData{URI: videoFile.URI}, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("生成内容失败: %v", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("未生成任何内容")
	}

	// 提取文本内容
	content := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])

	return parseMetadataJSON(content)
}

// GenerateMetadataFromText 从文本生成元数据（用于字幕）
func (g *GeminiClient) GenerateMetadataFromText(ctx context.Context, subtitleText string) (*VideoMetadata, error) {
	// 直接使用模型名称，SDK会自动处理
	model := g.client.GenerativeModel(g.model)

	// 设置生成参数
	model.SetMaxOutputTokens(int32(g.maxTokens))
	model.SetTemperature(0.7)

	// 设置安全设置 - 降低过滤级别以处理更多类型的内容
	model.SafetySettings = []*genai.SafetySetting{
		{Category: genai.HarmCategoryHarassment, Threshold: genai.HarmBlockNone},
		{Category: genai.HarmCategoryHateSpeech, Threshold: genai.HarmBlockNone},
		{Category: genai.HarmCategorySexuallyExplicit, Threshold: genai.HarmBlockNone},
		{Category: genai.HarmCategoryDangerousContent, Threshold: genai.HarmBlockNone},
	}

	// 从提示词管理器获取提示词
	promptManager := prompts.GetGlobalManager()
	promptTemplate, err := promptManager.GetPrompt(prompts.PromptMetadataText)
	if err != nil {
		return nil, fmt.Errorf("获取提示词失败: %v", err)
	}

	// 渲染用户提示词
	userPrompt, err := promptManager.RenderUserPrompt(prompts.PromptMetadataText, &prompts.PromptParams{
		SubtitleText: subtitleText,
	})
	if err != nil {
		return nil, fmt.Errorf("渲染提示词失败: %v", err)
	}

	// 组合系统提示词和用户提示词
	prompt := promptTemplate.System + "\n\n" + userPrompt

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("生成内容失败: %v", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("未生成任何内容")
	}

	// 提取文本内容
	content := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])

	return parseMetadataJSON(content)
}

// GenerateMetadataFromImage 从图片+文本生成元数据（用于超大视频的关键帧分析）
func (g *GeminiClient) GenerateMetadataFromImage(ctx context.Context, imagePath string, subtitleText string) (*VideoMetadata, error) {
	model := g.client.GenerativeModel(g.model)

	// 设置生成参数
	model.SetMaxOutputTokens(int32(g.maxTokens))
	model.SetTemperature(0.7)

	// 设置安全设置
	model.SafetySettings = []*genai.SafetySetting{
		{Category: genai.HarmCategoryHarassment, Threshold: genai.HarmBlockNone},
		{Category: genai.HarmCategoryHateSpeech, Threshold: genai.HarmBlockNone},
		{Category: genai.HarmCategorySexuallyExplicit, Threshold: genai.HarmBlockNone},
		{Category: genai.HarmCategoryDangerousContent, Threshold: genai.HarmBlockNone},
	}

	// 读取图片文件
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("读取图片文件失败: %v", err)
	}

	// 构建提示词
	var prompt string
	if subtitleText != "" {
		prompt = fmt.Sprintf(`请根据这张视频缩略图和以下字幕内容，生成一个吸引人的视频标题、详细描述和3-5个相关标签。

字幕内容：
%s

要求：
1. 标题要简洁有力，严格控制在30个字以内，能够准确概括视频主题
2. 描述要详细但不要过长，控制在600-800字以内
3. 标签要准确反映视频内容，3-5个即可
4. 必须使用中文
5. 输出格式必须是JSON：
{
  "title": "视频标题",
  "description": "视频描述",
  "tags": ["标签1", "标签2", "标签3"]
}

请直接返回JSON格式的结果。`, subtitleText)
	} else {
		prompt = `请根据这张视频缩略图，推测视频内容并生成一个吸引人的视频标题、详细描述和3-5个相关标签。

要求：
1. 标题要简洁有力，严格控制在30个字以内
2. 描述要详细但不要过长，控制在600-800字以内
3. 标签要准确反映视频内容，3-5个即可
4. 必须使用中文
5. 输出格式必须是JSON：
{
  "title": "视频标题",
  "description": "视频描述",
  "tags": ["标签1", "标签2", "标签3"]
}

请直接返回JSON格式的结果。`
	}

	// 调用 API
	resp, err := model.GenerateContent(ctx,
		genai.ImageData("image/jpeg", imageData),
		genai.Text(prompt),
	)
	if err != nil {
		return nil, fmt.Errorf("生成内容失败: %v", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("未生成任何内容")
	}

	// 提取文本内容
	content := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])

	return parseMetadataJSON(content)
}

// parseMetadataJSON 解析 JSON 格式的元数据
func parseMetadataJSON(content string) (*VideoMetadata, error) {
	var metadata VideoMetadata

	// 清理可能的 markdown 代码块标记
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
	}
	content = strings.TrimSpace(content)

	// 使用 json.Unmarshal 解析
	if err := json.Unmarshal([]byte(content), &metadata); err != nil {
		return nil, fmt.Errorf("解析元数据JSON失败: %v, 内容: %s", err, content)
	}

	// 验证必填字段
	if metadata.Title == "" {
		return nil, fmt.Errorf("生成的标题为空")
	}

	return &metadata, nil
}
