package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/difyz9/bilibili-go-sdk/bilibili"
	"github.com/difyz9/ytb2bili/internal/storage"
	"github.com/difyz9/ytb2bili/pkg/store/model"
	"gorm.io/gorm"
)

// BiliAccountService B站账号服务
type BiliAccountService struct {
	db *gorm.DB
}

// NewBiliAccountService 创建B站账号服务
func NewBiliAccountService(db *gorm.DB) *BiliAccountService {
	return &BiliAccountService{db: db}
}

// BindAccount 绑定B站账号到用户
func (s *BiliAccountService) BindAccount(userID uint, loginInfo *bilibili.LoginInfo, isPrimary bool) (*model.UserBiliAccount, error) {
	if loginInfo == nil || loginInfo.TokenInfo.Mid == 0 {
		return nil, errors.New("无效的登录信息")
	}

	// 序列化登录凭证
	cookies, err := json.Marshal(loginInfo)
	if err != nil {
		return nil, fmt.Errorf("序列化登录信息失败: %v", err)
	}

	// 检查是否已绑定该B站账号
	var existing model.UserBiliAccount
	err = s.db.Where("user_id = ? AND bili_mid = ?", userID, loginInfo.TokenInfo.Mid).First(&existing).Error
	if err == nil {
		// 已存在，更新凭证
		existing.Cookies = string(cookies)
		existing.BiliName = loginInfo.TokenInfo.Uname
		existing.BiliFace = loginInfo.TokenInfo.Face
		existing.IsEnabled = true
		now := time.Now()
		existing.ExpiresAt = &now
		existing.ExpiresAt = nil // 暂不设置过期时间，由刷新机制处理

		if err := s.db.Save(&existing).Error; err != nil {
			return nil, fmt.Errorf("更新账号失败: %v", err)
		}
		return &existing, nil
	}

	// 如果设置为主账号，先取消其他主账号
	if isPrimary {
		s.db.Model(&model.UserBiliAccount{}).
			Where("user_id = ? AND is_primary = ?", userID, true).
			Update("is_primary", false)
	}

	// 创建新绑定
	account := &model.UserBiliAccount{
		UserID:    userID,
		BiliMid:   loginInfo.TokenInfo.Mid,
		BiliName:  loginInfo.TokenInfo.Uname,
		BiliFace:  loginInfo.TokenInfo.Face,
		IsEnabled: true,
		IsPrimary: isPrimary,
		Cookies:   string(cookies),
	}

	if err := s.db.Create(account).Error; err != nil {
		return nil, fmt.Errorf("创建账号绑定失败: %v", err)
	}

	return account, nil
}

// GetUserAccounts 获取用户的所有B站账号
func (s *BiliAccountService) GetUserAccounts(userID uint) ([]model.UserBiliAccount, error) {
	var accounts []model.UserBiliAccount
	err := s.db.Where("user_id = ?", userID).Order("is_primary DESC, created_at ASC").Find(&accounts).Error
	return accounts, err
}

// GetAllEnabledAccounts 获取所有启用的B站账号（用于多账号上传）
func (s *BiliAccountService) GetAllEnabledAccounts() ([]model.UserBiliAccount, error) {
	var accounts []model.UserBiliAccount
	err := s.db.Where("is_enabled = ?", true).Order("is_primary DESC, created_at ASC").Find(&accounts).Error
	return accounts, err
}

// GetPrimaryAccount 获取用户的主B站账号
func (s *BiliAccountService) GetPrimaryAccount(userID uint) (*model.UserBiliAccount, error) {
	var account model.UserBiliAccount
	err := s.db.Where("user_id = ? AND is_primary = ? AND is_enabled = ?", userID, true, true).First(&account).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 没有主账号，尝试获取第一个启用的账号
			err = s.db.Where("user_id = ? AND is_enabled = ?", userID, true).First(&account).Error
		}
		if err != nil {
			return nil, err
		}
	}
	return &account, nil
}

// GetAccountByID 根据ID获取账号
func (s *BiliAccountService) GetAccountByID(accountID uint) (*model.UserBiliAccount, error) {
	var account model.UserBiliAccount
	err := s.db.First(&account, accountID).Error
	return &account, err
}

