# OmniCraft 安全×权限×信誉分体系全面审查报告

**审查日期**: 2026-05-22
**审查范围**: 认证鉴权 / CSRF / OSS隔离 / 错误脱敏 / Config脱敏 / 信誉分体系
**审查文件**: `reputation_service.go` `auth.go` `csrf.go` `oss_service.go` `config_public.go` `social_service.go` `content_service.go` `review_service.go` `judge_service.go` 等

---

## 1. 信誉分服务 (`backend/internal/service/reputation_service.go`)

### 状态：严重不完整

`reputation_service.go` 仅有 `AddReputation(userID, delta, reason, relatedID)` 和 `GetLogs()` 两个通用方法。**所有具体的信誉分规则逻辑都缺失，需在各调用方 service 中补充。**

### 1.1 加分规则对照

| # | 行为 | 要求 | 实现状态 | 说明 |
|---|------|------|---------|------|
| 1 | +3 优质创作 | 获赞 ≥ 阈值 | ❌ 未实现 | 需在点赞计数变更时检查阈值并触发 |
| 2 | +3 PR 被合并 | PR 合并加 3 分 | ❌ 未实现 | 需在 PR merge 流程中触发 |
| 3 | +2 优质评论 | 获赞 ≥ 阈值 | ❌ 未实现 | 需在评论点赞计数变更时检查 |
| 4 | +1 举报/标签 | 成功举报或标签被认可 | ❌ 未实现 | 需在审核确认后触发 |
| 5 | +1 判官准确率 | 投票后统计准确率奖励 | ❌ 未实现 | 需在判决结束后批量统计 |
| 6 | +1 素质课程 | 完成课程(每门仅一次) | ❌ 未实现 | 需在课程完成后触发 |

### 1.2 扣分规则对照

| # | 行为 | 要求 | 实现状态 | 说明 |
|---|------|------|---------|------|
| 7 | −3 恶意/抄袭内容 | AI 审核 block 或管理员确认 | ✅ `review_service.go:154` | — |
| 8 | −3 恶意 PR | PR 篡改内容取向 | ❌ 未实现 | — |
| 9 | −2 恶意评论 | — | ❌ 未实现 | — |
| 10 | −2 恶意举报 | 举报经审核认定无问题 | ❌ 未实现 | — |
| 11 | −1 恶意举报标签 | 举报无效 | ❌ 未实现 | — |
| 12 | −1 判官错误 | 投票后覆盖则不扣 | ❌ 未实现 | — |

### 1.3 特殊规则对照

| # | 规则 | 实现状态 | 文件 |
|---|------|---------|------|
| 13 | 恶意内容二次发布累计扣分(7 日内 ≥ 2 次额外扣 1 分 + 冻结发布 7 天) | ✅ | `review_service.go:197-245` |
| 14 | 多渠道举报同一内容仅第一次有效加分 | ❌ 未实现 | — |
| 15 | 自我举报过滤(禁止举报自己内容) | ❌ 未实现 | — |
| 16 | 判决覆盖后取消原判官错误扣分 | ❌ 未实现 | — |

### 1.4 二次违规逻辑 Bug

`review_service.go:227`:
```go
if count != int64(threshold) {
    return nil
}
```
`count != threshold` 只在违规次数**恰好等于**阈值时触发额外惩罚。举例：
- 阈值=2，第 2 次违规触发 → 额外扣 1 分 + 冻结 7 天 ✅
- 第 3 次违规(窗口期内) → `count==3 != 2` → 不触发 ❌
- 应在窗口期内每次超标都处罚，即 `count >= threshold` 或累计触发

---

## 2. Auth 中间件 (`backend/internal/middleware/auth.go`)

### 状态：基本可靠，有 Redis 依赖风险

