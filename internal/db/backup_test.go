package db_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/difyz9/ytb2bili/internal/db"
	"github.com/difyz9/ytb2bili/pkg/crypto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEncryptedBackup(t *testing.T) {
	// 1. 设置测试环境
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	backupDir := filepath.Join(tempDir, "backups")

	// 初始化测试数据库
	database, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	// Close DB connection after test to allow temp dir cleanup
	sqlDB, _ := database.DB()
	defer sqlDB.Close()

	// 初始化加密服务 (Mock key)
	os.Setenv("COOKIE_ENCRYPTION_KEY", "12345678901234567890123456789012")
	defer os.Unsetenv("COOKIE_ENCRYPTION_KEY")

	// 重新初始化单例以确保使用新的 Key
	// 注意：在实际测试中可能需要更优雅的方式重置 crypto 单例，这里假设它是无状态或可重新获取的
	// 如果 crypto 包是单例且不可重置，可能需要调整代码结构。
	// 假设 crypto.GetEncryptionService() 会读取环境变量。

	svc := db.NewBackupService(database, backupDir)

	// 2. 执行备份
	err = svc.CreateEncryptedBackup()
	if err != nil {
		t.Fatalf("CreateEncryptedBackup failed: %v", err)
	}

	// 3. 验证备份文件是否存在
	files, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("Expected 1 backup file, found %d", len(files))
	}

	backupFile := filepath.Join(backupDir, files[0].Name())

	// 4. 验证文件内容 (应该是加密的，不是 SQLite 头部)
	content, err := os.ReadFile(backupFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	// SQLite header is usually "SQLite format 3"
	if string(content[:15]) == "SQLite format 3" {
		t.Errorf("Backup file appears to be plaintext SQLite!")
	}

	// 5. 尝试解密验证
	encSvc, _ := crypto.GetEncryptionService()
	// Fix: Cast content to string
	decrypted, err := encSvc.Decrypt(string(content))
	if err != nil {
		t.Errorf("Failed to decrypt backup: %v", err)
	}

	if string(decrypted[:15]) != "SQLite format 3" {
		t.Errorf("Decrypted content does not match SQLite header")
	}
}

func TestBackupCleanup(t *testing.T) {
	// 此处可添加清理逻辑的测试，模拟旧文件被删除
}
