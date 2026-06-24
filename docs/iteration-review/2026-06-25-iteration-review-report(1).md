# 迭代质量审查报告 (1)

**审查日期**：2026-06-25
**审查范围**：origin/main（f538ef1）→ HEAD（642d804）
**审查人**：Coding Agent (1)
**审查基准文档**：`docs/iteration-review-prompt.md`

---

## 总体结论

| 维度 | 结论 | 关键数据 |
|------|------|---------|
| 维度 1（回归与副作用） | ⚠️ 条件通过 — 2 项低严重度警告 | 代码侧边栏与文档不一致（本次迭代文档修改制造）；#0969da 色值残留（先于本次迭代） |
| 维度 2（归档与清理） | ❌ 不通过 — 4 项残留 | UI Design.md 引用残留于 4 个活跃文档 |
| 维度 3（问题真实性） | ✅ 通过 | 47 个问题全部有决策关联，事实核查修正已标注；抽样 8 个问题覆盖 6/10 类别，全部确认真实 |
| 维度 4（策略质量） | ⚠️ 条件通过 — 24 个决策缺风险提示 | 33 个决策中仅 9 个含独立风险提示节；DEC-012 标题未反映实际操作 |
| 维度 5（文档同步） | ❌ 不通过 — 5 项不一致 | 跨文档侧边栏宽度、活跃引文残留、design-system 内部 220px vs 228px 矛盾 |

**总体评估**：⚠️ 需修复后复审 — 维度 2（4 项残留引用）和维度 5（设计系统内部与代码不一致）需要修复。维度 1 和维度 4 的发现均不阻塞下一步工作。

---

## 维度 1：回归与副作用验证

### 1.1 architecture.md §7 配置字段变更影响

**结论：✅ 通过**

- 在 `backend/internal/` 全局搜索旧字段名 `video_max_size_mb`、`video_max_duration_sec`、`mod_archive_max_size_mb`、`min_score_for_publish` → **无匹配**
- `backend/config.yaml:88-102` 已使用新字段名：`video_max_mb`、`video_max_sec`、`mod_max_mb`、`min_score_for_interaction`（覆盖 limits 段 88-94 和 reputation 段 96-102）
- `backend/config/config.go:99-110` 结构体标签与 `config.yaml` 一致：`mapstructure:"video_max_mb"` 等
- 质量阈值 `quality_content_threshold: 10`、`quality_comment_threshold: 5` 在 `config.yaml:98-99` 已生效
- `architecture.md` 中已无旧字段名残留

### 1.2 architecture.md §10.4 设计系统变更影响

**结论：⚠️ 警告**

- 本次 architecture.md §10.4 删除的是：
- 硬编码的 **GitHub blue 色值表**（`#0969da`、`#ffffff`、`#0d1117` 等具体色值）
- 直接内联在架构文档中的 Tailwind 配置代码块

**保留的是**：设计系统 Token **命名体系**（`canvas`/`border`/`fg`/`accent`）。`design/design-system.md:48-57` 将这些 Token 名重新定义为自定义 Token，赋予 Indigo 系新色值（如 `--accent-emphasis: #4F46E5`），与前端代码中使用的 `bg-canvas-subtle`/`text-fg-muted`/`text-accent-emphasis` 一致。

核查结论：
- 前端代码使用 `bg-canvas-subtle`/`text-fg-muted`/`text-accent-emphasis` 等类名 ✅ — 与 design-system.md 内的自定义 Token 一致
- **⚠️ `frontend/app/globals.css:83`** — `--chart-1: #0969da;` 保留旧 GitHub blue 色值。此为图表色板（shadcn/ui chart 变量），非品牌主色。但在 architecture.md 已删除该色值的背景下，存在混淆风险
  - **严重程度**：🟢 建议
  - **修复建议**：评估是否需要将图表色板也对齐 Indigo 主题（图表色可独立于品牌主色，非阻塞）

### 1.3 architecture.md §13.2 侧边栏宽度变更影响

**结论：🟡 警告 — 代码与文档不一致**

- `architecture.md §13.2`：本次从 224px（w-56）/ 64px（w-16）**改为** 228px / 48px ✅
- `CLAUDE.md` / `AGENTS.md`：同步更新为 228px / 48px ✅
- **❌ 代码未同步**：`frontend/components/studio/StudioSidebar.tsx:82` 仍使用 `"w-56"`（**224px**）和 `"w-[52px]"`（**52px**）

