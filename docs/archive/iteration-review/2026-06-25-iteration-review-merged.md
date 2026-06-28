# 迭代质量审查报告（合并版）

**审查日期**: 2026-06-25
**审查范围**: origin/main（f538ef1）→ HEAD（642d804）
**审查依据**: `docs/iteration-review-prompt.md` 五维质量审查提示词
**本次迭代 commit**:
- `8e83fe8` Docs: add design review report system (issue list + decisions)
- `3ecef1a` Docs: resolve design system conflicts and archive obsolete docs
- `642d804` Docs: add iteration quality review prompt for coding agent

**合并来源**: 本报告由两份独立审查报告合并而成，经交叉验证消除矛盾，取双方之长：
- 报告 A: `2026-06-25-iteration-review.md`（GLM-5.2 Agent）
- 报告 B: `2026-06-25-iteration-review-report(1).md`（Coding Agent (1) v1.1）

---

## 总体结论

| 维度 | 结论 | 阻塞项 | 警告项 | 关键发现 |
|------|------|--------|--------|---------|
| 维度 1（回归与副作用） | ⚠️ 条件通过 | 0 | 2 | 侧边栏展开宽度不一致由本次迭代引入；#0969da 色值残留（既存） |
| 维度 2（归档与清理） | ❌ 不通过 | 2 | 2 | UI Design.md 引用残留于 4 个活跃文档，其中 2 个为阻塞级 |
| 维度 3（问题真实性） | ✅ 通过 | 0 | 0 | 8 个抽样问题覆盖 6/10 类别，全部确认真实 |
| 维度 4（策略质量） | ⚠️ 条件通过 | 0 | 2 | 33 个决策中仅 9 个含独立风险提示；DEC-012 标题未反映实际操作 |
| 维度 5（文档同步） | ❌ 不通过 | 0 | 5 | 跨文档侧边栏宽度、活跃引文残留、design-system 内部 220px vs 228px 矛盾 |

**总体评估**: 需修复后复审。维度 2 有 2 项阻塞级问题（活跃设计文档引用已归档文档作为权威），维度 5 有 5 项不一致，维度 1 和维度 4 各有 2 项条件性警告。修复成本均为低。本次迭代主体质量高，文档归档、决策记录、问题清单均达到可执行标准。

本次迭代为**纯文档变更**，无代码改动。需注意：部分文档更新（如侧边栏展开宽度从 224px 改为 228px）**引入了新的代码-文档不一致**（迭代前代码与文档均为 224px，一致），并非仅暴露既存问题。

---

## 维度 1: 回归与副作用验证

### 1.1 architecture.md §7 配置字段变更影响

**核查结果**: ✅ 通过

- 全局搜索 `video_max_size_mb|video_max_duration_sec|mod_archive_max_size_mb|min_score_for_publish` 在 `backend/` 中 **无任何匹配** — 旧字段名已完全清除
- `backend/config/config.go:98-105` `LimitsConfig` 结构体使用新字段名 `video_max_mb`/`video_max_sec`/`image_max_mb`/`text_max_mb`/`mod_max_mb`/`sheet_music_max_mb` — 与 architecture.md §7 一致
- `backend/config.yaml:88-94`（limits 段）实际配置使用新字段名 — 一致
- `backend/config.yaml:96-102`（reputation 段）`quality_content_threshold: 10`、`quality_comment_threshold: 5` — 与更新后的 architecture.md 一致
- `backend/config/config.go:110` `MinScoreForInteraction` 字段 — 与新命名一致

**阈值变更影响**: 阈值从 50/20 → 10/5 的变更已在 config.yaml 中生效（实际配置本来就是 10/5，本次只是让 architecture.md 对齐），不会影响运行中的信誉分计算逻辑。

### 1.2 architecture.md §10.4 设计系统变更影响

**核查结果**: 🟡 警告（非回归，但存在文档与代码不一致）

- architecture.md §10.4 已删除 GitHub blue `#0969da` 色值表 ✅
- architecture.md 中搜索 `#0969da`、`GitHub 风格` — **无残留** ✅
- `frontend/app/globals.css:83` 仍存在 `--chart-1: #0969da;` — 这是图表色板（shadcn/ui chart 变量），非品牌主色，属于 🟢 建议优化项
- `design/design-system.md:48-57` 仍保留 `--canvas-default`/`--canvas-subtle`/`--border-default`/`--fg-muted`/`--accent-emphasis`/`--accent-subtle` 作为「自定义 Token」 — 这些 Token 名与代码中 `bg-canvas-subtle`/`text-fg-muted`/`text-accent-emphasis` 一致，**不是旧 Token 残留**
- 前端代码使用 `bg-canvas-subtle`/`text-fg-muted`/`text-accent-emphasis` 与 design-system.md 定义一致 ✅

**关键区分**: architecture.md §10.4 删除的是**写死 `#0969da` 等具体色值的旧色值表代码块**，而非 Token 命名体系。Token 命名体系（`canvas`/`border`/`fg`/`accent`）被 `design-system.md` 保留并映射到 Indigo 色值（如 `--accent-emphasis` 指向 `#4f46e5` 而非旧 GitHub blue）。因此代码中引用这些 Token 名是正确行为，无需清理。

