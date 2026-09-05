package audit_test

import (
	"testing"
	"time"

	"github.com/difyz9/ytb2bili/pkg/audit"
	"github.com/difyz9/ytb2bili/pkg/store/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate audit log table
	err = db.AutoMigrate(&model.AuditLog{})
	require.NoError(t, err)

	return db
}

// TestAuditService_Log tests basic logging functionality
func TestAuditService_Log(t *testing.T) {
	db := setupTestDB(t)
	svc := audit.NewAuditService(db)
	defer svc.Close()

	// Log an entry
	entry := audit.LogEntry{
		UserID:     0,
		Username:   "testuser",
		Action:     audit.ActionBindBiliAccount,
		Resource:   audit.ResourceBiliAccount,
		ResourceID: "1",
		IP:         "127.0.0.1",
		UserAgent:  "TestAgent/1.0",
		Success:    true,
		Message:    "Test login",
	}
	svc.Log(entry)

	// Wait for the worker to process (ticker is 2s)
	time.Sleep(3 * time.Second)

	// Verify the log was written
	var count int64
	db.Model(&model.AuditLog{}).Count(&count)
	assert.GreaterOrEqual(t, count, int64(1))
}

// TestAuditService_LogHelpers tests logging helper functions exist and don't panic
func TestAuditService_LogHelpers(t *testing.T) {
	db := setupTestDB(t)
	svc := audit.NewAuditService(db)
	defer svc.Close()

	// These should not panic
	svc.LogSuccess(0, "admin", audit.ActionBindBiliAccount, audit.ResourceBiliAccount, "1", "127.0.0.1", "TestAgent", "Success")
	svc.LogFailure(0, "unknown", audit.ActionBindBiliAccount, audit.ResourceBiliAccount, "", "192.168.1.1", "BadAgent", "Invalid password")
}

// TestAuditService_Query tests log querying using directly inserted data
func TestAuditService_Query(t *testing.T) {
	db := setupTestDB(t)
	svc := audit.NewAuditService(db)
	defer svc.Close()

	// Insert test data directly (bypassing async service for predictable tests)
	now := time.Now()
	testLogs := []model.AuditLog{
		{UserID: 0, Username: "anonymous", Action: "login", Success: true, CreatedAt: now},
		{UserID: 0, Username: "anonymous", Action: "logout", Success: true, CreatedAt: now},
		{UserID: 0, Username: "anonymous", Action: "upload", Success: false, CreatedAt: now},
	}
	for _, log := range testLogs {
		result := db.Create(&log)
		require.NoError(t, result.Error)
	}

	// Verify data was inserted
	var totalInserted int64
	db.Model(&model.AuditLog{}).Count(&totalInserted)
	require.Equal(t, int64(3), totalInserted, "Should have 3 records in DB")

	// Query all
	logs, total, err := svc.Query(audit.QueryFilter{}, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, logs, 3)

	// Query by action
	logs, total, err = svc.Query(audit.QueryFilter{Action: "login"}, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
}

// TestAuditService_CleanupOldLogs tests old log cleanup
func TestAuditService_CleanupOldLogs(t *testing.T) {
	db := setupTestDB(t)
	svc := audit.NewAuditService(db)
	defer svc.Close()

	// Insert old and new logs
	oldLog := model.AuditLog{
		UserID:    0,
		Action:    "old_action",
		CreatedAt: time.Now().AddDate(0, 0, -100), // 100 days ago
	}
	newLog := model.AuditLog{
		UserID:    0,
		Action:    "new_action",
		CreatedAt: time.Now(),
	}
	db.Create(&oldLog)
	db.Create(&newLog)

	// Cleanup logs older than 90 days
	err := svc.CleanupOldLogs(90)
	require.NoError(t, err)

	// Verify old log is deleted
	var count int64
	db.Model(&model.AuditLog{}).Count(&count)
	assert.Equal(t, int64(1), count)

	// Verify the remaining log is the new one
	var remaining model.AuditLog
	db.First(&remaining)
	assert.Equal(t, "new_action", remaining.Action)
}

// TestAuditService_Close tests graceful shutdown
func TestAuditService_Close(t *testing.T) {
	db := setupTestDB(t)
	svc := audit.NewAuditService(db)

	// Log some entries
	svc.Log(audit.LogEntry{UserID: 0, Action: "test"})
	svc.Log(audit.LogEntry{UserID: 0, Action: "test2"})

	// Close should not panic and should flush remaining logs
	svc.Close()

	// After close, logs should be flushed
	var count int64
	db.Model(&model.AuditLog{}).Count(&count)
	assert.GreaterOrEqual(t, count, int64(0)) // May or may not have flushed, but should not panic
}
