package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/difyz9/ytb2bili/pkg/crypto"
	"gorm.io/gorm"
)

// BackupService 数据库备份服务
type BackupService struct {
	db        *gorm.DB
	backupDir string
}

// NewBackupService 创建备份服务
func NewBackupService(db *gorm.DB, backupDir string) *BackupService {
	// 确保目录存在
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		log.Printf("⚠️ 无法创建备份目录: %v", err)
	}
	return &BackupService{
		db:        db,
		backupDir: backupDir,
	}
}

// CreateEncryptedBackup 创建加密的数据库备份
// 注意：这里我们只通过 SQLite 的 VACUUM INTO 命令来实现简单备份，
// 如果是其他数据库需要不同的实现。
// 由于 VACUUM INTO 产生的是标准 SQLite 文件，我们需要事后加密它。
func (s *BackupService) CreateEncryptedBackup() error {
	timestamp := time.Now().Format("20060102_150405")
	tempBackupFile := filepath.Join(s.backupDir, fmt.Sprintf("temp_backup_%s.db", timestamp))
	finalBackupFile := filepath.Join(s.backupDir, fmt.Sprintf("backup_%s.enc", timestamp))

	// 1. 生成临时未加密备份 (SQLite specific)
	// 使用 GORM 执行原生 SQL 来备份 SQLite
	// 注意：GORM 的 Exec 可能会受 sql.DB连接池影响，但 VACUUM INTO 是安全的
	if err := s.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", tempBackupFile)).Error; err != nil {
		return fmt.Errorf("生成数据库快照失败: %w", err)
	}
	defer os.Remove(tempBackupFile) // 确保清理临时文件

	// 2. 加密备份文件
	encSvc, err := crypto.GetEncryptionService()
	if err != nil {
		return fmt.Errorf("获取加密服务失败: %w", err)
	}

	// 读取原始文件
	data, err := os.ReadFile(tempBackupFile)
	if err != nil {
		return fmt.Errorf("读取临时备份文件失败: %w", err)
	}

	// 加密
	encryptedData, err := encSvc.Encrypt(data)
	if err != nil {
		return fmt.Errorf("加密备份数据失败: %w", err)
	}

	// 写入新文件
	if err := os.WriteFile(finalBackupFile, []byte(encryptedData), 0644); err != nil {
		return fmt.Errorf("写入加密备份文件失败: %w", err)
	}

	log.Printf("✅ 数据库已加密备份至: %s", finalBackupFile)

	// 3. 执行清理
	s.cleanupOldBackups(7) // 保留7天

	return nil
}

// cleanupOldBackups 清理旧备份
func (s *BackupService) cleanupOldBackups(retentionDays int) {
	files, err := os.ReadDir(s.backupDir)
	if err != nil {
		log.Printf("⚠️ 读取备份目录失败: %v", err)
		return
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(s.backupDir, file.Name())
			if err := os.Remove(path); err != nil {
				log.Printf("⚠️ 删除旧备份失败: %s, %v", file.Name(), err)
			} else {
				log.Printf("🧹 已清理旧备份: %s", file.Name())
			}
		}
	}
}
