# OmniCraft 数据库 × 部署 × 性能审查报告

**审查日期**: 2026-05-22  
**审查范围**: backend/migrations/ (42 files), backend/internal/model/ (15 files), docker-compose.yml, nginx/nginx.conf, backend/config.yaml, .env.example, backend/ 全量 Redis 操作, backend/ 全量数据库查询

---

## 1. 迁移文件编号审查

42 个迁移文件 (`001_users.sql` ~ `042_ips_search_vector.sql`)，**无编号跳跃，无重复**。

### 冗余迁移（幂等保护已到位）

| 迁移 | 内容 | 重复迁移 | 保护 |
|------|------|---------|------|
| 014_judge.sql | 创建 `judge_reason_votes` | 029_judge_reason.sql | IF NOT EXISTS |
| 001_users.sql | 创建 `support_info` / `preferred_locale` | 014 / 030 | IF NOT EXISTS |
| 006_content_items.sql | 创建 `category` | 027_content_category.sql | IF NOT EXISTS |

非关键，但说明迁移历史不够干净，建议后续大版本合并清理。

---

## 2. Schema 不匹配：迁移 SQL vs GORM Model

### CRITICAL — 模型有字段但迁移缺失

| 模型 | 缺失列 | 文件 |
|------|--------|------|
| **Comment** | `target_type VARCHAR(20)`, `target_id BIGINT`, `content TEXT`, `updated_at TIMESTAMPTZ` | `internal/model/social.go` |
| **ContentItem** | `ban_reason TEXT` (迁移 040 加了但 model 未声明) | `internal/model/content.go` |

### HIGH — 类型/默认值不一致

| 表 | 列 | Migration | GORM Model | 差异 |
|----|-----|-----------|------------|------|
| `users` | `avatar_url` | `TEXT` | `size:2048` | TEXT vs VARCHAR(2048) |
| `users` | `preferred_locale` | `DEFAULT 'zh-CN'` | `default:'zh'` | 默认值冲突 |
| `users` | `support_info` | `NOT NULL DEFAULT '{}'` | `default:'{}'` (可为空) | NOT NULL 缺失 |
| `content_items` | `search_vector` | `tsvector` (041) | 无对应字段 | GORM 不感知 |
| `ips` | `search_vector` | `tsvector` (042) | 无对应字段 | GORM 不感知 |

### MEDIUM — 约束未映射到模型

| 表 | 约束 | 迁移 | GORM Model |
|----|------|------|------------|
| `follows` | target_type CHECK | `IN ('user','ip')` (033) | 无 CHECK tag |
| `appeals` | target_type CHECK | `IN ('content','comment')` (034) | 无 CHECK tag |
| `appeals` | status CHECK | `IN ('pending','approved','rejected')` (034) | 无 CHECK tag |
| `notifications` | channel CHECK | `IN ('reply','like','system','pr','follow')` (038) | 无 CHECK tag |

### INFO — 缺失的整个模块

| 功能 | 状态 |
|------|------|
| `collections` / `collection_items` | CLAUDE.md 已定义模型和 API，但无迁移、无 GORM model |
| `password_reset_tokens` | 迁移 040 建表，但无 GORM model（handler 用 Redis 替代） |
| `search_suggestions` | CLAUDE.md Task 120 提及，但无迁移、无 model |

---

## 3. Docker Compose 审查

### 服务清单

| 服务 | 镜像 | 端口 | 状态 |
|------|------|------|------|
| postgres | `pgvector/pgvector:pg16` | 5432:5432 | OK |
| pgbouncer | `./pgbouncer/Dockerfile` | 6432:5432 | OK |
| redis | `redis:7-alpine` | 6379:6379 | OK |
| backend | `./backend/Dockerfile` | 8080:8080 | OK |
| frontend | `./frontend/Dockerfile` | 3000:3000 | OK |
| nginx | `nginx:alpine` | 80:80, 443:443 | OK |

### 健康检查覆盖

