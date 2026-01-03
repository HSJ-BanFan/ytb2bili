package store

import (
	"database/sql"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestDatabaseConnectionPoolConfig 测试数据库连接池配置
// 验证连接池配置是否适合细粒度锁的并发场景
func TestDatabaseConnectionPoolConfig(t *testing.T) {
	t.Log("========================================")
	t.Log("数据库连接池配置验证")
	t.Log("========================================")

	// 测试配置
	testConfigs := []struct {
		name            string
		maxOpenConns    int
		maxIdleConns    int
		connMaxLifetime time.Duration
		connMaxIdleTime time.Duration
	}{
		{
			name:            "当前配置",
			maxOpenConns:    100,
			maxIdleConns:    10,
			connMaxLifetime: time.Hour,
			connMaxIdleTime: 10 * time.Minute,
		},
		{
			name:            "小规模（推荐单机）",
			maxOpenConns:    25,
			maxIdleConns:    5,
			connMaxLifetime: time.Hour,
			connMaxIdleTime: 10 * time.Minute,
		},
		{
			name:            "中等规模（推荐生产）",
			maxOpenConns:    50,
			maxIdleConns:    10,
			connMaxLifetime: time.Hour,
			connMaxIdleTime: 10 * time.Minute,
		},
		{
			name:            "大规模（高并发）",
			maxOpenConns:    200,
			maxIdleConns:    20,
			connMaxLifetime: 30 * time.Minute,
			connMaxIdleTime: 5 * time.Minute,
		},
	}

	for _, tc := range testConfigs {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("配置：")
			t.Logf("  MaxOpenConns: %d", tc.maxOpenConns)
			t.Logf("  MaxIdleConns: %d", tc.maxIdleConns)
			t.Logf("  ConnMaxLifetime: %v", tc.connMaxLifetime)
			t.Logf("  ConnMaxIdleTime: %v", tc.connMaxIdleTime)

			// 评估配置
			evaluateConfig(t, tc)
		})
	}

	t.Log("========================================")
}

// evaluateConfig 评估连接池配置是否合理
func evaluateConfig(t *testing.T, config struct {
	name            string
	maxOpenConns    int
	maxIdleConns    int
	connMaxLifetime time.Duration
	connMaxIdleTime time.Duration
}) {
	t.Logf("  评估结果：")

	// 检查1: MaxOpenConns
	if config.maxOpenConns < 10 {
		t.Logf("    ⚠️  MaxOpenConns(%d) 偏低，可能成为瓶颈", config.maxOpenConns)
	} else if config.maxOpenConns > 200 {
		t.Logf("    ⚠️  MaxOpenConns(%d) 偏高，可能浪费资源", config.maxOpenConns)
	} else {
		t.Logf("    ✅ MaxOpenConns(%d) 合理", config.maxOpenConns)
	}

	// 检查2: MaxIdleConns 应该是 MaxOpenConns 的 10% 左右
	idleRatio := float64(config.maxIdleConns) / float64(config.maxOpenConns) * 100
	if idleRatio < 5 {
		t.Logf("    ⚠️  MaxIdleConns 偏低 (%.1f%%)，建议提高到 10%%", idleRatio)
	} else if idleRatio > 20 {
		t.Logf("    ⚠️  MaxIdleConns 偏高 (%.1f%%)，建议降低到 10%%", idleRatio)
	} else {
		t.Logf("    ✅ MaxIdleConns 比例合理 (%.1f%%)", idleRatio)
	}

	// 检查3: ConnMaxLifetime
	if config.connMaxLifetime < 30*time.Minute {
		t.Logf("    ⚠️  ConnMaxLifetime 偏短，可能导致频繁重连")
	} else if config.connMaxLifetime > 2*time.Hour {
		t.Logf("    ⚠️  ConnMaxLifetime 偏长，可能导致连接问题")
	} else {
		t.Logf("    ✅ ConnMaxLifetime 合理")
	}

	// 检查4: ConnMaxIdleTime
	if config.connMaxIdleTime == 0 {
		t.Logf("    ⚠️  未设置 ConnMaxIdleTime，可能导致连接泄漏")
	} else if config.connMaxIdleTime > 30*time.Minute {
		t.Logf("    ⚠️  ConnMaxIdleTime 偏长，空闲连接回收慢")
	} else {
		t.Logf("    ✅ ConnMaxIdleTime 合理")
	}

	// 检查5: 是否适合细粒度锁场景
	concurrentUploads := 5                  // 默认并发上传数
	requiredConns := concurrentUploads + 10 // 预留10个连接用于常规查询

	if config.maxOpenConns < requiredConns {
		t.Logf("    ⚠️  MaxOpenConns(%d) 可能不足以支持 %d 个并发上传", config.maxOpenConns, concurrentUploads)
		t.Logf("    建议：至少 %d 个连接", requiredConns)
	} else {
		t.Logf("    ✅ 连接数足以支持细粒度锁（%d个并发上传）", concurrentUploads)
	}
}

