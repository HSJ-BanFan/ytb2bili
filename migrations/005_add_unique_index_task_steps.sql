-- 修复 cw_task_steps 表重复数据问题
-- 创建唯一索引防止同一视频的同一步骤重复插入

-- 1. 删除已存在的重复数据（保留最早的一条）
DELETE FROM cw_task_steps
WHERE id NOT IN (
    SELECT MIN(id) FROM cw_task_steps
    GROUP BY video_id, step_name
);

-- 2. 删除旧的普通索引（如果存在）
DROP INDEX IF EXISTS idx_video_step;

-- 3. 创建唯一索引防止重复
CREATE UNIQUE INDEX idx_video_step ON cw_task_steps(video_id, step_name);

-- 注释：
-- 该唯一索引确保 (video_id, step_name) 组合唯一
-- 防止任务步骤重复插入导致的脏数据
-- 如果代码尝试插入重复记录，数据库将直接报错
