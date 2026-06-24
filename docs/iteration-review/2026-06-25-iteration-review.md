# 迭代质量审查报告

**审查日期**: 2026-06-25
**审查范围**: origin/main（f538ef1）→ HEAD（642d804）
**审查人**: GLM-5.2 Agent
**审查依据**: `docs/iteration-review-prompt.md` 五维质量审查提示词
**本次迭代 commit**:
- `8e83fe8` Docs: add design review report system (issue list + decisions)
- `3ecef1a` Docs: resolve design system conflicts and archive obsolete docs
- `642d804` Docs: add iteration quality review prompt for coding agent

## 总体结论

| 维度 | 结论 | 阻塞项 | 警告项 |
|------|------|--------|--------|
| 维度 1（回归与副作用） | ⚠️ 条件通过 | 0 | 2 |
| 维度 2（归档与清理） | ❌ 不通过 | 0 | 4 |
| 维度 3（问题真实性） | ✅ 通过 | 0 | 0 |
| 维度 4（策略质量） | ⚠️ 条件通过 | 0 | 2 |
| 维度 5（文档同步） | ❌ 不通过 | 0 | 3 |

**总体评估**: 需修复后复审。维度 2 和维度 5 各有少量残留问题，维度 1 和维度 4 存在条件性警告，修复成本均为低。本次迭代主体质量高，文档归档、决策记录、问题清单均达到可执行标准。

本次迭代为**纯文档变更**，无代码改动。需注意：部分文档更新（如侧边栏宽度从 224px 改为 228px）**引入了新的代码-文档不一致**，而非仅暴露既存问题。

---

## 维度 1: 回归与副作用验证

### 1.1 architecture.md §7 配置字段变更影响

**核查结果**: ✅ 通过

- 全局搜索 `video_max_size_mb|video_max_duration_sec|mod_archive_max_size_mb|min_score_for_publish` 在 `backend/` 中 **无任何匹配** — 旧字段名已完全清除
- `backend/config/config.go:98-105` `LimitsConfig` 结构体使用新字段名 `video_max_mb`/`video_max_sec`/`image_max_mb`/`text_max_mb`/`mod_max_mb`/`sheet_music_max_mb` — 与 architecture.md §7 一致
- `backend/config.yaml:88-94` 实际配置使用新字段名 — 一致
- `backend/config.yaml:98-99` `quality_content_threshold: 10`、`quality_comment_threshold: 5` — 与更新后的 architecture.md 一致
- `backend/config/config.go:110` `MinScoreForInteraction` 字段 — 与新命名一致

**阈值变更影响**: 阈值从 50/20 → 10/5 的变更已在 config.yaml 中生效（实际配置本来就是 10/5，本次只是让 architecture.md 对齐），不会影响运行中的信誉分计算逻辑。

### 1.2 architecture.md §10.4 设计系统变更影响

**核查结果**: 🟡 警告（非回归，但存在文档与代码不一致）

- architecture.md §10.4 已删除 GitHub blue `#0969da` 色值表 ✅
- architecture.md 中搜索 `#0969da`、`GitHub 风格` — **无残留** ✅
- `frontend/app/globals.css:83` 仍存在 `--chart-1: #0969da;` — 这是图表色板，非主色调，属于 🟢 建议优化项
- `design/design-system.md:48-57` 仍保留 `--canvas-default`/`--canvas-subtle`/`--border-default`/`--fg-muted`/`--accent-emphasis`/`--accent-subtle` 作为「自定义 Token」 — 这些 Token 名与代码中 `bg-canvas-subtle`/`text-fg-muted`/`text-accent-emphasis` 一致，**不是旧 Token 残留**
- 前端代码使用 `bg-canvas-subtle`/`text-fg-muted`/`text-accent-emphasis` 与 design-system.md 定义一致 ✅

**关键区分**: architecture.md §10.4 删除的是**写死 `#0969da` 等具体色值的旧色值表代码块**，而非 Token 命名体系。Token 命名体系（`canvas`/`border`/`fg`/`accent`）被 `design-system.md` 保留并映射到 Indigo 色值（如 `--accent-emphasis` 指向 `#4f46e5` 而非旧 GitHub blue）。因此代码中引用这些 Token 名是正确行为，无需清理。

