# 网页版 Agent（V0.4 MVP）

> 本文档由 2026-07-23 文档瘦身从 `architecture.md` §11 抽取，章节号保持原编号以便深链兼容。

## 11. 网页端 Agent（V0.4 MVP）

> **产品定位（2026-07-16 确认）**：Web Agent 是作品集和上线版本的核心能力，负责公开内容发现、带引用问答、使用指导和用户确认后的发布建议；Desktop Agent 负责受控下载与本地配置。两端共享 Provider、预算、追踪和安全原则，但不追求功能对称。
> **发布范围**：优先完成自然语言检索/问答、来源引用、使用指导、上传/发布建议、限流预算、评测和失败降级。内容初审辅助保留，但不能挤占用户侧检索问答闭环。
> 详细产品化规则以 `docs/superpowers/specs/2026-07-16-omnicraft-dual-surface-agent-productization-design.md` 为准。

### 11.1 设计原则

- **Provider 抽象**：LLM 供应商通过统一接口隔离；canonical 实现为 MiniMax-M3（minimax 适配器）+ 独立 embedding provider 拆分接线，原生 qwen 适配器已退役（fail-closed 拒绝并指路 compatible-mode）——选型唯一权威见 `docs/reference/agent-runtime-matrix.md`
- **Tool-Call 模式（MVP）**：LLM 从白名单 tool 列表中选择调用，单轮或短链路执行，不使用自主 ReAct 循环
- **SSE 流式传输**：所有 Agent 流式响应使用 Server-Sent Events，不引入 WebSocket
- **Feature Flag 控制**：`agent.web_agent_enabled: false`（默认关闭，独立于 Tauri `features.agent_enabled`；配置位置见 §11.3）
- **零独立基础设施**：向量检索使用 pgvector（已有 PostgreSQL），不引入新服务
- **Grounding 优先**：涉及站内事实的回答必须包含服务端校验过的内容引用；无足够依据时明确拒答或降级关键词搜索
- **不可信内容边界**：检索到的正文不能修改 system/tool policy；工具参数和引用均经过严格 schema 与可见性复核
- **可观测性**：每次请求生成 `trace_id`，记录脱敏工具状态、延迟、token 用量、引用 ID 和降级状态，不记录 chain-of-thought 或原始 Provider 错误

### 11.2 LLM Provider 抽象层（Go）

```go
// internal/pkg/llm/provider.go
type ChatMessage struct {
    Role    string `json:"role"`    // "system" | "user" | "assistant" | "tool"
    Content string `json:"content"`
}

type ChatRequest struct {
    Messages    []ChatMessage          `json:"messages"`
    Tools       []ToolDefinition       `json:"tools,omitempty"`
    Stream      bool                   `json:"stream"`
    MaxTokens   int                    `json:"max_tokens,omitempty"`
}

type ChatDelta struct {
    Content   string `json:"content"`
    ToolCalls []ToolCall `json:"tool_calls,omitempty"`
    Done      bool   `json:"done"`
}

type LLMProvider interface {
    Chat(ctx context.Context, req ChatRequest) (ChatMessage, error)
    ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatDelta, error)
}
```

> 上图为规格期草图；当前真实接口见 `internal/pkg/llm/provider.go`（`ChatDelta` 含 `Thinking`/`ToolCalls`/`Usage` 通道，`ChatRequest` 含 `MaxTokens`/`Temperature`/`Stream`，provider options 携带独立 embedding key/base/维度）。

**实现类**：

| 实现 | 说明 |
|---|---|
| `MiniMaxProvider` | MiniMax M 系列（canonical：MiniMax-M3）；流式 `<think>` 标签由 thinkSplitter 分流为 think_delta，解析 tool_calls；embedding 走 MiniMax 原生端点（legacy embo 路径） |
| `OpenAICompatProvider` | 通用 OpenAI 协议（DeepSeek / DashScope compatible-mode / 本地 Ollama 均可复用）；解析 `reasoning_content` 思考通道；承担 canonical 的独立 embedding provider（text-embedding-v4 批量） |
| `CompositeProvider` | 拆分接线（canonical）：chat/流式委托 minimax，单条与批量 embedding 委托 openai_compat，`GetEmbeddings` 批量转发保持 v4 批量契约 |
| `QwenProvider` | **已退役（2026-09-05）**：原生 DashScope 适配器不发送 tools/max_tokens、不解析 tool_calls/reasoning，工厂 fail-closed 拒绝；DashScope 一律走 openai_compat + compatible-mode |

