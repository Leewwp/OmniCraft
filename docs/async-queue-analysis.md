# OmniCraft 异步任务与消息队列引入分析

> 生成日期：2026-05-21 | 分析范围：backend/internal 全部异步任务 | 供后续可行性评审使用

---

## 一、核心问题总结

### 问题 1：审核阻塞用户请求（严重）

**位置**：`backend/internal/handler/content.go` — 内容发布 handler → `service/review_service.go:SubmitForAIReview()`

**现状**：发布内容的 HTTP handler 在请求路径内**同步调用**阿里云内容安全 API（文本扫描 + 图片扫描 + 视频异步扫描提交），扫描全部完成后才返回 201。

**影响**：
- 用户点击"发布"后等待时间 = 审核耗时（可能 2-5 秒）
- 阿里云 API 超时或慢响应直接拖死请求
- 无法批量重试或降级

### 问题 2：Fire-and-Forget 任务无持久化（中等）

以下 5 个任务使用 `go func()` 或 `recovery.GoSafe()` 即发即忘，**进程崩溃则任务丢失**：

| 位置 | 任务 | 触发频率 |
|------|------|---------|
| `service/notification_service.go:19` | 通知写入 DB | 高（每次评论/点赞/关注） |
| `handler/content.go:473` | 下载计数写入 Redis | 中 |
| `service/agent_service.go:392` | 内容 Embedding 向量生成 | 中 |
| `service/review_service.go:98` | 视频回调后处理 | 低（阿里云异步回调） |
| `service/review_service.go:247` | ensureJudgeCase | 低 |

### 问题 3：无重试机制（中等）

所有异步任务失败后仅 `slog.Error()` 记录日志，无自动重试。阿里云视频审核回调如果 handler 处理失败，审核结果永不到达。

### 问题 4：无可观测性（低）

无法回答：当前积压多少通知？embedding 生成平均耗时？审核失败率？队列深度？

### 问题 5：定时任务与事件驱动混用（低）

排行计算、计数刷库等正确使用 ticker 模式。通知/embedding/审核本质上应是事件驱动的"任务"，却被实现为 request 内 goroutine。两类机制边界模糊。

---

## 二、现有架构基础（可复用部分）

| 组件 | 作用 | 可复用度 |
|------|------|---------|
| `recovery.GoSafe()` (`pkg/recovery/recovery.go`) | panic 恢复包装器 | 直接复用到 worker |
| `pkg/scheduler/*.go` (4 个 scheduler) | ticker 驱动的后台任务模板 | 架构参考，ticker→consumer 转换 |
| `service/hot_rank_service.go:Run()` | 带 TryLock 防重叠的后台循环 | 模式直接应用于 worker 主循环 |
| Redis 7 实例 | 已有连接、配置、健康检查 | **零新增基础设施** |

---

## 三、可改动节点清单

### A 类：必须改造（从同步拆离为异步）

| # | 当前调用点 | 改造后 | 优先级 |
|---|-----------|--------|--------|
| A1 | `handler/content.go` PublishContent → 同步调用 `SubmitForAIReview` | `handler` 投递消息到 queue → 立即返回 201；独立 worker 消费审核 | **高** |
| A2 | `handler/internal.go` AI 回调 → 同步 `ProcessAICallback` | 回调 handler 投递消息 → 返回 200 给阿里云；worker 异步处理 | 中 |

### B 类：加固改造（goroutine 替换为消息队列）

| # | 当前调用点 | 改造后 | 优先级 |
|---|-----------|--------|--------|
| B1 | `service/notification_service.go:19` `recovery.GoSafe(func(){...})` | `queue.Publish("notification.create", payload)` | 中 |
| B2 | `handler/content.go:473` `go func(){ rdb.ZIncrBy(...) }()` | `queue.Publish("count.download", payload)` | 低 |
| B3 | `service/agent_service.go:392` `recovery.GoSafe(func(){...})` | `queue.Publish("content.embedding", payload)` | 低 |

### C 类：保留不动

| # | 任务 | 原因 |
|---|------|------|
| C1 | `scheduler/view_count_sync.go` | ticker 批量刷库是正确的模式，不需要改 |
| C2 | `scheduler/download_count_sync.go` | 同上 |
| C3 | `scheduler/judge_question_sync.go` | 周级定时任务，不需要实时性 |
| C4 | `scheduler/tag_usage_sync.go` | 同上 |
| C5 | `service/hot_rank_service.go` | 定时计算 + TryLock 防重叠，不需要改成事件驱动 |

---

## 四、推荐方案：Redis Streams 轻量任务队列

### 4.1 选型依据

| 候选 | 延迟 | 持久化 | 运维成本 | 当前适用度 |
|------|------|--------|---------|-----------|
| **Redis Streams** | < 1ms | ✓ (RDB/AOF) | 零（已有 Redis） | **推荐 P0→P1** |
| RabbitMQ | < 1ms | ✓ | +1 容器，+新监控 | P2 备选 |
| NATS JetStream | < 1ms | ✓ | +1 容器 | P2 备选 |
| Kafka | 10-100ms | ✓ | +1-3 容器，运维重 | 远期预留 |
| Go channel | < 1μs | ✗ | 零 | 当前方案，不持久 |

### 4.2 新增代码结构

```
backend/internal/pkg/queue/
├── queue.go              # Message / Producer / Consumer / Broker 接口定义
├── redis_stream.go       # Redis Streams 实现（Broker）
├── dlq.go               # 死信队列（失败消息归档，保留 7 天可重放）
└── observability.go     # 队列深度 / 消费速率 / 失败计数的 slog 日志打点

backend/internal/worker/
├── review_worker.go      # 消费 content.review → 调用 ReviewService
├── notification_worker.go # 消费 notification.create → 写通知 DB
└── embedding_worker.go   # 消费 content.embedding → 调用 LLM Embedding
```

