-- 数据库迁移: 添加审计日志表
-- 版本: 007
-- 日期: 2026-01-01
-- 描述: 创建 cw_audit_logs 表记录核心操作审计日志

CREATE TABLE IF NOT EXISTS cw_audit_logs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- 操作者信息
    user_id INT UNSIGNED DEFAULT 0,
    username VARCHAR(100),
    
    -- 操作信息
    action VARCHAR(50) NOT NULL,
    resource VARCHAR(100),
    resource_id VARCHAR(100),
    
    -- 请求信息
    ip VARCHAR(45),
    user_agent VARCHAR(500),
    
    -- 结果信息
    success BOOLEAN DEFAULT TRUE,
    message VARCHAR(500),
    
    -- 扩展信息
    details TEXT,
    
    -- 索引
    INDEX idx_created_at (created_at),
    INDEX idx_user_id (user_id),
    INDEX idx_action (action)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 说明：
-- action 可能的值:
--   bind_bili_account: 绑定B站账号
--   unbind_bili_account: 解绑B站账号
--   upload_video: 上传视频
--   upload_subtitle: 上传字幕