运行时通过 `config.yaml > agent.llm_provider` 选择，工厂函数 `llm.NewProvider(cfg)` 返回接口实例。

### 11.3 配置扩展（config.yaml）

> 当前已实现字段以 `config/config.go > AgentConfig` 为准；标注“产品化计划新增”的字段是启用 Web Agent 前的目标契约，尚未实现时不得假定运行时已读取。以下为当前与目标关键字段摘要：

```yaml
agent:
  web_agent_enabled: false          # 网页端 Agent 总开关（独立于 Tauri agent_enabled）
  llm_provider: minimax             # minimax | openai_compat（原生 qwen 已退役）
  llm_model: MiniMax-M3             # canonical；模型/端点唯一权威见 agent-runtime-matrix.md
  llm_api_base: "https://api.minimaxi.com"   # 不带尾部 /v1
  llm_api_key: ""                   # 从 env: AGENT_LLM_API_KEY 注入
  embedding_provider: openai_compat # 拆分接线：embedding 独立 provider（跨厂商必须自带 key，fail-closed）
  embedding_model: text-embedding-v4
  embedding_dimensions: 1536
  rate_limit_per_day: 50
  rate_limit_per_minute: 5          # Provider/tool 请求分钟突发上限（产品化计划新增）
  upload_assist_max_file_mb: 10
  max_user_message_chars: 4000      # 用户消息最大字符数
  chat_max_context_messages: 10     # 对话上下文最大消息数
  chat_context_token_budget: 8000   # 服务端上下文组装 token 预算（A-01，估算法：CJK 1 rune≈1 token）
  max_tool_calls_per_turn: 4        # 单轮工具调用上限（产品化计划新增）
  max_output_tokens: 1200           # 单轮最大输出 token（产品化计划新增）
  provider_timeout_sec: 30          # Provider 超时（产品化计划新增）
  provider_max_retries: 1           # 仅网络/429/5xx 有界重试（产品化计划新增）
  citation_max_count: 8             # 回答最大引用数（产品化计划新增）
  hmac_secret: ""                   # 当前禁用的 Tauri 原型兼容字段；D-03 后删除并改用 deploy.ed25519_*
```

**环境变量**（不写入 config.yaml）：

```
AGENT_LLM_API_KEY=sk-xxx
```

**LLM 配置来源（2026-09-05 修订）**：运行时 Provider 只在启动时从 **env（根目录 `.env`，最高优先）→ config.yaml `agent.*`** 构造一次；`llm_configs` 表（`/admin/agent-config`）降级为**配置登记 + 连接测试**，激活不改变运行中的 Agent——每请求生效身份以 OTel span（`gen_ai.request.model`）与 `/api/v1/config/public` 的非敏感身份为准。动态 resolver（DB active 热切换）属 Phase 2 范围；本节早期"DB 优先于 config.yaml"的承诺不再成立。

### 11.4 数据库 Schema

```sql
-- 启用 pgvector 扩展（一次性执行）
CREATE EXTENSION IF NOT EXISTS vector;

-- 对话历史
CREATE TABLE agent_conversations (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT REFERENCES users(id) ON DELETE CASCADE,
    context_type    VARCHAR(50) NOT NULL,  -- 'upload_assist'|'compliance'|'search'|'usage_guide'|'moderation'|'general'
    context_id      BIGINT,                -- 关联 content_item_id（可为 NULL）
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON agent_conversations (user_id);

CREATE TABLE agent_messages (
    id                  BIGSERIAL PRIMARY KEY,
    conversation_id     BIGINT REFERENCES agent_conversations(id) ON DELETE CASCADE,
    role                VARCHAR(20) NOT NULL,  -- 'user'|'assistant'|'tool'
    content             TEXT NOT NULL,
    tool_calls          JSONB,
    -- tool_calls 结构（遵循 OpenAI Chat Completions tool_calls 数组格式）：
    -- [{ "id": "call_xxx", "type": "function",
    --    "function": { "name": "search_content", "arguments": "{...json...}" } }, ...]
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON agent_messages (conversation_id);

-- 内容向量嵌入（用于语义搜索）
CREATE TABLE content_embeddings (
    content_item_id     BIGINT PRIMARY KEY REFERENCES content_items(id) ON DELETE CASCADE,
    embedding           vector(1536) NOT NULL,
    embedded_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- IVFFlat 索引（适合百万级以内向量；P2 阶段可升级为 HNSW）
CREATE INDEX ON content_embeddings USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
```

