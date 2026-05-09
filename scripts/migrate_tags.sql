-- 迁移脚本：移除旧的标签字段并添加新的 tags 字段

-- 1. 删除 BigTag 和 SmallTag 表
DROP TABLE IF EXISTS big_tags;
DROP TABLE IF EXISTS small_tags;

-- 2. 移除 contents 表中的旧标签字段
ALTER TABLE contents
    DROP COLUMN IF EXISTS big_tag_id,
    DROP COLUMN IF EXISTS small_tag_id;

-- 3. 添加新的 tags 字段（JSON格式）
ALTER TABLE contents
    ADD COLUMN IF NOT EXISTS tags TEXT NULL;

-- 4. 创建索引（可选）
CREATE INDEX IF NOT EXISTS idx_contents_tags ON contents((JSON_EXTRACT(tags, '$[0]')));

SELECT 'Migration completed successfully!' AS result;
