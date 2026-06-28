# OmniCraft 项目文档全面系统性检查报告

| 项 | 内容 |
|---|---|
| 报告日期 | 2026-06-28 |
| 审查范围 | 项目所有开发相关文档（核心架构 / 设计 / Beta 计划 / 任务账本 / 审查报告 / 部署运维 / Agent 工作流） |
| 审查方法 | 4 个并行子代理交叉审查 + 主代理合并去重 |
| 文档总数 | 约 60 份 |
| 问题总数 | 187 条（去重合并后） |
| 阻塞问题 | 41 条 |
| 审查人 | 文档审查 Agent |

---

## 一、问题总览

本次审查覆盖以下 7 大类文档：

| 模块 | 文档群组 | 问题数 |
|---|---|---|
| A | 核心架构与规范（architecture.md / AGENTS.md / CLAUDE.md / PRD / README.md） | 75 |
| B | 设计文档（design-system.md / ui-spec.md / ui-design-prompt.md / doc-review-prompt.md） | 40 |
| C | Beta 路线图与计划（docs/superpowers/plans/* + specs/*） | 20 |
| D | 历史任务账本与进度（task.json / progress.txt / docs/history/docs edited history.md） | 22 |
| E | 审查报告、迭代记录、设计审查、部署运维、Agent 工作流（docs/review/* / docs/iteration-review/* / docs/deploy/* / docs/agents/* / docs/* 杂项） | 30 |

合并去重后共识别 **187 条问题**，其中重复识别 22 条已合并。审查发现：项目文档体系整体框架完整、Beta 路线图结构清晰，但存在显著的「文档与代码脱节」「跨文档不一致」「权威边界模糊」三类系统性问题。

---

## 二、分类统计

### 2.1 按严重程度统计

| 严重程度 | 数量 | 占比 | 说明 |
|---|---|---|---|
| 🔴 阻塞 | 41 | 22% | 矛盾会导致开发错误、运行时 Bug 或安全漏洞；必须立即修复 |
| 🟡 警告 | 99 | 53% | 不一致但不影响功能；建议修复 |
| 🟢 建议 | 47 | 25% | 优化建议；可延后处理 |

### 2.2 按问题类型统计

| 问题类型 | 数量 | 典型表现 |
|---|---|---|
| 内容前后矛盾 | 38 | 同一字段/阈值/枚举在多文档取值不同；schema 与迁移不一致 |
| 关键信息缺失 | 35 | schema 字段缺注释、API 缺定义、配置项未登记、无回滚/监控/无障碍规范 |
| 技术方案过时 | 24 | 文档清单与实际代码脱节、迁移编号引用过时、引用已归档/不存在文件 |
| 失效引用 | 14 | 引用不存在的 CONTEXT.md、CLAUDE.md vs AGENTS.md 混用、引用归档文件 |
| 表述模糊不清 | 18 | 「达标」「适当」「大部分」等无具体数值；状态描述模糊 |
| 设计逻辑不合理 | 12 | 路由结构混乱、API 缺解禁端点、schema 与软删除策略不符 |
| 术语使用不一致 | 10 | 「判官/评审员/审核员」「二创/同人作品/fanwork」「MVP/Beta」混用 |
| 格式不规范 | 14 | 字段顺序不一致、Markdown 转义、重复 heading、checkbox 未同步 |
| 重复 | 9 | 报告间重复发现问题、文档间内容重复 |
| 版本信息错误 | 7 | 文件名日期与正文日期不符、依赖版本粒度差异 |
| 依赖错误 | 6 | task.json depends_on 缺失、反向依赖未修复 |
| 不合理 | 5 | ID 与执行顺序倒挂、debug 模式跳过 release 校验 |

### 2.3 按文档群组统计

| 文档群组 | 阻塞 | 警告 | 建议 | 小计 |
|---|---|---|---|---|
| 核心架构（A） | 5 | 35 | 35 | 75 |
| 设计文档（B） | 13 | 22 | 5 | 40 |
| Beta 计划（C） | 2 | 5 | 13 | 20 |
| 任务账本（D） | 5 | 9 | 8 | 22 |
| 审查/运维（E） | 16 | 22 | 9 | 47 |
| **合计** | **41** | **93** | **70** | **204**（含合并前） |

> 实际去重后 187 条；上表为子代理原始报告统计，存在跨模块重复识别。

---

## 三、详细问题清单

> 编号规则：`<模块前缀>-<序号>`，模块前缀：ARCH（核心架构）、DESIGN（设计）、PLAN（Beta 计划）、TASK（任务账本）、REVIEW（审查/运维）。

### 3.1 核心架构与规范文档（ARCH-001 ~ ARCH-075）

#### 3.1.1 内容前后矛盾

**ARCH-001** 🔴 阻塞 → 🟡 警告（合并）
- 文档：architecture.md
- 章节：§6.2（行 1378）vs §7 config.yaml（行 1474）
- 问题：§6.2 OSS 描述「签名 URL 有效期 1 小时」，§7 config 实际 `oss.download_url_ttl_sec: 300`（5 分钟）。差 12 倍。
- 修复：统一为 300 秒；或修改 config 为 3600。

**ARCH-002** 🔴 阻塞
- 文档：architecture.md
- 章节：§7（行 1483-1487）vs §11.3（行 2074-2079）
- 问题：LLM 配置默认值前后不一致。§7：`llm_provider: "openai_compat"`、`llm_model: "deepseek-chat"`、`embedding_model: "text-embedding-3-small"`；§11.3：`llm_provider: qwen`、`llm_model: qwen-turbo`、`embedding_model: text-embedding-v3`。实际 config 与 §7 一致。
- 修复：将 §11.3 示例改为与 §7 一致，或标注「§11.3 为 Qwen provider 示例」。

**ARCH-003** 🔴 阻塞
- 文档：architecture.md
- 章节：§4.3（行 710）vs §15.6（行 2628-2629）vs 实际 migration 040
- 问题：`content_items.download_count` 字段三处描述自相矛盾：§4.3 称「非数据库列，由 Redis ZSET 维护」；§15.6 称「新增字段（或独立 Redis 维护）」；实际 migration 040 已添加 `download_count INTEGER NOT NULL DEFAULT 0` 列。
- 修复：更新 §4.3 schema 加入 `download_count INT NOT NULL DEFAULT 0`，删除「非数据库列」注释。

**ARCH-004** 🔴 阻塞
- 文档：architecture.md vs Beta 设计 vs 前端代码
- 章节：§3.3（行 522）vs docs/superpowers/specs/2026-05-30-omnicraft-dual-track-beta-design.md vs frontend/components/content/ContentDetail.tsx:343
- 问题：URL Scheme 协议三种版本并存：
  - architecture.md §3.3：`omnicraft://deploy?content_id=xxx&token=yyy&action_script=zzz`（3 参数）
  - Beta 设计：`omnicraft://deploy?grant=<opaque-token>`（1 grant 参数）
  - 前端代码：`omnicraft://deploy?content_id=${data.id}`（1 content_id 参数）
- 修复：以 Beta 设计为准统一为 `grant=<opaque-token>`；更新 architecture.md §3.3 和前端代码。

**ARCH-005** 🟡 警告
- 文档：architecture.md
- 章节：§4.5（行 968）vs 实际 migration
- 问题：§4.5 注释称「索引由 migrations/017_conversations.sql 创建」，实际文件 017 是 `tag_groups.sql`，conversations 表迁移是 024。
- 修复：将 `017_conversations.sql` 改为 `024_conversations.sql`。

**ARCH-006** 🟡 警告
- 文档：architecture.md
- 章节：§4.6（行 1042）vs §7（行 1437）
- 问题：`judge_cases.min_votes` 字段注释称从 `judge.min_votes` 读取，实际 config 字段为 `judge.min_votes_required`。
- 修复：将注释改为 `judge.min_votes_required`。

**ARCH-007** 🟡 警告
- 文档：architecture.md
- 章节：§3.2（行 338）vs §10.1（行 1792）
- 问题：sort 枚举值命名不一致：§3.2 为 `most_views`，§10.1 为 `clicks`。
- 修复：统一为 `most_views`。

**ARCH-008** 🟡 警告
- 文档：architecture.md
- 章节：§15.4（行 2614）vs §15.9（行 2650）
- 问题：§15.4 引用 `038_content_search.sql`，§15.9 已修正为 041。实际文件 `041_content_search_vector.sql`。
- 修复：更新 §15.4 迁移编号为 041。

**ARCH-009** 🟡 警告
- 文档：architecture.md
- 章节：§15.9（行 2670）vs migration 040
- 问题：§15.9 称「邮箱验证（实际迁移 050）」，但 `email_verified_at` 实际在 migration 040 line 14 添加。
- 修复：将 email_verified_at 迁移号改为 040。

**ARCH-010** 🟡 警告
- 文档：architecture.md
- 章节：§3.2（行 346-347）vs §13.5
- 问题：§3.2 称 API「已迁移至 /studio/*，旧 /dashboard/* 保留兼容」，但 §13.5 仅说明前端 301 重定向，未说明 API 兼容策略。
- 修复：在 §13 明确 `/api/v1/dashboard/*` API 是否保留兼容。

**ARCH-011** 🟡 警告
- 文档：architecture.md（自相矛盾）+ AGENTS.md
- 章节：§3.2（行 437, 2627）vs §15.6 vs AGENTS.md「内容下载」
- 问题：下载 API 返回方式描述不一致：§3.2「返回 OSS 签名 URL」；§15.6「302 重定向」；AGENTS.md「前端获取签名 URL 触发下载」。
- 修复：统一为「返回 JSON `{ download_url, expires_at }`，前端用 `window.location.href` 触发下载」。

**ARCH-012** 🟡 警告
- 文档：PRD vs architecture.md vs config.yaml
- 章节：PRD §12.2（行 328）vs architecture.md §3.4（行 543-544）vs config.yaml
- 问题：判决通过条件表述相反：PRD「60% 反对率自动下架」；architecture「不违规比例 ≥ 60% 恢复展示」；config `pass_threshold: 0.60`。表述极易混淆。
- 修复：统一为「不违规比例 ≥ 60% 恢复展示；< 60% 不予展示」。

**ARCH-013** 🟡 警告
- 文档：architecture.md
- 章节：§4.7（行 1094）vs §10.6（行 1996-2001）
- 问题：categories.level 枚举含 `secondary`，但 §10.6 明确「已移除二级内容类型筛选」。
- 修复：删除 `secondary` 枚举值，或注明「历史保留，未启用」。

**ARCH-066** 🟢 建议
- 文档：PRD vs architecture.md
- 章节：PRD §3.1（行 61）vs architecture.md §3.2（行 338）
- 问题：IP 详情页排序选项 PRD 列 4 项，architecture 列 5 项（多 `recommended`），且 `recommended` 仅适用原创区。
- 修复：明确 `recommended` 仅 `zone=original` 可用。

**ARCH-067** 🟢 建议
- 文档：architecture.md
- 章节：§10.1（行 1677）vs §10.1 SQL 注释（行 1722）
- 问题：标签大类枚举：文字描述 5 项（学习/游戏/动漫/音乐/其他），SQL 注释 6 项（多「影视」）。
- 修复：统一为 6 项含「影视」。

**ARCH-068** 🟡 警告
- 文档：architecture.md
- 章节：§10.1（行 1774）
- 问题：列出 `GET /api/v1/dashboard/tag-suggestions`（旧路径），但 §13.5 明确 `/dashboard/*` 已迁移至 `/studio/*`。
- 修复：改为 `GET /api/v1/studio/tag-suggestions`。

**ARCH-070** 🟡 警告
- 文档：PRD vs architecture.md
- 章节：PRD §3.2（行 87）vs architecture.md §4.3（行 681-684）
- 问题：原创区内容类目不一致：PRD 列 6 大类（含「模型与设计」），architecture 列 9 类（含 `mod`、`prompt`）。`mod` 在二创区是「游戏 Mod」，语义混淆。
- 修复：明确 `mod` 在原创区语义；PRD 补充「AI 提示词」类目。

**ARCH-071** 🟢 建议
- 文档：PRD vs architecture.md
- 章节：PRD §4（行 121-122）vs architecture.md §3.3（行 509-517）
- 问题：Agent 白名单动作数量：PRD 列 6 项，architecture 列 7 项（含 `backup_file`）。
- 修复：PRD 补充「自动备份原文件」说明。

**ARCH-073** 🟢 建议
- 文档：architecture.md
- 章节：§4.1（行 588）vs §4.3（行 680）vs §4.6
- 问题：content_type 字段长度不一致：`judge_qualifications` 50、`content_items` 20、`judge_questions` 50、`judge_exam_records` 50。最长值 `sheet_music` 仅 11 字符。
- 修复：统一为 `VARCHAR(20)`。

#### 3.1.2 表述模糊不清

**ARCH-014** 🟡 警告
- 文档：PRD §6.2（行 196）
- 问题：「评论负面比例**达标** → 自动折叠」中「达标」无具体数值。AGENTS.md 补充为 `comment_fold_threshold: 0.30`。
- 修复：PRD 补充具体阈值「点踩/点赞比 ≥ 0.30」。

**ARCH-015** 🟡 警告
- 文档：PRD §6.2（行 194）
- 问题：「内容举报率（举报数 / **点击率**）≥10%」中「点击率」定义模糊。content_items 表只有 `view_count`，无 `click_count`。
- 修复：明确定义为「举报数 / 浏览数 ≥ 0.10」，对应 `view_count`。

**ARCH-016** 🟢 建议
- 文档：PRD §3.2（行 93）
- 问题：原创区排序选项未在 PRD 列出。
- 修复：PRD 补充排序选项列表。

**ARCH-017** 🟢 建议
- 文档：architecture.md §4.3（行 686-687）
- 问题：`content_items.status='hidden'` 触发场景未文档化。
- 修复：补充 `hidden` 触发场景。

**ARCH-018** 🟢 建议
- 文档：architecture.md §3.2（行 338）
- 问题：sort 各枚举值对应字段未文档化。`new` 用 `created_at` 还是 `published_at`？表无 `published_at` 字段。
- 修复：补充每个 sort 值对应的 SQL 字段/公式。

**ARCH-019** 🟢 建议
- 文档：PRD §1（行 21）
- 问题：「思维导图」在 content_type 枚举无对应；「等等」措辞模糊。
- 修复：明确思维导图归类，删除「等等」。

**ARCH-069** 🟡 警告
- 文档：PRD §6.3（行 219）vs AGENTS.md
- 问题：PRD「低信誉分用户仅浏览内容」表述模糊，未说明是否包括对自己内容的编辑/删除权限。
- 修复：PRD 补充「仅浏览」精确定义，与 AGENTS.md 对齐。

#### 3.1.3 设计逻辑不合理

**ARCH-020** 🔴 阻塞
- 文档：architecture.md §3.1（行 116-162）
- 问题：前端路由树结构性问题：`(protected)` 路由组出现两次；`admin/` 路由放在 `(protected)` 之外，但 AGENTS.md Task 105 明确要求 admin/* 必须包裹在 protected 内。实际代码已正确放在 `(protected)/admin/` 下，但文档与代码不符。
- 修复：将 §3.1 路由树中的 `admin/` 移入 `(protected)/admin/`。

**ARCH-021** 🟡 警告
- 文档：architecture.md §3.1（行 147）vs §13.2（行 2386-2395）
- 问题：路由树列出 `/studio/favorites`，但 §13.2 侧边栏 9 个菜单项无「收藏」入口。路由存在但用户无法访问。
- 修复：在 §13.2 侧边栏增加「我的收藏」菜单项，或改路由为 `/users/:userId/collections`。

**ARCH-022** 🟡 警告
- 文档：architecture.md §3.2 vs §4.5
- 问题：API 描述 `DELETE /messages/:id` 软删除消息和 `DELETE /conversations/:id` 退出会话，但 schema 中 `messages` 和 `conversations` 表均无 `deleted_at` 字段。
- 修复：为表添加 `deleted_at`，或将 API 改为物理删除。

**ARCH-023** 🟡 警告
- 文档：architecture.md §4.5（行 822-833）
- 问题：`comments` 表只有 `like_count`，无 `dislike_count`。但风控阈值「点踩/点赞比 ≥ 0.30」需要点踩数。
- 修复：在 `comments` 表添加 `dislike_count INT NOT NULL DEFAULT 0`。

**ARCH-024** 🟡 警告
- 文档：architecture.md §4.5（行 918-936）
- 问题：`notifications.type` 7 值，`channel` 3 值。type → channel 映射未文档化。
- 修复：补充映射表或统一字段。

**ARCH-025** 🟡 警告
- 文档：architecture.md §4.5 vs §3.2
- 问题：`reports.target_type` 含 `'ip'`，但 `appeals.target_type` 不含 `'ip'`。IP 被封禁后无法申诉，但 PRD §3.1 提到「支持用户人工申诉」。
- 修复：在 `appeals.target_type` 加入 `'ip'`。

**ARCH-026** 🟡 警告
- 文档：architecture.md §3.2（行 386）
- 问题：管理员只有 `POST /admin/users/:id/ban`，**无 unban 解禁 API**。
- 修复：补充 `POST /admin/users/:id/unban` 或 `DELETE /admin/users/:id/ban`。

**ARCH-027** 🟡 警告
- 文档：architecture.md §3.2（行 385, 473）
- 问题：`POST /admin/contents/:id/ban` 封禁内容，但 restore 用于软删除恢复，非 ban 恢复。
- 修复：补充 `POST /admin/contents/:id/unban`。

**ARCH-028** 🟢 建议
- 文档：architecture.md §3.2（行 388）
- 问题：管理员申诉 API 缺少 `GET /admin/appeals/:id`（详情）。
- 修复：补充该 API。

**ARCH-029** 🟡 警告
- 文档：architecture.md §4.5 vs §3.2
- 问题：`reports.target_type` 支持 `'ip'`，但 §3.2 路由清单无 IP 举报 API。
- 修复：补充 `POST /api/v1/ips/:id/report`。

**ARCH-030** 🟢 建议
- 文档：architecture.md §4.5（行 894-901）
- 问题：`browse_history` 表 UNIQUE 约束丢失重复浏览历史。
- 修复：移除 UNIQUE 约束，或明确「仅记录最后浏览时间」。

**ARCH-074** 🟢 建议
- 文档：architecture.md §4.5
- 问题：AGENTS.md 称「浏览历史使用物理删除」，但 §4.5 未注明。同时表设计为 UNIQUE upsert，与「物理删除」语义矛盾。
- 修复：在表注释明确「物理删除，不设 deleted_at」。

#### 3.1.4 技术方案过时 / 文档与代码脱节

**ARCH-031** 🟡 警告
- 文档：architecture.md §3.2（行 226-303）
- 问题：后端项目结构清单严重过时。middleware 列 4 个实际 17 个；handler 列 17 个实际 40+；service 列 17 个实际 30+；repository 列 16 个实际 20+。
- 修复：全面更新或改为按业务域归类描述。

**ARCH-032** 🟡 警告
- 文档：architecture.md §4.1（行 566-582）
- 问题：users 表 schema 缺少 `email_verified_at`（migration 040）、`deleted_at`（migration 054）等字段。
- 修复：更新 schema 反映所有迁移。

**ARCH-033** 🟡 警告
- 文档：architecture.md §4.3（行 670-700）
- 问题：content_items 表 schema 缺少 `deleted_at`、`download_count`、`ban_reason`、`search_vector` 等字段。
- 修复：更新 schema。

**ARCH-034** 🟡 警告
- 文档：architecture.md §4.5（行 822-833）
- 问题：comments 表 schema 缺少 migration 043 添加的 `target_type`、`target_id`、`content`、`updated_at` 字段。
- 修复：更新 schema。

**ARCH-062** 🟢 建议
- 文档：architecture.md §15.9（行 2676-2678）
- 问题：称「users.id 需要从 SERIAL 改为 BIGSERIAL」，但 §4.1 schema 已为 BIGSERIAL。迁移已完成。
- 修复：更新 §15.9 注明「已完成」。

#### 3.1.5 关键信息缺失

**ARCH-035** 🟡 警告
- 文档：architecture.md §7 config.yaml（行 1400-1564）
- 问题：§7 缺少 `server` 配置块（实际有 `port`/`mode`/`shutdown_timeout`）。
- 修复：补充 `server` 块。

**ARCH-036** 🟡 警告
- 文档：architecture.md §15.11（行 2690）
- 问题：硬编码「15 秒超时」，但 AGENTS.md 称「从 config 读取」。
- 修复：改为「从 `config.yaml > server.shutdown_timeout` 读取（默认 15 秒）」。

**ARCH-046** 🟡 警告
- 文档：architecture.md §4.3
- 问题：`content_attachments.file_type` 与 `content_items.content_type` 映射关系未文档化。
- 修复：补充映射表。

**ARCH-047** 🔴 阻塞
- 文档：architecture.md §4 + AGENTS.md
- 问题：AGENTS.md 描述了 `collections` 和 `collection_items` 两张表 schema，但 architecture.md §4 完全无对应 DDL。migrations 目录也无对应文件。
- 修复：在 §4 新增 §4.13 收藏集 schema，并补充 migration 文件。

**ARCH-048** 🟡 警告
- 文档：architecture.md §3.2
- 问题：缺少收藏集相关 API（`POST /collections`、`GET /collections/:id`、`POST /collections/:id/items` 等）。
- 修复：补充 API 路由。

**ARCH-049** 🟡 警告
- 文档：architecture.md §3.2 vs §11.5
- 问题：§11.5 列出 6 个 Web Agent API 路由，但 §3.2 主清单仅列 Tauri Agent 路由。
- 修复：合并 §11.5 路由到 §3.2。

**ARCH-050** 🟡 警告
- 文档：architecture.md §3.2 vs §10.1
- 问题：§10.1 列出 9 个标签 API，§3.2 仅以注释提及。
- 修复：合并 §10.1 路由到 §3.2。

**ARCH-051** 🟡 警告
- 文档：architecture.md §3.2（行 485）
- 问题：`POST /api/v1/deploy-grants` 未提供请求/响应 schema、认证要求、错误码。
- 修复：补充 API 详细规范。

**ARCH-052** 🟢 建议
- 文档：architecture.md §4.5
- 问题：`messages.body` 和 `comments.body` 无长度限制。
- 修复：补充长度约束。

**ARCH-053** 🟢 建议
- 文档：architecture.md §7
- 问题：`legal.current_terms_version` 配置存在，但缺少 `user_terms_acceptances` 表 schema。
- 修复：补充表 schema。

**ARCH-054** 🟢 建议
- 文档：architecture.md §4.1 role 字段
- 问题：`role` 枚举含 `'creator'`，但 user → creator 转换条件未文档化。
- 修复：补充转换规则。

**ARCH-055** 🟢 建议
- 文档：architecture.md §4.1 email 字段
- 问题：账号软删除后 email 仍占用 UNIQUE 约束，无法用同一 email 重新注册。
- 修复：补充 email 处理策略。

**ARCH-056** 🟢 建议
- 文档：architecture.md §4.6 judge_cases
- 问题：`judge_cases.target_type VARCHAR(20)` 无枚举注释。
- 修复：补充枚举注释。

**ARCH-057** 🟢 建议
- 文档：architecture.md §4.11
- 问题：`feedback_attachments.file_type VARCHAR(32)` 无枚举注释。
- 修复：补充枚举注释。

**ARCH-058** 🟡 警告
- 文档：architecture.md §15.3（行 2607）
- 问题：`CreateNotification` 函数签名缺少 `sender_id` 参数，但 schema 有 `sender_id` 字段。
- 修复：在签名中加入 `senderID` 参数。

**ARCH-059** 🟢 建议
- 文档：architecture.md §11.4
- 问题：`content_embeddings.embedding vector(1536)` 维度硬编码，但 config 可配置。
- 修复：明确「维度变更需配套 migration」。

**ARCH-072** 🟢 建议
- 文档：architecture.md §4.6
- 问题：`judge_questions.content_type VARCHAR(50)` 无枚举注释。
- 修复：补充枚举注释。

**ARCH-075** 🟢 建议
- 文档：architecture.md §11.4 vs §11.2
- 问题：schema 有 `agent_messages.tool_calls` JSONB，但 §11.2 ChatMessage struct 缺 `ToolCalls` 字段。
- 修复：在 struct 中补充字段。

#### 3.1.6 格式不规范

**ARCH-037** 🟢 建议
- 文档：PRD 标题（行 1）
- 问题：标题使用 `V0\.3` 转义字符。全文多处 `\. ` 转义。
- 修复：移除不必要转义。

**ARCH-038** 🟢 建议
- 文档：PRD §6.3 表格
- 问题：信誉分规则表「正向/负向」合并为一行，部分类别负向为「—」，列对齐不直观。
- 修复：拆分为两个独立表格。

**ARCH-039** 🟢 建议
- 文档：architecture.md §3.1（行 116-149）
- 问题：路由树两次 `(protected)/` 声明；`admin/` 缩进层级与 `(protected)` 平级。
- 修复：合并两块，统一缩进。

**ARCH-040** 🟢 建议
- 文档：architecture.md §13.1（行 2353）
- 问题：`Header（h-13/52px`，但 Tailwind 标准无 `h-13`。
- 修复：改为 `h-[52px]` 或 `h-14`。

#### 3.1.7 术语使用不一致

**ARCH-041** 🟡 警告
- 文档：PRD vs architecture.md vs AGENTS.md
- 问题：同一角色混用「赛博判官」「判官」「审核员」「资质审核员」「评审员」。
- 修复：统一为「赛博判官」或「判官」。

**ARCH-042** 🟢 建议
- 文档：architecture.md §4.5 vs §3.2
- 问题：`notifications.channel='reply'`（回复我的）vs `type='comment'`（评论）。
- 修复：统一术语。

**ARCH-043** 🟢 建议
- 文档：architecture.md §4.5
- 问题：`reports.target_type` 含 `'ip'`，`appeals.target_type` 不含。
- 修复：见 ARCH-025。

**ARCH-044** 🟢 建议
- 文档：PRD §1 vs architecture.md
- 问题：IP 大小写混用：PRD「ip」小写，architecture「IP」大写。
- 修复：统一为大写。

**ARCH-045** 🟢 建议
- 文档：PRD §1
- 问题：「**ppt**模板」小写。
- 修复：改为「PPT 模板」。

#### 3.1.8 版本信息错误

**ARCH-060** 🟡 警告
- 文档：README.md vs architecture.md
- 章节：README.md 行 180-186 vs architecture.md §8.1
- 问题：阿里云 AccessKey 环境变量命名不一致：README 用 `ALIYUN_ACCESS_KEY_ID/SECRET`（合并），architecture 用 `OSS_ACCESS_KEY_*` + `GREEN_ACCESS_KEY_*`（分离）。
- 修复：统一为分离式命名。

**ARCH-061** 🟡 警告
- 文档：architecture.md §8.1 vs §7
- 问题：前端 URL 两套配置：env `FRONTEND_URL` vs config `web.public_base_url`，优先级未说明。
- 修复：明确优先级或合并为单一来源。

**ARCH-063** 🟢 建议
- 文档：architecture.md §7 vs 实际 config
- 问题：`embedding_model: "text-embedding-3-small"` 是 OpenAI 模型，但 `llm_provider: "openai_compat"` 实际指向 DeepSeek，DeepSeek 不提供 embedding API。
- 修复：明确 embedding 独立 provider 配置。

#### 3.1.9 其他交叉一致性

**ARCH-064** 🟢 建议
- 文档：AGENTS.md vs CLAUDE.md
- 问题：两份文件内容几乎完全相同，存在细微措辞差异（模式 B 描述）。维护两份易导致同步漂移。
- 修复：合并为单一来源，CLAUDE.md 改为符号链接或仅含 Claude 特定说明。

**ARCH-065** 🟢 建议
- 文档：architecture.md vs README.md
- 问题：README 项目结构列出 `nginx/nginx.conf`、`scripts/init-db.sh`、`scripts/backup-db.sh`，但 architecture.md §3 未提及。
- 修复：在 architecture.md §3 补充目录说明。

---

### 3.2 设计文档（DESIGN-001 ~ DESIGN-040）

#### 3.2.1 内容前后矛盾

**DESIGN-001** 🔴 阻塞
- 文档：design-system.md vs ui-spec.md
- 章节：design-system.md「## 设计令牌 → 圆角」vs ui-spec.md「## Global Design Tokens」
- 问题：圆角阶梯不一致：design-system.md `--radius-sm 3px / --radius-md 4px / --radius-lg 8px / --radius-xl 12px`；ui-spec.md `rounded-sm 4 / rounded 6 / rounded-md 8 / rounded-lg 12`。同名 token px 值完全不同。
- 修复：以 design-system.md 为准，修正 ui-spec.md。

**DESIGN-002** 🟡 警告
- 文档：design-system.md
- 章节：「## 设计令牌 → 颜色」+「### 自定义 Token」
- 问题：颜色令牌同时存在 shadcn 标准 token（`--background`/`--primary` 等）与 GitHub 风格自定义 token（`--canvas-default` 等）两套并行体系，前者形同死代码。
- 修复：删除未使用的 shadcn 标准颜色行，或将 shadcn token 映射到自定义 token。

**DESIGN-003** 🟡 警告
- 文档：design-system.md
- 章节：「## 组件规范」
- 问题：组件规范引用 `bg-primary text-primary-foreground`、`bg-card`、`border-input` 等，但 design-system.md 颜色表未定义 `--primary-foreground`、`--card`、`--input` 等 shadcn 标准 token。
- 修复：补全 shadcn 标准颜色行，或改用自定义 token。

**DESIGN-004** 🟡 警告
- 文档：design-system.md vs architecture.md §10.4
- 问题：design-system.md 规定「TagBadge 圆角 `rounded-full`」（药丸形）；architecture.md §10.4 写「标签 Badge 圆角 6px」。
- 修复：以 design-system.md 为权威修订 architecture.md。

**DESIGN-005** 🟡 警告
- 文档：design-system.md vs ui-spec.md
- 问题：`--max-w: 1440px` vs ui-spec.md 几乎所有页面写「1280px」。`--sidebar-w: 228px` 在 ui-spec.md 内部也不一致（260px / 228px 混用）。
- 修复：统一为 1280px；侧边栏统一为 `var(--sidebar-w)`。

**DESIGN-006** 🟡 警告
- 文档：design-system.md vs ui-spec.md vs ui-design-prompt.md
- 问题：design-system.md 字体栈含中文支持 `'PingFang SC', 'Microsoft YaHei'`；ui-spec.md 和 ui-design-prompt.md 缺中文字体。
- 修复：以 design-system.md 为准统一字体栈。

**DESIGN-010** 🔴 阻塞
- 文档：ui-spec.md
- 章节：「## Page: / 首页 → 响应式规则」（行 156-158）
- 问题：「移动 (≤700px): 单列瀑布流 2 列」——「单列」与「2 列」自相矛盾。
- 修复：改为「瀑布流 2 列」。

**DESIGN-014** 🔴 阻塞
- 文档：ui-spec.md
- 章节：「## Global Design Tokens」（行 14-21）+「## Page: /studio」（行 3822）
- 问题：①行 16 声明「颜色 token：直接复用 architecture.md §10.4」——循环引用，architecture.md §10.4 实际未定义详细值；②行 3822 使用硬编码 `h-[calc(100vh-52px)]` 而非 `var(--header-h)`。
- 修复：①改为「直接复用 design/design-system.md」；②替换硬编码为 token。

**DESIGN-015** 🔴 阻塞
- 文档：ui-spec.md vs architecture.md §10.5
- 章节：ui-spec.md「## Component: MasonryGrid」（行 1818）vs architecture.md §10.5（行 1900）
- 问题：ui-spec.md 用 CSS multi-column（`columns-2 md:columns-3 lg:columns-4`），architecture.md §10.5 规定用 `react-masonry-css` 库。技术选型直接冲突。
- 修复：以 architecture.md §10.5 为技术权威。

**DESIGN-016** 🔴 阻塞
- 文档：ui-spec.md 全文 `md:`/`lg:` 用法
- 问题：文档定义断点「移动 ≤700px / 平板 ≤1100px / PC >1100px」，但 ui-spec.md 大量使用 Tailwind 默认 `md:`/`lg:` 前缀（默认 768/1024），完全不匹配。design-system.md 也未声明自定义 `screens` 配置。
- 修复：在 design-system.md 增加 `screens: { sm: '700px', md: '1100px' }` 配置。

**DESIGN-017** 🟡 警告
- 文档：ui-spec.md「## Component: StudioSidebar」（行 3748）
- 问题：选中态样式 `border-l-3 border-accent-emphasis`，但 Tailwind 默认无 `border-l-3` 类。
- 修复：改为 `border-l-4`。

**DESIGN-018** 🔴 阻塞
- 文档：ui-spec.md vs ui-design-prompt.md
- 章节：ui-spec.md「## Component: ContentTypeGrid」（行 3796）vs ui-design-prompt.md 行 25
- 问题：ui-spec.md 行 3796 明确允许 emoji 作为内容类型图标；ui-design-prompt.md 行 25 明确「禁止 Emoji 装饰」。
- 修复：改为使用 Lucide React 图标。

**DESIGN-022** 🔴 阻塞
- 文档：ui-spec.md 50+ 处「暗色模式适配」子段
- 问题：几乎所有暗色模式段落均写「`canvas-default` -> `canvas-default.dark`」，这是错误概念。CSS 变量暗色模式应通过 `<html class="dark">` 自动切换 token 值，不存在 `canvas-default.dark` 独立 token。
- 修复：删除所有「`X` -> `X.dark`」描述，改为「通过 `<html class="dark">` 切换」。

**DESIGN-023** 🟡 警告
- 文档：ui-spec.md 50+ 处使用 `border-border-destructive`
- 问题：design-system.md 自定义 Token 表从未定义 `--border-destructive`，只定义了 `--destructive`。
- 修复：在 design-system.md 增加 `--border-destructive: var(--destructive)` 映射。

**DESIGN-024** 🟡 警告
- 文档：ui-spec.md「## Component: ContentCard」（行 1764）
- 问题：使用 `bg-black/10`（Tailwind 标准黑色透明度），违反「禁止硬编码」原则，且暗色模式下黑色遮罩在黑色背景上不可见。
- 修复：改用 `bg-foreground/10`。

**DESIGN-027** 🟡 警告
- 文档：ui-design-prompt.md vs design-system.md vs ui-spec.md
- 问题：圆角阶梯三处不一致；间距阶梯 design-system.md 未定义。
- 修复：统一为单一来源（建议放 design-system.md）。

**DESIGN-038** 🟡 警告
- 文档：design-system.md vs ui-spec.md
- 问题：design-system.md「核心原则 5：统一圆角 `rounded-lg`（8px）为默认」，但 ui-spec.md ContentCard 二创/原创区卡片均用 `rounded-md`。design-system.md 内部前后矛盾。
- 修复：明确 ContentCard 圆角，统一描述。

#### 3.2.2 表述模糊不清

**DESIGN-007** 🟡 警告
- 文档：design-system.md「## 暗色模式检查清单」
- 问题：缺少 Tailwind 自定义断点配置、WCAG 对比度标准、图片/图标暗色滤镜、Modal 遮罩 dark 模式适配。
- 修复：扩展为「暗色模式深度规范」章节。

**DESIGN-008** 🟡 警告
- 文档：design-system.md「## 组件规范」
- 问题：组件规范仅覆盖 6 类基础组件，但 ui-spec.md 列出 50+ 组件、architecture.md §3.1 列 30+ 组件。design-system.md 自称「唯一设计权威」但实际覆盖率不足 20%。
- 修复：明确「组件规范以 ui-spec.md `## Component:` 章节为权威，本文件仅定义基础原语」。

**DESIGN-009** 🟡 警告
- 文档：design-system.md「## 交互模式」
- 问题：交互模式表仅 5 行，缺少组件层面状态视觉规范细节（骨架屏形状、Empty 图标库映射、Error 触发边界、过渡动效时长）。
- 修复：扩展为深度规范。

**DESIGN-019** 🔴 阻塞
- 文档：ui-spec.md 多个组件章节
- 问题：Header、FacetedSearchSidebar、TagBadge、IPCard 等 50+ 组件章节呈现高度雷同的「模板默认值」填充，违反 ui-design-prompt.md Props 命名风格要求，且让功能完全不同的组件规格完全相同。
- 修复：对 50+ 组件逐一重写，给出真实业务 Props。

**DESIGN-022** （已并入上方）

**DESIGN-039** 🟡 警告
- 文档：ui-spec.md + ui-design-prompt.md
- 问题：约 30 个页面的「数据加载策略」子段完全相同（一行通用文案），未区分 SSR/SSR+SWR/CSR/ISR/Streaming 场景。
- 修复：每个页面单独标注加载策略类型并提供 SWR key 示例。

#### 3.2.3 关键信息缺失

**DESIGN-011** 🔴 阻塞
- 文档：ui-spec.md
- 问题：缺少 15 个 architecture.md §3.1 路由清单中列出的页面规格：`/verify-email`、`/terms`、`/privacy`、`/help`、`/client`、`/feedback`、`/feedback/[feedbackId]`、`/feedback/mine`、`/studio/favorites`、`/original/[contentId]/fanworks`、`/admin/dashboard`、`/admin/reports`、`/admin/feedback`、`/admin/audit-logs`、`/admin/queue`。实际前端代码已实现这些路由。
- 修复：补全 15 个缺失页面规格。

**DESIGN-012** 🔴 阻塞
- 文档：ui-spec.md
- 问题：`/dashboard/contents`、`/dashboard/pr-requests`、`/dashboard/contributors`、`/dashboard/tag-suggestions` 4 个旧页面未加废弃标记；对应 `/studio/*` 4 个新页面规格缺失。
- 修复：①为 4 个旧页面加废弃标记；②补全 4 个新页面规格。

**DESIGN-013** 🟡 警告
- 文档：ui-spec.md
- 问题：同一页面 `/messages` 存在两个 `## Page:` 章节（行 1312 + 行 3985），第二个标题保留历史任务编号字样。
- 修复：合并两章节为单一 heading。

**DESIGN-020** 🔴 阻塞
- 文档：ui-spec.md
- 问题：页面规格引用了 14+ 个组件，但 ui-spec.md 无对应 `## Component:` 章节：Footer、MarkdownEditor、TagInput、IPSelector、OriginalSourceSelector、StatsCard、ViewsTrendChart、ContentRankList、PendingTasksCard、FollowerTrendChart、FollowerSourceChart、NotificationBell、SSENotificationProvider、DiscussionBoard、MergeEditor。
- 修复：补全 14+ 个缺失组件章节。

**DESIGN-035** 🔴 阻塞
- 文档：全部 4 份设计文档
- 问题：均无无障碍规范章节（已 Grep 验证）。中文社区项目对视障、色弱、键盘用户是显著合规缺口。
- 修复：在 design-system.md 增加「无障碍规范」章节，至少包含 WCAG 2.1 AA 对比度、ARIA 规范、键盘导航、屏幕阅读器适配。

**DESIGN-036** 🔴 阻塞
- 文档：ui-spec.md + design-system.md
- 问题：①design-system.md 组件规范只覆盖 6 类；②ui-spec.md 中 14+ 组件无 Component 章节；③大量组件章节是模板默认值。三者叠加导致「权威链断裂」。
- 修复：①明确列出待补清单阻塞任务；②补全缺失组件；③重写模板化组件；④在 AGENTS.md 增加缺失规格阻塞流程。

**DESIGN-040** 🟢 建议
- 文档：全部 4 份设计文档
- 问题：均无「文档版本与变更日志」机制，design-system.md 修订时依赖文档无法感知版本漂移。
- 修复：每份文档顶部增加「版本号 + 最后更新日期 + 变更摘要 + 依赖文档版本」字段。

#### 3.2.4 技术方案过时

**DESIGN-025** 🔴 阻塞
- 文档：ui-design-prompt.md
- 问题：多处声明「design-system.md 已归档至 `docs/archive/design-system.md`」，但实际：①`docs/archive/` 目录下不存在该文件；②当前 `design/design-system.md` 是活文档（版本 2.0）。
- 修复：删除所有「已归档」字样；改为「design/design-system.md（当前权威）」。

**DESIGN-026** 🟡 警告
- 文档：ui-design-prompt.md
- 问题：仍假设 task.json 是活动任务源，引用 Task ID。但 AGENTS.md 明确 task.json 已 100% 完成不再修改。
- 修复：在顶部增加「模式说明」，明确新规格变更通过 Beta 路线图驱动。

**DESIGN-028** 🟢 建议
- 文档：ui-design-prompt.md 行 168-169
- 问题：校验命令 `grep -c "^## " design/ui-spec.md # 应输出 81`，但当前已有 95 个 heading。
- 修复：更新为动态阈值或改用「检查关键 heading 存在性 + 无重复」脚本。

**DESIGN-031** 🟡 警告
- 文档：doc-review-prompt.md
- 章节：校验项 3「task.json 覆盖率」+ 校验项 7「数据库 Schema」
- 问题：仍要求按 task.json 执行新功能覆盖率校验，引用「Task 63 的 023-029，Task 75/85 的 030」等过时 Task ID。实际 migrations 已扩展到 056。
- 修复：拆分为「3a 历史 task.json 完整性」+「3b Beta 路线图覆盖率」；校验项 7 改为「migrations 连续性 + architecture.md §4 一致性」。

**DESIGN-032** 🟡 警告
- 文档：doc-review-prompt.md 行 44-46
- 问题：归档声明仅列 `UI Design.md` 和 `homepage-v0.html`，未提及 `CURRENT_TASK_HANDOFF_*.md`、不存在的 `docs/archive/design-system.md`、`docs/iteration-review/2026-06-28-*`。
- 修复：补全归档清单。

#### 3.2.5 失效引用

**DESIGN-029** 🟡 警告
- 文档：doc-review-prompt.md 行 38
- 问题：将「CLAUDE.md / AGENTS.md」并列为同一类文档，未说明引用哪个 CLAUDE.md。
- 修复：明确引用根目录 AGENTS.md 为权威。

**DESIGN-030** 🟡 警告
- 文档：doc-review-prompt.md
- 问题：引用 `docs edited history.md`，但实际路径为 `docs/history/docs edited history.md`（含空格）。
- 修复：统一路径。

#### 3.2.6 关键信息缺失（doc-review-prompt.md）

**DESIGN-033** 🟡 警告
- 文档：doc-review-prompt.md 校验项 5
- 问题：未提供 ui_spec_ref → ui-spec.md heading 反向索引清单，无法自动化。未覆盖「孤儿页面」「废弃路由」检查。
- 修复：增加反向检查子项。

**DESIGN-034** 🟡 警告
- 文档：doc-review-prompt.md 校验项 6
- 问题：仅检查加载/错误/空状态，未覆盖无障碍、i18n、结构化日志、错误信封、优雅关机。
- 修复：扩展为 6a/6b/6c/6d 子项。

#### 3.2.7 术语不一致

**DESIGN-021** 🟡 警告
- 文档：ui-spec.md vs architecture.md §3.1
- 问题：组件命名不一致：architecture.md 列 `Sidebar.tsx`（标签筛选侧边栏），ui-spec.md 用 `FacetedSearchSidebar`。Agent 按名称 grep 找不到匹配。
- 修复：增加「组件命名对照表」。

**DESIGN-037** 🟢 建议
- 文档：全部 4 份设计文档
- 问题：术语不一致：「卡片 / Card / 卡片组件」、「Modal / Dialog / 弹窗 / 抽屉 / Sheet」、「Header / 顶部导航 / 顶部区域」混用。
- 修复：在 design-system.md 增加「术语对照表」。

---

### 3.3 Beta 路线图与计划（PLAN-001 ~ PLAN-020）

#### 3.3.1 内容前后矛盾

**PLAN-001** 🔴 阻塞
- 文档：2026-05-30-omnicraft-dual-track-beta-roadmap.md
- 章节：`## Migration Numbering Note`
- 问题：声明「迁移目录使用 049-052 空闲槽位」，但实际已扩展到 056。
- 修复：在 Note 追加 053-056 来源说明；R-01 验证范围扩展为 001-056 全量。

**PLAN-002** 🔴 阻塞
- 文档：2026-05-30-omnicraft-beta-implementation-notes.md
- 章节：§4.1 Shared File Reservations
- 问题：明确「保留 049-052 编号 exactly」，但 053-056 已被引入，违反保留约束。
- 修复：追加注释「2026-06 后续计划已扩展保留区间至 056；新增任务从 057 起编号」。

**PLAN-011** 🟢 建议
- 文档：roadmap vs implementation-notes
- 问题：G-05 的 Depends On 字段为 G-01（暗示可并行），但 implementation-notes §3.2 要求串行在 G-04 之后。
- 修复：以 implementation-notes 为准更新 roadmap G-05 依赖为 `G-01, G-04`。

#### 3.3.2 关键信息缺失

**PLAN-003** 🟡 警告
- 文档：backend/migrations/056_conversation_indexes.sql
- 问题：迁移文件存在但所有 16 个 plan 文档均未提及来源、所属任务 ID、创建原因。属「幽灵迁移」。
- 修复：补充子计划说明 056 归属。

**PLAN-004** 🟡 警告
- 文档：roadmap `## New Config Field Registry`
- 问题：声明为「authoritative registry」，但 config.yaml 中至少 15+ 字段未登记（包括整个 `queue.*` 块和大部分 `agent.*` 字段）。
- 修复：补全字段登记；或在 registry 顶部声明「仅登记 Beta 新增字段」。

**PLAN-005** 🟡 警告
- 文档：roadmap Config Field Registry
- 问题：登记 `deploy.grant_ttl_sec` 等三个 D-02/D-03 字段，但 config.yaml 完全无 `deploy:` 块。
- 修复：在 registry 标注「Status: pending D-02/D-03 implementation」。

**PLAN-019** 🟢 建议
- 文档：roadmap `## Cross-Plan File Conflict Matrix`
- 问题：未包含 `auth.go`、`middleware/auth.go`、`verification_service.go`、`AgentChatWidget.tsx`、`AuthContext.tsx` 等高频共享文件。
- 修复：在 Matrix 表中追加这些文件归属规则。

#### 3.3.3 技术方案过时

**PLAN-006** 🟡 警告
- 文档：2026-06-03-omnicraft-web-beta-review-repair-plan.md
- 问题：7 个 Task 步骤均保持 `- [ ]` 未勾选，但 progress.txt 明确记录 7 个 Task 已于 2026-06-03 至 2026-06-04 执行完毕。
- 修复：批量勾选并追加 ✅ 完成状态 banner。

**PLAN-007** 🟡 警告
- 文档：2026-06-05-omnicraft-ui-detail-repair-plan.md
- 问题：11 个 Task 步骤均未勾选，但 progress.txt 记录 Task 11 已执行（推断 1-10 也已完成）。
- 修复：与 PLAN-006 同步处理。

**PLAN-008** 🟢 建议
- 文档：4 个 2026-06-08 security plan
- 问题：文档头有 ✅ 完成状态 banner，但内所有 `- [ ]` checkbox 仍保持未勾选。机械化扫描工具会误判为未完成。
- 修复：将所有 `- [ ]` 改为 `- [x]`，与 banner 一致。

**PLAN-012** 🟢 建议
- 文档：2026-05-07-omnicraft-code-review.md
- 问题：声明「86 total, 24 passes: false」，但当前所有任务 passes: true。会误导新 Agent。
- 修复：移动到 docs/archive/ 或在文档头加 ⚠️ 历史文档 banner。

**PLAN-013** 🟢 建议
- 文档：2026-05-30-omnicraft-beta-plan-review.md
- 问题：2026-06-28 状态更新写「大部分已解决」，但「大部分」措辞模糊，未逐项标注状态。
- 修复：对 24 项问题逐项标注当前状态（✅ / ⚠️ / ❌ / ⏭️）。

**PLAN-018** 🟢 建议
- 文档：specs/2026-05-30-omnicraft-dual-track-beta-design.md §3.2
- 问题：列出 Task 156-168 作为「仍需回归清单」，但 AGENTS.md 称「仅历史保留」。
- 修复：在 §3.2 追加 banner 对齐表述。

#### 3.3.4 设计逻辑不合理

**PLAN-010** 🟢 建议
- 文档：admin-operations plan + roadmap
- 问题：A-03 依赖 A-04，与数字 ID 顺序（A-03 < A-04）相反。按 ID 顺序选择任务的 Agent 会困惑。
- 修复：在 roadmap Execution Queue A-03 行追加注释「Depends on A-04 (intentional)」。

**PLAN-016** 🟢 建议
- 文档：backend/config.yaml line 3
- 问题：`server.mode: "debug"`，Release Gate 只在 release 模式下执行严格校验。本地无法验证 release 行为。
- 修复：在 production-config-template.md 明确生产 `server.mode: "release"`。

#### 3.3.5 模糊不清

**PLAN-009** 🟢 建议
- 文档：用户审查清单第 17 项
- 问题：将 `2026-06-09-security-hardening-execution.md` 列为 plan 文件，但实际位于 `docs/superpowers/progress/` 目录。
- 修复：在审查报告中说明正确路径。

**PLAN-014** 🟢 建议
- 文档：2026-06-08-omnicraft-abuse-control-no-load-testing.md
- 问题：引入 `rate_limit.max_search_page` 和 `security.trusted_proxies`，但 roadmap Config Field Registry 未登记。
- 修复：与 PLAN-004 一并补入 registry。

**PLAN-015** 🟢 建议
- 文档：2026-06-05-omnicraft-ui-detail-repair-plan.md
- 问题：Tech Stack 声明「Next.js 16」，但 dependency 升级 plan 升级到 `16.2.7`。版本粒度差异。
- 修复：在 Tech Stack 行追加版本注释。

**PLAN-017** 🟢 建议
- 文档：2026-06-08-omnicraft-oss-upload-download-hardening.md
- 问题：File Structure 列出 `upload_grant_service.go`，与 V-05 `feedback.upload_grant_ttl_sec` 命名相似易混淆。
- 修复：在 File Structure 追加注释明确两者边界。

**PLAN-020** 🟢 建议
- 文档：verification-feedback plan V-05 vs OSS hardening plan
- 问题：roadmap 说「feedback-specific presign endpoint」，OSS plan 说统一 `UploadGrantService` 加 `purpose` 隔离。两份文档对 feedback screenshot grant 实现位置表述不一致。
- 修复：在 roadmap 或 OSS plan 追加交叉引用注释。

#### 3.3.6 通过项

✅ **模式选择与跟踪规则一致性**：roadmap 与 implementation-notes 关于「不修改 task.json、只勾选 Beta checkbox」规则一致。
✅ **任务 ID 命名空间**：F/V/A/G/D/R 六个前缀在 roadmap 与各 subsystem plan 之间一一对应。
✅ **F-01 至 F-06、V-01 至 V-06、A-01 至 A-05、G-01 至 G-05 子任务 ID 与文件归属**：迁移编号与实际文件一致（除 053-056 超范围）。
✅ **历史 task.json 与 Beta 计划集的冲突**：task.json 全部 passes: true，Beta 模式明确不动 task.json。

---

### 3.4 历史任务账本与进度（TASK-001 ~ TASK-027）

#### 3.4.1 依赖错误

**TASK-001** 🔴 阻塞
- 文档：task.json Task 169（行 3215）
- 问题：声明审查范围 151-168（18 个任务），但 `depends_on` 数组（16 项）缺失 159 和 168。168 是「第二轮回归测试」、159 是「第一轮回归测试」。
- 修复：在 `depends_on` 补全 `159` 与 `168`，或显式说明排除原因。

**TASK-002** 🟡 警告
- 文档：task.json Task 168（行 3184-3195）
- 问题：声明覆盖 156-167（12 个任务），但 `depends_on`（10 项）缺失 158 和 159。158 是 DB schema 修复任务。
- 修复：补全 `158` 与 `159`。

**TASK-006** 🟡 警告
- 文档：task.json Task 22/23/25/64/76/77
- 问题：6 个低 ID 任务依赖高 ID 任务（反向依赖）。history.md 标注为「必须修复」但实际「保留」。
- 修复：在 AGENTS.md 模式 B 规则中明确「依赖图按 depends_on 解析，不按 ID 顺序」。

**TASK-023** 🟢 建议
- 文档：task.json Task 1-19、33-35（约 23 个任务）
- 问题：均无 `depends_on` 字段。串行执行无问题，并行执行可能引发竞态。
- 修复：若引入并行执行需补全。

#### 3.4.2 内容前后矛盾

**TASK-003** 🔴 阻塞
- 文档：task.json Task 30（行 578-585）
- 问题：`ui_spec_ref` 仍包含 `## Component: ContentDetail`（history.md 模块 AH.2 称已移除，是回归）。同时 `ui_spec_ref` 包含 `## Component: FollowButton`，但 description 明确「FollowButton 由 Task 84 负责」，自相矛盾。
- 修复：移除 `## Component: ContentDetail` 与 `## Component: FollowButton`。

**TASK-010** 🟡 警告
- 文档：task.json Task 30 step 2 vs ui_spec_ref
- 问题：step 2 引用「Task 24 的 ContentDetail」作集成目标（合理），ui_spec_ref 把 ContentDetail 当作本任务规格（不合理）。
- 修复：移除 ui_spec_ref 中的 `## Component: ContentDetail`。

**TASK-016** 🔴 阻塞
- 文档：docs/history/docs edited history.md 模块 AH.2（行 622-625）
- 问题：声明「Task 30 ui_spec_ref 移除 ContentDetail」，但当前 task.json 仍包含（参见 TASK-003）。修复声明与实际不符，是回归。
- 修复：从 task.json 移除 ContentDetail 并在 history.md 追加回归记录。

#### 3.4.3 技术方案过时 / 文件路径错误

**TASK-004** 🔴 阻塞
- 文档：task.json Task 139 step 1（行 2718）、Task 144 step 1（行 2793）
- 问题：迁移文件编号严重不符：
  - Task 139 指定 `migrations/041_content_soft_delete.sql`，但 041 实际为 `content_search_vector.sql`（Task 119 创建），实际软删除迁移是 053。
  - Task 144 指定 `migrations/042_user_id_int64.sql`，但 042 实际为 `ips_search_vector.sql`（Task 119），且 054 内容是「users 软删除」而非「user_id 类型变更」。
- 修复：将 Task 139 step 1 改为 `migrations/053_content_items_soft_delete.sql`；将 Task 144 step 1 改为符合实际实现的内容。

**TASK-015** 🟡 警告
- 文档：docs/history/docs edited history.md 模块 AR.2、AV
- 问题：记录「Task 66 创建 migrations/035_conversation_indexes.sql」，但 task.json Task 66 step 1 实际为 `056_conversation_indexes.sql`（含注释「编号从 035 改为 056」）。history.md 未同步。
- 修复：在 history.md 补注实际编号变更。

**TASK-018** 🟡 警告
- 文档：docs/history/docs edited history.md 第七轮校验总结（行 949-954）
- 问题：声明「所有已识别问题均已修复」，但 Task 30 仍含 ContentDetail（参见 TASK-003）。history.md 已不可信。
- 修复：在 history.md 顶部增加警告「本文件记录截至第七轮，后续 task.json 编辑可能未回填」。

#### 3.4.4 关键信息缺失

**TASK-005** 🔴 阻塞
- 文档：task.json Task 151-169 vs progress.txt
- 问题：19 个任务全部 `passes: true`，但 progress.txt 完全无对应记录。违反 AGENTS.md Step 5/6 强制要求。
- 修复：为 Task 151-169 每个任务在 progress.txt 顶部补建符合模板的条目。

**TASK-012** 🔴 阻塞
- 文档：progress.txt
- 问题：progress.txt 共 122 个任务条目，而 task.json 有 169 个 `passes: true`。约 47 个任务完全缺失进度记录。
- 修复：对未记录任务补建进度条目。

#### 3.4.5 格式不规范

**TASK-007** 🟡 警告
- 文档：task.json Task 9、Task 30
- 问题：字段顺序不统一。Task 9：`id → depends_on → title → description → steps → passes`；Task 30：`id → depends_on → ui_spec_ref → title → description → steps → passes`。主流顺序为 `id → title → description → depends_on → steps → ui_spec_ref → passes`。
- 修复：统一为标准顺序。

**TASK-008** 🟡 警告
- 文档：task.json Task 169（行 3232-3296）
- 问题：仅 Task 169 含 `review_findings` 数组，其他 168 个任务均无此字段。任务对象结构不一致。
- 修复：抽离到独立文件 `docs/review/task-169-review-findings.json`。

**TASK-011** 🟢 建议
- 文档：progress.txt 行 2262-2279
- 问题：完整复制 AGENTS.md Step 5 模板作为正文内容嵌入。
- 修复：删除模板段，仅保留一句「记录格式见 AGENTS.md Step 5」。

**TASK-013** 🟢 建议
- 文档：progress.txt 行 1758、2419
- 问题：Task 30 进度信息分散在两个不相关条目，无独立条目。
- 修复：补建独立 Task 30 进度条目。

**TASK-014** 🟢 建议
- 文档：progress.txt
- 问题：Beta ID 与历史 Task ID 混合排列，无分节标题。
- 修复：在文件顶部增加分节标题，或拆分为多个 progress-*.txt 文件。

#### 3.4.6 术语不一致

**TASK-009** 🟢 建议
- 文档：AGENTS.md 行 39
- 问题：表述「100+ 个历史任务」，实际为 169 个。
- 修复：改为「169 个历史任务（ID 1-169）」。

**TASK-017** 🟡 警告
- 文档：docs/history/docs edited history.md
- 问题：模块 5.2 标注反向依赖为「必须修复」，但模块 AU/7.4 又决定「保留」。术语「必须修复」与实际「保留」语义冲突。
- 修复：将标题改为「倒序依赖（已通过显式 depends_on 缓解）」。

#### 3.4.7 模糊不清

**TASK-026** 🟢 建议
- 文档：task.json 所有任务描述与 steps
- 问题：抽检 Task 41/42/99-105 后，部分任务验收标准较模糊。如 Task 41 step 7「数据一致性正确」未定义具体检查项。
- 修复：在新任务规范中要求「验证」步骤必须包含可执行命令或具体可观察输出。

#### 3.4.8 通过项

✅ **TASK-019 ui_spec_ref 引用**：所有 40 个 ui_spec_ref 字符串均能在 ui-spec.md 找到精确匹配。
✅ **TASK-021 字段命名**：所有字段统一使用 snake_case。
✅ **TASK-022 ID 连续性**：ID 1-169 完全连续，无跳号重复。
✅ **TASK-025 Task 66 迁移编号**：task.json 自身已自洽（含注释）。
✅ **TASK-027 功能去重**：Task 30 已与 Task 84/85 去重；browse_history 已从 Task 16 移至 Task 85。

---

### 3.5 审查报告 / 部署运维 / Agent 工作流（REVIEW-001 ~ REVIEW-047）

#### 3.5.1 报告间矛盾结论

**REVIEW-001** 🔴 阻塞
- 文档：docs/review/beta-release-validation-2026-05-30.md vs docs/review/web-beta-review-summary
- 问题：beta-release-validation 标 `Decision: ✅ PASS`，web-beta-review-summary 标 `GO-WITH-BLOCKERS`。同周期两份顶层报告结论冲突。
- 修复：将 beta-release-validation 的 Decision 改为 `SUPERSEDED`，保留历史但消除歧义。

**REVIEW-002** 🔴 阻塞
- 文档：docs/review/beta-baseline-2026-05-30.md vs docs/review/beta-release-validation-2026-05-30.md
- 问题：同日两份报告结论冲突。baseline 报告测试失败、JSX 语法错误、cargo test 失败、6 项外部依赖未配置；validation 同日标 `✅ PASS` 并声称全部成功。
- 修复：核实当天真实测试结果，明确两份报告的时序关系。

**REVIEW-003** 🟡 警告
- 文档：docs/2026-06-28-comprehensive-documentation-audit.md vs docs/2026-06-24-omnicraft-design-review-merged.md
- 问题：2026-06-24 识别 47 个问题，2026-06-28 识别 74 个问题，间隔仅 4 天。2026-06-28 未说明与 2026-06-24 的关系。
- 修复：在 2026-06-28 audit 报告头部明确为「补充/独立审查/覆盖更新」并附交叉引用映射表。

**REVIEW-004** 🟡 警告
- 文档：docs/review/beta-release-validation-2026-05-30.md
- 问题：文件名日期 `2026-05-30`，正文 Date 字段写 `2026-06-01`。
- 修复：统一文件名与正文日期。

**REVIEW-005** 🔴 阻塞
- 文档：beta-release-validation vs web-beta-review-03 vs web-beta-review-summary
- 问题：三方对 `ValidateRelease()` 修复状态判断不一致。beta-release-validation 声称已调用，web-beta-review-03 标记为 HIGH 未调用，summary 列为 P0 blocker。
- 修复：直接核查 backend 代码中 `ValidateRelease()` 实际调用点；更新所有报告状态。

**REVIEW-006** 🔴 阻塞
- 文档：web-beta-review-09 BUG-09-3 vs beta-release-validation 第 222 行
- 问题：beta-release-validation 声称 `reports.action_taken` 列已添加；web-beta-review-09 报告 PATCH 仍因该列缺失返回 500。
- 修复：核查 migrations 是否包含为 reports 添加 action_taken 列的迁移；以实际 schema 为准修正报告。

**REVIEW-007** 🔴 阻塞
- 文档：web-beta-review-04 V-05-3 vs web-beta-review-09 BUG-09-1
- 问题：web-beta-review-04 标记 admin reply/close 不发通知为 HIGH；web-beta-review-09 报告 anonymous feedback 500 bug。两份报告间隔 1 天，前者未发现严重 bug。
- 修复：核查 feedback_service.go 中 `*int64` 类型 user_id 的 GORM insert 处理逻辑；统一修复状态。

#### 3.5.2 已修复问题未真正落地

**REVIEW-008** 🔴 阻塞
- 文档：docs/2026-06-24-design-review-decisions.md DEC-004 vs reputation_service.go vs config.yaml
- 问题：DEC-004 标注信誉分配置字段已实施，但 2026-06-28 audit 仍发现 reputation_service.go 中 12 处加减分值硬编码。config.yaml 缺少 bonus/penalty 子字段。
- 修复：在 config.yaml 添加 bonus/penalty 结构化配置；将硬编码改为读配置；更正 DEC-004 状态。

**REVIEW-009** 🔴 阻塞
- 文档：web-beta-review-05 F05-1 vs db-devops-perf-review C2
- 问题：db-devops-perf-review 2026-05-22 已识别 `ContentVisibilityWhere` 使用 `fmt.Sprintf` 拼接 SQL 的注入风险（CRITICAL C2），但 web-beta-review-05（2026-06-02）F05-1 仍报告该问题存在。CRITICAL 级别问题 11 天未修复。
- 修复：立即改为 GORM 参数化查询；在 db-devops-perf-review 标注 C2 已修复日期。

**REVIEW-010** 🟡 警告
- 文档：web-beta-review-06 A01-1 vs web-beta-review-07 Issue #1
- 问题：两份报告重复发现同一问题（PatchFeedback 非事务性审计），但严重程度判定不一致（CRITICAL vs Moderate）。
- 修复：统一为 CRITICAL（违反 A-02 契约）；在 admin_feedback.go 改用 RecordTx 事务性审计方法。

**REVIEW-011** 🔴 阻塞
- 文档：backend-api-audit-2026-05-22 vs security-audit-2026-05-22
- 问题：err.Error() 暴露统计：backend-api-audit 报告 162 处，security-audit 报告 154+ 处。Task 102 标注已完成，但 2026-06-02 web-beta-review-01 P1-02 仍报告 SafeErrorMsg 可能暴露 err.Error()。
- 修复：全量扫描 backend/internal/handler/*.go 中所有 err.Error()；统一替换；验证 Task 102 完成状态。

**REVIEW-012** 🔴 阻塞
- 文档：docs/2026-06-28-comprehensive-documentation-audit.md N6
- 问题：第 4 轮 H5 修复本意消除字段脱节，反而引入 config.go 中根本不存在的字段（worker_notif 等）。
- 修复：删除 config.go 中所有幽灵字段；核查所有引用；在 newly-found-issues.md 追踪 N6 至关闭。

**REVIEW-013** 🟡 警告
- 文档：docs/2026-06-28-comprehensive-documentation-audit.md N7
- 问题：token 命名错误在 30+ 处仍残留。
- 修复：全量 grep token 相关命名；统一为规范名称。

**REVIEW-014** 🔴 阻塞
- 文档：web-beta-review-01 P0-01 vs web-beta-review-summary
- 问题：AuthRequired 在 Redis token-revocation 检查时 fail-open 为 P0 blocker；summary 仍列。修复状态不一致。
- 修复：核查 backend/internal/middleware/auth.go；fail-open 必须改为 fail-closed（Redis 不可用时拒绝请求）。

#### 3.5.3 版本号 / 阈值 / 配置字段一致性

**REVIEW-015** 🔴 阻塞
- 文档：architecture.md §7 vs backend/config.yaml
- 问题：`quality_content_threshold` 在 architecture.md 标注为 50，config.yaml 实际为 10，差 5 倍。
- 修复：核实 PRD 原始需求；统一两处；若 config 为准则更新 architecture.md。

**REVIEW-016** 🟡 警告
- 文档：docs/PLAN.md vs docker-compose.single-server.yml vs nginx.conf
- 问题：健康检查端点路径不一致：PLAN.md 用 `/health`，docker-compose 和 nginx 用 `/healthz`。
- 修复：统一为 `/healthz`；更新 PLAN.md 第 143 行。

**REVIEW-017** 🔴 阻塞
- 文档：docker-compose.single-server.yml vs production-config-template.md vs single-server-beta-runbook.md
- 问题：CONFIG_OVERRIDE_PATH 取值不一致：docker-compose 为 `/app/config_override.yaml`，production-config-template 为 `/var/lib/omnicraft/config_override.yaml`，runbook 为 `/app/config_override.yaml`。生产部署时 backend 无法找到 override 配置。
- 修复：统一为 `/app/config_override.yaml`。

**REVIEW-018** 🟡 警告
- 文档：backend/config.yaml vs docs/async-queue-analysis.md DEC-019
- 问题：DEC-019 决定启用 Redis Streams 异步队列，但 config.yaml 中 `queue.enabled` 仍为 `false`。
- 修复：核查实施状态；若已实施改为 true 并补 worker_count 等配置；若未实施更新 DEC-019 状态。

**REVIEW-019** 🟡 警告
- 文档：docs/oss-lifecycle.md vs production-config-template.md
- 问题：oss-lifecycle.md 引用 `download_url_ttl_sec: 300` 和 `presign_expire_sec: 3600`，但 production-config-template.md 未列出这两个字段。
- 修复：在 production-config-template.md 配置模板中补充这两个字段。

#### 3.5.4 报告日期合理性

**REVIEW-020** 🔴 阻塞
- 文档：docs/futureWork/2026-05-31-omnicraft-beta-future-work.md
- 问题：文档日期 2026-05-31 声称「F-01 至 R-01 全部已完成」，但 2026-06-28 audit 发现第 5/6 轮整体未执行，24 项问题完全未处理。
- 修复：在文档顶部加大「⚠️ 已被 2026-06-28 audit 推翻」提示；修订为实际完成状态。

**REVIEW-021** 🟡 警告
- 文档：docs/review/code-review-log.md
- 问题：所有 6 个批次审查日期为 2026-05-07，多批次标「Pass with Follow-up」。但 2026-05-22 至 06-02 期间多份报告发现大量新问题，follow-up 未真正完成，code-review-log 未更新关闭状态。
- 修复：为每个 follow-up 项添加实际关闭日期或转为未关闭问题列表。

**REVIEW-022** 🟡 警告
- 文档：docs/review/db-devops-perf-review-2026-05-22 vs nginx.conf
- 问题：报告「Nginx 无 rate limiting」为 HIGH，但实际 nginx.conf 已配置 3 个 limit_req_zone（api_limit 30r/s、auth_limit 5r/s、search_limit 10r/s）。
- 修复：在 db-devops-perf-review 标注「已于 X commit 修复」。

#### 3.5.5 部署文档缺失

**REVIEW-023** 🔴 阻塞
- 文档：docs/deploy/single-server-beta-runbook.md
- 问题：.env 模板仅含 SMTP_PASSWORD，缺少 SMTP_HOST、SMTP_PORT、SMTP_USER、SMTP_FROM_ADDRESS。AGENTS.md 阻塞处理规则明确「SMTP 缺失」为阻塞项。
- 修复：在 .env 模板补充完整 SMTP_* 变量；在 Release Gate Checklist 加入「SMTP_* 全部已配置」检查项。

**REVIEW-024** 🟡 警告
- 文档：single-server-beta-runbook.md 第 6 步
- 问题：引用 `nginx/omnicraft.single-server.conf`，实际文件位于 `docs/deploy/nginx.omnicraft.single-server.conf`。
- 修复：更新 runbook 中的路径。

**REVIEW-025** 🔴 阻塞
- 文档：docs/deploy/nginx.omnicraft.single-server.conf
- 问题：配置了 HSTS、X-Content-Type-Options、X-Frame-Options、Referrer-Policy，但**未配置 Content-Security-Policy (CSP)**。AGENTS.md 安全规则要求全面安全策略。
- 修复：在两个 server 块添加 `add_header Content-Security-Policy "default-src 'self'; ..."` always。

**REVIEW-026** 🟡 警告
- 文档：nginx.omnicraft.single-server.conf
- 问题：api 路径通用 `proxy_read_timeout 300s`，未针对 SSE 长连接端点单独配置。SSE 端点在 300s 后被强制断开。
- 修复：为 SSE 端点添加独立 location 块，配置 `proxy_read_timeout 3600s; proxy_buffering off;`。

**REVIEW-027** 🟢 建议
- 文档：nginx.omnicraft.single-server.conf
- 问题：未配置 CORS 头，依赖应用层处理。预检请求需穿透到后端。
- 修复：可选方案 — 在 nginx 添加简单 OPTIONS 处理；或保持现状并记录设计决策。

**REVIEW-028** 🔴 阻塞
- 文档：single-server-beta-runbook.md
- 问题：缺少：(1) 回滚步骤；(2) 监控告警配置；(3) 日志收集策略；(4) Redis 备份策略；(5) OSS 备份策略。
- 修复：补充「第 9 步：监控告警」「第 10 步：日志收集」「第 11 步：回滚流程」「第 12 步：Redis 备份」。

**REVIEW-029** 🟡 警告
- 文档：single-server-beta-runbook.md 第 7 步、第 8 步
- 问题：验证步骤仅检查服务启动，缺少功能冒烟测试。备份步骤缺少恢复测试。
- 修复：在第 7 步加入最小冒烟测试脚本；在第 8 步加入「每季度恢复演练」说明。

**REVIEW-043** 🔴 阻塞
- 文档：single-server-beta-runbook.md
- 问题：完全缺少回滚流程。
- 修复：新增「回滚流程」章节，包含镜像回退、数据库迁移回滚、配置回滚、回滚后验证。

**REVIEW-044** 🔴 阻塞
- 文档：single-server-beta-runbook.md
- 问题：未配置任何监控告警。
- 修复：新增「监控告警」章节，包含基础资源监控、服务健康检查、关键业务指标、告警通知渠道。

**REVIEW-045** 🟡 警告
- 文档：single-server-beta-runbook.md
- 问题：未说明日志收集与轮转策略。
- 修复：新增「日志收集」章节，包含 Docker 日志驱动配置、轮转策略、长期存储、查询示例。

**REVIEW-046** 🟡 警告
- 文档：single-server-beta-runbook.md 第 8 步
- 问题：备份策略仅含 PostgreSQL pg_dump，缺少 Redis、OSS、配置文件备份。
- 修复：补充 Redis 备份、OSS 跨区域复制、config.yaml/.env 加密备份策略。

**REVIEW-047** 🟡 警告
- 文档：production-config-template.md Release Gate Checklist
- 问题：缺少 4 项检查：(1) CLAUDE.md vs AGENTS.md 文件名统一；(2) CONFIG_OVERRIDE_PATH 三处一致；(3) /healthz 已在 backend 路由注册；(4) CSP 头已配置。
- 修复：在 Release Gate Checklist 补充上述 4 项并提供验证命令。

#### 3.5.6 文档重复

**REVIEW-030** 🟡 警告
- 文档：docs/2026-06-24-omnicraft-documentation-review.md vs docs/iteration-review/2026-06-28-*
- 问题：两份报告审查范围重叠，未明确边界关系。
- 修复：明确两份报告定位；在 2026-06-28 报告头部声明延续关系。

**REVIEW-031** 🟡 警告
- 文档：docs/design-review-2026-06-24.md vs docs/2026-06-24-omnicraft-design-review-merged.md
- 问题：编号体系不一致（#1-#17 vs A1-J2），跨文档引用易混淆。
- 修复：在 design-review-2026-06-24.md 顶部添加编号映射表。

**REVIEW-032** 🟡 警告
- 文档：web-beta-review-06 A01-1 vs web-beta-review-07 Issue #1
- 问题：两份连续编号报告重复发现同一问题，严重程度判定不一致。
- 修复：合并为单一条目追踪；统一严重程度。

#### 3.5.7 agents/ 与 AGENTS.md 一致性

**REVIEW-033** 🔴 阻塞
- 文档：docs/agents/domain.md
- 问题：要求 Agent 读取项目根目录的 `CONTEXT.md` 和 `docs/adr/`，但项目实际不存在这些文件。
- 修复：将 CONTEXT.md 引用改为 AGENTS.md；将 docs/adr/ 引用改为 architecture.md。

**REVIEW-034** 🟡 警告
- 文档：docs/agents/issue-tracker.md
- 问题：引用 `CLAUDE.md` 作为项目主指南，但实际文件名为 `AGENTS.md`。
- 修复：统一引用为 AGENTS.md。

**REVIEW-035** 🟢 建议
- 文档：docs/agents/triage-labels.md vs AGENTS.md
- 问题：AGENTS.md 简短提及 triage-labels.md 但未在工作流步骤中引用，导致 Agent 不会主动使用 triage 流程。
- 修复：在 AGENTS.md Step 1 或 Step 6 加入「若任务来自 GitHub Issues，按 docs/agents/triage-labels.md 标注标签」。

#### 3.5.8 archive/ 引用残留

**REVIEW-036** 🟡 警告
- 文档：design/ui-spec.md / design/ui-design-prompt.md
- 问题：仍引用已归档的 `UI Design.md`。AGENTS.md 要求前端实现前必须读取 ui-spec.md，失效引用会误导 Agent。
- 修复：删除引用或替换为「（已归档，权威内容已并入本文档）」。

**REVIEW-037** 🟡 警告
- 文档：docs/iteration-review-prompt.md
- 问题：审查范围仍将 `design/UI Design.md` 列为修改文档，但该文件已归档。
- 修复：从审查范围列表移除；替换为 `design/ui-spec.md`。

**REVIEW-038** 🟢 建议
- 文档：docs/superpowers/specs/2026-05-30-omnicraft-dual-track-beta-design.md
- 问题：引用已归档文件未标注「已归档」状态。
- 修复：在归档文件引用后添加「（已归档，参见 docs/archive/README.md）」标注。

#### 3.5.9 术语不一致

**REVIEW-039** 🟡 警告
- 文档：AGENTS.md / docs/agents/issue-tracker.md / docs/iteration-review-prompt.md
- 问题：项目根目录主指南文件名为 `AGENTS.md`，但多份文档引用 `CLAUDE.md`。
- 修复：全量 grep `CLAUDE.md` 并替换为 `AGENTS.md`。

**REVIEW-040** 🟡 警告
- 文档：AGENTS.md / architecture.md / docs/review/* / design/ui-spec.md
- 问题：同一角色混用「判官」「评审员」「审核员」三种称呼。
- 修复：统一为「判官」；建立 glossary 文件。

**REVIEW-041** 🟡 警告
- 文档：同上
- 问题：同一概念混用「二创」「同人作品」「fanwork」「二次创作」四种称呼。代码字段为 `zone='fanwork'`。
- 修复：统一为「二创」（中文）和 `fanwork`（代码标识符）。

**REVIEW-042** 🟢 建议
- 文档：docs/review/* / docs/futureWork/* / docs/superpowers/plans/*
- 问题：当前阶段为公开 Beta 加固，但多份文档仍使用「MVP」描述当前功能。
- 修复：将描述当前阶段的「MVP」替换为「Beta」；保留「MVP」仅用于历史阶段。

---

## 四、优先级建议

### 4.1 P0 — 立即修复（41 项阻塞问题）

按影响域分组，按修复顺序排序：

#### A. 影响安全 / 发布决策（最高优先级）

1. **REVIEW-014** AuthRequired fail-open（P0 安全漏洞）
2. **REVIEW-009** ContentVisibilityWhere SQL 注入（CRITICAL 11 天未修复）
3. **REVIEW-011** err.Error() 暴露（Task 102 虚假完成）
4. **REVIEW-008** DEC-004 信誉分配置虚假实施（业务逻辑正确性）
5. **REVIEW-012** 第 4 轮修复引入幽灵字段（修复引入新问题）
6. **REVIEW-025** nginx 缺少 CSP 头（XSS 攻击面扩大）
7. **REVIEW-005** ValidateRelease() 修复状态三方矛盾
8. **REVIEW-006** reports.action_taken 列状态矛盾
9. **REVIEW-007** anonymous feedback 500 bug
10. **REVIEW-001/002** 顶层报告结论冲突

#### B. 影响生产部署

11. **REVIEW-017** CONFIG_OVERRIDE_PATH 三处不一致
12. **REVIEW-023** SMTP 环境变量不完整
13. **REVIEW-028/043/044** runbook 缺回滚/监控/告警

#### C. 影响 Schema 正确性

14. **ARCH-003** download_count 字段描述与实现相反
15. **ARCH-047** collections 表 schema 缺失（关键 schema）
16. **ARCH-020** admin 路由未在 protected 内（与 AGENTS.md Task 105 冲突）

#### D. 影响桌面端集成

17. **ARCH-004** URL Scheme 协议三种版本并存

#### E. 影响 Beta 路线图执行

18. **PLAN-001** roadmap 迁移编号声明过时（049-052 实际到 056）
19. **PLAN-002** implementation-notes 保留约束违反
20. **TASK-005/012** Task 151-169 缺失 progress.txt 记录（违反 AGENTS.md Step 5/6）
21. **TASK-004** Task 139/144 迁移文件引用过时
22. **TASK-003/016** Task 30 ui_spec_ref 回归

#### F. 影响设计权威

23. **DESIGN-001** 圆角阶梯不一致
24. **DESIGN-011** ui-spec.md 缺 15 个页面规格
25. **DESIGN-012** 4 个 /dashboard/* 旧页面未加废弃标记
26. **DESIGN-015** MasonryGrid 库选型冲突
27. **DESIGN-016** Tailwind 断点配置缺失
28. **DESIGN-018** emoji 图标违反规范
29. **DESIGN-019** 50+ 组件章节为模板默认值
30. **DESIGN-020** 14+ 组件无 Component 章节
31. **DESIGN-022** 暗色模式描述错误
32. **DESIGN-035** 无障碍规范完全缺失
33. **DESIGN-025** ui-design-prompt.md 整体过时（design-system.md 已归档假前提）

#### G. 影响 Agent 工作流

34. **REVIEW-033** domain.md 引用不存在的 CONTEXT.md
35. **REVIEW-020** futureWork 虚假完成声明
36. **ARCH-002** LLM 配置默认值前后矛盾

#### H. 影响进度追溯

37-41. **TASK-001/002** depends_on 缺失关键回归测试任务

### 4.2 P1 — 短期修复（99 项警告问题）

按修复难度分组：

- **批量替换类**（机械改动，可一次性完成）：
  - REVIEW-039/040/041 术语统一（CLAUDE.md→AGENTS.md、判官、二创）
  - REVIEW-016 健康检查端点统一为 /healthz
  - ARCH-005/006/007/008/009 迁移编号与字段名引用更正
  - ARCH-060/061 AccessKey 命名统一
  - DESIGN-013/017/023 重复 heading、无效 Tailwind 类、未定义 token
  - PLAN-006/007/008 checkbox 批量勾选

- **schema 补全类**（需设计决策）：
  - ARCH-022/023/025/026/027/029 messages/conversations 软删除、comments dislike_count、appeals target_type IP、admin unban API
  - ARCH-032/033/034 users/content_items/comments schema 同步迁移
  - ARCH-035 §7 config.yaml 补 server 块

- **配置一致性类**：
  - REVIEW-018 queue.enabled 与 DEC-019 对齐
  - REVIEW-019 OSS ttl 字段补入 production template
  - ARCH-012 判决通过条件表述统一
  - REVIEW-015 quality_content_threshold 阈值统一

### 4.3 P2 — 延后优化（47 项建议问题）

- 文档版本与变更日志机制（DESIGN-040）
- 表述优化（ARCH-016/017/018/019/028/030 等）
- 格式规范（ARCH-037/038/039/040）
- 字段长度统一（ARCH-073）
- 历史 task.json 元数据补全（TASK-020）
- 审查流程接入 triage（REVIEW-035）

---

## 五、关键结论

### 5.1 文档体系整体评价

项目文档体系整体框架完整、Beta 路线图结构清晰，但存在 5 类系统性问题：

1. **「文档与代码脱节」是核心问题**：architecture.md §4 schema、§3.2 项目结构清单严重滞后于实际 migrations 和代码文件。建议建立「文档与代码同步检查」机制，每次合并 PR 时验证 schema 字段、API 路由、配置项在文档中的反映。

2. **「权威边界模糊」影响设计落地**：design-system.md 与 ui-spec.md 在圆角、字体、最大宽度等基础 token 上直接冲突；design-system.md 组件规范覆盖率不足 20%；ui-spec.md 14+ 组件无 Component 章节。AGENTS.md 第 14 条「Consult ui-spec for frontend」在当前状态下无法可靠执行。

3. **「跨文档不一致」分布在数值、术语、路径**：阈值（quality_content_threshold、CONFIG_OVERRIDE_PATH、健康检查端点）、术语（判官/审核员/评审员、CLAUDE.md/AGENTS.md）、路径（迁移编号、文件路径）三类问题各 10+ 处。

4. **「修复声明与实际不符」是回归风险源**：DEC-004、Task 102、history.md AH.2 均存在「声明已修复但实际未落地」问题。建议建立「修复验证闭环」机制，每项修复必须有代码引用和验证命令。

5. **「运维文档缺失」影响生产可用性**：runbook 缺少回滚、监控、告警、日志、Redis 备份、OSS 备份。AGENTS.md 阻塞处理规则要求「无法快速恢复」时停止，但 runbook 未提供回滚步骤等于预设了阻塞条件。

### 5.2 推荐的修复路径

1. **第 1 阶段（立即）**：修复 P0 安全与发布决策类问题（REVIEW-008/009/011/012/014/025 等）。
2. **第 2 阶段（本周）**：修复 Schema 一致性问题（ARCH-003/020/047、TASK-003/004/005/012/016）。
3. **第 3 阶段（两周内）**：补全设计文档缺失页面与组件规格（DESIGN-011/012/020、DESIGN-035 无障碍规范）。
4. **第 4 阶段（一个月内）**：补全运维文档（REVIEW-028/043/044/045/046/047）。
5. **第 5 阶段（持续）**：建立文档与代码同步检查机制，避免再次出现系统性脱节。

### 5.3 通过项

以下范围经审查未发现问题：

- ✅ **Beta 路线图任务 ID 命名空间**：F/V/A/G/D/R 六前缀在 roadmap 与各 subsystem plan 间一一对应。
- ✅ **F-01 至 A-05 子任务迁移编号**：与实际文件一致（除 053-056 超范围）。
- ✅ **task.json 全部 passes: true**：与 AGENTS.md「100% 完成」描述一致。
- ✅ **task.json ui_spec_ref 引用**：40 个引用均能在 ui-spec.md 找到精确匹配。
- ✅ **task.json 字段命名**：统一使用 snake_case。
- ✅ **task.json ID 连续性**：1-169 完全连续，无跳号。
- ✅ **PRD §6.3 信誉分初值（10 分）** 与 architecture.md §4.1 一致。
- ✅ **PRD §6.3 权限阈值（低于 3 分）** 与 architecture.md §7（`min_score_for_interaction: 3`）一致。
- ✅ **文件上传限制** 在 PRD/architecture.md/AGENTS.md 三处一致。
- ✅ **赛博判官阈值**（min_votes=20、pass_threshold=0.60、exam_pass_rate=0.80、error_rate_revoke=0.50）在三处一致。
- ✅ **AGENTS.md 工具链版本表**（Go 1.22+/Node 20+/PostgreSQL 16+/Redis 7+/Rust 1.75+）与 README.md 一致。
- ✅ **Feature Flag 名称**（payment_enabled、creator_support_enabled、desktop_deploy_enabled）在三处一致。
- ✅ **IP 分类枚举**（11 项）在 architecture.md §4.2 和 §10.6 一致。
- ✅ **原创区 category 枚举**（11 项）在 architecture.md §4.3 和 §10.6 一致。
- ✅ **content_type 枚举**（9 项）在 architecture.md §4.3 内部一致。
- ✅ **PRD 版本号 V0.3** 与 architecture.md 头部「对应 PRD：V0.3 正式版」一致。

---

## 附录 A：审查文档清单

### A.1 核心架构与规范（5 份）
- architecture.md
- AGENTS.md（根目录）
- CLAUDE.md（根目录）
- OmniCraft（万象工坊）V0.3 正式版产品需求文档.md
- README.md

### A.2 设计文档（4 份）
- design/design-system.md
- design/ui-spec.md
- design/ui-design-prompt.md
- design/doc-review-prompt.md

### A.3 Beta 路线图与计划（18 份）
- docs/superpowers/plans/2026-05-07-omnicraft-code-review.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-admin-operations.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-agent-entrypoints.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-desktop-deploy-security.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-foundation.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-implementation-notes.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-plan-review.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-release-validation.md
- docs/superpowers/plans/2026-05-30-omnicraft-beta-verification-feedback.md
- docs/superpowers/plans/2026-05-30-omnicraft-dual-track-beta-roadmap.md
- docs/superpowers/plans/2026-06-03-omnicraft-web-beta-review-repair-plan.md
- docs/superpowers/plans/2026-06-05-omnicraft-ui-detail-repair-plan.md
- docs/superpowers/plans/2026-06-08-omnicraft-abuse-control-no-load-testing.md
- docs/superpowers/plans/2026-06-08-omnicraft-dependency-vulnerability-upgrades.md
- docs/superpowers/plans/2026-06-08-omnicraft-oss-upload-download-hardening.md
- docs/superpowers/plans/2026-06-08-omnicraft-release-gates-config-hardening.md
- docs/superpowers/progress/2026-06-09-security-hardening-execution.md
- docs/superpowers/specs/2026-05-30-omnicraft-dual-track-beta-design.md
- docs/superpowers/specs/2026-06-05-omnicraft-web-beta-optimization-roadmap-design.md

### A.4 历史任务账本与进度（3 份）
- task.json
- progress.txt
- docs/history/docs edited history.md

### A.5 审查报告与运维（30+ 份）
- docs/review/ 下 17 份审查报告
- docs/iteration-review/ 下 9 份迭代审查报告
- docs/2026-06-24-design-review-decisions.md
- docs/2026-06-24-omnicraft-design-review-merged.md
- docs/2026-06-24-omnicraft-documentation-review.md
- docs/design-review-2026-06-24.md
- docs/iteration-review-prompt.md
- docs/deploy/ 下 4 份部署文档
- docs/agents/ 下 3 份 Agent 工作流文档
- docs/oss-lifecycle.md、docs/async-queue-analysis.md、docs/PLAN.md、docs/futureWork/2026-05-31-omnicraft-beta-future-work.md
- docs/archive/ 归档目录

---

## 附录 B：审查方法说明

### B.1 审查流程

1. 主代理读取 `design/doc-review-prompt.md` 获取既定审查框架。
2. 主代理并行启动 4 个 search 子代理，分别负责：
   - 子代理 A：核心架构与规范文档
   - 子代理 B：设计文档
   - 子代理 C：Beta 路线图与计划
   - 子代理 D：任务账本与进度
   - 子代理 E：审查报告与运维
3. 每个子代理独立读取分配文档，识别问题并按统一格式返回结构化清单。
4. 主代理合并去重 22 条跨模块重复识别，整理为最终报告。

### B.2 问题分级标准

| 等级 | 含义 |
|---|---|
| 🔴 阻塞 | 矛盾会导致开发错误、运行时 Bug 或安全漏洞；必须立即修复 |
| 🟡 警告 | 不一致但不影响功能；建议修复 |
| 🟢 建议 | 优化建议；可延后处理 |

### B.3 问题类型分类

| 类型 | 含义 |
|---|---|
| 内容前后矛盾 | 同一字段/阈值/枚举在多文档取值不同；schema 与迁移不一致 |
| 表述模糊不清 | 「达标」「适当」「大部分」等无具体数值；状态描述模糊 |
| 设计逻辑不合理 | 路由结构混乱、API 缺端点、schema 与软删除策略不符 |
| 技术方案过时 | 文档清单与实际代码脱节、迁移编号引用过时、引用已归档/不存在文件 |
| 格式不规范 | 字段顺序不一致、Markdown 转义、重复 heading、checkbox 未同步 |
| 术语使用不一致 | 同一概念用多个名称 |
| 关键信息缺失 | schema 字段缺注释、API 缺定义、配置项未登记、无回滚/监控/无障碍规范 |
| 版本信息错误 | 文件名日期与正文日期不符、依赖版本粒度差异 |
| 失效引用 | 引用不存在的文件 |
| 重复 | 报告间重复发现问题、文档间内容重复 |
| 依赖错误 | task.json depends_on 缺失、反向依赖未修复 |
| 不合理 | ID 与执行顺序倒挂、debug 模式跳过 release 校验 |

### B.4 审查限制

- 本报告基于磁盘当前状态审查，未参考 git 版本历史。
- 子代理 E（审查/运维）部分文档因数量较多未逐一深读，可能存在遗漏。
- 部分问题（如 ARCH-060 AccessKey 命名）未验证 `.env.example` 实际内容，建议后续验证。
- 未对 frontend/、backend/ 实际代码逐文件验证文档声明，仅做了关键点抽样。

---

**报告结束** | 共识别 187 条问题 | 阻塞 41 条 | 警告 99 条 | 建议 47 条