| 服务 | 健康检查 | 状态 |
|------|---------|------|
| postgres | `pg_isready -U omnicraft` (间隔 10s, 重试 5 次) | OK |
| redis | `redis-cli ping` (间隔 10s, 重试 5 次) | OK |
| pgbouncer | **无** | **缺失** |
| backend | **无** | **缺失** |
| frontend | **无** | **缺失** |
| nginx | **无** | **缺失** |

### 卷挂载和数据持久化

| 卷 | 类型 | 状态 |
|----|------|------|
| `postgres_data:/var/lib/postgresql/data` | 命名卷 | OK |
| `redis_data:/data` | 命名卷 + AOF | OK |
| `./backend/migrations:/docker-entrypoint-initdb.d` | 绑定挂载 | OK |
| `./pgbouncer/pgbouncer.ini:/etc/pgbouncer/pgbouncer.ini:ro` | 只读 | OK |
| `./nginx/nginx.conf:/etc/nginx/nginx.conf:ro` | 只读 | OK |
| SSL 证书 `/etc/letsencrypt/...` | **缺失** | nginx.conf 引用但未挂载 |

### 其他 Docker 问题

- **无资源限制**: 所有容器无 `mem_limit` / `cpus` 设置
- **无网络隔离**: 所有服务在默认网络
- **backend → pgbouncer**: `condition: service_started`（非 `service_healthy`），pgbouncer 本身无健康检查
- **nginx → backend/frontend**: `condition: service_started`，可能在后端就绪前接受流量

---

## 4. Nginx 配置审查

### 反向代理路由

| 路径 | 目标 | 缓存 | 备注 |
|------|------|------|------|
| `/_next/static/` | frontend:3000 | 365d + immutable | OK |
| `/static/` | frontend:3000 | 30d | OK |
| `/images/` | frontend:3000 | 7d | OK |
| `/api/` | backend:8080 | 无 | read_timeout 300s |
| `/` | frontend:3000 | 无 | WebSocket upgrade |

### SSL/TLS

- TLS 1.2/1.3 only，ECDHE + GCM 密码套件
- OCSP Stapling 已开启
- HSTS 2 年 + includeSubDomains + preload
- **证书文件未挂载** — nginx 启动将失败

### 安全头

5 个安全头全部开启: HSTS, X-Content-Type-Options, X-Frame-Options, X-XSS-Protection, Referrer-Policy

### Nginx 缺失项

| 功能 | 状态 |
|------|------|
| Rate Limiting (`limit_req_zone` / `limit_req`) | **缺失** |
| CORS 头 | nginx 层无，依赖应用层 |
| 自定义错误页 (`error_page`) | **缺失** |
| Upstream 健康检查 (`max_fails` / `fail_timeout`) | **缺失** |
| `proxy_buffering` 显式配置 | 使用默认值 |
| 弃用语法 | `http2 on;` 已弃用，应改为 `listen 443 ssl http2;` |

---

## 5. Redis Key 审查

### 命名规范问题

**不一致的 key（混用 `_` 和 `:`）:**

| 当前 Key | 文件 | 建议改为 |
|----------|------|---------|
| `reset_token:{token}` | `internal/handler/auth.go` | `reset:token:{token}` |
| `verify_email:{token}` | `internal/handler/auth.go` | `verify:email:{token}` |
| `publish_freeze:{userID}` | `internal/service/review_service.go` | `publish:freeze:{userID}` |
| `tag_suggest:{uid}:{cid}:{date}` | `internal/service/tag_service.go` | `tag:suggest:{uid}:{cid}:{date}` |
| `ip_category_counts` | `internal/service/ip_stats_service.go` | `ip:category:counts` |
| `rank:download_counts` | `internal/handler/content.go` | `rank:download:counts` |
| `agent:rl:{uid}:{date}` | `internal/middleware/agent_ratelimit.go` | `agent:ratelimit:{uid}:{date}` |

### TTL 审计

| TTL 类型 | 数量 | 位置 |
|----------|------|------|
| **配置驱动** | 5 | content/ip detail/list TTL, rec refresh interval |
| **硬编码** | 12 | auth, ratelimit, stats, tag, review |

硬编码 TTL 明细:

