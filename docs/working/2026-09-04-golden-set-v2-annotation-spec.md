# Golden Set v2 标注规范与可执行合同（v2.0-draft-r2）

> 创建日期：2026-09-04 ｜ **预计失效日期**: 2026-10-31（v2 冻结报告产生后由其取代）
> 状态：**合同草案——待用户批复后升 v2.0 定稿**；定稿后任何修改须升版本重冻结（seed spec §7 纪律）。
> 上位与关联：`2026-09-02-corpus-v2-seed-spec.md` **v1.1**（分层配比权威已移交本文，见其 §3 v1.1 修订）；审计依据 `2026-09-04-golden-set-audit-report.md` v1.2；069 schema = `backend/migrations/069_rag_evaluation.sql`；runner 现状 = `backend/internal/service/rag_eval/{runner,metrics,answer_eval}.go`。
> 范围纪律（2026-09-04 用户裁决）：**本合同步骤只改规范/映射/验证脚本，不改语料、不改 DB、不改 runner、不生成 196 条、不跑 A-04**。

## 0. 为什么是「可执行合同」

v1 草案描述的六层/三档/双策略/split/provenance 在 069 表、retrieval runner、answer runner 中均无落点：069 无对应列（需明确 JSONB 路径）；runner 只解析数值 `viewer_user_id`（answer eval 更是全局单用户 `ChatViewerUserID` 跑全部 case）；指标只有 Recall@10/@20、二值 nDCG、返回 ID 泄漏。**先把「数据写什么、放哪里、runner 怎么执行」一次定死**，后续 harness 适配与数据生成才都是机械工作。

## 1. 三个产品语义（2026-09-04 两轮裁决，定稿）

### 1.1 no_answer 双策略

| 策略 | 适用 | 正确行为 | 硬失败（判整条败） |
|------|------|---------|------|
| `strict_not_found` | 事实问题 / 指定作品查找 | 明确说明库内找不到；禁止编造 | 回答引用相似文顶替为答案；编造内容 |
| `related_recommendation_allowed` | 内容发现型请求（求推荐/想看） | 明确「没有完全对应」+ 允许推荐真实存在的相似作品 | 把相似文谎称为精确对应；推荐不存在的内容 |

`trap_hit@10`（forbidden 进 Top-10）**对两种策略都只是检索层诊断**——检索器固定返回 Top-K，相似文进结果 ≠ 回答冒充答案；硬失败只发生在引用/回答声明层。related 策略下相似文进 Top-10 不罚。

### 1.2 权限模型：按平台实际能力（is_public 布尔）

- 相关性层（ke/sd/be/hn/na）：principal 用 `anon`（或等价可见性的 `fixture:viewer-anon`），**目标文档只允许 public**。
- visibility 层：principal 在 `anon` 与 `fixture:viewer-anon` **双身份**下运行，四泄漏面（检索结果/引用/回答正文提及/工具步骤事件）全零才通过。
- 运行时无粉丝可见性档：**v2 不假装执行 fans_only 语义**；语料原始 visibility 保留在 `classification.corpus_visibility` 仅作元数据。
- `authorized_private`（作者 fixture 查自己非公开文）：**已裁决延后至 v2.1**，不占 196 条。
- is_public 一致性 preflight：任何重注入/迁移后全量比对语料 visibility ↔ DB is_public（2026-09-04 实测 1600/1600 零漂移）。

### 1.3 多正确结果：三档相关性

- `expected`（必须返回/必引）→ 069 `expected_citations`；`acceptable`（返回也合理）→ `answer_rubric.acceptable_content_ids`；`forbidden`（相似但明显不满足关键约束）→ 069 `forbidden_content_ids`，**每条附 reason**（写入 `answer_rubric.forbidden_reasons`，key→理由）。
- **编造/不存在的作品没有 corpus key，禁止进 forbidden_content_ids**——写入 `answer_rubric.must_not_claim[]`（声明层禁止清单）。
- hard negative 逐条人工判定，不接受「同 IP 即 forbidden」。

