package services

import (
	"fmt"

	"github.com/difyz9/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// UserConfigService 用户配置服务
type UserConfigService struct {
	DB     *gorm.DB
	Logger *zap.SugaredLogger
}

// NewUserConfigService 创建用户配置服务
func NewUserConfigService(db *gorm.DB, logger *zap.SugaredLogger) *UserConfigService {
	return &UserConfigService{
		DB:     db,
		Logger: logger,
	}
}

// GetOrCreateUserAIConfig 获取或创建用户AI配置
func (s *UserConfigService) GetOrCreateUserAIConfig(userID uint) (*model.UserAIConfig, error) {
	var config model.UserAIConfig
	err := s.DB.Where("user_id = ?", userID).First(&config).Error

	if err == gorm.ErrRecordNotFound {
		// 创建默认配置
		config = model.UserAIConfig{
			UserID:          userID,
			DeepSeekModel:   "deepseek-chat",
			GeminiModel:     "gemini-2.0-flash",
			OpenAIModel:     "gpt-3.5-turbo",
			GeminiTimeout:   120,
			GeminiMaxTokens: 8000,
			OpenAITimeout:   60,
			OpenAIMaxTokens: 4000,
		}
		if err := s.DB.Create(&config).Error; err != nil {
			return nil, fmt.Errorf("创建用户AI配置失败: %w", err)
		}
		s.Logger.Infof("✅ 为用户 %d 创建默认AI配置", userID)
		return &config, nil
	}

	if err != nil {
		return nil, fmt.Errorf("查询用户AI配置失败: %w", err)
	}

	return &config, nil
}

// UpdateUserAIConfig 更新用户AI配置
func (s *UserConfigService) UpdateUserAIConfig(userID uint, config *model.UserAIConfig) error {
	config.UserID = userID
	return s.DB.Save(config).Error
}

// GetUserAIConfig 获取用户AI配置（不自动创建）
func (s *UserConfigService) GetUserAIConfig(userID uint) (*model.UserAIConfig, error) {
	var config model.UserAIConfig
	err := s.DB.Where("user_id = ?", userID).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// GetOrCreateUserPreference 获取或创建用户偏好
func (s *UserConfigService) GetOrCreateUserPreference(userID uint) (*model.UserPreference, error) {
	var pref model.UserPreference
	err := s.DB.Where("user_id = ?", userID).First(&pref).Error

	if err == gorm.ErrRecordNotFound {
		// 创建默认偏好
		pref = model.UserPreference{
			UserID:                    userID,
			DefaultAutoUpload:         true,
			DefaultUploadDelay:        10,
			DefaultSubtitleDelay:      10,
			DefaultCopyright:          2,
			DefaultSource:             "YouTube",
			DefaultTid:                122,
			Theme:                     "light",
			Language:                  "zh",
			ItemsPerPage:              20,
			ShowAdvanced:              false,
			EmailNotificationsEnabled: true,
			EnableAnalytics:           false,
		}
		if err := s.DB.Create(&pref).Error; err != nil {
			return nil, fmt.Errorf("创建用户偏好失败: %w", err)
		}
		s.Logger.Infof("✅ 为用户 %d 创建默认偏好", userID)
		return &pref, nil
	}

	if err != nil {
		return nil, fmt.Errorf("查询用户偏好失败: %w", err)
	}

	return &pref, nil
}

// UpdateUserPreference 更新用户偏好
func (s *UserConfigService) UpdateUserPreference(userID uint, pref *model.UserPreference) error {
	pref.UserID = userID
	return s.DB.Save(pref).Error
}

// GetUserPreference 获取用户偏好（不自动创建）
func (s *UserConfigService) GetUserPreference(userID uint) (*model.UserPreference, error) {
	var pref model.UserPreference
	err := s.DB.Where("user_id = ?", userID).First(&pref).Error
	if err != nil {
		return nil, err
	}
	return &pref, nil
}

// HasConfiguredAI 检查用户是否配置了AI服务
func (s *UserConfigService) HasConfiguredAI(userID uint, provider string) bool {
	config, err := s.GetUserAIConfig(userID)
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
