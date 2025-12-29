# Google Gemini 集成指南

## 📋 概述

本指南详细说明如何在项目中添加 Google Gemini AI 支持，使其可以与现有的 DeepSeek 一起使用。

## 🎯 目标架构

```
翻译服务架构
├── DeepSeek (已有)
├── Gemini (新增)
└── 统一接口层
    ├── 自动选择提供商
    ├── 失败自动切换
    └── 配置化管理
```

## 📚 需要修改的文件

### 1. 配置文件

#### `internal/core/types/app_config.go`
添加 Gemini 配置结构

#### `config.toml`
添加 Gemini 配置项

### 2. 客户端实现

#### `internal/chain_task/handlers/gemini_client.go` (新建)
实现 Gemini API 客户端

### 3. 翻译逻辑

#### `internal/chain_task/handlers/translate_subtitle.go`
修改翻译逻辑，支持多个 AI 提供商

#### `internal/chain_task/handlers/generate_metadata.go`
修改元数据生成逻辑

### 4. API 接口

#### `internal/handler/config_handler.go`
添加 Gemini 配置的 API 接口

## 🔧 详细实现步骤

### 步骤 1：添加配置结构

在 `internal/core/types/app_config.go` 中添加：

```go
// GeminiConfig Google Gemini AI 配置
type GeminiConfig struct {
	Enabled   bool   `toml:"enabled"`    // 是否启用
	ApiKey    string `toml:"api_key"`    // Gemini API密钥
	Model     string `toml:"model"`      // 使用的模型，默认 gemini-pro
	Endpoint  string `toml:"endpoint"`   // API端点
	Timeout   int    `toml:"timeout"`    // 超时时间（秒）
	MaxTokens int    `toml:"max_tokens"` // 最大token数
}

// AIProviderConfig AI 提供商总配置
type AIProviderConfig struct {
	DefaultProvider   string   `toml:"default_provider"`   // 默认提供商: "deepseek" 或 "gemini"
	FallbackProviders []string `toml:"fallback_providers"` // 备选提供商列表
	MaxRetries        int      `toml:"max_retries"`        // 最大重试次数
}
```

在 `AppConfig` 结构中添加：

```go
type AppConfig struct {
	// ... 现有字段 ...
	GeminiConfig     *GeminiConfig     `toml:"GeminiConfig"`     // Gemini配置
	AIProviderConfig *AIProviderConfig `toml:"AIProviderConfig"` // AI提供商配置
}
```

在 `NewDefaultConfig()` 中添加默认值：

```go
// Gemini 配置（默认值）
GeminiConfig: &GeminiConfig{
	Enabled:   false,
	ApiKey:    "",
	Model:     "gemini-pro",
	Endpoint:  "https://generativelanguage.googleapis.com/v1beta",
	Timeout:   60,
	MaxTokens: 4000,
},

// AI 提供商配置
AIProviderConfig: &AIProviderConfig{
	DefaultProvider:   "deepseek",
	FallbackProviders: []string{"gemini"},
	MaxRetries:        3,
},
```

### 步骤 2：创建 Gemini 客户端

创建新文件 `internal/chain_task/handlers/gemini_client.go`：