// GetLoginInfo 获取账号的登录信息
func (s *BiliAccountService) GetLoginInfo(account *model.UserBiliAccount) (*bilibili.LoginInfo, error) {
	if account.Cookies == "" {
		return nil, errors.New("账号凭证为空")
	}

	var loginInfo bilibili.LoginInfo
	if err := json.Unmarshal([]byte(account.Cookies), &loginInfo); err != nil {
		return nil, fmt.Errorf("解析登录信息失败: %v", err)
	}

	return &loginInfo, nil
}

// GetLoginInfoForUser 获取用户主账号的登录信息（便捷方法）
func (s *BiliAccountService) GetLoginInfoForUser(userID uint) (*bilibili.LoginInfo, *model.UserBiliAccount, error) {
	account, err := s.GetPrimaryAccount(userID)
	if err != nil {
		return nil, nil, fmt.Errorf("获取主账号失败: %v", err)
	}

	loginInfo, err := s.GetLoginInfo(account)
	if err != nil {
		return nil, nil, err
	}

	return loginInfo, account, nil
}

// SetPrimaryAccount 设置主账号
func (s *BiliAccountService) SetPrimaryAccount(userID uint, accountID uint) error {
	// 先取消所有主账号
	if err := s.db.Model(&model.UserBiliAccount{}).
		Where("user_id = ?", userID).
		Update("is_primary", false).Error; err != nil {
		return err
	}

	// 设置新的主账号
	return s.db.Model(&model.UserBiliAccount{}).
		Where("id = ? AND user_id = ?", accountID, userID).
		Update("is_primary", true).Error
}

// EnableAccount 启用账号（按用户隔离）
func (s *BiliAccountService) EnableAccount(userID uint, accountID uint) error {
	return s.db.Model(&model.UserBiliAccount{}).
		Where("id = ? AND user_id = ?", accountID, userID).
		Update("is_enabled", true).Error
}

// DisableAccount 禁用账号（按用户隔离）
func (s *BiliAccountService) DisableAccount(userID uint, accountID uint) error {
	return s.db.Model(&model.UserBiliAccount{}).
		Where("id = ? AND user_id = ?", accountID, userID).
		Update("is_enabled", false).Error
}

// UnbindAccount 解绑账号（通过数据库ID）
func (s *BiliAccountService) UnbindAccount(userID uint, accountID uint) error {
	return s.db.Where("id = ? AND user_id = ?", accountID, userID).Delete(&model.UserBiliAccount{}).Error
}

// UnbindAccountByMid 解绑账号（通过B站MID）
func (s *BiliAccountService) UnbindAccountByMid(userID uint, biliMid int64) error {
	return s.db.Where("bili_mid = ? AND user_id = ?", biliMid, userID).Delete(&model.UserBiliAccount{}).Error
}

// EnableAccountByMid 启用账号（通过B站MID）
func (s *BiliAccountService) EnableAccountByMid(userID uint, biliMid int64) error {
	return s.db.Model(&model.UserBiliAccount{}).
		Where("bili_mid = ? AND user_id = ?", biliMid, userID).
		Update("is_enabled", true).Error
}

// DisableAccountByMid 禁用账号（通过B站MID）
func (s *BiliAccountService) DisableAccountByMid(userID uint, biliMid int64) error {
	return s.db.Model(&model.UserBiliAccount{}).
		Where("bili_mid = ? AND user_id = ?", biliMid, userID).
		Update("is_enabled", false).Error
}

// SetPrimaryAccountByMid 设置主账号（通过B站MID）
func (s *BiliAccountService) SetPrimaryAccountByMid(userID uint, biliMid int64) error {
	// 先取消所有主账号
	if err := s.db.Model(&model.UserBiliAccount{}).
		Where("user_id = ? AND is_primary = ?", userID, true).
		Update("is_primary", false).Error; err != nil {
		return err
	}

	// 设置新的主账号
	return s.db.Model(&model.UserBiliAccount{}).
		Where("bili_mid = ? AND user_id = ?", biliMid, userID).
		Update("is_primary", true).Error
}

