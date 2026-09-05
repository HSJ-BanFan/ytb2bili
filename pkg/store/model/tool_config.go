package model

import (
	"time"
)

// ToolAIConfig 工具AI配置
type ToolAIConfig struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// DeepSeek 配置
	DeepSeekEnabled bool   `gorm:"default:false" json:"deepseek_enabled"`
	DeepSeekAPIKey  string `gorm:"size:255" json:"deepseek_api_key,omitempty"`
	DeepSeekModel   string `gorm:"size:50;default:'deepseek-chat'" json:"deepseek_model"`

	// Gemini 配置
	GeminiEnabled   bool   `gorm:"default:false" json:"gemini_enabled"`
	GeminiAPIKey    string `gorm:"size:255" json:"gemini_api_key,omitempty"`
	GeminiAPIKeys   string `gorm:"type:text" json:"gemini_api_keys,omitempty"` // JSON数组
	GeminiModel     string `gorm:"size:50;default:'gemini-2.0-flash'" json:"gemini_model"`
	GeminiTimeout   int    `gorm:"default:120" json:"gemini_timeout"`
	GeminiMaxTokens int    `gorm:"default:8000" json:"gemini_max_tokens"`

	// OpenAI 兼容配置
	OpenAIEnabled   bool   `gorm:"default:false" json:"openai_enabled"`
	OpenAIProvider  string `gorm:"size:50" json:"openai_provider,omitempty"`
	OpenAIAPIKey    string `gorm:"size:255" json:"openai_api_key,omitempty"`
	OpenAIBaseURL   string `gorm:"size:255" json:"openai_base_url,omitempty"`
	OpenAIModel     string `gorm:"size:50;default:'gpt-3.5-turbo'" json:"openai_model"`
	OpenAITimeout   int    `gorm:"default:60" json:"openai_timeout"`
	OpenAIMaxTokens int    `gorm:"default:4000" json:"openai_max_tokens"`

	// Baidu 配置
	BaiduEnabled bool   `gorm:"default:false" json:"baidu_enabled"`
	BaiduAppID   string `gorm:"size:100" json:"baidu_app_id,omitempty"`
	BaiduSecret  string `gorm:"size:255" json:"baidu_secret,omitempty"`

	// 备注
	Notes string `gorm:"type:text" json:"notes,omitempty"`
}

// ToolPreference 工具全局偏好
type ToolPreference struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 任务默认设置
	DefaultAutoUpload    bool   `gorm:"default:true" json:"default_auto_upload"`
	DefaultUploadDelay   int    `gorm:"default:10" json:"default_upload_delay"`
	DefaultSubtitleDelay int    `gorm:"default:10" json:"default_subtitle_delay"`
	DefaultCopyright     int    `gorm:"default:2" json:"default_copyright"`
	DefaultSource        string `gorm:"default:'YouTube'" json:"default_source"`
	DefaultTid           int    `gorm:"default:122" json:"default_tid"`

	// 界面设置
	Theme        string `gorm:"default:'light';size:20" json:"theme"` // light/dark
	Language     string `gorm:"default:'zh';size:10" json:"language"` // zh/en
	ItemsPerPage int    `gorm:"default:20" json:"items_per_page"`
	ShowAdvanced bool   `gorm:"default:false" json:"show_advanced"`

	// 隐私设置
	EnableAnalytics bool `gorm:"default:false" json:"enable_analytics"`
}

// TableName 指定表名
func (ToolAIConfig) TableName() string {
	return "cw_user_ai_configs"
}

func (ToolPreference) TableName() string {
	return "cw_user_preferences"
}
