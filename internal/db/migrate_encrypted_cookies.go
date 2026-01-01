package db

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/difyz9/bilibili-go-sdk/bilibili"
	"github.com/difyz9/ytb2bili/pkg/crypto"
	"github.com/difyz9/ytb2bili/pkg/store/model"
	"gorm.io/gorm"
)

// MigrateDatabaseCookies 迁移数据库中的明文 Cookies 到加密存储
// 此函数应该在应用启动后执行一次，用于将旧数据迁移到新的加密格式
func MigrateDatabaseCookies(db *gorm.DB) error {
	log.Println("🔍 检查数据库中是否有明文账号凭证...")

	// 查找所有明文账号（encryption_version = 0 且 cookies 不为空）
	var accounts []model.UserBiliAccount
	err := db.Where("encryption_version = 0 AND cookies IS NOT NULL AND cookies != ''").Find(&accounts).Error
	if err != nil {
		return fmt.Errorf("查询账号失败: %w", err)
	}

	if len(accounts) == 0 {
		log.Println("✅ 没有需要迁移的明文数据")
		return nil
	}

	log.Printf("🔐 找到 %d 个明文账号，开始迁移...", len(accounts))

	// 初始化加密服务
	encSvc, err := crypto.GetEncryptionService()
	if err != nil {
		return fmt.Errorf("加密服务未初始化，无法迁移: %w", err)
	}

	successCount := 0
	failedCount := 0
	startTime := time.Now()

	for i, account := range accounts {
		log.Printf("[%d/%d] 迁移账号 #%d (Mid: %d, Name: %s)",
			i+1, len(accounts), account.ID, account.BiliMid, account.BiliName)

		// 验证 JSON 格式
		var loginInfo bilibili.LoginInfo
		if err := json.Unmarshal([]byte(account.Cookies), &loginInfo); err != nil {
			log.Printf("⚠️  账号 #%d 的 Cookies 格式无效，跳过: %v", account.ID, err)
			failedCount++
			continue
		}

		// 加密 cookies
		encrypted, err := encSvc.EncryptString(account.Cookies)
		if err != nil {
			log.Printf("⚠️  加密账号 #%d 失败: %v", account.ID, err)
			failedCount++
			continue
		}

		// 使用事务更新数据库
		tx := db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// 更新数据库
		err = tx.Model(&account).Updates(map[string]interface{}{
			"cookies_encrypted": encrypted,
			"cookies":           "", // 清空明文
			"encryption_version": 2,
		}).Error

		if err != nil {
			tx.Rollback()
			log.Printf("⚠️  更新账号 #%d 失败: %v", account.ID, err)
			failedCount++
			continue
		}

		if err := tx.Commit().Error; err != nil {
			log.Printf("⚠️  提交账号 #%d 事务失败: %v", account.ID, err)
			failedCount++
			continue
		}

		successCount++
		log.Printf("✅ 账号 #%d 迁移成功", account.ID)
	}

	duration := time.Since(startTime)
	log.Println("═════════════════════════════════════════════════════════")
	log.Println("✅ 数据库迁移完成")
	log.Printf("📊 成功: %d, 失败: %d, 总计: %d", successCount, failedCount, len(accounts))
	log.Printf("⏱️  耗时: %v", duration)
	log.Println("═════════════════════════════════════════════════════════")

	if failedCount > 0 {
		log.Printf("⚠️  有 %d 个账号迁移失败，请检查日志", failedCount)
	}

	return nil
}