### 1.3 architecture.md §13.2 侧边栏宽度变更影响

**核查结果**: 🟡 警告（本次迭代引入了新的代码-文档不一致）

- `architecture.md §13.2` 已从 224px（`w-56`）/ 64px（`w-16`）**改为** 228px / 48px ✅
- `CLAUDE.md` / `AGENTS.md` 同步更新为 228px / 48px ✅
- `design/design-system.md:63` `--sidebar-w: 228px` ✅
- `frontend/components/studio/StudioSidebar.tsx:82` 仍使用 `collapsed ? "w-[52px]" : "w-56"` — **代码是 224px/52px，与文档 228px/48px 不一致**

**因果关系分析**（git diff 验证）:
- **展开宽度**: 迭代前 architecture.md=224px/`w-56`，代码=`w-56`(224px) → **一致**；迭代后 architecture.md=228px，代码仍=224px → **不一致**（**本次迭代引入的新差距**）
- **收起宽度**: 迭代前 architecture.md=64px/`w-16`，代码=`w-[52px]`(52px) → 不一致（既存）；迭代后 architecture.md=48px，代码仍=52px → 不一致（既存，但目标值变了）

**影响评估**: 本次迭代将 architecture.md §13.2 的展开宽度从 224px 改为 228px，**制造了展开宽度上新的代码-文档不一致**（迭代前代码与文档均为 224px，一致）。收起宽度的不一致为既存问题。建议在后续 Beta 任务中同步修复代码。

### 1.4 CLAUDE.md / AGENTS.md 规则变更影响

**核查结果**: ✅ 通过

- Key Rule 18 从 "Soft delete only" 改为 "Soft delete preferred" — `CLAUDE.md:600` 和 `AGENTS.md:600` 一致更新，引用 DEC-031
- 与现有代码冲突评估：现有代码中 users/content_items 已有软删除，comments 等表无软删除 — 新规则「以软删除为主，部分物理删除」与现状兼容，DEC-031 已列出待补全的表清单
- 模式 A 规则 10（桌面端部署）: `CLAUDE.md` 模式 A 规则 10 明确「D-02 至 D-05 完成后开启 `features.desktop_deploy_enabled: true`；R-02 验证通过前不得发布到生产环境」 — 不会误导 Agent 提前开启

### 1.5 文档内部逻辑一致性

**核查结果**: ✅ 通过

- **DEC-007 策略 4 vs DEC-033**: `docs/2026-06-24-design-review-decisions.md:530-534` DEC-007 策略 4 已明确标注「**更新(DEC-033)**: P3 讨论后决定通知系统也改为 SSE 推送」并指向 DEC-033 — 矛盾已正确标注
- **47 个问题的决策后状态**: 抽样核查问题优先级矩阵（`merged:979-1027`），所有问题均有 `→ 决策:DEC-XXX` 或 `✅ 已修复关闭` 标注 — 一致

### 维度 1 小结

| 检查项 | 结果 |
|--------|------|
| 配置字段重命名 | ✅ 通过 |
| 设计系统 Token（含 Token 名 vs 色值表区分） | ⚠️ #0969da 在 globals.css 图表色板残留（🟢） |
| 侧边栏宽度代码一致性 | ⚠️ 本次文档修改制造了新差距：代码 224px vs 文档 228px（🟡） |
| Key Rules 变更 | ✅ 通过 |
| DEC-007/DEC-033 一致性 | ✅ 通过 |

---

## 维度 2: 归档完整性与依赖清理

### 2.1 归档文件移动验证

**核查结果**: ✅ 通过

- `design/UI Design.md` 已移至 `docs/archive/UI Design.md`，原位置不存在 ✅（Glob 确认无匹配）
- `design/homepage-v0.html` 已移至 `docs/archive/homepage-v0.html`，原位置不存在 ✅
- `docs/archive/` 目录存在且包含上述两个文件 ✅

### 2.2 残留引用扫描

**核查结果**: ❌ 不通过 — 4 项活跃残留引用（含 2 项阻塞级）

| # | 文件 | 行号 | 引用内容 | 严重程度 | 修复建议 |
|---|------|------|---------|---------|---------|
| 1 | `design/ui-spec.md` | 5, 8 | "Agent 应参考对应 Task 的 ui_spec_ref 和 UI Design.md 页面小节获取真实组件列表" / "优先以 ## Component: 具体组件的规范和 UI Design.md 对应页面为准" | 🔴 阻塞 | ui-spec.md 是当前唯一 UI 权威，不应引导 Agent 去参考已归档文档。移除这两处引用，改为指回 design/design-system.md 和对应 Task 的 steps |
| 2 | `design/ui-design-prompt.md` | 9, 79, 144-154 | 多达 13 处引用 UI Design.md，将其描述为"规格输入清单"、组件来源 | 🔴 阻塞 | 全文替换 UI Design.md 为 `docs/archive/UI Design.md（已归档，仅供历史参考）` 或在文件顶部标注归档状态 |
| 3 | `docs/superpowers/specs/2026-05-30-omnicraft-dual-track-beta-design.md` | 72 | 文件清单中列出 `UI Design.md` 未标注已归档 | 🟡 警告 | 更新为 `design/ui-spec.md` 或标注「已归档至 docs/archive/」 |
| 4 | `task.json` | 1582 | Task 76 steps 描述中引用 `UI Design.md P04a` | 🟡 警告 | 标注为 `[已归档] UI Design.md P04a`。虽属历史任务描述，但模式 B 仍可能触发该任务，Agent 按 steps 执行时会尝试查找已不存在的 UI Design.md |