| 检查项 | 状态 | 说明 |
|--------|------|------|
| `AuthRequired` 解析 Bearer Token | ✅ | — |
| `AuthRequired` 检查 token 黑名单(Redis) | ✅ | key: `blacklist:token:{token}` |
| `AuthRequired` 检查用户封禁状态(Redis) | ⚠️ | key: `user:status:{id}`，见下方风险 |
| `AuthRequired` 检查 token subject=access | ✅ | — |
| `OptionalAuth` 设置 userID=0 | ✅ | 无 token / 无效 token / 封禁用户均设为 0 |
| `AdminRequired` 检查 role=admin | ✅ | role != admin 返回 403 |

### 严重风险：Redis 不可达时封禁检查被跳过

`auth.go:55-84`：
```go
if rdb != nil {
    // ... Redis ban check ...
    if err == nil {
        // check ban, return 401 if banned
    }
}
// FALLTHROUGH: if Redis is nil OR Redis error, skip ban check
c.Set(UserIDKey, claims.UserID)
c.Set(UserRoleKey, claims.Role)
c.Next()
```

当 `rdb != nil` 但 Redis 不可达时，`rdb.Get()` 返回 error `≠ nil` → 整个 ban/status 检查块被跳过 → 直接走到第 82-84 行从 JWT claims 设置用户信息放行。**被封禁用户在 Redis 宕机期间可绕过封禁检查。**

**修复建议**：Redis 查询失败时 fallback 到 DB 查询 `users.is_banned` 和 `users.role`。

### Redis 缓存生命周期

| 事件 | 操作 | 位置 |
|------|------|------|
| 登录 | `SetUserStatusCache` 写入 `user:status:{id}` (5min TTL) | `auth.go:55/83` |
| 注册 | 同上 | `auth.go:55` |
| Ban 用户 | `InvalidateUserStatusCache` 删除缓存 | `admin.go:191` → 下次请求 DB fallback |
| Unban 用户 | 同上 | `admin.go:210` |
| Logout | 黑名单: `auth_service.go` 写 `blacklist:token:{token}` | — |

---

## 3. CSRF 中间件 (`backend/internal/middleware/csrf.go`)

### 状态：可靠

| 检查项 | 状态 | 详情 |
|--------|------|------|
| 覆盖 POST/PATCH/PUT/DELETE | ✅ | `csrf.go:40-53` |
| 双提交 Cookie 模式 | ✅ | Cookie token vs Header `X-CSRF-Token` |
| 常量时间比较 | ✅ | `hmacEqual()` 逐字节 XOR |
| 全局注册 | ✅ | `main.go:51` |
| 安全 Cookie 名 (`__Host-csrf`) | ✅ | release 模式自动启用 |
| 内部 API 豁免 | ✅ | `/api/v1/internal/*` 路径跳过 |
| 随机 token 生成 | ✅ | `crypto/rand` 32 字节 hex |

**轻微问题**：`rand.Read` 返回值未检查(`csrf.go:82`)，极低风险。

---

## 4. OSS 路径隔离 (`backend/internal/service/oss_service.go`)

### 状态：可靠

`buildOSSKey` (`oss_service.go:173-181`) 强制 `uploads/{userID}/` 前缀：
```go
fmt.Sprintf("uploads/%d/%s/%d/%02d/%02d/%d_%s%s",
    userID, fileType, stamp.Year(), stamp.Month(), stamp.Day(),
    stamp.Unix(), randomPart, ext)
```
- 路径隔离正确
- 文件名含随机 hex 防碰撞
- 上传大小/类型/MIME 均有校验(`validateUploadByType`)

---

## 5. 错误消息脱敏 — `err.Error()` 残留（Task 102）

### 状态：严重不达标

`SafeErrorResponse` 仅在 `admin.go` 中使用(14 处)。**其余全部 handler 共 154+ 处直接暴露 `err.Error()` 给 HTTP 响应。** 这些错误可能包含数据库连接串、表名、列名、SQL 错误等敏感信息。

### 按文件统计

