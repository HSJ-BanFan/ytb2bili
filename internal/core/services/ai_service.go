package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/difyz9/ytb2bili/internal/core/types"
	"go.uber.org/zap"
)

// AIProvider AI服务提供商类型
type AIProvider string

const (
	AIProviderOpenAICompatible AIProvider = "openai_compatible" // 用户配置的OpenAI兼容API（首选）
	AIProviderDeepSeek         AIProvider = "deepseek"          // DeepSeek
	AIProviderGemini           AIProvider = "gemini"            // Gemini（原生）
)

// AIServiceStatus AI服务状态
type AIServiceStatus struct {
	Provider    AIProvider `json:"provider"`
	Name        string     `json:"name"`
	Enabled     bool       `json:"enabled"`
	Available   bool       `json:"available"`
	LastChecked time.Time  `json:"last_checked"`
	LastError   string     `json:"last_error,omitempty"`
	Model       string     `json:"model,omitempty"`
	BaseURL     string     `json:"base_url,omitempty"`
}

// AIServiceManager AI服务管理器
// 管理多个AI服务提供商，支持首选服务和自动故障转移
type AIServiceManager struct {
	config     *types.AppConfig
	logger     *zap.SugaredLogger
	mu         sync.RWMutex
	statusMap  map[AIProvider]*AIServiceStatus
	lastUpdate time.Time
}

// NewAIServiceManager 创建AI服务管理器
func NewAIServiceManager(config *types.AppConfig, log *zap.SugaredLogger) *AIServiceManager {
	manager := &AIServiceManager{
		config:    config,
		logger:    log,
		statusMap: make(map[AIProvider]*AIServiceStatus),
	}
	manager.initStatus()
	return manager
}

// initStatus 初始化服务状态
func (m *AIServiceManager) initStatus() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// OpenAI兼容API（首选）
	m.statusMap[AIProviderOpenAICompatible] = &AIServiceStatus{
		Provider:  AIProviderOpenAICompatible,
		Name:      "OpenAI兼容API（首选）",
		Enabled:   false,
		Available: false,
	}

	// DeepSeek
	m.statusMap[AIProviderDeepSeek] = &AIServiceStatus{
		Provider:  AIProviderDeepSeek,
		Name:      "DeepSeek",
		Enabled:   false,
		Available: false,
	}

	// Gemini
	m.statusMap[AIProviderGemini] = &AIServiceStatus{
		Provider:  AIProviderGemini,
		Name:      "Gemini",
		Enabled:   false,
		Available: false,
	}

	m.updateStatusFromConfig()
}

// updateStatusFromConfig 从配置更新状态
func (m *AIServiceManager) updateStatusFromConfig() {
	// OpenAI兼容API
	if cfg := m.config.OpenAICompatibleConfig; cfg != nil && cfg.Enabled {
		status := m.statusMap[AIProviderOpenAICompatible]
		status.Enabled = true
		status.Model = cfg.Model
		status.BaseURL = cfg.BaseURL
		// 根据Provider显示更友好的名称
		switch cfg.Provider {
		case "openai":
			status.Name = "OpenAI"
		case "deepseek":
			status.Name = "DeepSeek (兼容模式)"
		case "qwen":
			status.Name = "通义千问"
		case "zhipu":
			status.Name = "智谱AI"
		case "gemini":
			status.Name = "Gemini (代理)"
		case "custom":
			status.Name = "自定义API"
		default:
			status.Name = "OpenAI兼容API"
		}
	}

	// DeepSeek
	if cfg := m.config.DeepSeekTransConfig; cfg != nil && cfg.Enabled {
		status := m.statusMap[AIProviderDeepSeek]
		status.Enabled = true
		status.Model = cfg.Model
		status.BaseURL = cfg.Endpoint
	}

	// Gemini
	if cfg := m.config.GeminiConfig; cfg != nil && cfg.Enabled {
		status := m.statusMap[AIProviderGemini]
		status.Enabled = true
		status.Model = cfg.Model
	}
}

// RefreshConfig 刷新配置（配置更新后调用）
func (m *AIServiceManager) RefreshConfig(config *types.AppConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	m.updateStatusFromConfig()
}