**已排除的引用**（视为历史记录，无需修复）:
- `docs/history/docs edited history.md` — 修改历史记录，引用 UI Design.md 是历史事实
- `docs/2026-06-24-*.md` — 审查报告自身，引用是审查对象的描述
- `docs/iteration-review-prompt.md` — 审查提示词，引用是审查范围描述
- `docs/archive/UI Design.md` — 归档文件自身引用

**未发现 `design/homepage-v0.html` 的活跃残留引用**（除审查报告自身和 `architecture.md:1610` 标注"已归档"外）✅

### 2.3 task.json 中的 ui_spec_ref 字段检查

**核查结果**: ✅ 通过（ui_spec_ref 字段无问题，但 steps 描述有残留引用）

- task.json 中 20+ 个 `ui_spec_ref` 字段引用的都是章节名（如 `## Global Interaction Patterns`、`## Component: Header`），**不是文件路径** ✅
- `task.json:1582` 在 Task 76 steps 描述中提及 `UI Design.md P04a` — 已列入 2.2 残留引用清单第 4 项（🟡 警告级），建议标注为 `[已归档]`

### 2.4 design/ 目录完整性

**核查结果**: ✅ 通过

`design/` 目录下仅保留 4 个文件:
- `design-system.md`
- `ui-spec.md`
- `ui-design-prompt.md`
- `doc-review-prompt.md`

与 CLAUDE.md/AGENTS.md 项目结构中列出的一致，无遗留文件。

### 2.5 docs/archive/ 目录说明

**核查结果**: ✅ 通过

- `CLAUDE.md:258` 和 `AGENTS.md:258` 项目结构中标注 `archive/  # 已归档文档（不再作为开发参考）` ✅
- `design/doc-review-prompt.md:44-46` 已添加归档文档说明 ✅

### 维度 2 小结

| 检查项 | 结果 |
|--------|------|
| 归档文件物理移动 | ✅ 通过 |
| 残留引用扫描 | ❌ 4 处活跃残留（含 2 项 🔴 阻塞） |
| task.json ui_spec_ref | ⚠️ 1 处 steps 描述引用 |
| design/ 目录完整性 | ✅ 通过 |
| archive 目录说明 | ✅ 通过 |

---

## 维度 3: 新发现问题真实性与上下文完整性

### 3.1 问题真实性验证（抽样核查）

**核查结果**: ✅ 通过 — 8 个抽样问题覆盖 6/10 类别，全部确认真实

| 抽样问题 | 类别 | 描述准确性 | 代码证据 | 结论 |
|---------|------|-----------|---------|------|
| **A1**（设计系统三重冲突） | A | 三套互不一致的设计系统 | 审查时确实存在。architecture.md §10.4 已重写，homepage-v0.html 已归档 | 真实 |
| **A2**（侧边栏宽度矛盾） | A | architecture.md 224px/64px vs design-system.md 228px/48px | 审查时确实存在。architecture.md 已更新为 228px/48px | 真实 |
| **C1**（信誉分配置缺失） | C | config.yaml reputation 部分缺少加减分值字段 | `config.yaml:96-102` 仅有 6 个字段，12 个 bonus/penalty 分值全部缺失。`reputation_service.go:85,89,103,107,111,122,126,130,134,138,142,153` 全部硬编码 `3`/`2`/`1`/`-3`/`-2`/`-1` | 真实 |
| **C5**（Agent LLM 配置混乱） | C | config.yaml agent.llm_api_key 为空且有"需修复"注释 | `config.yaml:143` `llm_api_key: ""` 注释「留空，改用 env var 注入（需修复 config.go）」 | 真实 |
| **D1**（Studio 存根页面） | D | studio/contributors 为空存根 | `studio/contributors/page.tsx` 仅 18 行，纯 EmptyState，无 API 调用 | 真实 |
| **F1**（瀑布流无分页） | F | 原创区只有 24 条无分页 | `original/page.tsx:73` `page_size: "24"` 无 page 参数，无 loadMore，无无限滚动 | 真实 |
| **B3**（中文搜索配置不匹配） | B | 查询侧 to_tsquery('simple') 与索引侧 jiebacfg 不匹配 | `search_repo.go:149,169,170,171,175` 全部使用 `to_tsquery('simple', ?)` / `phraseto_tsquery('simple', ?)` / `ts_headline('simple', ...)` — 与索引侧 `jiebacfg` 不匹配 | 真实 |
| **H1**（Token XSS 泄露风险） | H | Token 存储方案不安全 | 已标注事实核查修正（Refresh Token 已在 HttpOnly Cookie），与 DEC-002 修正结论一致 | 真实 |

