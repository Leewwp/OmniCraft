# Agent Answer Eval 实测证据（groundedness / answer relevance / first-token / token usage）

> Created: 2026-08-29
> **预计失效日期**: 2026-10-29
> Scope: local interview evidence; not a production release artifact

## 结论

在冻结的 current-v1 语料上，用真实 MiniMax-M3（chat）+ `embo-01`（query embedding）+ OpenSearch/pgvector 混合检索，把 63 条 golden case 全部跑过真实 AgentService 工具循环，完成四个此前缺失的测量：

| 指标 | 结果 |
|---|---|
| answered | **55 / 63**（8 条 no_evidence，0 降级，0 provider 错误） |
| SSE first-token latency | **P50 2078 ms / P95 8259 ms**（min 586 / max 18947） |
| 整轮耗时（含工具循环） | P50 13026 ms / P95 32155 ms（min 3326 / max 44008） |
| token usage | prompt 总计 **143,744**、completion 总计 **33,705**，平均 **3226 tokens/答案**（prompt 794-5249，completion 28-1200） |
| groundedness（确定性逐字包含代理） | 均值 **0.005**（max 0.125） |
| answer relevance（词面重叠代理） | 均值 **0.035** |
| 直接作答率（未检索就回答） | **25.7%**（19/74 次尝试） |
| 引用 | answered case 平均 **4.2 条/答案**（1-5，上限 5），全部经服务端 citation 复核；231 个证据块全部经 viewer 可见性 seam 解析成功 |

## Provenance（与 08-26 差分评测同一冻结语料）

- Dataset checksum：`sha256:e759d965ae70cb2b…`（与 `2026-08-26-rag-real-minimax-differential-evidence.md` 的 golden-set identity 一致）
- Corpus identity：`sha256:240b3bac94916e79…`（current-v1，169 contents/chunks，generation 2）
- 检索：`hybrid-rrf-pg-fallback-v1`，BM25=OpenSearch、向量=pgvector（embo-01 1536 维），finalTopK=10，RRF K=60
- 隔离评测库：`omnicraft_test_minimax_20260822`；conversation/消息落库该库（74 次尝试 = 74 个会话）

因此本运行的检索层、引用复核与 08-26 的 K10/K20 差分评测**直接可比**，三层证据（检索质量 → 异步链路 → 答案质量）闭合在同一语料上。

## 方法（`TestAgentAnswerEval`，env 门控）

- 入口：`backend/internal/service/rag_eval/answer_eval_test.go`；评分/报告：`answer_eval.go`（复用既有 `Groundedness`/`AnswerRelevance`/`P95`，`runner_test` 锁定的 retrieval-only nil 契约不变）。
- 每个 case：真实 provider 流式调用 → AgentService 服务端工具循环（4 个固定只读工具）→ `done` 事件收集答案/引用/usage → 引用经 `LoadVisibleCitationTruth`（与线上相同的可见性 + 当前世代谓词）解析为证据全文 → 计算确定性指标。
- **first-token 在 provider seam 测量**：`firstTokenMeasuringProvider` 包装 `llm.LLMProvider.ChatStream`，记录首个 content delta 时间戳——服务层的 `delta` 事件是整轮结束才发的单条事件，不能代表首 token。
- **受限重试与诚实计数**：MiniMax-M3 在生产极简 system prompt（仅 `[OmniCraft Agent Context] surface=global`）下是否调用检索工具是概率性的（见下文发现）。每 case 最多尝试 2 次（`OMNICRAFT_AGENT_ANSWER_MAX_ATTEMPTS` 可调），直接作答的尝试计入 `direct_answers`/`direct_answer_rate`，不隐藏也不粉饰。
- **聊天身份**：`ChatStream` 落库带 users 外键，匿名 viewer 0 无法落库。runner 自动选择**不拥有任何私有语料内容的种子用户**（本次 user 2）——其检索可见范围与 golden case 的匿名 viewer 完全等价（私有内容对两者都不可见），元数据记录 `chat_viewer_user_id`。
- 运行进程内调用（非 HTTP）； Jaeger 认证 HTTP 链路证据（`2026-08-27-jaeger-end-to-end-evidence.md`）已单独证明 HTTP→DB→MiniMax→SSE 同路径可用。

## 运行中发现并修复的两个产品缺陷

