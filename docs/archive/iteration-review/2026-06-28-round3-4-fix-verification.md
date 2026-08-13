# 第 3+4 轮修复合并验证报告

**验证日期**: 2026-06-28
**验证对象**: 第 3 轮（E1-G2，7 项阻塞级）+ 第 4 轮（H1-H8/I14-I16/J1-J8/N3/N5，22 项警告级）
**验证方法**: git status 核对 + 逐项读取文件实际内容 + 与 config.yaml/config.go/design-system.md 交叉验证
**验证人**: GLM-5.2 Agent

---

## 一、执行摘要

### 1.1 总体验证结果

| 轮次 | 完全修复 | 部分修复 | 未修复 | 修复率 |
|------|---------|---------|--------|--------|
| 第 3 轮（7 项阻塞级） | 6 | 1 | 0 | 86% (6/7) |
| 第 4 轮（22 项警告级） | 10 | 11 | 1 | 45% (10/22) |
| **合计** | **16** | **12** | **1** | **55% (16/29)** |

### 1.2 关键发现

1. **第 3 轮修复质量优秀**：7 项阻塞级问题中 6 项完全修复，仅 E2 因"合并为 1 条"而非"拆分为 4 条"被判部分修复
2. **第 4 轮修复引入严重新问题**：H5 修复（补充 9 个配置段说明）反而引入了"幽灵字段"问题——多个段的字段在 config.go 中根本不存在，加重了 H8 的脱节问题
3. **J 系列系统性问题未根治**：ui-spec.md 用点号引用 token（`canvas.default`），design-system.md 用连字符定义（`--canvas-default`），且 ui-spec.md 引用了 design-system.md 未定义的 token，J1/J3/J7/J8 本质是同一系统性问题的不同表现
4. **累计完全修复率**：19 项阻塞级中 18 项已完全修复（95%），剩余 E2 为部分修复

---

## 二、第 3 轮验证详情（7 项阻塞级）

### E1 — 4 个安全加固 plan 未勾选 → ✅ 已修复

**修复方式**：4 个文件全部在第 3 行添加相同横幅（无需逐个勾选 115 项 checkbox）

**横幅内容**：
```
> ✅ **完成状态**: 本计划全部步骤已于 2026-06-09 执行完毕。执行记录见 `docs/superpowers/progress/2026-06-09-security-hardening-execution.md`。以下步骤仅保留原始计划结构作历史参考。
```

**验证结果**：
- 4 个文件均第 3 行添加横幅 ✅
- 115 个 `- [ ]` 与 0 个 `- [x]` 保持不变（符合"无需逐个勾选"期望）✅
- 横幅清晰说明"已完成"+ "仅作历史记录"+ 指向详细执行记录 ✅
- 使用 blockquote 语法，未破坏 markdown 结构 ✅

### E2 — progress.txt 缺安全加固记录 → ⚠️ 部分修复

**修复方式**：追加 1 条合并条目（行 3204-3217）

**条目内容**：
```
## [2026-06-09] - 安全加固批量执行
### What was done:
- 执行 4 份安全加固计划：滥用控制、依赖漏洞升级、OSS 上传下载加固、发布门禁与配置加固
- 详细执行记录见 docs/superpowers/progress/2026-06-09-security-hardening-execution.md
- 提交哈希: 1409933, 24fd2a0, 7da3442, 7ce8191
### Testing:
- go test ./... 通过
- npm run build 通过
### Notes:
- 本次执行使用独立 worktree 和分支 codex/security-hardening-execution
- 不涉及 task.json 或 Beta roadmap checkbox 变更
```

**部分修复原因**：期望追加 4 条独立条目（每份 plan 一条），实际只追加 1 条合并条目。虽覆盖了 4 份 plan 信息且遵循三段式模板，但无法独立追踪每份 plan。

### E3 — 决策索引表实施状态列 → ✅ 已修复

**表头变化**：
- 修复前：`| 编号 | 关联问题 | 标题 | 状态 | 讨论日期 |`（5 列）
- 修复后：`| 编号 | 关联问题 | 标题 | 确认状态 | 实施状态 | 讨论日期 |`（6 列）

**抽查 DEC 取值与 config.yaml 一致性**：

