package migration

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// AddRoleToUsers 在 cw_users 表中添加 role 字段
func AddRoleToUsers(db *gorm.DB) error {
	fmt.Println("═══════════════════════════════════════")
	fmt.Println("📦 数据库迁移: 添加用户角色字段")
	fmt.Println("═══════════════════════════════════════")
	fmt.Println()

	// 获取数据库类型
	dialectorName := db.Dialector.Name()

	// 检查字段是否已存在
	var count int64
	var checkSQL string

	switch dialectorName {
	case "sqlite":
		checkSQL = `SELECT COUNT(*) FROM pragma_table_info('cw_users') WHERE name = 'role'`
	case "mysql":
		checkSQL = `SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'cw_users' AND column_name = 'role' AND table_schema = DATABASE()`
	case "postgres":
		checkSQL = `SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'cw_users' AND column_name = 'role'`
	default:
		// 尝试通用方式
		checkSQL = `SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'cw_users' AND column_name = 'role'`
	}

	if err := db.Raw(checkSQL).Scan(&count).Error; err != nil {
		return fmt.Errorf("检查字段失败: %w", err)
	}

	if count > 0 {
		fmt.Println("✅ role 字段已存在，跳过迁移")
		return nil
	}

	// 添加字段
	fmt.Println("📝 添加 role 字段...")
	var alterSQL string

	switch dialectorName {
	case "sqlite":
		alterSQL = `ALTER TABLE cw_users ADD COLUMN role VARCHAR(20) DEFAULT 'user'`
	case "mysql":
		alterSQL = `ALTER TABLE cw_users ADD COLUMN role VARCHAR(20) DEFAULT 'user' COMMENT '用户角色: admin/user'`
	case "postgres":
		alterSQL = `ALTER TABLE cw_users ADD COLUMN role VARCHAR(20) DEFAULT 'user'`
	default:
		alterSQL = `ALTER TABLE cw_users ADD COLUMN role VARCHAR(20) DEFAULT 'user'`
	}

	if err := db.Exec(alterSQL).Error; err != nil {
		return fmt.Errorf("添加字段失败: %w", err)
	}

	// 创建索引（SQLite 在 ALTER TABLE 时不自动创建索引）
	fmt.Println("📝 创建索引...")
	indexSQL := `CREATE INDEX IF NOT EXISTS idx_users_role ON cw_users(role)`

	if dialectorName == "mysql" {
		// MySQL 使用不同的语法
		// 先检查索引是否存在
		var indexCount int64
		db.Raw(`SELECT COUNT(*) FROM information_schema.statistics WHERE table_name = 'cw_users' AND index_name = 'idx_users_role'`).Scan(&indexCount)
		if indexCount == 0 {
			indexSQL = `CREATE INDEX idx_users_role ON cw_users(role)`
			if err := db.Exec(indexSQL).Error; err != nil {
				fmt.Printf("⚠️ 创建索引失败（可忽略）: %v\n", err)
			}
		}
	} else {
		if err := db.Exec(indexSQL).Error; err != nil {
			fmt.Printf("⚠️ 创建索引失败（可忽略）: %v\n", err)
		}
	}

	// 将第一个用户设为管理员
	fmt.Println("👑 设置第一个用户为管理员...")
	var userID uint
	firstUserSQL := `SELECT id FROM cw_users ORDER BY id ASC LIMIT 1`

	if err := db.Raw(firstUserSQL).Scan(&userID).Error; err != nil || userID == 0 {
		fmt.Println("⚠️  没有找到用户，跳过管理员设置")
	} else {
		updateSQL := `UPDATE cw_users SET role = 'admin' WHERE id = ?`
		if err := db.Exec(updateSQL, userID).Error; err != nil {
			return fmt.Errorf("设置管理员失败: %w", err)
		}
		fmt.Printf("✅ 用户 ID=%d 已设为管理员\n", userID)
	}

	fmt.Println()
	fmt.Println("✅ 迁移完成!")
	fmt.Println("═══════════════════════════════════════")
	fmt.Println()

	return nil
}

// MigrateAll 执行所有迁移
func MigrateAll(db *gorm.DB) error {
	fmt.Println("🚀 开始数据库迁移...")
	fmt.Println()

	// 记录开始时间
	startTime := time.Now()

	// 执行迁移
	if err := AddRoleToUsers(db); err != nil {
		return fmt.Errorf("添加角色字段失败: %w", err)
	}

	duration := time.Since(startTime)
	fmt.Printf("⏱️  迁移完成，耗时: %v\n", duration)
	fmt.Println()

	return nil
}
