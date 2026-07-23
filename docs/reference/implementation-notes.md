# 实现笔记（安全加固与架构优化，Task 99-145）

> 本文档由 2026-07-23 文档瘦身从 `architecture.md` §14-15 抽取，章节号保持原编号以便深链兼容。

## 14. 安全加固（Task 99-105）

> 本章节对应 task.json 任务 99-105，记录安全漏洞修复的架构决策。

### 14.1 Admin Config 泄露防护

- `GET /admin/config` 和 `PATCH /admin/config` 仅返回 `PublicConfig` 结构体（Features、Limits、Reputation、Judge、Social、Recommendation 等安全配置字段）
- 所有敏感字段（JWT.Secret、OSS.AccessKeySecret、Green.AccessKeySecret、Agent.LLMAPIKey、Agent.HMACSecret、Database.DSN）在 Config 结构体中标记 `json:"-"`
- 管理员运行时配置修改持久化到 `data/config_override.yaml`，重启后合并加载

### 14.2 CORS 策略

- CORS 中间件不再使用 `Access-Control-Allow-Origin: *`，改为从 `config.yaml > allowed_origins` 列表动态匹配请求 Origin
- 开发环境允许 `localhost` 全系列；生产环境必须配置具体域名
- 所有 POST/PATCH/DELETE 请求要求 `X-CSRF-Token` 请求头（Task 104）

### 14.3 Auth 中间件实时状态检查

- `AuthRequired` 中间件在 JWT 验证后增加 Redis 用户状态缓存检查：`user:status:{userID}` → `{"is_banned":bool,"role":"string"}`，TTL=5min
- `AdminRequired` 在 AuthRequired 基础上从缓存检查 `role=="admin"`
- 用户被 ban 或角色变更后立即更新缓存，无需等 token 过期
- 登录/注册时写入缓存；BanUser/UnbanUser/角色变更时更新缓存

### 14.4 错误消息脱敏

- 创建 `internal/pkg/response/safe_error.go`：业务已知错误（ErrContentNotFound etc.）返回原始消息，未知错误统一返回"操作失败，请稍后重试"
- 创建 `internal/pkg/response/error_codes.go`：错误码到 HTTP 状态码的标准映射表
- 全局替换 `c.JSON(500, gin.H{"error": err.Error()})` 为 `SafeErrorResponse(c, err)`
- 内部错误仅记录到结构化日志（slog），不返回客户端

### 14.5 账号删除与 OSS 路径隔离

- `DeleteAccount` 将 `password_hash` 设为 `bcrypt.GenerateFromPassword(randomBytes(32), 14)` 而非空字符串
- OSS 预签名 URL 的 key 必须以 `uploads/{userID}/` 为前缀，handler 校验路径前缀与当前用户 ID 匹配

### 14.6 Goroutine Panic Recovery

- 创建 `internal/pkg/recovery.go`：`GoSafe(fn func())` 包装 `defer recover + slog.Error`
- 所有异步 goroutine（submitIPForAIReview、EmbedContentAsync、triggerVideoSnapshot 等）使用 `go recovery.GoSafe(func() { ... })`
- 19 处静默丢弃错误（`_ = h.contentRepo.IncrViewCount(id)` 等）改为 `slog.Error + 优雅降级`

---


---

## 15. 架构优化（Task 106-145）

### 15.1 前端受保护路由守卫

- 新增 `(protected)/layout.tsx`：统一鉴权守卫，未登录用户重定向到 `/login`，消除了各页面独立 `useAuth()` 检查导致的内容闪烁
- admin 路由在 auth 守卫基础上叠加角色检查

### 15.2 Service Container 模式

- 创建 `internal/container/container.go`：统一初始化所有 repo → service → handler，通过依赖注入避免 service 重复实例化
- `cmd/server/main.go` 中初始化 `ServiceContainer` 并注入所有 handler
- 每个 service 只初始化一次，handler 从 container 获取 service 实例

### 15.3 通知自动创建

- 创建 `internal/service/notification_service.go`：`CreateNotification(userID, channel, type, title, body, targetType, targetID)`
- 在 CommentService、SocialService、FollowService、PRService、AppealService、MessageService 中调用
- 所有通知创建使用异步 goroutine（`go recovery.GoSafe()`），不阻塞主流程
- 数据库索引 `idx_notifications_user_unread ON notifications(user_id, is_read, created_at DESC)`

### 15.4 内容全文搜索

- 新增迁移 `038_content_search.sql`：在 `content_items` 表添加 `search_vector tsvector` 列，创建 GIN 索引；创建触发器在 title/description 变更时自动更新
- 新增 `GET /contents/search?q=&zone=&category=&content_type=&tags=&sort=relevance&page=&limit=20`
- 使用 `to_tsquery + ts_rank` 实现全文检索，`ts_headline` 返回高亮片段
- 应用 `OptionalAuth` 中间件：登录用户可个性化排序，匿名用户纯相关性排序

### 15.5 搜索建议与热门搜索