**关键事实**：本次迭代前文档和代码均为 224px/w-56，二者一致。本次迭代只改了文档没改代码，**引入**了一个新的文档-代码差距。这不是纯粹的"既存问题"。
  - **严重程度**：🟡 警告
  - **修复建议**：在后续 Beta 任务中同步更新 StudioSidebar.tsx 的 Tailwind 类名，并检查 StudioLayout.tsx 中依赖侧边栏宽度的布局

### 1.4 CLAUDE.md / AGENTS.md 规则变更影响

**结论：✅ 通过**

- **Key Rule 18**："Soft delete only" → "Soft delete preferred"：变更描述准确反映 DEC-031 的部分统一策略，与代码预期行为一致（`browse_history`、`notifications` 物理删除，其余软删除）
- **模式 A 规则 10**：从"必须保持 false"改为"D-02 至 D-05 完成后开启"：语义更精确，不会误导 Agent 提前开启 `desktop_deploy_enabled`

### 1.5 文档内部逻辑一致性

**结论：✅ 通过**

- **DEC-007 策略 4 vs DEC-033**：`docs/2026-06-24-design-review-decisions.md:532-533` 明确标注"策略 4:通知系统保持轮询(已被 DEC-033 更新)"，矛盾已标注解决
- **问题清单决策后状态**：所有 47 个问题均有决策关联标注或「已修复关闭」标注，与决策记录中的 33 个决策一一对应

### 维度 1 小结

| 检查项 | 结果 |
|--------|------|
| 配置字段重命名 | ✅ 通过 |
| 设计系统 Token（含 Token 名 vs 色值表区分） | ⚠️ #0969da 在 globals.css 图表色板残留（🟢） |
| 侧边栏宽度代码一致性 | ⚠️ 本次文档修改制造了新差距：代码 224px vs 文档 228px（🟡） |
| Key Rules 变更 | ✅ 通过 |
| DEC-007/DEC-033 一致性 | ✅ 通过 |

---

## 维度 2：归档完整性与依赖清理

### 2.1 归档文件移动验证

**结论：✅ 通过**

- `design/UI Design.md` → `docs/archive/UI Design.md` ✅（原位置已不存在，Glob 确认无匹配）
- `design/homepage-v0.html` → `docs/archive/homepage-v0.html` ✅（原位置已不存在）
- `docs/archive/` 目录存在，包含两个文件 ✅

### 2.2 残留引用扫描

**结论：❌ 不通过 — 发现 4 处活跃残留引用**

| # | 文件 | 行号 | 引用内容 | 严重程度 | 修复建议 |
|---|------|------|---------|---------|---------|
| 1 | `design/ui-spec.md` | 5, 8 | "Agent 应参考…UI Design.md 页面小节" / "优先以…UI Design.md 对应页面为准" | 🔴 阻塞 | ui-spec.md 是当前唯一 UI 权威，不应引导 Agent 去参考已归档文档。移除这两处引用，或改为指回 design/design-system.md 和对应 Task 的 steps |
| 2 | `design/ui-design-prompt.md` | 9, 79, 144-154 | 多达 13 处引用 UI Design.md，将其描述为"规格输入清单"、组件来源 | 🔴 阻塞 | 全文替换 UI Design.md 为 docs/archive/UI Design.md（已归档，仅供历史参考） |
| 3 | `task.json` | 1582 | UI Design.md P04a | 🟡 警告 | 标注为 [已归档] UI Design.md P04a 或更新为 design/ui-spec.md 对应章节 |
| 4 | `docs/superpowers/specs/2026-05-30-omnicraft-dual-track-beta-design.md` | 72 | 列出 UI Design.md 未标注已归档 | 🟡 警告 | 标注为 UI Design.md（已归档至 docs/archive/） |

**已排除的引用**（视为历史记录，无需修复）：
- `docs/history/docs edited history.md` — 修改历史记录，引用 UI Design.md 是历史事实
- `docs/2026-06-24-*.md` — 审查报告自身，引用是审查对象的描述
- `docs/iteration-review-prompt.md` — 审查提示词，引用是审查范围描述
- `docs/archive/UI Design.md` — 归档文件自身引用

**未发现 `design/homepage-v0.html` 的活跃残留引用**（除审查报告自身和 `architecture.md:1610` 标注"已归档"外）✅

