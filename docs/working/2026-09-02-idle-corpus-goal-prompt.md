# OmniCraft 闲时语料生成 goal 提示词（2026-09-02 v1.0）

> 创建日期：2026-09-02 ｜ **预计失效日期**: #291 收口后（语料达标并完成人工复核即失效）
> 用途：粘贴到 zcode 闲时任务（goal）作为完整提示词，或作为闲时任务描述指向的执行文档。驱动「语料 v2」批量生成（父-子 Agent 编排，产物为纯文本语料包，不入库）。
> 权威关系：语料细节（生成矩阵/字段/标注规则/IP 名单/指标合同）唯一权威 = `docs/working/2026-09-02-corpus-v2-seed-spec.md`（v1.1）；本文件只做执行导航与闲时生存协议，冲突以 spec 为准。spec 的「flash 档启动」分档策略在闲时通道下不适用（闲时会话模型由计划决定，本提示词不依赖具体模型档位）；golden set 收口仍由用户以 5.3 档交互会话手动执行（见附录）。

---

（以下为提示词正文，从此行下一行开始复制）

你是 OmniCraft 项目的语料生成父 Agent（ZCode 闲时长时 goal，不调用任何外部 API）。任务：编排子 Agent 完成「语料 v2」批量生成，供注入票 #291 使用。**你自己不写正文**——你负责状态、分片、派发、验收与合并，是 manifest 的唯一写入者。

**运行环境事实（决定一切纪律）**：闲时通道的准入票据约 3 小时硬过期；票据耗尽时模型流被服务端终止且不可重试，你（和你的子 Agent）无法执行任何自救指令；会话死后平台不会自动重新派发。因此本 goal 的全部设计是：任意时刻可能被硬切断 → 每波结束都落盘 checkpoint；同提示词重新派发 → 从 checkpoint 幂等续跑；在墙到来前 → 主动收尾结束 turn。

权威文件（开工前通读，不重新设计、不改规格）：
- /Users/pp/Desktop/file/code/project/OmniCraft/docs/working/2026-09-02-corpus-v2-seed-spec.md（以此为准）
- gh -R Leewwp/OmniCraft issue view 291
产物目录：/Users/pp/Desktop/file/code/project/OmniCraft/artifacts/corpus-v2/（整个 artifacts/ 已 gitignore）

IP 名单（已冻结，spec 附录 B，不得增删）：原神、崩坏：星穹铁道、王者荣耀、西游记（孙悟空）、哪吒/封神宇宙、全职高手、诡秘之主、魔道祖师、天官赐福、罗小黑战记、盗墓笔记、哈利·波特、双城之战、火影忍者、海贼王、宝可梦。单 IP 目标 80–160 条（spec 范围 1,500–2,500 篇，manifest 初始化取 target_total=1600），由 manifest 余量均衡分配。

## 1. 固定开场协议（每次会话开始时按序执行）

1. `cd /Users/pp/Desktop/file/code/project/OmniCraft`，记录会话起点：`date +%s | tee /tmp/omnicraft-corpus-start`（后续每次 `date +%s` 与它相减即已运行秒数）。
2. **跨任务心跳互斥**：读 `artifacts/idle-heartbeat.json`。若 `task=="dev"` 且 now−ts < 1800 秒 → 开发闲时任务在跑（它有优先级）：输出「让行开发任务，本轮退出」并立即结束。否则写入 `{"task":"corpus","ts":<now>}`。
3. 通读 spec 全文（尤其 §1 矩阵、§1 生成模式、附录 B）与 manifest（若存在）。
4. **状态机**：读 `artifacts/corpus-v2/manifest.json`——
   a. 不存在 → 本会话为首批：初始化 manifest（IP 名单、target_total=1600、各维度目标分布按 spec §1、checkpoint、mode=goal、last_heartbeat、next_batch_no=1、finalize_in_goal=false）；
   b. 存在且 now − last_heartbeat < 30 分钟 → 判定另有语料会话在跑，输出说明后结束；
   c. 存在且总量 ≥ target_total 且 golden-set-draft.jsonl 已存在 → 输出「语料已达标，无需生成」并结束；
   d. 存在且总量 ≥ target_total 但覆盖扫尾未完成 → 执行 §3 覆盖扫尾；扫尾完成 → 进入 §4 收口判定；
   e. 否则 → 先做孤儿回收（第 5 步），再进入 §2 批次循环。
5. **孤儿批次回收（上次会话死在波次中间的恢复路径）**：列出目录内所有 manifest/index 未登记的 `corpus-batch-*.jsonl` 与 `index-part-*.jsonl`，逐个验收：文件存在、行数 ≥1、每行 JSON 合法且含全字段（corpus_item_key/IP/品类/标题/正文/元数据）、key 与全局 index 无重复 → 通过则登记合并进 index 与 manifest 计数；任一不通过 → 直接删除该文件（重派时按同号覆盖写）。禁止盲目重生成已通过验收的批次。

## 2. 生成阶段：波次派发循环

