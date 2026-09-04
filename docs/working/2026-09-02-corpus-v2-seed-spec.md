# 语料 v2 注入与 Golden Set 冻结规格（CORPUS-01 · #291）

> 创建日期：2026-09-02 ｜ **预计失效日期**: 2026-10-31（原表述：#291 收口后）
> 版本：**v1.1（2026-09-04 修订）**——golden set 分层配比与两处指标语义由 `2026-09-04-golden-set-v2-annotation-spec.md`（v2 可执行合同）取代，见 §1/§3/§5 内 v1.1 标注；其余正文继承 v1.0（2026-09-02 二轮修订，采纳外部审查 8 点，已对照 `069_rag_evaluation.sql`、`rag_eval/metrics.go`、`rag-deepening-design.md §6`、`agent-workspace-rework-design.md §7` 逐条核实）。
> 来源：用户 2026-09-02 批准的总序修订一 + 同日 golden set 审查采纳决定。
> 权威关系：执行顺序以总序 §3 为准（第 6 段 A-03 与 A-04 之间）；**golden set 规模与指标合同由本 spec 统一定义**，`agent-workspace-rework-design.md §7` 的「30–50 条 / Recall@5」表述由本 spec 取代（K=5 保留为消融报告附加列）。

## 0. 目标与定位

为 A-04（#286）四配置消融与 #204 面试证据链提供有规模说服力的数据底座：合成语料（本地，目标 chunks 8k–15k）+ 冻结 golden set（150–200 条）。生产演示种子（2k–5k）为人工执行清单，不属 idle goal。

## 1. 语料生成矩阵（独立 ZCode 会话执行，产出为纯文本，不入库）

**生成模式（v1.1 变更）**：单个长时 goal 内**父-子 Agent 编排**——父 Agent 管状态/分片/派发/验收，是 manifest 的唯一写入者；子 Agent 分片生成（每片 30–50 条，独立批次文件与 index 分片）。断点续跑、同提示词幂等，可退化为定时多会话批次模式。**分档策略**：批量生成 goal 以 flash 档启动（父+子同模型——派发工具不支持为子 Agent 指定其他模型）；golden set 收口以 5.3 档手动单独执行（会话级分档）。

| 维度 | 设计 | 目的 |
| --- | --- | --- |
| IP | 16 个真实头部 IP（2026-09-02 用户授权自热门榜候选选定，名单冻结见附录 B；不爬取任何内容） | 演示可理解性；面试追问可接 |
| 品类 | 长篇同人（多章节深标题链，占比 ≥40% 以撑 chunk 总量）/ 短篇 / 设定集（2–3% 含 Markdown 表格）/ 讨论首帖（含无标题短帖样本） | 覆盖 chunker 三边界；达 chunks 目标 |

> **（v1.1 增补，2026-09-04）无标题短帖样本的标题语义**：意图性无标题短帖保留 `raw_title=""`，检索/展示用确定性 `display_title` 回退（`title_origin=fallback`），且**不得进入 golden set known-item exact 层**；意外缺失标题的条目以 LLM 生成候选并**全量人工复核**（`title_origin=generated`；有正式标题者 `title_origin=raw`）。index 与 DB 统一保存实际展示/检索标题并保留 `title_origin` 供审计。
| 标题形态 | 精确型 / 模糊型 / 部分重叠型按比例混合 | golden set 精确/语义分层的真值基础 |
| 可见性 | 公开 / 仅粉丝 / 私密分层；虚拟作者 20–30 个 + 作者-粉丝关系网（冻结 fixture） | 可见性过滤与引用复核的评测与演示弹药 |
| 版本链 | 约 10% 内容带 2–4 个版本 | 过期引用丢弃的演示弹药 |
| 时间分布 | 发布时间铺开 ≥ 90 天 | 站点观感真实 |
| 语言 | zh 主体；en 内容 8–12%（英文圈向同人）；mixed 标题/中英混排若干 | 对齐 069 语言分布不变量（en 查询需有可命中语料） |
| 冷热带 | 冷门/长尾题材 ≥ 20% | 对齐 069 classification 口径（cold ≥ 20%） |
| 规模 | 内容 **1,500–2,500 篇 → chunks 8k–15k**（会话内生成模式的现实规模） | 规模锚点（≥10k 才可写"万级"，8–10k 写"近万级"） |
| 质量控制 | 多样性 prompt 控制、标题与正文相似度去重、5% 人工抽样 | 避免低区分度文本稀释评测 |

产出：corpus-batch-*.jsonl（正文 Markdown + 完整元数据 + `corpus_item_key` 稳定键）+ manifest（checkpoint、规模、分布统计、checksum）。语料文件目录 gitignore，manifest 与统计入库。

## 2. 注入方案（分层）

### 2.1 本地全量——REST API 注入