### 2.3 task.json 中的 ui_spec_ref 字段检查

**结论：⚠️ 发现 1 处**

- `task.json:1582` — Task 76 步骤描述中引用 `UI Design.md P04a`（见 2.2 第 3 项）
- 该引用在 Task 76 步骤描述文本中，Agent 按步骤执行时会找不到对应文档

### 2.4 design/ 目录完整性

**结论：✅ 通过**

`design/` 目录当前包含 4 个文件：
1. `design-system.md` ✅
2. `ui-spec.md` ✅
3. `ui-design-prompt.md` ✅
4. `doc-review-prompt.md` ✅

与 CLAUDE.md/AGENTS.md 项目结构中列出的一致。无遗留文件。

### 2.5 docs/archive/ 目录说明

**结论：✅ 通过**

- CLAUDE.md/AGENTS.md 项目结构中已标注 `├── archive/ # 已归档文档（不再作为开发参考）`
- `design/doc-review-prompt.md:44-46` 中已添加"已归档文档"说明

### 维度 2 小结

| 检查项 | 结果 |
|--------|------|
| 归档文件物理移动 | ✅ 通过 |
| 残留引用扫描 | ❌ 4 处活跃残留 |
| task.json ui_spec_ref | ⚠️ 1 处引用 |
| design/ 目录完整性 | ✅ 通过 |
| archive 目录说明 | ✅ 通过 |

---

## 维度 3：新发现问题真实性与上下文完整性

### 3.1 问题真实性验证（抽样核查）

**结论：✅ 通过（经修正后）**

本次迭代为纯文档变更，抽样侧重问题描述与文档/配置实际状态的一致性。覆盖 6 个类别、8 个问题：

| 抽样问题 | 类别 | 描述准确性 | 验证结果 |
|---------|------|-----------|---------|
| **A1（设计系统三重冲突）** | A | 三套互不一致的设计系统 | 审查时确实存在。architecture.md §10.4 已重写，homepage-v0.html 已归档 ✅ |
| **A2（侧边栏宽度矛盾）** | A | architecture.md 224px/64px vs design-system.md 228px/48px | 审查时确实存在。architecture.md 已更新为 228px/48px ✅ |
| **C1（信誉分配置缺失）** | C | config.yaml reputation 部分缺少加减分值字段 | **确认真实**：`config.yaml:96-102` 仅有 6 个字段，12 个 bonus/penalty 分值全部缺失。reputation_service.go 中硬编码了这些分值 ✅ |
| **C5（Agent LLM 配置混乱）** | C | config.yaml agent.llm_api_key 为空且有"需修复"注释 | 问题清单描述准确（`config.yaml:143` llm_api_key 为空） ✅ |
| **D1（Studio 存根页面）** | D | studio/contributors 为空存根 | 问题清单描述准确（EmptyState 占位，无 API 调用） ✅ |
| **F1（瀑布流无分页）** | F | 原创区只有 24 条无分页 | 问题清单描述准确（original/page.tsx page_size: "24"，无 page/loadMore） ✅ |
| **B3（中文搜索配置不匹配）** | B | 查询侧 to_tsquery('simple') 与索引侧 jiebacfg 不匹配 | 已标注事实核查修正（索引侧已用 jiebacfg，查询侧用 simple），与 DEC-001 核查一致 ✅ |
| **H1（Token XSS 泄露风险）** | H | Token 存储方案不安全 | 已标注事实核查修正（Refresh Token 已在 HttpOnly Cookie），与 DEC-002 修正结论一致 ✅ |

> **抽样说明**：覆盖 6/10 类别（A/B/C/D/F/H），8/47 问题（~17%）。未覆盖类别 E/G/I/J。所有抽样问题经代码/配置交叉核查后均确认真实。

### 3.2 已关闭问题核查

**结论：✅ 通过**（基于问题清单自述和代码状态交叉验证）

| 问题 | 声称状态 | 验证 |
|------|---------|------|
| B1（推荐缓存 TTL） | ✅ 已修复关闭 | 问题清单确认 |
| F2（侧边栏折叠） | ✅ 已修复关闭 | StudioSidebar.tsx 确认有折叠+图标+tooltip |
| F3（ContentCard 单一实现） | ✅ 已修复关闭 | 问题清单确认 |
| F4（分类 Tab sticky） | ✅ 已修复关闭 | 问题清单确认 |
| G3（URL Scheme） | ✅ 已修复关闭 | 问题清单确认 |
| H2（密码重置状态） | ✅ 已修复关闭 | 问题清单确认 |

