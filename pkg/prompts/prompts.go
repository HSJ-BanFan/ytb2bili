package prompts

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"text/template"

	"github.com/goccy/go-yaml"
)

// PromptType 提示词类型
type PromptType string

const (
	// 元数据生成相关
	PromptMetadataVideo    PromptType = "metadata_video"    // Gemini 视频分析
	PromptMetadataImage    PromptType = "metadata_image"    // Gemini 图片分析
	PromptMetadataText     PromptType = "metadata_text"     // 字幕文本分析
	PromptMetadataFallback PromptType = "metadata_fallback" // 备选 AI 元数据生成

	// 翻译相关
	PromptTranslateSubtitle PromptType = "translate_subtitle" // 字幕翻译
	PromptTranslateContext  PromptType = "translate_context"  // 带上下文字幕翻译
	PromptTranslateGeneral  PromptType = "translate_general"  // 通用翻译
	PromptTranslateBatch    PromptType = "translate_batch"    // 批量翻译

	// 字幕修复
	PromptFixSubtitle PromptType = "fix_subtitle" // 字幕修复

	// 语言检测
	PromptDetectLanguage PromptType = "detect_language" // 语言检测
)

// PromptTemplate 提示词模板
type PromptTemplate struct {
	Name        PromptType
	System      string  // 系统提示词
	User        string  // 用户提示词模板
	Temperature float64 // 推荐温度
	MaxTokens   int     // 推荐最大 token
}

// PromptParams 提示词参数
type PromptParams struct {
	// 通用参数
	Count      int      // 数量（如字幕句数）
	Text       string   // 文本内容
	Texts      []string // 多个文本
	SourceLang string   // 源语言
	TargetLang string   // 目标语言
	TextType   string   // 文本类型
	Domain     string   // 领域

	// 上下文参数
	PrevContext []string // 前置上下文
	NextContext []string // 后置上下文
	TargetStart int      // 目标开始位置
	TargetEnd   int      // 目标结束位置

	// 元数据参数
	SubtitleText string // 字幕文本（用于元数据生成）
}

// Manager 提示词管理器
type Manager struct {
	templates    map[PromptType]*PromptTemplate
	mu           sync.RWMutex
	configPath   string // YAML 配置文件路径
	lastModified int64  // 配置文件最后修改时间
}

// NewManager 创建提示词管理器
func NewManager() (*Manager, error) {
	m := &Manager{
		templates: make(map[PromptType]*PromptTemplate),
	}

	// 初始化所有内置提示词
	m.initBuiltinPrompts()

	return m, nil
}

