package membership

import (
	"context"
	"fmt"
)

// PermissionService 权限检查服务
type PermissionService struct {
	store   MembershipStore
	checker *FeatureChecker
}

// NewPermissionService 创建权限服务
func NewPermissionService(store MembershipStore, checker *FeatureChecker) *PermissionService {
	return &PermissionService{
		store:   store,
		checker: checker,
	}
}

// CanAutoUpload 检查用户是否可以自动上传
func (s *PermissionService) CanAutoUpload(ctx context.Context, userID string) (bool, string, error) {
	membership, err := s.store.GetUserMembership(ctx, userID)
	if err != nil {
		return false, "", fmt.Errorf("获取会员信息失败: %w", err)
	}

	config := membership.GetConfig()
	if !config.Features.AutoUpload {
		return false, fmt.Sprintf("自动上传是 %s 会员功能", TierPro), nil
	}
	return true, "", nil
}

// GetMaxConcurrentTasks 获取用户最大并发任务数
func (s *PermissionService) GetMaxConcurrentTasks(ctx context.Context, userID string) (int, error) {
	membership, err := s.store.GetUserMembership(ctx, userID)
	if err != nil {
		return 1, fmt.Errorf("获取会员信息失败: %w", err)
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
		return fmt.Errorf("%s，请升级会员", reason)
	}

	return nil
}

// GetUserTier 获取用户等级
func (s *PermissionService) GetUserTier(ctx context.Context, userID string) (Tier, error) {
	membership, err := s.store.GetUserMembership(ctx, userID)
	if err != nil {
		return TierFree, err
	}
	return membership.GetEffectiveTier(), nil
}