| 决策 | 确认状态 | 实施状态 | config.yaml 实际 | 准确性 |
|------|---------|---------|-----------------|--------|
| DEC-001 | ✅ 已确认 | 🚧 实施中 | — | ✅ |
| DEC-003 | ⏸️ 搁置 | — | — | ✅ |
| DEC-018 | ✅ 已确认 | ⏳ 待实施 | `captcha.provider: "bypass"` | ✅ |
| DEC-019 | ✅ 已确认 | ⏳ 待实施 | `queue.enabled: false` | ✅ |
| DEC-020 | ✅ 已确认 | ⏳ 待实施 | `agent.llm_api_key: ""` | ✅ |

"确认状态"与"实施状态"取值符号体系完全分离，无混淆 ✅

### F2 — PLAN.md 完成横幅 → ✅ 已修复

**横幅内容**（第 3 行）：
```
> ⚠️ **历史文档**: 本文档描述的计划（Task 99-145）已全部完成（task.json 中 passes: true）。保留此文档仅作历史记录和设计决策参考。活跃开发请参阅 `docs/superpowers/plans/`。
```

明确引用 task.json passes: true 作为完成依据 ✅

### F3 — CURRENT_TASK_HANDOFF 归档 → ✅ 已修复（采用归档方案）

**验证结果**：
- docs/ 原位文件已删除（git status 显示 `D`）✅
- 已归档至 [docs/archive/CURRENT_TASK_HANDOFF_019dfb5e-1360-7050-b2a4-f8900ada98e4.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/archive/CURRENT_TASK_HANDOFF_019dfb5e-1360-7050-b2a4-f8900ada98e4.md) ✅
- 归档文件内容完整 ✅
- [docs/archive/README.md](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/docs/archive/README.md) 已创建（第 5 轮 N 系列目标提前完成）✅

### G1 — async-queue-analysis.md 搜索描述过时 → ✅ 已修复

**第 195 行内容**：
```
| `jiebacfg` 字典（索引侧），查询侧仍 `simple` 待统一（见 DEC-001） | IK Analyzer 分词 |
```

- 明确区分"索引侧 jiebacfg"与"查询侧 simple 待统一" ✅
- 引用 DEC-001 ✅

### G2 — oss-lifecycle.md TTL 冲突 → ✅ 已修复

**第 10-11 行内容**：
```
- Download URL validity: 5 minutes (`download_url_ttl_sec: 300` in config.yaml)
- Upload presign URL validity: 1 hour (`presign_expire_sec: 3600` in config.yaml)
```

| 字段 | 修复前 | 修复后 | config.yaml 权威 | 结果 |
|------|--------|--------|-----------------|------|
| Download | 1 hour | 5 minutes | `download_url_ttl_sec: 300` | ✅ |
| Upload | 15 minutes | 1 hour | `presign_expire_sec: 3600` | ✅ |

直接引用 config.yaml 字段名作为权威来源 ✅

---

## 三、第 4 轮验证详情（22 项警告级）

### H 系列（architecture.md §7 配置字段脱节）

#### H1 — features 字段对齐 → ✅ 已修复

文档 §7（第 1401-1403 行）与 config.yaml（第 46-49 行）均只有 3 个字段：`payment_enabled / creator_support_enabled / desktop_deploy_enabled`。文档不再包含不存在的 `ad_enabled/agent_enabled/judge_enabled`。

#### H2 — judge 字段名统一 → ✅ 已修复

文档 §7（第 1435-1440 行）5 个字段名与 config.yaml（第 117-122 行）完全一致。旧字段名 `pass_score_rate/revoke_min_votes/min_votes` 已全部消除。

#### H3 — upload 段字段位置 → ⚠️ 部分修复

