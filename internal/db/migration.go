package db

import (
	"embed"
	"fmt"
	"sort"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

//go:embed *.sql
var migrationFiles embed.FS

// Migration 迁移记录
type Migration struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"size:255;not null;uniqueIndex"`
	Executed  bool   `gorm:"default:false"`
	CreatedAt string `gorm:"size:50"`
}

// RunMigrations 执行数据库迁移
func RunMigrations(db *gorm.DB, logger *zap.SugaredLogger) error {
	// 1. 创建迁移记录表
	if err := db.AutoMigrate(&Migration{}); err != nil {
		return fmt.Errorf("创建迁移表失败: %v", err)
	}

	// 2. 读取迁移文件
	files, err := migrationFiles.ReadDir(".")
	if err != nil {
		return fmt.Errorf("读取迁移文件失败: %v", err)
	}

	// 3. 过滤并排序迁移文件（只执行 *.sql 文件，排除 rollback）
	var migrations []string
	for _, file := range files {
		name := file.Name()
		if strings.HasSuffix(name, ".sql") && !strings.Contains(name, "rollback") {
			migrations = append(migrations, name)
		}
	}
	sort.Strings(migrations)

	// 4. 执行未执行的迁移
	for _, migrationName := range migrations {
		// 检查是否已执行
		var count int64
		db.Model(&Migration{}).Where("name = ?", migrationName).Count(&count)
		if count > 0 {
			logger.Infof("✓ 迁移已跳过: %s", migrationName)
			continue
		}

		// 读取SQL内容
		content, err := migrationFiles.ReadFile(migrationName)
		if err != nil {
			return fmt.Errorf("读取迁移文件 %s 失败: %v", migrationName, err)
		}

		// 执行SQL
		if err := db.Exec(string(content)).Error; err != nil {
			return fmt.Errorf("执行迁移 %s 失败: %v", migrationName, err)
		}

		// 记录迁移
		db.Create(&Migration{Name: migrationName, Executed: true})
		logger.Infof("✓ 迁移已执行: %s", migrationName)
	}

	logger.Infof("✅ 数据库迁移完成，共执行 %d 个迁移", len(migrations))
	return nil
}

// RollbackMigration 回滚指定迁移（需要手动执行回滚SQL）
func RollbackMigration(db *gorm.DB, migrationName string) error {
	// 读取回滚文件
	rollbackName := strings.Replace(migrationName, ".sql", "_rollback.sql", 1)
	content, err := migrationFiles.ReadFile(rollbackName)
	if err != nil {
		return fmt.Errorf("读取回滚文件 %s 失败: %v", rollbackName, err)
	}

	// 执行回滚SQL
	if err := db.Exec(string(content)).Error; err != nil {
		return fmt.Errorf("执行回滚失败: %v", err)
	}

	// 删除迁移记录
	db.Where("name = ?", migrationName).Delete(&Migration{})

	return nil
}

// GetMigrationStatus 获取迁移状态
func GetMigrationStatus(db *gorm.DB) ([]Migration, error) {
	var migrations []Migration
	if err := db.Find(&migrations).Error; err != nil {
		return nil, err
	}
	return migrations, nil
}