// GetPreferredProvider 获取首选的AI服务提供商
// 优先使用用户选择的首选服务，如果未设置则按默认优先级
func (m *AIServiceManager) GetPreferredProvider() (AIProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 1. 首先检查用户选择的首选服务
	if m.config.PrimaryAIService != "" {
		provider := AIProvider(m.config.PrimaryAIService)
		if status, ok := m.statusMap[provider]; ok && status.Enabled {
			return provider, nil
		}
		// 用户选择的服务未启用，继续查找其他可用服务
	}

	// 2. 按默认优先级查找：OpenAI兼容API > DeepSeek > Gemini
	if status := m.statusMap[AIProviderOpenAICompatible]; status.Enabled {
		return AIProviderOpenAICompatible, nil
	}

	if status := m.statusMap[AIProviderDeepSeek]; status.Enabled {
		return AIProviderDeepSeek, nil
	}

	if status := m.statusMap[AIProviderGemini]; status.Enabled {
		return AIProviderGemini, nil
	}

	return "", fmt.Errorf("没有可用的AI服务，请先配置AI服务")
}

// GetAvailableProvider 获取可用的AI服务提供商（带故障转移）
// 如果首选服务不可用，自动切换到备选服务
func (m *AIServiceManager) GetAvailableProvider() (AIProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providers := []AIProvider{
		AIProviderOpenAICompatible,
		AIProviderDeepSeek,
		AIProviderGemini,
	}

	for _, provider := range providers {
		status := m.statusMap[provider]
		if status.Enabled && status.Available {
			return provider, nil
		}
	}

	// 如果没有已验证可用的，返回第一个启用的
	for _, provider := range providers {
		status := m.statusMap[provider]
		if status.Enabled {
			return provider, nil
		}
	}

	return "", fmt.Errorf("没有可用的AI服务")
}

// GetAllStatus 获取所有AI服务状态
func (m *AIServiceManager) GetAllStatus() []*AIServiceStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 按优先级顺序返回
	result := make([]*AIServiceStatus, 0, 3)
	for _, provider := range []AIProvider{AIProviderOpenAICompatible, AIProviderDeepSeek, AIProviderGemini} {
		if status, ok := m.statusMap[provider]; ok {
			// 复制一份避免并发问题
			statusCopy := *status
			result = append(result, &statusCopy)
		}
	}
	return result
}

// GetStatus 获取指定提供商的状态
func (m *AIServiceManager) GetStatus(provider AIProvider) *AIServiceStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if status, ok := m.statusMap[provider]; ok {
		statusCopy := *status
		return &statusCopy
	}
	return nil
}

// SetAvailable 设置服务可用状态
func (m *AIServiceManager) SetAvailable(provider AIProvider, available bool, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if status, ok := m.statusMap[provider]; ok {
		status.Available = available
		status.LastChecked = time.Now()
		if !available {
			status.LastError = errMsg
		} else {
			status.LastError = ""
		}
	}
}

// GetOpenAICompatibleConfig 获取OpenAI兼容API配置
func (m *AIServiceManager) GetOpenAICompatibleConfig() *types.OpenAICompatibleConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.OpenAICompatibleConfig
}

// GetDeepSeekConfig 获取DeepSeek配置
func (m *AIServiceManager) GetDeepSeekConfig() *types.DeepSeekTransConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.DeepSeekTransConfig
}

// GetGeminiConfig 获取Gemini配置
func (m *AIServiceManager) GetGeminiConfig() *types.GeminiConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.GeminiConfig
}

// IsOpenAICompatibleEnabled 检查OpenAI兼容API是否启用
func (m *AIServiceManager) IsOpenAICompatibleEnabled() bool {
	cfg := m.GetOpenAICompatibleConfig()
	return cfg != nil && cfg.Enabled && cfg.ApiKey != ""
}

// IsDeepSeekEnabled 检查DeepSeek是否启用
func (m *AIServiceManager) IsDeepSeekEnabled() bool {
	cfg := m.GetDeepSeekConfig()
	return cfg != nil && cfg.Enabled && cfg.ApiKey != ""
}

// IsGeminiEnabled 检查Gemini是否启用
func (m *AIServiceManager) IsGeminiEnabled() bool {
	cfg := m.GetGeminiConfig()
	return cfg != nil && cfg.Enabled && cfg.ApiKey != ""
}

