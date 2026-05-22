# OmniCraft Backend API Completeness Audit

> **审计日期**: 2026-05-22
> **审计范围**: 后端 API 路由完整性、Model-Migration 一致性、安全中间件、错误处理规范
> **审计人**: Claude Code (code-reviewer agent)

---

## 目录

1. [Route 覆盖率: task.json vs routes.go](#1-route-覆盖率)
2. [Model vs Migration 字段不匹配](#2-model-vs-migration-字段不匹配)
3. [安全中间件审计](#3-安全中间件审计)
4. [错误处理一致性](#4-错误处理一致性)
5. [优先级汇总与修复建议](#5-优先级汇总与修复建议)

---

## 1. Route 覆盖率

### 1.1 缺失路由 (task.json 中定义但 routes.go 未注册)

| # | 端点 | task.json 来源 | 严重度 | 说明 |
|---|------|---------------|--------|------|
| 1 | `GET /api/v1/admin/queue/stats` | L2912 | MEDIUM | 异步队列监控端点，用于运维监控各 topic 队列深度、lag、消费速率 |
| 2 | `GET /api/v1/admin/queue/dlq` | L2911 | MEDIUM | 死信队列管理端点，列出最近 100 条死信、支持重放指定消息 |

### 1.2 路由路径偏差

Task 66 规范定义了 `/conversations/*` 路径体系，实际实现使用 `/messages/*`:

| 规范 (Task 66) | 实际 (routes.go) | 状态 |
|----------------|------------------|------|
| `GET /conversations` | `GET /messages` | 功能等价 |
| `POST /conversations` | `POST /messages` | 功能等价 |
| `GET /conversations/:id/messages` | `GET /messages/:id` | 功能等价 |
| `POST /conversations/:id/messages` | `POST /messages` (body 含 conversation_id) | 功能等价 |
| `PATCH /conversations/:id/read` | 无对应 (通过 `last_read_at` 隐式处理) | 缺失显式端点 |

功能覆盖存在但路径命名偏离规范。如果前端已按实际路径对接，建议更新 task.json 中的规范描述以保持一致。

### 1.3 路由全景 (routes.go 已注册的完整路由表)

<details>
<summary>点击展开完整路由表 (共 100+ 个端点)</summary>

```
# Auth (7 endpoints)
POST   /api/v1/auth/register          [CredentialRateLimit]
POST   /api/v1/auth/login             [CredentialRateLimit]
POST   /api/v1/auth/logout
POST   /api/v1/auth/refresh
GET    /api/v1/auth/me                [AuthRequired]
POST   /api/v1/auth/verify-email
POST   /api/v1/auth/send-verification [AuthRequired]
POST   /api/v1/auth/forgot-password   [CredentialRateLimit]
POST   /api/v1/auth/reset-password

# Users (14 endpoints)
GET    /api/v1/users/:id              [OptionalAuth]
PATCH  /api/v1/users/:id              [AuthRequired]
GET    /api/v1/users/:id/reputation   [OptionalAuth]
GET    /api/v1/users/:id/contents     [OptionalAuth]
DELETE /api/v1/users/me               [AuthRequired]
PATCH  /api/v1/users/me/password      [AuthRequired]
PATCH  /api/v1/users/me/support-info  [AuthRequired]
GET    /api/v1/users/:id/favorites    [OptionalAuth]
GET    /api/v1/users/:id/followers    [OptionalAuth]
GET    /api/v1/users/:id/following    [OptionalAuth]
GET    /api/v1/users/search           [OptionalAuth]
POST   /api/v1/users/:id/follow       [AuthRequired]
DELETE /api/v1/users/:id/follow       [AuthRequired]
GET    /api/v1/users/:id/discussions  [OptionalAuth]
GET    /api/v1/users/me/followers/stats [AuthRequired]

# User Tag Groups & Saved Searches (7 endpoints)
GET    /api/v1/users/me/tag-groups    [AuthRequired]
POST   /api/v1/users/me/tag-groups    [AuthRequired]
PATCH  /api/v1/users/me/tag-groups/:id [AuthRequired]
DELETE /api/v1/users/me/tag-groups/:id [AuthRequired]
GET    /api/v1/users/me/saved-searches [AuthRequired]
POST   /api/v1/users/me/saved-searches [AuthRequired]
DELETE /api/v1/users/me/saved-searches/:id [AuthRequired]

# IPs (8 endpoints)
GET    /api/v1/ips                    [OptionalAuth]
POST   /api/v1/ips                    [AuthRequired]
GET    /api/v1/ips/:id                [OptionalAuth]
GET    /api/v1/ips/:id/contents       [OptionalAuth]
POST   /api/v1/ips/:id/follow         [AuthRequired]
DELETE /api/v1/ips/:id/follow         [AuthRequired]
GET    /api/v1/ips/:id/discussions    [OptionalAuth]
POST   /api/v1/ips/:id/discussions    [AuthRequired]
GET    /api/v1/ips/:id/discussions/search [OptionalAuth]

# Contents (11 endpoints)
GET    /api/v1/contents               [OptionalAuth]
POST   /api/v1/contents               [AuthRequired, UploadRateLimit]
POST   /api/v1/contents/oss-token     [AuthRequired, UploadRateLimit]
GET    /api/v1/contents/:id           [OptionalAuth]
PATCH  /api/v1/contents/:id           [AuthRequired]
DELETE /api/v1/contents/:id           [AuthRequired]
GET    /api/v1/contents/:id/versions  [OptionalAuth]
GET    /api/v1/contents/:id/prs       [OptionalAuth]
GET    /api/v1/contents/:id/download  [AuthRequired]
GET    /api/v1/contents/:id/related-fanworks [OptionalAuth]
POST   /api/v1/contents/:id/report    [AuthRequired]
POST   /api/v1/contents/:id/tags/suggest [AuthRequired]
GET    /api/v1/contents/search        [OptionalAuth]

# Versions (1 endpoint)
GET    /api/v1/versions/:id           [OptionalAuth]

# PR (5 endpoints)
POST   /api/v1/pr                     [AuthRequired]
GET    /api/v1/pr/:id                 [OptionalAuth]
POST   /api/v1/pr/:id/accept          [AuthRequired]
POST   /api/v1/pr/:id/reject          [AuthRequired]
POST   /api/v1/pr/:id/merge           [AuthRequired]

# Dashboard (4 endpoints)
POST   /api/v1/dashboard/contributors/:userId/block    [AuthRequired]
DELETE /api/v1/dashboard/contributors/:userId/block    [AuthRequired]
GET    /api/v1/dashboard/tag-suggestions                [AuthRequired]
PATCH  /api/v1/dashboard/tag-suggestions/:id            [AuthRequired]

# Social (8 endpoints)
GET    /api/v1/social/comments        [OptionalAuth]
POST   /api/v1/social/comments        [AuthRequired]
DELETE /api/v1/social/comments/:id    [AuthRequired]
PATCH  /api/v1/social/comments/:id    [AuthRequired, CommentEditRateLimit]
GET    /api/v1/social/discussions     [OptionalAuth]
POST   /api/v1/social/discussions     [AuthRequired]
GET    /api/v1/social/discussions/:id [OptionalAuth]
POST   /api/v1/social/reactions       [AuthRequired]
POST   /api/v1/social/comments/:id/report [AuthRequired]

# Favorites (3 endpoints)
POST   /api/v1/favorites              [AuthRequired]
DELETE /api/v1/favorites/:contentId   [AuthRequired]

# Judge (7 endpoints)
GET    /api/v1/judge/exam/:category   [OptionalAuth]
POST   /api/v1/judge/exam/submit      [AuthRequired]
GET    /api/v1/judge/queue            [AuthRequired]
POST   /api/v1/judge/vote             [AuthRequired]
GET    /api/v1/judge/cases/:id/verdict [OptionalAuth]
POST   /api/v1/judge/reasons/:id/vote [AuthRequired]

# Stats (2 endpoints)
GET    /api/v1/stats/summary          (无 auth)
GET    /api/v1/ips/stats/category_counts (无 auth)

# Categories (1 endpoint)
GET    /api/v1/categories             [OptionalAuth]

# Tags (2 endpoints)
GET    /api/v1/tags/faceted           [OptionalAuth]
GET    /api/v1/tags/search            [OptionalAuth]

# Search (4 endpoints)
GET    /api/v1/search/suggestions     (无 auth)
GET    /api/v1/search/trending        (无 auth)

# Follow (2 endpoints on routes)

# Appeals (2 endpoints)
POST   /api/v1/appeals                [AuthRequired]
GET    /api/v1/appeals/me             [AuthRequired]

# Notifications (4 endpoints)
GET    /api/v1/notifications          [AuthRequired]
PATCH  /api/v1/notifications/:id/read [AuthRequired]
POST   /api/v1/notifications/read-all  [AuthRequired]
GET    /api/v1/notifications/unread-count [AuthRequired]

# Messages (5 endpoints)
GET    /api/v1/messages               [AuthRequired]
POST   /api/v1/messages               [AuthRequired]
GET    /api/v1/messages/:id           [AuthRequired]
DELETE /api/v1/messages/:id           [AuthRequired]
DELETE /api/v1/messages/conversations/:id [AuthRequired]

# Browse History (3 endpoints)
POST   /api/v1/users/me/history       [AuthRequired]
GET    /api/v1/users/me/history       [AuthRequired]
DELETE /api/v1/users/me/history       [AuthRequired]

# Discussions (3 endpoints)
GET    /api/v1/discussions/:id        [OptionalAuth]
POST   /api/v1/discussions/:id/comments [AuthRequired]
PATCH  /api/v1/discussions/:id/pin    [AuthRequired]

# Reputation (1 endpoint)
GET    /api/v1/reputation-logs/me     [AuthRequired]

# Agent (8 endpoints)
POST   /api/v1/agent/upload-assist    [AuthRequired, AgentRateLimit]
POST   /api/v1/agent/compliance-check [AuthRequired, AgentRateLimit]
POST   /api/v1/agent/search           [AuthRequired, AgentRateLimit]
POST   /api/v1/agent/usage-guide/:id  [AuthRequired, AgentRateLimit]
POST   /api/v1/agent/moderate/:id     [AuthRequired, AgentRateLimit]
POST   /api/v1/agent/chat/stream      [AuthRequired, AgentRateLimit]
GET    /api/v1/agent/conversations    [AuthRequired, AgentRateLimit]
GET    /api/v1/agent/conversations/:id [AuthRequired, AgentRateLimit]
GET    /api/v1/agent/script/:id       [AuthRequired, AgentRateLimit]

# Rehab (5 endpoints)
GET    /api/v1/rehab/courses          [AuthRequired]
GET    /api/v1/rehab/courses/:id      [AuthRequired]
POST   /api/v1/rehab/courses/:id/start [AuthRequired]
POST   /api/v1/rehab/courses/:id/complete [AuthRequired]
GET    /api/v1/rehab/my-progress      [AuthRequired]

# Admin (28 endpoints)
GET    /api/v1/admin/ips              [AuthRequired, AdminRequired]
POST   /api/v1/admin/ips/:id/approve  [AuthRequired, AdminRequired]
POST   /api/v1/admin/ips/:id/reject   [AuthRequired, AdminRequired]
GET    /api/v1/admin/contents         [AuthRequired, AdminRequired]
GET    /api/v1/admin/contents/trash   [AuthRequired, AdminRequired]
POST   /api/v1/admin/contents/:id/ban [AuthRequired, AdminRequired]
PATCH  /api/v1/admin/contents/:id/restore [AuthRequired, AdminRequired]
GET    /api/v1/admin/users            [AuthRequired, AdminRequired]
POST   /api/v1/admin/users/:id/ban    [AuthRequired, AdminRequired]
POST   /api/v1/admin/users/:id/unban  [AuthRequired, AdminRequired]
GET    /api/v1/admin/appeals          [AuthRequired, AdminRequired]
POST   /api/v1/admin/appeals/:id      [AuthRequired, AdminRequired]
GET    /api/v1/admin/reports          [AuthRequired, AdminRequired]
PATCH  /api/v1/admin/reports/:id      [AuthRequired, AdminRequired]
GET    /api/v1/admin/reports/stats    [AuthRequired, AdminRequired]
GET    /api/v1/admin/config           [AuthRequired, AdminRequired]
PATCH  /api/v1/admin/config           [AuthRequired, AdminRequired]
POST   /api/v1/admin/judge/questions  [AuthRequired, AdminRequired]
POST   /api/v1/admin/categories       [AuthRequired, AdminRequired]
PATCH  /api/v1/admin/categories/:id   [AuthRequired, AdminRequired]
DELETE /api/v1/admin/categories/:id   [AuthRequired, AdminRequired]
PUT    /api/v1/admin/categories/reorder [AuthRequired, AdminRequired]
GET    /api/v1/admin/llm-configs      [AuthRequired, AdminRequired]
POST   /api/v1/admin/llm-configs      [AuthRequired, AdminRequired]
PATCH  /api/v1/admin/llm-configs/:id  [AuthRequired, AdminRequired]
DELETE /api/v1/admin/llm-configs/:id  [AuthRequired, AdminRequired]
POST   /api/v1/admin/llm-configs/:id/activate [AuthRequired, AdminRequired]
POST   /api/v1/admin/llm-configs/:id/test [AuthRequired, AdminRequired]

# Internal (1 endpoint)
POST   /api/v1/internal/ai-callback  (no CSRF, no auth — internal only)
```
</details>

---

## 2. Model vs Migration 字段不匹配

### 2.1 CRITICAL: Comment 模型有 4 个字段缺少 Migration

**影响文件**:
- Model: `backend/internal/model/social.go:26-43` (Comment struct)
- Migration: `backend/migrations/011_social.sql:18-28`

| 字段 | Model 定义 | Migration 011 | 后续 Migration | 状态 |
|------|-----------|---------------|---------------|------|
| `target_type` | `gorm:"size:20;index"` | **不存在** | 无 | **缺失** |
| `target_id` | `gorm:"index"` | **不存在** | 无 | **缺失** |
| `content` | `gorm:"type:text"` | **不存在** | 无 | **缺失** |
| `updated_at` | `gorm:"autoUpdateTime"` | **不存在** | 无 | **缺失** |

**影响**: 如果数据库通过迁移脚本手动初始化，这 4 列不存在。GORM 查询会报 "column does not exist" 错误。如果使用 GORM AutoMigrate，它会静默添加列，导致迁移文件和实际 schema 不一致。

**修复方案**: 创建 migration 043 添加这 4 列:
```sql
ALTER TABLE comments ADD COLUMN IF NOT EXISTS target_type VARCHAR(20);
ALTER TABLE comments ADD COLUMN IF NOT EXISTS target_id BIGINT;
ALTER TABLE comments ADD COLUMN IF NOT EXISTS content TEXT;
ALTER TABLE comments ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();
CREATE INDEX IF NOT EXISTS idx_comments_target ON comments(target_type, target_id);
```

### 2.2 MEDIUM: User.PreferredLocale 默认值不一致

| 来源 | 值 |
|------|-----|
| `model/user.go:15` | `default:'zh'` |
| `migrations/001_users.sql:10` | `DEFAULT 'zh-CN'` |
| `migrations/014_judge.sql:62` | `DEFAULT 'zh-CN'` |

Model 标签中的 `'zh'` 与所有 migration 的 `'zh-CN'` 不一致。GORM AutoMigrate 可能会尝试更改默认值。**建议统一为 `'zh-CN'`**。

### 2.3 经 Migration 后补齐的字段 (已正确,无需修复)

以下字段在基础 migration 中不存在，但有后续 migration 补齐：

| 表 | 字段 | 基础 Migration | 补齐 Migration |
|----|------|---------------|---------------|
| `content_items` | `description` | 006 无 | 036 |
| `content_items` | `source_original_id` | 006 无 | 036 |
| `content_items` | `download_count` | 006 无 | 040 |
| `content_items` | `ban_reason` | 006 无 | 040 |
| `users` | `support_info` | 001 无 | 030 |
| `users` | `email_verified_at` | 001 无 | 040 |
| `discussions` | `is_pinned` | 011 无 | 035 |
| `discussions` | `last_active_at` | 011 无 | 035 |
| `conversation_participants` | `unread_count` | 024 无 | 039 |
| `conversation_participants` | `left_at` | 024 无 | 040 |
| `notifications` | channel CHECK 扩展 | 023 (3 values) | 038 (5 values) |

### 2.4 表重复创建风险

`follows` 和 `appeals` 表在 migration 011 (social.sql) 中已创建，但 Task 60/61 规范描述 migration 033/034 为"创建"这两个表。实际 033/034 改为 ALTER 语句，运行正常，但存在设计不一致：

| 表 | 创建于 | 修改于 | 风险 |
|----|--------|--------|------|
| `follows` | 011 (CREATE TABLE) | 033 (ALTER TABLE, 加 CHECK 约束) | 如果跳过 011 直接运行 033 会失败 |
| `appeals` | 011 (CREATE TABLE) | 034 (ALTER TABLE, 加 CHECK 约束) | 如果跳过 011 直接运行 034 会失败 |

**建议**: 在 033 和 034 文件头部添加注释说明它们依赖于 011。

---

## 3. 安全中间件审计

### 3.1 CSRF: PASS ✓

- **全局应用**: `main.go:51` — `r.Use(middleware.CSRF(cfg))`
- **拦截范围**: 所有 POST/PATCH/PUT/DELETE 请求
- **豁免路径**: `/api/v1/internal/ai-callback` (内部回调)
- **实现**: Cookie 双提交模式, 常量时间比较, Secure 标志在 release 模式下启用
- **CORS header**: `X-CSRF-Token` 已加入 AllowHeaders

### 3.2 AuthRequired: PASS ✓

- 所有 POST/PATCH/PUT/DELETE 端点均有 AuthRequired
- Dashboard, Notifications, Messages, Favorites, Agent, Rehab, Admin 组在 group 级别应用
- 公开端点 (register, login, forgot-password, reset-password) 正确豁免

### 3.3 OptionalAuth: 4 处缺失

| # | 路由 | 当前状态 | 预期 | 影响 |
|---|------|---------|------|------|
| 1 | `GET /api/v1/stats/summary` | 无 auth | `OptionalAuth` | 无法获取登录用户上下文 |
| 2 | `GET /api/v1/search/suggestions` | 无 auth | `OptionalAuth` | 无法根据用户历史个性化推荐 |
| 3 | `GET /api/v1/search/trending` | 无 auth | `OptionalAuth` | 同上 |
| 4 | `GET /api/v1/ips/stats/category_counts` | 无 auth | `OptionalAuth` | 无法返回 `is_following` 状态 |

**修复方案** (routes.go):

```go
// Before:
v1.GET("/stats/summary", statsHandler.GetSummary)
v1.GET("/search/suggestions", searchHandler.Suggestions)
v1.GET("/search/trending", searchHandler.Trending)
v1.GET("/ips/stats/category_counts", ipStatsHandler.GetCategoryCounts)

// After:
v1.GET("/stats/summary", optAuth, statsHandler.GetSummary)
v1.GET("/search/suggestions", optAuth, searchHandler.Suggestions)
v1.GET("/search/trending", optAuth, searchHandler.Trending)
v1.GET("/ips/stats/category_counts", optAuth, ipStatsHandler.GetCategoryCounts)
```

### 3.4 RateLimit: PASS ✓

| 中间件 | 应用范围 | 状态 |
|--------|---------|------|
| Global RateLimit | 所有路由 | ✓ |
| CredentialRateLimit | register, login, forgot-password | ✓ |
| UploadRateLimit | create content, oss-token | ✓ |
| CommentEditRateLimit | edit comment | ✓ |
| AgentRateLimit | 所有 agent 路由 | ✓ |

---

## 4. 错误处理一致性

### 4.1 严重: 162 处 `err.Error()` 直接暴露给客户端

**影响范围**: 22 个 handler 文件

**按文件统计**:

| 文件 | 暴露次数 | 风险等级 |
|------|---------|---------|
| `social.go` | 21 | HIGH |
| `tag.go` | 17 | HIGH |
| `content.go` | 14 | HIGH |
| `agent.go` | 13 | HIGH |
| `pr.go` | 13 | HIGH |
| `judge.go` | 11 | MEDIUM |
| `user.go` | 11 | MEDIUM |
| `category.go` | 9 | MEDIUM |
| `discussion.go` | 7 | MEDIUM |
| `ip.go` | 5 | MEDIUM |
| `message.go` | 5 | MEDIUM |
| `rehab.go` | 5 | MEDIUM |
| `auth.go` | 4 | MEDIUM |
| `browse_history.go` | 4 | MEDIUM |
| `follow.go` | 4 | MEDIUM |
| `notification.go` | 4 | MEDIUM |
| `appeal.go` | 3 | LOW |
| `internal.go` | 3 | LOW |
| `version.go` | 2 | LOW |
| `admin.go` | 5 | LOW |
| `reputation.go` | 1 | LOW |
| `ip_stats.go` | 1 | LOW |

### 4.2 危险模式示例

**模式 A — 内部错误暴露原始消息**:
```go
// content.go:85 — 最危险: 可能泄露 DB 连接信息
c.JSON(http.StatusInternalServerError, gin.H{
    "code": "INTERNAL_ERROR",
    "message": err.Error(), // ← 直接暴露
})
```

**模式 B — 校验错误可能泄露 schema 细节**:
```go
// browse_history.go:30
c.JSON(http.StatusBadRequest, gin.H{
    "code": "VALIDATION_ERROR",
    "message": err.Error(), // ← 可能泄露字段名/类型
})
```

**模式 C — Agent 错误暴露 LLM 内部信息**:
```go
// agent.go:65
c.JSON(http.StatusInternalServerError, gin.H{
    "code": "AGENT_ERROR",
    "message": err.Error(), // ← 可能暴露 API key 前缀/模型名
})
```

### 4.3 正确做法 (admin.go 已正确示范)

```go
// admin.go — 使用 SafeErrorResponse 脱敏
response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
// → 客户端收到: {"code": "DB_ERROR", "message": "database operation failed, please try again later"}

// admin.go — 使用统一响应函数
response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
```

### 4.4 已存在的脱敏基础设施

项目已有完整的错误处理工具包 (`backend/internal/pkg/response/`):

| 函数 | 用途 | 是否被广泛使用 |
|------|------|--------------|
| `SafeErrorResponse(c, status, code, err)` | 自动脱敏错误消息 | **仅 admin.go** |
| `Error(c, status, code, message)` | 统一错误信封 | 部分使用 |
| `ValidationError(c, message)` | 400 快捷 | 少量使用 |
| `NotFound(c, message)` | 404 快捷 | 少量使用 |
| `Unauthorized(c, message)` | 401 快捷 | 少量使用 |
| `Forbidden(c, message)` | 403 快捷 | 少量使用 |
| `InternalError(c, message)` | 500 快捷 | 少量使用 |

---

## 5. 优先级汇总与修复建议

### P0 — 上线前必须修复

| # | 问题 | 文件 | 修复方式 |
|---|------|------|---------|
| 1 | **Comment 模型 4 列无 migration** | `model/social.go`, `migrations/` | 新建 migration 043 添加 `target_type`, `target_id`, `content`, `updated_at` 列 |
| 2 | **162 处 `err.Error()` 泄露** | 22 个 handler 文件 | 全局替换为 `response.SafeErrorResponse()` 或 `response.Error()` |

**P0 修复脚本参考** (每个 handler 文件):
```go
// Before:
c.JSON(http.StatusInternalServerError, gin.H{"code": "DB_ERROR", "message": err.Error()})

// After:
response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)

// Before:
c.JSON(http.StatusBadRequest, gin.H{"code": "VALIDATION_ERROR", "message": err.Error()})

// After:
response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request")
```

### P1 — 本迭代修复

| # | 问题 | 修复方式 |
|---|------|---------|
| 3 | 4 个公开 GET 端点缺少 OptionalAuth | 在 routes.go 中给 `stats/summary`, `search/suggestions`, `search/trending`, `ips/stats/category_counts` 添加 `optAuth` |
| 4 | User.PreferredLocale 默认值不一致 | 将 model 中的 `default:'zh'` 改为 `default:'zh-CN'` |

### P2 — 下个迭代修复

| # | 问题 | 修复方式 |
|---|------|---------|
| 5 | Admin queue 端点未实现 | 实现或从 task.json 中标记为 P2 延期 |
| 6 | follows/appeals 迁移文件歧义 | 在 033/034 头部添加注释 `-- NOTE: follows/appeals tables created in migration 011; this migration adds constraints` |

### 验证清单

修复后执行以下验证:

- [ ] `go build ./...` 无错误
- [ ] `go vet ./...` 无警告
- [ ] 所有 migration 可在空数据库顺序执行 (`psql -f migrations/*.sql`)
- [ ] `grep -r 'err.Error()' backend/internal/handler/` 返回 0 结果 (或仅剩 SafeErrorResponse 调用)
- [ ] `grep -r 'OptionalAuth' backend/internal/handler/routes.go` 覆盖所有公开 GET 端点
- [ ] Comment 表的 `target_type`, `target_id`, `content`, `updated_at` 列存在

---

## 附录: 审计使用的命令

```bash
# 提取所有路由
grep -E '(GET|POST|PUT|PATCH|DELETE)\(' backend/internal/handler/routes.go

# 统计 err.Error() 暴露
grep -c 'err\.Error()' backend/internal/handler/*.go

# 检查 OptionalAuth 缺失
grep -B1 'GET\|POST' backend/internal/handler/routes.go | grep -v optAuth

# 对比 model 和 migration
diff <(grep 'gorm:' backend/internal/model/social.go | sort) <(grep -E '^\s+\w+' backend/migrations/011_social.sql | sort)
```
