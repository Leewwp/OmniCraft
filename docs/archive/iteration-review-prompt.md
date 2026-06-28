# OmniCraft 迭代质量审查提示词

> **用途**：将本提示词连同项目当前状态提供给 Coding Agent，执行对本次迭代全部改动的系统性质量审查。
>
> **适用范围**：本次迭代包含两个新增文档（设计审查问题清单、决策记录）以及对 CLAUDE.md、AGENTS.md、architecture.md、design/doc-review-prompt.md 的修改，并归档了 design/UI Design.md 和 design/homepage-v0.html。
>
> **审查基准**：`origin/main`（commit f538ef1）→ 当前 HEAD（本次迭代所有 commit）

---

## 提示词正文

你是一名严格的全栈架构师兼质量保证工程师，擅长系统性审查文档变更、代码修改和项目结构演进。请基于本次迭代的全部改动，执行以下五维质量审查，并输出结构化报告。

### 审查范围

本次迭代涉及以下改动（通过 `git log origin/main..HEAD` 和 `git diff origin/main..HEAD` 获取完整变更）：

**新增文档**：
1. `docs/2026-06-24-omnicraft-documentation-review.md` — 文档一致性审查报告（文档 A）
2. `docs/design-review-2026-06-24.md` — 架构设计与 UX 审查报告（文档 B，用户提供）
3. `docs/2026-06-24-omnicraft-design-review-merged.md` — 合并版问题清单（47 个问题，发现层）
4. `docs/2026-06-24-design-review-decisions.md` — 决策记录（33 个决策，决策层）

**修改文档**：
5. `architecture.md` — §7 配置对齐、§10.4 设计系统、§13.1/§13.2 侧边栏宽度
6. `CLAUDE.md` / `AGENTS.md` — 项目结构、模式 B 说明、下载权限、软删除规则、桌面端部署规则、侧边栏宽度
7. `design/doc-review-prompt.md` — 移除 UI Design.md 引用，改为 ui-spec.md

**归档文件**：
8. `design/UI Design.md` → `docs/archive/UI Design.md`
9. `design/homepage-v0.html` → `docs/archive/homepage-v0.html`

---

## 审查任务

请完成以下五维审查，逐项输出结果。每个维度必须给出明确的"通过/不通过"结论，不通过项需提供具体证据和修复建议。

### 维度 1：回归与副作用验证

**目标**：确认本次改动未引入新 bug、错误或非预期行为，未破坏既有功能。

**审查清单**：

1. **architecture.md §7 配置字段变更影响**
   - 检查 `video_max_size_mb → video_max_mb`、`video_max_duration_sec → video_max_sec`、`mod_archive_max_size_mb → mod_max_mb`、`min_score_for_publish → min_score_for_interaction` 的字段重命名
   - 在 `backend/internal/` 全局搜索旧字段名，确认代码中读取配置的代码是否仍使用旧字段名（若是，则为回归 bug）
   - 检查 `backend/config.yaml` 中的字段名是否与新文档一致
   - 检查 `quality_content_threshold: 50→10`、`quality_comment_threshold: 20→5` 的阈值变更是否会影响正在运行的信誉分计算逻辑

2. **architecture.md §10.4 设计系统变更影响**
   - 确认删除 GitHub blue 色值表后，是否有前端代码仍引用 `canvas`、`border.default`、`fg`、`accent.emphasis` 等 Token（这些是旧 Token 名）
   - 检查 `frontend/tailwind.config.ts` 是否与 `design/design-system.md` 一致
   - 确认 `frontend/app/globals.css` 中的 CSS 变量是否与文档描述的 Indigo `#4F46E5` 一致

3. **architecture.md §13.2 侧边栏宽度变更影响**
   - 检查 `frontend/components/studio/StudioSidebar.tsx` 实际使用的宽度类名
   - 确认 `w-56`（224px）和 `w-16`（64px）是否仍存在于代码中（若是，则文档与代码不一致）
   - 检查 `frontend/components/studio/StudioLayout.tsx` 中的布局是否依赖特定宽度值

