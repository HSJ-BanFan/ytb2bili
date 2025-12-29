-- =====================================================
-- 回滚脚本 001: 删除上传元数据字段
-- =====================================================

-- 删除索引
DROP INDEX IF EXISTS idx_metadata_source ON cw_saved_videos;
DROP INDEX IF EXISTS idx_metadata_edit_status ON cw_saved_videos;

-- 删除字段
ALTER TABLE cw_saved_videos
  DROP COLUMN upload_title,
  DROP COLUMN upload_desc,
  DROP COLUMN upload_tags,
  DROP COLUMN metadata_source,
  DROP COLUMN metadata_edit_status;
