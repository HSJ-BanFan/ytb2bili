-- 数据库迁移: 添加 B站账号加密字段
-- 版本: 006
-- 日期: 2026-01-01
-- 描述: 为 cw_user_bili_accounts 表添加加密存储字段

-- 添加加密字段
ALTER TABLE cw_user_bili_accounts ADD COLUMN IF NOT EXISTS cookies_encrypted TEXT;
ALTER TABLE cw_user_bili_accounts ADD COLUMN IF NOT EXISTS access_token_encrypted TEXT;
ALTER TABLE cw_user_bili_accounts ADD COLUMN IF NOT EXISTS refresh_token_encrypted TEXT;
ALTER TABLE cw_user_bili_accounts ADD COLUMN IF NOT EXISTS encryption_version INT DEFAULT 0;

-- 说明：
-- encryption_version = 0: 明文存储（旧数据）
-- encryption_version = 2: AES-256-GCM 加密存储

-- 注意：
-- 1. 旧数据的明文字段（cookies, access_token, refresh_token）暂时保留，用于兼容
-- 2. 应用启动时会自动将明文数据迁移到加密字段
-- 3. 迁移完成后，可以手动清空明文字段：
--    UPDATE cw_user_bili_accounts SET cookies = '', access_token = '', refresh_token = '' WHERE encryption_version = 2;
