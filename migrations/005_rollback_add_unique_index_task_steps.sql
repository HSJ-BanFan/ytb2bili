-- 回滚：删除唯一索引，恢复为普通索引

-- 1. 删除唯一索引
DROP INDEX IF EXISTS idx_video_step;

-- 2. 创建普通索引（允许重复）
CREATE INDEX idx_video_step ON cw_task_steps(video_id, step_name);

-- 注意：回滚后可能再次产生重复数据，仅用于紧急情况
