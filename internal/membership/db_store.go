package membership

import (
	"context"
	"fmt"
	"time"

	"github.com/difyz9/ytb2bili/pkg/store/model"
	"gorm.io/gorm"
)

// DBMembershipStore 数据库实现的会员存储
type DBMembershipStore struct {
	db *gorm.DB
}

// NewDBMembershipStore 创建数据库会员存储
func NewDBMembershipStore(db *gorm.DB) *DBMembershipStore {
	return &DBMembershipStore{db: db}
}

// ensureUserExists 确保用户存在，如果不存在则自动创建免费会员
func (s *DBMembershipStore) ensureUserExists(ctx context.Context, userID string) error {
	var count int64
	s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Count(&count)
	if count > 0 {
		return nil // 用户已存在
	}

	// 自动创建免费会员用户
	now := time.Now()
	username := fmt.Sprintf("bili_%s", userID)
	email := fmt.Sprintf("%s@bilibili.user", userID)

	// 使用原始 SQL 插入，因为 ID 是 B站的 MID（大数字）
	// password 字段是 NOT NULL，使用空字符串占位（B站用户不需要密码）
	result := s.db.WithContext(ctx).Exec(
		`INSERT INTO cw_users (id, username, email, password, status, membership_tier, created_at, updated_at, daily_usage_count, boost_pack_videos) 
		 VALUES (?, ?, ?, '', 1, 'free', ?, ?, 0, 0)`,
		userID, username, email, now, now,
	)
	if result.Error != nil {
		return fmt.Errorf("创建用户失败: %w", result.Error)
	}

	fmt.Printf("✅ 自动创建会员用户: %s\n", userID)
	return nil
}

// GetUserMembership 获取用户会员状态
func (s *DBMembershipStore) GetUserMembership(ctx context.Context, userID string) (*UserMembership, error) {
	// 确保用户存在
	if err := s.ensureUserExists(ctx, userID); err != nil {
		// 如果创建失败，仍然返回默认免费会员（不阻塞流程）
		return &UserMembership{
			UserID:    userID,
			Tier:      TierFree,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil
	}

	var user model.User
	err := s.db.WithContext(ctx).Where("id = ?", userID).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		// 用户不存在，返回免费会员
		return &UserMembership{
			UserID:    userID,
			Tier:      TierFree,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 转换为会员状态
	tier := Tier(user.MembershipTier)
	if tier == "" {
		tier = TierFree
	}

	// 处理时间指针
	var expiresAt time.Time
	if user.MembershipExpire != nil {
		expiresAt = *user.MembershipExpire
	}

	return &UserMembership{
		UserID:         fmt.Sprintf("%d", user.ID),
		Tier:           tier,
		ExpiresAt:      expiresAt,
		SubscriptionID: user.SubscriptionID,
		CreatedAt:      user.CreatedAt,
		UpdatedAt:      user.UpdatedAt,
	}, nil
}

// SaveUserMembership 保存用户会员状态
func (s *DBMembershipStore) SaveUserMembership(ctx context.Context, m *UserMembership) error {
	now := time.Now()

	// 更新会员相关字段
	updates := map[string]interface{}{
		"membership_tier":   string(m.Tier),
		"membership_expire": m.ExpiresAt,
		"subscription_id":   m.SubscriptionID,
		"updated_at":        now,
	}

	result := s.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", m.UserID).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("更新会员状态失败: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("用户不存在: %s", m.UserID)
	}

	return nil
}

// GetDailyUsage 获取用户当日使用量
func (s *DBMembershipStore) GetDailyUsage(ctx context.Context, userID string, date string) (int, error) {
	var user model.User
	err := s.db.WithContext(ctx).
		Select("daily_usage_count", "daily_usage_date").
		Where("id = ?", userID).
		First(&user).Error

	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("查询使用量失败: %w", err)
	}

	// 如果日期不匹配，说明是新的一天，返回0
	if user.DailyUsageDate != date {
		return 0, nil
	}

	return user.DailyUsageCount, nil
}

// IncrDailyUsage 增加用户当日使用量
func (s *DBMembershipStore) IncrDailyUsage(ctx context.Context, userID string, date string) (int, error) {
	var user model.User
	err := s.db.WithContext(ctx).
		Select("daily_usage_count", "daily_usage_date").
		Where("id = ?", userID).
		First(&user).Error

	if err == gorm.ErrRecordNotFound {
		return 0, fmt.Errorf("用户不存在: %s", userID)
	}
	if err != nil {
		return 0, fmt.Errorf("查询用户失败: %w", err)
	}

	var newCount int
	if user.DailyUsageDate == date {
		// 同一天，增加计数
		newCount = user.DailyUsageCount + 1
	} else {
		// 新的一天，重置计数
		newCount = 1
	}

	// 更新数据库
	err = s.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"daily_usage_count": newCount,
			"daily_usage_date":  date,
		}).Error

	if err != nil {
		return 0, fmt.Errorf("更新使用量失败: %w", err)
	}

	return newCount, nil
}

// GetUserBoostPack 获取用户加油包状态
func (s *DBMembershipStore) GetUserBoostPack(ctx context.Context, userID string) (*UserBoostPack, error) {
	var user model.User
	err := s.db.WithContext(ctx).
		Select("boost_pack_videos", "boost_pack_expire").
		Where("id = ?", userID).
		First(&user).Error

	if err == gorm.ErrRecordNotFound {
		return &UserBoostPack{
			UserID:          userID,
			VideosRemaining: 0,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询加油包失败: %w", err)
	}

	var boostExpire time.Time
	if user.BoostPackExpire != nil {
		boostExpire = *user.BoostPackExpire
	}

	return &UserBoostPack{
		UserID:          userID,
		VideosRemaining: user.BoostPackVideos,
		ExpiresAt:       boostExpire,
	}, nil
}

// SaveUserBoostPack 保存用户加油包状态
func (s *DBMembershipStore) SaveUserBoostPack(ctx context.Context, pack *UserBoostPack) error {
	err := s.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", pack.UserID).
		Updates(map[string]interface{}{
			"boost_pack_videos": pack.VideosRemaining,
			"boost_pack_expire": pack.ExpiresAt,
		}).Error

	if err != nil {
		return fmt.Errorf("保存加油包失败: %w", err)
	}

	return nil
}

// DecrBoostPack 消耗加油包配额
func (s *DBMembershipStore) DecrBoostPack(ctx context.Context, userID string) error {
	pack, err := s.GetUserBoostPack(ctx, userID)
	if err != nil {
		return err
	}

	if !pack.IsValid() {
		return fmt.Errorf("加油包无效或已过期")
	}

	// 原子减少
	result := s.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ? AND boost_pack_videos > 0", userID).
		Update("boost_pack_videos", gorm.Expr("boost_pack_videos - 1"))

	if result.Error != nil {
		return fmt.Errorf("消耗加油包失败: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("加油包配额不足")
	}

	return nil
}

// AutoMigrate 自动迁移数据库表结构
func (s *DBMembershipStore) AutoMigrate() error {
	return s.db.AutoMigrate(&model.User{})
}

// MigrateExistingUsers 迁移现有用户数据（将 membership_tier 为空的用户设置为 free）
func (s *DBMembershipStore) MigrateExistingUsers(ctx context.Context) error {
	return s.db.WithContext(ctx).
		Model(&model.User{}).
		Where("membership_tier = '' OR membership_tier IS NULL").
		Update("membership_tier", string(TierFree)).Error
}