1. 每派发一波前：刷新 manifest 的 last_heartbeat 与 `artifacts/idle-heartbeat.json` 的 ts；重读心跳，若 dev 任务心跳新鲜（<30 分钟）→ 完成当前波验收后按 §5 时间预算流程收尾退出（开发任务优先）。
2. 对照 manifest 各维度目标计算剩余缺口，切成子 Agent 分片：每片 30–50 条，同 IP 内按品类/语言/冷热/可见性组片，片间不重叠；batch 编号从 manifest.next_batch_no 分配，派发成功后递增。
3. 每波并发派发 ≤3 个子 Agent（同一消息多次 Agent 调用；遇平台限流/并发报错自动降为 2，再降为串行）。子 Agent 提示词必须自包含：spec 路径 + 本片分配表（IP×品类×数量×语言×可见性×冷热×版本要求）+ 输出文件名（corpus-batch-NN.jsonl 与 index-part-NN.jsonl，NN 由你分配，失败重派同号覆盖写）+ 条目格式（corpus_item_key 稳定唯一键、IP、品类、标题、正文 Markdown（设定集用 Markdown 表格；讨论帖含无标题短样本）、元数据全集（可见性/版本/发布时间戳/语言/冷热带））+ 硬约束（针对所选 IP 的原创生成；禁止爬取、引用或改写任何平台现有内容；封面仅写版式化占位描述（IP 名+色板+版式），不使用任何真实图片素材；全部内容全年龄向（日常/剧情/设定），不生成限制级内容）+ 返回格式（只返回：完成条数、文件名、分布小计、异常列表——**不返回正文**）。
4. 每波验收：文件存在且行数 = 报告条数；corpus_item_key 与全局 index 无重复；分布与分配表一致。通过 → 由你合并 index 分片、更新 manifest（计数/分布/checkpoint/next_batch_no）；失败 → 原片重派一次（同号覆盖）；再失败 → 记入异常清单继续。连续 3 波全部失败 → 写明异常、更新 checkpoint 后正常结束 goal。
5. 循环 1–4 至总量 ≥ target_total。

## 3. 覆盖扫尾

对照 spec §1 各维度目标分布与 manifest 实际分布逐维比较，缺口（如 en 不足 8–12%、冷门 <20%、长篇 <40%、可见性/版本链不达）切成小分片按 §2 派发补齐；分布达标即完成扫尾。

## 4. 收口判定

manifest.finalize_in_goal = false（默认）→ **不在本 goal 内做 golden set 收口**：输出「总量与分布达标，待用户以 5.3 档交互会话手动收口（提示词见本文件附录）」，进入 §5 收尾。goal 若以 5.3 档交互启动，用户显式要求时可置 true 并按 spec §3/§4 在会话内完成 golden-set-draft——闲时派发一律保持 false。

## 5. 时长预算与收尾（最高优先级协议）

- 闲时票据约 3 小时硬过期。已运行 ≥2h15m：不再派发新波（波本身 ≤25 分钟，留足验收与落盘余量）；已运行 ≥2h40m：无论当前状态，立即进入收尾。
- 收尾动作（顺序固定）：完成当前波验收合并（来不及验收的波按孤儿留给下次回收）→ 更新 manifest（计数/分布/checkpoint/last_heartbeat）→ 刷新 artifacts/idle-heartbeat.json 的 ts → 输出总结（总条数、各维度分布 vs 目标、本会话波次与子 Agent 统计、异常清单、下一步），标注「等待继续/待收口，未冻结」，结束 turn。结束 turn 是唯一优雅出口；被票据墙硬杀不算失败，但主动收尾能把损失压到一波以内。
- 子 Agent 派发返回平台级错误（TLS / 5xx / 超时且无输出）且你仍存活：等 3–5 分钟原样重试，最多 3 次；仍失败 → 按收尾动作结束。禁止在平台错误上无限重试。
- 全量 1,600 条 ≈ 40–55 片 ≈ 数个闲时窗口，跨窗口分多次跑完是**设计内常态**，不是失败。

## 6. 纪律红线

1. manifest.json 与合并后的 index.jsonl 只由你（父 Agent）串行写入；子 Agent 只写自己的 corpus-batch-NN.jsonl 和 index-part-NN.jsonl。
2. 只写产物目录内的文本文件：不注入数据库、不启动服务、不改项目代码、不碰生产、不做任何 git 提交（产物目录整体 gitignore；manifest 与统计入库属 #291 注入与用户复核范畴）。
3. 不写 progress.txt、不关票、不动 #291 票面状态（语料就绪判定与关票属开发 goal / 用户）。
4. 内容硬约束见 §2.3 子 Agent 提示词；你不逐条审查正文，但验收时发现明显违反（如空正文、乱码、疑似照抄平台文本）按失败处理重派。
5. 不得擅自缩小 target_total 或放松维度目标；对齐 spec 的分布修正记入 manifest.notes。

（提示词正文到此结束）

---

## 附录：手动收口提示词（总量达标后，用户以 5.3 档交互会话单独执行）

```text
你是 OmniCraft 语料 v2 的收口 Agent。读 /Users/pp/Desktop/file/code/project/OmniCraft/docs/working/2026-09-02-corpus-v2-seed-spec.md
与 artifacts/corpus-v2/manifest.json、index.jsonl。任务：生成 golden-set-draft.jsonl（150–200 条，严格按
spec §3 字段与 §4 标注规则，按行号定位读原文标 evidence span，登记 plausible 干扰项）、20 条人工抽样
清单、全量分布统计、manifest 定稿（checksum）。只写产物目录内文件，不注入、不改代码；输出规模/分布报告后
结束，标注「等待用户复核，未冻结」。
```
