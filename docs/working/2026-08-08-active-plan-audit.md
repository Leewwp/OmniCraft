# OmniCraft 活计划与执行顺序审计

**创建日期**: 2026-08-08 · **预计失效日期**: 2026-10-08

> 创建日期：2026-08-08  
> 预计失效日期：2026-10-08  
> 审计方式：只读核对本地权威文档、当前代码/迁移真实状态与 GitHub Issues 原生元数据；未修改 issue、计划或代码。  
> GitHub 核对时点：2026-08-08（Asia/Shanghai）。

## 1. 审计范围与结论

一手来源：

- 任务源与流程：`AGENTS.md` 活计划注册表及执行车道（尤其是第 28–57、61–122 行）。
- 最高约束：`.specify/memory/constitution.md` Principle III/IV/IX/XII/XIV（第 59–77、156–171、227–236、263–269 行）。
- 设计输入：`docs/superpowers/specs/2026-08-07-omnicraft-web-experience-corrections-design.md`、`docs/superpowers/specs/2026-08-08-omnicraft-media-experience-design.md`、`docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md`。
- 执行计划：`docs/superpowers/plans/2026-06-30-omnicraft-community-source-linkage.md`、`docs/superpowers/plans/2026-06-30-omnicraft-community-collaboration-invites.md`、`docs/superpowers/plans/2026-07-16-omnicraft-web-agent-productization.md`。
- 最近裁决：`docs/working/2026-08-08-architecture-fix-backlog.md`。
- 当前真实数据面：`backend/migrations/007_content_attachments.sql`、`backend/internal/model/content.go`、`backend/internal/repository/content_repo.go`、`backend/internal/migration/fixture.go`。
- GitHub：[#1](https://github.com/Leewwp/OmniCraft/issues/1)、[#64](https://github.com/Leewwp/OmniCraft/issues/64)/[#65–#76](https://github.com/Leewwp/OmniCraft/issues/65)、[#80](https://github.com/Leewwp/OmniCraft/issues/80)/[#81–#90](https://github.com/Leewwp/OmniCraft/issues/81)、[#92](https://github.com/Leewwp/OmniCraft/issues/92)，包括 issue body、labels、comments、sub-issues 和原生 dependency summary。

**总体判断：两套新规格的用户故事拆分总体合理，组内依赖也基本正确；但当前还不是可靠的 AFK 执行队列。** 存在 2 个全局阻断级问题、6 个高优先级计划缺口和若干状态/文档漂移。尤其是：#92 未进注册表却阻断所有正常合并；宪法仍命令已废弃的 `task.json` 工作流；媒体集规格没有对“有序媒体”提供可持久化合同，也没有服务端权威约束。

## 2. 已确认合理的部分

1. **#64 拆票完整。** #64 原生 sub-issues 正好是 #65–#76，12 票均 OPEN、`ready-for-agent`、0 comments。组内原生依赖与票据 `Blocked by` 完全一致：65←∅，66←65，67←65，68←67，69←68，70←65，71←65，72←65/66，73←65，74←65/71，75←74，76←75。
2. **#80 的 10 票内部依赖正确。** #81–#90 均 OPEN、`ready-for-agent`、0 comments；原生依赖与正文一致：81/82/83←∅，84←83，85←83/84，86←85，87←84，88←85，89←85/88，90←88。
3. **风险拆分方向正确。** IP history `064` 和 favorites drop `065` 被分成独立 heavy 任务；#76 保留访问日志+可恢复备份人工门（Web 规格第 109–118、134–140 行）；#84 已加入 C7 非连续 migration fixture 验证门。
4. **迁移编号当前无占用冲突。** 仓库现有为 001–060、062；预留为 063 collaboration invites、064 IP history、065 favorites drop、066 source linkage、067 cover dimensions。Migration runner 按版本集合/升序计划，不把“最大已应用版本”当成唯一边界；因此先 067、后补 063–066 在 runner 层面可行。风险在 fixture 验证而不是 runner 排序，已由 #84/C7 识别。

## 3. 发现与必要调整

### [P0] F-01：#92 是全局合并前置，却不在活计划注册表

**证据**

- 架构 backlog 第 30–33 行明确记录：main 上 Ops alerting / SBOM / Security 三个 gate 全红，branch protection `project-gate` 阻断一切正常 PR 合并，已建 heavy issue #92；#79/#91 只能 admin-merge。
- [#92](https://github.com/Leewwp/OmniCraft/issues/92) 当前 OPEN、0 labels、0 dependencies。
- `AGENTS.md` 活计划只有优先级 1–4（第 28–34 行），没有 #92；同时规定注册表是任务唯一来源（第 61–65、241 行）。

**影响**：Agent 会按注册表领取 #64/#80，直到 PR 阶段才发现正常合并不可用；持续 admin-merge 会把发布门失效常态化。

**必要调整**：

1. 把 #92 登记为优先级 0、heavy、全局合并前置，并加 `ready-for-agent`（若已有人处理则加 assignee/进行中证据）。
2. 在 #92 关闭前，注册表应明示“可开发，不得常规合并”，不应将 admin-merge 当作默认路径。

### [P0] F-02：最高权威宪法仍强制已废弃的 `task.json` 流程

**证据**

- 宪法 Principle IX 第 156–171 行要求从 “Beta roadmap mode A / task.json mode B” 选任务、设 `passes: true`、将 `task.json` 与代码单一原子 commit。Principle XIV 第 267–269 行仍以 `task.json.ui_spec_ref` 定位视觉规格。
- `AGENTS.md` 第 49 行明确 `task.json` 模式已废弃；第 53–57 行允许 light 车道多个逻辑 commit；第 118–122 行要求勾选当前 plan/ticket。
- `AGENTS.md` 权威表规定宪法高于一切，因此不能简单用“AGENTS 更新”消解冲突。

**影响**：任何 agent 都会同时收到互斥的强制命令；当下注册表、light/heavy 车道和计划勾选在最高权威上没有合法基础。

**必要调整**：先做一次 constitution MINOR amendment，用活计划注册表 + GitHub tickets + light/heavy 车道替换 Principle IX/XIV 的旧语句，并按宪法自身第 385–386 行同步 plan/spec/tasks templates。在此修正前不应称当前队列为“无冲突权威流程”。

### [P1] F-03：媒体集需要顺序，但数据模型和 ticket 没有持久化顺序合同

**证据**

- 媒体规格第 22、37–39、70–75 行将“拖拽排序”、“第一张即封面”和“可顺序浏览的媒体序列”定为核心合同；[#83](https://github.com/Leewwp/OmniCraft/issues/83) 也要求拖拽排序。
- 当前 `content_attachments` 只有 `id/content_item_id/file_type/.../is_primary/created_at`，没有 `sort_order`/position（`backend/migrations/007_content_attachments.sql` 第 1–12 行；`backend/internal/model/content.go` 第 36–48 行）。
- `GetAttachments` 只做 `WHERE content_item_id = ?`，没有 `ORDER BY`（`backend/internal/repository/content_repo.go` 第 216–219 行）。
- #84/迁移 067 只规划 `content_items.cover_width/cover_height`，没有附件顺序列（媒体规格第 74 行、#84 acceptance criteria）。

**影响**：UI 中排好的顺序在发布后不是数据库保证的顺序；“首图即封面”和画廊翻页可在不同查询/数据库计划下漂移。

**必要调整**：补充设计决策和 #84（或独立 heavy 迁移票）：为 `content_attachments` 加 `sort_order`/position；为兼容不回填的旧行可允许 NULL，但新发布的媒体集必须提供非负、同内容内唯一的顺序（可用 partial unique index），repository 显式 `ORDER BY sort_order NULLS LAST, id`，发布 DTO 显式接收顺序，并以服务/handler 测试证明首项派生封面。如果产品要求发布后可重排，还需定义修改 API 和并发语义；若不支持，明确写入 out of scope。

### [P1] F-04：媒体集的数量/类型约束只在发布端，且数值与“配置优先”冲突

**证据**

- 媒体规格第 22、70–75 行规定 image=2–9 张纯图片、video=1–3 个纯视频、不允许混排。
- #83 的范围是“发布端媒体编排”，acceptance 只要求 UI 数量校验；#84 覆盖封面宽高派生，没有要求服务端拒绝混合类型、过少/过多或尺寸不完整的请求。
- 宪法 Principle IV 第 70–77 行规定所有 upload limits 必须从 `config.yaml` 读取，禁止硬编码；规格/票据没有新配置字段、public config 白名单或后端权威验证。

**影响**：直接 API 客户端可绕过纯净规则；前后端上限将漂移；实现者必须在“遵守已确认 ticket”和“遵守最高权威宪法”之间二选一。

**必要调整**：在 #83/#84 前补齐权威合同：

1. `config.yaml`/Go config 加 image/video gallery min/max（或经正式宪法修订明确这不属可调 upload limit）。
2. 前端需要的非敏感上限通过 public config 提供，不复制常量。
3. 后端在消耗 upload grants/写数据前权威验证数量、`content_type`↔`file_type` 一致性、宽高正整数和 poster 合同；加 handler 正常/错误路径测试。

### [P1] F-05：跨计划顺序只写在散落文档，GitHub frontier 仍会选出错票

**证据**

- backlog 第 35–41 行确认顺序是 `#64 T03/T04 → 媒体 T04 → T05...`；`AGENTS.md` 第 30–31 行也要求 #64 T03/T04 先合并。
- [#84](https://github.com/Leewwp/OmniCraft/issues/84) 原生 blocked-by 只有 #83，没有 #68；因此 #83 一关闭，GitHub 就会把 #84 判为无阻塞，并继而解锁 #85/#87 链。
- [#81](https://github.com/Leewwp/OmniCraft/issues/81) 一方面写 `None — can start immediately`，另一方面写与 #64 T02/T07 的排序/筛选共享面需串行；其中 T07 是 Reaction，排序应为 T08/#72。`AGENTS.md` 又称 #81 可与 #66 并行。
- Web 规格第 160–164 行要求先修正 ui-spec 再编码；媒体规格第 117–119 行同样要求 ui-spec 同步和精确文件预约。但 #82 目前无阻塞，而现行 `design/ui-spec.md` CollectionPicker 第 5535–5549 行仍要求成功后全局 Toast，与 #82 的弹窗内 notice 直接冲突。

**必要调整**：

1. 为 #84 加原生 blocked-by #68，让后续媒体链传递等待。
2. 裁定 #66/#81 的唯一顺序；按注册表优先级，建议 #66 → #81，并将 #81 的 `T07` 改为 `T08/#72`，明确 #81 不拥有共享 SortSelect 收口。
3. 媒体工作增 T00（先同步 ui-spec，并 blocked by #65），或把具体 ui-spec 章节和唯一 owner 分别补入 #81/#82/#85–#90；不能只靠“实现前读取”解决权威内容本身过时。
4. #81–#90 应加精确文件预约，履行媒体规格第 117 行已写明的发票条件。

### [P1] F-06：C2+C6 是必做代码前置，但没有 owner、ticket 或验收项

**证据**

- 架构 backlog 第 16、20、35–39 行要求 ContentDetail 家族收敛、双侧栏收敛和 shared overlay hook 先于 #64 T03/T04、媒体 T05/T09。还明确列了删 `ContentDetailClient`、类型去重、`IP: ` i18n、历史记录器收敛等子项。
- #65 只是重叠/权威矩阵；#67 是 throwaway 转场原型；#68 是正式转场接线。三票都没有 C2+C6 的完整验收。
- `AGENTS.md` 第 30 行从 T01/T02/T06 直接跳到 T03/T04，没有 backlog 第 38 行单列的 C2+C6 批。

**影响**：执行队列可以“全部照票完成”但仍遗漏已裁决的架构前置，或在 #68 临时扩大范围。

**必要调整**：首选新建一张 light 前置票，用 backlog 子项作验收，并让 #67/#68（以及需要时 #85/#89）原生 blocked-by 该票。如坚持不拆票，则必须将完整验收并入一张指定票，并在注册表显式写出该前置提交。

### [P1] F-07：媒体 T10 与 source-linkage Task 7 重复拥有“关联内容”与同一个 ContentDetail 插入点

**证据**

- 媒体规格第 58、84 行和 [#90](https://github.com/Leewwp/OmniCraft/issues/90) 要求正文+评论后同时展示“相似推荐 + 关联原创/二创”。
- source-linkage Task 7 第 480–564 行明确要求 **Create** `RelatedFanworks.tsx`、修改 `ContentDetail.tsx`，并在正文后/评论前接入 related row。
- 当前注册表顺序是媒体 T05–T10 全部完成后才执行 source-linkage（`AGENTS.md` 第 30–32 行）；#90 没有指定组件/插入点唯一 owner。

**影响**：#90 若完整实现验收，必然先建一套关联行或容器；source-linkage 后续又按计划新建/接入另一套，导致重复、返工或文件冲突。

**必要调整**：将执行序调为“媒体 T05–T09 → source-linkage → 媒体 T10”，让 Task 7 先交付 `RelatedFanworks`和来源 API，#90 再拥有统一 related container、相似推荐和到底提示。如保留当前顺序，则必须重写 #90/Task 7 的边界：#90 只建容器+相似推荐，Task 7 只插入 linked row，并共享明确的 props/API 合同。

### [P1] F-08：两个 heavy cutover 票还不是可直接执行的实施计划

**证据**

- [#73](https://github.com/Leewwp/OmniCraft/issues/73) 只描述“记录、最近列表和匿名历史合并合同”，但 Web 规格第 100–102、131 行也没有给出 API path/method、merge request schema、时间戳可信边界、batch cap、无效条目的原子/部分成功语义。
- [#76](https://github.com/Leewwp/OmniCraft/issues/76) 是 heavy 生产 cutover，但只给验收目标；没有指定证据文件路径、访问日志查询的旧 endpoint 全集、备份可恢复判定方式、部署/迁移具体入口和中途失败时的 forward-fix/stop boundary。
- `AGENTS.md` 第 55–57、104 行对 heavy 任务要求一任务一分支/提交、先红测试、两阶段审查；当前没有 #73/#76 对应的详细 implementation plan。

**影响**：不同 agent 会自行发明 API 或生产命令；对一次性 cutover 来说，“验收完整”不等于“过程可安全重现”。

**必要调整**：分别补 `docs/superpowers/plans/` implementation plan（或将同等粒度的 steps 放入 issue）。#73 先锁定 HTTP 合同与边界；#76 先锁定 preflight→deploy→pre-drop smoke→migration→post-smoke→evidence 的逐步命令、停止条件和证据存放位置。

### [P2] F-09：Issue 状态/父子结构存在失真

1. **#1 已完成却仍可领取。** [#1](https://github.com/Leewwp/OmniCraft/issues/1) 仍 OPEN + `ready-for-agent`，但 `docs/archive/plans/2026-07-18-omnicraft-ui-polish-hardening.md` 第 3、15–19 行明确它已完成 U-12 并归档。#65 应将 #1 当历史边界输入，不是当前可重复领取的工作。**调整：**带归档证据关闭 #1，并修正 #65 表述。
2. **#80 没有原生子 issue。** #80 的 `sub_issues_summary.total=0`；#81–#90 只在正文写 `Parent - #80`。对照 #64 的 12 个原生子票，这会使父票进度/导航失真。**调整：**把 #81–#90 挂为 #80 原生 sub-issues。
3. **#76 未满足 AFK-ready 条件。** 仓库 `docs/agents/triage-labels.md` 把 `ready-for-agent` 定义为“Fully specified, ready for an AFK agent”；#76 同时依赖未完成的 #75 和云端人工门。**调整：**在人工证据就绪前移除 `ready-for-agent`，改 `ready-for-human`/`needs-info` 或拆出 manual-gate issue，满足后再切回。

### [P2] F-10：本地 spec、GitHub spec 与历史 community spec 存在状态漂移

1. **#64 正文过时。** GitHub #64 Further Notes 仍写“Ops-08 and real Provider validation remain blocked”；本地 Web 规格第 163 行和 `AGENTS.md` 第 45 行已明确 Ops-08 Step 5 完成，只有 Web Agent Task 6 缺真实 Provider 凭证。**调整：**同步 #64 body。#80 与本地 spec 无实质差异。
2. **favorites 回滚窗口决策未标注被取代。** 2026-06-29 community spec 第 1166、1254 行仍要求旧 favorites 表保留 30 天；2026-08-07 Web 规格第 109–116 行改为访问日志+可恢复备份后 forward-only drop，且允许丢弃测试/演示数据。后者同目录且更新，应为当前裁决，但旧 spec 没有 superseded 注记。**调整：**在 community spec DEC-015/附录 F 加被 2026-08-07 规格取代的注记，不要删除历史决策。

### [P2] F-11：注册表“余项”和社区计划的执行来源文字不一致

**证据**

- 实际计数：source-linkage 文件有 63 个 `- [ ]`，注册表写 64；collaboration-invites 有 61，注册表写 62；Web Agent 文件有 14 个未勾选框（Task 6 六步 + Plan Self-Check 八项），注册表写 6。这表明“余项”有时数全部 checkbox，有时只数当前 task steps。
- source-linkage 第 15 行和 collaboration-invites 第 15 行仍写“只有用户显式点名本计划才可执行”；`AGENTS.md` 第 28–49、61–65 行则把它们登记为可按优先级自动选取的活计划。
- 两个社区 plan 第 3 行还强制使用旧 `superpowers:subagent-driven-development`/`executing-plans` 子技能名，与当前仓库统一的注册表执行流程没有对齐说明。

**必要调整**：先定义“余项”是未完成执行 step、未完成 task 还是所有 checkbox，再重算每行；删除两个社区 plan 的“须用户显式点名”旧门槛，改为遵循 `AGENTS.md` 注册表；将子技能要求改成当前可用的通用执行规则，或降级为非强制建议。

### [P3] F-12：低风险文字精度问题

- #73 的 064 编号理由只提 060 已占用，漏 062 已存在/063 已预留；#76 的 065 理由漏 063。文件名当前正确，但应补全理由。
- #84 引用 `fixture.go:116` 为连续性断言；当前真正的 `len(expectedSources) != baseline` 在 `backend/internal/migration/fixture.go` 第 118–119 行。建议引用函数/语义而非不稳定行号。
- source-linkage Task 2 的测试表里 `IP-only fanwork succeeds` 重复一次（第 180 行附近）；Task 3 Step 6 只跑 `TestCreateContentRoute`（第 281–288 行），没有立即重跑该 Task 前面新增的 source immutable update 测试。全量 gate 最终会捕获，但 focused red→green 闭环应修正。
- collaboration-invites Plan Self-Check 第 672 行写“Migration number is 063, after source-linkage 066”。这里的 after 指执行顺序而非数字顺序，应改为“executed after source-linkage although its reserved version is 063”。

## 4. 建议的修正后执行图

以下是在不改变已确认产品范围下，最小化文件冲突和返工的顺序：

1. **全局治理门**：修宪法旧工作流；优先修 #92；关闭过时 #1；修正 #64 body/#80 sub-issues/状态 labels。
2. **视觉权威门**：#65 先完成 Web + 媒体的 ui-spec/重叠矩阵（或紧接独立媒体 T00）。
3. **无阻塞高收益批**：#66、#70；#81 改为在 #66 后；#82 在 CollectionPicker 权威修正后；#83 在媒体顺序/配置/服务端合同补齐后开始。
4. **共享浮层前置**：完成 C2+C6 独立票，再 #67→#68。
5. **媒体核心**：#83→#84（增 blocked-by #68，同时落 C7 和有序媒体 schema）→#85；然后 #86/#87/#88，#89 等 #88。
6. **关联内容交接**：source-linkage（066，C3 随行）→#90；或在保留旧顺序时事先拆清 #90/Task 7 的 owner。
7. **Web 余下 light/heavy 链**：#71→#69→#72→#73（064）→#74→#75→#76（065 + manual gate）。#73/#76 先补 heavy implementation plan。
8. **后续共享面**：collaboration-invites（063）→ Web Agent Task 6；Task 6 仍以真实 `agent.llm_api_key` 为解阻条件，仓库默认保持关闭。

> 说明：#71/#69/#72 彼此主要是共享文件串行而非业务硬依赖，可根据精确文件预约调整内部顺序；不得与 source-linkage/#90 同时修改 `ContentDetail`/翻译面。

## 5. 建议的最小文档/Tracker 改动集

1. Constitution：修正 Principle IX/XIV 及三份 template。
2. `AGENTS.md`：加 #92 priority 0，显式加 C2+C6，修正 #84/#90/source-linkage 交错顺序，定义“余项”计数口径。
3. Media spec/tickets：补 attachment ordering、config/server validation、ui-spec owner/精确文件预约；给 #84 加 #68 dependency，裁定 #66/#81 顺序。
4. 新 C2+C6 light ticket：用 backlog 已裁决内容作验收和下游依赖。
5. Heavy plans：补 #73 IP history API/merge 计划和 #76 cutover runbook-style 实施计划。
6. GitHub 治理：#80 挂 10 个 sub-issues；关 #1；同步 #64；修正 #76 label；给 #92 与新前置票加正确 label/dependencies。
7. 历史 spec：对 favorites 30-day 决策加 superseded 注记；修正 community migration 编号说明中“按最大编号递增”与已确认补缺策略的冲突。

## 6. 不应调整的部分

- 不补写 061，不改历史 migration/checksum fixture；这与 backlog C7 裁决一致。
- 不把 IP history 和 favorites drop 合并成一个迁移/任务。
- 不为弥补计划缺口而恢复 Tauri/Desktop 范围或改变关闭的 feature flags。
- 不在缺少真实 Provider/云端证据时用 mock 代替 Web Agent Task 6/#76 的真实门。