- `GET /search/suggestions?q=&limit=10`：tags name 前缀匹配 + content_items title 前缀匹配，返回 Top 10
- `GET /search/trending?limit=20`：从 Redis hot_rank 数据提取热门关键词
- 搜索建议按使用频率排序（Redis 计数）

### 15.6 内容下载 API

- `GET /contents/:id/download`：校验登录态 + `AllowCopy=true` 后返回 OSS 签名下载 URL（302 重定向）
- Redis 维护下载计数 ZSET `download_counts`
- `content_items` 新增 `download_count` 字段（或独立 Redis 维护）

### 15.7 收藏去重与分页筛选

- `AddFavorite` 幂等性：已收藏返回 200 而非 500
- `RemoveFavorite` 安全处理：删除不存在记录返回 200
- `GET /users/:id/favorites` 增加 `content_type`、`page`、`limit` 查询参数，支持分页元数据
- `GET /contents/:id` 增加 `is_favorited` 字段（OptionalAuth）

### 15.8 内容软删除

- 新增迁移 `041_content_soft_delete.sql`：在 `content_items` 表添加 `deleted_at TIMESTAMPTZ` 字段（nullable，索引）
- `DeleteContent` 改为 `UPDATE content_items SET deleted_at=NOW() WHERE id=?`
- 所有内容列表查询添加 `WHERE deleted_at IS NULL` 条件（GORM DefaultScope 或全局 scope）
- 新增 `GET /admin/contents/trash`：管理员查看已软删除内容
- 新增 `PATCH /admin/contents/:id/restore`：恢复软删除内容
- 定时任务：软删除 30 天后永久删除（管理后台手动触发或后台 cron）

### 15.9 数据库 Schema 变更汇总（Task 99-145 新增）

> 注：以下为原始计划编号。实际迁移文件编号因并行开发已发生偏移，最新编号见 `backend/migrations/` 目录（001-056）。
> 关键错位：计划 038=全文搜索 → 实际 038=notification_channels, 041=content_search_vector；
> 计划 039=密码重置 → 实际 039=conversation_unread_count；
> 计划 040=P0 修复（含 download_count/ban_reason/email_verified_at）；
> 计划 041=内容软删除 → 实际 053=content_items_soft_delete。

```sql
-- P0 修复（实际迁移 040_p0_fixes.sql）
ALTER TABLE content_items ADD COLUMN ban_reason TEXT;
ALTER TABLE content_items ADD COLUMN download_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;

-- 全文搜索（实际迁移 041_content_search_vector.sql）
ALTER TABLE content_items ADD COLUMN search_vector tsvector;
CREATE INDEX idx_content_items_search ON content_items USING GIN(search_vector);

-- 密码重置（实际迁移 050_verification_and_terms.sql，与其他验证功能合并）
CREATE TABLE password_reset_tokens (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token       VARCHAR(255) UNIQUE NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ
);
CREATE INDEX idx_password_reset_token ON password_reset_tokens(token, expires_at);
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;

-- 内容软删除（实际迁移 053_content_items_soft_delete.sql）
ALTER TABLE content_items ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX idx_content_items_deleted ON content_items(deleted_at) WHERE deleted_at IS NOT NULL;

-- User.ID 类型统一（uint → int64）
-- 注意：此迁移需要将 users.id 从 SERIAL 改为 BIGSERIAL（若当前为 INTEGER）
-- 所有引用 users.id 的外键列同步改为 BIGINT
```

### 15.10 新增 API 路由汇总（Task 99-145）

> 路由已合并至 §3.2 API 路由清单。本节仅保留作为 Task 99-145 的历史范围参考。

### 15.11 结构化日志与优雅关机

- 使用 Go 1.21+ 内置 `slog` 包替换 `fmt.Printf`
- 新增 `internal/middleware/request_id.go`：为每个请求生成唯一 `X-Request-ID`（UUID）
- 新增 `internal/middleware/logging.go`：使用 slog.Info 记录请求方法、路径、状态码、耗时、Request-ID
- 服务启动使用 `http.Server` + `srv.Shutdown(ctx)` 支持优雅关机（15秒超时）
- 配置修改持久化到 `data/config_override.yaml`，启动时合并加载

### 15.12 前端数据缓存与暗色模式修复

- 引入 SWR 或 @tanstack/react-query 作为全局数据缓存层，避免每个页面独立 fetch
- AuthContext 使用 SWR 的 mutate 实现登录/登出后缓存失效
- Sidebar.tsx、StudioSidebar.tsx、Footer.tsx 中所有硬编码颜色替换为 Tailwind 语义化类（bg-white → bg-background 等）
- 新增 `(protected)/layout.tsx` 统一鉴权守卫
- 硬编码中文字符串全部提取到 `messages/zh.json` 和 `messages/en.json`
- 首页/原创区统计数字从 API 动态获取（`GET /stats/summary`）
- 注册成功后直接使用返回 token，不再二次调用 login()
- API 层 401 响应自动尝试 refresh token，多并发请求仅触发一次 refresh
