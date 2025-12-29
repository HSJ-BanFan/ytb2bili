-- 添加邮箱验证状态字段
ALTER TABLE cw_users ADD COLUMN email_verified BOOLEAN DEFAULT FALSE;
ALTER TABLE cw_users ADD COLUMN email_verified_at DATETIME;

-- 创建索引以提高查询性能
CREATE INDEX idx_users_email_verified ON cw_users(email_verified);

-- 注释：
-- email_verified: 用户邮箱是否已验证
-- email_verified_at: 邮箱验证时间
-- 注册用户通过验证码验证后会自动标记为已验证
