package store

import (
	"fmt"
	"time"

	"github.com/difyz9/ytb2bili/internal/core/types"
	"github.com/difyz9/ytb2bili/pkg/store/model"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// NewDatabase 创建数据库连接
func NewDatabase(config *types.AppConfig) (*gorm.DB, error) {
	// GORM配置
	gormConfig := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "cw_", // crypto_wallet prefix
			SingularTable: false,
		},
	}

	// 设置日志级别 - 只在出错时输出日志，避免控制台污染
	// Debug 模式下使用 Warn 级别，生产环境使用 Silent
	if config.Debug {
		gormConfig.Logger = logger.Default.LogMode(logger.Warn)
	} else {
		gormConfig.Logger = logger.Default.LogMode(logger.Silent)
	}

	// 根据数据库类型创建连接
	var db *gorm.DB
	var err error

	switch config.Database.Type {
	case "postgres", "postgresql":
		dsn := config.Database.GetDSN()
		db, err = gorm.Open(postgres.Open(dsn), gormConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
		}
	case "mysql":

		dsn := config.Database.GetDSN()
		db, err = gorm.Open(mysql.Open(dsn), gormConfig)

		if err != nil {
			return nil, fmt.Errorf("failed to connect to MySQL: %w", err)
		}
	case "sqlite", "sqlite3":
		dsn := config.Database.GetDSN()
		db, err = gorm.Open(sqlite.Open(dsn), gormConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to SQLite: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported database type: %s (supported: postgres, mysql, sqlite)", config.Database.Type)
	}

	// 获取底层的sql.DB对象
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// 设置连接池参数（SQLite 不需要连接池，但设置也不会有问题）
	if config.Database.Type != "sqlite" && config.Database.Type != "sqlite3" {
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)
	} else {
		// SQLite 使用单个连接
		sqlDB.SetMaxOpenConns(1)
	}

	return db, nil
}

// AutoMigrate 自动迁移数据库表
func AutoMigrate(db *gorm.DB) error {
	// 导入所有模型并执行迁移
	return db.AutoMigrate(
		&model.User{},
		&model.SavedVideo{},
		&model.App{},
		&model.UserToken{},
	)
}