```go
package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GeminiClient Google Gemini API客户端
type GeminiClient struct {
	APIKey     string
	BaseURL    string
	Model      string
	Client     *http.Client
	MaxRetries int
	RetryDelay time.Duration
}

// GeminiRequest API请求结构
type GeminiRequest struct {
	Contents         []GeminiContent         `json:"contents"`
	GenerationConfig *GeminiGenerationConfig `json:"generationConfig,omitempty"`
}

// GeminiContent 内容结构
type GeminiContent struct {
	Parts []GeminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

// GeminiPart 内容部分
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiGenerationConfig 生成配置
type GeminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	TopP            float64 `json:"topP,omitempty"`
	TopK            int     `json:"topK,omitempty"`
}

// GeminiResponse API响应结构
type GeminiResponse struct {
	Candidates []GeminiCandidate `json:"candidates"`
	UsageMetadata GeminiUsage    `json:"usageMetadata,omitempty"`
}

// GeminiCandidate 候选结果
type GeminiCandidate struct {
	Content       GeminiContent `json:"content"`
	FinishReason  string        `json:"finishReason"`
	SafetyRatings []interface{} `json:"safetyRatings,omitempty"`
}

// GeminiUsage 使用量统计
type GeminiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// NewGeminiClient 创建Gemini客户端
func NewGeminiClient(apiKey, model string) *GeminiClient {
	if model == "" {
		model = "gemini-pro"
	}
	return &GeminiClient{
		APIKey:     apiKey,
		BaseURL:    "https://generativelanguage.googleapis.com/v1beta",
		Model:      model,
		MaxRetries: 3,
		RetryDelay: 2 * time.Second,
		Client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ChatCompletion 执行对话补全（带重试机制）
func (c *GeminiClient) ChatCompletion(systemPrompt, userPrompt string) (string, error) {
	var lastErr error
	
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(c.RetryDelay * time.Duration(attempt))
		}
		
		result, err := c.doRequest(systemPrompt, userPrompt)
		if err == nil {
			return result, nil
		}
		
		lastErr = err
		
		// 如果是API限制错误，延长等待时间
		if strings.Contains(err.Error(), "rate limit") || strings.Contains(err.Error(), "429") {
			time.Sleep(time.Duration(attempt+1) * 5 * time.Second)
		}
	}
	
	return "", fmt.Errorf("重试 %d 次后仍然失败: %v", c.MaxRetries, lastErr)
}

// doRequest 执行单次API请求
func (c *GeminiClient) doRequest(systemPrompt, userPrompt string) (string, error) {
	// Gemini 将 system prompt 和 user prompt 合并
	combinedPrompt := systemPrompt + "\n\n" + userPrompt
	
	request := GeminiRequest{
		Contents: []GeminiContent{
			{
				Parts: []GeminiPart{
					{Text: combinedPrompt},
				},
			},
		},
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:     0.3,
			MaxOutputTokens: 4000,
			TopP:            0.8,
			TopK:            40,
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %v", err)
	}

	// 构建完整的 URL
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.BaseURL, c.Model, c.APIKey)
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API返回错误 (状态码: %d): %s", resp.StatusCode, string(body))
	}

	var response GeminiResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	if len(response.Candidates) == 0 {
		return "", fmt.Errorf("API响应中没有结果")
	}

	if len(response.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("API响应内容为空")
	}

	return response.Candidates[0].Content.Parts[0].Text, nil
}
```

### 步骤 3：创建统一的 AI 客户端接口

创建 `internal/chain_task/handlers/ai_client.go`：

```go
package handlers

// AIClient 统一的 AI 客户端接口
type AIClient interface {
	ChatCompletion(systemPrompt, userPrompt string) (string, error)
}

// AIClientFactory AI 客户端工厂
type AIClientFactory struct {
	deepseekConfig *types.DeepSeekTransConfig
	geminiConfig   *types.GeminiConfig
	providerConfig *types.AIProviderConfig
}

// NewAIClientFactory 创建 AI 客户端工厂
func NewAIClientFactory(deepseekConfig *types.DeepSeekTransConfig, 
                        geminiConfig *types.GeminiConfig,
                        providerConfig *types.AIProviderConfig) *AIClientFactory {
	return &AIClientFactory{
		deepseekConfig: deepseekConfig,
		geminiConfig:   geminiConfig,
		providerConfig: providerConfig,
	}
}

// GetClient 获取 AI 客户端（根据配置自动选择）
func (f *AIClientFactory) GetClient(provider string) (AIClient, error) {
	switch provider {
	case "deepseek":
		if f.deepseekConfig != nil && f.deepseekConfig.Enabled && f.deepseekConfig.ApiKey != "" {
			return NewDeepSeekClient(f.deepseekConfig.ApiKey), nil
		}
		return nil, fmt.Errorf("DeepSeek 未配置或未启用")
	
	case "gemini":
		if f.geminiConfig != nil && f.geminiConfig.Enabled && f.geminiConfig.ApiKey != "" {
			return NewGeminiClient(f.geminiConfig.ApiKey, f.geminiConfig.Model), nil
		}
		return nil, fmt.Errorf("Gemini 未配置或未启用")
	
	default:
		return nil, fmt.Errorf("未知的 AI 提供商: %s", provider)
	}
}

// GetDefaultClient 获取默认的 AI 客户端
func (f *AIClientFactory) GetDefaultClient() (AIClient, error) {
	if f.providerConfig != nil && f.providerConfig.DefaultProvider != "" {
		return f.GetClient(f.providerConfig.DefaultProvider)
	}
	
	// 默认优先使用 DeepSeek
	if client, err := f.GetClient("deepseek"); err == nil {
		return client, nil
	}
	
	// 回退到 Gemini
	if client, err := f.GetClient("gemini"); err == nil {
		return client, nil
	}
	
	return nil, fmt.Errorf("没有可用的 AI 提供商")
}

// ChatCompletionWithFallback 带回退机制的对话补全
func (f *AIClientFactory) ChatCompletionWithFallback(systemPrompt, userPrompt string) (string, error) {
	// 尝试默认提供商
	client, err := f.GetDefaultClient()
	if err == nil {
		result, err := client.ChatCompletion(systemPrompt, userPrompt)
		if err == nil {
			return result, nil
		}
	}
	
	// 尝试备选提供商
	if f.providerConfig != nil {
		for _, provider := range f.providerConfig.FallbackProviders {
			client, err := f.GetClient(provider)
			if err != nil {
				continue
			}
			
			result, err := client.ChatCompletion(systemPrompt, userPrompt)
			if err == nil {
				return result, nil
			}
		}
	}
	
	return "", fmt.Errorf("所有 AI 提供商都失败了")
}
```