### 3.3 事实核查修正标注验证

**结论：✅ 通过**

- **B3**（`merged.md:234`）：修正标注清晰说明了索引侧已使用 jiebacfg，实际问题是查询侧不匹配。与 DEC-001 核实结论一致 ✅
- **F1**（`:605`）：修正标注清晰说明了实际问题是"没有无限滚动"而非"无状态恢复"。与 DEC-006 核实结论一致 ✅
- **H1**（`:825`）：修正标注清晰说明了 Token 存储"已在 HttpOnly Cookie"。与 DEC-002 核实结论一致 ✅

### 3.4 上下文完整性检查

**结论：✅ 通过**

所有 47 个问题均包含：
- 发现位置（文件路径 + 章节/行号）✅
- 问题描述（具体、无歧义）✅
- 建议方案（可执行、技术可行）✅
- 来源标注（[A] / [B] / [A+B]）✅
- 决策关联（→ 决策:DEC-XXX 或 ✅ 已修复关闭）✅

### 3.5 决策关联完整性

**结论：✅ 通过**

- 问题清单优先级矩阵（`merged.md:979-1027`）：47 个问题全部有决策后状态 ✅
- 决策记录索引（`decisions.md:14-50`）：33 个决策全部有关联问题 ✅
- 决策记录末尾覆盖清单（`decisions.md:1799-1829`）：确认 33 个决策覆盖全部 47 个问题 ✅

### 维度 3 小结

| 检查项 | 结果 |
|--------|------|
| 问题真实性（抽样） | ✅ 通过 |
| 已关闭问题核查 | ✅ 通过 |
| 事实核查修正标注 | ✅ 通过 |
| 上下文完整性 | ✅ 通过 |
| 决策关联完整性 | ✅ 通过 |

---

## 维度 4：决策策略质量评估

### 4.1 策略全面性

**结论：⚠️ 33 个决策中仅 9 个（27%）有独立风险提示段落**

全量统计 `### 风险提示` 段落：DEC-014/015/018/019/020/021/031/032/033 共 **9 个**含独立风险提示。其余 **24 个**（73%）无独立风险提示。

抽检 DEC-001、DEC-004、DEC-007、DEC-008、DEC-011、DEC-031、DEC-032 的结构完整性：

| 决策 | 问题核实 | 明确决策 | 实施策略 | 实施要点 | 验证标准 | 风险提示 |
|------|---------|---------|---------|---------|---------|---------|
| DEC-001 | ✅ | ✅ | ✅（4 条） | ✅ | ✅ | ✅ |
| DEC-004 | ✅ | ✅ | ✅（3 条） | ✅ | ✅ | ✅ |
| DEC-007 | ✅ | ✅ | ✅（4 条） | ✅ | ✅ | ✅ |
| DEC-008 | ✅ | ✅ | ✅（5 条） | ✅ | ✅ | ⚠️ 缺失 |
| DEC-011 | ✅ | ✅ | ✅（2 条） | ✅ | ✅ | ⚠️ 缺失 |
| DEC-031 | ✅ | ✅ | ✅（含迁移 SQL） | ✅ | ✅ | ✅ |
| DEC-032 | ✅ | ✅ | ✅（3 条） | ✅ | ✅ | ✅ |

- **DEC-008**（草稿系统完整实现）：不含独立风险提示。该决策涉及 `content_items.status='draft'` 复用，存在草稿数据与已发布数据混在同一查询中的风险（需确保所有查询默认过滤 `status != 'draft'`），建议补充
- **DEC-011**（设计系统统一）：不含风险提示。建议补充"可能存在第三方工具/脚本引用旧色值"的迁移风险
- 其余 5 个抽检决策均有完整结构 ✅

> **说明**：风险提示缺失不代表决策质量差——部分决策风险本身较低。但统一补充后有助于后续实施者评估影响面。

### 4.2 技术合理性

**结论：✅ 通过（无阻塞性问题）**