### 1.3 architecture.md §13.2 侧边栏宽度变更影响

**核查结果**: 🟡 警告（本次迭代引入了新的代码-文档不一致）

- `architecture.md §13.2` 已更新为「展开 228px / 收起 48px」 ✅
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

---

## 维度 2: 归档完整性与依赖清理

### 2.1 归档文件移动验证

**核查结果**: ✅ 通过

- `design/UI Design.md` 已移至 `docs/archive/UI Design.md`，原位置不存在 ✅
- `design/homepage-v0.html` 已移至 `docs/archive/homepage-v0.html`，原位置不存在 ✅
- `docs/archive/` 目录存在且包含上述两个文件 ✅

### 2.2 残留引用扫描

**核查结果**: ❌ 不通过 — 4 项活跃残留引用

| # | 文件 | 行号 | 引用内容 | 严重程度 | 修复建议 |
|---|------|------|---------|---------|---------|
| 1 | `design/ui-spec.md` | 5, 8 | `Agent 应参考对应 Task 的 ui_spec_ref 和 UI Design.md 页面小节获取真实组件列表` / `优先以 ## Component: 具体组件的规范和 UI Design.md 对应页面为准` | 🟡 警告 | 将 `UI Design.md` 替换为 `design/ui-spec.md` 自身或移除该引用，因为 UI Design.md 已归档 |
| 2 | `design/ui-design-prompt.md` | 9, 79, 144-154 | `UI Design.md —— 本文档是规格的输入清单，列出了所有 P01-P29 页面...` 等 10+ 处引用 | 🟡 警告 | 此文件是「UI 设计生成提示词」，将 UI Design.md 作为输入清单引用。归档后应更新为引用 `design/ui-spec.md` 或在文件顶部标注「UI Design.md 已归档，本提示词仅作历史参考」 |
| 3 | `docs/superpowers/specs/2026-05-30-omnicraft-dual-track-beta-design.md` | 72 | 文件清单中列出 `UI Design.md` | 🟡 警告 | 更新为 `design/ui-spec.md` 或标注「已归档」 |
| 4 | `task.json` | 1582 | Task 76 steps 描述中引用 `UI Design.md P04a` | 🟢 建议 | 标注为 `[已归档] UI Design.md P04a`。虽属历史任务描述，但模式 B 仍可能触发该任务，Agent 按 steps 执行时会尝试查找已不存在的 UI Design.md |

**可接受的引用**（历史记录，无需修改）:
- `docs/2026-06-24-*.md` 审查报告中的引用 — 历史快照
- `docs/history/docs edited history.md` — 历史修改记录

### 2.3 task.json 中的 ui_spec_ref 字段检查

**核查结果**: ✅ 通过（ui_spec_ref 字段无问题，但 steps 描述有残留引用）

- task.json 中 20+ 个 `ui_spec_ref` 字段引用的都是章节名（如 `## Global Interaction Patterns`、`## Component: Header`），**不是文件路径** ✅
- `task.json:1582` 在 Task 76 steps 描述中提及 `UI Design.md P04a` — 已列入 2.2 残留引用清单第 4 项（🟢 建议级），建议标注为 `[已归档]`

### 2.4 design/ 目录完整性

**核查结果**: ✅ 通过

`design/` 目录下仅保留 4 个文件:
- `design-system.md`
- `ui-spec.md`
- `ui-design-prompt.md`
- `doc-review-prompt.md`

### 2.5 docs/archive/ 目录说明

**核查结果**: ✅ 通过

- `CLAUDE.md:258` 和 `AGENTS.md:258` 项目结构中标注 `archive/  # 已归档文档（不再作为开发参考）` ✅
- `design/doc-review-prompt.md:44-46` 已添加归档文档说明 ✅

---

## 维度 3: 新发现问题真实性与上下文完整性

### 3.1 问题真实性验证（抽样核查）

**核查结果**: ✅ 通过 — 5 个抽样问题全部属实