> **抽样说明**: 覆盖 6/10 类别（A/B/C/D/F/H），8/47 问题（~17%）。未覆盖类别 E/G/I/J。所有抽样问题经代码/配置交叉核查后均确认真实。

### 3.2 已关闭问题核查

**核查结果**: ✅ 通过 — 6 个已关闭问题标注合理

| 问题 | 声称状态 | 验证 |
|------|---------|------|
| B1（推荐缓存 TTL） | ✅ 已修复关闭 | `recommendation_service.go:128-137` 显式设置 TTL |
| F2（侧边栏折叠） | ✅ 已修复关闭 | `StudioSidebar.tsx:135-141` 确认有折叠+图标+tooltip |
| F3（ContentCard 单一实现） | ✅ 已修复关闭 | 问题清单确认 |
| F4（分类 Tab sticky） | ✅ 已修复关闭 | 问题清单确认 |
| G3（URL Scheme） | ✅ 已修复关闭 | 问题清单确认 |
| H2（密码重置状态） | ✅ 已修复关闭 | 问题清单确认 |

### 3.3 事实核查修正标注验证

**核查结果**: ✅ 通过

- **B3 修正**: `merged:234` 明确标注「⚠️ 事实核查修正」— 索引侧已用 `jiebacfg`，实际问题在查询侧 — 与代码核查一致
- **F1 修正**: `merged:605` 明确标注「⚠️ 事实核查修正」— 实际不是「无限滚动无状态恢复」而是「根本没有无限滚动」— 与代码核查一致
- **H1 修正**: `merged:825` 明确标注「⚠️ 事实核查修正」— Refresh Token 已在 HttpOnly Cookie 中 — 与 DEC-002 核查结论一致

### 3.4 上下文完整性检查

**核查结果**: ✅ 通过

抽样核查 8 个问题（A1/A2/C1/C5/D1/F1/B3/H1），每个均包含:
- ✅ 发现位置（文件路径 + 章节/行号）
- ✅ 问题描述（具体、无歧义）
- ✅ 建议方案（可执行、技术可行）
- ✅ 来源标注（`[A]` / `[B]` / `[A+B]`）
- ✅ 决策关联（`→ 决策:DEC-XXX` 或 `✅ 已修复关闭`）

### 3.5 决策关联完整性

**核查结果**: ✅ 通过

- 47 个问题全部有决策关联标注或已关闭标注（见 `merged:979-1027` 优先级矩阵）
- 33 个决策在 `decisions:14-50` 决策索引中均有对应关联问题
- 决策记录末尾覆盖清单（`decisions:1796-1829`）确认 33 个决策覆盖全部 47 个问题 ✅
- 优先级矩阵与决策索引一致 ✅

---

## 维度 4: 决策策略质量评估

### 4.1 策略全面性

**核查结果**: ⚠️ 条件通过 — 33 个决策中仅 9 个（27%）有独立风险提示节

全量统计 `### 风险提示` 段落：**9 个**决策含独立风险提示，所属 DEC 为：DEC-014、DEC-015、DEC-018、DEC-019、DEC-020、DEC-021、DEC-031、DEC-032、DEC-033。其余 **24 个**（73%）无独立风险提示。

抽检 DEC-001、DEC-004、DEC-005、DEC-008、DEC-011、DEC-014、DEC-031 的结构完整性：

| 决策 | 问题核实 | 明确决策 | 实施策略 | 实施要点 | 验证标准 | 风险提示 |
|------|---------|---------|---------|---------|---------|---------|
| DEC-001 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| DEC-004 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| DEC-005 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| DEC-008 | ✅ | ✅ | ✅（5 条） | ✅ | ✅ | ⚠️ 缺失 |
| DEC-011 | ✅ | ✅ | ✅（2 条） | ✅ | ✅ | ⚠️ 缺失 |
| DEC-014 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅（标注「高风险：涉及 100+ 文件移动」） |
| DEC-031 | ✅ | ✅ | ✅（含迁移 SQL） | ✅ | ✅ | ✅ |

- **DEC-008**（草稿系统完整实现）：无独立 `### 风险提示` 节。该决策涉及 `content_items.status='draft'` 复用，存在草稿数据与已发布数据混在同一查询中的风险（需确保所有查询默认过滤 `status != 'draft'`）。虽然"实施要点"中提到了部分注意事项，但未独立声明风险。
- **DEC-011**（设计系统统一）：同样无独立 `### 风险提示` 节。建议补充"可能存在第三方工具/脚本引用旧色值"的迁移风险。

> **说明**: 风险提示缺失不代表决策质量差——部分决策风险本身较低。但统一补充后有助于后续实施者评估影响面。

### 4.2 技术合理性