// TestDatabaseConnectionPoolSimulation 模拟并发场景下的连接池使用
func TestDatabaseConnectionPoolSimulation(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过模拟测试（使用 -short 跳过）")
	}

	t.Log("========================================")
	t.Log("连接池并发场景模拟")
	t.Log("========================================")

	scenarios := []struct {
		name              string
		concurrentQueries int
		maxOpenConns      int
		expectedBehavior  string
	}{
		{
			name:              "当前配置 - 5个并发查询",
			concurrentQueries: 5,
			maxOpenConns:      100,
			expectedBehavior:  "连接充足",
		},
		{
			name:              "当前配置 - 50个并发查询",
			concurrentQueries: 50,
			maxOpenConns:      100,
			expectedBehavior:  "连接充足，但接近上限",
		},
		{
			name:              "当前配置 - 150个并发查询",
			concurrentQueries: 150,
			maxOpenConns:      100,
			expectedBehavior:  "连接不足，需要等待",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Logf("场景：")
			t.Logf("  并发查询数：%d", scenario.concurrentQueries)
			t.Logf("  最大连接数：%d", scenario.maxOpenConns)

			// 模拟连接获取
			availableConns := scenario.maxOpenConns
			waitingQueries := 0

			if scenario.concurrentQueries <= availableConns {
				t.Logf("  预期：%s", scenario.expectedBehavior)
				t.Logf("  ✅ 所有查询可以立即执行")
			} else {
				waitingQueries = scenario.concurrentQueries - availableConns
				t.Logf("  预期：%s", scenario.expectedBehavior)
				t.Logf("  ⚠️  %d 个查询需要等待连接释放", waitingQueries)
			}
		})
	}

	t.Log("========================================")
}

// TestDatabaseConnectionPool_RealWorldTest 真实场景测试
// 使用 SQLite 测试连接池行为
func TestDatabaseConnectionPool_RealWorldTest(t *testing.T) {
	t.Log("========================================")
	t.Log("真实场景连接池测试")
	t.Log("========================================")

	// 创建临时 SQLite 数据库
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// 获取底层 sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("Failed to get sql.DB: %v", err)
	}

	// 设置连接池参数
	sqlDB.SetMaxOpenConns(1) // SQLite 只支持单连接
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	// 验证配置
	stats := sqlDB.Stats()
	t.Logf("SQLite 连接池状态：")
	t.Logf("  MaxOpenConnections: %d", stats.MaxOpenConnections)
	t.Logf("  OpenConnections: %d", stats.OpenConnections)
	t.Logf("  InUse: %d", stats.InUse)
	t.Logf("  Idle: %d", stats.Idle)
	t.Logf("  WaitCount: %d", stats.WaitCount)
	t.Logf("  WaitDuration: %v", stats.WaitDuration)

	// 模拟并发查询
	t.Run("并发查询测试", func(t *testing.T) {
		var wg sync.WaitGroup
		successCount := int32(0)
		errorCount := int32(0)

		// 启动 10 个并发查询
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// 执行查询
				var result int
				if err := db.Raw("SELECT 1").Scan(&result).Error; err != nil {
					t.Logf("    Query %d failed: %v", id, err)
					atomic.AddInt32(&errorCount, 1)
				} else {
					atomic.AddInt32(&successCount, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("  查询结果：")
		t.Logf("    成功：%d", successCount)
		t.Logf("    失败：%d", errorCount)
		t.Logf("    成功率：%.1f%%", float64(successCount)/10*100)

		// 检查连接池状态
		stats = sqlDB.Stats()
		t.Logf("  最终连接池状态：")
		t.Logf("    WaitCount: %d (等待连接的次数)", stats.WaitCount)

		if stats.WaitCount > 0 {
			t.Logf("    ⚠️  有 %d 次连接等待，说明并发度超过了连接池容量", stats.WaitCount)
		} else {
			t.Logf("    ✅ 没有连接等待，连接池配置充足")
		}
	})

	// 清理
	db.Exec("DROP TABLE IF EXISTS test_table")
	sqlDB.Close()

	t.Log("========================================")
}

// BenchmarkDatabaseConnectionPool 基准测试：连接获取性能
func BenchmarkDatabaseConnectionPool(b *testing.B) {
	// 创建测试数据库
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		b.Fatalf("Failed to connect to database: %v", err)
	}

	// 获取底层 sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		b.Fatalf("Failed to get sql.DB: %v", err)
	}
	defer sqlDB.Close()

	// 预热连接池
	for i := 0; i < 10; i++ {
		db.Exec("SELECT 1")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result int
		db.Raw("SELECT 1").Scan(&result)
	}
}