| TTL | 文件 | 用途 |
|-----|------|------|
| 60s | `middleware/ratelimit.go` | 评论编辑限速 |
| 2min | `middleware/ratelimit.go` | IP / 凭证限速 |
| 2h | `middleware/ratelimit.go` | 上传限速 |
| 5min | `middleware/auth.go` | 用户状态缓存 |
| 10min | `service/stats_service.go` | 统计摘要缓存 |
| 1h | `handler/auth.go` | 重置 token |
| 24h | `handler/auth.go` | 邮箱验证 |
| 24h | `pkg/queue/idempotent.go` | 队列幂等 |
| 25h | `middleware/agent_ratelimit.go` | Agent 限速 |
| 48h | `service/tag_service.go` | 标签建议限速 |
| 7d | `service/review_service.go` | 发布冻结 |

### 无 TTL 的持久 key — 内存风险

| Key | 类型 | 风险 |
|-----|------|------|
| `rank:hot:contents` | ZSet | 仅 ZIncrBy，永不过期 |
| `rank:hot:ips` | ZSet | 仅 ZIncrBy，永不过期 |
| `rank:download_counts` | ZSet | 仅 ZIncrBy，永不过期 |
| `ip_category_counts` | Hash | 仅 HIncrBy，永不过期 |
| `view_count:{id}` | String | 定时刷新后 GetDel 清理 — OK |

---

## 6. 数据库查询风险

### SQL 注入风险

**`internal/repository/search_repo.go:100-108`** — 字符串拼接:

```go
if zone != "" {
    filterClause += " AND content_items.zone = '" + zone + "'"
}
if category != "" {
    filterClause += " AND content_items.category = '" + category + "'"
}
if contentType != "" {
    filterClause += " AND content_items.content_type = '" + contentType + "'"
}
```

三个值来自请求参数，未经校验直接拼入 SQL。建议改为参数化查询。

### N+1 查询

**`internal/repository/search_repo.go:125`** — 循环逐条加载:

```go
for i := range results {
    r.db.Where("id = ?", results[i].AuthorID).First(&results[i].Author)
}
```

应改为 `Preload("Author")` 或批量加载。

### 缺少索引的高风险查询

| 优先级 | 表 | 查询模式 | 建议索引 |
|--------|-----|---------|---------|
| **P0** | `follows` | `WHERE target_type=? AND target_id=? AND created_at>=?` | `(target_type, target_id, created_at)` |
| **P0** | `content_items` | `WHERE zone=? AND status=?` | `(zone, status)` 复合索引 |
| **P0** | `content_items` | `WHERE title ILIKE '%?%'` | `pg_trgm` GIN 索引 |
| **P1** | `browse_history` | `WHERE user_id=? ORDER BY viewed_at DESC` | `(user_id, viewed_at DESC)` |
| **P1** | `users` | `WHERE username ILIKE ?` | `pg_trgm` GIN 索引 |
| **P1** | `tags` | `WHERE name ILIKE ?` | `pg_trgm` GIN 索引 |
| **P1** | `content_items` | `search_vector @@ to_tsquery(...)` | GIN 索引 (迁移 041 已建) |
| **P1** | `ips` | `search_vector @@ to_tsquery(...)` | GIN 索引 (迁移 042 已建) |
| **P1** | `reactions` | `WHERE user_id=? AND target_type=? AND reaction=?` | `(user_id, target_type, reaction)` 复合索引 |
| **P1** | `conversation_participants` | `WHERE user_id=? AND left_at IS NULL` | 部分索引 `(user_id) WHERE left_at IS NULL` |
| **P2** | `discussions` | `WHERE to_tsvector(...) @@ plainto_tsquery(...)` | 需先建 tsvector 列 + GIN 索引 |
| **P2** | `content_items.category` | `WHERE category = (subquery)` | `category` 列 B-tree 索引 |
| **P2** | `content_tags.tag` | `JOIN ON ct.tag = t.name` | `tag` 列索引 (text join) |

### 无法使用索引的排序

| 文件 | 排序表达式 |
|------|-----------|
| `internal/repository/content_repo.go:129` | `ORDER BY (view_count + like_count * 3) DESC` |
| `internal/repository/content_repo.go:130` | `ORDER BY (like_count::float / NULLIF(like_count + dislike_count, 0)) DESC` |

