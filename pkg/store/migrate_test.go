package store

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRemoveLegacyAccountSchema(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("PRAGMA foreign_keys = ON").Error)

	for _, statement := range []string{
		"CREATE TABLE cw_users (id INTEGER PRIMARY KEY)",
		"CREATE TABLE cw_apps (id INTEGER PRIMARY KEY, owner_id INTEGER, CONSTRAINT fk_cw_apps_owner FOREIGN KEY(owner_id) REFERENCES cw_users(id))",
		"CREATE TABLE cw_user_tokens (id INTEGER PRIMARY KEY, user_id INTEGER, CONSTRAINT fk_cw_user_tokens_user FOREIGN KEY(user_id) REFERENCES cw_users(id))",
		"CREATE TABLE cw_email_verifications (id INTEGER PRIMARY KEY, email TEXT)",
		"CREATE TABLE cw_saved_videos (id INTEGER, user_id INTEGER)",
		"CREATE TABLE cw_user_bili_accounts (id INTEGER, user_id INTEGER, CONSTRAINT fk_cw_user_bili_accounts_user FOREIGN KEY(user_id) REFERENCES cw_users(id))",
		"CREATE INDEX idx_saved_user ON cw_saved_videos(user_id)",
		"CREATE TABLE cw_user_preferences (id INTEGER, user_id INTEGER, email_notifications_enabled BOOLEAN, notification_email TEXT)",
	} {
		require.NoError(t, db.Exec(statement).Error)
	}

	require.NoError(t, removeLegacyAccountSchema(db))
	require.False(t, db.Migrator().HasColumn("cw_saved_videos", "user_id"))
	require.False(t, db.Migrator().HasColumn("cw_user_bili_accounts", "user_id"))
	require.False(t, db.Migrator().HasColumn("cw_user_preferences", "user_id"))
	require.False(t, db.Migrator().HasColumn("cw_user_preferences", "notification_email"))
	require.False(t, db.Migrator().HasTable("cw_users"))
	require.False(t, db.Migrator().HasTable("cw_apps"))
	require.False(t, db.Migrator().HasTable("cw_user_tokens"))
	require.False(t, db.Migrator().HasTable("cw_email_verifications"))
}