| 问题 | 描述准确性 | 代码证据 | 结论 |
|------|-----------|---------|------|
| **C1**（信誉分配置缺失） | ✅ 准确 | `config.yaml:96-102` 只有 threshold 字段，无 bonus/penalty；`reputation_service.go:85,89,103,107,111,122,126,130,134,138,142,153` 全部硬编码 `3`/`2`/`1`/`-3`/`-2`/`-1` | 真实 |
| **C5**（Agent LLM 配置混乱） | ✅ 准确 | `config.yaml:143` `llm_api_key: ""` 注释「留空，改用 env var 注入（需修复 config.go）」 | 真实 |
| **D1**（Studio 存根页面） | ✅ 准确 | `studio/contributors/page.tsx` 仅 18 行，纯 EmptyState，无 API 调用 | 真实 |
| **F1**（瀑布流无分页） | ✅ 准确 | `original/page.tsx:73` `page_size: "24"` 无 page 参数，无 loadMore，无无限滚动 | 真实 |
| **B3**（中文搜索配置不匹配） | ✅ 准确 | `search_repo.go:149,169,170,171,175` 全部使用 `to_tsquery('simple', ?)` / `phraseto_tsquery('simple', ?)` / `ts_headline('simple', ...)` — 与索引侧 `jiebacfg` 不匹配 | 真实 |

### 3.2 已关闭问题核查

**核查结果**: ✅ 通过 — 6 个已关闭问题标注合理

- **B1/F2/F3/F4/G3/H2** 均在问题清单中标注「✅ 已修复关闭（核查确认）」并附核查依据（如 `recommendation_service.go:128-137 显式设置 TTL`、`StudioSidebar.tsx:135-141` 等）
- `decisions:1776-1787`「已关闭问题」表格列出每个问题的核查结论和关闭依据

### 3.3 事实核查修正标注验证

**核查结果**: ✅ 通过

- **B3 修正**: `merged:234` 明确标注「⚠️ 事实核查修正」— 索引侧已用 `jiebacfg`，实际问题在查询侧 — 与代码核查一致
- **F1 修正**: `merged:605` 明确标注「⚠️ 事实核查修正」— 实际不是「无限滚动无状态恢复」而是「根本没有无限滚动」— 与代码核查一致
- **H1 修正**: `merged:825` 明确标注「⚠️ 事实核查修正」— Refresh Token 已在 HttpOnly Cookie 中 — 与 DEC-002 核查结论一致

### 3.4 上下文完整性检查

**核查结果**: ✅ 通过

抽样核查 5 个问题（C1/C5/D1/F1/B3），每个均包含:
- ✅ 发现位置（文件路径 + 章节/行号）
- ✅ 问题描述（具体、无歧义）
- ✅ 建议方案（可执行、技术可行）
- ✅ 来源标注（`[A]` / `[B]` / `[A+B]`）
- ✅ 决策关联（`→ 决策:DEC-XXX` 或 `✅ 已修复关闭`）

### 3.5 决策关联完整性

**核查结果**: ✅ 通过

- 47 个问题全部有决策关联标注或已关闭标注（见 `merged:979-1027` 优先级矩阵）
- 33 个决策在 `decisions:15-51` 决策索引中均有对应关联问题
- 优先级矩阵与决策索引一致 ✅

---

## 维度 4: 决策策略质量评估

### 4.1 策略全面性

**核查结果**: ⚠️ 条件通过 — 2 个决策缺少独立风险提示节

抽样核查 DEC-001/DEC-004/DEC-005/DEC-008/DEC-014/DEC-031，各决策均包含问题核实结论、明确决策、实施策略、实施要点、验证标准。但风险提示节存在缺失：

| 决策 | 问题核实 | 明确决策 | 实施策略 | 实施要点 | 验证标准 | 风险提示 |
|------|---------|---------|---------|---------|---------|---------|
| DEC-001 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| DEC-004 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| DEC-005 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| DEC-008 | ✅ | ✅ | ✅ | ✅ | ✅ | ⚠️ 缺失 |
| DEC-014 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅（标注「高风险：涉及 100+ 文件移动」） |
| DEC-031 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