// initBuiltinPrompts 初始化内置提示词
func (m *Manager) initBuiltinPrompts() {
	// 视频元数据生成
	m.templates[PromptMetadataVideo] = &PromptTemplate{
		Name: PromptMetadataVideo,
		System: `你是一位资深的 Bilibili 视频内容创作助手，拥有丰富的视频运营经验和对 B站用户偏好的深刻理解。

请仔细分析这个视频内容，然后生成以下元数据：

## 标题要求
- 长度：15-30个中文字符（最佳 20-25 字）
- 风格：吸引眼球但不标题党，真实反映视频内容
- 技巧：可适当使用数字、疑问句式、热点词增加吸引力

## 描述要求
- 长度：80-150字（简洁精炼）
- 结构：开头概括视频主题 + 1-2个核心亮点
- 避免：过于冗长、堆砌关键词

## 标签要求
- 数量：3-5个
- 类型：内容标签为主，可加1-2个热门相关标签
- 规范：每个标签2-8个字，避免敏感词

## 输出格式
必须返回有效的 JSON 格式，不要添加任何其他说明文字：
{
  "title": "视频标题",
  "description": "视频描述",
  "tags": ["标签1", "标签2", "标签3"]
}`,
		Temperature: 0.7,
		MaxTokens:   1000,
	}

	// 图片元数据生成
	m.templates[PromptMetadataImage] = &PromptTemplate{
		Name: PromptMetadataImage,
		System: `这是一个视频的缩略图/关键帧。请根据图片内容推测视频主题，生成以下元数据：

## 分析要求
1. 仔细观察图片中的人物、场景、文字、物品等元素
2. 根据视觉线索推测视频可能的主题和内容
3. 生成合理且吸引人的元数据

## 标题要求
- 长度：15-30个中文字符
- 基于图片内容合理推测，不要过度夸张

## 描述要求
- 长度：80-150字
- 根据图片内容描述可能的视频主题

## 标签要求
- 数量：3-5个

## 输出格式
必须返回有效的 JSON 格式：
{
  "title": "视频标题",
  "description": "视频描述",
  "tags": ["标签1", "标签2", "标签3"]
}`,
		Temperature: 0.7,
		MaxTokens:   1000,
	}

	// 文本元数据生成
	m.templates[PromptMetadataText] = &PromptTemplate{
		Name: PromptMetadataText,
		System: `你是一位专业的视频内容分析师，擅长从字幕内容中提炼视频主题并生成吸引人的元数据。

请根据提供的视频字幕内容，生成符合 Bilibili 平台规范的视频元数据。

## 标题要求
- 长度：15-30个中文字符
- 准确概括视频主题，简洁有力

## 描述要求
- 长度：80-150字
- 包含视频核心内容和亮点

## 标签要求
- 数量：3-5个

## 输出格式
必须返回有效的 JSON 格式：
{
  "title": "视频标题",
  "description": "视频描述",
  "tags": ["标签1", "标签2", "标签3"]
}`,
		User:        "请根据以下字幕内容生成视频元数据：\n\n{{.SubtitleText}}",
		Temperature: 0.7,
		MaxTokens:   1000,
	}

	// 备选元数据生成
	m.templates[PromptMetadataFallback] = &PromptTemplate{
		Name: PromptMetadataFallback,
		System: `你是一个专业的视频内容分析师，擅长为 Bilibili 视频生成吸引人的标题和描述。

请根据提供的字幕内容，生成以下元数据：
1. 标题：简洁有力，能吸引观众点击，不超过80个字符
2. 描述：详细介绍视频内容，包含关键信息，适合SEO
3. 标签：5-8个相关标签

## 输出格式
必须返回有效的 JSON 格式：
{
  "title": "视频标题",
  "description": "视频描述",
  "tags": ["标签1", "标签2", "标签3", "标签4", "标签5"]
}

注意：只返回 JSON，不要添加其他内容`,
		User:        "请根据以下字幕内容生成视频元数据：\n\n{{.SubtitleText}}",
		Temperature: 0.7,
		MaxTokens:   2000,
	}

	// 字幕翻译
	m.templates[PromptTranslateSubtitle] = &PromptTemplate{
		Name: PromptTranslateSubtitle,
		System: `你是一个专业的视频字幕翻译专家，专注于将英文视频字幕翻译成自然流畅的中文。

## 翻译要求
1. **自然流畅**：使用口语化表达，符合中文字幕的阅读习惯
2. **准确传神**：忠实原文含义，保持语气和情感色彩
3. **简洁明了**：字幕需要快速阅读，避免冗长表达
4. **格式严格**：必须输出 {{.Count}} 句翻译，不多不少

## 输出规范
- 每句翻译用 "###SENTENCE_BREAK###" 分隔
- 只返回翻译的中文文本
- 不要添加序号、解释或其他内容`,
		User:        "请将以下 {{.Count}} 句英文字幕翻译成中文（用 \"###SENTENCE_BREAK###\" 分隔）：\n\n{{.Text}}",
		Temperature: 0.3,
		MaxTokens:   4000,
	}

	// 带上下文的字幕翻译
	m.templates[PromptTranslateContext] = &PromptTemplate{
		Name: PromptTranslateContext,
		System: `你是一个专业的视频字幕翻译专家。需要翻译连续字幕的目标部分。

## 翻译要求
1. **上下文连贯**：理解整体语境，确保翻译前后呼应
2. **自然流畅**：使用口语化表达，符合中文字幕习惯
3. **准确传神**：忠实原文含义，保持语气和情感
4. **数量严格**：必须输出 {{.Count}} 句翻译，不多不少

## 输出规范
- 只翻译目标部分
- 每句翻译用 "###SENTENCE_BREAK###" 分隔
- 只返回中文翻译，不要添加序号或其他内容`,
		User:        "请翻译以下内容的目标部分：\n\n{{.Text}}",
		Temperature: 0.3,
		MaxTokens:   4000,
	}

	// 字幕修复
	m.templates[PromptFixSubtitle] = &PromptTemplate{
		Name: PromptFixSubtitle,
		System: `你是专业的视频字幕翻译专家。现在需要重新翻译 {{.Count}} 句有问题的英文字幕。

## 背景说明
这些字幕在之前的自动翻译中可能出现了缺失、不完整或错误，现在需要你提供完整准确的重新翻译。

## 翻译要求
1. **完整输出**：必须为每句英文提供完整的中文翻译，不能有遗漏
2. **自然流畅**：使用口语化表达，符合中文字幕习惯
3. **数量严格**：必须输出 {{.Count}} 句翻译，不多不少

## 输出规范
- 每句翻译用 "###SENTENCE_BREAK###" 分隔
- 只返回中文翻译，不要添加序号或其他内容`,
		User:        "请重新翻译以下 {{.Count}} 句英文字幕：\n\n{{.Text}}",
		Temperature: 0.3,
		MaxTokens:   4000,
	}

	// 语言检测
	m.templates[PromptDetectLanguage] = &PromptTemplate{
		Name: PromptDetectLanguage,
		System: `你是一个语言检测专家。请检测给定文本的语言，并返回 ISO 639-1 语言代码。

## 要求
- 只返回语言代码，如 'en'、'zh'、'ja'、'ko' 等
- 不要添加任何解释或其他内容
- 如果无法确定，返回 'auto'`,
		User:        "{{.Text}}",
		Temperature: 0.1,
		MaxTokens:   50,
	}
}

