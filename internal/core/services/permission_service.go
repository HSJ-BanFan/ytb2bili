package services

import (
	"context"
	"fmt"
	"time"

	"github.com/difyz9/ytb2bili/internal/core/types"
	"gorm.io/gorm"
)

// PermissionService 权限服务
type PermissionService struct {
	db *gorm.DB
}

// NewPermissionService 创建权限服务
func NewPermissionService(db *gorm.DB) *PermissionService {
	return &PermissionService{db: db}
}

// GetUserMembership 获取用户会员信息
func (s *PermissionService) GetUserMembership(ctx context.Context, userID string) (*types.UserMembership, error) {
	var membership types.UserMembership
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).First(&membership).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 如果没有记录，返回默认的基础版会员
			return &types.UserMembership{
				UserID: userID,
				Tier:   types.TierBasic,
			}, nil
		}
		return nil, err
	}
	return &membership, nil
}

// CanAutoUpload 检查用户是否可以自动上传
func (s *PermissionService) CanAutoUpload(ctx context.Context, userID string) (bool, string, error) {
	membership, err := s.GetUserMembership(ctx, userID)
	if err != nil {
		return false, "", fmt.Errorf("failed to get membership: %w", err)
	}

	config := membership.GetConfig()
	if !config.Features.AutoUpload {
		return false, "自动上传功能需要专业版及以上", nil
	}
	return true, "", nil
}

// GetMaxConcurrentTasks 获取用户最大并发任务数
func (s *PermissionService) GetMaxConcurrentTasks(ctx context.Context, userID string) (int, error) {
	membership, err := s.GetUserMembership(ctx, userID)
	if err != nil {
		// 出错时降级为 1
		return 1, fmt.Errorf("failed to get membership: %w", err)
	}

	config := membership.GetConfig()
	return config.Limits.MaxConcurrentTasks, nil
}

// CheckUploadPermission 检查上传权限（综合检查）
func (s *PermissionService) CheckUploadPermission(ctx context.Context, userID string) error {
	canUpload, reason, err := s.CanAutoUpload(ctx, userID)
	if err != nil {
		return err
	}

	if !canUpload {
		return fmt.Errorf("upload permission denied: %s", reason)
	}

	return nil
}

// GetUserTier 获取用户等级
func (s *PermissionService) GetUserTier(ctx context.Context, userID string) (types.Tier, error) {
	membership, err := s.GetUserMembership(ctx, userID)
	if err != nil {
		return types.TierBasic, err
	}
	return membership.GetEffectiveTier(), nil
}

// CanUseFeature 检查用户是否可以使用特定功能
func (s *PermissionService) CanUseFeature(ctx context.Context, userID string, featureKey string) (bool, string, error) {
	// 获取用户会员信息
	membership, err := s.GetUserMembership(ctx, userID)
	if err != nil {
		return false, "查询会员信息失败", err
	}

	config := membership.GetConfig()
	return s.checkFeatureInConfig(config, featureKey)
}

func (s *PermissionService) checkFeatureInConfig(config types.TierConfig, featureKey string) (bool, string, error) {
	switch featureKey {
	case "auto_upload":
		if config.Features.AutoUpload {
			return true, "", nil
		}
	case "subtitle_translation":
		if config.Features.SubtitleTranslation {
			return true, "", nil
		}
	case "metadata_generation", "gemini_video_analysis":
		if config.Features.MetadataGeneration {
			return true, "", nil
		}
	case "custom_templates":
		if config.Features.CustomTemplates {
			return true, "", nil
		}
	case "priority_support":
		if config.Features.PrioritySupport {
			return true, "", nil
		}
	default:
		// 默认允许未知功能?? 最好是默认拒绝
		return false, "未知功能: " + featureKey, nil
	}

	return false, fmt.Sprintf("%s 等级不支持此功能 (%s)", config.Name, featureKey), nil
}

// QuotaInfo 配额信息
type QuotaInfo struct {
	TotalLimit     int  `json:"total_limit"`
	UsedToday      int  `json:"used_today"`
	TotalRemaining int  `json:"total_remaining"`
	IsUnlimited    bool `json:"is_unlimited"`
}

// GetQuotaInfo 获取用户配额信息
func (s *PermissionService) GetQuotaInfo(ctx context.Context, userID string) (*QuotaInfo, error) {
	// 获取用户会员信息
	tier, err := s.GetUserTier(ctx, userID)
	if err != nil {
		return nil, err
	}

	config := types.GetTierConfig(tier)
	limit := config.Limits.DailyUploadLimit

	used, err := s.GetDailyUsage(ctx, userID)
	if err != nil {
		return nil, err
	}

	isUnlimited := limit <= 0
	remaining := 0
	if !isUnlimited {
		remaining = limit - used
		if remaining < 0 {
			remaining = 0
		}
	} else {
		remaining = 999999 // 无限制
	}

	return &QuotaInfo{
		TotalLimit:     limit,
		UsedToday:      used,
		TotalRemaining: remaining,
		IsUnlimited:    isUnlimited,
	}, nil
}

// ConsumeQuota 消耗配额 (包装 IncrDailyUsage)
func (s *PermissionService) ConsumeQuota(ctx context.Context, userID string) error {
	_, err := s.IncrDailyUsage(ctx, userID)
	return err
}

// IncrDailyUsage 增加每日使用量
func (s *PermissionService) IncrDailyUsage(ctx context.Context, userID string) (int, error) {
	today := time.Now().Format("2006-01-02")
	var usage types.UserUsage

	// 查找或创建今日使用记录
	err := s.db.WithContext(ctx).Where("user_id = ? AND date = ?", userID, today).First(&usage).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			usage = types.UserUsage{
				UserID:    userID,
				Date:      today,
				UsageType: "upload",
				Amount:    1,
			}
			// 如果 UserUsage 定义使用 uint string，则需要转换。
			// 假设 services 包无法看到 uint 转换，我们暂时假设 types.UserUsage 使用 string UserID
			return 1, s.db.Create(&usage).Error
		}
		return 0, err
	}

	// 增加使用次数
	err = s.db.Model(&usage).UpdateColumn("amount", gorm.Expr("amount + ?", 1)).Error
	if err != nil {
		return int(usage.Amount), err
	}
	return int(usage.Amount) + 1, nil
}

// GetDailyUsage 获取每日使用量
func (s *PermissionService) GetDailyUsage(ctx context.Context, userID string) (int, error) {
	today := time.Now().Format("2006-01-02")
	var usage types.UserUsage
	err := s.db.WithContext(ctx).Where("user_id = ? AND date = ?", userID, today).First(&usage).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}
	return int(usage.Amount), nil
}
