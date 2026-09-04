-- Depends on: 034_appeals.sql (appeals table and target_type check must exist)
-- Modifies: appeals table constraints
-- T29（FIX-15）：封禁用户申诉出路——target_type 增加 'account'（账号申诉），
-- 批准后解封（users.is_banned=false + 清 ban_reason）。约束重建保持幂等。

ALTER TABLE appeals
    DROP CONSTRAINT IF EXISTS appeals_target_type_check;

ALTER TABLE appeals
    ADD CONSTRAINT appeals_target_type_check CHECK (target_type IN ('content','comment','account'));