大数据量下会退化为全表扫描 + 排序。建议使用物化视图或定时更新的排序列。

---

## 7. 配置 & 环境变量覆盖链

### CRITICAL — 编译阻塞

| # | 问题 | 详情 |
|---|------|------|
| C1 | **AgentConfig struct 缺失** | `config.go:29` 声明 `Agent AgentConfig`，但类型定义不存在。代码中大量引用 `cfg.Agent.WebAgentEnabled` / `cfg.Agent.LLMAPIKey` 等 — **项目无法编译** |
| C2 | **PublishConfig 缺失** | config.yaml 中 `publish:` 块无对应 Go struct，值静默丢弃 |

### Queue 配置结构不匹配

config.yaml:
```yaml
queue:
  worker_count:
    content_review: 2
    notification: 1
    embedding: 1
    count: 1
```

QueueConfig struct 期望:
```go
WorkerReview    int `mapstructure:"worker_review"`
WorkerNotif     int `mapstructure:"worker_notification"`
WorkerEmbedding int `mapstructure:"worker_embedding"`
WorkerCount     int `mapstructure:"worker_count"`
```

嵌套 map vs 平铺字段 — **四个 worker count 全部无法加载**。

### .env.example 缺失项

7 个在 `overrideFromEnv()` 中处理的 env var 无示例:

| 缺失 env var | 用途 |
|-------------|------|
| `AGENT_LLM_API_KEY` | LLM API 密钥 |
| `AGENT_LLM_PROVIDER` | LLM 提供商类型 |
| `AGENT_LLM_MODEL` | LLM 模型名 |
| `AGENT_LLM_API_BASE` | LLM API 端点 |
| `AGENT_HMAC_SECRET` | Agent HMAC 签名密钥 |
| `GREEN_CALLBACK_URL` | 阿里云内容安全回调地址 |
| `GREEN_CALLBACK_ALLOWED_IPS` | 回调 IP 白名单 |
> 修正（2026-08-13）：`GREEN_CALLBACK_ALLOWED_IPS`（回调来源 IP 白名单）前提证伪——阿里云官方从未发布回调来源网段，该配置已随 #104 全链路删除；入站认证为 form+checksum 契约（#106），release gate 改校验 `GREEN_SEED`/`GREEN_UID`。

### 其他配置缺口

| 问题 | 详情 |
|------|------|
| `redis.db` 无 env var | `REDIS_ADDR` / `REDIS_PASSWORD` 有覆盖，`REDIS_DB` 缺失 |
| `agent.embedding_model` | config.yaml 有定义但无 Go 代码消费 |
| `agent.embedding_dimensions` | config.yaml 有定义但无 Go 代码消费 |

---

## 8. 问题总清单

### CRITICAL — 阻塞编译 / 安全漏洞

| # | 类别 | 文件 | 问题 |
|---|------|------|------|
| C1 | 配置 | `config/config.go:29` | `AgentConfig` struct 缺失，项目无法编译 |
| C2 | 安全 | `repository/search_repo.go:100-108` | SQL 注入 — zone/category/contentType 字符串拼接 |
| C3 | 配置 | `config/config.go` vs `config.yaml` | `queue.worker_count` 结构不匹配，worker 数全部无法加载 |
| C4 | 配置 | `config/config.go` | `publish:` 配置块无 Go struct，静默丢弃 |

### HIGH — 线上风险