| 关键决策 | 评估 |
|---------|------|
| **DEC-001**（查询侧对齐 jiebacfg） | ✅ 合理：动态探测 + fallback 模式成熟，避免硬编码配置名 |
| **DEC-004**（阈值统一为 10） | ✅ 合理：降低优质内容门槛利于早期社区活跃度 |
| **DEC-008**（复用 content_items 表） | ✅ 合理：避免表膨胀，status='draft' 区分清晰 |
| **DEC-014**（DDD 完整重构） | ⚠️ 潜在风险：与"尽量让功能全面"导向存在张力。DDD 重构投入大（决策标注"高成本"），可能延迟功能交付。但决策本身在技术上是合理的长期投资 |
| **DEC-028**（向量索引升级 HNSW） | ✅ 合理：HNSW 在查询性能上显著优于 IVFFlat，适合生产环境 |
| **DEC-031**（软删除部分统一） | ✅ 合理：browse_history 和 notifications 物理删除的理由充分 |

### 4.3 内部一致性

**结论：⚠️ 1 项注意**

1. **DEC-012 与 DEC-013 的语义冲突**（`decisions.md:834`）：
   - DEC-012 标题为"更新 UI Design.md 移除二级筛选描述"
   - DEC-013 标题为"归档 UI Design.md"
   - 决策文档自身已标注："此决策与 DEC-013(归档 UI Design.md)冲突。DEC-013 决定归档 UI Design.md,因此 DEC-012 的'更新'操作改为'归档前标注过时内容'"
   - **评估**：冲突已在文档内解决，但 DEC-012 的标题未反映实际执行的操作。该冲突不影响执行，但可能使后续 Agent 阅读时困惑
   - **严重程度**：🟢 建议

2. **DEC-007 vs DEC-033**：
   - ✅ DEC-007 策略 4 已标注"(已被 DEC-033 更新)"
   - ✅ DEC-033 实施策略中明确"与 DEC-007 的协调"
   - 无矛盾

3. **迁移编号检查**：
   - DEC-001 → 迁移 056、DEC-008 → 迁移 057、DEC-028 → 迁移 058、DEC-031 → 迁移 059
   - **无编号冲突** ✅

### 4.4 表述清晰度

**结论：✅ 通过**

- "可选，或复用"等模糊用语：已在 DEC-008 中修复（明确要求改为精确用语），无残留
- "100 个任务" vs "86 个任务"：已在 DEC-021 中修复，CLAUDE.md/AGENTS.md 已统一为"100+ 个历史任务"
- API 路径：DEC-007 的 `GET /api/v1/messages/stream` 和 DEC-033 的 `GET /api/v1/notifications/stream` 清晰无歧义
- 决策标题准确反映决策内容 ✅

### 4.5 暂缓/搁置决策评估

**结论：✅ 通过**

| 决策 | 状态 | 解除条件 | 明确性 |
|------|------|---------|--------|
| DEC-003（法律文本搁置） | ⏸️ 搁置 | "用户自行处理法律文本" | ✅ 明确 |
| DEC-016（Tauri 离线暂缓） | ⏸️ 暂缓 | 标注"降级 P3"，明确为低优先级 | ✅ 明确 |
| DEC-017（PWA 暂缓） | ⏸️ 暂缓 | 标注"降级 P3" | ✅ 明确 |
| DEC-026（Tauri 账号同步暂缓） | ⏸️ 暂缓 | "未来重启"，有明确暂缓理由 | ✅ 明确 |

### 维度 4 小结

| 检查项 | 结果 |
|--------|------|
| 策略全面性 | ⚠️ 仅 9/33 决策有风险提示（含 DEC-008/DEC-011 缺失） |
| 技术合理性 | ✅ 通过 |
| 内部一致性 | ⚠️ DEC-012 标题未反映实际操作（文档内已解决冲突） |
| 表述清晰度 | ✅ 通过 |
| 暂缓/搁置决策 | ✅ 通过 |

---

## 维度 5：文档同步与冲突验证

### 5.1 跨文档字段名一致性

**结论：✅ 通过**

| 字段 | architecture.md §7 | backend/config.yaml | CLAUDE.md/AGENTS.md | backend/config.go |
|------|-------------------|---------------------|---------------------|-------------------|
| `video_max_mb` | ✅ | ✅ (89) | N/A | ✅ (99) |
| `video_max_sec` | ✅ | ✅ (90) | N/A | ✅ (100) |
| `mod_max_mb` | ✅ | ✅ (93) | N/A | ✅ (103) |
| `min_score_for_interaction` | ✅ | ✅ (97) | ✅ | ✅ (110) |
| `quality_content_threshold: 10` | ✅ | ✅ (98) | ✅ | ✅ (108) |
| `quality_comment_threshold: 5` | ✅ | ✅ (99) | ✅ | ✅ (109) |