### 步骤 4：修改翻译和元数据生成逻辑

在 `translate_subtitle.go` 和 `generate_metadata.go` 中：

```go
// 替换原来的 DeepSeekClient 创建
// 旧代码：
// client := NewDeepSeekClient(t.App.Config.DeepSeekTransConfig.ApiKey)

// 新代码：
factory := NewAIClientFactory(
	t.App.Config.DeepSeekTransConfig,
	t.App.Config.GeminiConfig,
	t.App.Config.AIProviderConfig,
)

result, err := factory.ChatCompletionWithFallback(systemPrompt, userPrompt)
```

### 步骤 5：添加配置示例

在 `config.toml.example` 中添加：

```toml
[GeminiConfig]
  enabled = false
  api_key = "your-gemini-api-key"
  model = "gemini-pro"
  endpoint = "https://generativelanguage.googleapis.com/v1beta"
  timeout = 60
  max_tokens = 4000

[AIProviderConfig]
  default_provider = "deepseek"        # 默认使用 deepseek
  fallback_providers = ["gemini"]      # 失败时回退到 gemini
  max_retries = 3
```

## 📖 Google Gemini 官方文档

### 主要参考文档

1. **Gemini API 快速开始**
   - https://ai.google.dev/tutorials/get_started_web
   - 获取 API Key 和基础使用

2. **Gemini API 参考文档**
   - https://ai.google.dev/api/rest/v1beta/models/generateContent
   - API 接口详细说明

3. **Gemini 模型列表**
   - https://ai.google.dev/models/gemini
   - 可用模型：gemini-pro, gemini-pro-vision 等

4. **认证和 API Key**
   - https://ai.google.dev/tutorials/setup
   - 如何获取和使用 API Key

5. **速率限制和配额**
   - https://ai.google.dev/pricing
   - 免费额度和付费计划

6. **最佳实践**
   - https://ai.google.dev/docs/best_practices
   - 提示词优化和错误处理

### 获取 API Key

1. 访问 https://makersuite.google.com/app/apikey
2. 使用 Google 账号登录
3. 点击 "Create API Key"
4. 复制生成的 API Key

### API 端点

```
POST https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent?key={apiKey}
```

### 请求示例

```json
{
  "contents": [{
    "parts": [{
      "text": "Translate this to Chinese: Hello World"
    }]
  }],
  "generationConfig": {
    "temperature": 0.3,
    "maxOutputTokens": 4000
  }
}
```

## 🧪 测试

### 1. 测试 Gemini 连接

```go
client := NewGeminiClient("your-api-key", "gemini-pro")
result, err := client.ChatCompletion("You are a translator", "Translate to Chinese: Hello")
```

### 2. 测试自动切换

```toml
[AIProviderConfig]
  default_provider = "deepseek"
  fallback_providers = ["gemini"]
```

如果 DeepSeek 失败，会自动切换到 Gemini。

## 💡 优势

1. **多提供商支持**：不依赖单一 AI 服务
2. **自动回退**：主服务失败时自动切换
3. **灵活配置**：可以根据需要选择不同的 AI
4. **成本优化**：可以根据价格选择不同服务

## ⚠️ 注意事项

1. **API Key 安全**：不要将 API Key 提交到 Git
2. **速率限制**：注意各服务的 API 调用限制
3. **成本控制**：监控 API 使用量
4. **响应格式**：不同 AI 的响应格式可能略有差异

## 📊 对比

| 特性 | DeepSeek | Gemini |
|------|----------|--------|
| 中文支持 | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| 速度 | 快 | 中等 |
| 价格 | 较低 | 免费额度 |
| 稳定性 | 高 | 高 |
| 模型选择 | deepseek-chat | gemini-pro, gemini-pro-vision |

## 🚀 下一步

完成以上步骤后，您的项目将支持：
- ✅ DeepSeek AI
- ✅ Google Gemini AI
- ✅ 自动切换和回退
- ✅ 灵活的配置管理