- 虚拟作者 token 经真实 REST API 发布；**禁止直写数据库**（绕过发布事件 = 索引永不更新）。
- 走完整真实管线：表单校验 → 审核门 → 发布事件 → Outbox → Worker → chunk + embed → 双路索引。
- 脚本要求：限流（2–5 rps）、断点续跑（幂等）、错误率阈值熔断（>2% 停止）、管理员审核流转（不走免审白名单）。
- 注入后按 AGENTS Step 2 核查 worker lag=0；建立 `corpus_item_key → DB content_id` 映射表（golden set 冻结前置）。

### 2.2 本地小子集——Playwright UI 注入（Agent 扮演真实用户）

20–50 条内容 + 若干评论/讨论/收藏/关注，经浏览器表单完成：主流程 E2E 验证 + 演示素材。评论不入检索索引。

### 2.3 生产种子（2k–5k）——人工执行清单（不属 idle goal）

pg_dump 备份 → 稳定 release 后低峰窗口 → 同一脚本指向生产 HTTPS API、限速 1–2 rps → 前后健康检查（/healthz、P95、postgres 内存、worker lag）→ 截图归档 #204。口径红线：种子数据永远是"种子数据"，不出现用户量/DAU/运营数据表述。

## 3. Golden Set（150–200 条，对齐 069 schema）

- **机器真源 = PostgreSQL `eval_golden_cases`（069 迁移，schema_version 随导出）**；JSONL 仅为草稿与 CI 导出物（对齐 rag-deepening §6"PostgreSQL 为唯一真源"）。草稿阶段用 `corpus_item_key`，#291 注入后映射为 DB content_id 再冻结。
- **规模：总量 150–200 条**，其中含 **≥30–50 条「口语化推荐场景」子层**（口语化兴趣描述 → 期望命中实体集合，吸收 rework design §7 的扩充目标，归入语义层）。
- **配比（primary_layer）与取整**：精确 = floor(N×0.4)、语义 = floor(N×0.3)（含口语化推荐子层）、可见性 = floor(N×0.15)、无答案 = N − 前三者（承接取整误差）。
  - **（v1.1 修订，2026-09-04）本条四层固定配比已由 v2 六层方案取代**：known_item_exact 64 / semantic_discovery 48 / body_evidence 24 / hard_neighbor 16 / no_answer 20（域外 12 + 域内 8）/ visibility 24，合计 196（仍在 150–200 区间）。依据 = 检索有效性审计（`2026-09-04-golden-set-audit-report.md`：semantic 48/48 标题复读、29 条权限矛盾、no_answer 重复等）+ 用户 2026-09-04 两轮裁决。分层与配额权威转至 `2026-09-04-golden-set-v2-annotation-spec.md`，本条保留为历史口径；语言与冷热不变量（下行）继续有效。
- **语言与冷热不变量（沿 069 设计口径）**：zh ≥ 50%、en ≥ 20%、mixed ≥ 10%；冷门查询 ≥ 20%。
- **字段（069 表对齐）**：`case_key`（唯一）、`query`、`query_language`、`viewer_context`（匿名 / 非粉丝 fixture 用户 / 粉丝 / 作者）、`relevant_evidence`（(content_id, content_version, source_start, source_end) 列表）、`relevant_content_ids`（0..n，空集仅限无答案类）、`expected_citations`（必须被引用的证据子集）、`forbidden_content_ids`（必须不出现：隐藏/私密/未审核/封禁/陷阱干扰项）、`answer_rubric`、`classification`（primary_layer / query_form(exact|semantic) / visibility_context / ip_scope / answerability / 内容类型 / 冷热带）。

## 4. 标注规则 v1.0（已采纳审查，待用户确认后冻结）

1. **允许多答案**：relevant_content_ids 1..n 全标，不追唯一最优；**空集合仅允许无答案类**。
2. **答案主体 + 证据 span**：每条 relevant 至少绑定一个 (content_id, version, source_start, source_end)；仅顺带提及不纳入。多篇综合题：每篇含**不可替代的答案事实**才纳入。
3. **expected_citations 独立标注**：仅登记"回答必须引用"的证据子集，不自动等于 relevant_content_ids。
4. **精确类**：查询必须包含完整标题，或包含当前语料中唯一可识别目标的专有名词；专有名词不唯一时必须增加消歧条件。
5. **语义类**：不得直接复制完整标题或连续的核心标题短语；必要的专有名词、领域词可保留，但问题必须自然地同义改写并保持原意与约束不变。
6. **可见性类**：viewer_context 固定为匿名用户 + 指定非粉丝 fixture 用户（作者-粉丝关系随语料冻结）；**泄漏面 = 检索结果、引用、回答正文提及（标题/摘要/ID）、工具步骤事件**。隐藏内容若包含答案主体 → 归可见性类，不算无答案类。
7. **无答案类**：整个冻结语料中均不存在答案主体才可冻结；须登记 plausible 干扰项（貌似相关实则答不了的内容）进 forbidden/陷阱集合。
8. **复核形态**：四列表（问题 | 期望/禁止内容 | 出题理由 | 主分层）仅为人工审阅视图；机器真源为 069 表字段全集。用户逐条只判断"配对是否合理"。