4. **CLAUDE.md / AGENTS.md 规则变更影响**
   - 确认 Key Rule 18 从 "Soft delete only" 改为 "Soft delete preferred" 后，是否与现有代码中的删除逻辑冲突
   - 确认模式 A 规则 10（桌面端部署）的描述变更不会误导 Agent 在 D-02~D-05 未完成时就开启 `desktop_deploy_enabled`

5. **文档内部逻辑一致性**
   - 检查决策记录中 DEC-007 策略 4（通知系统保持轮询）与 DEC-033（通知系统 SSE 推送）是否存在未标注的矛盾
   - 检查问题清单中所有 47 个问题的"决策后状态"是否与决策记录中的决策一致

**输出要求**：
- 列出所有发现的回归风险
- 每个风险标注：严重程度（🔴阻塞/🟡警告/🟢建议）、具体文件和行号、修复建议
- 若无回归风险，明确标注 ✅ 通过

---

### 维度 2：归档完整性与依赖清理

**目标**：确认所有标记为删除或归档的内容已从活跃项目组件中完全移除，关联依赖已正确清理，无残留引用。

**审查清单**：

1. **归档文件移动验证**
   - 确认 `design/UI Design.md` 已移至 `docs/archive/UI Design.md`，原位置不存在
   - 确认 `design/homepage-v0.html` 已移至 `docs/archive/homepage-v0.html`，原位置不存在
   - 确认 `docs/archive/` 目录存在且包含上述两个文件

2. **残留引用扫描（全项目）**
   - 在整个项目中搜索 `design/UI Design.md`（排除 `docs/2026-06-24-*.md` 审查报告和 `docs/archive/` 自身），确认无活跃引用
   - 在整个项目中搜索 `design/homepage-v0.html`（同上排除），确认无活跃引用
   - 检查以下文件是否仍引用已归档文档：
     - `.specify/memory/constitution.md`
     - `frontend/CLAUDE.md`
     - `docs/superpowers/plans/*.md`（Beta 路线图相关）
     - `docs/superpowers/specs/*.md`
     - 任何 `README.md` 文件
     - 任何 `*.json` 配置文件（如 `task.json` 中的 `ui_spec_ref` 字段）

3. **task.json 中的 ui_spec_ref 字段检查**
   - 搜索 `task.json` 中是否有 `ui_spec_ref` 字段引用了 `UI Design.md` 的章节（如 `P01`、`P06` 等）
   - 若有，确认这些引用是否需要更新为 `design/ui-spec.md` 的对应章节

4. **design/ 目录完整性**
   - 确认 `design/` 目录下仅保留：`design-system.md`、`ui-spec.md`、`ui-design-prompt.md`、`doc-review-prompt.md`
   - 确认无其他遗留文件

5. **docs/archive/ 目录说明**
   - 确认归档目录的用途已在 CLAUDE.md/AGENTS.md 项目结构中标注为"已归档文档（不再作为开发参考）"
   - 确认 design/doc-review-prompt.md 中已添加归档文档说明

**输出要求**：
- 列出所有残留引用（文件路径 + 行号 + 引用内容）
- 每个残留引用给出修复建议（删除引用 / 更新路径 / 保留作为历史记录）
- 若无残留引用，明确标注 ✅ 通过

---

### 维度 3：新发现问题真实性与上下文完整性

**目标**：确认问题清单中标记为"新发现"的问题真实、可复现，并附带完整准确的上下文描述。

**审查清单**：

1. **问题真实性验证（抽样核查）**
   - 从 47 个问题中抽样核查以下关键问题，确认问题描述与代码/配置实际状态一致：
     - **C1（信誉分配置缺失）**：检查 `backend/config.yaml` 的 `reputation` 部分，确认加减分值字段是否确实缺失
     - **C5（Agent LLM 配置混乱）**：检查 `backend/config.yaml` 的 `agent.llm_api_key`，确认是否为空且有"需修复"注释
     - **D1（Studio 存根页面）**：检查 `frontend/app/(protected)/studio/contributors/page.tsx`，确认是否为空存根
     - **F1（瀑布流无分页）**：检查 `frontend/app/(public)/original/page.tsx`，确认是否只请求 24 条无分页
     - **B3（中文搜索配置不匹配）**：检查 `backend/internal/repository/search_repo.go`，确认查询侧是否用 `to_tsquery('simple', ?)` 而索引侧用 `jiebacfg`
   - 对每个抽样问题，验证：问题描述是否准确、发现位置是否正确、建议方案是否技术可行

