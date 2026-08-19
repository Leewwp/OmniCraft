# OmniCraft Agent/RAG 简历与演示证据合同

> **创建日期**：2026-08-11
> **状态**：`accepted`（2026-08-11）。#32 仅被 T01–T09 阻塞；本文件不预填任何数字，只有真实运行测量后才能填写。
> **上游约束**：#30 resolution（三层证据分级）；#28（指标口径）；#31（实现/closure tickets）。
> **不得违反**：不得提前写吞吐、P95、召回率、成本节省或生产用户规模；不得把 Designed/Future option 写成已实现。

## 1. 证据晋升规则（由证据分级推导）

| 分级 | 证据要求 | 简历写法 |
|---|---|---|
| Implemented | 主线代码 + 默认/full-infra profile 可启动 + 自动化测试 + 关键指标（真实测量） + ≥1 个失败/降级演练 | 「实现/使用/构建」+ 真实指标数字 |
| Designed | ADR/契约/数据所有权/失败场景/降级恢复/选型比较/实施计划 | 「设计/规划/评估」，不写指标数字 |
| Future option | 仅触发条件与不采用原因 | 不写入已用技术栈；面试追问可讲 |

**晋升动作序列**（任何数字进入简历前必须满足）：
1. 对应能力在主线并有自动化测试；
2. 关键指标来自 T09 指标归档（真实 run，可复现命令记录在案）；
3. ≥1 个故障演练证据（截图/日志）在仓库或 evidence archive 可查；
4. 通过该能力所属实现 ticket 与 T09 closure 验收门；
5. 在本文件对应条目勾选并注明证据位置（commit/run id/截图路径）。

## 2. Demo 脚本模板（live demo，数字字段留空）

> 主演示：OmniCraft 站内单 Agent RAG。演示顺序与讲点（每步约 30–60 秒）：

```text
0. 开场（30s）：一句话定位 —— 模块化单体业务落地后，按故障隔离与扩缩容证据逐步拆异步 Worker；
   展示 docker-compose 拓扑（默认 profile + full-infra profile）。
1. 内容导入（30s）：已发布内容（标题/正文/标签/IP 归属）→ 结构化分块 → chunk 落库 → embedding；
   展示 chunk 表与 index_projection_status。
2. 建索引（30s）：OpenSearch 索引与 alias 世代；展示 index 健康与文档计数。
3. 正常问答与引用（1min）：/agent 工作台提问 → SSE 流式 → citation 卡片（content_id/version/chunk_key/source）
   → 点击引用打开内容详情浮层；展示引用与正文的一致性（groundedness）。
4. trace 查看（1min）：Jaeger UI 展示 HTTP → worker → DB → LLM 全链 span；SSE trace_id（OTel）与 HTTP trace 一致。
5. golden-set 报告（1min）：eval harness 输出 Recall@K/MRR/nDCG/citation precision/coverage/leak count；
   强调 visibility leak count = 0。
6. OpenSearch 故障降级（1min）：停 opensearch → 请求仍成功（PG keyword 降级，degraded 标记）；
   恢复后追平。
7. 重复事件幂等（30s）：重复投递同 event_id → 消费幂等，指标不重复。
8. DLQ replay（30s）：制造永久失败事件 → DLQ → replay → 收敛。
9. 索引 rebuild（30s）：full rebuild 世代切换，读侧不中断。
10. 收尾（30s）：面试官追问入口 —— 选型/失败/量化来源。
```

## 3. 指标采集清单（全部空位，T09 后填写）

| 指标 | 来源 | 填值 |
|---|---|---|
| Recall@K（K=10/20） | rag_eval harness | __ |
| MRR / nDCG@10 | rag_eval harness | __ |
| citation precision / coverage | rag_eval harness | __ |
| groundedness / answer relevance | rag_eval harness（judge + 确定性） | __ |
| visibility leak count | rag_eval harness | __（必须为 0） |
| P95 检索延迟 | harness 计时 / Prometheus | __ ms |
| SSE first-token latency | 应用日志 / harness | __ ms |
| token/request | usage 事件 | __ |
| OpenSearch/embedding 降级成功率 | 故障演练采集 | __ % |
| outbox pending oldest / relay 追平时间 | 队列指标 | __ s |
| consumer success / DLQ 数量 | Prometheus | __ / __ |
| 默认 profile 启动时间 / full-infra 启动时间 | 本地实测 | __ s / __ s |
| 验证脚本通过项 | verify-project.sh --full | __ 项 |

