# 文档修复验证新发现问题清单（累计）

**记录日期**: 2026-06-28
**来源**: 多轮修复验证过程中新发现的问题（详见各轮验证报告）
**用途**: 作为后续"新问题修复任务"的输入清单，避免遗漏
**状态**: 部分已处理，部分待处理
**更新历史**:
- 2026-06-28 第一轮验证新增 N1-N3
- 2026-06-28 第二轮验证新增 N4-N5
- 2026-06-28 第三+四轮验证新增 N6-N9

---

## 一、问题清单

### N1. 根 docker-compose.yml backend 健康检查使用业务接口（反模式）

| 字段 | 内容 |
|------|------|
| **严重度** | 🟡 警告 |
| **位置** | [docker-compose.yml:88](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docker-compose.yml#L88) |
| **紧迫性** | 🔴 **立即处理** — 必须纳入第二轮 D3 修复范围 |
| **当前值** | `wget -qO- http://localhost:8080/api/v1/stats/summary` |
| **期望值** | `wget -qO- http://localhost:8080/healthz`（与 D3 统一端点一致） |
| **问题描述** | 根 docker-compose.yml 的 backend 健康检查使用业务接口 `/api/v1/stats/summary` 而非专用健康端点。这违反"健康检查应使用专用轻量端点"的最佳实践：①业务接口变更会意外破坏健康检查；②业务接口可能因鉴权/限流导致健康检查失败；③业务接口响应体较大，浪费健康检查开销。 |
| **影响** | 若 D3 仅修复 single-server.yml 和运维文档，根 docker-compose.yml 仍残留反模式，开发环境健康检查仍不规范。 |
| **修复建议** | 在第二轮 D3 修复时，一并处理根 `docker-compose.yml:88`，统一为 `/healthz`。注意：根 compose 是开发环境，后端实际注册的是 `/health`（[main.go:73](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/cmd/server/main.go#L73)），D3 修复需同时调整后端注册端点或在 nginx/healthcheck 中映射。 |

---

### N2. constitution 1.3.0 Changelog 漏列 Toolchain 变更归属 ~~（已撤销）~~

| 字段 | 内容 |
|------|------|
| **状态** | ❌ **撤销——经核实不成立** |
| **撤销日期** | 2026-06-28 |
| **撤销理由** | 经用户核实，"via PgBouncer connection pool" 描述在 constitution 1.2.0 版本中已存在于 L30，1.3.0 仅变更了版本号（15+→16+），未变更"via PgBouncer"描述。Changelog 的作用是记录"本次版本变更了什么"，1.3.0 既未变更"via PgBouncer"描述，Changelog 无需提及。原始判断混淆了"状态描述"与"变更记录"，N2 不成立。 |
| **当前 Changelog 表述** | "PostgreSQL minimum version raised from 15+ to 16+ (matching `AGENTS.md` and `docker-compose.yml`)." —— 准确反映 1.3.0 实际变更内容，无需修改。 |

---

### N3. architecture.md §7 reputation 段未同步 A1 新增的 13 个 score_* 字段

| 字段 | 内容 |
|------|------|
| **严重度** | 🟡 警告 |
| **位置** | [architecture.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md) §7 reputation 配置段（约第 1247-1269 行附近） |
| **紧迫性** | 🟡 中 — 第 4 轮 H 系列处理时一并修复 |
| **当前状态** | A1 已在 [config.yaml:103-115](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/config.yaml#L103-L115) 和 [config.go:117-129](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/config/config.go#L117-L129) 补齐 13 个 `score_*` 字段，但 architecture.md §7 reputation 段仍只描述原 6 个字段，未同步新增的 13 个字段。 |
| **影响** | architecture.md §7 与 config.yaml/config.go 三方脱节，违反"配置字段注册表"机制。开发者读架构文档无法知道这些字段存在，可能误判为未实现。与 H 系列（architecture.md §7 配置字段脱节）属同类问题。 |
| **修复建议** | 在第 4 轮 H 系列修复时，将这 13 个 `score_*` 字段补入 architecture.md §7 reputation 段，与 H1-H8 一并处理。字段清单参见 [config.yaml:103-115](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/config.yaml#L103-L115)。 |

### N4. production-config-template.md 第 174 行健康检查命令遗漏

| 字段 | 内容 |
|------|------|
| **严重度** | 🟡 警告 |
| **位置** | [production-config-template.md:174](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/deploy/production-config-template.md#L174) |
| **紧迫性** | 🟢 低 — 第 9 轮新问题修复时一行修正 |
| **当前值** | `curl.exe https://api.leeppp.online/health` |
| **期望值** | `curl.exe https://api.leeppp.online/healthz` |
| **问题描述** | 第二轮 D3 修复时遗漏了 production-config-template.md 第 174 行的 curl 验证命令。该行位于 "Quick Verification Commands" 章节，按当前后端实际注册路径（`/healthz`）执行将返回 404。 |
| **影响** | 仅影响文档验证命令的可用性，不影响运行时行为（运行时 healthcheck、nginx 路由、后端注册都已正确指向 /healthz）。 |
| **修复建议** | 一行修正 `/health` → `/healthz`。 |

---

### N5. architecture.md §3.2 路由清单未补全 feedback/audit-logs API 路由

| 字段 | 内容 |
|------|------|
| **严重度** | 🟡 警告 |
| **位置** | [architecture.md §3.2](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md) 行 279-401（路由清单） |
| **紧迫性** | 🟡 中 — 建议第 4 轮 H 系列处理时一并修复 |
| **当前状态** | 第二轮 C2 修复补全了 §4.11/§4.12 表设计，但 §3.2 路由清单未补全对应 API 路由。`/admin/audit-logs` 全文零匹配。 |
| **影响** | architecture.md 仍无法作为 API 完整参考；与 routes.go 第 247-254、360、364 行实际存在的路由脱节。 |
| **应补全的 7 条 API 路由**（参考 routes.go 实际注册） | 1. `POST /api/v1/feedback` — 用户提交反馈工单<br>2. `GET /api/v1/feedback/me` — 我的反馈列表<br>3. `GET /api/v1/feedback/:id` — 反馈详情<br>4. `GET /api/v1/admin/feedback` — 管理员反馈列表<br>5. `POST /api/v1/admin/feedback/:id/reply` — 管理员回复<br>6. `PATCH /api/v1/admin/feedback/:id` — 更新状态/优先级/分配<br>7. `GET /api/v1/admin/audit-logs` — 审计日志查询 |
| **修复建议** | 第 4 轮 H 系列处理 architecture.md 时一并补全（与 H 系列同属 architecture.md 修复，避免重复打开文件）。 |
| **当前状态（第 4 轮后）** | ✅ 已修复——补全 9 条路由（超出预期 7 条），与 routes.go 完全一致 |

---

### N6. architecture.md §7 多段引入"幽灵字段"（严重）

| 字段 | 内容 |
|------|------|
| **严重度** | 🔴 严重 |
| **来源** | 第 4 轮 H5 修复引入 |
| **位置** | [architecture.md §7](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md) cache/queue/rate_limit/security/oss 段 |
| **紧迫性** | 🔴 **高 — 第 5 轮优先处理** |
| **问题定义** | "幽灵字段"：文档 §7 中出现的字段，在 [config.go](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/config/config.go) 对应结构体中根本不存在 |
| **问题描述** | 第 4 轮 H5 修复（补充 9 个配置段说明）时，多个段的字段并非从 config.go 结构体拷贝，而是凭印象编写，导致引入了 config.go 中根本不存在的字段，且字段类型也错误。这比 H5 修复前更严重——修复前是"字段缺失"，修复后是"字段错误"。 |
| **影响** | 开发者按文档配置这些字段会被 viper 静默忽略，功能不生效且难以排查。H5 修复本意是消除 H8 脱节，反而加重了 H8 问题。 |
| **幽灵字段清单** | |

| 段 | 幽灵字段（config.go 无） | 缺失字段（config.go 有但文档无） | 类型/值错误 |
|----|------------------------|-------------------------------|-----------|
| cache | `embed_cache_max_mb`、`embed_cache_ttl_h` | 全部 11 个实际字段（content_list_ttl/content_detail_ttl/ip_list_ttl/ip_detail_ttl/view_count_flush_interval/hot_rank_zset_ttl/user_status_ttl/tag_cache_ttl/email_verify_ttl/password_reset_ttl/publish_freeze_ttl） | — |
| queue | `max_retries` | `dlq_ttl_hours`/`maxlen`/`worker_review`/`worker_notification`/`worker_embedding`/`worker_count` | `max_retries` 应为 `max_attempts` |
| queue | — | — | `retry_backoff_sec: 60` 应为数组 `[10, 60, 300]` |
| rate_limit | `global_rps`、`comment_edit_per_hour` | `enabled`/`normal_per_minute`/`normal_window_sec`/`upload_window_sec`/`agent_window_sec`/`max_json_body_bytes`/`max_query_chars`/`max_search_limit`/`max_search_page` | — |
| security | `csrf_enabled`、`frame_ancestors` | `trusted_proxies` | — |
| oss | `presign_expire_sec` | `endpoint`/`access_key_id`/`access_key_secret`/`bucket_name`/`domain` | — |
| green | — | `access_key_id`/`access_key_secret`/`callback_url`/`callback_allowed_ips` | — |
> 修正（2026-08-13）：`callback_allowed_ips` 前提证伪——阿里云官方从未发布回调来源网段，该字段已随 #104 删除；配置段现为 `access_key_id`/`access_key_secret`/`callback_url`/`region`/`seed`/`uid`。
| captcha | — | `prefix`/`access_key_id`/`access_key_secret` | — |
| publish | — | `require_review`/`max_daily_posts`/`freeze_on_violation` | — |

| **修复建议** | 第 5 轮优先重写 architecture.md §7 的 cache/queue/rate_limit/oss/green/captcha/security/publish 段，严格按 [config.go](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/config/config.go) 结构体字段对齐，删除所有幽灵字段、补齐缺失字段、修正类型错误。修复后必须执行字段逐一核对。 |

---

### N7. ui-spec.md token 引用形式系统性错误（严重）

| 字段 | 内容 |
|------|------|
| **严重度** | 🔴 严重 |
| **来源** | 第 4 轮 J 系列修复引入 |
| **位置** | [design/ui-spec.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/design/ui-spec.md) 全文（30+ 处）+ [design/design-system.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/design/design-system.md) |
| **紧迫性** | 🔴 **高 — 第 5 轮优先处理** |
| **问题描述** | ui-spec.md 大量使用**点号形式**引用 token（如 `bg-canvas.default`、`text-fg.muted`、`border-border.default`、`ring-accent.emphasis`），但：①Tailwind CSS 类名不支持点号分隔符，这些不是有效类名；②[design-system.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/design/design-system.md) 定义的是**连字符形式**（`--canvas-default`、`--fg-muted`）；③ui-spec.md 引用了 design-system.md 未定义的 token。 |
| **影响** | 前端开发者按 ui-spec.md 写出的 className 会无效，需自行转换为连字符形式，违背"ui-spec 为唯一视觉权威"的设计目标。J1/J3/J7/J8 本质是同一系统性问题的不同表现。 |
| **未定义的 token** | ui-spec.md 使用但 design-system.md 未定义：`border.muted`/`--border-muted`（15+ 处）、`canvas.default.dark`/`--canvas-default-dark`（17 处）、`border.default.dark`/`--border-default-dark`（17 处） |
| **修复建议** | 第 5 轮建立 ui-spec.md 与 design-system.md 的 token 命名映射约定，统一为连字符形式（`bg-canvas-default` 或 `bg-[var(--canvas-default)]`），并在 design-system.md 补定义缺失 token，或在 ui-spec.md 中替换为已定义 token。 |

---

### N8. architecture.md §11.3 agent 段未与 §7 同步

| 字段 | 内容 |
|------|------|
| **严重度** | 🟡 警告 |
| **来源** | 第 4 轮 I16 修复遗留 |
| **位置** | [architecture.md §11.3](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md#L2050-L2061) agent 段 |
| **紧迫性** | 🟡 中 — 第 5 轮或第 9 轮处理 |
| **问题描述** | §7 agent 段已在第 4 轮补全 12 个字段，但 §11.3 仍是 9 个字段，缺失 3 个：`max_user_message_chars`、`chat_max_context_messages`、`hmac_secret`。同文件内同字段不同步。 |
| **影响** | 同一文档内 agent 配置两处描述不一致，开发者可能误信 §11.3 而遗漏 3 个字段。 |
| **修复建议** | 在 §11.3 agent 段补 3 个字段，与 §7 完全对齐。 |

---

### N9. 2 份旧 iteration-review 报告被删除

| 字段 | 内容 |
|------|------|
| **严重度** | 🟢 建议 |
| **来源** | 第 3+4 轮 git status 观察到 |
| **位置** | `docs/iteration-review/2026-06-25-iteration-review-report(1).md`（477 行）、`docs/iteration-review/2026-06-25-iteration-review.md`（389 行） |
| **紧迫性** | 🟢 低 — 仅需确认 |
| **问题描述** | git status 显示这两个文件被删除（D 标记），需确认是否为预期行为。可能是被新版报告替代，但删除旧报告应在 archive 而非直接删除，以保留历史可追溯性。 |
| **影响** | 若为预期删除则无影响；若为误删则会丢失历史审查记录。 |
| **修复建议** | 确认删除意图。若需保留历史，可从 git 历史恢复并移至 `docs/archive/`；若确认无需保留则关闭本项。 |

---

## 二、问题与修复轮次映射

| 问题 | 严重度 | 建议处理轮次 | 处理方式 | 状态 |
|------|--------|-------------|---------|------|
| N1 | 🟡 警告 | 第 2 轮（D3 修复时一并） | 立即纳入 D3 修复范围 | ✅ 已落地（2026-06-28 二次确认） |
| N2 | 🟢 建议 | ~~第 5 轮~~ | ~~扩展 Toolchain changelog 描述~~ | ❌ 撤销（经核实不成立） |
| N3 | 🟡 警告 | 第 4 轮（H 系列处理时） | 补 13 个 score_* 字段说明 | ✅ 已修复（第 4 轮，与 config.yaml 完全一致） |
| N4 | 🟡 警告 | 第 9 轮或顺手修正 | 一行修正 `/health` → `/healthz` | 待处理 |
| N5 | 🟡 警告 | 第 4 轮（H 系列处理时） | 补 7 条 API 路由到 §3.2 | ✅ 已修复（第 4 轮，补 9 条与 routes.go 一致） |
| N6 | 🔴 严重 | **第 5 轮（P0 优先）** | 重写 §7 多段，删除幽灵字段，补齐缺失字段 | 待处理 |
| N7 | 🔴 严重 | **第 5 轮（P0 优先）** | 统一 token 命名约定，补定义缺失 token | 待处理 |
| N8 | 🟡 警告 | 第 5 轮或第 9 轮 | §11.3 agent 段补 3 个字段 | 待处理 |
| N9 | 🟢 建议 | 第 5 轮确认 | 确认删除意图 | 待确认 |

---

## 三、立即处理项说明（N1）

**必须在第二轮 D3 修复时一并处理**，否则 D3 修复完成后仍会残留反模式，需要返工。

### N1 修复要点

1. **后端实际注册端点**：[main.go:73](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/cmd/server/main.go#L73) `r.GET("/health", ...)` — 当前唯一注册端点是 `/health`
2. **D3 修复决策**：用户计划统一为 `/healthz`（与 nginx 配置一致）
3. **N1 关联修复**：D3 修复时需同时处理：
   - 后端 main.go 注册端点 `/health` → `/healthz`（或保留 `/health` 并在 nginx 映射）
   - single-server.yml:76 backend healthcheck `/health` → `/healthz`
   - **根 docker-compose.yml:88 backend healthcheck `/api/v1/stats/summary` → `/healthz`**（N1）
   - 运维文档（runbook:186、production-config-template:174）`/health` → `/healthz`
4. **验证要点**：修复后 grep `health|healthz` 在 `docs/deploy/` 和根 `docker-compose.yml` 应全部为 `/healthz`，无 `/health` 或 `/api/v1/stats/summary` 残留

---

## 四、跟踪机制

本文档作为"新问题修复任务"的输入清单，随每轮验证持续更新。

### 当前状态汇总（截至第 4 轮验证后）

| 状态 | 数量 | 问题编号 |
|------|------|---------|
| ✅ 已修复/已落地 | 3 | N1、N3、N5 |
| ❌ 撤销（不成立） | 1 | N2 |
| 🔴 待处理（P0 优先） | 2 | N6、N7 |
| 🟡 待处理 | 2 | N4、N8 |
| 🟢 待确认 | 1 | N9 |

### 处理路径建议

1. **第 5 轮 P0 优先**：N6（§7 幽灵字段）、N7（token 命名约定）—— 这两项是第 4 轮引入的严重新问题，必须优先修复
2. **第 5 轮常规**：N8（§11.3 同步）、N9（确认删除意图）、N4（一行修正）
3. **第 9 轮收尾**：第 8 轮 91 项 P2 处理完后，复核本清单所有"待处理"项是否已关闭

### 验证机制建议

建议第 7 轮"新增 Beta impl-notes 同步规则"时追加：

> **文档字段对齐验证规则**：修改 architecture.md §7 任何配置段后，必须将文档字段与 [config.go](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/config/config.go) 对应结构体的 `mapstructure` tag 逐一核对，确保零幽灵字段、零缺失字段、零类型错误。

---

*文档创建时间：2026-06-28*
*最后更新：2026-06-28（追加 N6-N9，更新 N3/N5 为已修复）*
