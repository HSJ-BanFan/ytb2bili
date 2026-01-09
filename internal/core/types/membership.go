package types

import "time"

// Tier 会员等级
type Tier string

const (
	TierBasic      Tier = "basic"
	TierPro        Tier = "pro"
	TierEnterprise Tier = "enterprise"
)

// LicenseActivation 许可证激活记录
type LicenseActivation struct {
	ID          int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	LicenseKey  string     `gorm:"type:varchar(64);uniqueIndex;not null" json:"license_key"`
	UserID      string     `gorm:"type:varchar(64);index;not null" json:"user_id"`
	Tier        Tier       `gorm:"type:varchar(32);not null" json:"tier"`
	Plan        string     `gorm:"type:varchar(32);not null" json:"plan"`
	ExpiresAt   *time.Time `gorm:"type:datetime" json:"expires_at,omitempty"`
	ActivatedAt time.Time  `gorm:"type:datetime;not null" json:"activated_at"`
	CreatedAt   time.Time  `gorm:"type:datetime;not null;autoCreateTime" json:"created_at"`
}

func (LicenseActivation) TableName() string {
	return "cw_license_activations"
}

// UserMembership 用户会员信息
type UserMembership struct {
	UserID    string    `gorm:"type:varchar(64);primaryKey" json:"user_id"`
	Tier      Tier      `gorm:"type:varchar(32);not null;default:free;index" json:"tier"`
	ExpiresAt time.Time `gorm:"type:datetime;index" json:"expires_at"`
	CreatedAt time.Time `gorm:"type:datetime;not null;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:datetime;not null;autoUpdateTime" json:"updated_at"`
}

func (UserMembership) TableName() string {
	return "cw_user_memberships"
}

// GetEffectiveTier 获取有效等级（检查是否过期）
func (m *UserMembership) GetEffectiveTier() Tier {
	if m == nil {
		return TierBasic
	}
	// 如果 ExpiresAt 是零值，表示永久有效
	if m.ExpiresAt.IsZero() {
		return m.Tier
	}
	if time.Now().After(m.ExpiresAt) {
		return TierBasic
	}
	return m.Tier
}

// TierConfig 会员等级配置
type TierConfig struct {
	Tier     Tier         `json:"tier"`
	Name     string       `json:"name"`
	Features TierFeatures `json:"features"`
	Limits   TierLimits   `json:"limits"`
}

// TierFeatures 功能特性开关
type TierFeatures struct {
	AutoUpload          bool `json:"auto_upload"`          // 自动上传
	SubtitleTranslation bool `json:"subtitle_translation"` // 字幕翻译
	MetadataGeneration  bool `json:"metadata_generation"`  // 元数据生成
	CustomTemplates     bool `json:"custom_templates"`     // 自定义模板
	PrioritySupport     bool `json:"priority_support"`     // 优先支持
}

// TierLimits 限制配置
type TierLimits struct {
	MaxConcurrentTasks int   `json:"max_concurrent_tasks"` // 最大并发任务数
	DailyUploadLimit   int   `json:"daily_upload_limit"`   // 每日上传限制 (0=无限制)
	MaxVideoDuration   int   `json:"max_video_duration"`   // 最大视频时长 (分钟, 0=无限制)
	StorageLimit       int64 `json:"storage_limit"`        // 存储空间限制 (MB, 0=无限制)
}

// GetConfig 获取会员配置
func (m *UserMembership) GetConfig() TierConfig {
	tier := m.GetEffectiveTier()
	return GetTierConfig(tier)
}

// GetTierConfig 获取指定等级的配置
func GetTierConfig(tier Tier) TierConfig {
	switch tier {
	case TierBasic:
		return TierConfig{
			Tier: TierBasic,
			Name: "基础版",
			Features: TierFeatures{
				AutoUpload:          true,
				SubtitleTranslation: false,
				MetadataGeneration:  false,
				CustomTemplates:     false,
				PrioritySupport:     false,
			},
			Limits: TierLimits{
				MaxConcurrentTasks: 2,
				DailyUploadLimit:   10,
				MaxVideoDuration:   60,
				StorageLimit:       10240, // 10GB
			},
		}
	case TierPro:
		return TierConfig{
			Tier: TierPro,
			Name: "专业版",
			Features: TierFeatures{
				AutoUpload:          true,
				SubtitleTranslation: false, // User text didn't list it for Pro
				MetadataGeneration:  true,
				CustomTemplates:     true,
				PrioritySupport:     true,
			},
			Limits: TierLimits{
				MaxConcurrentTasks: 5,
				DailyUploadLimit:   50,
				MaxVideoDuration:   180,
				StorageLimit:       51200, // 50GB
			},
		}
	case TierEnterprise:
		return TierConfig{
			Tier: TierEnterprise,
			Name: "企业版",
			Features: TierFeatures{
				AutoUpload:          true,
				SubtitleTranslation: true,
				MetadataGeneration:  true,
				CustomTemplates:     true,
				PrioritySupport:     true,
			},
			Limits: TierLimits{
				MaxConcurrentTasks: 10,
				DailyUploadLimit:   0, // 无限制
				MaxVideoDuration:   0, // 无限制
				StorageLimit:       0, // 无限制
			},
		}
	default: // Basic (default tier)
		return TierConfig{
			Tier: TierBasic,
			Name: "基础版",
			Features: TierFeatures{
				AutoUpload:          true,
				SubtitleTranslation: false,
				MetadataGeneration:  false,
				CustomTemplates:     false,
				PrioritySupport:     false,
			},
			Limits: TierLimits{
				MaxConcurrentTasks: 2,
				DailyUploadLimit:   10,
				MaxVideoDuration:   60,
				StorageLimit:       10240, // 10GB
			},
		}
	}
}

// UserUsage 用户用量统计
type UserUsage struct {
	UserID    string    `gorm:"primaryKey;type:varchar(32)"`
	Date      string    `gorm:"primaryKey;type:varchar(10);index"` // YYYY-MM-DD
	UsageType string    `gorm:"primaryKey;type:varchar(32)"`       // bandwidth, api_call, etc.
	Amount    int64     `gorm:"type:bigint"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}
