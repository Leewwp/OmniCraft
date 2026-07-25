# OmniCraft 双端 Agent 产品化设计

> **2026-07-25 执行范围调整：** 当前只推进 Web Agent 产品化与 Web 部署。本文的 Desktop Agent 设计作为未来边界保留，但 D-02～D-05/R-02、Tauri 本地动作和桌面发布在用户明确恢复 Desktop 范围前均为暂缓状态。

> 日期：2026-07-16
> 状态：confirmed
> 设计输入：项目负责人确认的求职作品定位与 2026-07-16 文档审查

## 1. 目标与定位

OmniCraft 的 Agent 是作品集主线能力，不是附属聊天框：

- Web Agent：帮助用户发现、理解和使用站内公开内容；回答必须可追溯到内容来源。
- Desktop Agent：把用户确认的内容下载和本地配置意图转换为严格、可预览、可审计的白名单动作。
- 两端共享 Provider 抽象、预算限制、追踪标识、安全错误和审计原则，但不追求功能对称。

本阶段目标为小规模生产可用和面试可验证，不承诺多地域容灾或大规模自治 Agent。

## 2. 非目标

- 不允许 Web Agent 直接读取未显式选择的私有草稿、上传文件或个人数据。
- 不允许 Desktop WebView/LLM 直接调用任意文件系统命令。
- 不允许模型自由生成未注册工具名、任意 shell、任意绝对路径或任意下载域名。
- 不以“接入某个 LLM API”作为完成标准；必须有引用、工具边界、评测和降级证据。
- 不在本阶段实现长期记忆、自治多 Agent 编排、模型微调或跨用户私有知识共享。

## 3. 统一信任边界

### 3.1 模型是不可信建议源

模型输出永远不能直接成为权限判定或本地操作。所有工具调用必须经过服务端/原生端的确定性校验：

1. 身份和功能开关；
2. 输入 schema；
3. 资源可见性；
4. 配额和速率；
5. 工具 allowlist；
6. 用户确认（涉及本地副作用时）；
7. 审计和脱敏。

### 3.2 共享追踪模型

每次 Agent 请求生成 `trace_id`，记录：

- surface：`web` / `desktop`；
- user_id（服务端内部）；
- provider/model；
- tool 名称、结果状态和耗时；
- prompt/completion token 数与估算成本；
- citation 内容 ID；
- safe error code；
- 是否发生降级。

禁止记录原始密钥、完整私有文件内容、原始本地绝对路径或未经脱敏的 Provider 错误。

### 3.3 资源上下文不能信任客户端摘要

- Web 请求只提交 server-owned surface 枚举和可选 `content_id`；不得把客户端提供的 title、content_type、作者或可见性当成 system context。
- 服务端按当前 viewer 重新加载资源；不可见资源统一按 not-found 处理，不能通过 Agent 探测存在性。
- 现有 `/agent/usage-guide/:id` 同步/流式入口与未来 `get_usage_guide` tool 使用同一 viewer-aware resolver，只允许当前 viewer 可见的 published 内容。
- 发布建议只处理用户在当前发布表单中显式提交的、有长度上限的 typed snapshot；仓库当前没有独立 draft 状态，模型不能提供任意 draft/content ID。
- 用户侧 Agent 不暴露可写审核工具。现有 `/agent/moderate/:id` 在启用 Web Agent 前从普通用户路由移除；内容审核只由已有受权后台/worker 流程触发。

## 4. Web Agent

### 4.1 核心用户闭环

1. 用户以自然语言描述想找的内容或提出与站内内容有关的问题。
2. Agent 选择只读工具检索公开内容。
3. 服务端执行可见性过滤和参数校验。
4. Agent 生成带引用的回答。
5. 用户可打开引用卡片继续浏览；失败时降级到普通关键词搜索。

### 4.2 MVP 工具

