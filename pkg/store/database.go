package store

import (
	"fmt"
	"time"

	"github.com/difyz9/ytb2bili/internal/core/types"

	"github.com/glebarez/sqlite" // Pure-Go SQLite driver (no CGO required)
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
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
	// 调高慢查询阈值到2秒，避免频繁输出慢SQL警告
	if config.Debug {
		gormConfig.Logger = logger.Default.LogMode(logger.Error) // 只在错误时输出，不输出慢查询警告
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
		// 添加忙超时设置，避免 database is locked
		if config.Database.Type == "sqlite" || config.Database.Type == "sqlite3" {
			// 如果 DSN 不包含参数，添加之
			// 注意：这里简单追加，假设用户没有在 DSN 中写复杂的参数
			// 更稳健的方式是在 DSN 构建时处理，但这里为了修复 bug 先直接改
		}

		db, err = gorm.Open(sqlite.Open(dsn), gormConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to SQLite: %w", err)
		}

		// 启用 WAL 模式以支持更高并发
		db.Exec("PRAGMA journal_mode=WAL;")
		db.Exec("PRAGMA busy_timeout=5000;")
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
		// 最大打开连接数：根据细粒度锁的并发需求调整
		// 支持 max_concurrent_uploads（默认5）+ 常规查询 + 预留
		sqlDB.SetMaxOpenConns(100) // 支持高并发场景

		// 最大空闲连接数：保持 10% 的 MaxOpenConns
		// 平衡连接复用和资源占用
		sqlDB.SetMaxIdleConns(10)

		// 连接最大生命周期：1小时
		// 防止长期连接导致的数据库端问题（超时、连接泄漏等）
		sqlDB.SetConnMaxLifetime(time.Hour)

		// 连接最大空闲时间：10分钟（新增）
		// 空闲连接超过10分钟会被回收，释放资源
		// 与 MaxIdleConns 配合，避免连接池膨胀
		sqlDB.SetConnMaxIdleTime(10 * time.Minute)
	} else {
		// SQLite 在 WAL 模式下支持并发读
		// 设置为 1 会导致所有查询串行化，一个慢查询就会阻塞所有请求
		// 设置为 100 允许并发读取，写入时 sqlite 驱动会利用忙等待处理锁
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}

	return db, nil
}

// AutoMigrate 自动迁移数据库表
func AutoMigrate(db *gorm.DB) error {
	return MigrateDatabase(db)
}