旧字段名（`video_max_size_mb`、`mod_archive_max_size_mb`、`min_score_for_publish`、`quality_content_threshold: 50`、`quality_comment_threshold: 20`）在三份文档中均已清除 ✅

### 5.2 设计系统权威性验证

**结论：⚠️ 警告**

- `architecture.md §10.4`：明确指向 `design/design-system.md` 和 `design/ui-spec.md` 为唯一权威 ✅
- `CLAUDE.md` / `AGENTS.md` Key Rule 14/15：指向 `design/ui-spec.md` ✅
- `design/doc-review-prompt.md`：文档清单序号 4 已更新 ✅
- ⚠️ `design/ui-spec.md:5,8`：自身仍引用 `UI Design.md` 作为 fallback 权威（见维度 2.2）
- ⚠️ `design/ui-design-prompt.md:9`：将 `UI Design.md` 描述为"本文档是规格的输入清单"
- ⚠️ `docs/superpowers/specs/2026-05-30-omnicraft-dual-track-beta-design.md:72`：参考文档清单中列出 `UI Design.md`（未标注已归档）

### 5.3 侧边栏宽度一致性

**结论：❌ 不通过 — 多处不一致**

| 位置 | 展开宽度 | 收起宽度 | 状态 |
|------|---------|---------|------|
| `architecture.md §13.2` | 228px | 48px | ✅ |
| `CLAUDE.md` / `AGENTS.md` | 228px | 48px | ✅ |
| `design/design-system.md:63,113,157` | 228px | 48px | ✅ |
| `design/design-system.md:168` (AdminLayout 图) | **220px** | — | ❌ |
| `design/design-system.md:178` (StudioLayout 图) | **220px** | — | ❌ |
| `StudioSidebar.tsx:82` | **224px** (`w-56`) | **52px** (`w-[52px]`) | ❌ |

- **文档内部不一致**：`design-system.md` 中 AppLayout 图标注 228px（line 157），但 AdminLayout 图（line 168）和 StudioLayout 图（line 178）标注 220px。需确认这三个布局的侧边栏是否应统一为 228px，还是 Admin/Studio 侧边栏有意使用不同宽度（若是，需在文档中说明原因）
- **代码-文档不一致**：`StudioSidebar.tsx` 实际使用 `w-56`（224px）和 `w-[52px]`（52px），与文档的 228px/48px 不匹配

### 5.4 信誉分阈值一致性

**结论：✅ 通过**

- `architecture.md §7`：`quality_content_threshold: 10`、`quality_comment_threshold: 5` ✅
- `backend/config.yaml`：匹配 ✅
- `CLAUDE.md` / `AGENTS.md`：加分规则表格的阈值描述使用"从 config.yaml 读取"动态引用，不硬编码 ✅

### 5.5 下载权限描述一致性

**结论：✅ 通过**

- `CLAUDE.md:435`：已更新为"信誉分 < config.yaml > reputation.min_score_for_interaction（默认 3）用户禁止下载"，添加统一门槛说明 ✅
- `AGENTS.md:435`：同步更新 ✅

### 5.6 软删除策略描述一致性

**结论：✅ 通过**

- `CLAUDE.md` Key Rule 18："以软删除为主…仅对确无分析价值的数据（如浏览历史、已读通知）使用物理删除" ✅
- `AGENTS.md` Key Rule 18：同步 ✅
- DEC-031 物理删除表清单（`browse_history`、`notifications`）与 Key Rule 18 描述一致 ✅
- 不存在"禁止物理删除"与"部分物理删除"的矛盾 ✅

### 5.7 桌面端部署描述一致性

**结论：✅ 通过**

- `architecture.md §7`：新增 `desktop_deploy_enabled: false`（注释"D-02~D-05 完成后开启"）✅
- `CLAUDE.md` / `AGENTS.md` 模式 A 规则 10："D-02 至 D-05 完成后开启…R-02 验证通过前不得发布到生产环境" ✅
- DEC-032：明确 "D-02 到 D-05 完成后设置 desktop_deploy_enabled: true" ✅
- 三处描述一致 ✅