**核查结果**: ✅ 通过

| 决策 | 合理性评估 | 结论 |
|------|-----------|------|
| **DEC-001**（动态探测 pg_jieba + fallback） | 合理 — 兼顾生产环境（jiebacfg）和开发环境（simple fallback），不引入 gojieba 二进制依赖 | ✅ |
| **DEC-004**（阈值统一为 10） | 合理 — config.yaml 实际值为 10/5，architecture.md 的 50/20 是文档过时；个位数加减分符合实际业务规模 | ✅ |
| **DEC-008**（复用 content_items 表 status='draft'） | 合理 — 避免新建表，利用现有 status 枚举扩展，草稿不进入 search_vector | ✅ |
| **DEC-014**（DDD 完整重构） | 合理 — 与用户导向「尽量让功能全面」一致；风险已标注（高成本），建议独立分支执行 | ✅ |
| **DEC-028**（HNSW 替代 IVFFlat） | 合理 — HNSW 在百万级以内数据查询性能更优，构建慢但查询快 2-5 倍 | ✅ |
| **DEC-031**（软删除为主，browse_history/notifications 物理删除） | 合理 — browse_history 数据量大且已有独立清理策略，notifications 无分析价值 | ✅ |

### 4.3 内部一致性

**核查结果**: ⚠️ 条件通过 — 1 项注意

- **DEC-007 vs DEC-033**: `decisions:530-534` DEC-007 策略 4 已明确标注「**更新(DEC-033)**: P3 讨论后决定通知系统也改为 SSE 推送」 — 矛盾已正确标注 ✅
- **DEC-008 草稿系统 vs 定时发布 E2**: DEC-008 策略 5 明确「定时发布（顺带实现 E2）」— 逻辑自洽 ✅
- **DEC-011 vs DEC-013**: DEC-011 统一 design-system.md，DEC-013 归档 UI Design.md — 互补不冲突 ✅
- **DEC-012 vs DEC-013**: `decisions:834` DEC-012 明确标注「此决策与 DEC-013 冲突。DEC-013 决定归档 UI Design.md，因此 DEC-012 的『更新』操作改为『归档前标注过时内容』」 — 冲突已在文档内解决 ✅
  - ⚠️ **注意**: DEC-012 标题为「更新 UI Design.md 移除二级筛选描述」，但实际执行的操作是「归档前标注过时内容」（因 DEC-013 将整个文档归档）。标题未反映实际操作，可能使后续 Agent 阅读时困惑。严重程度：🟢 建议
- **DEC-032 vs CLAUDE.md 模式 A 规则 10**: DEC-032 决定「现在实施 D-02 到 D-05」，CLAUDE.md 模式 A 规则 10 要求「D-02 至 D-05 完成后开启 desktop_deploy_enabled」 — 一致，DEC-032 是实施计划，规则 10 是启用条件 ✅
- **迁移编号**: DEC-001: 056, DEC-008: 057, DEC-028: 058, DEC-031: 059 — 无冲突 ✅

### 4.4 表述清晰度

**核查结果**: ✅ 通过

- 抽样核查 DEC-008「复用 content_items 表」表述清晰，无「可选，或复用」类模糊用语
- DEC-021 已修复「100 个任务 vs 86 个任务」数据不一致（`decisions:1314` 明确「task.json 100 个任务全部 passes: true」）
- G6 的 `/api/v1/messages/stream` API 路径描述清晰无歧义（`decisions:508`）
- 决策标题准确反映决策内容 ✅

### 4.5 暂缓/搁置决策评估

**核查结果**: ✅ 通过

| 决策 | 状态 | 解除条件 | 明确性 |
|------|------|---------|--------|
| **DEC-003**（法律文本搁置） | ⏸️ 搁置 | `decisions:217`「用户完成法律文本起草后，告知版本号和文本内容，即可启动代码实施」 | ✅ 明确 |
| **DEC-016**（Tauri 离线暂缓） | ⏸️ 暂缓 | `decisions:1062`「标记为 P3 长期优化」 | ✅ 标注降级 |
| **DEC-017**（PWA 暂缓） | ⏸️ 暂缓 | `decisions:1091`「标记为 P3 长期优化」 | ✅ 标注降级 |
| **DEC-026**（Tauri 账号同步暂缓） | ⏸️ 暂缓 | `decisions:1441`「暂缓，标记为长期优化」 | ✅ 标注暂缓 |

### 维度 4 小结

| 检查项 | 结果 |
|--------|------|
| 策略全面性 | ⚠️ 仅 9/33 决策有风险提示（含 DEC-008/DEC-011 缺失） |
| 技术合理性 | ✅ 通过 |
| 内部一致性 | ⚠️ DEC-012 标题未反映实际操作（文档内已解决冲突） |
| 表述清晰度 | ✅ 通过 |
| 暂缓/搁置决策 | ✅ 通过 |

---

## 维度 5: 文档同步与冲突验证

### 5.1 跨文档字段名一致性

**核查结果**: ✅ 通过