### 4.3 Topic 与 Worker 映射

```
Redis Stream: omnicraft:content.review       → Worker: review        (并发=2)
Redis Stream: omnicraft:notification.create  → Worker: notification  (并发=1)
Redis Stream: omnicraft:content.embedding    → Worker: embedding     (并发=1)
Redis Stream: omnicraft:count.download       → Worker: count         (并发=1)
Redis Stream: omnicraft:dead-letter          → 无消费者，手动排查后重放
```

### 4.4 配置扩展 (config.yaml)

```yaml
queue:
  max_attempts: 3
  retry_backoff_sec: [10, 60, 300]
  dlq_ttl_hours: 168
  worker_count:
    content_review: 2
    notification: 1
    embedding: 1
    count: 1
```

### 4.5 main.go 改动概要

```go
// 新增
broker := queue.NewRedisStreamBroker(rdb)
reviewWorker := worker.NewReviewWorker(reviewSvc, cfg)
broker.Subscribe(ctx, "content.review", reviewWorker.Handle)
// ... 其他 worker 同理

// 注入 broker 到 container
ctr := container.NewContainer(db, rdb, broker, cfg)  // 多一个参数

// handler 从 broker.Publish() 替代 go func()
```

### 4.6 关键接口（解耦实现）

```go
// Producer — handler 层只依赖这个
type Producer interface {
    Publish(ctx context.Context, topic string, payload any) error
}

// Consumer — main.go 注册 worker
type Consumer interface {
    Subscribe(ctx context.Context, topic string, group string, handler Handler) error
}

// Handler — worker 实现，返回 error 决定是否重试
type Handler func(ctx context.Context, msg Message) error
```

### 4.7 重试策略

```
1st 失败 → 等待 10s → 重试 (attempts=1)
2nd 失败 → 等待 60s → 重试 (attempts=2)
3rd 失败 → 等待 300s → 重试 (attempts=3)
3次后仍失败 → 移入 dead-letter stream，日志报警
```

### 4.8 优雅关机

worker 在收到 SIGTERM 时：
1. 停止接收新消息 (`XREADGROUP` 退出循环)
2. 等待正在处理的消息完成（`sync.WaitGroup`）
3. 超时 (15s) 后强制退出但保留未 ACK 消息（消费者组 rebalance 后由其他实例接管）

---

## 五、未来 P2+ 演进路线

### 5.1 搜索增强：Elasticsearch 替代 tsvector

**触发条件**：用户量 > 10000，搜索成为瓶颈

| 当前 (PostgreSQL tsvector) | 未来 (Elasticsearch) |
|---------------------------|---------------------|
| `jiebacfg` 字典（索引侧），查询侧仍 `simple` 待统一（见 DEC-001） | IK Analyzer 分词 |
| GIN 索引 (jieba 分词)，单表内搜索 | 倒排索引 + 分布式搜索 |
| 无聚合分析 | 搜索热词、聚合统计、搜索建议 |
| 内容表增加负担 | 读写分离，搜索独立扩展 |

### 5.2 消息队列升级：RabbitMQ / NATS JetStream

**触发条件**：Redis 内存成为瓶颈（Stream 消息积压 > 100万条）或需要更丰富的路由策略

Redis Streams → RabbitMQ 迁移要点：
- 接口抽象层 (`queue.Broker`) 保证 handler 代码**零改动**
- 仅需替换 `redis_stream.go` 实现
- RabbitMQ 额外功能：TLS 原生支持、管理面板、更成熟的集群

### 5.3 向量检索：专用向量数据库

**触发条件**：`content_embeddings` 突破百万级，pgvector IVFFlat 召回速度下降

候选：Milvus（功能最全）、Qdrant（Rust 实现，单机性能好）、Weaviate（自带 embedding 模块）

### 5.4 时间线：什么时候做什么

```
当前 (P0/P1, <1000 人)
├── ✅ 引入 Redis Streams 解决审核阻塞 + 任务持久化
└── ✅ Worker 模式替换即发即忘 goroutine

P1 中后期 (1000-10000 人)
├── 🔲 如果搜索质量不够 → 引入 Elasticsearch
└── 🔲 如果 Redis 内存吃紧 → Redis Cluster 或升级到 RabbitMQ

P2 (>10000 人)
├── 🔲 迁移到 K8s，worker 天然支持水平扩展
├── 🔲 如果 pgvector 慢 → 引入专用向量 DB
└── 🔲 如果消息量巨大 → Kafka 做事件溯源
```

---

## 六、风险与注意事项

1. **Redis 内存**：Stream 消息会持久化直到被 ACK。如果消费者挂掉，消息堆积会吃内存。需要监控 `XLEN` 并设 `MAXLEN` 上限（如保留最近 10 万条）。

2. **消息幂等**：审核任务可能被重试导致重复扫描。worker 处理逻辑需要幂等——当前 `recordAIReview` 写入不检查重复，需加 `UNIQUE(target_type, target_id, provider, result)` 或先查后写。

3. **消息顺序**：同一内容的多条消息（审核→通知→embedding）不需要严格顺序，各自独立消费即可。

4. **测试**：集成测试需要用 `miniredis` 或真实 Redis 验证整个链路：Publish → Worker 消费 → DB 状态变更。

5. **向后兼容**：改造期间，`Publish` 接口加一个 `queue_enabled` feature flag，默认 off。验证通过后 on，完全稳后删旧代码。