// GetPrompt 获取指定类型的提示词
func (m *Manager) GetPrompt(promptType PromptType) (*PromptTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, exists := m.templates[promptType]
	if !exists {
		return nil, fmt.Errorf("未知的提示词类型: %s", promptType)
	}
	// 返回副本，避免并发修改
	copy := *t
	return &copy, nil
}

// RenderSystemPrompt 渲染系统提示词
func (m *Manager) RenderSystemPrompt(promptType PromptType, params *PromptParams) (string, error) {
	t, err := m.GetPrompt(promptType)
	if err != nil {
		return "", err
	}
	return m.renderTemplate(t.System, params)
}

// RenderUserPrompt 渲染用户提示词
func (m *Manager) RenderUserPrompt(promptType PromptType, params *PromptParams) (string, error) {
	t, err := m.GetPrompt(promptType)
	if err != nil {
		return "", err
	}
	if t.User == "" {
		if params != nil {
			return params.Text, nil
		}
		return "", nil
	}
	return m.renderTemplate(t.User, params)
}

// renderTemplate 渲染模板
func (m *Manager) renderTemplate(templateStr string, params *PromptParams) (string, error) {
	if params == nil {
		return templateStr, nil
	}

	tmpl, err := template.New("prompt").Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("解析模板失败: %v", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("渲染模板失败: %v", err)
	}

	return buf.String(), nil
}

// GetTemperature 获取推荐温度
func (m *Manager) GetTemperature(promptType PromptType) float64 {
	t, err := m.GetPrompt(promptType)
	if err != nil {
		return 0.7 // 默认温度
	}
	return t.Temperature
}