| 字段 | architecture.md §7 | config.yaml | CLAUDE.md/AGENTS.md | config.go |
|------|-------------------|-------------|---------------------|-----------|
| `video_max_mb` | ✅ | ✅ (89) | ✅ | ✅ (99) |
| `video_max_sec` | ✅ | ✅ (90) | ✅ | ✅ (100) |
| `mod_max_mb` | ✅ | ✅ (93) | ✅ | ✅ (103) |
| `min_score_for_interaction` | ✅ | ✅ (97) | ✅ | ✅ (110) |
| `quality_content_threshold: 10` | ✅ | ✅ (98) | ✅ | ✅ (108) |
| `quality_comment_threshold: 5` | ✅ | ✅ (99) | ✅ | ✅ (109) |

旧字段名（`video_max_size_mb`、`mod_archive_max_size_mb`、`min_score_for_publish`、`quality_content_threshold: 50`、`quality_comment_threshold: 20`）在三份文档中均已清除 ✅

### 5.2 设计系统权威性验证

**核查结果**: ❌ 不通过 — design/ 目录下文档仍引用已归档的 UI Design.md

- `architecture.md §10.4` 已改为指向 `design/design-system.md` 和 `design/ui-spec.md` ✅
- `CLAUDE.md:596-597` Key Rule 14/15 指向 `design/ui-spec.md` ✅
- `design/doc-review-prompt.md:39` 文档清单序号 4 指向 `design/design-system.md + design/ui-spec.md` ✅
- ❌ `design/ui-spec.md:5,8` 自身仍引用 `UI Design.md` 作为 fallback 权威（见维度 2.2 第 1 项，🔴 阻塞）
- ❌ `design/ui-design-prompt.md:9` 将 `UI Design.md` 描述为"本文档是规格的输入清单"（见维度 2.2 第 2 项，🔴 阻塞）
- ❌ `docs/superpowers/specs/2026-05-30-omnicraft-dual-track-beta-design.md:72` 参考文档清单中列出 `UI Design.md`（未标注已归档）（见维度 2.2 第 3 项，🟡 警告）

**说明**: 5.2 不仅需检查 architecture.md/CLAUDE.md/doc-review-prompt.md 是否正确指向 design-system.md，还需检查 design/ 目录下文档自身是否包含对已归档文档的引用。后者检查发现 3 处残留引用，与维度 2.2 交叉确认。

### 5.3 侧边栏宽度一致性

**核查结果**: ❌ 不通过 — 多处不一致

| 位置 | 展开宽度 | 收起宽度 | 状态 |
|------|---------|---------|------|
| `architecture.md §13.2` | 228px | 48px | ✅ |
| `CLAUDE.md` / `AGENTS.md` | 228px | 48px | ✅ |
| `design/design-system.md:63` | `--sidebar-w: 228px` | — | ✅ |
| `design/design-system.md:113` | 228px | 48px | ✅ |
| `design/design-system.md:157`（AppLayout 图） | 228px | — | ✅ |
| `design/design-system.md:168`（AdminLayout 图） | **220px** | — | ❌ |
| `design/design-system.md:178`（StudioLayout 图） | **220px** | — | ❌ |
| `StudioSidebar.tsx:82` | **224px** (`w-56`) | **52px** (`w-[52px]`) | ❌ |

- **文档内部不一致**: `design-system.md` 中 AppLayout 图标注 228px（line 157），但 AdminLayout 图（line 168）和 StudioLayout 图（line 178）标注 220px。需统一为 228px，或在文档中说明 Admin/Studio 侧边栏有意使用不同宽度的原因。
- **代码-文档不一致**: `StudioSidebar.tsx` 实际使用 `w-56`（224px）和 `w-[52px]`（52px），与文档的 228px/48px 不匹配。其中展开宽度不一致由本次迭代引入（迭代前文档也是 224px/w-56），收起宽度不一致为既存问题。

**修复建议**:
1. 将 `design/design-system.md:168` 和 `:178` 的 `220px` 改为 `228px`
2. 将 `StudioSidebar.tsx:82` 的 `w-56` 改为 `w-[228px]`，`w-[52px]` 改为 `w-12`（48px）— 建议在后续 Beta 任务中修复，同步检查 StudioLayout.tsx

### 5.4 信誉分阈值一致性

**核查结果**: ✅ 通过

- `architecture.md §7` `quality_content_threshold: 10`、`quality_comment_threshold: 5` ✅
- `config.yaml:98-99` `quality_content_threshold: 10`、`quality_comment_threshold: 5` ✅
- CLAUDE.md/AGENTS.md 信誉分体系部分未引用具体阈值数值，引用 `config.yaml > reputation.quality_content_threshold` ✅

### 5.5 下载权限描述一致性

**核查结果**: ✅ 通过

- `CLAUDE.md` 内容下载部分已更新为「信誉分 < `config.yaml > reputation.min_score_for_interaction`（默认 3）用户禁止下载...所有信誉分门槛统一使用 `min_score_for_interaction`」 ✅
- `AGENTS.md` 同样更新 ✅