- **DEC-008**（草稿系统完整实现）：无独立 `### 风险提示` 节。该决策涉及 `content_items.status='draft'` 复用，存在草稿数据与已发布数据混在同一查询中的风险（需确保所有查询默认过滤 `status != 'draft'`）。虽然"实施要点"中提到了部分注意事项，但未独立声明风险。33 个决策中共有 9 个包含独立 `### 风险提示` 节，DEC-008 属于未包含的决策之一。
- **DEC-011**（设计系统统一）：同样无独立 `### 风险提示` 节。建议补充"可能存在第三方工具/脚本引用旧色值"的迁移风险。

### 4.2 技术合理性

**核查结果**: ✅ 通过

| 决策 | 合理性评估 | 结论 |
|------|-----------|------|
| **DEC-001**（动态探测 pg_jieba + fallback） | 合理 — 兼顾生产环境（jiebacfg）和开发环境（simple fallback），不引入 gojieba 二进制依赖 | ✅ |
| **DEC-004**（阈值统一为 10） | 合理 — config.yaml 实际值为 10/5，architecture.md 的 50/20 是文档过时；个位数加减分符合实际业务规模 | ✅ |
| **DEC-008**（复用 content_items 表 status='draft'） | 合理 — 避免新建表，利用现有 status 枚举扩展，草稿不进入 search_vector | ✅ |
| **DEC-014**（DDD 完整重构） | 合理 — 与用户导向「尽量让功能全面」一致；风险已标注，建议独立分支执行 | ✅ |
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

| 决策 | 解除条件 | 评估 |
|------|---------|------|
| **DEC-003**（法律文本搁置） | `decisions:217`「用户完成法律文本起草后，告知版本号和文本内容，即可启动代码实施」 | ✅ 明确 |
| **DEC-016**（Tauri 离线暂缓） | `decisions:1062`「标记为 P3 长期优化」 | ✅ 标注降级 |
| **DEC-017**（PWA 暂缓） | `decisions:1091`「标记为 P3 长期优化」 | ✅ 标注降级 |
| **DEC-026**（Tauri 账号同步暂缓） | `decisions:1441`「暂缓，标记为长期优化」 | ✅ 标注暂缓 |

---

## 维度 5: 文档同步与冲突验证

### 5.1 跨文档字段名一致性

**核查结果**: ✅ 通过

| 字段 | architecture.md §7 | config.yaml | CLAUDE.md/AGENTS.md |
|------|-------------------|-------------|---------------------|
| `video_max_mb` | ✅ | ✅ | ✅ |
| `mod_max_mb` | ✅ | ✅ | ✅ |
| `min_score_for_interaction` | ✅ | ✅ | ✅ |
| `quality_content_threshold: 10` | ✅ | ✅ | ✅ |
| `quality_comment_threshold: 5` | ✅ | ✅ | ✅ |

### 5.2 设计系统权威性验证

**核查结果**: ❌ 不通过 — design/ 目录下文档仍引用已归档的 UI Design.md

- `architecture.md §10.4` 已改为指向 `design/design-system.md` 和 `design/ui-spec.md` ✅
- `CLAUDE.md:596-597` Key Rule 14/15 指向 `design/ui-spec.md` ✅
- `design/doc-review-prompt.md:39` 文档清单序号 4 指向 `design/design-system.md + design/ui-spec.md` ✅
- ❌ `design/ui-spec.md:5,8` 自身仍引用 `UI Design.md` 作为 fallback 权威（见维度 2.2 第 1 项）
- ❌ `design/ui-design-prompt.md:9` 将 `UI Design.md` 描述为"本文档是规格的输入清单"（见维度 2.2 第 2 项）
- ❌ `docs/superpowers/specs/2026-05-30-omnicraft-dual-track-beta-design.md:72` 参考文档清单中列出 `UI Design.md`（未标注已归档）（见维度 2.2 第 3 项）

**说明**: 5.2 不仅需检查 architecture.md/CLAUDE.md/doc-review-prompt.md 是否正确指向 design-system.md，还需检查 design/ 目录下文档自身是否包含对已归档文档的引用。后者检查发现 3 处残留引用，与维度 2.2 交叉确认。

### 5.3 侧边栏宽度一致性

**核查结果**: ❌ 不通过 — 2 项不一致