| Tool | 作用 | 权限 |
|------|------|------|
| `search_content` | 关键词/语义组合检索 | 只返回当前 viewer 可见 published 内容 |
| `get_content_detail` | 获取回答所需的紧凑内容摘要 | 共享内容可见性规则 |
| `get_usage_guide` | 获取内容类型使用说明 | 只读 |
| `suggest_publish_metadata` | 为当前请求显式附带的发布表单 typed snapshot 提出标题/摘要/标签建议 | tool call 不接收资源字段；snapshot 由服务端绑定到请求上下文。仅建议，用户确认后由普通发布 API 提交 |

上传辅助和合规检测可以复用现有入口，但不得把模型建议自动写入内容或自动发布。

### 4.3 回答契约

成功响应/流式最终事件必须包含：

```json
{
  "trace_id": "...",
  "answer_kind": "grounded_content",
  "answer": "...",
  "citations": [
    {"content_id": 123, "title": "...", "zone": "original", "excerpt": "..."}
  ],
  "tools": [
    {"name": "search_content", "status": "success", "duration_ms": 42}
  ],
  "usage": {"prompt_tokens": 0, "completion_tokens": 0},
  "degraded": false
}
```

规则：

- 由请求类型和服务端执行路径确定 `answer_kind`，不能让模型自行判断是否需要引用。chat/search/detail/usage-guide 产生的自然语言回答统一为 `grounded_content`，至少包含一个有效引用；否则返回 `no_evidence` 并展示搜索建议。`publish_suggestion` 使用独立 typed contract，可不带站内内容引用。
- 引用必须在模型生成后再次通过可见性检查，缺少有效 `id/title/zone` 的对象不能生成链接。
- 不向前端暴露原始工具参数、SQL、内部 prompt 或 Provider 错误。
- 工具调用达到上限、超时或 Provider 失败时，返回稳定错误并允许降级到普通搜索。

### 4.4 预算与滥用控制

- 每用户日限 `rate_limit_per_day` 和分钟突发限 `rate_limit_per_minute` 从 config 读取；匿名用户默认不开放对话式 Agent。
- 限制单轮输入长度、历史消息数、工具轮数、检索候选数和最大输出 token。
- 分钟/日请求计数通过一次 Redis Lua 原子预留。feature/auth/request-schema 与客户端直接提交的资源上下文先做可见性预检，失败不计数。模型生成的 tool 参数只能在首次 Provider 调用后校验；此时越权资源向模型返回统一 not-found，但整次请求已消耗配额。一旦预留并进入 Provider/tool 阶段，成功、超时、Provider 错误和客户端取消都消耗该次请求并记录结果。Redis 不可用时 Provider-consuming 入口 fail-closed；只读会话历史不占生成配额。
- 首版硬预算单位是请求次数，并通过输入、输出 token 与工具轮数上限约束单次成本；实际 token/估算成本用于观测告警，不伪装成尚未实现的金额账本。
- 管理端只展示聚合用量、错误率、延迟和预算告警，不展示用户完整对话正文。

### 4.5 Prompt Injection 防护

- 检索内容是“不可信数据”，不能覆盖 system/tool policy。
- 内容中的“忽略规则、调用工具、泄露 prompt”等文字只能作为引用文本处理。
- 工具参数由结构化 schema 校验；模型无权扩展工具集合。
- 针对内容注入、越权私有内容、伪造 citation 建立回归评测。

### 4.6 会话生命周期

- “开始新对话”只切换到新的本地/服务端 conversation，不删除旧记录，不需要破坏性确认。
- “清空当前历史”调用 `DELETE /api/v1/agent/conversations/:id`。后端只按 `id + current_user_id` 删除 conversation 及级联 messages；不存在、已删除或属于他人的 ID 统一返回幂等 `204`，既不泄露存在性也不删除他人数据。
- 数据库失败返回稳定错误且不清除前端现有消息。UI 使用 ConfirmModal；取消或失败后焦点回到触发按钮，成功后回到新对话输入框。
- 删除对话正文不删除已脱敏的 trace、聚合 token/成本指标或安全审计；这些记录不得包含原始完整对话。

