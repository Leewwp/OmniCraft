# OmniCraft context map

`docs/GLOSSARY.md` 是产品术语和定义的唯一权威。本文件只提供面向开发与测试的导航，不复制术语定义；涉及名称、边界或 Avoid 词汇时必须读取 Glossary 原文。

## Domain map

- **列表分页契约（#668）**: 标准列表端点的 page/page_size 解析唯一住所 = `backend/internal/handler/pagination.go` 的 `pageQuery`（page<1→1；page_size<1 或 >100→20 回落）；响应回显生效值。裸 `Query("page")` 解析由 `pagination_gate_test.go` 守门拦截，例外（搜索/Agent 会话/admin trace/parsePositiveInt 家族）在 helper 注释与守门白名单登记理由。
- **错误信封（#669）**: handler 错误响应唯一发送处 = `backend/internal/pkg/response`（Error 系标准形 {code,message[,details]}；CodeOnly 历史 code-only ×10；CaptchaError 验证码错误 ×4 附加 captcha_result:false）。全部 AbortWithStatusJSON。handler 非测试源码新增裸 `gin.H{"code"` 由 `envelope_gate_test.go` 守门拦截；成功 ack（`gin.H{"message"`）合法。码值集与逐点迁移账见 docs/working/2026-09-25-669-envelope-census.md（本地）。

- 内容发现：推荐流、原创/二创分区、IP 库与 IP 详情页；决策入口见 `docs/GLOSSARY.md` 和 `docs/working/2026-08-04-content-discovery-gap-plan.md`。
- 内容浏览：所有卡片入口最终复用内容详情浮层，完整详情页保留给直达 URL；媒体集/媒体查看器/连续浏览/相关内容规范见 `docs/superpowers/specs/2026-08-08-omnicraft-media-experience-design.md`；路由决策见 `docs/working/2026-07-25-wayfinder-ticket-content-modal-routing.md`。
- Web Agent：顶部导航进入受保护的 `/agent` 全页工作台，全站搜索保持关键词职责；见 `docs/adr/0003-web-agent-dedicated-workspace.md`。
- Agent 工作台编排：Agent 工作台内部负责会话上下文、工具策略、流式事件顺序和受控任务协同；不直接拥有发布辅助、合规检测或会话历史的数据访问实现。一次问答的服务端周期见「回答回合」。
- Agent 会话历史：Agent 会话的创建与续写、消息持久化、标题与置顶管理的事实源；列表、读取与删除属于独立的历史查询职责。
- 引用验真：从服务端候选引用和查看者身份重新加载内容、版本、chunk 与可见性真相，并构造 Agent 引用卡片；模型提供的 URL、标题和 route 不属于可信输入。
- 发布辅助：面向创作者工作室发布表单的标题、描述、分类和标签建议能力；Agent 工作台只能通过受控工具 seam 调用，不拥有其 Provider 细节。
- 合规检测：面向发布流程的文本/媒体安全检查能力；属于内容发布规则，不属于 Agent 工作台会话编排。
- 站内内容问答：面向 OmniCraft 已发布内容的 RAG 能力；内容、标签、IP 与来源关系属于社区内容域，Markdown 仅是本地导入适配器，不构成独立知识库产品。
- 社区互动：用户资料浮层、私信聊天浮层和冷启动私信共享消息事实源；详细规则见 `docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md` §1。
- 内容审核：AI 审核（阿里云 Green）与人工判官双轨；术语见下方 Language 的"扫描结果回调 / 审核结果处理"分化，契约事实见 `docs/working/2026-08-08-aliyun-content-safety-callback-research.md`。

## Language

**扫描结果回调**（Aliyun scan callback）：阿里云内容安全向应用推送异步媒体审核结果的入站通知（`application/x-www-form-urlencoded`，`checksum`+`content`，SHA256(uid+seed+content) 认证）。_Avoid_：单独使用"回调"一词

**审核结果处理**（AI review processing）：应用内部将审核结果落库并变更内容/IP 状态的流程（`AICallbackInput` → `ProcessAICallback`），由扫描结果回调或同步审核路径触发，与入站通知是两个契约。_Avoid_：回调、回调处理

**AI 审核终态**（AI review terminal state）：`banned` 是 AI 审核通道的终态——后续 AI 审核结果（pass/review）不得覆盖（防同步图片 block 后被异步视频 pass 复活）；**人工通道不受限**：申诉批准（`AppealTargetUpdates` approved → published）与判官翻案仍可翻转 banned。_Avoid_：把 banned 说成全局不可逆

**确定性检索工作流**（deterministic retrieval workflow）：查询改写、关键词/向量召回、过滤、rerank 与可见性复核组成的可观测步骤；这些步骤不因采用 Agent 产品而自动成为 Agent。

**回答 Agent**（answer agent）：消费服务端已验证的检索证据并生成带站内引用的回答；不得绕过可见性复核直接信任模型输出。

**多 Agent 编排**（multi-agent orchestration）：多个具有独立提示词、预算或职责的 Agent 协同完成任务。它不是 OmniCraft 当前 RAG 的默认形态，只有相对单 Agent 基线的质量、成本、延迟与安全收益经对照评测证明后才升级实施。

**会话续写**（conversation continuation）：Agent 会话以稳定身份跨轮存在；用户每轮只提交增量消息，完整上下文由服务端按预算组装。重命名、自动标题与置顶属于会话管理操作，不改变会话身份。_Avoid_：把"新建会话"当作每轮默认行为

**回答回合**（answer turn）：一次提问到终局事件之间的完整服务端周期——会话解析、轮次编排（turnRunner 的 LLM/工具循环，经 ToolRuntime seam 执行工具）、引用验真、终局落库与事件收口；成功/错误/规则短路三条终局路径共用同一收口（finalizeTurn 的终局四件套：落库+trace 终行+metrics+终局事件），任何结局不可能漏件。_Avoid_：把一个回合的终局逻辑写在收口之外

**思考块**（thinking block)：模型推理过程中产生的中间内容，仅用于过程展示，不属于回答正文；不参与引用验真，也不是工具结果。_Avoid_：把思考内容当作回答或证据引用

**引用锚定**（citation anchoring）：回答正文中的编号角标只是指向引用卡片的展示层映射；引用的可信度判定只发生在服务端复验后的卡片级，模型自由发挥的 URL、标题与路由不属于可信输入。

## Test seams

- HTTP/API 行为通过 handler/router 边界验证，业务事务与缓存恢复通过 service 集成测试补充。
- 前端页面行为通过可访问角色、路由和用户可见状态验证，不依赖组件内部实现。
- 发布门通过统一 verifier、workflow contract 和 evidence schema 验证。

## Authority

权威冲突顺序与任务来源以根目录 `AGENTS.md` 为准；架构决策索引见 `docs/adr/README.md`。
