-- 创建邮箱验证码表
CREATE TABLE IF NOT EXISTS cw_email_verifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email VARCHAR(100) NOT NULL,
    code VARCHAR(10) NOT NULL,
    expires_at DATETIME NOT NULL,
    used BOOLEAN DEFAULT 0,
    type VARCHAR(20) DEFAULT 'register',
    attempt_count INTEGER DEFAULT 0,
    last_attempt_at DATETIME,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME
);

-- 创建索引以提高查询性能
CREATE INDEX IF NOT EXISTS idx_email_verifications_email ON cw_email_verifications(email);
CREATE INDEX IF NOT EXISTS idx_email_verifications_email_type ON cw_email_verifications(email, type);

-- 注释：
-- type 字段值说明:
--   - register: 注册验证
--   - login: 登录验证
--   - reset_password: 重置密码验证
-- 验证码默认 10 分钟有效期
-- 同一邮箱的旧验证码在发送新验证码时会标记为已使用
