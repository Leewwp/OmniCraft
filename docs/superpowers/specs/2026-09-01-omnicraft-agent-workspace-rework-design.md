# OmniCraft Agent 工作台重造（Agent Workspace Rework / SP-13）

> **创建日期**: 2026-09-01
> **状态**: 已确认（2026-09-01 grill-with-docs 会话 R1~R4 四轮 15 项决策，用户逐条拍板；tickets SP-13/A-01~A-07 已登记，**全部执行冻结，等待用户全面审查后下达执行顺序**）
> **输入依据**: 用户原始需求（UI 全面模仿 DeepSeek 网页端 + Agent 功能企业级最小实现）× 代码级摸底（Explore 子代理：Agent 前后端全链路、检索三套并存、护栏与可观测现状）× 开源调研（Onyx/Dify/RAGFlow/LobeChat/Open WebUI 盘点、Go 框架评估、MiniMax 官方文档查证、GitHub Copilot 模式提炼、DashScope/SiliconFlow 嵌入与 rerank 通道查证）
> **权威关系**: 本 spec 是设计输入；Agent 契约权威为 `docs/specs/web-agent-v0.4-mvp.md`（SSE v2 变更落地时由 A-02 同步修订）；不可妥协红线为 `.specify/memory/constitution.md` §XIII（SSE-only、工具白名单、DB 配置优先、禁自主 ReAct 长链路）；视觉 token 权威 `design/design-system.md`（SP-12 定稿后）。本 spec 与 SP-12（UI 精细化）共享工作台页面文件，执行顺序由用户整合裁决。
> **治理记录**: 本批次 = 活计划注册表**第二个第一阶段例外**（SP-12 是第一个）；将 Phase 2 (#208) 中「Session History 独立模块、SSE 合同变更」与 rag-deepening spec（2026-08-11）的 Future 项「查询改写、rerank」按**评测门控**提前激活。目标窗口：约两周（面试准备期）。

---

## Problem Statement

用户判定当前 Agent「不符合企业级标准、更像练手项目」。代码摸底核实：**问题不在架构缺失，而在产品化打磨**——链路已具备工具调用循环（4 个只读白名单工具、≤8 次/轮）、typed SSE 7 事件契约、引用服务端复验（content_version+chunk_key+excerpt 逐项比对，无依据拒答）、Redis 限流 fail-closed、OTel GenAI span、评估夹具。真正的缺口有六项：

1. **伪流式**：chat 回答的 delta 在工具循环内缓冲、最后一次性发出（仅 usage-guide 是真流式），最伤「像真实产品」的观感。
2. **会话模型残缺**：每轮新建会话并重复落库历史快照（无 conversation_id 续写）、无标题（前端显示「未命名 #id」）、停止/失败即删会话、上下文仅客户端截尾 10 条。
3. **渲染与操作缺口**：回答是纯文本气泡（项目已有 react-markdown 但 Agent 未接）、无复制、无思考过程展示（`<think>` 被服务端剥离丢弃）。
4. **引用只有列表卡片**，无行内锚定，推荐答案可信度观感不足。
5. **chat 输入/输出不过内容审核**（Green 只覆盖 publish/compliance 路径）——内容安全体系的唯一旁路，合规硬伤 + 存储型违规内容风险（违规输入原样落库 agent_messages 并在历史反复展示）。
6. **检索默认单路 pgvector**：hybrid（RRF k=60）已实现但默认关闭；无查询扩展、无 rerank——口语化兴趣输入与站内标签存在语义鸿沟，粗排后无意图级精排。

## Solution

双线收口（产品 + 架构，同一批工作量高度重合）：

- **产品线**：DeepSeek 交互范式 1:1 落地——完整生成形态（思考折叠区流式展开→完成折叠 + 工具步骤区 + 逐字正文真流式）、markdown 渲染、行内引用锚定、会话侧边栏三交互（选中态/悬停 ⋯ 菜单：重命名/置顶/删除）、输入框自动增高；视觉 token 映射 OmniCraft 设计语言（不推翻 SP-12 全站裁决）。
- **架构线**：会话模型重构（续写 + 服务端上下文组装 + 自动标题）、SSE 契约 v2（真流式 + 思考/工具步骤事件）、检索管线企业级升级（DashScope v4 嵌入换代 + hybrid on Postgres + 查询扩展 + qwen3-rerank 精排，**三开关全部评测门控、消融数据定默认值**）、护栏补口（输入前置 Green + 输出事后异步审核）。
- **选型结论**（开源调研）：无任何开源项目可直接引用/嵌入（盘点对象无一 Go 后端，Dify/LobeChat/Open WebUI 许可证另有附加条款）；模式参考系 = **Onyx**（企业级检索助手形态）+ **GitHub Copilot**（工具注册表/知识库引用/确认卡模式）+ RAGFlow（引用细节）；**零新框架、零新 SDK**——现有 `pkg/llm` 已具备 MiniMax 流式 + function calling + think 处理，唯一新增客户端是 DashScope 原生 rerank 小客户端。

## 决策记录（R1~R4，15 项）

| 轮/项 | 决策 |
|---|---|
| R1-Q1 | 交互与布局 1:1 复刻 DeepSeek；**视觉 token 映射 OmniCraft 设计语言**（indigo 主色、操作控件 8px 矩形 + 三档高度、#f5f5f5/#010409 画布、150ms 动效） |
| R1-Q2 | **保留全局导航**，Agent 区在其下模仿 DeepSeek；**反冗余原则**：全局导航已有的跳转/功能，Agent 工作台不再重复设置 |
| R1-Q3 | 新立治理方案（本 spec + SP-13 票）；正式开发前用户全面审查，再定任务与顺序 |
| R1-Q4 | 面试叙事 = 产品 + 架构双线 |
| R1-Q5 | 明暗双主题跟随全站；桌面优先；窄屏基础适配（侧栏可收、布局不破） |
| R2-Q6 | 生成体验目标 = **完整 DeepSeek 形态**：思考折叠区 + 工具步骤 + 逐字正文（真流式是硬底线） |
| R2-Q7 | `<think>` **保留并流式转发**（生成中展开、完成后折叠）；think 仅展示层，不参与引用复验、不作为工具结果 |
| R2-Q8 | 会话模型整套：`conversation_id` 续写、服务端按 token 预算组装上下文、自动标题（首轮后异步）+ 手动重命名 + 置顶（`pinned_at`）+ 删除（既有）；停止/失败**保留**半成品会话；延后跨会话全文搜索 |
| R2-Q9 | 行内引用锚定：句末 `[1][2]` 角标 = 展示层映射到服务端复验后的引用卡片；**复验语义零改动** |
| R2-Q10 | 补口护栏：chat 输入**前置** Green 文本审核（A4 分环境语义，dev fail-open）+ 输出**事后异步**审核（落库 assistant 消息，违规标记/隐藏，不阻塞流式） |
| R3-Q11 | 嵌入换代：**DashScope text-embedding-v4，dimensions=1536**（零 DDL，仅重嵌入值）；SiliconFlow Qwen3-Embedding-8B 为 fallback 配置；bge-m3 否决（固定 1024 + dimensions 参数 400 坑）；切换评测门控；qwen3.7-text-embedding 列 Phase 2 |
| R3-Q12 | **hybrid on Postgres**：启用既有 RRF 管线（k=60），词法路走 pg_jieba tsvector，**不依赖 OpenSearch**（零新基础设施）；启用前 golden set 对比 |
| R3-Q13 | 本批边界：聊天工作台 + 搜索页 SearchAgentInput 统一改造；发布流辅助（upload-assist/compliance）与 ContentDetail 的 usage-guide **不动** |
| R4-Q14 | 查询扩展 + rerank 纳入（修正形态，见下）；其余延后清单确认 |
| R4-Q14' | GREEN AccessKey 已恢复（2026-09-01 探测全绿），不再是部署阻塞 |

## Implementation Decisions

### 1. 布局与视觉（A-06/A-07）

- 全局导航保留；其下 DeepSeek 同构布局：左侧会话历史栏 + 主对话区；工作台内不重复全局导航功能（引用卡片打开 ContentDetailOverlay 属内容导航，不违反反冗余）。
- 全部视觉 token 取自 design-system（SP-12 U-01/U-02 定稿后消费）：主色 indigo、操作控件 8px 矩形 + 高度三档、亮 #f5f5f5 画布/暗 #010409、150ms 动效；明暗双主题。
- 输入框：自动增高（约至 8 行转内部滚动）、Enter 发送 / Shift+Enter 换行、流式中发送按钮切换为停止（既有交互保留）。
- 侧边栏：当前会话选中态高亮；悬停 ⋯ 菜单 = 重命名 / 置顶 / 删除；置顶分组置顶；桌面折叠持久化 + 移动端抽屉沿用现有模式。
- 空态 = 居中欢迎 + 推荐向建议 chips（替代/升级现有硬编码 quick prompts）。
- 搜索页 SearchAgentInput 与新工作台交互统一（Q13=B）；「顶部搜索栏保持关键词职责」边界不变。

### 2. 会话模型（A-01，heavy：含迁移）

- 迁移：`agent_conversations` 新增 `title`（可空）与 `pinned_at`（可空 TIMESTAMPTZ）；存量会话无 title 时维持「未命名」显示，不回填。
- API：`POST /agent/chat/stream` 请求体改为 `{conversation_id?, message}`（首次省略则服务端建会话并返回 id）；客户端不再上传整段历史。
- 上下文组装移到服务端：system + 会话历史按 token 预算装填（工具结果按需截尾）；替换客户端截尾 10 条。
- 标题：首轮问答完成后异步生成摘要标题（M3 短调用，失败回退首条消息截断显示）；`PATCH /agent/conversations/:id` 支持 rename 与 pin/unpin。
- 列表排序：置顶组按 `pinned_at` desc，其余按 `updated_at` desc。
- 停止/失败：保留会话与已流出的部分内容（不再删除本轮会话）；删除沿用幂等 owner-scoped 既有实现。

### 3. SSE 契约 v2（A-02）

- chat 从伪流式改**真流式**：LLM delta 逐段转发（含 think delta）。
- 事件保持 typed 风格，v2 覆盖：思考增量、工具步骤（工具名+参数摘要+命中数+耗时，供过程折叠区）、正文增量、citations、done（含 message_id/usage/会话 id）、error（统一错误码）。具体事件命名在实现时定稿并同步 `docs/specs/web-agent-v0.4-mvp.md` 与前端 `lib/agent.ts` normalizer。
- think 仅展示层：不参与引用复验、不作为工具结果；落库策略实现时定（建议落库并标记 phase，便于历史回放与审计）。
- SSE-only 红线不变（不引入 WebSocket）；断线续传（Last-Event-ID）明确延后——刷新/断线后由前端从 DB 重载会话（A-01 会话模型使其无损）。

### 4. 生成体验（A-06）

- 三层：思考折叠区（流式展开→完成自动折叠）+ 工具步骤区（可展开，展示检索词扩展/命中数等）+ 逐字正文。
- markdown 渲染接入（react-markdown，代码块高亮 + 复制）；外链安全处理。
- 行内引用锚定：system prompt 指示模型句末标 `[1][2]`；前端把角标渲染为可点击（高亮/滚动到对应引用卡片）；服务端复验语义零改动，锚点纯展示层映射。
- 消息操作：复制（新增）、重新生成（既有）、停止（既有）；编辑重发延后。

### 5. 检索管线（A-03）

- **embedding 换代**：DashScope text-embedding-v4（OpenAI 兼容 compatible-mode 端点，走既有 `openai_compat` provider 加 base_url/维度配置），`dimensions=1536` → 零 DDL、索引与双世代投影（071）结构不变，仅全量重嵌入值（复用 admin rag rebuild；v4 批量上限 10 条/33K token，重建任务按批）。fallback 配置 = SiliconFlow `Qwen/Qwen3-Embedding-8B`（同支持 1536）；embo-01 保留为可回退配置。
- **hybrid on Postgres**：`rag_hybrid_enabled` 启用既有 RRF（k=60）管线；词法路必须消费 pg_jieba tsvector（041/042 现役索引）——若现有 Postgres BM25 兜底适配器未走 tsvector 排序，A-03 内升级该适配器；不启用 OpenSearch。`search_content` 工具的检索实现切换到 hybrid 管线（工具契约不变）。
- **查询扩展**（`rag_query_expansion_enabled` 门控）：M3 短调用（cap 输出 token）把口语化输入扩展为 3-5 个同义/相关检索词；原始 query + 扩展词**合并为一次 v4 批量嵌入调用**（≤10 条上限内）；扩展词同时喂词法路与向量路；扩展词在工具步骤区展示（过程可见、可演示）。
- **rerank**（`rag_rerank_enabled` 门控）：RRF 融合 top-20 → **DashScope qwen3-rerank**（¥0.5/1M token，原生格式小客户端；注意 gte-rerank 已于 2026-05-30 下线、新版返回 `results` 字段解析）→ top-10 进工具结果。fallback = SiliconFlow `bge-reranker-v2-m3`（免费、限速）；失败降级回 RRF 序 + degraded 标记（沿用现有降级模式）。否决项：Cohere（国内服务器不可达 + 外币厂商）、自托管 BGE-reranker（lean 服务器新基础设施，列 Phase 2）、M3 自打分（一致性与延迟差，不作正式路径）。
- 降级链整体维持：向量不可用→关键词检索，degraded=true。
- 新增外部凭证：DashScope API Key（百炼；embedding+rerank 同 key，与 OSS/Green 同一阿里云账号体系）；SiliconFlow key 可选（fallback）。

### 6. 护栏（A-05）

- 输入前置：chat 用户输入过 Green 文本审核（复用 TextModerationPlus + 既有 A4 分环境语义，本地 dev fail-open）；命中即拒绝并返回统一错误码。
- 输出事后：assistant 消息落库后异步过审，违规则标记 + 前端隐藏/提示；不阻塞流式（业界对流式的标准做法）。
- 既有护栏不动：限流（分钟 5/日 50 fail-closed）、可见性防探测、结构性注入防护、`<think>` 剥离语义（改为转发后仍不进引用复验）、no_evidence 拒答。

### 7. 评测消融（A-04）

- golden set（069 表）扩充推荐场景 30-50 条：口语化兴趣描述 → 期望命中实体 id 集合。
- 四配置消融：① embo-01 单路基线 → ② hybrid + v4 → ③ ②+查询扩展 → ④ ③+rerank。
- 指标：Recall@5 / nDCG@5 / citation precision。
- 消融脚本接入 `verify-project.sh`（mocked 模式跑 fixture，真实模式可选）；**数据决定三个开关的默认值**——某项无增益则默认关并记录消融结论（同样是可交付成果）。

### 8. 可观测

- GenAI span 体系扩展：查询扩展调用、v4 嵌入调用、rerank 调用各有 span/事件属性（延迟、token、命中数）；trace_id 继续随 SSE 事件下发；`traceAgentEvent` 结构化事件补充检索阶段字段。

### 9. 依赖与文档同步

- 零新框架/零新 SDK；前端仅既有 react-markdown 复用。
- 配置键新增（A-03 实现时定稿并同步 config.yaml / .env.example / doc-validator）：embedding provider 切换（DashScope base_url + key + dimensions）、`rag_query_expansion_enabled`、`rag_rerank_enabled`（与 `rag_hybrid_enabled` 同区）。
- 落地时同步文档：`docs/specs/web-agent-v0.4-mvp.md`（SSE v2）、`docs/reference/business-rules.md`（双端 Agent 边界小节）、`design/ui-spec.md`（新增 Agent 工作台章节，A-06 验收项）、`docs/GLOSSARY.md`（若新增产品术语）。
- 改 config.go / migrations / routes.go 后运行 `cd tools/doc-validator && go run . --fix`。

## 范围外（明确延后，防扩散）

断线续传（Last-Event-ID）｜消息编辑重发｜附件/文件上传｜联网搜索工具｜上下文摘要压缩｜跨会话全文搜索｜多 Agent 编排 / MCP 协议｜rerank 自托管（TEI/Xinference）｜qwen3.7-text-embedding｜OpenSearch hybrid（保持可选基础设施）｜P2 代操作（订阅/收藏等——仅保留「确认卡片 + 用户点击执行既有 REST」的契约注记，不实现）｜发布流辅助与 ContentDetail usage-guide 改造（Q13 边界）。

## 部署前提

| 项 | 状态 |
|---|---|
| DashScope API Key（百炼，embedding+rerank 同 key） | **待用户开通**；开通前本地走 fail-open/mock 路径不阻塞开发 |
| GREEN AccessKey | ✅ 已恢复（2026-09-01 服务器探测文本+图片全绿） |
| MiniMax（MiniMax-M3 chat） | ✅ 正常 |
| SiliconFlow key（fallback，可选） | 未开通则 fallback 配置留空，降级链回 RRF |
| 服务器发布 | .env 增补新凭证；重建 backend/worker 镜像（compose 不随镜像内容变更自动重建的坑，见部署记忆） |

## 批次清单（A-01~A-07）

| 票 | 内容 | 车道 | Blocked by |
|---|---|---|---|
| A-01 | 后端会话模型重构：title/pinned_at 迁移、续写 API、服务端上下文组装、自动标题、停止/失败保留 | **heavy**（迁移） | 无（frontier 头） |
| A-02 | SSE 契约 v2：真流式 + 思考块转发 + 工具步骤事件（含 web-agent-v0.4 同步） | light | 无（与 A-01 并行，前端消费在 A-06） |
| A-03 | 检索管线升级：v4 嵌入切换 + hybrid on Postgres + 查询扩展 + rerank（DashScope/SiliconFlow 客户端 + 降级链 + 三开关） | light | 无（独立于 A-01/A-02） |
| A-04 | 评测消融与门禁：golden set 扩充、四配置消融、verify-project 接入、默认值数据裁决 | light | A-03 |
| A-05 | 护栏补口：chat 输入前置 Green + 输出事后异步审核 | light | 无 |
| A-06 | 前端工作台 DeepSeek 化：布局/侧边栏三交互/输入框/三层生成形态/markdown + 行内锚定/复制/空态/明暗/响应式 + SSE v2 消费 + ui-spec 增补 | light | A-01、A-02 |
| A-07 | 搜索页 Agent 入口与新工作台统一 | light | A-06 |

执行顺序与 SP-12 U 批次的关系（工作台页面在 U-04 sweep 清单内、共享视觉基底）由用户在全面审查/整合时统一定；本表依赖边仅为批内约束。

## User Stories

1. 作为用户，我用口语化描述兴趣（「最近有点 emo，想看点治愈的」）就能拿到贴合的站内内容推荐，而不是要求我先会搜索关键词。
2. 作为用户，我看到回答像 DeepSeek 一样逐字生成、思考过程可展开折叠，感觉这是一个成熟的商业产品而非 demo。
3. 作为用户，我能点击回答里的 `[1]` 角标直接定位到它对应的引用卡片并打开内容详情，信任推荐的每个出处。
4. 作为用户，我的会话有名字、能重命名、能置顶常用的、能删除不要的，下次回来能接着聊（上下文由服务端记住）。
5. 作为用户，我中途停止生成时，已生成的部分被保留，而不是整轮消失。
6. 作为用户，我在亮色和暗色主题下使用工作台，观感与全站一致。
7. 作为平台方，任何进入 Agent 的用户输入都过内容审核，任何落库的回答都可被事后审核标记——内容安全体系无旁路。
8. 作为面试观看者，我能从工具步骤区直观看到「查询扩展 → 混合检索 → 精排」的检索管线在工作。
9. 作为项目负责人（简历撰写者），我能用消融数据（Recall@5/nDCG@5）证明每一层检索升级的增益，而不是空口宣称。
10. 作为开发 Agent，我消费 SP-12 定稿的设计 token 与 ui-spec 新章节实现工作台，不产生新的视觉不一致。
11. 作为运维，我不需要在生产服务器新增 OpenSearch 或自托管模型推理服务就获得混合检索与精排。
12. 作为后续开发者，SSE v2 契约、config 开关与降级链在文档中同步成文，我不再需要读代码反推行为。