| 文件 | 泄露次数 | 典型错误 |
|------|---------|---------|
| `handler/social.go` | ~15 | DB_ERROR, INTERNAL_ERROR, VALIDATION_ERROR |
| `handler/tag.go` | ~14 | DB_ERROR, VALIDATION_ERROR |
| `handler/content.go` | ~10 | DB_ERROR, INTERNAL_ERROR, VALIDATION_ERROR |
| `handler/judge.go` | ~9 | DB_ERROR, VALIDATION_ERROR |
| `handler/agent.go` | ~10 | AGENT_ERROR, VALIDATION_ERROR, SSE error |
| `handler/category.go` | ~8 | DB_ERROR, VALIDATION_ERROR |
| `handler/user.go` | ~8 | DB_ERROR, VALIDATION_ERROR |
| `handler/discussion.go` | ~6 | DB_ERROR, VALIDATION_ERROR |
| `handler/rehab.go` | ~5 | DB_ERROR |
| `handler/follow.go` | ~4 | DB_ERROR |
| `handler/notification.go` | ~4 | DB_ERROR |
| `handler/pr.go` | ~10 | DB_ERROR, VALIDATION_ERROR |
| `handler/ip.go` | ~4 | DB_ERROR, VALIDATION_ERROR |
| `handler/message.go` | ~5 | DB_ERROR, VALIDATION_ERROR |
| `handler/appeal.go` | ~3 | DB_ERROR, VALIDATION_ERROR |
| `handler/auth.go` | ~3 | VALIDATION_ERROR, + err.Error() 字符串比较 |
| `handler/admin.go` | ~6 | 部分使用 SafeErrorResponse，部分裸露 |

### 特殊情况

**`auth.go:43`**：使用字符串比较而非 errors.Is：
```go
if err.Error() == "username already taken" {
```
这是反模式。应使用 `errors.Is(err, service.ErrUsernameTaken)`。

**`agent.go:139/212`**：SSE 事件中直接暴露 `err.Error()`：
```go
c.SSEvent("error", gin.H{"message": err.Error()})
```

---

## 6. Config Public 脱敏 (`config_public.go` + `config/config.go`)

### 状态：可靠

所有敏感字段正确标记 `json:"-"`：

| 字段 | json:"-" | 所属结构体 |
|------|----------|-----------|
| `JWT.Secret` | ✅ | JWTConfig |
| `OSS.AccessKeyID` | ✅ | OSSConfig |
| `OSS.AccessKeySecret` | ✅ | OSSConfig |
| `Green.AccessKeyID` | ✅ | GreenConfig |
| `Green.AccessKeySecret` | ✅ | GreenConfig |
| `Green.CallbackURL` | ✅ | GreenConfig |
| `Green.CallbackAllowedIPs` | ✅ | GreenConfig |
> 修正（2026-08-13）：`Green.CallbackAllowedIPs`（回调来源 IP 白名单）前提证伪——阿里云官方从未发布回调来源网段，该字段已随 #104 从 `GreenConfig` 删除；入站认证为 form+checksum 契约（#106），新增 `Green.Seed`/`Green.UID` release 必填。
| `Agent.LLMAPIKey` | ✅ | AgentConfig |
| `Agent.HMACSecret` | ✅ | AgentConfig |
| `Database.DSN` | ✅ | DatabaseConfig |
| `Database.ReadDSN` | ✅ | DatabaseConfig |
| `Redis.Addr` | ✅ | RedisConfig |
| `Redis.Password` | ✅ | RedisConfig |

**`GetConfig`** (`admin.go:311-337`)：
- 只返回 `PublicConfig`（不含上述敏感字段）
- 额外返回 `ConfigRedactStatus`（仅 bool：是否已配置）
- ✅ 正确脱敏