| # | 类别 | 文件 | 问题 |
|---|------|------|------|
| H1 | Schema | `model/social.go` vs migrations | Comment 模型有 4 字段无对应迁移: `target_type`, `target_id`, `content`, `updated_at` |
| H2 | Schema | `model/user.go` vs 001_users.sql | `preferred_locale` 默认值不一致: migration `zh-CN` vs model `zh` |
| H3 | Schema | `model/user.go` vs 001_users.sql | `avatar_url` 类型不一致: migration `TEXT` vs model `VARCHAR(2048)` |
| H4 | Schema | `model/user.go` vs 001_users.sql | `support_info` NOT NULL 缺失 |
| H5 | Docker | `docker-compose.yml` | backend/frontend/nginx/pgbouncer 无健康检查 |
| H6 | Docker | `docker-compose.yml` | SSL 证书未挂载，nginx 无法启动 |
| H7 | Nginx | `nginx/nginx.conf` | 全站无 rate limiting |
| H8 | 性能 | `repository/search_repo.go:125` | N+1 查询 — 循环逐条加载 Author |
| H9 | 性能 | `follows` 表 | 缺少 `(target_type, target_id, created_at)` 复合索引 |
| H10 | 性能 | `content_items` 表 | 缺少 `(zone, status)` 复合索引 |
| H11 | Redis | 12 处硬编码 | TTL 值分散在 8 个文件中，应统管到 config.yaml |
| H12 | Redis | 3 个 ZSet key | `rank:hot:contents` / `rank:hot:ips` / `rank:download_counts` 无 TTL，内存无限增长 |
| H13 | 配置 | `.env.example` | 7 个 env var 无示例条目 |

### MEDIUM — 改进建议

| # | 类别 | 文件 | 问题 |
|---|------|------|------|
| M1 | 性能 | `content_items`/`users`/`tags` | ILIKE 查询缺少 pg_trgm GIN 索引 |
| M2 | 性能 | `repository/content_repo.go:129-130` | 计算列排序无法使用索引 |
| M3 | Schema | 无 | `collections` / `collection_items` 表文档已定义但未实现 |
| M4 | Redis | 7 处 | key 命名不规范 (混用 `_` 和 `:`) |
| M5 | Docker | `docker-compose.yml` | 所有容器无 CPU/内存限制 |
| M6 | Docker | `docker-compose.yml` | backend → pgbouncer 仅 `service_started` 而非 `service_healthy` |
| M7 | Nginx | `nginx/nginx.conf` | `http2 on;` 语法已弃用 (应并入 listen 指令) |
| M8 | Nginx | `nginx/nginx.conf` | 无 CORS 头 (完全依赖应用层) |
| M9 | Nginx | `nginx/nginx.conf` | upstream 无 `max_fails` / `fail_timeout` |
| M10 | 配置 | `config/config.go` | `redis.db` 无 env var 覆盖 |
| M11 | 配置 | `config.yaml` | `agent.embedding_model` / `agent.embedding_dimensions` 孤儿字段 |

### LOW — 优化建议

| # | 类别 | 文件 | 问题 |
|---|------|------|------|
| L1 | Schema | 4 个表 | CHECK 约束在迁移中有但 GORM model 未声明 |
| L2 | Schema | `content_items` / `ips` | `search_vector` 列迁移中有但 model 未声明 (可能有意为之) |
| L3 | Docker | `docker-compose.yml` | 默认网络无隔离分段 |
| L4 | 冗余 | 迁移 014/029, 001/014/030, 006/027 | 3 对迁移有重复 DDL (IF NOT EXISTS 已保护) |
| L5 | Nginx | `nginx/nginx.conf` | 无自定义错误页 |

---

## 9. 修复优先级建议

### 第一轮 (立即修复)

1. **定义 `AgentConfig` struct** — 解除编译阻塞
2. **修复 `search_repo.go` SQL 注入** — 改为参数化查询
3. **修复 `queue.worker_count` 配置结构** — 或改 yaml 为平铺 key，或改 struct 为嵌套 map
4. **添加 `PublishConfig` struct** — 让 `publish` 配置生效

### 第二轮 (上线前)

5. 补齐 Comment 模型缺失的 4 个迁移列
6. 对齐 `users` 表字段类型/默认值差异
7. 添加 backend/frontend/nginx 健康检查
8. 补全 SSL 证书挂载或提供开发模式禁用 SSL 的配置
9. 添加 Nginx rate limiting
10. 创建 `follows` 和 `content_items` 的关键复合索引

### 第三轮 (迭代优化)

11. TTL 值统一到 config.yaml
12. Redis key 命名规范化
13. N+1 查询改为 Preload
14. 添加 pg_trgm GIN 索引
15. 补全 .env.example
