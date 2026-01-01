package integration

import (
	"testing"
	"time"

	"github.com/difyz9/ytb2bili/pkg/audit"
	"github.com/difyz9/ytb2bili/pkg/store/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuditLog_Integration 测试审计日志完整流程
func TestAuditLog_Integration(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	// 清空现有日志
	app.ClearAuditLogs()

	// 记录一条审计日志
	app.AuditSvc.LogSuccess(
		1,
		"test_user",
		audit.ActionUserLogin,
		audit.ResourceUser,
		"1",
		"127.0.0.1",
		"TestAgent/1.0",
		"测试登录成功",
	)

	// 等待异步写入
	time.Sleep(3 * time.Second)

	// 验证日志已写入
	var count int64
	app.DB.Model(&model.AuditLog{}).Count(&count)
	assert.GreaterOrEqual(t, count, int64(1), "应至少有1条日志")

	// 查询日志
	var log model.AuditLog
	err := app.DB.Where("action = ?", audit.ActionUserLogin).First(&log).Error
	require.NoError(t, err)
	assert.Equal(t, uint(1), log.UserID)
	assert.Equal(t, "test_user", log.Username)
	assert.True(t, log.Success)
}

// TestAuditLog_FailureLogging 测试失败操作日志查询
func TestAuditLog_FailureLogging(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	app.ClearAuditLogs()

	// 使用原始SQL插入以避免gorm默认值问题
	app.DB.Exec(`INSERT INTO cw_audit_logs (user_id, username, action, resource, ip, user_agent, success, message, created_at) 
		VALUES (0, 'unknown', 'user_login', 'user', '192.168.1.1', 'BadAgent', 0, '密码错误', datetime('now'))`)

	// 验证
	var log model.AuditLog
	err := app.DB.First(&log).Error
	require.NoError(t, err)
	assert.False(t, log.Success)
	assert.Equal(t, "密码错误", log.Message)
}

// TestAuditLog_Query 测试审计日志查询
func TestAuditLog_Query(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	app.ClearAuditLogs()

	// 直接插入测试数据（绕过异步）
	testLogs := []model.AuditLog{
		{UserID: 1, Username: "user1", Action: "login", Success: true, CreatedAt: time.Now()},
		{UserID: 2, Username: "user2", Action: "logout", Success: true, CreatedAt: time.Now()},
		{UserID: 1, Username: "user1", Action: "upload", Success: false, CreatedAt: time.Now()},
	}
	for _, log := range testLogs {
		app.DB.Create(&log)
	}

	// 查询所有
	logs, total, err := app.AuditSvc.Query(audit.QueryFilter{}, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, logs, 3)

	// 按用户ID查询
	logs, total, err = app.AuditSvc.Query(audit.QueryFilter{UserID: 1}, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)

	// 按操作查询
	logs, total, err = app.AuditSvc.Query(audit.QueryFilter{Action: "login"}, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
}

// TestAuditLog_Cleanup 测试日志清理
func TestAuditLog_Cleanup(t *testing.T) {
	app := SetupTestApp(t)
	defer app.Cleanup()

	app.ClearAuditLogs()

	// 插入一条旧日志和一条新日志
	oldLog := model.AuditLog{
		UserID:    1,
		Action:    "old_action",
		CreatedAt: time.Now().AddDate(0, 0, -100),
	}
	newLog := model.AuditLog{
		UserID:    2,
		Action:    "new_action",
		CreatedAt: time.Now(),
	}
	app.DB.Create(&oldLog)
	app.DB.Create(&newLog)

	// 清理90天前的日志
	err := app.AuditSvc.CleanupOldLogs(90)
	require.NoError(t, err)

	// 验证旧日志已删除
	var count int64
	app.DB.Model(&model.AuditLog{}).Count(&count)
	assert.Equal(t, int64(1), count)

	// 验证保留的是新日志
	var remaining model.AuditLog
	app.DB.First(&remaining)
	assert.Equal(t, "new_action", remaining.Action)
}