// MigrateDatabaseCookiesDryRun 预演迁移（不实际修改数据）
// 用于测试迁移逻辑，查看有多少数据需要迁移
func MigrateDatabaseCookiesDryRun(db *gorm.DB) error {
	log.Println("🔍 预演数据库迁移...")

	var count int64
	err := db.Model(&model.UserBiliAccount{}).
		Where("encryption_version = 0 AND cookies IS NOT NULL AND cookies != ''").
		Count(&count).Error
	if err != nil {
		return fmt.Errorf("查询账号数量失败: %w", err)
	}

	if count == 0 {
		log.Println("✅ 没有需要迁移的明文数据")
		return nil
	}

	log.Printf("📊 找到 %d 个明文账号需要迁移", count)

	// 显示前10个账号的详情
	var accounts []model.UserBiliAccount
	err = db.Where("encryption_version = 0 AND cookies IS NOT NULL AND cookies != ''").
		Limit(10).
		Find(&accounts).Error
	if err != nil {
		return fmt.Errorf("查询账号详情失败: %w", err)
	}

	log.Println("示例账号（前10个）:")
	for i, account := range accounts {
		log.Printf("  %d. ID=%d, Mid=%d, Name=%s, CreatedAt=%s",
			i+1, account.ID, account.BiliMid, account.BiliName, account.CreatedAt.Format("2006-01-02"))
	}

	if count > 10 {
		log.Printf("  ... 还有 %d 个账号", count-10)
	}

	return nil
}

// RollbackDatabaseMigration 回滚加密迁移（恢复明文数据）
// ⚠️ 警告：此函数仅用于紧急恢复，使用后密钥文件必须妥善保管
func RollbackDatabaseMigration(db *gorm.DB) error {
	log.Println("⚠️  准备回滚数据库加密迁移...")
	log.Println("⚠️  这将恢复所有账号的明文 Cookies，请确保密钥文件已备份！")

	// 查找所有加密账号
	var accounts []model.UserBiliAccount
	err := db.Where("encryption_version = 2 AND cookies_encrypted IS NOT NULL AND cookies_encrypted != ''").
		Find(&accounts).Error
	if err != nil {
		return fmt.Errorf("查询账号失败: %w", err)
	}

	if len(accounts) == 0 {
		log.Println("✅ 没有需要回滚的加密数据")
		return nil
	}

	log.Printf("🔓 找到 %d 个加密账号，准备回滚...", len(accounts))

	// 初始化加密服务
	encSvc, err := crypto.GetEncryptionService()
	if err != nil {
		return fmt.Errorf("加密服务未初始化，无法解密: %w", err)
	}

	successCount := 0
	failedCount := 0

	for i, account := range accounts {
		log.Printf("[%d/%d] 回滚账号 #%d (Mid: %d, Name: %s)",
			i+1, len(accounts), account.ID, account.BiliMid, account.BiliName)

		// 解密 cookies
		decrypted, err := encSvc.DecryptString(account.CookiesEncrypted)
		if err != nil {
			log.Printf("⚠️  解密账号 #%d 失败: %v", account.ID, err)
			failedCount++
			continue
		}

		// 验证 JSON 格式
		var loginInfo bilibili.LoginInfo
		if err := json.Unmarshal([]byte(decrypted), &loginInfo); err != nil {
			log.Printf("⚠️  账号 #%d 的解密数据格式无效，跳过: %v", account.ID, err)
			failedCount++
			continue
		}

		// 使用事务更新数据库
		tx := db.Begin()

		// 更新数据库
		err = tx.Model(&account).Updates(map[string]interface{}{
			"cookies":            decrypted,
			"cookies_encrypted":  "", // 清空密文
			"encryption_version": 0,
		}).Error

		if err != nil {
			tx.Rollback()
			log.Printf("⚠️  更新账号 #%d 失败: %v", account.ID, err)
			failedCount++
			continue
		}

		if err := tx.Commit().Error; err != nil {
			log.Printf("⚠️  提交账号 #%d 事务失败: %v", account.ID, err)
			failedCount++
			continue
		}

		successCount++
		log.Printf("✅ 账号 #%d 回滚成功", account.ID)
	}

	log.Println("═════════════════════════════════════════════════════════")
	log.Println("✅ 数据库回滚完成")
	log.Printf("📊 成功: %d, 失败: %d, 总计: %d", successCount, failedCount, len(accounts))
	log.Println("═════════════════════════════════════════════════════════")

	if failedCount > 0 {
		log.Printf("⚠️  有 %d 个账号回滚失败，请检查日志", failedCount)
	}

	return nil
}