| 位置 | 宽度值 | 期望值 | 状态 |
|------|--------|--------|------|
| `architecture.md §13.2:2131-2132` | 228px/48px | 228px/48px | ✅ |
| `CLAUDE.md:499` | 228px/48px | 228px/48px | ✅ |
| `design/design-system.md:63` | `--sidebar-w: 228px` | 228px | ✅ |
| `design/design-system.md:113` | 228px/48px | 228px/48px | ✅ |
| `design/design-system.md:157` | 228px | 228px | ✅ |
| `design/design-system.md:168` | **220px** | 228px | ❌ |
| `design/design-system.md:178` | **220px** | 228px | ❌ |
| `StudioSidebar.tsx:82` | `w-56`(224px)/`w-[52px]`(52px) | 228px/48px | ❌（展开宽度不一致由本次迭代引入，收起宽度不一致为既存） |

**修复建议**:
1. 将 `design/design-system.md:168` 和 `:178` 的 `220px` 改为 `228px`
2. 将 `StudioSidebar.tsx:82` 的 `w-56` 改为 `w-[228px]`，`w-[52px]` 改为 `w-12`（48px）— 建议在后续 Beta 任务中修复

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
- 无「禁止物理删除」与「部分物理删除」的矛盾 ✅

### 5.7 桌面端部署描述一致性

**核查结果**: ✅ 通过

- `CLAUDE.md` 模式 A 规则 10「D-02 至 D-05 完成后开启 `features.desktop_deploy_enabled: true`」 ✅
- `architecture.md §7` 新增 `desktop_deploy_enabled: false  # 桌面端一键部署（D-02~D-05 完成后开启）` ✅
- `config.go:95` `DesktopDeployEnabled` 字段存在 ✅

### 5.8 task.json 状态描述一致性

**核查结果**: ✅ 通过

- `CLAUDE.md` 模式 B 说明「task.json 中 100+ 个历史任务已全部完成（passes: true），仅作历史记录保留」 ✅
- `design/doc-review-prompt.md:40` 文档清单序号 5「历史任务列表：100+ 个开发任务，已 100% 完成，仅作历史记录」 ✅

### 5.9 项目结构一致性

**核查结果**: ✅ 通过

- CLAUDE.md 和 AGENTS.md 项目结构部分一致（仅「本文件」注释行不同） ✅
- 项目结构中 `design/` 目录列出 4 个文件 ✅
- 项目结构中包含 `docs/archive/` 目录 ✅

### 5.10 过时信息扫描

**核查结果**: ✅ 通过

在 architecture.md 中搜索以下过时关键词:
- `GitHub 风格` — **无残留** ✅
- `#0969da` — **无残留** ✅
- `w-56` — **无残留**（ASCII 图中已改为 228px） ✅
- `video_max_size_mb`/`mod_archive_max_size_mb`/`min_score_for_publish` — **无残留** ✅
- `quality_content_threshold: 50`/`quality_comment_threshold: 20` — **无残留** ✅

---

## 修复建议汇总