> **现状（2026-09-05）**：上表为 022 迁移的 legacy content 级向量（每内容一条，`title+description+tags` 拼接），现服务推荐系统/legacy 读域。Agent RAG 已升级为 **chunk 级投影**（迁移 071：`rag_chunks` / `chunk_embeddings` / `index_projection_status`，双世代），两域的模型身份、重建与回滚边界见 `docs/reference/agent-runtime-matrix.md` §1。

### 11.5 API 路由

所有 Agent 接口挂载于 `/api/v1/agent/`，需登录（JWT）。下表先记录当前代码真相；`web_agent_enabled=false` 时这些 Provider 入口必须返回 feature-disabled。

| 方法 | 当前路径 | 当前/目标说明 |
|---|---|---|
| `POST` | `/agent/upload-assist` | 当前接受用户显式提交的发布表单 snapshot，返回建议；产品化后归入 typed publish suggestion 契约 |
| `POST` | `/agent/compliance-check` | 当前对显式表单文本执行合规建议；不是模型可自由选择的 tool |
| `POST` | `/agent/search` | **已下线（2026-09-04，A-07 后端清理）**：NLSearch 端点随前端唯一消费者退役而删除；检索能力并入 `/agent/chat/stream` 工具链（服务端引用复验/可见性行为由 chat 工具链共享） |
| `GET` | `/agent/usage-guide/:id?stream=true` | 当前同步/流式共用路径；产品化前必须加入 viewer-aware reload |
| `POST` | `/agent/moderate/:id` | **待移除的普通用户旧入口**：当前可写 AI review，不得随 Web Agent 开启；审核仅保留在受权 admin/worker 链路 |
| `POST` | `/agent/chat/stream` | 通用流式对话；typed SSE events、server-owned surface 和可选 content ID。**A-01（2026-09-02）请求体硬切换**：`{conversation_id?, message, context?}`——首次省略 conversation_id 则服务端建会话并在 start/done 事件返回 id；续写由服务端按 token 预算组装上下文，客户端不再上传整段历史（旧 `{messages:[...]}` body 400 拒绝） |
| `GET` | `/agent/conversations`、`/agent/conversations/:id` | 仅当前用户会话历史；只读请求不占 Provider 生成配额。列表排序（A-01）：置顶组 `pinned_at DESC`，其余 `updated_at DESC` |
| `PATCH`（A-01 新增） | `/agent/conversations/:id` | owner-scoped 重命名（`{title}` ≤50 runes）与置顶/取消置顶（`{pinned: bool}`）；foreign/missing 均 404；`updated_at` 仅由聊天轮次推进 |
| `DELETE`（产品化新增） | `/agent/conversations/:id` | owner-scoped 幂等清空当前会话/messages；missing/foreign 均 204，不删除脱敏 trace/聚合指标 |

**会话模型（A-01，2026-09-02）**：`agent_conversations` 增 `title`（可空 VARCHAR(200)，首轮问答完成后异步生成摘要标题，失败回退首条用户消息截断 ≤50 runes；存量不回填，无 title 显示「未命名」）与 `pinned_at`（可空 TIMESTAMPTZ）；停止/失败保留会话与已流出的部分内容，删除仅发生在用户显式 DELETE（迁移 074）。

**当前上传辅助请求与产品化 snapshot 约定**：