## 5. 指标合同（统一定义，A-04 消融按此执行）

| 指标 | 定义 | 备注 |
| --- | --- | --- |
| HitRate@10 | 任一 relevant 进入 Top-10 的 case 占比 | 即 v0.9 误写的"Recall@10 命中"；主表口径，与 current-v1 历史可比 |
| Recall@K | Top-K 命中 relevant 数 / relevant 总数（`metrics.go` 现实现），K∈{5,10} | 多答案漏答可见；K=5 为 A-04 spec 兼容附加列 |
| MRR | 首个 relevant 排名倒数 | `metrics.go` 既有 |
| nDCG@10 | 二值相关度折扣增益（`NDCGAt10` 既有；@5 由 K 参数化） | |
| citation precision | 引用中属于 relevant/expected 的占比 | A-04 主口径 |
| no_answer_trap_hit@10 | 无答案 case 的 Top-10 命中登记干扰项即计阳 | **（v1.1 修订）仅作检索层诊断，strict 策略亦不据此判整条失败；硬失败发生在引用/回答声明层**。拒答率单独统计，不复用 Recall |
| visibility_leak | 四泄漏面命中 forbidden_content_ids 的 case 数 | 目标 0；含引用/正文/工具步骤，不止返回 ID。**（v1.1 增补）不存在/编造的作品无 corpus key，不进 forbidden_content_ids，写入 answer_rubric 禁止声明** |

## 6. 依赖与外部输入

- blocked_by A-03（#285，v4 嵌入落地后入库）；外部输入：**语料文本（ZCode 会话生成，无外部 key）**；DashScope key 已就绪（`OmniCraft/.env`，gitignored）。
- 语料文本未就绪 → #291 保持 OPEN 阻塞，禁止 mock（与 C-11 纪律一致）。

## 7. 验收

语料冻结报告（data/reports/：规模/分布/checksum）+ golden set 冻结件（069 导入 + checksum + 标注规则定稿）+ 注入验证记录（API 成功率、UI 子集截图、20 条精确标题检索冒烟全命中、worker lag=0）+ corpus_item_key→content_id 映射表。

## 附录 A：Tika 调研结论（Phase 2 备选，本轮不实现）

Apache Tika：JVM 容器化文档解析服务（tika-server），REST 接口（PUT 文件 → 纯文本 + 元数据），支持 PDF/Office 全家/OCR 等 1000+ 格式；Elasticsearch ingest-attachment 内嵌即 Tika。适配 Go 架构：一个 HTTP 客户端函数 + 解析结果转统一 chunk 模型，零 SDK、零 Python 依赖，约 1–2 天。服务器影响：常驻 256–512 MB（JVM），大 PDF 解析有峰值——对 3.6 GiB lean 主机是显著新增，故列 Phase 2（升配后常驻，或上传时短时拉起）。启用前提：附件上传成为产品功能。

## 附录 B：IP 名单（16 个，2026-09-02 冻结）

| # | IP | 题材/语境 | 语料形态倾向 |
| --- | --- | --- | --- |
| 1 | 原神 | 游戏·幻想 | 长篇/设定集主力，zh 为主 |
| 2 | 崩坏：星穹铁道 | 游戏·科幻 | 长篇 + CP 语义层 |
| 3 | 王者荣耀 | 游戏·历史人物 | 短中篇 + 讨论帖 |
| 4 | 西游记（孙悟空） | 古典·公版 | 设定集 + 长篇（公版题材版权最稳） |
| 5 | 哪吒/封神宇宙 | 国漫电影 | CP 语义层 + 短篇 |
| 6 | 全职高手 | 网文·电竞 | 中篇 + 讨论帖 |
| 7 | 诡秘之主 | 网文·西幻 | 设定集主力 + en 向潜力 |
| 8 | 魔道祖师 | 网文·仙侠 | 中长篇（Lofter 长热） |
| 9 | 天官赐福 | 网文·仙侠 | 中篇 + 短篇 |
| 10 | 罗小黑战记 | 国漫·治愈 | 短篇/无标题讨论帖主力 |
| 11 | 盗墓笔记 | 网文·悬疑 | 长篇 + 探险剧情向 |
| 12 | 哈利·波特 | 全球·奇幻 | en 向内容主力 + What-if 长篇 |
| 13 | 双城之战（LoL 系） | 全球·动画剧集 | en 向 + CP 语义层 |
| 14 | 火影忍者 | 全球·民工漫 | zh/en 双向中长篇 |
| 15 | 海贼王 | 全球·民工漫 | zh/en 双向冒险向 |
| 16 | 宝可梦 | 全球·游戏 | 轻松向短内容 + en 向 |

约束：全部内容保持**全年龄向**（日常/剧情/设定），不生成限制级内容；非商用演示口径不变。单 IP 目标 80–160 条，由 manifest 余量均衡分配。