### 5.8 task.json 状态描述一致性

**结论：✅ 通过**

- `CLAUDE.md` / `AGENTS.md` 模式 B 说明：标注"100+ 个历史任务已全部完成（passes: true），仅作历史记录保留" ✅
- `design/doc-review-prompt.md` 文档清单序号 5：标注"100+ 个开发任务，已 100% 完成，仅作历史记录" ✅
- 描述一致 ✅

### 5.9 项目结构一致性

**结论：✅ 通过**

- `CLAUDE.md` 和 `AGENTS.md` 项目结构部分一致（仅"本文件"注释行不同）✅
- 项目结构中 `design/` 目录列出了所有 4 个文件 ✅
- 项目结构中包含 `docs/archive/` 目录 ✅
- 项目结构中包含 `docs/2026-06-24-*.md` 审查报告 ✅

### 5.10 过时信息扫描

**结论：⚠️ 1 项残留**

在 `architecture.md` 中搜索过时关键词：

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

但 `frontend/app/globals.css:83` 仍保留 `--chart-1: #0969da`（见维度 1.2）。

### 维度 5 小结

| 检查项 | 结果 |
|--------|------|
| 跨文档字段名一致性 | ✅ 通过 |
| 设计系统权威性 | ⚠️ ui-spec/ui-design-prompt/Beta spec 仍引用 UI Design.md |
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
| 🔴 阻塞 | 2 | ui-spec.md 引用已归档的 UI Design.md 作为权威来源 | 移除 line 5, 8 的 UI Design.md 引用，改为指回 design/design-system.md 和对应 Task 的 steps | design/ui-spec.md |
| 🔴 阻塞 | 2 | ui-design-prompt.md 全文 13+ 处引用 UI Design.md 作为有效输入 | 全文替换 UI Design.md 为 docs/archive/UI Design.md（已归档，仅供历史参考）或移除 | design/ui-design-prompt.md |
| 🟡 警告 | 2 | task.json:1582 — Task 76 步骤引用 UI Design.md P04a | 标注为 [已归档] UI Design.md P04a 或更新对应章节引用 | task.json |
| 🟡 警告 | 2 | Beta design spec 列出 UI Design.md 未标注已归档 | 标注为 UI Design.md（已归档至 docs/archive/） | docs/superpowers/specs/2026-05-30-omnicraft-dual-track-beta-design.md |
| 🟡 警告 | 5 | design-system.md Admin/Studio 布局图标注 220px（vs AppLayout 228px） | 统一为 228px（或显式说明 220px 的特殊原因） | design/design-system.md |
| 🟡 警告 | 1, 5 | StudioSidebar.tsx:82 使用 w-56（224px）/ w-[52px]（52px）vs 文档 228px/48px | 更新 Tailwind 类名匹配文档值，同步检查 StudioLayout.tsx | frontend/components/studio/StudioSidebar.tsx |
| 🟢 建议 | 1, 5 | globals.css:83 --chart-1: #0969da 来自已移除的旧设计系统色值表 | 确认是否替换为 Indigo 系色值，或添加注释说明 | frontend/app/globals.css |
| 🟢 建议 | 4 | DEC-008 缺少独立风险提示 | 补充草稿状态混入查询的风险说明 | docs/2026-06-24-design-review-decisions.md |
| 🟢 建议 | 4 | DEC-012 标题与实际操作（归档前标注）不符 | 更新标题或合并入 DEC-013 | docs/2026-06-24-design-review-decisions.md |

---

## 附录：审查方法说明

- **审查工具**：git diff、git log、grep、Glob、Read
- **审查范围**：3 个 commit（8e83fe8、3ecef1a、642d804），11 个变更文件
- **未审查内容**：docs/2026-06-24-*.md 审查报告内部的一致性和质量（属于原始审查工作产品，本审查仅验证其与决策记录的交叉引用）
- **不涉及**：代码逻辑正确性验证（本次迭代为纯文档变更）

---

*审查完成时间：2026-06-25*
*本报告由 Coding Agent (1) 生成。v1.1 修正：C1 验证结论订正、维度 1/4 结论等级调整、抽样覆盖扩展至 8 问题 6 类别、Token 名 vs 色值表区分、风险提示全量统计（9/33）*