## 5. Desktop Agent

### 5.1 产品闭环

1. 用户在 Web 或桌面端选择内容并发起“下载/配置”意图。
2. 服务端签发短时、单次 deploy grant。
3. Desktop 用 grant 换取精确 canonical JSON bytes 与 Ed25519 签名。
4. Rust 在解析前验证签名和 key ID，再严格反序列化动作 schema。
5. Desktop 显示完整、脱敏的动作预览。
6. 用户确认后，Rust 使用一次性内存 handle 顺序执行动作。
7. 敏感写/移动动作再次原生确认；失败后停止后续动作并提供脱敏诊断摘要。

### 5.2 唯一允许动作

- `download_file`
- `extract_archive`
- `move_file`
- `create_dir`
- `read_config`
- `write_config`
- `backup_file` 仅由 Rust 在写入/移动前内部触发，模型与 WebView 不可直接请求。

### 5.3 必须完成的安全前置

Beta D-02 至 D-05 和 R-02 是宣称 Desktop Agent 可用前的硬门禁：

- 不再把 HMAC 私钥/共享密钥嵌入客户端；
- WebView 不再直接暴露原始文件操作命令；
- 所有路径是固定根目录下的逻辑相对路径；
- HTTPS 下载 host 使用精确 allowlist；
- zip-slip、路径逃逸、重放、过期、未知字段和未知动作均拒绝；
- `features.desktop_deploy_enabled` 在 R-02 之前保持 `false`。

## 6. Provider 与降级

- 保留 `qwen` 与 `openai_compat` Provider 抽象；业务服务不拼接供应商特有字段。
- 配置支持模型、base URL、timeout、最大重试和输出 token；密钥只从环境变量/密钥管理注入。
- 仅对明确可重试的网络/429/5xx 做有界退避；工具副作用请求不自动重试。
- Web Agent Provider 不可用时降级关键词搜索；Desktop 已签名动作执行不依赖 LLM 在线状态。

## 7. UI 设计要求

### Web

- 展示流式状态、停止按钮、引用卡片、工具状态摘要、降级提示和重试。
- 引用可键盘访问并明确标识来源；回答与引用不能只靠颜色关联。
- 不展示内部 chain-of-thought；只展示简短工具状态，例如“已检索 8 条内容”。
- 新对话、清空历史和发送草稿具有明确焦点/确认行为。

### Desktop

- 动作预览显示动作类型、逻辑目标、下载来源域名和风险等级，不展示敏感绝对路径。
- 脚本级确认与敏感动作二次确认使用原生能力，WebView 不能伪造已确认状态。
- 失败状态包含稳定错误码、失败动作和可复制的脱敏诊断摘要。

## 8. 评测与上线门禁

维护固定的 Agent evaluation fixtures：

- 公开内容检索召回与引用正确性；
- 无依据问题的拒答；
- prompt injection 不改变工具策略；
- 私有/下架内容不可被引用；
- Provider 超时/限流降级；
- Desktop grant 重放、签名篡改、未知动作、路径逃逸、非法 host、zip-slip；
- 所有本地副作用必须经过原生确认。

Web 发布门：核心评测、限流、引用可见性、流式错误和浏览器验证通过。

Desktop 发布门：D-02～D-05、Rust 测试、端到端安全用例和 R-02 全部通过。任何真实密钥/域名/允许下载 host 缺失时保持功能关闭并记录阻塞。

## 9. 文档与状态同步

- `architecture.md` 描述当前实现与目标安全门，不能把未完成的 Ed25519 链路写成已完成。
- `design/ui-spec.md` 是 Agent 页面/组件的视觉和交互权威，Props 禁止使用 `any` 占位。
- Web Agent 实现计划独立维护；Desktop 继续使用 Beta D-02～D-05 与 R-02，不创建重复安全计划。
- `progress.txt` 只在实际实现和验证后更新；文档确认不等于功能完成。