| 优先级 | 维度 | 问题 | 修复建议 | 涉及文件 |
|--------|------|------|---------|---------|
| 🟡 警告 | 维度 2、5 | `design/ui-spec.md` 仍引用 `UI Design.md` 作为权威来源 | 将第 5、8 行的 `UI Design.md` 替换为 `design/ui-spec.md` 自身或移除引用 | `design/ui-spec.md:5,8` |
| 🟡 警告 | 维度 2、5 | `design/ui-design-prompt.md` 仍将 `UI Design.md` 作为输入清单引用（10+ 处） | 在文件顶部标注「UI Design.md 已归档，本提示词仅作历史参考」或更新引用为 `design/ui-spec.md` | `design/ui-design-prompt.md:9,79,144-154` |
| 🟡 警告 | 维度 2、5 | Beta 设计规格文件清单仍列出 `UI Design.md` | 更新为 `design/ui-spec.md` 或标注「已归档」 | `docs/superpowers/specs/2026-05-30-omnicraft-dual-track-beta-design.md:72` |
| 🟢 建议 | 维度 2 | `task.json:1582` Task 76 steps 描述引用 `UI Design.md P04a` | 标注为 `[已归档] UI Design.md P04a` | `task.json:1582` |
| 🟡 警告 | 维度 5 | `design-system.md` 内部 220px vs 228px 不一致 | 将第 168、178 行的 `220px` 改为 `228px` | `design/design-system.md:168,178` |
| 🟡 警告 | 维度 1、5 | `StudioSidebar.tsx` 代码使用 `w-56`(224px)/`w-[52px]`(52px)，与文档 228px/48px 不一致（展开宽度不一致由本次迭代引入） | 将 `w-56` 改为 `w-[228px]`，`w-[52px]` 改为 `w-12`（48px）— 建议在后续 Beta 任务中修复 | `frontend/components/studio/StudioSidebar.tsx:82` |
| 🟢 建议 | 维度 1 | `globals.css` 图表色板仍含 `#0969da`（GitHub blue） | 评估是否需要将图表色板也对齐 Indigo 主题（非阻塞，图表色与主色调独立） | `frontend/app/globals.css:83` |
| 🟢 建议 | 维度 4 | DEC-008 缺少独立风险提示节 | 补充草稿状态混入查询的风险说明 | `docs/2026-06-24-design-review-decisions.md` |
| 🟢 建议 | 维度 4 | DEC-011 缺少独立风险提示节 | 补充"可能存在第三方工具/脚本引用旧色值"的迁移风险 | `docs/2026-06-24-design-review-decisions.md` |
| 🟢 建议 | 维度 4 | DEC-012 标题「更新 UI Design.md 移除二级筛选描述」与实际操作「归档前标注过时内容」不符 | 更新标题为「归档前标注 UI Design.md P06 二级筛选描述过时」或合并入 DEC-013 | `docs/2026-06-24-design-review-decisions.md` |

---

## 补充说明

1. **本次迭代性质**: 本次迭代为**纯文档变更**，无代码改动。需注意：部分文档更新（如侧边栏展开宽度从 224px 改为 228px）**引入了新的代码-文档不一致**（迭代前代码与文档均为 224px，一致），并非仅暴露既存问题。收起宽度的不一致（代码 52px vs 文档原 64px/现 48px）为既存问题。

2. **维度 2 和维度 5 的不通过项**: 均为残留引用清理不彻底或文档内部数值不一致，修复成本均为低（每项 < 5 分钟）。建议在进入修复计划执行阶段前一次性清理。

3. **维度 1 和维度 4 的条件通过**: 维度 1 的 2 项警告（#0969da 残留、侧边栏宽度不一致）和维度 4 的 2 项建议（DEC-008/011 缺少风险提示、DEC-012 标题不符）均为非阻塞性问题，但应记录跟踪。

4. **维度 3 的通过**: 问题清单和决策记录质量高，47 个问题均有完整上下文和决策关联，33 个决策结构完整、内部一致性已正确标注。DEC-007 vs DEC-033、DEC-012 vs DEC-013 的冲突均已显式标注解决路径。

5. **未覆盖范围**: 本审查未对 `docs/2026-06-24-design-review-decisions.md` 中 33 个决策的实施策略进行逐项技术可行性深度评估（如 DEC-014 DDD 重构的工作量估算、DEC-028 HNSW 索引构建对生产的影响等），这些应在修复计划撰写阶段单独评估。

6. **复审条件**: 修复维度 2 的 4 项残留引用和维度 5 的 design-system.md 内部 220px/228px 不一致后即可通过复审。`StudioSidebar.tsx` 代码修复可延后到后续 Beta 任务。维度 4 的 3 项建议（DEC-008/011 风险提示、DEC-012 标题）可在后续决策文档维护时一并处理。

7. **修订记录**: 本报告已根据交叉审查反馈进行修订，主要变更包括：纠正维度 1.3 因果关系分析（展开宽度不一致由本次迭代引入）、纠正维度 5.2 内部矛盾（design/ 目录下文档仍有 UI Design.md 残留引用）、将 task.json:1582 从"可接受引用"移入残留引用清单、纠正维度 4.1 对 DEC-008 风险提示的过度泛化判定、补充维度 4.3 DEC-012 标题问题、补充维度 1.2 Token 名与色值表的区分说明。
