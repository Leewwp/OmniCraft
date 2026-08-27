# OmniCraft Agent 工作台最小架构改进规格

> 创建日期：2026-08-27
> 状态：已完成（T01/T02/T03，2026-08-28 验证）
> 目标阶段：面试与简历展示的本地最小实现

## Current execution commitment (2026-08-27)

- 用户已确认本轮同时执行 T01、T02 和 T03；T01 按 heavy/TDD，T02/T03 按 light 串行处理。
- 交付窗口：2026-09-03（用户确认面试约一周后）。完成标准是本地可运行、相关测试与项目验证门通过，并能支持页面浏览和 Agent 工作台 live demo。
- 用户已在仓库根目录 `.env` 配置 MiniMax 真实凭证。凭证值不得写入 Git、日志、截图或证据文档；需要 provider 验证时只记录模型、状态、延迟和安全摘要。
- 目标岗位优先级：AI 应用开发 > Agent 开发 > AI 全栈开发。简历与演示叙事必须优先突出引用安全、Agent 编排、RAG/工具调用和可验证性。
- 第二阶段方向继续由 #208 收容；本轮不提前实现 Session History、生产部署、桌面/App 或大规模 Agent 拆分。

## Problem Statement

当前 Agent 工作台的 `AgentService` 同时承载流式对话、工具调用、引用处理、发布辅助、合规检测、自然语言搜索、嵌入、内容审核和桌面部署脚本等职责。这个结构能够工作，但安全相关的引用验真规则与 Agent 编排规则耦合在较大的实现中，难以单独说明、测试和复用。

本项目面向面试和简历展示，不需要为真实用户规模做大规模模块化或服务拆分。改动必须以可运行、可测试、可解释为目标，避免为小数据量场景引入不匹配的基础设施或仅为形式上的抽象。

## Solution

以现有 Agent HTTP/SSE 合同为最高测试 seam，抽取一个服务端拥有的引用验真模块，集中处理候选引用的身份重载、版本/chunk 校验、viewer 可见性复核和引用卡片构造。保持现有 Agent 行为不变。

在不改变业务行为的前提下，可选地增加使用侧 Provider 小接口和 Agent 内部轻量工具注册结构，作为低成本的设计展示。保留 `AgentService` 兼容外壳，不进行完整调用方迁移或删除。

## User Stories

1. 作为面试官，我希望看到模型输出的引用经过服务端重新验证，以便判断项目是否理解 LLM 输出不可信这一安全事实。
2. 作为查看者，我希望 Agent 只显示我有权访问的引用，以便隐藏内容不会通过问答结果泄露。
3. 作为查看者，我希望引用始终指向当前仍有效的内容版本和 chunk，以便引用不会指向过期或篡改后的内容。
4. 作为查看者，我希望引用卡片的标题、区域、路由、摘要和来源由服务端生成，以便模型不能伪造站内链接或元数据。
5. 作为 Agent 工作台维护者，我希望工具结果和最终流式回答共用同一引用验真实现，以便修复一次即可覆盖两条路径。
6. 作为 Agent 工作台维护者，我希望现有 SSE 事件名称、顺序和错误语义保持不变，以便前端和演示脚本无需重写。
7. 作为项目维护者，我希望无有效引用时保持 `no_evidence` 分类，以便系统不会把未经验证的回答包装成有依据的回答。
8. 作为项目维护者，我希望引用验真失败时返回稳定的安全结果，而不是底层数据库或 Provider 错误，以便客户端不会看到内部实现细节。
9. 作为测试作者，我希望通过服务端接口测试伪造引用、隐藏内容、版本不匹配和 chunk 不匹配，以便安全契约有明确证据。
10. 作为面试官，我希望看到 Agent 只依赖流式 Provider 能力，以便判断接口是否按使用方需求设计，而不是复用过宽的 Provider 接口。
11. 作为 Agent 工作台维护者，我希望固定工具白名单仍由服务端控制，以便模型不能注册工具或绕过调用上限。
12. 作为项目维护者，我希望可选改动不会引入数据库迁移、前端重构、微服务或新基础设施，以便本地演示保持简单可靠。
13. 作为项目维护者，我希望失败/取消的流式回合仍清理新建会话，以便保留当前持久化语义。
14. 作为项目维护者，我希望 RAG 降级到关键词检索的行为保持不变，以便小数据量本地环境不依赖大型检索系统。

