package store

import (
	"fmt"
	"log"

	db_migration "github.com/difyz9/ytb2bili/internal/db"
	"github.com/difyz9/ytb2bili/pkg/crypto"
	"github.com/difyz9/ytb2bili/pkg/store/model"
	"gorm.io/gorm"
)

// MigrateDatabase 自动迁移数据库表
func MigrateDatabase(db *gorm.DB) error {
	// 1. 先运行 AutoMigrate 创建表
	if err := db.AutoMigrate(
		&model.User{},
		&model.SavedVideo{},
		&model.TaskStep{},
		&model.App{},
		&model.UserToken{},
		&model.UserBiliAccount{},
		&model.UserAIConfig{},      // 用户AI配置
		&model.UserPreference{},    // 用户偏好设置
		&model.EmailVerification{}, // 邮箱验证码
		&model.AuditLog{},          // 审计日志
	); err != nil {
		return err
	}

	// 2. 运行自定义迁移（添加 role 字段等）
	// 注意：由于 AutoMigrate 已经创建了表结构（包括 role 字段），这里主要用于数据修正和索引创建
	if err := runCustomMigrations(db); err != nil {
		return err
	}

	// 3. 迁移数据库中的明文 Cookies 到加密存储
	if err := migrateEncryptedCookies(db); err != nil {
		log.Printf("⚠️  Cookies 加密迁移失败: %v", err)
		// 不返回错误，允许应用继续启动（新数据会使用加密）
	}

	// 4. 初始化种子数据（如初始用户）
	return seedInitialData(db)
}

// seedInitialData 初始化种子数据
func seedInitialData(db *gorm.DB) error {
	// 检查是否有用户
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		return fmt.Errorf("检查用户数量失败: %w", err)
	}

	if count == 0 {
		fmt.Println("🌟 数据库为空，正在创建初始管理员用户...")
		user := model.User{
			Username:      "Admin",
			Email:         "3330876408@qq.com",
			Password:      "$2a$10$vIysWJwYXHJECRrret5pAeuwpzjwzVeXDDLPbWJrzng7Xx6oRS6sK", // 123456
			Role:          "admin",
			Status:        1,
			EmailVerified: true,
		}
		if err := db.Create(&user).Error; err != nil {
			return fmt.Errorf("创建初始用户失败: %w", err)
		}
		fmt.Println("✅ 初始管理员创建成功: 3330876408@qq.com / 123456")
	}
	return nil
}

// runCustomMigrations 运行需要手动处理的迁移
func runCustomMigrations(db *gorm.DB) error {
	// 添加 role 字段到 users 表（如果不存在）
	if err := addRoleFieldIfNotExists(db); err != nil {
		return err
	}

	// 添加审计日志索引
	return addAuditIndexes(db)
}

// addAuditIndexes 添加审计日志表的索引
func addAuditIndexes(db *gorm.DB) error {
	// 定义索引结构
	indexes := []struct {
		name   string
		column string
	}{
		{"idx_audit_logs_user_created", "(user_id, created_at)"},
		{"idx_audit_logs_action_created", "(action, created_at)"},
		{"idx_audit_logs_resource_id", "(resource, resource_id)"},
		{"idx_audit_logs_success_created", "(success, created_at)"},
	}

	for _, idx := range indexes {
		// MySQL 不支持 CREATE INDEX IF NOT EXISTS，需要先检查索引是否存在
		// 使用 GORM 的 Migrator 或手动检查
		var count int64
		checkSQL := fmt.Sprintf(
			"SELECT COUNT(*) FROM information_schema.statistics "+
				"WHERE table_schema = DATABASE() AND table_name = 'cw_audit_logs' AND index_name = '%s'",
			idx.name,
		)

		if err := db.Raw(checkSQL).Count(&count).Error; err != nil {
			fmt.Printf("⚠️ 检查索引 %s 失败: %v\n", idx.name, err)
			continue
		}

		// 如果索引已存在，跳过
		if count > 0 {
			fmt.Printf("✅ 索引 %s 已存在，跳过\n", idx.name)
			continue
		}

		// 创建索引（不使用 IF NOT EXISTS）
		createSQL := fmt.Sprintf("CREATE INDEX %s ON cw_audit_logs %s", idx.name, idx.column)
		if err := db.Exec(createSQL).Error; err != nil {
			fmt.Printf("⚠️ 创建索引失败: %v (SQL: %s)\n", err, createSQL)
		} else {
			fmt.Printf("✅ 成功创建索引: %s\n", idx.name)
		}
	}
	return nil
}

