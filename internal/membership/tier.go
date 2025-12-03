// Package membership 会员系统核心模块
package membership

import "time"

// Tier 会员等级
type Tier string

const (
	TierFree       Tier = "free"
	TierBasic      Tier = "basic"
	TierPro        Tier = "pro"
	TierEnterprise Tier = "enterprise"
)

// AllTiers 所有等级列表（按优先级排序）
var AllTiers = []Tier{TierFree, TierBasic, TierPro, TierEnterprise}

// Limits 配额限制
type Limits struct {
	VideosPerDay int `json:"videos_per_day"` // -1 表示无限
	BatchSize    int `json:"batch_size"`
}

// Features 功能开关
type Features struct {
	AITranslation       bool `json:"ai_translation"`        // AI 字幕翻译
	TranslationOptimize bool `json:"translation_optimize"`  // 翻译质量优化
	AITitleGeneration   bool `json:"ai_title_generation"`   // AI 标题生成
	GeminiVideoAnalysis bool `json:"gemini_video_analysis"` // Gemini 视频分析
	AutoUpload          bool `json:"auto_upload"`           // 自动上传
	PriorityQueue       bool `json:"priority_queue"`        // 优先队列
	APIAccess           bool `json:"api_access"`            // API 访问
	CustomTemplate      bool `json:"custom_template"`       // 自定义模板
	DataExport          bool `json:"data_export"`           // 数据导出
	TeamCollaboration   bool `json:"team_collaboration"`    // 团队协作
}

// TierConfig 等级配置
type TierConfig struct {
	Tier        Tier     `json:"tier"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       float64  `json:"price"`        // 月价格
	YearlyPrice float64  `json:"yearly_price"` // 年价格
	Limits      Limits   `json:"limits"`
	Features    Features `json:"features"`
	Priority    int      `json:"priority"` // 队列优先级，数值越大优先级越高
}

// DefaultTierConfigs 默认等级配置
var DefaultTierConfigs = map[Tier]TierConfig{
	TierFree: {
		Tier:        TierFree,
		Name:        "免费版",
		Description: "基础功能，适合个人体验",
		Price:       0,
		YearlyPrice: 0,
		Limits:      Limits{VideosPerDay: 5, BatchSize: 1},
		Features:    Features{}, // 全部 false
		Priority:    0,
	},
	TierBasic: {
		Tier:        TierBasic,
		Name:        "基础版",
		Description: "AI 增强功能，适合轻度用户",
		Price:       29,
		YearlyPrice: 290,
		Limits:      Limits{VideosPerDay: 20, BatchSize: 5},
		Features: Features{
			AITranslation:     true,
			AITitleGeneration: true,
			CustomTemplate:    true,
		},
		Priority: 1,
	},
	TierPro: {
		Tier:        TierPro,
		Name:        "专业版",
		Description: "全功能解锁，适合内容创作者",
		Price:       99,
		YearlyPrice: 990,
		Limits:      Limits{VideosPerDay: 100, BatchSize: 20},
		Features: Features{
			AITranslation:       true,
			TranslationOptimize: true,
			AITitleGeneration:   true,
			GeminiVideoAnalysis: true,
			AutoUpload:          true,
			PriorityQueue:       true,
			CustomTemplate:      true,
			DataExport:          true,
		},
		Priority: 2,
	},
	TierEnterprise: {
		Tier:        TierEnterprise,
		Name:        "企业版",
		Description: "无限制使用，适合团队和企业",
		Price:       299,
		YearlyPrice: 2990,
		Limits:      Limits{VideosPerDay: -1, BatchSize: 100}, // -1 表示无限
		Features: Features{
			AITranslation:       true,
			TranslationOptimize: true,
			AITitleGeneration:   true,
			GeminiVideoAnalysis: true,
			AutoUpload:          true,
			PriorityQueue:       true,
			APIAccess:           true,
			CustomTemplate:      true,
			DataExport:          true,
			TeamCollaboration:   true,
		},
		Priority: 3,
	},
}

// UserMembership 用户会员状态
type UserMembership struct {
	UserID         string    `json:"user_id"`
	Tier           Tier      `json:"tier"`
	ExpiresAt      time.Time `json:"expires_at"`
	SubscriptionID string    `json:"subscription_id,omitempty"` // 支付平台订阅ID
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// IsExpired 检查会员是否过期
func (m *UserMembership) IsExpired() bool {
	if m.Tier == TierFree {
		return false // 免费用户永不过期
	}
	// 如果过期时间为零值，认为是永久会员（不过期）
	if m.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(m.ExpiresAt)
}

// GetEffectiveTier 获取有效等级（考虑过期情况）
func (m *UserMembership) GetEffectiveTier() Tier {
	if m.IsExpired() {
		return TierFree
	}
	return m.Tier
}

// GetConfig 获取当前有效的等级配置
func (m *UserMembership) GetConfig() TierConfig {
	tier := m.GetEffectiveTier()
	if config, ok := DefaultTierConfigs[tier]; ok {
		return config
	}
	return DefaultTierConfigs[TierFree]
}

// DaysUntilExpiry 返回距离过期的天数
func (m *UserMembership) DaysUntilExpiry() int {
	if m.Tier == TierFree {
		return -1 // 永不过期
	}
	if m.IsExpired() {
		return 0
	}
	return int(time.Until(m.ExpiresAt).Hours() / 24)
}

// GetTierConfig 获取指定等级的配置
func GetTierConfig(tier Tier) TierConfig {
	if config, ok := DefaultTierConfigs[tier]; ok {
		return config
	}
	return DefaultTierConfigs[TierFree]
}

// GetAllTierConfigs 获取所有等级配置
func GetAllTierConfigs() []TierConfig {
	configs := make([]TierConfig, 0, len(AllTiers))
	for _, tier := range AllTiers {
		configs = append(configs, DefaultTierConfigs[tier])
	}
	return configs
}

// CompareTiers 比较两个等级，返回 -1, 0, 1
func CompareTiers(a, b Tier) int {
	configA := GetTierConfig(a)
	configB := GetTierConfig(b)
	if configA.Priority < configB.Priority {
		return -1
	}
	if configA.Priority > configB.Priority {
		return 1
	}
	return 0
}

// IsHigherTier 检查 a 是否比 b 等级更高
func IsHigherTier(a, b Tier) bool {
	return CompareTiers(a, b) > 0
}
