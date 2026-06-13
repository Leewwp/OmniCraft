-- Test fixture seed data for search integration tests.
--
-- Reserved ID ranges (do not use outside test fixtures):
--   users:        9001–9099
--   ips:          8001–8099
--   content_items: 7001–7099
--
-- ON CONFLICT DO UPDATE ensures fixture drift is corrected on every run,
-- so the database always matches the latest fixture definition.

INSERT INTO users (id, email, password_hash, username, role, is_banned, reputation, email_verified_at)
VALUES
  (9001, 'search_author@example.com', '$2a$10$placeholder', 'search_author', 'user', false, 10, NOW()),
  (9002, 'banned_author@example.com', '$2a$10$placeholder', 'banned_author', 'user', true, 10, NOW()),
  (9003, 'deleted_author@example.com', '$2a$10$placeholder', 'deleted_author', 'user', false, 10, NOW())
ON CONFLICT (id) DO UPDATE SET
  email            = EXCLUDED.email,
  password_hash    = EXCLUDED.password_hash,
  username         = EXCLUDED.username,
  nickname         = EXCLUDED.nickname,
  role             = EXCLUDED.role,
  is_banned        = EXCLUDED.is_banned,
  reputation       = EXCLUDED.reputation,
  email_verified_at = EXCLUDED.email_verified_at;

UPDATE users SET deleted_at = NOW() WHERE id = 9003;

INSERT INTO ips (id, name, category, status, created_at)
VALUES
  (8001, '搜索测试IP', 'anime', 'approved', NOW())
ON CONFLICT (id) DO UPDATE SET
  name        = EXCLUDED.name,
  category    = EXCLUDED.category,
  status      = EXCLUDED.status;

INSERT INTO content_items (id, title, description, author_id, zone, ip_id, category, content_type, status, is_public, created_at, updated_at)
VALUES
  (7001, '春日穿搭指南', '春季穿搭推荐', 9001, 'original', NULL, 'beauty_fashion', 'image', 'published', true, NOW(), NOW()),
  (7002, '桌面改造计划', '分享我的桌面布置', 9001, 'original', NULL, 'home', 'article', 'published', true, NOW(), NOW()),
  (7003, '隐藏内容测试', '此内容不应出现在搜索结果', 9001, 'original', NULL, 'tech_digital', 'article', 'published', false, NOW(), NOW()),
  (7004, '已下架内容', '此内容已下架', 9001, 'original', NULL, 'food', 'image', 'under_review', true, NOW(), NOW()),
  (7005, '封禁用户内容', '此作者被封禁', 9002, 'original', NULL, 'gaming', 'video', 'published', true, NOW(), NOW()),
  (7006, '已删除用户内容', '此作者已删除', 9003, 'original', NULL, 'literature', 'article', 'published', true, NOW(), NOW()),
  (7007, '二创区内容测试', '二创搜索测试', 9001, 'fanwork', 8001, NULL, 'image', 'published', true, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
  title        = EXCLUDED.title,
  description  = EXCLUDED.description,
  zone         = EXCLUDED.zone,
  category     = EXCLUDED.category,
  content_type = EXCLUDED.content_type,
  status       = EXCLUDED.status,
  is_public    = EXCLUDED.is_public,
  author_id    = EXCLUDED.author_id;

UPDATE content_items SET deleted_at = NOW() WHERE id = 7004;

INSERT INTO content_tags (content_item_id, tag)
VALUES
  (7001, '穿搭'),
  (7001, '春季'),
  (7002, '桌面改造'),
  (7002, '家居'),
  (7007, '二创')
ON CONFLICT DO NOTHING;