1. **MiniMax 流式响应不返回 usage**（对照 curl 实测确认）：即使 `stream_options.include_usage=true`，usage 也随末块（空 choices）返回，但 `MiniMaxProvider.ChatStream` 在 finish_reason 块处提前 return，读不到末块；且空 choices 的 usage 块会在 choices 守卫处被丢弃。修复：`openAIRequest` 增加 `stream_options`（omitempty，其他 provider 请求体不变，仅 MiniMax 流式启用）；解析循环先处理 usage 块、finish 后继续扫描到 EOF。修复后 usage 正常（`llm` 包 27 项测试通过）。
2. **MiniMax-M3 工具调用合规性是概率性的**：探针（`cmd/answer-probe`，保留作诊断工具）显示同一查询在极简 prompt 下约一半概率直接作答而非调用 `search_content`，与 max_tokens 上限无关（1200/2048 均出现两种行为）。生产 prompt 无 grounding 指令（#37 时代即如此，agent-smoke 的强指令是 smoke 自带）。这正是 answer-eval 需要受限重试与 direct_answer_rate 口径的原因，也是后续产品改进（低温度/更强 goal prompt）的依据。

## 指标解读边界

- **groundedness/relevance 代理饱和于低值（0.005/0.035）是预期结果**：两者是确定性逐字包含/词面重叠代理，`metrics.go` 注释明确"judge-based scoring is layered on top in answer-producing runs"。MiniMax-M3 以转述而非摘录方式作答，逐字包含率趋零不代表引用错误——引用正确性由服务端 citation 复核证明：引用由服务端经 citation-truth seam 生成（`agent_tools.go`），本评测将 231/231 条引用经同一 seam（viewer 可见性 + 当前世代谓词）重新解析成功，加上检索层 0 visibility leak。本运行的价值是把这两个字段从"永远 null"变成"有数字、有分母、可复现"，并为 judge 层建立基线。
- **completion 触顶**：观察到 completion=1200（= `agent.max_output_tokens` 上限）的答案，说明推理模型 think+答案可触及上限，建议为推理模型预留更高 max_output_tokens（产品改进候选，未实施）。
- 本地证据，无生产流量/SLA/质量结论；answer 文本为模型输出，不含用户个人信息。

## 复现

```bash
cd backend
set -a; source ../.env; set +a   # AGENT_LLM_* / AGENT_EMBEDDING_* / RAG_INDEX_EMBEDDING_MODEL（MiniMax 真实凭据）
export OMNICRAFT_RAG_EVAL_DSN='host=127.0.0.1 port=5432 user=omnicraft password=omnicraft dbname=omnicraft_test_minimax_20260822 sslmode=disable'
export OMNICRAFT_RAG_EMBEDDING_MODE=real
export OMNICRAFT_RAG_OPENSEARCH_URL=http://127.0.0.1:9200
export OMNICRAFT_AGENT_ANSWER_EVAL=1
export OMNICRAFT_AGENT_ANSWER_REPORT=../docs/working/2026-08-29-agent-answer-eval-real.json
export OMNICRAFT_AGENT_ANSWER_GIT_COMMIT=$(git rev-parse HEAD)
go test ./internal/service/rag_eval -run TestAgentAnswerEval -v -timeout 90m
```

CI 不受影响：未设 `OMNICRAFT_AGENT_ANSWER_EVAL` 时该测试跳过；新增单元测试（句子切分/评分/汇总分母/落盘脱敏契约）始终运行。

## 本次变更文件

- 新增 `backend/internal/service/rag_eval/answer_eval.go`（评分/汇总/脱敏落盘）
- 新增 `backend/internal/service/rag_eval/answer_eval_test.go`（env 门控 runner + 受限重试 + 等价身份解析）
- 新增 `backend/internal/service/rag_eval/answer_eval_report_test.go`（常驻单测）
- 新增 `backend/cmd/answer-probe/main.go`（工具调用合规性诊断，手动运行）
- 修改 `backend/internal/pkg/llm/openai_compat.go`（`stream_options` 请求字段，omitempty）
- 修改 `backend/internal/pkg/llm/minimax.go`（流式启用 include_usage；usage 块解析与 finish 后继续扫描）
- 产物 `docs/working/2026-08-29-agent-answer-eval-real.json`（schema `agent-answer-eval-v1`，63 case 全量明细）
