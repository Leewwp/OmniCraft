-- 080_ip_category_backfill.sql — SP-19 G1-3（#517）
-- IP 分类词表统一为 11 类后，把历史第三套词表值回填进正式类目：
--   gaming → game（7 个，原任何筛选不可见）
--   video  → other（2 个，原任何筛选不可见）
--   literature 原值保留（9 个，新晋正式类目）；vtuber 由前端补正式标签，无存量。
-- 未知历史值一并归 other（执行前后跑分布 SELECT 对账，见票据 #517/#525）。
--
-- 回滚依据：先落备份表 ips_category_backup_20260914（保留 id + category），
-- 异常时按备份表回写：
--   UPDATE ips i SET category = b.category
--   FROM ips_category_backup_20260914 b WHERE i.id = b.id;

CREATE TABLE IF NOT EXISTS ips_category_backup_20260914 AS
SELECT id, category FROM ips;

UPDATE ips SET category = 'game' WHERE category = 'gaming';
UPDATE ips SET category = 'other' WHERE category IN ('video', '') OR category IS NULL;
-- 未知值兜底：不在 11 类白名单内的全部归 other（幂等，白名单内值不动）。
UPDATE ips SET category = 'other'
WHERE category NOT IN (
  'game', 'film_tv', 'anime', 'manga', 'novel', 'literature',
  'music', 'variety', 'short_drama', 'vtuber', 'other'
);