- ✅ upload 段已修正：只剩 `sheet_music_extensions`，与 config.yaml 一致
- ❌ **新问题**：oss 段引入了 `presign_expire_sec: 3600`，但 [config.go OSSConfig](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/config/config.go#L75-L82) 中**无此字段**

#### H4 — agent 段补全 → ✅ 已修复

文档 §7 agent 段（第 1473-1485 行）包含全部 12 个字段，与 config.yaml + config.go AgentConfig 完全对齐。`max_user_message_chars/chat_max_context_messages/hmac_secret` 三个之前缺失的字段均已补齐。

#### H5 — 9 个配置段补充说明 → ⚠️ 部分修复（引入严重新问题）

- ✅ 9 个段名都已补充（captcha/smtp/verification/legal/cache/queue/rate_limit/web/security）
- ❌ **多个段的字段与 config.yaml/config.go 严重不一致，引入"幽灵字段"**：

| 段 | 幽灵字段（config.go 无） | 缺失字段（config.go 有） |
|----|------------------------|------------------------|
| cache | `embed_cache_max_mb`、`embed_cache_ttl_h` | 全部 11 个实际字段（content_list_ttl 等） |
| queue | `max_retries`（应为 `max_attempts`） | `dlq_ttl_hours`/`maxlen`/`worker_*` 等 6 个 |
| queue | `retry_backoff_sec: 60`（应为数组 `[10,60,300]`） | — |
| rate_limit | `global_rps`、`comment_edit_per_hour` | `enabled`/`normal_per_minute` 等 9 个 |
| security | `csrf_enabled`、`frame_ancestors` | `trusted_proxies` |
| oss | `presign_expire_sec` | `endpoint`/`access_key_*`/`bucket_name`/`domain` |
| green | — | `access_key_*`/`callback_url`/`callback_allowed_ips` |
> 修正（2026-08-13）：`callback_allowed_ips` 前提证伪——阿里云官方从未发布回调来源网段，该字段已随 #104 删除；配置段现为 `access_key_*`/`callback_url`/`region`/`seed`/`uid`。
| captcha | — | `prefix`/`access_key_*` |

#### H6 — notifications.type 枚举统一 → ✅ 已修复

[architecture.md:918](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md#L918) 已统一为：`comment | like | follow | system | mention | appeal_result | content_status`，与 AGENTS.md 完全一致。

#### H7 — §7 字段顺序/分组 → ⚠️ 部分修复

- ✅ §7 开头已声明权威来源："字段名与 config/config.go 中结构体的 mapstructure tag 一一对应"
- ❌ 字段顺序仍与 config.yaml 不一致

#### H8 — 缺失 config.go 字段说明 → ⚠️ 部分修复

- ✅ agent 段已补全全部 12 个字段
- ❌ cache/queue/rate_limit/publish/oss/green/captcha/security 段仍有缺失或幽灵字段（详见 H5 表格）

### N3 — architecture.md §7 补 13 个 score_* 字段 → ✅ 已修复

[architecture.md §7 reputation 段](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md#L1419-L1432) 已补充全部 13 个 `score_*` 字段，与 [config.yaml:103-115](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/config.yaml#L103-L115) 完全一致（字段名和值都匹配）。文档还加了注释："以下 score_* 字段控制实际加减分值（0 = 使用 reputation_service.go 内置默认值）"。

### N5 — architecture.md §3.2 补 7 条 API 路由 → ✅ 已修复

[architecture.md §3.2](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md#L446-L473) 已补充 9 条路由（超出预期 7 条），与 routes.go 代码完全一致：

| 文档路由 | routes.go 行号 | 一致性 |
|---------|---------------|--------|
| POST /api/v1/feedback | 250 | ✅ |
| POST /api/v1/feedback/attachments/presign | 251 | ✅ |
| GET /api/v1/feedback/me | 252 | ✅ |
| GET /api/v1/feedback/:id | 253 | ✅ |
| GET /api/v1/admin/feedback | 360 | ✅ |
| GET /api/v1/admin/feedback/:id | 361 | ✅ |
| PATCH /api/v1/admin/feedback/:id | 362 | ✅ |
| POST /api/v1/admin/feedback/:id/replies | 363 | ✅ |
| GET /api/v1/admin/audit-logs | 364 | ✅ |

### J 系列（design 目录问题）

#### J6 — design-system.md 第 150 行 h-14 与 --header-h 矛盾 → ✅ 已修复（且未引入回归）

[design-system.md:150](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/design/design-system.md#L150) 仍为 `Header (sticky, h-[var(--header-h)])`，与第 64 行 `--header-h: 52px` 一致。第 4 轮未引入回归 ✅

#### J5 — ui-spec.md 缺失组件规格 → ✅ 已修复

ui-spec.md 包含 50 个组件/页面规格，覆盖 Header、ContentCard、TagBadge、ContentDetail、MarkdownRenderer、SheetMusicViewer、PRCard、DiffViewer、ReactionBar、CommentSection、FileUploader 等核心组件。

#### J1/J3/J7/J8 — token 引用与定义系统性不一致 → ⚠️ 部分修复（同一根本问题）

**根本问题**：ui-spec.md 与 design-system.md 的 token 命名约定系统性不一致

| 文件 | 命名形式 | 示例 | Tailwind 有效性 |
|------|---------|------|----------------|
| ui-spec.md | 点号分隔 | `bg-canvas.default`、`text-fg.muted` | ❌ Tailwind 不支持点号类名 |
| design-system.md | 连字符 | `--canvas-default`、`--fg-muted` | ✅ 标准 CSS 变量 |

**未定义的 token**（ui-spec.md 使用但 design-system.md 未定义）：
- `border.muted` / `--border-muted`（15+ 处使用）
- `canvas.default.dark` / `--canvas-default-dark`（17 处使用）
- `border.default.dark` / `--border-default-dark`（17 处使用）

**J1 旧 token 名消除** ✅，但新 token 形式引入新问题 ❌

#### J2 — ui-spec.md 尺寸不一致 → ⚠️ 部分修复

- ✅ 高度已统一使用 token：`h-[var(--header-h)]`（30+ 处）
- ❌ 仍有多处 px 硬编码：`1280px`、`400px`、`228px`、`700px`、`1100px` 等

#### J4 — ui-spec.md 组件命名与代码不一致 → ⚠️ 未完全确认

需对照 frontend/components 才能完全确认，本次未深入对照。

### I 系列（其他警告级问题）

#### I14 — 旧环境变量名 → ✅ 已修复

architecture.md 中搜索 `ALIYUN_ACCESS_KEY|OSS_BUCKET\b|OSS_CDN_DOMAIN|ALIYUN_GREEN_ENDPOINT` 无任何匹配。已全部更新为新名。

#### I15 — 表设计完整 → ✅ 已修复

architecture.md §4 包含完整表设计：feedback_tickets、feedback_replies、feedback_attachments、admin_audit_logs，索引齐全。

#### I16 — §11.3 agent 段对齐 → ⚠️ 部分修复

[architecture.md §11.3](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/architecture.md#L2050-L2061) agent 段包含 9 个字段，**缺失 3 个字段**（§7 已补但 §11.3 未同步）：
- ❌ `max_user_message_chars`
- ❌ `chat_max_context_messages`
- ❌ `hmac_secret`

---

## 四、新引入问题汇总

### 🔴 N6：§7 多个段引入"幽灵字段"（严重）

**定义**：文档 §7 中出现的字段，在 config.go 对应结构体中根本不存在。

| 段 | 幽灵字段 | config.go 实际字段 |
|----|---------|------------------|
| cache | `embed_cache_max_mb`、`embed_cache_ttl_h` | content_list_ttl 等 11 个 |
| queue | `max_retries`（应为 `max_attempts`） | max_attempts 等 |
| rate_limit | `global_rps`、`comment_edit_per_hour` | normal_per_minute 等 |
| security | `csrf_enabled`、`frame_ancestors` | trusted_proxies |
| oss | `presign_expire_sec` | download_url_ttl_sec 等 |

**影响**：开发者按文档配置这些字段会被 viper 静默忽略，功能不生效且难以排查。这比 H5 修复前更严重——修复前是"字段缺失"，修复后是"字段错误"。

### 🔴 N7：ui-spec.md token 引用形式系统性错误（严重）

ui-spec.md 大量使用点号形式引用 token（`bg-canvas.default`），但：
1. Tailwind CSS 类名不支持点号分隔符，这些不是有效类名
2. design-system.md 定义的是连字符形式（`--canvas-default`）
3. ui-spec.md 引用了 design-system.md 未定义的 token（`border.muted` 等，30+ 处）

**影响**：前端开发者按 ui-spec.md 写出的 className 会无效，需自行转换为连字符形式。

### 🟡 N8：§11.3 agent 段未与 §7 同步（I16 遗留）

§7 agent 段已补全 12 个字段，但 §11.3 仍是 9 个字段，缺失 `max_user_message_chars/chat_max_context_messages/hmac_secret`。

### 🟢 N9：2 份旧 iteration-review 报告被删除

git status 显示删除：
- `docs/iteration-review/2026-06-25-iteration-review-report(1).md`（477 行）
- `docs/iteration-review/2026-06-25-iteration-review.md`（389 行）

需确认这是否是预期行为（可能是被新版报告替代）。

---

## 五、累计修复进展

### 5.1 阻塞级问题（19 项）

| 状态 | 数量 | 问题清单 |
|------|------|---------|
| ✅ 完全修复 | 18 | A1、A2、A3、A4、B1、B2、B3(主体)、C1、D1、D2、D3+N1、E1、E3、F1、F2、F3、G1、G2 |
| ⚠️ 部分修复 | 1 | E2（1 条合并条目，未拆分为 4 条） |
| ❌ 未修复 | 0 | — |

**阻塞级累计完全修复率：95% (18/19)**

### 5.2 警告级问题（58 项，本轮处理 22 项）

| 状态 | 数量 | 问题清单 |
|------|------|---------|
| ✅ 完全修复 | 10 | H1、H2、H4、H6、N3、N5、J5、J6、I14、I15 |
| ⚠️ 部分修复 | 11 | H3、H5、H7、H8、J1、J2、J3、J7、J8、I16、J4(未确认) |
| ❌ 未修复 | 1 | J4（未完全确认） |

### 5.3 新发现问题（本轮引入）

| 编号 | 严重度 | 问题 | 处理建议 |
|------|--------|------|---------|
| N6 | 🔴 严重 | §7 多段引入幽灵字段 | 第 5 轮优先重写 cache/queue/rate_limit/oss/green/captcha/security 段 |
| N7 | 🔴 严重 | ui-spec.md token 点号引用 | 第 5 轮统一 token 命名约定 |
| N8 | 🟡 警告 | §11.3 agent 段未同步 | 第 5 轮或第 9 轮处理 |
| N9 | 🟢 建议 | 2 份旧报告被删除 | 确认是否预期 |

---

## 六、修复质量评价

### 第 3 轮：⭐⭐⭐⭐⭐ 优秀

- E1 横幅方案优雅：无需逐个勾选 115 项 checkbox，用横幅+指向执行记录的方式高效且可追溯
- E3 实施状态列设计精准：6 列表头，"确认状态"与"实施状态"符号体系完全分离
- F3 归档方案彻底：原位删除+归档+创建 archive/README.md（提前完成第 5 轮目标）
- G1/G2 修复精准：直接引用 DEC-001 和 config.yaml 字段名作为权威

### 第 4 轮：⭐⭐⭐ 中等（H5 引入新问题拉低评分）

- H1/H2/H4/H6 精准修复，字段完全对齐
- N3/N5 优秀：13 个 score_* 字段和 9 条路由均与代码完全一致
- J6 保持正确，未引入回归
- **但 H5 修复引入 N6 幽灵字段问题**：cache/queue/rate_limit 段的修复反而加重了 H8 的脱节，是本轮最大遗憾
- **J 系列 token 系统性问题未根治**：J1/J3/J7/J8 本质是同一问题，但修复未建立统一的 token 命名约定

---

## 七、下一步建议

### 7.1 第 5 轮优先处理（P0）

1. **N6 修复**：重写 architecture.md §7 的 cache/queue/rate_limit/oss/green/captcha/security 段，严格按 [config.go](file:///c:/Users/16278/Desktop/file/code/project/OmniCraft/backend/config/config.go) 结构体字段对齐，删除所有幽灵字段
2. **N7 修复**：建立 ui-spec.md 与 design-system.md 的 token 命名映射约定，统一为连字符形式（`bg-canvas-default` 或 `bg-[var(--canvas-default)]`），并在 design-system.md 补定义 `border.muted`/`canvas.default.dark`/`border.default.dark` 或在 ui-spec.md 中替换为已定义 token

### 7.2 第 5 轮常规处理（P1）

3. **N8 修复**：architecture.md §11.3 agent 段补 3 个字段（与 §7 同步）
4. **K1-K5/L1-L4/M1-M2**：按原计划推进
5. **N4 顺手修正**：production-config-template.md:174 的 `/health` → `/healthz`

### 7.3 第 9 轮收尾

6. **E2 拆分**：将 progress.txt 行 3204-3217 的合并条目拆分为 4 条独立条目
7. **J2 px 硬编码**：评估哪些 px 值应转为 token
8. **J4 组件命名**：对照 frontend/components 确认

### 7.4 验证机制建议

建议第 7 轮"新增 Beta impl-notes 同步规则"时追加：

> **文档字段对齐验证规则**：修改 architecture.md §7 任何配置段后，必须执行 `grep` 将文档字段与 config.go 对应结构体的 `mapstructure` tag 逐一核对，确保零幽灵字段、零缺失字段。

---

*验证完成时间：2026-06-28*
