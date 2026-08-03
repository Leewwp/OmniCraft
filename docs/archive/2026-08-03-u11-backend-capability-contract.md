# U-11 后端能力临时交接契约

**创建日期**: 2026-08-03
**预计失效日期**: 2026-10-03

> 本文仅用于 U-11 前后端串行交接。业务规则、执行计划和生产代码仍是权威来源；若发生冲突，以 `AGENTS.md` 的文档权威顺序为准。

## 响应兼容性

- `POST /api/v1/auth/login` 保留既有顶层 `user`、`tokens`，在顶层新增 `capabilities`。
- `GET /api/v1/auth/me` 保留既有顶层 `user`、`csrf_token`，在顶层新增同形状 `capabilities`。
- 本轮只提供 `can_interact`；没有已确认的 `can_publish` 消费者，不扩展发布能力。
- 不在任何响应或 public config 中暴露 `threshold`、`min_score`、`reputation` 配置值或等价字段。

```json
{
  "capabilities": {
    "can_interact": true
  }
}
```

拒绝状态可附带稳定、安全且可本地化的 `interaction_denial_reason` code；允许状态省略该字段。允许的 code 为 `EMAIL_NOT_VERIFIED`、`INSUFFICIENT_REPUTATION`、`AUTH_STATUS_UNAVAILABLE`。不得返回原始后端错误或阈值。

## 判定与拒绝矩阵

成功响应中的互动能力判定顺序与 action middleware 保持一致：邮箱未验证 → 信誉不足 → 状态不可用。发布冻结不参与 `can_interact` 判定。

| 状态 | login | `/auth/me` | 成功响应 capability / action gate |
|---|---|---|---|
| 未认证 | `401 INVALID_CREDENTIALS`（凭证失败） | `401 UNAUTHORIZED` | 无成功 capability；action 为 `401 UNAUTHORIZED` |
| 封禁 | `403 USER_BANNED` | auth middleware `401 USER_BANNED` | 不改变既有鉴权语义，不为返回 capability 放行 |
| 邮箱未验证 | `403 EMAIL_NOT_VERIFIED` | 成功响应但 `can_interact=false`、reason=`EMAIL_NOT_VERIFIED` | action 为 `403 EMAIL_NOT_VERIFIED` |
| 信誉低于配置阈值 | 成功响应，`can_interact=false`、reason=`INSUFFICIENT_REPUTATION` | 同左 | action 为 `403 INSUFFICIENT_REPUTATION` |
| 信誉等于或高于阈值 | 成功响应，`can_interact=true` | 同左 | action 放行 |
| 仅发布冻结 | `can_interact=true` | `can_interact=true` | 普通互动放行；发布策略仍可返回 `PUBLISH_FROZEN` |
| 状态/配置不可用 | 不伪造 capability，按既有安全失败响应 | 不伪造 capability，按既有安全失败响应 | action fail closed |

## 一致性与恢复

- login 与 `/auth/me` 使用同一个纯判定函数；互动 middleware 也复用该函数，避免阈值、邮箱验证及拒绝码漂移。
- 课程完成事务成功后必须失效 `user:status:{user_id}` runtime cache，使 capability 与 action gate 在下一请求同时看到新信誉值。
- 前端课程完成成功后调用 `refreshUser()`，并从响应中的 capability 刷新 UI；缺失 `capabilities` 时 fail closed，显示本地化“互动状态暂不可用”，不得回退到 `user.reputation >= 3`。

## 明确排除

- 当前 middleware 读取 `user:publish_freeze:{id}`，而内容/审核服务写入 `publish:freeze:{id}`。U-11 记录该差异，但不静默扩大到发布冻结修复。
- 不修改 public config、阈值配置、迁移、compose、release、Tauri、U-12 verifier 或 Ops-01 文件。
