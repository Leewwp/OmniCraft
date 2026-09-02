-- 074: Agent 会话模型重构（A-01 / #283）
-- agent_conversations 新增可空 title 与 pinned_at：标题首轮后异步生成（失败回退首条消息截断），
-- 置顶由 PATCH /agent/conversations/:id 维护；存量会话不回填，无 title 时前端显示「未命名」。
-- 列表排序：置顶组 pinned_at DESC，其余 updated_at DESC（查询层 ORDER BY ... NULLS LAST，无需新索引）。

ALTER TABLE agent_conversations ADD COLUMN IF NOT EXISTS title VARCHAR(200);
ALTER TABLE agent_conversations ADD COLUMN IF NOT EXISTS pinned_at TIMESTAMPTZ;