2. **已关闭问题核查**
   - 确认 6 个已关闭问题（B1/F2/F3/F4/G3/H2）的"已修复"状态是否属实：
     - **B1（推荐缓存 TTL）**：检查推荐缓存是否确实有 2h TTL
     - **F2（侧边栏折叠）**：检查 StudioSidebar 是否确实有折叠+图标+tooltip
     - **F3（ContentCard 单一实现）**：检查是否确实只有一个 ContentCard 组件
     - **F4（分类 Tab sticky）**：检查原创区分类 Tab 是否确实有 sticky 定位
     - **G3（URL Scheme）**：检查 Tauri 客户端是否确实实现了 URL Scheme
     - **H2（密码重置状态）**：检查密码重置流程是否确实有三种状态提示

3. **事实核查修正标注验证**
   - 确认 B3、F1、H1 的"⚠️ 事实核查修正"标注内容是否准确
   - 确认修正后的描述与代码实际状态一致

4. **上下文完整性检查**
   - 对每个问题，检查是否包含：
     - 发现位置（文件路径 + 章节/行号）
     - 问题描述（具体、无歧义）
     - 建议方案（可执行、技术可行）
     - 来源标注（[A] / [B] / [A+B]）
     - 决策关联（→ 决策:DEC-XXX 或 ✅ 已修复关闭）

5. **决策关联完整性**
   - 确认所有 47 个问题都有决策关联标注或已关闭标注
   - 确认决策记录中的 33 个决策都有对应的关联问题
   - 确认问题清单的优先级矩阵与决策记录的决策索引一致

**输出要求**：
- 列出所有描述不准确的问题（问题编号 + 不准确之处 + 实际状态）
- 列出所有上下文不完整的问题（问题编号 + 缺失字段）
- 若全部真实且完整，明确标注 ✅ 通过

---

### 维度 4：决策策略质量评估

**目标**：评估所有问题解决策略是否全面、技术合理、内部一致、表述清晰无歧义。

**审查清单**：

1. **策略全面性**
   - 对每个决策（DEC-001 到 DEC-033），检查是否包含：
     - 问题核实结论（含代码核查）
     - 明确的决策（方案 A/B/暂缓等）
     - 实施策略（具体步骤）
     - 实施要点（注意事项）
     - 验证标准（可衡量）
     - 风险提示（如适用）
   - 标注缺失字段的决策

2. **技术合理性**
   - 评估以下关键技术决策的合理性：
     - **DEC-001**：查询侧配置对齐 jiebacfg（动态探测 + fallback）— 是否合理？
     - **DEC-004**：阈值统一为 10（而非 architecture.md 的 50）— 业务上是否合理？
     - **DEC-008**：复用 content_items 表（status='draft'）而非新建 content_drafts 表 — 是否合理？
     - **DEC-014**：DDD 完整重构 — 是否与"尽量让功能全面"导向一致？
     - **DEC-028**：向量索引升级为 HNSW — 是否比 IVFFlat 更适合当前规模？
     - **DEC-031**：软删除策略部分统一 — 物理删除表清单是否合理？

3. **内部一致性**
   - 检查决策之间是否存在矛盾：
     - DEC-007（私信 SSE）与 DEC-033（通知 SSE）是否冲突？DEC-007 策略 4 是否已标注被 DEC-033 更新？
     - DEC-008（草稿系统）与 DEC-008 策略 5（定时发布 E2）是否逻辑自洽？
     - DEC-011（统一 design-system.md）与 DEC-013（归档 UI Design.md）是否互补？
     - DEC-032（桌面端部署现在实施）与 CLAUDE.md 模式 A 规则 10 是否一致？
   - 检查迁移编号是否冲突（DEC-001: 056, DEC-008: 057, DEC-028: 058, DEC-031: 059）