```jsonc
// POST /agent/upload-assist
{
  "title": "当前表单标题",
  "description": "当前表单简介",
  "filename": "用户当前已选择文件的服务端受控标识",
  "content_type": "mod"
}
// Response: publish_suggestion typed DTO（非流式）
{
  "suggested_tags": ["Minecraft", "家具", "1.20+"],
  "suggested_category": "mod",
  "suggested_title": "简约北欧风家具包 v2.0",
  "suggested_description": "..."
}

// 产品化 POST /agent/chat/stream → typed SSE（A-02 契约 v2，2026-09-02 定稿）
// 事件集：start / think_delta / tool_status / delta / citation / usage / done / error
// 真流式：正文与思考增量按 provider chunk 逐段转发（首事件延迟 ≈ 首 token 延迟），
// 终态裁决在 done；done.message_id = 落库的 assistant 消息 id。
event: start
data: {"trace_id": "...", "conversation_id": 21, "answer_kind": "grounded_content"}
event: think_delta
data: {"delta": "思考增量片段（仅展示层：不进引用复验、不作为工具结果、不并入 answer）"}
event: tool_status
data: {"tool": {"name": "search_content", "args_summary": "像素风 游戏", "hits": 3, "status": "success", "duration_ms": 42}}
event: delta
data: {"delta": "根据已检索内容..."}
event: citation
data: {"citation": {"content_id": 123, "title": "...", "zone": "original"}}
event: usage
data: {"usage": {"prompt_tokens": 812, "completion_tokens": 240}}
event: done
data: {"trace_id": "...", "conversation_id": 21, "message_id": 66, "answer_kind": "grounded_content", "answer": "最终裁决文本（no_evidence/degraded 时为空，客户端以 done 为准替换已流出正文）", "citations": [...], "tools": [...], "usage": {...}, "degraded": false}
event: error
data: {"error_code": "AGENT_PROVIDER_ERROR|AGENT_PROVIDER_TIMEOUT|STREAM_CANCELLED|AGENT_STORAGE_ERROR|AGENT_CONVERSATION_NOT_FOUND", "error_message": "安全文案，不含原始 Provider 错误"}
```

**v2 语义要点**：① `<think>` 思考块由 provider 层分流为 think_delta 转发（MiniMax 标签式 + OpenAI 兼容 `reasoning_content` 通道），落库为独立 phase 标记行（`tool_calls={"phase":"think"}`）供历史回放，答案行不含 think；② no_evidence/degraded 轮的正文可能已流出——done 是最终裁决（answer 置空、answer_kind 标注），客户端据此替换显示（与 A-05 输出事后审核不阻塞流式同理）；③ 工具步骤事件携带服务端派生的参数摘要（检索词/content_id）与命中数、耗时，不含原始参数 JSON；④ T19 行缓冲语义（前端逐段累积 + 流尾 flush）继续成立——delta 事件形状不变，仅从「一次性全文」变为「逐段多次」。

产品化目标不会新增让模型直接提交 draft/content/file ID 的写工具。内部 tool dispatcher、引用验证和限流语义见 §11.6、§11.9～§11.10；真实路由变更后由 doc-validator 刷新本节上方的自动路由表。

### 11.6 MVP Tool 白名单

Agent 在 Tool-Call 模式下只能调用以下内部工具，不可执行任意代码：

| Tool 名称 | 签名 | 说明 |
|---|---|---|
| `search_content` | `(query string, filters SearchFilters) → []ContentSummary` | 调用已有内容搜索逻辑，只返回 viewer 可见 published 内容 |
| `get_content_detail` | `(id int64) → ContentDetail` | 获取内容详情（含 attachments、tags） |
| `get_usage_guide` | `(content_id int64) → UsageGuide` | 获取内容类型与附件的使用说明 |
| `suggest_publish_metadata` | `() → PublishSuggestions` | 服务端从当前请求绑定、严格限长的 PublishFormSnapshot 取上下文；模型不传资源字段，用户确认后由普通发布 API 写入 |

上传格式检查与阿里云内容安全仍由用户已选择文件后的普通后端流程确定性触发，不注册为模型可自由选择的工具。标签候选包含在 `suggest_publish_metadata` 的建议结果中，最终写入仍由发布 API 校验和用户确认。
向量召回是 `search_content` 的服务端内部实现细节，不作为模型可直接选择的独立工具。

新工具须经过评审后加入白名单，**Agent 不可自定义或组合超出白名单的调用**。

### 11.7 向量化 Pipeline

