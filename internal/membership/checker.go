package membership

import (
	"context"
	"fmt"
	"time"
)

// CheckResult 检查结果
type CheckResult struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
	Code    string `json:"code,omitempty"`    // 错误码
	Upgrade string `json:"upgrade,omitempty"` // 建议升级的等级
}

// FeatureChecker 功能检查器
type FeatureChecker struct {
	store MembershipStore
}

// NewFeatureChecker 创建功能检查器
func NewFeatureChecker(store MembershipStore) *FeatureChecker {
	return &FeatureChecker{store: store}
}

// GetUserMembership 获取用户会员信息
func (c *FeatureChecker) GetUserMembership(ctx context.Context, userID string) (*UserMembership, error) {
	return c.store.GetUserMembership(ctx, userID)
}

// CanUseFeature 检查用户是否可以使用某功能
func (c *FeatureChecker) CanUseFeature(ctx context.Context, userID, feature string) CheckResult {
	membership, err := c.store.GetUserMembership(ctx, userID)
	if err != nil {
		return CheckResult{
			Allowed: false,
			Reason:  "获取会员信息失败",
			Code:    "MEMBERSHIP_ERROR",
		}
	}

	config := membership.GetConfig()

	// 功能映射表：功能名 -> {是否启用, 最低要求等级, 提示信息}
	type featureInfo struct {
		enabled bool
		upgrade Tier
		msg     string
	}

	featureMap := map[string]featureInfo{
		"ai_translation":        {config.Features.AITranslation, TierBasic, "AI 字幕翻译是付费功能"},
		"translation_optimize":  {config.Features.TranslationOptimize, TierPro, "翻译质量优化是专业版功能"},
		"ai_title_generation":   {config.Features.AITitleGeneration, TierBasic, "AI 标题生成是付费功能"},
		"gemini_video_analysis": {config.Features.GeminiVideoAnalysis, TierPro, "Gemini 视频分析是专业版功能"},
		"auto_upload":           {config.Features.AutoUpload, TierPro, "自动上传是专业版功能"},
		"priority_queue":        {config.Features.PriorityQueue, TierPro, "优先队列是专业版功能"},
		"api_access":            {config.Features.APIAccess, TierEnterprise, "API 访问是企业版功能"},
		"custom_template":       {config.Features.CustomTemplate, TierBasic, "自定义模板是付费功能"},
		"data_export":           {config.Features.DataExport, TierPro, "数据导出是专业版功能"},
		"team_collaboration":    {config.Features.TeamCollaboration, TierEnterprise, "团队协作是企业版功能"},
		"multi_account_upload":  {config.Features.MultiAccountUpload, TierEnterprise, "多账号上传是企业版专属功能，请升级到企业版"},
	}

	if f, ok := featureMap[feature]; ok {
		if !f.enabled {
			return CheckResult{
				Allowed: false,
				Reason:  f.msg,
				Code:    "FEATURE_NOT_ALLOWED",
				Upgrade: string(f.upgrade),
			}
		}
	}

	return CheckResult{Allowed: true}
}

// CanProcessVideo 检查用户是否可以处理视频（配额检查）
func (c *FeatureChecker) CanProcessVideo(ctx context.Context, userID string) CheckResult {
	membership, err := c.store.GetUserMembership(ctx, userID)
	if err != nil {
		return CheckResult{
			Allowed: false,
			Reason:  "获取会员信息失败",
			Code:    "MEMBERSHIP_ERROR",
		}
	}

	config := membership.GetConfig()

	// 无限配额
	if config.Limits.VideosPerDay == -1 {
		return CheckResult{Allowed: true}
	}

	// 检查每日使用量
	today := time.Now().Format("2006-01-02")
	used, err := c.store.GetDailyUsage(ctx, userID, today)
	if err != nil {
		return CheckResult{
			Allowed: false,
			Reason:  "获取使用量失败",
			Code:    "USAGE_ERROR",
		}
	}

	// 每日配额未用完
	if used < config.Limits.VideosPerDay {
		return CheckResult{Allowed: true}
	}

	// 检查加油包
	boostPack, _ := c.store.GetUserBoostPack(ctx, userID)
	if boostPack != nil && boostPack.IsValid() {
		return CheckResult{Allowed: true}
	}

	// 配额已用完
	return CheckResult{
		Allowed: false,
		Reason:  fmt.Sprintf("今日配额已用完 (%d/%d)", used, config.Limits.VideosPerDay),
		Code:    "QUOTA_EXCEEDED",
		Upgrade: c.suggestUpgrade(membership.GetEffectiveTier()),
	}
}

// CanBatchSubmit 检查用户是否可以批量提交
func (c *FeatureChecker) CanBatchSubmit(ctx context.Context, userID string, count int) CheckResult {
	membership, err := c.store.GetUserMembership(ctx, userID)
	if err != nil {
		return CheckResult{
			Allowed: false,
			Reason:  "获取会员信息失败",
			Code:    "MEMBERSHIP_ERROR",
		}
	}

	config := membership.GetConfig()

	if count > config.Limits.BatchSize {
		return CheckResult{
			Allowed: false,
			Reason:  fmt.Sprintf("批量提交数量超限 (最多 %d 个)", config.Limits.BatchSize),
			Code:    "BATCH_SIZE_EXCEEDED",
			Upgrade: c.suggestUpgrade(membership.GetEffectiveTier()),
		}
	}

	return CheckResult{Allowed: true}
}

// GetUserPriority 获取用户队列优先级
func (c *FeatureChecker) GetUserPriority(ctx context.Context, userID string) int {
	membership, err := c.store.GetUserMembership(ctx, userID)
	if err != nil {
		return 0 // 默认最低优先级
	}
	return membership.GetConfig().Priority
}

// GetUserTier 获取用户当前有效等级
func (c *FeatureChecker) GetUserTier(ctx context.Context, userID string) Tier {
	membership, err := c.store.GetUserMembership(ctx, userID)
	if err != nil {
		return TierFree
	}
	return membership.GetEffectiveTier()
}

// suggestUpgrade 根据当前等级建议升级目标
func (c *FeatureChecker) suggestUpgrade(current Tier) string {
	switch current {
	case TierFree:
		return string(TierBasic)
	case TierBasic:
		return string(TierPro)
	case TierPro:
		return string(TierEnterprise)
	default:
		return ""
	}
}
