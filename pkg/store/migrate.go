package store

import (
	"fmt"
	"log"

	db_migration "github.com/difyz9/ytb2bili/internal/db"
	"github.com/difyz9/ytb2bili/pkg/crypto"
	"github.com/difyz9/ytb2bili/pkg/store/model"
	"gorm.io/gorm"
)

// MigrateDatabase creates the tables used by the standalone tool and removes
// schema pieces that belonged to the retired account system.
func MigrateDatabase(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.SavedVideo{},
		&model.TaskStep{},
		&model.UserBiliAccount{},
		&model.ToolAIConfig{},
		&model.ToolPreference{},
		&model.AuditLog{},
	); err != nil {
		return err
	}

	if err := removeLegacyAccountSchema(db); err != nil {
		return err
	}
	if err := migrateEncryptedCookies(db); err != nil {
		log.Printf("⚠️ Cookies 加密迁移失败: %v", err)
	}
	return nil
}

// removeLegacyAccountSchema makes old installations global without touching
// video or credential data. Unused legacy tables are dropped because no
// current model or route can read them.
func removeLegacyAccountSchema(db *gorm.DB) error {
	for _, table := range []string{
		"cw_saved_videos",
		"cw_user_bili_accounts",
		"cw_user_ai_configs",
		"cw_user_preferences",
	} {
		if !db.Migrator().HasTable(table) {
			continue
		}
		if db.Migrator().HasColumn(table, "user_id") {
			if err := dropLegacyColumn(db, table, "user_id"); err != nil {
				return fmt.Errorf("drop legacy user_id from %s: %w", table, err)
			}
		}
		for _, column := range []string{"email_notifications_enabled", "notification_email"} {
			if table == "cw_user_preferences" && db.Migrator().HasColumn(table, column) {
				if err := dropLegacyColumn(db, table, column); err != nil {
					return fmt.Errorf("drop legacy %s from %s: %w", column, table, err)
				}
			}
		}
	}

	// Drop dependents before cw_users so foreign keys work on MySQL/PostgreSQL too.
	for _, table := range []string{
		"cw_user_tokens",
		"cw_apps",
		"cw_email_verifications",
		"tb_oauth_token",
		"tb_upload",
		"tb_course",
		"tb_channel",
		"tb_user",
		"cw_users",
	} {
		if db.Migrator().HasTable(table) {
			if err := db.Migrator().DropTable(table); err != nil {
				return fmt.Errorf("drop legacy table %s: %w", table, err)
			}
		}
	}
	return nil
}

type legacyUser struct {
	ID uint
}
type legacyBiliAccount struct {
	UserID uint
	User   legacyUser `gorm:"foreignKey:UserID"`
}

func (legacyBiliAccount) TableName() string {
	return "cw_user_bili_accounts"
}

func dropLegacyColumn(db *gorm.DB, table, column string) error {
	if table == "cw_user_bili_accounts" && db.Table(table).Migrator().HasConstraint(&legacyBiliAccount{}, "User") {
		if err := db.Table(table).Migrator().DropConstraint(&legacyBiliAccount{}, "User"); err != nil {
			return err
		}
	}

	indexes, err := db.Migrator().GetIndexes(table)
	if err != nil {
		return err
	}
	for _, index := range indexes {
		for _, indexedColumn := range index.Columns() {
			if indexedColumn == column {
				if err := db.Migrator().DropIndex(table, index.Name()); err != nil {
					return err
				}
				break
			}
		}
	}
	return db.Exec(fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, column)).Error
}

func migrateEncryptedCookies(db *gorm.DB) error {
	if _, err := crypto.GetEncryptionService(); err != nil {
		log.Printf("⚠️ 加密服务未初始化，跳过 Cookies 加密迁移")
		return nil
	}
	return db_migration.MigrateDatabaseCookies(db)
}