// ChatCompletion 执行对话补全（自动选择AI服务）
// 优先使用首选服务，失败后自动切换到备选服务
func (m *AIServiceManager) ChatCompletion(systemPrompt, userPrompt string) (string, AIProvider, error) {
	providers := []AIProvider{
		AIProviderOpenAICompatible,
		AIProviderDeepSeek,
	}

	var lastErr error
	for _, provider := range providers {
		if !m.isProviderEnabled(provider) {
			continue
		}

		m.logger.Infof("🤖 尝试使用 %s 进行AI对话...", m.getProviderName(provider))

		result, err := m.chatWithProvider(provider, systemPrompt, userPrompt)
		if err == nil {
			m.SetAvailable(provider, true, "")
			m.logger.Infof("✅ %s 调用成功", m.getProviderName(provider))
			return result, provider, nil
		}

		lastErr = err
		m.SetAvailable(provider, false, err.Error())
		m.logger.Warnf("⚠️ %s 调用失败: %v，尝试下一个服务...", m.getProviderName(provider), err)
	}

	return "", "", fmt.Errorf("所有AI服务都不可用: %v", lastErr)
}

// chatWithProvider 使用指定提供商进行对话
func (m *AIServiceManager) chatWithProvider(provider AIProvider, systemPrompt, userPrompt string) (string, error) {
	switch provider {
	case AIProviderOpenAICompatible:
		return m.chatWithOpenAICompatible(systemPrompt, userPrompt)
	case AIProviderDeepSeek:
		return m.chatWithDeepSeek(systemPrompt, userPrompt)
	default:
		return "", fmt.Errorf("不支持的AI提供商: %s", provider)
	}
}

// chatWithOpenAICompatible 使用OpenAI兼容API进行对话
func (m *AIServiceManager) chatWithOpenAICompatible(systemPrompt, userPrompt string) (string, error) {
	cfg := m.GetOpenAICompatibleConfig()
	if cfg == nil || !cfg.Enabled {
		return "", fmt.Errorf("OpenAI兼容API未启用")
	}

	client := m.createOpenAICompatibleClient(cfg)
	return client.ChatCompletion(systemPrompt, userPrompt)
}

// chatWithDeepSeek 使用DeepSeek进行对话
func (m *AIServiceManager) chatWithDeepSeek(systemPrompt, userPrompt string) (string, error) {
	cfg := m.GetDeepSeekConfig()
	if cfg == nil || !cfg.Enabled {
		return "", fmt.Errorf("DeepSeek未启用")
	}

	// 使用OpenAI兼容客户端调用DeepSeek
	client := m.createOpenAICompatibleClientFromDeepSeek(cfg)
	return client.ChatCompletion(systemPrompt, userPrompt)
}

// createOpenAICompatibleClient 创建OpenAI兼容客户端
func (m *AIServiceManager) createOpenAICompatibleClient(cfg *types.OpenAICompatibleConfig) *OpenAICompatibleClient {
	return NewOpenAICompatibleClient(&OpenAIClientConfig{
		APIKey:      cfg.ApiKey,
		BaseURL:     cfg.BaseURL,
		Model:       cfg.Model,
		Timeout:     cfg.Timeout,
		MaxRetries:  3,
		Temperature: cfg.Temperature,
		MaxTokens:   cfg.MaxTokens,
	})
}

// createOpenAICompatibleClientFromDeepSeek 从DeepSeek配置创建OpenAI兼容客户端
func (m *AIServiceManager) createOpenAICompatibleClientFromDeepSeek(cfg *types.DeepSeekTransConfig) *OpenAICompatibleClient {
	baseURL := cfg.Endpoint
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}

	return NewOpenAICompatibleClient(&OpenAIClientConfig{
		APIKey:      cfg.ApiKey,
		BaseURL:     baseURL,
		Model:       cfg.Model,
		Timeout:     cfg.Timeout,
		MaxRetries:  3,
		Temperature: 0.3,
		MaxTokens:   cfg.MaxTokens,
	})
}

// isProviderEnabled 检查提供商是否启用
func (m *AIServiceManager) isProviderEnabled(provider AIProvider) bool {
	switch provider {
	case AIProviderOpenAICompatible:
		return m.IsOpenAICompatibleEnabled()
	case AIProviderDeepSeek:
		return m.IsDeepSeekEnabled()
	case AIProviderGemini:
		return m.IsGeminiEnabled()
	default:
		return false
	}
}

