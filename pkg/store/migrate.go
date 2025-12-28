package store

import (
	"fmt"

	"github.com/difyz9/ytb2bili/pkg/store/model"
	"gorm.io/gorm"
)

// MigrateDatabase 自动迁移数据库表
func MigrateDatabase(db *gorm.DB) error {
	// 运行自定义迁移（添加 role 字段等）
	if err := runCustomMigrations(db); err != nil {
		return err
	}

	// GORM 自动迁移（创建表和添加新字段）
	return db.AutoMigrate(
		&model.User{},
		&model.SavedVideo{},
		&model.TaskStep{},
		&model.App{},
		&model.UserToken{},
		&model.UserBiliAccount{},
		&model.UserAIConfig{},   // 用户AI配置
		&model.UserPreference{}, // 用户偏好设置
	)
}

// runCustomMigrations 运行需要手动处理的迁移
func runCustomMigrations(db *gorm.DB) error {
	// 添加 role 字段到 users 表（如果不存在）
	return addRoleFieldIfNotExists(db)
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