// TestDatabaseConnectionPool_HealthCheck 连接池健康检查指南
func TestDatabaseConnectionPool_HealthCheck(t *testing.T) {
	t.Log("========================================")
	t.Log("连接池健康检查指南")
	t.Log("========================================")

	checks := []struct {
		name     string
		command  string
		expected string
	}{
		{
			name:     "1. 查看当前连接池状态",
			command:  "SHOW PROCESSLIST",
			expected: "查看活跃连接数",
		},
		{
			name:     "2. 检查连接池配置",
			command:  "SHOW VARIABLES LIKE 'max_connections'",
			expected: "查看数据库最大连接数",
		},
		{
			name:     "3. 监控连接池使用率",
			command:  "SELECT COUNT(*) FROM information_schema.processlist",
			expected: "统计当前连接数",
		},
	}

	for _, check := range checks {
		t.Logf("")
		t.Logf("【%s】", check.name)
		t.Logf("  命令：%s", check.command)
		t.Logf("  说明：%s", check.expected)
	}

	t.Log("")
	t.Log("========================================")
	t.Log("调优建议")
	t.Log("========================================")

	tunings := []struct {
		problem        string
		solution       string
		implementation string
	}{
		{
			problem:        "连接池耗尽（WaitCount 很高）",
			solution:       "增加 MaxOpenConns",
			implementation: "sqlDB.SetMaxOpenConns(200)",
		},
		{
			problem:        "空闲连接过多（Idle 很高）",
			solution:       "降低 MaxIdleConns 或减少 ConnMaxIdleTime",
			implementation: "sqlDB.SetMaxIdleConns(5)",
		},
		{
			problem:        "频繁重连",
			solution:       "增加 ConnMaxLifetime",
			implementation: "sqlDB.SetConnMaxLifetime(2 * time.Hour)",
		},
		{
			problem:        "连接泄漏（连接一直不释放）",
			solution:       "设置 ConnMaxIdleTime",
			implementation: "sqlDB.SetConnMaxIdleTime(10 * time.Minute)",
		},
	}

	for _, tuning := range tunings {
		t.Logf("")
		t.Logf("问题：%s", tuning.problem)
		t.Logf("解决方案：%s", tuning.solution)
		t.Logf("实现：%s", tuning.implementation)
	}

	t.Log("")
	t.Log("========================================")
}

// getPoolUsageRate 计算连接池使用率（辅助函数）
func getPoolUsageRate(stats sql.DBStats) float64 {
	if stats.MaxOpenConnections <= 0 {
		return 0
	}
	return float64(stats.OpenConnections) / float64(stats.MaxOpenConnections) * 100
}
