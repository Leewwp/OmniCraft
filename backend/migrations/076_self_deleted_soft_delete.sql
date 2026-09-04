-- Depends on: 001_initial_schema.sql (users table must exist)
-- Modifies: users rows (legacy self_deleted accounts)
-- T30（FIX-20）：存量「注销伪装成封禁」用户一次性幂等修正——
-- ban_reason='self_deleted' 的行改写为 deleted_at 软删除并清除 is_banned；
-- deleted_at 已回填的行只清理残留 is_banned（重跑幂等，时间戳不覆盖）。
-- 真实封禁（其他 ban_reason）不受影响。

UPDATE users
SET deleted_at = CURRENT_TIMESTAMP,
    is_banned = FALSE,
    ban_reason = ''
WHERE ban_reason = 'self_deleted'
  AND deleted_at IS NULL;

UPDATE users
SET is_banned = FALSE,
    ban_reason = ''
WHERE ban_reason = 'self_deleted'
  AND deleted_at IS NOT NULL
  AND (is_banned = TRUE OR ban_reason <> '');