// GetMaxTokens 获取推荐最大 token
func (m *Manager) GetMaxTokens(promptType PromptType) int {
	t, err := m.GetPrompt(promptType)
	if err != nil {
		return 4000 // 默认值
	}
	return t.MaxTokens
}

// Separator 分隔符常量
const (
	SentenceBreak = "###SENTENCE_BREAK###"
)

// JoinWithSeparator 使用分隔符连接文本
func JoinWithSeparator(texts []string) string {
	return strings.Join(texts, "\n"+SentenceBreak+"\n")
}

// SplitBySeparator 按分隔符分割文本
func SplitBySeparator(text string) []string {
	parts := strings.Split(text, SentenceBreak)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// =====================================
// A/B 测试支持
// =====================================

// PromptVersion 提示词版本（用于 A/B 测试）
type PromptVersion struct {
	ID          string          `yaml:"id"`          // 版本 ID
	Version     string          `yaml:"version"`     // 版本号
	Description string          `yaml:"description"` // 版本描述
	IsActive    bool            `yaml:"is_active"`   // 是否激活
	Weight      float64         `yaml:"weight"`      // 选择权重 (0-1)
	Template    *PromptTemplate // 模板内容
}

// ABTestConfig A/B 测试配置
type ABTestConfig struct {
	Enabled  bool                            `yaml:"enabled"` // 是否启用 A/B 测试
	Versions map[PromptType][]*PromptVersion // 各类型的版本列表
}

// SelectVersion 根据权重选择版本（简单实现）
func (m *Manager) SelectVersion(promptType PromptType) *PromptTemplate {
	// 当前简单实现：直接返回主模板
	// 未来可扩展为基于权重的随机选择
	t, _ := m.GetPrompt(promptType)
	return t
}

// =====================================
// 配置文件加载
// =====================================

// YAMLConfig YAML 配置文件结构
type YAMLConfig struct {
	MetadataVideo     *YAMLPrompt `yaml:"metadata_video"`
	MetadataImage     *YAMLPrompt `yaml:"metadata_image"`
	MetadataText      *YAMLPrompt `yaml:"metadata_text"`
	MetadataFallback  *YAMLPrompt `yaml:"metadata_fallback"`
	TranslateSubtitle *YAMLPrompt `yaml:"translate_subtitle"`
	TranslateContext  *YAMLPrompt `yaml:"translate_context"`
	FixSubtitle       *YAMLPrompt `yaml:"fix_subtitle"`
	DetectLanguage    *YAMLPrompt `yaml:"detect_language"`
}

// YAMLPrompt YAML 中的提示词配置
type YAMLPrompt struct {
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
	System      string  `yaml:"system"`
	User        string  `yaml:"user"`
}

// LoadFromYAML 从 YAML 文件加载配置（合并到现有配置）
func (m *Manager) LoadFromYAML(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config YAMLConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 记录配置文件路径和修改时间
	m.configPath = path
	if info, err := os.Stat(path); err == nil {
		m.lastModified = info.ModTime().Unix()
	}

	// 合并配置
	m.mergeConfig(&config)
	return nil
}

// ReloadIfChanged 检查配置文件是否变化，如果变化则重新加载（热更新）
func (m *Manager) ReloadIfChanged() (bool, error) {
	m.mu.RLock()
	configPath := m.configPath
	lastModified := m.lastModified
	m.mu.RUnlock()

	if configPath == "" {
		return false, nil
	}

	info, err := os.Stat(configPath)
	if err != nil {
		return false, nil // 文件不存在，不报错
	}

	if info.ModTime().Unix() <= lastModified {
		return false, nil // 文件未变化
	}

	// 文件已变化，重新加载
	if err := m.LoadFromYAML(configPath); err != nil {
		return false, err
	}

	return true, nil
}

// mergeConfig 合并 YAML 配置到内置模板
func (m *Manager) mergeConfig(config *YAMLConfig) {
	if config.MetadataVideo != nil {
		m.mergePrompt(PromptMetadataVideo, config.MetadataVideo)
	}
	if config.MetadataImage != nil {
		m.mergePrompt(PromptMetadataImage, config.MetadataImage)
	}
	if config.MetadataText != nil {
		m.mergePrompt(PromptMetadataText, config.MetadataText)
	}
	if config.MetadataFallback != nil {
		m.mergePrompt(PromptMetadataFallback, config.MetadataFallback)
	}
	if config.TranslateSubtitle != nil {
		m.mergePrompt(PromptTranslateSubtitle, config.TranslateSubtitle)
	}
	if config.TranslateContext != nil {
		m.mergePrompt(PromptTranslateContext, config.TranslateContext)
	}
	if config.FixSubtitle != nil {
		m.mergePrompt(PromptFixSubtitle, config.FixSubtitle)
	}
	if config.DetectLanguage != nil {
		m.mergePrompt(PromptDetectLanguage, config.DetectLanguage)
	}
}

// mergePrompt 合并单个提示词配置
func (m *Manager) mergePrompt(promptType PromptType, yamlPrompt *YAMLPrompt) {
	t, exists := m.templates[promptType]
	if !exists {
		return
	}

	if yamlPrompt.System != "" {
		t.System = yamlPrompt.System
	}
	if yamlPrompt.User != "" {
		t.User = yamlPrompt.User
	}
	if yamlPrompt.Temperature > 0 {
		t.Temperature = yamlPrompt.Temperature
	}
	if yamlPrompt.MaxTokens > 0 {
		t.MaxTokens = yamlPrompt.MaxTokens
	}
}

// UpdatePrompt 运行时更新单个提示词（支持热更新）
func (m *Manager) UpdatePrompt(promptType PromptType, system, user string, temperature float64, maxTokens int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, exists := m.templates[promptType]
	if !exists {
		return fmt.Errorf("未知的提示词类型: %s", promptType)
	}

	if system != "" {
		t.System = system
	}
	if user != "" {
		t.User = user
	}
	if temperature > 0 {
		t.Temperature = temperature
	}
	if maxTokens > 0 {
		t.MaxTokens = maxTokens
	}

	return nil
}

// GetAllPromptTypes 获取所有提示词类型
func (m *Manager) GetAllPromptTypes() []PromptType {
	return []PromptType{
		PromptMetadataVideo,
		PromptMetadataImage,
		PromptMetadataText,
		PromptMetadataFallback,
		PromptTranslateSubtitle,
		PromptTranslateContext,
		PromptFixSubtitle,
		PromptDetectLanguage,
	}
}

// ExportToMap 导出所有提示词为 map（用于 API 接口）
func (m *Manager) ExportToMap() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]interface{})
	for promptType, t := range m.templates {
		result[string(promptType)] = map[string]interface{}{
			"system":      t.System,
			"user":        t.User,
			"temperature": t.Temperature,
			"max_tokens":  t.MaxTokens,
		}
	}
	return result
}

// GetConfigPath 获取当前配置文件路径
func (m *Manager) GetConfigPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configPath
}

// =====================================
// 全局单例
// =====================================

var (
	globalManager     *Manager
	globalManagerOnce sync.Once
)

// GetGlobalManager 获取全局提示词管理器（单例模式）
func GetGlobalManager() *Manager {
	globalManagerOnce.Do(func() {
		var err error
		globalManager, err = NewManager()
		if err != nil {
			panic(fmt.Sprintf("初始化提示词管理器失败: %v", err))
		}
	})
	return globalManager
}

// InitGlobalManagerWithConfig 使用配置文件初始化全局管理器
func InitGlobalManagerWithConfig(configPath string) error {
	manager := GetGlobalManager()
	if configPath != "" {
		if err := manager.LoadFromYAML(configPath); err != nil {
			// 配置文件加载失败不是致命错误，使用内置默认值
			return fmt.Errorf("加载提示词配置失败（使用默认值）: %v", err)
		}
	}
	return nil
}