// getProviderName 获取提供商显示名称
func (m *AIServiceManager) getProviderName(provider AIProvider) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if status, ok := m.statusMap[provider]; ok {
		return status.Name
	}
	return string(provider)
}

// OpenAICompatibleClient 引用handlers包中的客户端
type OpenAICompatibleClient = openAICompatibleClientWrapper

// openAICompatibleClientWrapper 包装器，避免循环引用
type openAICompatibleClientWrapper struct {
	apiKey      string
	baseURL     string
	model       string
	timeout     int
	maxRetries  int
	temperature float64
	maxTokens   int
}

// OpenAIClientConfig 客户端配置
type OpenAIClientConfig struct {
	APIKey      string
	BaseURL     string
	Model       string
	Timeout     int
	MaxRetries  int
	Temperature float64
	MaxTokens   int
}

// NewOpenAICompatibleClient 创建客户端
func NewOpenAICompatibleClient(config *OpenAIClientConfig) *openAICompatibleClientWrapper {
	return &openAICompatibleClientWrapper{
		apiKey:      config.APIKey,
		baseURL:     config.BaseURL,
		model:       config.Model,
		timeout:     config.Timeout,
		maxRetries:  config.MaxRetries,
		temperature: config.Temperature,
		maxTokens:   config.MaxTokens,
	}
}

// ChatCompletion 执行对话
func (c *openAICompatibleClientWrapper) ChatCompletion(systemPrompt, userPrompt string) (string, error) {
	// 构建请求
	type Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type Request struct {
		Model       string    `json:"model"`
		Messages    []Message `json:"messages"`
		Stream      bool      `json:"stream"`
		Temperature float64   `json:"temperature,omitempty"`
		MaxTokens   int       `json:"max_tokens,omitempty"`
	}
	type Choice struct {
		Message Message `json:"message"`
	}
	type Response struct {
		Choices []Choice `json:"choices"`
		Error   *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error,omitempty"`
	}

	request := Request{
		Model: c.model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Stream:      false,
		Temperature: c.temperature,
		MaxTokens:   c.maxTokens,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %v", err)
	}

	// 构建API URL
	apiURL := strings.TrimSuffix(c.baseURL, "/")
	if !strings.Contains(apiURL, "/chat/completions") {
		if !strings.HasSuffix(apiURL, "/v1") {
			apiURL = apiURL + "/v1"
		}
		apiURL = apiURL + "/chat/completions"
	}

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: time.Duration(c.timeout) * time.Second,
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(2 * time.Second * time.Duration(attempt))
		}

		req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return "", fmt.Errorf("创建请求失败: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("发送请求失败: %v", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("读取响应失败: %v", err)
			continue
		}

		var response Response
		if err := json.Unmarshal(body, &response); err != nil {
			// 尝试解析数组格式的响应（某些代理返回数组）
			var arrayResponse []Response
			if arrErr := json.Unmarshal(body, &arrayResponse); arrErr == nil && len(arrayResponse) > 0 {
				response = arrayResponse[0]
			} else {
				// 尝试解析 Gemini 原生格式
				var geminiResp struct {
					Candidates []struct {
						Content struct {
							Parts []struct {
								Text string `json:"text"`
							} `json:"parts"`
						} `json:"content"`
					} `json:"candidates"`
				}
				if gemErr := json.Unmarshal(body, &geminiResp); gemErr == nil && len(geminiResp.Candidates) > 0 {
					if len(geminiResp.Candidates[0].Content.Parts) > 0 {
						return geminiResp.Candidates[0].Content.Parts[0].Text, nil
					}
				}
				lastErr = fmt.Errorf("解析响应失败: %v", err)
				continue
			}
		}

		if response.Error != nil {
			lastErr = fmt.Errorf("API错误: %s", response.Error.Message)
			if strings.Contains(response.Error.Message, "rate limit") {
				time.Sleep(5 * time.Second * time.Duration(attempt+1))
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("API返回错误 (状态码: %d): %s", resp.StatusCode, string(body))
			continue
		}

		if len(response.Choices) == 0 {
			lastErr = fmt.Errorf("API响应中没有结果")
			continue
		}

		return response.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("重试 %d 次后仍然失败: %v", c.maxRetries, lastErr)
}
