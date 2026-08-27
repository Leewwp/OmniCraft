# OmniCraft Agent 工作台最小架构改进执行计划

> 创建日期：2026-08-27
> 状态：active
> 来源规格：`docs/superpowers/specs/2026-08-27-omnicraft-agent-workspace-minimal-architecture-design.md`
> 执行追踪：GitHub issue #207

> **本轮执行确认（2026-08-27）**：用户确认 T01/T02/T03 全部执行，目标窗口为 2026-09-03（面试约一周后）。岗位叙事优先级为 AI 应用开发、Agent 开发、AI 全栈开发；根目录 `.env` 已配置 MiniMax 真实凭证，但任何凭证值都不得进入 Git、日志或证据。

## 目标

为面试/简历本地演示增加一个有实际安全价值、可测试且不扩大基础设施的 Citation Verification seam，并保留现有 Agent 兼容行为。

## 当前执行单元

### T01 [heavy] Citation Verification seam

- 先为伪造引用、viewer 可见性失败、版本/chunk 不匹配和无有效证据编写失败测试。
- 提取服务端引用验真实现；统一工具结果和最终流式输出的验证路径。
- 保持现有 HTTP/SSE 合同、DTO、错误码、降级、配额和失败回合清理语义。
- 运行相关 Go 测试、`go vet`、`go build`、项目验证脚本和文档校验。

### T02 [light] 使用侧 Provider seam

- 仅在 Agent 工作台内部定义流式聊天所需的最小 interface。
- 使用 fake adapter 验证 Agent 不依赖 embedding 或非流式 Chat。
- 不修改 Provider 协议，不迁移发布/合规/嵌入调用方。

### T03 [light] Agent 内部轻量工具 registry

- 用固定名称到处理器的本地 registry 整理现有四个工具分发。
- 保持白名单、参数校验、viewer 可见性、错误归一化和调用限制。
- 不新增跨包 adapter、动态注册或工具并行执行。

## 不作为当前执行单元

Session History、Publish Assistance、Compliance Assistance 的完整拆分、AgentService 删除、服务拆分、数据库迁移、前端重构和大规模检索基础设施均记录在规格的 Further Notes，属于未来方向。
