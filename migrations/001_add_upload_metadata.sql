-- =====================================================
-- 迁移 001: 添加上传元数据字段
-- =====================================================

-- 1. 新增字段（upload_* 为实际上传使用的数据）
ALTER TABLE cw_saved_videos
  ADD COLUMN upload_title VARCHAR(500) COMMENT '实际上传到B站的标题',
  ADD COLUMN upload_desc TEXT COMMENT '实际上传到B站的描述',
  ADD COLUMN upload_tags VARCHAR(1000) COMMENT '实际上传到B站的标签（逗号分隔）',
  ADD COLUMN metadata_source VARCHAR(20) DEFAULT 'original' COMMENT '元数据来源: original/ai_generated/user_edited',
  ADD COLUMN metadata_edit_status VARCHAR(20) DEFAULT 'auto' COMMENT '编辑状态: auto/pending_review/edited';

-- 2. 为现有数据迁移（根据配置和现有字段决定 upload_*）
-- 注意：这里假设默认使用原始数据，AI数据需要显式配置才使用
UPDATE cw_saved_videos
SET
  upload_title = COALESCE(
    NULLIF(generated_title, ''),
    title
  ),
  upload_desc = COALESCE(
    NULLIF(generated_desc, ''),
    description
  ),
  upload_tags = COALESCE(
    NULLIF(generated_tags, ''),
    NULL
  ),
  metadata_source = CASE
    WHEN generated_title IS NOT NULL AND generated_title != '' THEN 'ai_generated'
    ELSE 'original'
  END
WHERE upload_title IS NULL;

-- 3. 添加索引（优化查询）
CREATE INDEX idx_metadata_source ON cw_saved_videos(metadata_source);
CREATE INDEX idx_metadata_edit_status ON cw_saved_videos(metadata_edit_status);

-- 4. 添加注释
ALTER TABLE cw_saved_videos COMMENT = '视频存储表 - 包含原始、AI生成和最终上传的元数据';