## 2. 六层定义与 196 配额（权威：本文；seed spec §3 v1.1 已让渡）

| 层（primary_layer 新值） | 配额 | 目的 | 查询形态约束 |
|----|------|------|-------------|
| known_item_exact | 64 | 标题查找**诊断集**（索引/匹配/IP 过滤/链路），不用于证明语义能力 | 查询=完整标题；16 条 ip_scoped 带「在IP内：」前缀子报（**已裁决：不加无前缀对照条数，不超 196**；如需严格对照另跑不计入冻结配额的配对诊断） |
| semantic_discovery | 48 | 脱离标题的语义泛化 | 藏标题生成；不复用标题独特短语；必要实体（IP 名/角色名）允许保留 |
| body_evidence | 24 | 正文解析/分块/正文向量有效性 | 答案仅正文可得；答案关键信息不得出现在标题 |
| hard_neighbor | 16 | 同 IP 近邻区分度 | 1–3 条 forbidden（人工判三档） |
| no_answer | 20 | 拒答与诚实推荐（域外 12 + 域内 8） | 每条带 strategy；域内题生成时检索验证零精确对应 |
| visibility | 24 | 权限过滤零泄漏 | 查询=受限文档标题；双 principal |

语言/冷门/口语化/IP 覆盖不变量沿用 seed spec（zh≥50%、en≥20%、mixed≥10%、cold≥20%、口语化≥30、16 IP 全覆盖），按 196 折算。

## 3. 069 物理映射（schema_version=2；不新增迁移，全部走既有列/JSONB）

| v2 概念 | 069 落点 | 说明 |
|---------|---------|------|
| case_key | `case_key` | 新命名空间 ke-/sd-/be-/hn-/na-/vi-####；与 gs-c2-* 经 `migration-map-v2.jsonl` 双向可溯 |
| schema_version | `schema_version` = 2 | v1 行保持 1（向后兼容） |
| query / query_language | 同名列 | 原样 |
| principal_key | `viewer_context` JSONB：`{"principal_key":"anon"\|"fixture:viewer-anon"\|"fixture:author:aXX"}` | **冻结数据禁存环境相关数字 ID**；v1 的 `{"viewer_user_id":N}` 由 harness 兼容解析（legacy 路径） |
| expected 三档-必引 | `expected_citations` | 语义=必须返回/必须引用 |
| acceptable | `answer_rubric.acceptable_content_ids` | content_id 数组（冻结时由 key 映射） |
| forbidden（真实语料项） | `forbidden_content_ids` | 仅可解析到 corpus 的条目；reason 存 `answer_rubric.forbidden_reasons` |
| 编造作品禁止声明 | `answer_rubric.must_not_claim[]` | 无 corpus key，声明层禁止清单（judge 规则读此） |
| no_answer 策略 | `classification.no_answer_strategy` | strict_not_found / related_recommendation_allowed |
| split | `classification.split` | "dev" / "test"；harness 按 split 过滤，test 防误跑 |
| provenance | `classification.provenance` | {generator, input_mask, generated_at, human_reviewed} |
| 层 | `classification.primary_layer` | 六层新值；ip_scope/ip/language/temperature_band/corpus_visibility 等维度沿用 |
| relevant + span | `relevant_evidence` JSONB 数组：`{content_id, content_version, source_start, source_end, chunk_key, chunking_version, reason}` | **source_start/end = 正文中真正支持答案的 Unicode code point 半开区间 [start,end)，不机械等于 chunk 边界**；chunk_key/chunking_version 仅为冻结时点的验证快照——重分块后由 harness 按 span 重解析到当前 chunk（069 头注既有设计：「resolved to chunk keys under the current chunking_version at eval time」） |
| 作废 | `is_active=false` | 冻结后不删行，只停用 |

## 4. principal registry（运行时解析）