### 5.6 软删除策略描述一致性

**核查结果**: ✅ 通过

- `CLAUDE.md:600` Key Rule 18「Soft delete preferred — 删除操作以软删除为主...仅对确无分析价值的数据（如浏览历史、已读通知）使用物理删除（Task 103，DEC-031）」 ✅
- `AGENTS.md:600` 同样更新 ✅
- DEC-031 物理删除表清单（`browse_history`、`notifications`）与 Key Rule 18 描述一致 ✅
- 无「禁止物理删除」与「部分物理删除」的矛盾 ✅

### 5.7 桌面端部署描述一致性

**核查结果**: ✅ 通过

- `CLAUDE.md` 模式 A 规则 10「D-02 至 D-05 完成后开启 `features.desktop_deploy_enabled: true`」 ✅
- `architecture.md §7` 新增 `desktop_deploy_enabled: false  # 桌面端一键部署（D-02~D-05 完成后开启）` ✅
- `config.go:95` `DesktopDeployEnabled` 字段存在 ✅
- DEC-032 明确 "D-02 到 D-05 完成后设置 desktop_deploy_enabled: true" ✅
- 三处描述一致 ✅

### 5.8 task.json 状态描述一致性

**核查结果**: ✅ 通过

- `CLAUDE.md` 模式 B 说明「task.json 中 100+ 个历史任务已全部完成（passes: true），仅作历史记录保留」 ✅
- `design/doc-review-prompt.md:40` 文档清单序号 5「历史任务列表：100+ 个开发任务，已 100% 完成，仅作历史记录」 ✅

### 5.9 项目结构一致性

**核查结果**: ✅ 通过

- CLAUDE.md 和 AGENTS.md 项目结构部分一致（仅「本文件」注释行不同） ✅
- 项目结构中 `design/` 目录列出 4 个文件 ✅
- 项目结构中包含 `docs/archive/` 目录 ✅
- 项目结构中包含 `docs/2026-06-24-*.md` 审查报告 ✅

### 5.10 过时信息扫描

**核查结果**: ⚠️ 1 项残留（非阻塞）

在 `architecture.md` 中搜索以下过时关键词:

| 关键词 | architecture.md | 说明 |
|--------|----------------|------|
| `GitHub 风格` | 无 | §10.4 标题已移除 ✅ |
| `#0969da` | 无 | 已移除 ✅ |
| `w-56` / `w-16` | 无 | ASCII 图中已移除 ✅ |
| `video_max_size_mb` | 无 | ✅ |
| `mod_archive_max_size_mb` | 无 | ✅ |
| `min_score_for_publish` | 无 | ✅ |
| `quality_content_threshold: 50` | 无 | ✅ |
| `quality_comment_threshold: 20` | 无 | ✅ |

但 `frontend/app/globals.css:83` 仍保留 `--chart-1: #0969da`（见维度 1.2，🟢 建议）。

### 维度 5 小结

| 检查项 | 结果 |
|--------|------|
| 跨文档字段名一致性 | ✅ 通过 |
| 设计系统权威性 | ❌ ui-spec/ui-design-prompt/Beta spec 仍引用 UI Design.md（含 2 项 🔴 阻塞） |
| 侧边栏宽度一致性 | ❌ design-system 内部不一致 + 代码-文档不一致 |
| 信誉分阈值一致性 | ✅ 通过 |
| 下载权限描述一致性 | ✅ 通过 |
| 软删除策略描述一致性 | ✅ 通过 |
| 桌面端部署描述一致性 | ✅ 通过 |
| task.json 状态一致性 | ✅ 通过 |
| 项目结构一致性 | ✅ 通过 |
| 过时信息扫描 | ⚠️ #0969da 在 globals.css 残留 |

---

## 修复建议汇总