// addRoleFieldIfNotExists 添加 role 字段到 users 表（如果不存在）
func addRoleFieldIfNotExists(db *gorm.DB) error {
	// 检查字段是否已存在
	var columnExists bool
	checkSQL := `
		SELECT COUNT(*) as count
		FROM information_schema.columns
		WHERE table_name = 'cw_users'
		AND column_name = 'role'
		AND table_schema = DATABASE()
	`

	if db.Dialector.Name() == "sqlite" {
		checkSQL = `
			SELECT COUNT(*) as count
			FROM pragma_table_info('cw_users')
			WHERE name = 'role'
		`
	}

	result := db.Raw(checkSQL).Scan(&columnExists)
	if result.Error != nil {
		return fmt.Errorf("检查 role 字段失败: %w", result.Error)
	}

	if columnExists {
		// 字段已存在，检查是否需要设置管理员
		var adminCount int64
		if err := db.Table("cw_users").Where("role = ? OR role IS NULL", "admin").Count(&adminCount).Error; err != nil {
			return fmt.Errorf("检查管理员数量失败: %w", err)
		}

		// 检查是否有空角色需要填充默认值
		var emptyRoleCount int64
		if err := db.Table("cw_users").Where("role = ? OR role IS NULL", "").Count(&emptyRoleCount).Error; err == nil && emptyRoleCount > 0 {
			fmt.Printf("📝 发现 %d 个用户的 role 字段为空，填充默认值 'user'...\n", emptyRoleCount)
			db.Table("cw_users").Where("role = ? OR role IS NULL", "").Update("role", "user")
		}

		if adminCount == 0 {
			// 如果没有管理员，将第一个用户设为管理员
			var userID uint
			if err := db.Table("cw_users").Order("id ASC").Limit(1).Select("id").Scan(&userID).Error; err != nil {
				return fmt.Errorf("查询第一个用户失败: %w", err)
			}

			if userID > 0 {
				if err := db.Table("cw_users").Where("id = ?", userID).Update("role", "admin").Error; err != nil {
					return fmt.Errorf("设置管理员失败: %w", err)
				}
				fmt.Printf("✅ 用户 ID=%d 已设为管理员\n", userID)
			}
		} else {
			fmt.Printf("✅ 已有 %d 个管理员，跳过管理员设置\n", adminCount)
		}
		return nil
	}

	// 添加字段
	fmt.Println("📝 添加 role 字段到 cw_users 表...")
	alterSQL := `
		ALTER TABLE cw_users
		ADD COLUMN role VARCHAR(20) DEFAULT 'user'
		COMMENT '用户角色: admin/user'
	`

	if db.Dialector.Name() == "sqlite" {
		alterSQL = `
			ALTER TABLE cw_users
			ADD COLUMN role VARCHAR(20) DEFAULT 'user'
		`
	}

	if err := db.Exec(alterSQL).Error; err != nil {
		return fmt.Errorf("添加 role 字段失败: %w", err)
	}

	// 创建索引
	fmt.Println("📝 创建 role 索引...")
	if db.Dialector.Name() != "sqlite" {
		indexSQL := `CREATE INDEX idx_users_role ON cw_users(role)`
		if err := db.Exec(indexSQL).Error; err != nil {
			return fmt.Errorf("创建 role 索引失败: %w", err)
		}
	} else {
		fmt.Println("✅ SQLite 索引已自动创建")
	}

	// 将第一个用户设为管理员
	var userID uint
	if err := db.Order("id ASC").Limit(1).Select("id").Scan(&userID).Error; err == nil && userID > 0 {
		db.Table("cw_users").Where("id = ?", userID).Update("role", "admin")
		fmt.Printf("✅ 用户 ID=%d 已设为管理员\n", userID)
	}

	fmt.Println("✅ role 字段迁移完成!")
	return nil
}

// migrateEncryptedCookies 迁移数据库中的明文 Cookies 到加密存储
func migrateEncryptedCookies(db *gorm.DB) error {
	// 检查加密服务是否可用
	_, err := crypto.GetEncryptionService()
	if err != nil {
		log.Printf("⚠️  加密服务未初始化，跳过 Cookies 加密迁移")
		return nil // 不返回错误，允许应用继续启动
	}

	// 执行迁移
	return db_migration.MigrateDatabaseCookies(db)
}