> **现状（2026-09-05，A-03 已落地）**：本节原始的「title+description+tags 单向量 → content_embeddings」流程已被 chunk 级投影取代——发布/更新触发投影 worker：按 `rag.chunking` 切块 → **批量嵌入**（DashScope text-embedding-v4@1536，10 条/批，走 openai_compat embedding provider）→ 写 `rag_chunks` / `chunk_embeddings` / `index_projection_status`（双世代，迁移 071）。hybrid 开启时检索为 RRF（pg_jieba 词法主路 + pgvector 向量，`rag.hybrid.keyword_source: postgres`）；查询扩展与 qwen3-rerank 各自开关门控，三开关默认 off 由 A-04 消融数据定默认。向量化失败不阻断发布流程，失败时记录日志、稍后重试（语义保留）。旧单向量路径仅存于 legacy `content_embeddings`（推荐系统域）。选型与开关唯一权威：`docs/reference/agent-runtime-matrix.md`。

### 11.8 限流实现

```
Redis Key: agent:rl:{user_id}:{YYYY-MM-DD}
操作: Lua 原子预留 → 检查并递增；不得用分离 GET/INCR 绕过并发上限
过期: EXPIRE 86400（当日到期自动清零）
```

超限响应：`HTTP 429 { "code": "AGENT_RATE_LIMIT_EXCEEDED", "reset_at": "2026-04-16T00:00:00Z" }`

### 11.9 前端 Agent 组件

| 组件 | 位置 | 说明 |
|---|---|---|
| `AgentWorkspace.tsx` | 受保护路由 `/agent` | 顶部导航进入的独立全页工作台；`web_agent_enabled=false` 时隐藏入口并拒绝进入 |
| `UploadAssistPanel.tsx` | 发布页（publish/page.tsx）| 上传完成后显示「AI 自动填写」按钮，调用 `/agent/upload-assist` |
| `ComplianceCheckBadge.tsx` | 发布页确认区 | 展示合规检测结果（通过/风险/违规），支持流式进度 |
| `GlobalSearchInput.tsx` | Header 与搜索页（/search） | 仅提供关键词建议、历史、热搜和普通搜索，不提供 Agent 模式切换 |
| `UsageGuidePanel.tsx` | 内容详情页侧边 | 「AI 使用指导」折叠卡片，按需加载 |

SSE 流式渲染复用 `lib/useSSE.ts` hook。该 hook 使用 `fetch` + `ReadableStream` 以支持带 JSON body 的 POST 流式请求和 AbortController 取消，不使用原生 `EventSource`。

### 11.10 产品化回答与评测契约

- 流式事件统一为 v2 **八事件** registry：`start`、`think_delta`、`tool_status`、`delta`、`citation`、`usage`、`done`、`error`（与 §11.5 契约一致；唯一清单与兼容规则见 `docs/reference/agent-runtime-matrix.md` §5——前端忽略未知事件、无思考通道的 provider 其 think_delta 自然缺席、done 为终态裁决）；最终事件包含 `trace_id` 和 `degraded`。
- Citation 由后端内容摘要构建并在输出前再次执行共享可见性检查；模型提供的任意 URL 不能直接成为站内引用。
- Chat context 只接受 server-owned surface 枚举和可选资源 ID；title/type/visibility 均由服务端按 viewer 重载，客户端摘要不能进入 system context。
- `/agent/usage-guide/:id` 同步/流式路径与 tool registry 共用 viewer-aware resolver；普通用户 Agent 路由不得暴露会写审核记录的 `/agent/moderate/:id`。
- `answer_kind` 由服务端执行路径确定：chat/search/detail/usage 的自然语言结果必须有有效 citation，否则进入 no-evidence；publish suggestion 使用独立 typed DTO。
- UI 展示简短工具状态和引用卡片，不展示内部 prompt、工具原始参数或 chain-of-thought；引用卡片打开与推荐流共用的内容详情浮层，关闭后恢复会话滚动位置和触发点焦点。
- CI 使用 deterministic fake Provider 与固定 evaluation fixtures；真实 Provider 仅作为需要密钥的发布 smoke，不作为不稳定的 CI 判定器。
- `agent.web_agent_enabled` 的仓库默认值保持 `false`；真实 Provider、预算、限流、引用评测和浏览器证据全部通过后，生产 override 才可开启。

---
