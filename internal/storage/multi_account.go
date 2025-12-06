package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/difyz9/bilibili-go-sdk/bilibili"
)

// BiliAccount 单个B站账号信息
type BiliAccount struct {
	ID        string              `json:"id"`         // 账号唯一标识 (使用 Mid 字符串)
	Mid       int64               `json:"mid"`        // B站用户ID
	Name      string              `json:"name"`       // 用户名
	Face      string              `json:"face"`       // 头像URL
	IsEnabled bool                `json:"is_enabled"` // 是否启用
	IsPrimary bool                `json:"is_primary"` // 是否为主账号（用于上传）
	LoginInfo *bilibili.LoginInfo `json:"login_info"` // 登录凭证
	UserInfo  *UserBasicInfo      `json:"user_info"`  // 用户详细信息
	CreatedAt time.Time           `json:"created_at"` // 添加时间
	UpdatedAt time.Time           `json:"updated_at"` // 更新时间
	ExpiresAt time.Time           `json:"expires_at"` // 过期时间
}

// MultiAccountStore 多账号存储管理器
type MultiAccountStore struct {
	storePath string
	mu        sync.RWMutex
}

// MultiAccountData 多账号存储数据结构
type MultiAccountData struct {
	Version  int            `json:"version"`  // 数据版本
	Accounts []*BiliAccount `json:"accounts"` // 账号列表
}

var (
	multiAccountStore *MultiAccountStore
	multiAccountOnce  sync.Once
)

// GetMultiAccountStore 获取多账号存储器单例
func GetMultiAccountStore() *MultiAccountStore {
	multiAccountOnce.Do(func() {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Printf("Warning: Failed to get user home directory: %v", err)
			homeDir = "."
		}

		storePath := filepath.Join(homeDir, ".bili_up", "accounts.json")
		multiAccountStore = &MultiAccountStore{storePath: storePath}

		// 确保存储目录存在
		if err := os.MkdirAll(filepath.Dir(storePath), 0700); err != nil {
			log.Printf("Warning: Failed to create storage directory: %v", err)
		}

		// 尝试从旧的单账号存储迁移
		multiAccountStore.migrateFromSingleAccount()
	})
	return multiAccountStore
}

// migrateFromSingleAccount 从单账号存储迁移到多账号
func (s *MultiAccountStore) migrateFromSingleAccount() {
	// 检查是否已有多账号数据
	if _, err := os.Stat(s.storePath); err == nil {
		return // 已存在多账号数据，无需迁移
	}

	// 尝试加载旧的单账号数据
	oldStore := GetDefaultStore()
	loginInfo, userInfo, err := oldStore.LoadWithUserInfo()
	if err != nil {
		return // 没有旧数据
	}

	// 迁移到多账号格式
	account := &BiliAccount{
		ID:        fmt.Sprintf("%d", loginInfo.TokenInfo.Mid),
		Mid:       loginInfo.TokenInfo.Mid,
		Name:      loginInfo.TokenInfo.Uname,
		Face:      loginInfo.TokenInfo.Face,
		IsEnabled: true,
		IsPrimary: true,
		LoginInfo: loginInfo,
		UserInfo:  userInfo,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}

	if userInfo != nil {
		account.Name = userInfo.Name
		account.Face = userInfo.Face
	}

	data := &MultiAccountData{
		Version:  1,
		Accounts: []*BiliAccount{account},
	}

	if err := s.saveData(data); err != nil {
		log.Printf("Warning: Failed to migrate single account: %v", err)
		return
	}

	log.Printf("Successfully migrated single account to multi-account format (Mid: %d)", account.Mid)
}

// loadData 加载多账号数据
func (s *MultiAccountStore) loadData() (*MultiAccountData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, err := os.Stat(s.storePath); os.IsNotExist(err) {
		return &MultiAccountData{Version: 1, Accounts: []*BiliAccount{}}, nil
	}

	data, err := os.ReadFile(s.storePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read accounts data: %w", err)
	}

	var accountData MultiAccountData
	if err := json.Unmarshal(data, &accountData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal accounts data: %w", err)
	}

	return &accountData, nil
}

// saveData 保存多账号数据
func (s *MultiAccountStore) saveData(data *MultiAccountData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal accounts data: %w", err)
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(s.storePath), 0700); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// 原子写入
	tempPath := s.storePath + ".tmp"
	if err := os.WriteFile(tempPath, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write accounts data: %w", err)
	}

	if err := os.Rename(tempPath, s.storePath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to save accounts data: %w", err)
	}

	return nil
}

// GetAllAccounts 获取所有账号
func (s *MultiAccountStore) GetAllAccounts() ([]*BiliAccount, error) {
	data, err := s.loadData()
	if err != nil {
		return nil, err
	}

	// 过滤掉过期的账号（但不删除，只标记）
	for _, acc := range data.Accounts {
		if time.Now().After(acc.ExpiresAt) {
			acc.IsEnabled = false
		}
	}

	return data.Accounts, nil
}

// GetEnabledAccounts 获取所有启用的账号
func (s *MultiAccountStore) GetEnabledAccounts() ([]*BiliAccount, error) {
	accounts, err := s.GetAllAccounts()
	if err != nil {
		return nil, err
	}

	var enabled []*BiliAccount
	for _, acc := range accounts {
		if acc.IsEnabled && time.Now().Before(acc.ExpiresAt) {
			enabled = append(enabled, acc)
		}
	}

	return enabled, nil
}