4. **表述清晰度**
   - 检查是否有歧义表述：
     - "可选，或复用" 类模糊用语（已在 DEC-008 中修复，确认无残留）
     - "100 个任务" vs "86 个任务" 类数据不一致（已在 DEC-021 中修复，确认无残留）
     - API 路径描述是否清晰（G6 的 `/api/v1/messages/stream` 是否无歧义）
   - 检查决策标题是否准确反映决策内容

5. **暂缓/搁置决策评估**
   - 确认以下暂缓/搁置决策是否有明确的解除条件：
     - DEC-003（法律文本搁置）— 解除条件是否明确？
     - DEC-016（Tauri 离线暂缓）— 是否标注降级 P3？
     - DEC-017（PWA 暂缓）— 是否标注降级 P3？
     - DEC-026（Tauri 账号同步暂缓）— 是否有未来重启计划？

**输出要求**：
- 列出所有策略不全面的决策（决策编号 + 缺失字段）
- 列出所有技术合理性存疑的决策（决策编号 + 存疑之处 + 建议替代方案）
- 列出所有内部一致性冲突（冲突的决策对 + 冲突描述）
- 列出所有表述不清晰的决策（决策编号 + 歧义之处）
- 若全部通过，明确标注 ✅ 通过

---

### 维度 5：文档同步与冲突验证

**目标**：确认所有相关文档已更新以反映当前变更，最新本地文档版本完全同步，无内容冲突、歧义和过时信息。

**审查清单**：

1. **跨文档字段名一致性**
   - 对比以下三处的字段名，确认完全一致：
     - `architecture.md §7`（本次修改后）
     - `backend/config.yaml`（实际配置）
     - `CLAUDE.md` / `AGENTS.md`（业务规则部分）
   - 重点检查：
     - `video_max_mb`（非 `video_max_size_mb`）
     - `mod_max_mb`（非 `mod_archive_max_size_mb`）
     - `min_score_for_interaction`（非 `min_score_for_publish`）
     - `quality_content_threshold: 10`（非 50）
     - `quality_comment_threshold: 5`（非 20）

2. **设计系统权威性验证**
   - 确认以下文档均指向 `design/design-system.md` 作为唯一设计权威：
     - `architecture.md §10.4`（本次修改后）
     - `CLAUDE.md` / `AGENTS.md`（Key Rule 14/15）
     - `design/doc-review-prompt.md`（文档清单序号 4）
   - 确认无文档仍引用 `design/UI Design.md` 作为设计规范（审查报告中的历史引用除外）

3. **侧边栏宽度一致性**
   - 确认以下文档的侧边栏宽度值一致（228px/48px）：
     - `architecture.md §13.2`（本次修改后）
     - `CLAUDE.md` / `AGENTS.md`（侧边栏结构部分）
     - `design/design-system.md`（`--sidebar-w` 变量）
   - 检查 `design/design-system.md` 内部是否存在 220px vs 228px 的不一致

4. **信誉分阈值一致性**
   - 确认以下文档的信誉分阈值一致（10/5）：
     - `architecture.md §7`（本次修改后）
     - `backend/config.yaml`（实际配置）
     - `CLAUDE.md` / `AGENTS.md`（信誉分体系部分）
   - 检查 CLAUDE.md 中"加分规则"表格的阈值描述是否仍引用旧值

5. **下载权限描述一致性**
   - 确认以下文档的下载权限描述一致（统一使用 `min_score_for_interaction`）：
     - `CLAUDE.md` / `AGENTS.md`（内容下载部分，本次修改后）
     - `architecture.md`（如有下载权限相关描述）

6. **软删除策略描述一致性**
   - 确认以下文档的软删除描述一致（以软删除为主，部分物理删除）：
     - `CLAUDE.md` / `AGENTS.md`（Key Rule 18，本次修改后）
     - `architecture.md`（如有软删除相关描述）
   - 检查是否存在"禁止物理删除"与"部分物理删除"的矛盾

