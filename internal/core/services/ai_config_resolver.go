package services

import (
	"encoding/json"

	"github.com/difyz9/ytb2bili/internal/core/types"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AIConfigResolver AI配置解析器
// 实现优先级逻辑：用户配置 → 系统配置 → 优雅降级
type AIConfigResolver struct {
	DB                *gorm.DB
	Logger            *zap.SugaredLogger
	UserConfigService *UserConfigService
	AppConfig         *types.AppConfig
}

// NewAIConfigResolver 创建AI配置解析器
func NewAIConfigResolver(db *gorm.DB, logger *zap.SugaredLogger, userConfigService *UserConfigService, appConfig *types.AppConfig) *AIConfigResolver {
	return &AIConfigResolver{
		DB:                db,
		Logger:            logger,
		UserConfigService: userConfigService,
		AppConfig:         appConfig,
	}
}

// ResolvedAIConfig 解析后的AI配置
type ResolvedAIConfig struct {
	Provider       string // deepseek, gemini, openai, baidu
	Enabled        bool
	APIKey         string
	APIKeys        []string // 用于多密钥轮询
	Model          string
	BaseURL        string // 用于OpenAI兼容API
	Timeout        int
	MaxTokens      int
	UsesUserConfig bool // 标识是否使用了用户配置
}

// GetDeepSeekConfig 获取DeepSeek配置（优先级：用户 → 系统 → 空）
func (r *AIConfigResolver) GetDeepSeekConfig(userID uint) *ResolvedAIConfig {
	config := &ResolvedAIConfig{
		Provider: "deepseek",
	}

	// 1. 尝试从用户配置获取
	userAIConfig, err := r.UserConfigService.GetUserAIConfig(userID)
	if err == nil && userAIConfig.DeepSeekEnabled && userAIConfig.DeepSeekAPIKey != "" {
		config.Enabled = true
		config.APIKey = userAIConfig.DeepSeekAPIKey
		config.Model = userAIConfig.DeepSeekModel
		config.UsesUserConfig = true
		r.Logger.Debugf("✅ 使用用户配置的 DeepSeek API Key (user_id=%d)", userID)
		return config
	}

	// 2. 回退到系统配置
	if r.AppConfig.DeepSeekTransConfig != nil && r.AppConfig.DeepSeekTransConfig.Enabled && r.AppConfig.DeepSeekTransConfig.ApiKey != "" {
		config.Enabled = true
		config.APIKey = r.AppConfig.DeepSeekTransConfig.ApiKey
		config.Model = r.AppConfig.DeepSeekTransConfig.Model
		config.UsesUserConfig = false
		r.Logger.Debugf("✅ 使用系统配置的 DeepSeek API Key")
		return config
	}

	// 3. 无可用配置
	r.Logger.Debugf("⚠️  DeepSeek 未配置（用户和系统均无API Key）")
	return config
}

// GetGeminiConfig 获取Gemini配置（优先级：用户 → 系统 → 空）
func (r *AIConfigResolver) GetGeminiConfig(userID uint) *ResolvedAIConfig {
	config := &ResolvedAIConfig{
		Provider: "gemini",
	}

	// 1. 尝试从用户配置获取
	userAIConfig, err := r.UserConfigService.GetUserAIConfig(userID)
	if err == nil && userAIConfig.GeminiEnabled {
		// 优先使用多密钥（如果有）
		if userAIConfig.GeminiAPIKeys != "" {
			var keys []string
			if err := json.Unmarshal([]byte(userAIConfig.GeminiAPIKeys), &keys); err == nil && len(keys) > 0 {
				config.Enabled = true
				config.APIKeys = keys
				config.Model = userAIConfig.GeminiModel
				config.Timeout = userAIConfig.GeminiTimeout
				config.MaxTokens = userAIConfig.GeminiMaxTokens
				config.UsesUserConfig = true
				r.Logger.Debugf("✅ 使用用户配置的 Gemini API Keys (user_id=%d, %d个密钥)", userID, len(keys))
				return config
			}
		}

		// 回退到单密钥
		if userAIConfig.GeminiAPIKey != "" {
			config.Enabled = true
			config.APIKey = userAIConfig.GeminiAPIKey
			config.APIKeys = []string{userAIConfig.GeminiAPIKey}
			config.Model = userAIConfig.GeminiModel
			config.Timeout = userAIConfig.GeminiTimeout
			config.MaxTokens = userAIConfig.GeminiMaxTokens
			config.UsesUserConfig = true
			r.Logger.Debugf("✅ 使用用户配置的 Gemini API Key (user_id=%d)", userID)
			return config
		}
	}

	// 2. 回退到系统配置
	if r.AppConfig.GeminiConfig != nil && r.AppConfig.GeminiConfig.Enabled {
		// 优先使用 ApiKeys 数组，否则使用单个 ApiKey
		var systemKeys []string
		if len(r.AppConfig.GeminiConfig.ApiKeys) > 0 {
			systemKeys = r.AppConfig.GeminiConfig.ApiKeys
		} else if r.AppConfig.GeminiConfig.ApiKey != "" {
			systemKeys = []string{r.AppConfig.GeminiConfig.ApiKey}
		}
		if len(systemKeys) > 0 {
			config.Enabled = true
			config.APIKeys = systemKeys
			config.Model = r.AppConfig.GeminiConfig.Model
			config.Timeout = r.AppConfig.GeminiConfig.Timeout
			config.MaxTokens = r.AppConfig.GeminiConfig.MaxTokens
			config.UsesUserConfig = false
			r.Logger.Debugf("✅ 使用系统配置的 Gemini API Keys (%d个密钥)", len(systemKeys))
			return config
		}
	}

	// 3. 无可用配置
	r.Logger.Debugf("⚠️  Gemini 未配置（用户和系统均无API Key）")
	return config
}

// GetOpenAICompatibleConfig 获取OpenAI兼容配置（优先级：用户 → 系统 → 空）
func (r *AIConfigResolver) GetOpenAICompatibleConfig(userID uint) *ResolvedAIConfig {
	config := &ResolvedAIConfig{
		Provider: "openai",
	}

	// 1. 尝试从用户配置获取
	userAIConfig, err := r.UserConfigService.GetUserAIConfig(userID)
	if err == nil && userAIConfig.OpenAIEnabled && userAIConfig.OpenAIAPIKey != "" {
		config.Enabled = true
		config.APIKey = userAIConfig.OpenAIAPIKey
		config.Model = userAIConfig.OpenAIModel
		config.BaseURL = userAIConfig.OpenAIBaseURL
		config.Timeout = userAIConfig.OpenAITimeout
		config.MaxTokens = userAIConfig.OpenAIMaxTokens
		config.UsesUserConfig = true
		r.Logger.Debugf("✅ 使用用户配置的 OpenAI Compatible API (user_id=%d, provider=%s)", userID, userAIConfig.OpenAIProvider)
		return config
	}

	// 2. 回退到系统配置
	if r.AppConfig.OpenAICompatibleConfig != nil && r.AppConfig.OpenAICompatibleConfig.Enabled && r.AppConfig.OpenAICompatibleConfig.ApiKey != "" {
		config.Enabled = true
		config.APIKey = r.AppConfig.OpenAICompatibleConfig.ApiKey
		config.Model = r.AppConfig.OpenAICompatibleConfig.Model
		config.BaseURL = r.AppConfig.OpenAICompatibleConfig.BaseURL
		config.Timeout = r.AppConfig.OpenAICompatibleConfig.Timeout
		config.MaxTokens = r.AppConfig.OpenAICompatibleConfig.MaxTokens
		config.UsesUserConfig = false
		r.Logger.Debugf("✅ 使用系统配置的 OpenAI Compatible API")
		return config
	}

	// 3. 无可用配置
	r.Logger.Debugf("⚠️  OpenAI Compatible API 未配置（用户和系统均无API Key）")
	return config
}

// GetBaiduConfig 获取百度翻译配置（优先级：用户 → 系统 → 空）
func (r *AIConfigResolver) GetBaiduConfig(userID uint) *ResolvedAIConfig {
	config := &ResolvedAIConfig{
		Provider: "baidu",
	}

	// 1. 尝试从用户配置获取
	userAIConfig, err := r.UserConfigService.GetUserAIConfig(userID)
	if err == nil && userAIConfig.BaiduEnabled && userAIConfig.BaiduAppID != "" && userAIConfig.BaiduSecret != "" {
		config.Enabled = true
		config.APIKey = userAIConfig.BaiduAppID // 用APIKey字段存储AppID
		config.Model = userAIConfig.BaiduSecret // 用Model字段存储Secret
		config.UsesUserConfig = true
		r.Logger.Debugf("✅ 使用用户配置的百度翻译 (user_id=%d)", userID)
		return config
	}

	// 2. 回退到系统配置
	if r.AppConfig.BaiduTransConfig != nil && r.AppConfig.BaiduTransConfig.Enabled &&
		r.AppConfig.BaiduTransConfig.AppId != "" && r.AppConfig.BaiduTransConfig.SecretKey != "" {
		config.Enabled = true
		config.APIKey = r.AppConfig.BaiduTransConfig.AppId
		config.Model = r.AppConfig.BaiduTransConfig.SecretKey
		config.UsesUserConfig = false
		r.Logger.Debugf("✅ 使用系统配置的百度翻译")
		return config
	}

	// 3. 无可用配置
	r.Logger.Debugf("⚠️  百度翻译未配置（用户和系统均无配置）")
	return config
}

// GetPrimaryTranslationConfig 获取首选翻译服务配置
// 根据 PrimaryAIService 设置或自动选择可用的翻译服务
func (r *AIConfigResolver) GetPrimaryTranslationConfig(userID uint) *ResolvedAIConfig {
	// 1. 如果用户设置了首选服务，优先使用
	// 注意：这里简化处理，实际可以根据系统设置或用户偏好来决定

	// 2. 优先级：OpenAI Compatible > DeepSeek > Baidu
	if config := r.GetOpenAICompatibleConfig(userID); config.Enabled {
		return config
	}

	if config := r.GetDeepSeekConfig(userID); config.Enabled {
		return config
	}

	if config := r.GetBaiduConfig(userID); config.Enabled {
		return config
	}

	// 3. 无可用翻译服务
	r.Logger.Warn("⚠️  无可用的翻译服务配置")
	return &ResolvedAIConfig{Provider: "none", Enabled: false}
}

// GetMetadataConfig 获取元数据生成配置（固定使用Gemini）
func (r *AIConfigResolver) GetMetadataConfig(userID uint) *ResolvedAIConfig {
	return r.GetGeminiConfig(userID)
}

// HasAnyAIConfig 检查用户是否有任何AI配置（用户配置或系统配置）
func (r *AIConfigResolver) HasAnyAIConfig(userID uint) bool {
	// 检查用户配置
	userAIConfig, err := r.UserConfigService.GetUserAIConfig(userID)
	if err == nil {
		if userAIConfig.DeepSeekEnabled || userAIConfig.GeminiEnabled ||
			userAIConfig.OpenAIEnabled || userAIConfig.BaiduEnabled {
			return true
		}
	}

	// 检查系统配置
	if r.AppConfig.DeepSeekTransConfig != nil && r.AppConfig.DeepSeekTransConfig.Enabled {
		return true
	}
	if r.AppConfig.GeminiConfig != nil && r.AppConfig.GeminiConfig.Enabled {
		return true
	}
	if r.AppConfig.OpenAICompatibleConfig != nil && r.AppConfig.OpenAICompatibleConfig.Enabled {
		return true
	}
	if r.AppConfig.BaiduTransConfig != nil && r.AppConfig.BaiduTransConfig.Enabled {
		return true
	}

	return false
}