| principal_key | 实体 | 运行时 |
|---------------|------|--------|
| `anon` | 未登录 | 不带 token |
| `fixture:viewer-anon` | viewer-anon@corpus.omnicraft.local（登录、非关注） | harness 登录该 fixture；关注关系已随语料冻结（56 条 follower_edges 在 DB），case 不声明 follows |
| `fixture:author:aXX` | 语料作者 | v2.1 备用（authorized_private） |

harness 解析规则：见 §8-H1。

## 5. 生成纪律（防同源泄漏）

1. **藏标题出题**（sd）：生成者输入=正文+摘要+IP+分类元数据，无标题；自动校验：查询与标题核心连续重合 zh<4 字 / en<2 内容词，不含标题副题原文。
2. **正文出题**（be）：输入=真实 chunk 正文；答案关键串不得出现在标题（自动校验）。
3. **hard negative**（hn）：从**多检索配置候选并集**（A-04 harness 各配置 Top-K）挖掘，人工判三档+理由。
4. **域内 no_answer**：IP 内合理主题，生成时检索验证零精确对应并留证。
5. **同源隔离**：出题模型 ≠ 语料生成模型（或遮蔽输入并在 provenance 记录 input_mask）；标答由独立 pass 依正文核对。
6. span 标注：人工框定支持区间（codepoint 半开），自动写入当时 chunk 快照；冻结门槛=无 0..400 占位值。
7. 人工复核：sd/be/hn/na/vi 全审，ke 抽 20%（材料 v3）。

## 6. 语料源数据修复（v2 生成前置；本合同步骤不改）

1. **68 条空标题分类处理**（seed spec §1 v1.1 增补）：意图性无标题（讨论短帖）→ `raw_title=""` + 确定性 `display_title`（title_origin=fallback），**不得进 ke 层**；意外缺失 → LLM 候选 + **全量 68 条人工复核**（title_origin=generated）；正式标题 title_origin=raw。index 与 DB 统一保存实际展示/检索标题 + title_origin；重生成 index、同步 DB、三端一致性校验。3 条 DB 真空（content_id 755/1472/496）必须修复。
2. **canonical 标题保留**：不删《》；渲染去重包裹；检索用 normalized_title（NFC+剥外层《》+trim）。
3. is_public preflight（§1.2）。

## 7. 切分：group-aware 80/20（调参前完成）

- **不做 layer×language×IP×temperature 完整笛卡尔分层**（多数组合仅 1 条，不可分）。
- 原子组（不可拆）：同一目标内容的多条 case、同一查询家族（同题变体）、配对变体（如 ke 冒号回归对）。
- 组级分配：先满足原子组约束，再在组间尽量平衡层、语言、IP、冷热；确定性算法（seed=20260904），结果写 `classification.split`，checksum 进冻结报告；**test 约 40 条**，A-04 调参轮次只读 dev，final test 只跑一次。

## 8. 指标合同与 harness v2 适配需求（PR 步骤 2 的验收清单）

分层独立报告（原始分子/分母 + 置信区间），不给综合准确率：

| 层 | 指标 |
|----|------|
| ke | Recall@1/@3、MRR（ip_scoped/global 子报；ASCII 冒号子集修复前/后分列） |
| sd/be/hn | Recall@K、MRR、nDCG@K（hn 另报误中 forbidden 率） |
| na | 检索层 trap_hit@10（双策略均仅诊断）；回答层 strict=拒答率+零编造、related=诚实声明率+推荐真实性（must_not_claim 违反即败） |
| vi | 四泄漏面泄漏率=0 硬门（双 principal 分列） |
| 消融 | 标题遮蔽（sd/be）；四配置 dev-only → 定默认 → final test 一次 |