// GetPrimaryAccount 获取主账号（用于上传）
func (s *MultiAccountStore) GetPrimaryAccount() (*BiliAccount, error) {
	accounts, err := s.GetEnabledAccounts()
	if err != nil {
		return nil, err
	}

	// 优先返回标记为主账号的
	for _, acc := range accounts {
		if acc.IsPrimary {
			return acc, nil
		}
	}

	// 如果没有主账号，返回第一个启用的账号
	if len(accounts) > 0 {
		return accounts[0], nil
	}

	return nil, fmt.Errorf("no enabled account found")
}

// GetAccountByMid 根据 Mid 获取账号
func (s *MultiAccountStore) GetAccountByMid(mid int64) (*BiliAccount, error) {
	accounts, err := s.GetAllAccounts()
	if err != nil {
		return nil, err
	}

	for _, acc := range accounts {
		if acc.Mid == mid {
			return acc, nil
		}
	}

	return nil, fmt.Errorf("account not found: %d", mid)
}

// AddAccount 添加新账号
func (s *MultiAccountStore) AddAccount(loginInfo *bilibili.LoginInfo, userInfo *UserBasicInfo) (*BiliAccount, error) {
	data, err := s.loadData()
	if err != nil {
		return nil, err
	}

	mid := loginInfo.TokenInfo.Mid

	// 检查是否已存在
	for i, acc := range data.Accounts {
		if acc.Mid == mid {
			// 更新现有账号
			data.Accounts[i].LoginInfo = loginInfo
			data.Accounts[i].UserInfo = userInfo
			data.Accounts[i].Name = loginInfo.TokenInfo.Uname
			data.Accounts[i].Face = loginInfo.TokenInfo.Face
			data.Accounts[i].UpdatedAt = time.Now()
			data.Accounts[i].ExpiresAt = time.Now().Add(30 * 24 * time.Hour)
			data.Accounts[i].IsEnabled = true

			if userInfo != nil {
				data.Accounts[i].Name = userInfo.Name
				data.Accounts[i].Face = userInfo.Face
			}

			if err := s.saveData(data); err != nil {
				return nil, err
			}

			log.Printf("Updated existing account (Mid: %d, Name: %s)", mid, data.Accounts[i].Name)
			return data.Accounts[i], nil
		}
	}

	// 创建新账号
	account := &BiliAccount{
		ID:        fmt.Sprintf("%d", mid),
		Mid:       mid,
		Name:      loginInfo.TokenInfo.Uname,
		Face:      loginInfo.TokenInfo.Face,
		IsEnabled: true,
		IsPrimary: len(data.Accounts) == 0, // 第一个账号设为主账号
		LoginInfo: loginInfo,
		UserInfo:  userInfo,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}

	if userInfo != nil {
		account.Name = userInfo.Name
		account.Face = userInfo.Face
	}

	data.Accounts = append(data.Accounts, account)

	if err := s.saveData(data); err != nil {
		return nil, err
	}

	log.Printf("Added new account (Mid: %d, Name: %s)", mid, account.Name)
	return account, nil
}

// RemoveAccount 删除账号
func (s *MultiAccountStore) RemoveAccount(mid int64) error {
	data, err := s.loadData()
	if err != nil {
		return err
	}

	var newAccounts []*BiliAccount
	var removedPrimary bool

	for _, acc := range data.Accounts {
		if acc.Mid != mid {
			newAccounts = append(newAccounts, acc)
		} else if acc.IsPrimary {
			removedPrimary = true
		}
	}

	if len(newAccounts) == len(data.Accounts) {
		return fmt.Errorf("account not found: %d", mid)
	}

	// 如果删除的是主账号，将第一个账号设为主账号
	if removedPrimary && len(newAccounts) > 0 {
		newAccounts[0].IsPrimary = true
	}

	data.Accounts = newAccounts

	if err := s.saveData(data); err != nil {
		return err
	}

	log.Printf("Removed account (Mid: %d)", mid)
	return nil
}

// SetAccountEnabled 设置账号启用状态
func (s *MultiAccountStore) SetAccountEnabled(mid int64, enabled bool) error {
	data, err := s.loadData()
	if err != nil {
		return err
	}

	for _, acc := range data.Accounts {
		if acc.Mid == mid {
			acc.IsEnabled = enabled
			acc.UpdatedAt = time.Now()

			if err := s.saveData(data); err != nil {
				return err
			}

			log.Printf("Set account enabled=%v (Mid: %d)", enabled, mid)
			return nil
		}
	}

	return fmt.Errorf("account not found: %d", mid)
}

// SetPrimaryAccount 设置主账号
func (s *MultiAccountStore) SetPrimaryAccount(mid int64) error {
	data, err := s.loadData()
	if err != nil {
		return err
	}

	found := false
	for _, acc := range data.Accounts {
		if acc.Mid == mid {
			acc.IsPrimary = true
			acc.IsEnabled = true // 主账号必须启用
			acc.UpdatedAt = time.Now()
			found = true
		} else {
			acc.IsPrimary = false
		}
	}

	if !found {
		return fmt.Errorf("account not found: %d", mid)
	}

	if err := s.saveData(data); err != nil {
		return err
	}

	log.Printf("Set primary account (Mid: %d)", mid)
	return nil
}

// GetAccountCount 获取账号数量
func (s *MultiAccountStore) GetAccountCount() (int, error) {
	accounts, err := s.GetAllAccounts()
	if err != nil {
		return 0, err
	}
	return len(accounts), nil
}
