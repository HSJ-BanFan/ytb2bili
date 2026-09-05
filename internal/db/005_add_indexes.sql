-- 数据库复合索引优化
-- 用于提升查询性能，特别是在 UploadScheduler 和任务查询场景
-- 创建时间: 2025-12-30
-- 
-- ⚠️ 注意事项:
-- 1. 所有索引都使用 IF NOT EXISTS 确保幂等性
-- 2. 执行后请用 EXPLAIN 验证索引是否生效
-- 3. 大表场景下，索引创建可能会锁表，建议在低峰期执行


-- 2. 状态+创建时间 复合索引
-- 用途: 按状态查询并按时间排序（如 UploadScheduler 查询待上传视频）
-- 场景: uploadNextVideo(), uploadNextSubtitle()
-- 验证: EXPLAIN SELECT * FROM cw_saved_videos WHERE status = '200' ORDER BY created_at ASC LIMIT 1;
CREATE INDEX IF NOT EXISTS idx_status_created ON cw_saved_videos(status, created_at);

-- 3. 视频+步骤名 唯一索引（业务约束）
-- 用途: 确保每个视频的每个步骤只有一条记录（防止重复数据）
-- 场景: TaskStepService 查询和更新特定步骤
-- 验证: EXPLAIN SELECT * FROM cw_task_steps WHERE video_id = 'xxx' AND step_name = '下载视频';
CREATE UNIQUE INDEX IF NOT EXISTS idx_video_step ON cw_task_steps(video_id, step_name);

-- 4. 状态+处理完成时间 复合索引（延迟上传优化）
-- 用途: 延迟上传模式查询 processing_completed_at 字段
-- 场景: uploadNextVideo() 的延迟模式查询
-- 验证: EXPLAIN SELECT * FROM cw_saved_videos WHERE status = '200' AND processing_completed_at <= NOW();
CREATE INDEX IF NOT EXISTS idx_status_processing ON cw_saved_videos(status, processing_completed_at);

-- 5. 状态+字幕计划时间 复合索引（字幕上传优化）
-- 用途: 字幕上传调度查询
-- 场景: uploadNextSubtitle() 查询待上传字幕
-- 验证: EXPLAIN SELECT * FROM cw_saved_videos WHERE status = '300' AND subtitle_scheduled_at <= NOW();
CREATE INDEX IF NOT EXISTS idx_status_subtitle ON cw_saved_videos(status, subtitle_scheduled_at);