harness 需求（现状 → 目标，编号供 PR 引用）：
- **H1 principal 解析**：`ParseViewerContext` 现仅读数值 `viewer_user_id` → 增加 `principal_key` 解析 + registry 运行时映射（anon=无 token；fixture 登录）；legacy 数值 id 继续兼容（v1 fixture 不破坏）。
- **H2 answer eval 按 case principal**：现 `ChatViewerUserID` 全局单用户 → 按 case principal_key 逐条切换身份。
- **H3 三档与分层**：metrics 现仅 relevant 集合（Recall/MRR/二值 nDCG）→ expected/acceptable/forbidden 分档计分（acceptable 计入分级相关性增益）+ 按 classification.primary_layer/split 分组聚合。
- **H4 split 过滤**：runner 增加 dev/test 过滤参数；test 运行需显式 flag（防误跑）。
- **H5 span→chunk 解析**：metrics.go 已能解析 span，runner 未用 → eval 时把 relevant_evidence 的 [start,end) 解析到当前 chunking_version 的 chunk 命中（chunk_key 快照仅校验，不作为真值）。
- **H6 no_answer 回答规则**：按 classification.no_answer_strategy 分 strict/related 两套 judge；must_not_claim 违反即败。
- **H7 visibility 四泄漏面**：现仅 `VisibilityLeaks(返回 IDs)` → 增加引用面（citations）、回答正文提及面（标题/摘要/ID 文本匹配 forbidden）、工具步骤事件面（SSE 工具事件流）。
- **H8 向后兼容**：v1 fixture（backend/testdata/rag_golden_cases.json）在 v1 路径下继续通过。

## 9. 人工复核材料 v3 规格

列：case_key | 层 | 查询 | principal | strategy | expected | acceptable | forbidden(含理由) | corpus_visibility | 正文证据摘要（≤60 字）。sd/be/hn/na/vi 全审、ke 抽 20%。由 `scripts/corpus/render_review_materials.py` 扩展生成。

## 10. 执行顺序（2026-09-04 用户裁决六步）

1. 📄 **v2 可执行合同检查点 PR**（本文所在）：规范+069 映射+迁移映射+验证脚本；修订 seed spec v1.1 与审计报告 v1.2。不改语料/DB/runner。
2. 评测 harness v2 适配 PR（§8 H1–H8）。
3. 语料源修复 PR（§6；分类 68 空标题、修 3 条真空、canonical 保留、重建 index/同步 DB/触发重索引、校验三端+is_public+mapping+worker lag）。
4. Golden Set v2 生成与人工复核（§5；span 校准；group-aware 切分；材料 v3）。
5. 冻结 #291（复核通过 → 映射 content_id → 导入 069 → checksum/冻结报告 → 精确标题冒烟 20/20 + worker lag=0 → #316/#327/合同 PR/冻结件回写 #291 关闭）。
6. 执行 A-04（§8：dev-only 四配置 → 定默认 → final test 一次 → 分层报告+消融；na/vi 同时跑回答层）。

## 11. 已裁决问题记录（原三问 + 本轮四修正）

- authorized_private → **延后 v2.1**；68 空标题 → **分类处理**（意图性 fallback 不进 ke；意外缺失 LLM+全量人审）；IP 前缀 → **16 条子报，不加 212**（配对诊断不占冻结配额）。
- 本轮四修正：trap_hit@10 双策略均仅诊断（硬失败在引用/声明层）；编造作品进 answer_rubric 不进 forbidden；span=codepoint 半开区间+chunk 仅快照；80/20 group-aware（原子组约束+维度平衡，约 40 条 test）。

## 12. 旧→新迁移

全量映射 = `artifacts/corpus-v2/golden-set/migration-map-v2.{md,jsonl}`（入仓），由 `scripts/corpus/build_migration_map.py` 确定性生成，`scripts/corpus/verify_migration_map.py` 验证（覆盖 1:1、v2 键唯一、六层配额、处置与审计清单一致、swap 候选池>0、重建一致）。摘要：exact 45 保+19 换 / semantic 38 重写+10 换后重写 / no_answer 12 保+12 合并删+8 新增 / visibility 24 保双身份 / be+24 / hn+16 新增 = 196。