| 优先级 | 维度 | 问题 | 修复建议 | 涉及文件 |
|--------|------|------|---------|---------|
| 🔴 阻塞 | 2、5 | `design/ui-spec.md` 引用已归档的 UI Design.md 作为权威来源 | 移除 line 5, 8 的 UI Design.md 引用，改为指回 design/design-system.md 和对应 Task 的 steps | `design/ui-spec.md` |
| 🔴 阻塞 | 2、5 | `design/ui-design-prompt.md` 全文 13+ 处引用 UI Design.md 作为有效输入 | 全文替换为 `docs/archive/UI Design.md（已归档，仅供历史参考）` 或在文件顶部标注归档状态 | `design/ui-design-prompt.md` |
| 🟡 警告 | 2、5 | Beta design spec 列出 UI Design.md 未标注已归档 | 标注为 `UI Design.md（已归档至 docs/archive/）` | `docs/superpowers/specs/2026-05-30-omnicraft-dual-track-beta-design.md` |
| 🟡 警告 | 2 | `task.json:1582` Task 76 步骤引用 UI Design.md P04a | 标注为 `[已归档] UI Design.md P04a` 或更新对应章节引用 | `task.json` |
| 🟡 警告 | 5 | `design-system.md` Admin/Studio 布局图标注 220px（vs AppLayout 228px） | 统一为 228px（或显式说明 220px 的特殊原因） | `design/design-system.md` |
| 🟡 警告 | 1、5 | `StudioSidebar.tsx:82` 使用 w-56（224px）/ w-[52px]（52px）vs 文档 228px/48px（展开宽度不一致由本次迭代引入） | 更新 Tailwind 类名匹配文档值（`w-[228px]`/`w-12`），同步检查 StudioLayout.tsx | `frontend/components/studio/StudioSidebar.tsx` |
| 🟢 建议 | 1、5 | `globals.css:83` `--chart-1: #0969da` 来自已移除的旧设计系统色值表 | 确认是否替换为 Indigo 系色值，或添加注释说明与品牌主色无关 | `frontend/app/globals.css` |
| 🟢 建议 | 4 | DEC-008 缺少独立风险提示节 | 补充草稿状态混入查询的风险说明 | `docs/2026-06-24-design-review-decisions.md` |
| 🟢 建议 | 4 | DEC-011 缺少独立风险提示节 | 补充"可能存在第三方工具/脚本引用旧色值"的迁移风险 | `docs/2026-06-24-design-review-decisions.md` |
| 🟢 建议 | 4 | DEC-012 标题「更新 UI Design.md 移除二级筛选描述」与实际操作「归档前标注过时内容」不符 | 更新标题为「归档前标注 UI Design.md P06 二级筛选描述过时」或合并入 DEC-013 | `docs/2026-06-24-design-review-decisions.md` |

---

## 补充说明

1. **本次迭代性质**: 本次迭代为**纯文档变更**，无代码改动。需注意：部分文档更新（如侧边栏展开宽度从 224px 改为 228px）**引入了新的代码-文档不一致**（迭代前代码与文档均为 224px，一致），并非仅暴露既存问题。收起宽度的不一致（代码 52px vs 文档原 64px/现 48px）为既存问题。

2. **维度 2 和维度 5 的不通过项**: 维度 2 有 2 项 🔴 阻塞级问题（ui-spec.md 和 ui-design-prompt.md 作为活跃设计文档引用已归档的 UI Design.md 作为权威，会直接误导 Agent），需优先修复。维度 5 的 design-system.md 内部 220px/228px 不一致修复成本均为低（每项 < 5 分钟）。建议在进入修复计划执行阶段前一次性清理。

3. **维度 1 和维度 4 的条件通过**: 维度 1 的 2 项警告（#0969da 残留、侧边栏宽度不一致）和维度 4 的 2 项建议（DEC-008/011 缺少风险提示、DEC-012 标题不符）均为非阻塞性问题，但应记录跟踪。

4. **维度 3 的通过**: 问题清单和决策记录质量高，47 个问题均有完整上下文和决策关联，33 个决策结构完整、内部一致性已正确标注。DEC-007 vs DEC-033、DEC-012 vs DEC-013 的冲突均已显式标注解决路径。8 个抽样问题覆盖 6/10 类别，全部确认真实。

5. **未覆盖范围**: 本审查未对 `docs/2026-06-24-design-review-decisions.md` 中 33 个决策的实施策略进行逐项技术可行性深度评估（如 DEC-014 DDD 重构的工作量估算、DEC-028 HNSW 索引构建对生产的影响等），这些应在修复计划撰写阶段单独评估。抽样未覆盖类别 E/G/I/J。

6. **复审条件**: 修复维度 2 的 2 项 🔴 阻塞（ui-spec.md 和 ui-design-prompt.md 的 UI Design.md 引用）和维度 5 的 design-system.md 内部 220px/228px 不一致后即可通过复审。`StudioSidebar.tsx` 代码修复可延后到后续 Beta 任务。维度 4 的 3 项建议（DEC-008/011 风险提示、DEC-012 标题）可在后续决策文档维护时一并处理。

7. **合并说明**: 本报告由两份独立审查报告合并而成，经交叉验证消除矛盾。主要合并决策包括：
   - 维度 2.2 严重程度采用报告 B 的分级（🔴 阻塞 for items 1-2，🟡 for items 3-4），因为 ui-spec.md 和 ui-design-prompt.md 是活跃设计文档，引用已归档文档作为权威会直接误导 Agent
   - 维度 3.1 抽样采用报告 B 的 8 个样本（覆盖 6/10 类别），补充报告 A 的详细代码证据
   - 维度 4.1 风险提示 DEC 列表使用经验证的正确列表 `DEC-014/015/018/019/020/021/031/032/033`（报告 B 原列表有 6/9 错误，已修正）
   - 维度 1.3 因果分析采用报告 A 的 git diff 验证（确认展开宽度不一致由本次迭代引入）
   - 维度 1.2 Token 区分采用报告 A 的详细说明
   - 维度 5.3 表格综合两版并细化（拆分 AppLayout/AdminLayout/StudioLayout 标签，比两份源报告都更详细）

---

*审查完成时间：2026-06-25*
*本报告为两份独立审查报告的合并版，经交叉验证消除矛盾后生成。*