// UpdateLastUsed 更新最后使用时间
func (s *BiliAccountService) UpdateLastUsed(accountID uint) error {
	now := time.Now()
	return s.db.Model(&model.UserBiliAccount{}).
		Where("id = ?", accountID).
		Update("last_used_at", &now).Error
}

// MigrateFromLegacyStore 从旧的单账号存储迁移到新系统
// 用于兼容旧版本数据
func (s *BiliAccountService) MigrateFromLegacyStore(userID uint) error {
	// 检查用户是否已有绑定账号
	var count int64
	s.db.Model(&model.UserBiliAccount{}).Where("user_id = ?", userID).Count(&count)
	if count > 0 {
		return nil // 已有账号，无需迁移
	}

	// 尝试从旧存储加载
	legacyStore := storage.GetDefaultStore()
	if !legacyStore.IsValid() {
		return nil // 旧存储无有效数据
	}

	loginInfo, err := legacyStore.Load()
	if err != nil {
		return nil
	}

	// 迁移到新系统
	_, err = s.BindAccount(userID, loginInfo, true)
	return err
}

// GetGlobalLoginInfo 获取全局登录信息（兼容旧逻辑，用于无用户上下文的场景）
// 优先级: 数据库账号 > 多账号存储 > 旧单账号存储
func (s *BiliAccountService) GetGlobalLoginInfo() (*bilibili.LoginInfo, error) {
	log.Printf("[BiliAccountService] GetGlobalLoginInfo 开始...")

	// 1. 尝试从数据库获取任意一个启用的账号
	var account model.UserBiliAccount
	err := s.db.Where("is_enabled = ?", true).Order("is_primary DESC, last_used_at DESC").First(&account).Error
	if err == nil {
		log.Printf("[BiliAccountService] 从数据库找到账号: %s (MID: %d)", account.BiliName, account.BiliMid)
		return s.GetLoginInfo(&account)
	}
	log.Printf("[BiliAccountService] 数据库中没有启用的账号: %v", err)

	// 2. 尝试从多账号存储获取（前端绑定的账号）
	multiStore := storage.GetMultiAccountStore()
	log.Printf("[BiliAccountService] 多账号存储是否存在: %v", multiStore != nil)
	if multiStore != nil {
		// 获取主账号
		primaryAccount, err := multiStore.GetPrimaryAccount()
		log.Printf("[BiliAccountService] 获取主账号结果: err=%v, account=%v", err, primaryAccount != nil)
		if err == nil && primaryAccount != nil {
			log.Printf("[BiliAccountService] 主账号: Name=%s, IsEnabled=%v, HasLoginInfo=%v",
				primaryAccount.Name, primaryAccount.IsEnabled, primaryAccount.LoginInfo != nil)
			if primaryAccount.IsEnabled && primaryAccount.LoginInfo != nil {
				log.Printf("[BiliAccountService] 使用主账号: %s (MID: %d)", primaryAccount.Name, primaryAccount.Mid)
				return primaryAccount.LoginInfo, nil
			}
		}
		// 获取任意启用的账号
		accounts, _ := multiStore.GetAllAccounts()
		log.Printf("[BiliAccountService] 所有账号数量: %d", len(accounts))
		for i, acc := range accounts {
			log.Printf("[BiliAccountService] 账号[%d]: Name=%s, IsEnabled=%v, HasLoginInfo=%v",
				i, acc.Name, acc.IsEnabled, acc.LoginInfo != nil)
			if acc.IsEnabled && acc.LoginInfo != nil {
				log.Printf("[BiliAccountService] 使用账号: %s (MID: %d)", acc.Name, acc.Mid)
				return acc.LoginInfo, nil
			}
		}
	}

	// 3. 回退到旧单账号存储
	log.Printf("[BiliAccountService] 尝试旧单账号存储...")
	legacyStore := storage.GetDefaultStore()
	if legacyStore.IsValid() {
		log.Printf("[BiliAccountService] 旧存储有效，加载中...")
		return legacyStore.Load()
	}
	log.Printf("[BiliAccountService] 旧存储无效")

	return nil, errors.New("没有可用的B站登录信息")
}