### T09 本地开发测量（2026-08-19）

以下数字来自真实本地运行，但不满足真实 provider、生产样本或 Jaeger full-infra 晋升条件；不得写成生产指标：

| 指标 | 本地测量 | 运行/限制 |
|---|---:|---|
| Recall@10 | 0.476 | 63-case hybrid；deterministic SHA-256 stand-in；34-content corpus |
| MRR | 0.365 | 同 run；keyword baseline 0.370，stand-in gate gap |
| nDCG@10 | 0.393 | 同 run |
| citation precision / coverage | 0.049 / 0.476 | retrieval oracle，不是生成答案质量 |
| groundedness / answer relevance | 未测量 | 未配置真实 LLM |
| visibility leak count | 0 | viewer-aware hybrid revalidation |
| P95 检索延迟 | 5.8 ms（最新；此前 55.7 ms） | 本地 OpenSearch yellow；deterministic stand-in |
| SSE first-token / token per request | 未测量 | 未配置真实 LLM |
| OpenSearch / embedding 降级 | 通过 / 通过 | 一次 stop/restart + 注入 provider error；不是总体成功率 |
| outbox/consumer/DLQ 指标 | 合同通过 | 复用 T03 本地演练；未生成生产规模率 |
| verify-project.sh --full | 73/73 | mocked browser contracts |

证据包与精确命令：`docs/working/2026-08-19-t144-rag-closure-evidence.md`。T09 关闭本地开发批次，但本节不勾选 Implemented 晋升；真实 embedding provider、LLM、生产样本和 Jaeger UI 证据恢复后重新运行。

## 4. 文案模板（三级）

- **Implemented（真实实现后可写，示例骨架，数字留空）**：
  > 基于 Go/Gin + PostgreSQL + Redis 构建内容社区后端，在模块化单体稳定后抽出独立 Worker；使用 Transactional Outbox、Redis Streams、幂等消费、重试与 DLQ 保障异步流程；使用 OpenSearch 关键词/过滤与 pgvector 语义检索做混合召回（RRF）；Agent 通过 SSE 输出带服务端可见性复核的站内引用；OpenTelemetry 全链追踪；golden-set 评测闭环。检索 Recall@K __、P95 __ms、泄漏 0。
- **Designed（现在可写，不加数字）**：
  > 设计过混合检索（RRF 基线与 alpha 对照）、站内引用溯源和评测闭环；评估过 Agent/Search 独立进程与 gRPC 边界（仅设计）。
- **Future option（只入追问，不入简历）**：多智能体对照实验、cross-encoder reranker、Kafka。

## 5. 面试追问清单（预演）

1. 为什么先保持模块化单体？什么证据触发拆 Worker / Agent/Search？
2. RRF 为什么比 alpha 加权稳？k 与 topK 如何定？
3. 为什么最小实现不强制 reranker？评测门是什么？
4. chunk 为什么绑定 content_version？引用如何回溯版本？
5. 为什么不向客户端暴露 visibility_checked？服务端如何保证越权引用不会进入 SSE？
6. OpenSearch 挂了检索怎么走？降级如何度量成功率？
7. Outbox 解决了什么双写问题？为什么仍可能重复投递？
8. 消费幂等在哪里做？Redis 幂等与 DB 幂等的取舍？
9. 索引怎么重建？alias 世代切换如何不中断读侧？
10. golden set 怎么构造？judge 自评偏差如何控制？
11. trace_id 如何贯穿 HTTP/队列/DB/LLM？采样策略？
12. 两节点 Research→Answer 为什么只是实验？对照指标是什么？
13. 这些数字从哪来？（必须能指到具体 run/归档，不能只答"测过"）

## 6. GitHub 依赖接线

- **#32 原生 `blocked_by` 指向 #136–#144（T01–T09）**；**#145（T10）不挂 #32**，不作为当前 Implemented 晋升门。
- #32 在 T09 证据包归档前保持 OPEN；不写完成状态。