**`PatchConfig`** (`admin.go:340-463`)：
- `filterSensitivePatches` 过滤 key 含 `secret`/`access_key_id`/`access_key_secret`/`api_key`/`dsn`/`hmac_secret`/`password` 的字段
- ✅ 正确防护

---

## 7. 信誉分 < 3 权限限制

### 状态：不完整（6 项中仅 3 项实现）

| 操作 | 需要检查 | 实际 | 位置 |
|------|---------|------|------|
| 评论 | 信誉 < 3 禁止 | ✅ | `social_service.go:204-215` `ensureCanInteract()` |
| 点赞/点踩 | 信誉 < 3 禁止 | ✅ | 同上 |
| 讨论 | 信誉 < 3 禁止 | ✅ | 同上 |
| **发布内容** | **信誉 < 3 禁止** | **❌** | `content_service.go:85` 只检查 `publish_freeze`，不检查 reputation |
| **众裁投票** | **信誉 < 3 禁止** | **❌** | `judge_service.go` 无 reputation 检查 |
| **PR 操作** | **信誉 < 3 禁止** | **❌** | `pr.go` handler 无 reputation 检查 |
| **下载内容** | **信誉 < 3 禁止** | **❌** | 需检查 download handler |
| 收藏 | 信誉 < 3 禁止 | **❌** | `social.go:280` FavoriteHandler 不调用 ensureCanInteract |

---

## 8. 问题清单汇总

### 🔴 Critical — 需立即修复

| # | 问题 | 文件 | 行号 |
|---|------|------|------|
| C1 | `AuthRequired` Redis 不可达时跳过 ban 检查，无 DB fallback | `middleware/auth.go` | 55-84 |
| C2 | 内容发布不检查信誉分 < 3 | `service/content_service.go` | 85 |
| C3 | 众裁投票不检查信誉分 < 3 | `service/judge_service.go` | — |
| C4 | PR 操作不检查信誉分 < 3 | `handler/pr.go` | — |

### 🟠 High — 需尽快修复

| # | 问题 | 文件 |
|---|------|------|
| H1 | 154+ 处 `err.Error()` 直接暴露给 HTTP 响应(Task 102 未完成) | 全部 handler |
| H2 | 信誉分规则大面积缺失：15 条规则中仅 3 条实现 | 各 service |
| H3 | `applyRepeatViolationPenalty` 逻辑 bug：`count != threshold` 应为 `count >= threshold` | `service/review_service.go:227` |
| H4 | 缺少自我举报过滤 | `service/social_service.go:Report()` |
| H5 | `auth.go:43` 字符串比较 `err.Error()` 反模式 | `handler/auth.go` |

### 🟡 Medium

| # | 问题 | 文件 |
|---|------|------|
| M1 | 缺少多渠道重复举报去重加分逻辑 | `service/social_service.go` |
| M2 | 判官判决被管理员覆盖后不取消原错误扣分 | `service/judge_service.go` |
| M3 | 收藏操作不调用 `ensureCanInteract` | `handler/social.go:280-333` |
| M4 | `rand.Read` 返回值未检查 | `middleware/csrf.go:82` |
| M5 | SSE 事件中暴露 `err.Error()` | `handler/agent.go:139,212` |

### 🟢 Low / 已确认安全

| # | 检查项 | 状态 |
|---|--------|------|
| L1 | CSRF 双提交 Cookie 覆盖所有写操作 | ✅ |
| L2 | OSS buildOSSKey 强制用户目录隔离 | ✅ |
| L3 | Admin Config API 脱敏敏感字段 | ✅ |
| L4 | Config struct 敏感字段均有 `json:"-"` | ✅ |
| L5 | `filterSensitivePatches` 阻止敏感配置写入 | ✅ |
| L6 | Redis 黑名单缓存(登录/ban/unban 时正确更新) | ✅ |
| L7 | `OptionalAuth` 正确设置封禁用户 userID=0 | ✅ |
| L8 | `AdminRequired` 正确检查 role=admin | ✅ |