## Implementation Decisions

- 必做模块是 Citation Verification。其接口接收服务端生成的候选引用和 viewer identity，重新加载内容、content version、chunk 和可见性事实，并返回服务端构造的 Agent Citation Card 或稳定的无效结果。
- 可信引用身份固定为 `content_id + content_version + chunk_key + chunk_index`；模型生成的 URL、title、route、source 和 textual citation marker 均不是可信事实。
- Agent 工作台、工具结果和最终流式输出均通过同一 Citation Verification seam；不改变已有 DTO、SSE 事件集合、事件顺序、配额时序、Provider 超时/取消和降级语义。
- 保留现有 `AgentService` 作为兼容外壳。首轮改动只允许委托到新模块，不删除外壳，不要求迁移所有直接构造 `AgentService` 的测试和 worker。
- 可选模块 A：在 Agent 工作台使用侧定义仅包含流式聊天能力的内部 Provider interface；发布辅助若后续需要，可独立定义非流式聊天 interface。不得修改第三方 Provider 协议。
- 可选模块 B：在 Agent 包内将固定工具白名单和分发逻辑整理为轻量 registry。工具数量仍为现有四个，参数校验、可见性检查、错误归一化和调用上限保持不变；不引入跨包 adapter 层或动态注册机制。
- 不新增数据库表或 migration，不替换 PostgreSQL/pgvector，不引入 OpenSearch、gRPC、微服务、多 Agent 编排或新的前端状态模型。
- 该改动属于 heavy lane：引用安全、viewer 可见性和内容版本校验必须先有失败测试，再做最小实现；可选 Provider/registry 改动若单独执行可按 light lane 处理。

## Testing Decisions

- 测试通过现有 Agent 服务和 HTTP/SSE seam 验证外部行为，不断言具体私有函数、目录结构或 map/switch 实现。
- 必做测试覆盖：有效引用、伪造引用拒绝、viewer 无权访问、content version 不匹配、chunk identity 不匹配、服务端 route/title/excerpt/source 构造、无有效引用时的 `no_evidence`、工具结果与最终回答的一致性。
- 保留并扩展现有 Agent contract、RAG contract、tool、stream、evaluation、handler SSE 测试；错误测试确认不泄露原始 Provider/数据库错误。
- 可选 Provider seam 测试使用仅实现 `ChatStream` 的 fake adapter，验证 Agent 不需要 embedding 或非流式 Chat 能力。
- 可选 registry 测试验证固定工具名称、未知工具拒绝、参数错误、隐藏内容拒绝和最大调用次数；不测试 registry 的具体容器类型。
- 完成验证至少包括相关 Go 测试、`go vet`、`go build` 和项目验证脚本；修改文档权威文件后运行 doc-validator。

## Out of Scope

- Agent Session History 的独立模块和 `BeginTurn/CompleteTurn/AbortTurn` 全量迁移。
- Publish Assistance、Compliance Assistance、Moderate、EmbedContent 和 NLSearch 的完整模块拆分；这些能力保持现有兼容表面。
- 删除 `AgentService` 兼容外壳或一次性迁移所有 handler、worker、container 和测试构造函数。
- 工具并行执行、新重试策略、SSE 合同变化、前端重构、数据库迁移和 RAG 排名质量调整。
- Agent/Search 微服务、gRPC、多 Agent 平台、Kafka、cross-encoder reranker 或面向超大数据量的向量基础设施。
- 桌面部署脚本和 Tauri 能力恢复。

## Further Notes

未来方向按触发条件排序：

1. 当会话列表、读取、分页、删除和流式回合持久化出现独立变化需求时，再评估 Agent Session History 模块；届时优先隐藏事务细节，而不是机械暴露数据库生命周期。
2. 当发布辅助或合规规则需要独立 Provider、独立测试和独立演进时，再拆分 Publish Assistance 与 Compliance Assistance。
3. 当工具数量、权限策略或多个适配器真实增长时，再把轻量 registry 深化为独立模块；单一实现不足以证明更大 seam 的价值。
4. 当真实数据规模、检索延迟或故障证据达到明确阈值时，再评估 OpenSearch、独立 Agent/Search 进程或更复杂的检索基础设施；当前小数据量本地演示不采用这些技术。
5. 简历表述应只声称已实现并通过测试的引用安全和 Agent SSE 行为；其余内容标记为设计或未来方向，不写成已上线能力。