7. **桌面端部署描述一致性**
   - 确认以下文档的桌面端部署描述一致：
     - `CLAUDE.md` / `AGENTS.md`（模式 A 规则 10，本次修改后）
     - `architecture.md §7`（`desktop_deploy_enabled` 字段，本次新增）
     - `docs/superpowers/plans/2026-05-30-omnicraft-beta-desktop-deploy-security.md`（如有相关描述）

8. **task.json 状态描述一致性**
   - 确认以下文档对 task.json 状态的描述一致（已 100% 完成，仅作历史记录）：
     - `CLAUDE.md` / `AGENTS.md`（模式 B 说明，本次修改后）
     - `design/doc-review-prompt.md`（文档清单序号 5，本次修改后）

9. **项目结构一致性**
   - 确认 `CLAUDE.md` 和 `AGENTS.md` 的项目结构部分一致（仅"本文件"注释行不同）
   - 确认项目结构中 `design/` 目录列出了所有 4 个文件
   - 确认项目结构中包含 `docs/archive/` 目录

10. **过时信息扫描**
    - 在 `architecture.md` 中搜索以下过时关键词，确认无残留：
      - `GitHub 风格`（§10.4 标题已移除）
      - `#0969da`（GitHub blue 色值）
      - `w-56`（旧侧边栏宽度类名，ASCII 图中已移除）
      - `video_max_size_mb`、`mod_archive_max_size_mb`、`min_score_for_publish`（旧字段名）
      - `quality_content_threshold: 50`、`quality_comment_threshold: 20`（旧阈值）

**输出要求**：
- 列出所有文档不一致项（文档 A 位置 + 文档 B 位置 + 不一致描述）
- 列出所有过时信息残留（文件 + 行号 + 过时内容）
- 若全部一致，明确标注 ✅ 通过

---

## 输出格式

请按以下结构输出审查报告：

```markdown
# 迭代质量审查报告

**审查日期**：[日期]
**审查范围**：origin/main（f538ef1）→ HEAD
**审查人**：[Agent 名称]

## 总体结论

- 维度 1（回归与副作用）：[✅ 通过 / ❌ 不通过 — X 项风险]
- 维度 2（归档与清理）：[✅ 通过 / ❌ 不通过 — X 项残留]
- 维度 3（问题真实性）：[✅ 通过 / ❌ 不通过 — X 项不准确]
- 维度 4（策略质量）：[✅ 通过 / ❌ 不通过 — X 项存疑]
- 维度 5（文档同步）：[✅ 通过 / ❌ 不通过 — X 项不一致]

**总体评估**：[可进入下一步 / 需修复后复审]

## 维度 1：回归与副作用验证

### 1.1 architecture.md §7 配置字段变更影响
[详细发现]

### 1.2 architecture.md §10.4 设计系统变更影响
[详细发现]

...

## 维度 2：归档完整性与依赖清理

### 2.1 归档文件移动验证
[详细发现]

### 2.2 残留引用扫描
[详细发现]

...

## 维度 3：新发现问题真实性与上下文完整性

### 3.1 问题真实性验证（抽样核查）
[详细发现]

...

## 维度 4：决策策略质量评估

### 4.1 策略全面性
[详细发现]

...

## 维度 5：文档同步与冲突验证

### 5.1 跨文档字段名一致性
[详细发现]

...

## 修复建议汇总

| 优先级 | 维度 | 问题 | 修复建议 | 涉及文件 |
|--------|------|------|---------|---------|
| 🔴 阻塞 | ... | ... | ... | ... |
| 🟡 警告 | ... | ... | ... | ... |
| 🟢 建议 | ... | ... | ... | ... |
```

## 补充约束

- **不要修改任何文件**，仅输出审查报告
- 所有发现必须标注具体文件路径和行号（或章节号）
- 对于"不通过"项，必须提供可执行的修复建议
- 保持客观，不要为了凑问题而制造伪问题；如果某个维度确实没问题，直接标注 ✅ 通过
- 审查报告应使用中文，技术术语可保留英文
- 对于审查报告中引用的代码位置，使用 `文件路径:行号` 格式（如 `architecture.md:1256`）
