package services

import (
	"fmt"

	"github.com/difyz9/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ToolConfigService 管理工具的全局配置。
type ToolConfigService struct {
	DB     *gorm.DB
	Logger *zap.SugaredLogger
}

func NewToolConfigService(db *gorm.DB, logger *zap.SugaredLogger) *ToolConfigService {
	return &ToolConfigService{DB: db, Logger: logger}
}

func (s *ToolConfigService) GetOrCreateAIConfig() (*model.ToolAIConfig, error) {
	var config model.ToolAIConfig
	if err := s.DB.First(&config).Error; err == nil {
		return &config, nil
	} else if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("查询AI配置失败: %w", err)
	}

	config = model.ToolAIConfig{
		DeepSeekModel:   "deepseek-chat",
		GeminiModel:     "gemini-2.0-flash",
		OpenAIModel:     "gpt-3.5-turbo",
		GeminiTimeout:   120,
		GeminiMaxTokens: 8000,
		OpenAITimeout:   60,
		OpenAIMaxTokens: 4000,
	}
	if err := s.DB.Create(&config).Error; err != nil {
		return nil, fmt.Errorf("创建AI配置失败: %w", err)
	}
	return &config, nil
}

func (s *ToolConfigService) UpdateAIConfig(config *model.ToolAIConfig) error {
	return s.DB.Save(config).Error
}

func (s *ToolConfigService) GetAIConfig() (*model.ToolAIConfig, error) {
	var config model.ToolAIConfig
	if err := s.DB.First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

func (s *ToolConfigService) GetOrCreatePreference() (*model.ToolPreference, error) {
	var pref model.ToolPreference
	if err := s.DB.First(&pref).Error; err == nil {
		return &pref, nil
	} else if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("查询偏好设置失败: %w", err)
	}

	pref = model.ToolPreference{
		DefaultAutoUpload:    true,
		DefaultUploadDelay:   10,
		DefaultSubtitleDelay: 10,
		DefaultCopyright:     2,
		DefaultSource:        "YouTube",
		DefaultTid:           122,
		Theme:                "light",
		Language:             "zh",
		ItemsPerPage:         20,
	}
	if err := s.DB.Create(&pref).Error; err != nil {
		return nil, fmt.Errorf("创建偏好设置失败: %w", err)
	}
	return &pref, nil
}

func (s *ToolConfigService) UpdatePreference(pref *model.ToolPreference) error {
	return s.DB.Save(pref).Error
}

func (s *ToolConfigService) GetPreference() (*model.ToolPreference, error) {
	var pref model.ToolPreference
	if err := s.DB.First(&pref).Error; err != nil {
		return nil, err
	}
	return &pref, nil
}

func (s *ToolConfigService) HasConfiguredAI(provider string) bool {
	config, err := s.GetAIConfig()
	if err != nil {
		return false
	}

	switch provider {
	case "deepseek":
		return config.DeepSeekEnabled && config.DeepSeekAPIKey != ""
	case "gemini":
		return config.GeminiEnabled && (config.GeminiAPIKey != "" || config.GeminiAPIKeys != "")
	case "openai":
		return config.OpenAIEnabled && config.OpenAIAPIKey != ""
	default:
		return false
	}
}
