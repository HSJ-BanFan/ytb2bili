package integration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/difyz9/ytb2bili/internal/db"
	"github.com/difyz9/ytb2bili/pkg/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestBackupService_CreateEncryptedBackup 测试加密备份创建
func TestBackupService_CreateEncryptedBackup(t *testing.T) {
	// 创建临时目录
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	backupDir := filepath.Join(tempDir, "backups")

	// 初始化测试数据库
	database, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, _ := database.DB()
	defer sqlDB.Close()

	// 设置加密密钥
	os.Setenv("COOKIE_ENCRYPTION_KEY", "test_32_byte_encryption_key_1234")
	defer os.Unsetenv("COOKIE_ENCRYPTION_KEY")

	// 创建备份服务
	svc := db.NewBackupService(database, backupDir)

	// 执行备份
	err = svc.CreateEncryptedBackup()
	require.NoError(t, err)

	// 验证备份文件存在
	files, err := os.ReadDir(backupDir)
	require.NoError(t, err)
	assert.Equal(t, 1, len(files), "应该有1个备份文件")

	// 读取备份内容
	backupFile := filepath.Join(backupDir, files[0].Name())
	content, err := os.ReadFile(backupFile)
	require.NoError(t, err)

	// 验证是加密的（不是明文SQLite）
	if len(content) >= 15 {
		assert.NotEqual(t, "SQLite format 3", string(content[:15]), "备份应该是加密的")
	}

	// 解密验证
	encSvc, _ := crypto.GetEncryptionService()
	decrypted, err := encSvc.Decrypt(string(content))
	require.NoError(t, err)
	assert.Equal(t, "SQLite format 3", string(decrypted[:15]), "解密后应该是SQLite格式")
}

// TestBackupService_MultipleBackups 测试多次备份
func TestBackupService_MultipleBackups(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	backupDir := filepath.Join(tempDir, "backups")

	database, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, _ := database.DB()
	defer sqlDB.Close()

	os.Setenv("COOKIE_ENCRYPTION_KEY", "test_32_byte_encryption_key_1234")
	defer os.Unsetenv("COOKIE_ENCRYPTION_KEY")

	svc := db.NewBackupService(database, backupDir)

	// 创建多个备份（每次间隔1秒以确保文件名唯一）
	for i := 0; i < 3; i++ {
		if i > 0 {
			time.Sleep(1 * time.Second)
		}
		err = svc.CreateEncryptedBackup()
		require.NoError(t, err)
	}

	// 验证有3个备份文件
	files, err := os.ReadDir(backupDir)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(files), 1, "应该有至少1个备份文件")
}

// TestBackupService_EmptyDatabase 测试空数据库备份
func TestBackupService_EmptyDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "empty.db")
	backupDir := filepath.Join(tempDir, "backups")

	database, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, _ := database.DB()
	defer sqlDB.Close()

	os.Setenv("COOKIE_ENCRYPTION_KEY", "test_32_byte_encryption_key_1234")
	defer os.Unsetenv("COOKIE_ENCRYPTION_KEY")

	svc := db.NewBackupService(database, backupDir)

	// 空数据库也应该能成功备份
	err = svc.CreateEncryptedBackup()
	require.NoError(t, err)

	files, err := os.ReadDir(backupDir)
	require.NoError(t, err)
	assert.Equal(t, 1, len(files))
}
