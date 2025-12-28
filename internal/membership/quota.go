package membership

import (
	"context"
	"fmt"
	"time"
)

// QuotaService 配额服务
type QuotaService struct {
	store   MembershipStore
	checker *FeatureChecker
}

// NewQuotaService 创建配额服务
func NewQuotaService(store MembershipStore, checker *FeatureChecker) *QuotaService {
	return &QuotaService{
		store:   store,
		checker: checker,
	}
}

// QuotaInfo 配额信息
type QuotaInfo struct {
	DailyLimit         int  `json:"daily_limit"`          // 每日限制 (-1 表示无限)
	DailyUsed          int  `json:"daily_used"`           // 今日已用
	DailyRemaining     int  `json:"daily_remaining"`      // 今日剩余
	BoostPackRemaining int  `json:"boost_pack_remaining"` // 加油包剩余
	TotalRemaining     int  `json:"total_remaining"`      // 总剩余 (每日+加油包)
	IsUnlimited        bool `json:"is_unlimited"`         // 是否无限
}

// GetQuotaInfo 获取用户配额信息
func (s *QuotaService) GetQuotaInfo(ctx context.Context, userID string) (*QuotaInfo, error) {
	membership, err := s.store.GetUserMembership(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("获取会员信息失败: %w", err)
	}

	config := membership.GetConfig()

	// 无限配额
	if config.Limits.VideosPerDay == -1 {
		return &QuotaInfo{
			DailyLimit:  -1,
			IsUnlimited: true,
		}, nil
	}

	// 获取今日使用量
	today := time.Now().Format("2006-01-02")
	used, err := s.store.GetDailyUsage(ctx, userID, today)
	if err != nil {
		return nil, fmt.Errorf("获取使用量失败: %w", err)
	}

	// 计算每日剩余
	dailyRemaining := config.Limits.VideosPerDay - used
	if dailyRemaining < 0 {
		dailyRemaining = 0
	}

	// 获取加油包剩余
	boostPack, _ := s.store.GetUserBoostPack(ctx, userID)
	boostRemaining := 0
	if boostPack != nil && boostPack.IsValid() {
		boostRemaining = boostPack.VideosRemaining
	}

	return &QuotaInfo{
		DailyLimit:         config.Limits.VideosPerDay,
		DailyUsed:          used,
		DailyRemaining:     dailyRemaining,
		BoostPackRemaining: boostRemaining,
		TotalRemaining:     dailyRemaining + boostRemaining,
		IsUnlimited:        false,
	}, nil
}

// ConsumeQuota 消耗配额（处理一个视频时调用）
func (s *QuotaService) ConsumeQuota(ctx context.Context, userID string) error {
	// 先检查是否有配额
	check := s.checker.CanProcessVideo(ctx, userID)
	if !check.Allowed {
		return fmt.Errorf("配额不足: %s", check.Reason)
	}

	membership, err := s.store.GetUserMembership(ctx, userID)
	if err != nil {
		return fmt.Errorf("获取会员信息失败: %w", err)
	}

	config := membership.GetConfig()

	// 无限配额，不需要消耗
	if config.Limits.VideosPerDay == -1 {
		return nil
	}

	// 获取今日使用量
	today := time.Now().Format("2006-01-02")
	used, err := s.store.GetDailyUsage(ctx, userID, today)
	if err != nil {
		return fmt.Errorf("获取使用量失败: %w", err)
	}

	// 优先消耗每日配额
	if used < config.Limits.VideosPerDay {
		newCount, err := s.store.IncrDailyUsage(ctx, userID, today)
		if err != nil {
			return fmt.Errorf("增加每日使用量失败: %w", err)
		}
		// 添加日志便于调试
		fmt.Printf("✅ 用户 %s 消耗每日配额: %d/%d\n", userID, newCount, config.Limits.VideosPerDay)
		return nil
	}

	// 每日配额用完，消耗加油包
	err = s.store.DecrBoostPack(ctx, userID)
	if err != nil {
		// 提供更详细的错误信息
		return fmt.Errorf("消耗加油包失败 (每日配额已用完 %d/%d): %w", used, config.Limits.VideosPerDay, err)
	}
	fmt.Printf("✅ 用户 %s 消耗加油包配额\n", userID)
	return nil
}

// HasQuota 检查用户是否有可用配额
func (s *QuotaService) HasQuota(ctx context.Context, userID string) bool {
	check := s.checker.CanProcessVideo(ctx, userID)
	return check.Allowed
}

// GetBatchLimit 获取用户批量提交限制
func (s *QuotaService) GetBatchLimit(ctx context.Context, userID string) int {
	membership, err := s.store.GetUserMembership(ctx, userID)
	if err != nil {
		return 1 // 默认最小批量
	}
	return membership.GetConfig().Limits.BatchSize
}

// CanBatch 检查用户是否可以批量提交指定数量
func (s *QuotaService) CanBatch(ctx context.Context, userID string, count int) bool {
	check := s.checker.CanBatchSubmit(ctx, userID, count)
	return check.Allowed
}
