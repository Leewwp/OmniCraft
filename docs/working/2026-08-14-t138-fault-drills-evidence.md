# #138 (T03) 故障演练证据 — relay 暂停/幂等/DLQ replay/独立 Worker

- **创建日期**: 2026-08-14
- **预计失效日期**: 2026-10-14
- **关联**: `docs/superpowers/plans/2026-08-11-omnicraft-reliable-async-observability-foundation-roadmap.md` §T03（GitHub issue #138）
- **环境**: worktree `OmniCraft-wt-t138`（分支 `async/t138`）；本机 Docker compose 的 PostgreSQL（16, pgvector）+ 独立演练 Redis（`omnicraft-drill-redis` :6380，与开发 Redis 隔离）；演练专用库 `omnicraft_drill_t138`（全量迁移 001~072 已应用）；真实编译产物 `cmd/server`（:8080）与 `cmd/worker` 本地进程。
- **演练发现并修复一个真实缺陷**: 见文末「演练中发现的问题」。

## 0. 基线

- 四场演练均从 `FLUSHDB`（演练 Redis）+ `TRUNCATE outbox_events, inbox_consumers[, notifications] RESTART IDENTITY` 的干净状态开始。
- 数据流全链路：`outbox_events`（DB 行）→ relay（`cmd/worker` 内轮询）→ `omnicraft:content.published`（Redis Stream）→ indexer 消费者（同事务 inbox 幂等）→ 副作用（`content.embedding` 作业发布 / embedding 行删除）。
- 消费者组: `omnicraft-indexer`（content.published/updated/banned/deleted）、`omnicraft-notification` 等；`worker.concurrency=2`（每 topic 2 个消费者 goroutine）。
- 关键可观测点：`outbox_events.status/attempts/next_attempt_at`、`omnicraft:*` Stream `XLEN`、`inbox_consumers`（UNIQUE(consumer_group,event_id)）、`admin_audit_logs`。

## 1. 演练①：停 redis → relay 暂停不丢事件 → 恢复后追平

**操作**：
1. Redis 在线时向 outbox 写 2 条 `content.published`（content 1001/1002 已存在）→ relay 正常投递。
2. `docker stop omnicraft-drill-redis`，Redis 停机期间再向 outbox 写 2 条（content 1003/1004）。
3. `docker start omnicraft-drill-redis`，等待 backoff（`retry_backoff_sec=[10,60,300]`）到期后 relay 追平。

**观察结果**：

| 阶段 | outbox pending | outbox sent | stream `content.published` | inbox 行 | 说明 |
|------|---------------|------------|---------------------------|---------|------|
| 基线 | 0 | 0 | 0 | 0 | |
| Redis 在线投递后 | 0 | 2 | 2 | 2 | 全链路 2 条消费完成 |
| Redis 停机 6s 后 | 2 | 2 | 2 | 2 | 新事件保持 pending，attempts=1、`next_attempt_at` 均在未来 |
| Redis 恢复 ~25s 后 | 0 | 4 | 4 | 4 | 追平：补投 2 条并消费完成 |

停机期间 relay 日志（投递失败 → 指数退避，事件不丢失）：

```json
{"msg":"relay: delivery failed, scheduling backoff retry","event_id":3,"event_type":"content.published","attempt":0,"error":"xadd to omnicraft:content.published: dial tcp [::1]:6380: connect: connection refused"}
```

恢复后 relay 日志（追平投递）：

```json
{"msg":"relay_worker: delivered outbox events","count":2}
```

**结论**：relay 在 Redis 不可用时暂停（不误标 sent、不丢事件，事件带退避驻留 outbox）；Redis 恢复后自动追平，含消费端收尾。✅

## 2. 演练②：手动重复投递同 event_id → 消费幂等（副作用只发生一次）

**操作**：
1. 写 1 条 `content.published`（content 2001 存在），relay 自然投递（event_id=1，outbox 行 id）。
2. 手动 `XADD` 同一条 envelope（相同 `event_id`，新 stream id）到 `omnicraft:content.published` 共 3 次，模拟 relay「已 XAdd 未 MarkSent 崩溃 → 重投」窗口（at-least-once 允许重复）。

**观察结果**：
- 投递共 3 次，stream 中 3 条消息的 `event_id` 全部为 `1`（`"event_id":1` × 3）。
- `inbox_consumers` 仅 1 行：`omnicraft-indexer | 1`。
- 副作用仅 1 次：`omnicraft:content.embedding` 作业仅 1 条（`embed_jobs=1`）。
- indexer 对重复投递的跳过日志：

```json
{"msg":"indexer: duplicate delivery skipped","event_id":1,"event_type":"content.published"}
```

**结论**：重复投递同一 `event_id`（无论同 stream id 重投还是新 stream id 重发）均被 `inbox_consumers` UNIQUE 守卫跳过，业务副作用只执行一次。✅

## 3. 演练③：永久失败事件 → DLQ → 修复后 replay 成功（含审计可见）

**操作**：
1. 写 1 条指向不存在内容的 `content.published`（aggregate_id=999999）→ relay 投递 → indexer `record not found` → 重试（退避 10s/60s/300s，max_attempts=3）→ 进入 `omnicraft:dead-letter`。
2. 修复根因：插入 content 999999 行。
3. `POST /api/v1/admin/queue/dlq/1786675617201-0/replay`（admin JWT + CSRF）→ 200。
4. 验证 replay 后消费成功与审计行。

**观察结果**：
- DLQ 条目（带 `consumer_group` 字段，供 replay 分诊）：

```json
{"attempts":3,"consumer_group":"omnicraft-indexer","error":"indexer: load content 999999: record not found","original_id":"1786675246366-0","original_topic":"content.published","payload":"{\"event_id\":1,...,\"aggregate_id\":999999,...}"}
```

- replay 响应 `{"message":"dlq entry replayed","entry":{"id":"1786675617201-0","original_topic":"content.published","original_id":"1786675246366-0","consumer_group":"omnicraft-indexer",...}}`。
- replay 后：`content.published` stream_len=2（原投递+重投）、`inbox_consumers`=1、embedding 作业=1；indexer 日志 `"indexer: consuming content event","event_id":1,...,"content_id":999999`。
- 审计日志（操作者/时间/条目 ID 全齐）：

```
 action    | target_id         | admin_user_id | result  | created_at
 dlq_replay| 1786675617201-0   | 1 (drilladmin)| success | 2026-08-14 02:47:37.982273+00
```

- 审计 metadata 保留原始主题与消息 id：`{"original_id": "1786675246366-0", "original_topic": "content.published"}`。
- 反路径：不存在的 DLQ 条目 → `404 {"code":"DLQ_ENTRY_NOT_FOUND"}`，并落 `dlq_replay ... result=failed` 审计行；非法 ID → `400 {"code":"INVALID_ID"}` + failed 审计行。401/403（匿名/非 admin）由 `admin_dlq_replay_test.go` 覆盖（本演练为 404/400/成功三态现场证据）。

**结论**：永久失败事件经重试耗尽进入 DLQ（带 consumer_group）；根因修复后 replay 端点将其重新投递并消费成功；正/反路径均留下审计记录。✅

## 4. 演练④：backend 停 / worker 独立存活（消费不中断），反之亦然

**Phase A — backend 停，worker 独立存活**：
- `kill <backend pid>` 确认进程停止（worker 不受影响，继续运行）。
- 直接 `XADD` 1 条 `notification.create`（channel=system）到 stream。
- 结果：backend 停机期间 `notifications`=3（含此前 2 条）、`inbox_consumers`=3 —— worker 独立完成消费，消费不中断。✅

**Phase B — worker 停，backend 独立存活**：
- 停 worker，backend 继续服务（`/healthz` → `{"service":"omnicraft-backend","status":"ok"}`）。
- `XADD` 第 2 条 `notification.create` → 3s 后 `notifications` 不增长（消息留在 stream pending）。
- 重启 worker → 数秒内 backlog 被消费：`notifications`=2、`inbox_consumers`=2、`XPENDING`=0。✅

**附加观察（事务性回滚）**：Phase A 首次尝试使用非法 channel `inapp`（违反 `notifications_channel_check`）时，worker 正确消费消息但副作用失败 —— `inbox_consumers` 与 `notifications` 均不落行（同一事务回滚），消息保持 pending 并按退避重试 —— 与「失败副作用 + inbox 完成记录同事务提交/回滚」合同一致。

## 5. 演练中发现的问题（已修复）

**问题**：`Subscribe` 只在启动时调用一次 `ensureGroup`；若消费者运行期间其 consumer group 被外部清除（FLUSHDB、运维清理、TTL 过期），`XREADGROUP` 将永远返回 `NOGROUP`，消费者进入「报错-空转」死循环，无法恢复。演练①的 `FLUSHDB` 操作暴露此缺陷（日志刷屏 `NOGROUP No such key ...`，后续投递无人消费）。

**修复**（`backend/internal/pkg/queue/redis_stream.go`）：`XREADGROUP` 返回 `NOGROUP` 时重新 `ensureGroup`（`XGroupCreateMkStream` 从流头开始，恢复后回补缺失期间的全部消息，保持 at-least-once 不丢消息），而不是永久报错空转。

**TDD 证据**：
- 先写失败测试 `TestRedisStreamBrokerRecreatesDeletedGroup`（queue_test.go）：FLUSHDB 后发布的消息永远不被处理 → RED。
- 修复后：组重建、消息恢复消费 → GREEN（`go test ./internal/pkg/queue/ -run TestRedisStreamBrokerRecreatesDeletedGroup` PASS）。

## 6. 演练环境备注

- 演练库 `omnicraft_drill_t138` 与演练 Redis `omnicraft-drill-redis`(:6380) 均为隔离环境，演练结束后保留在 Docker/Postgres 中供复验，不影响开发库 `omnicraft`。
- 演练期间清理了上一轮 agent 会话遗留的僵尸进程（`/tmp/t138-drill/omnicraft-server|worker`、collab-t8 遗留 `go run cmd/server`），它们分别占用 8080/9091 端口并干扰演练；非本任务产物。
- 演练日志原件（含全部中间过程）归档于 `docs/working/2026-08-14-t138-fault-drills-raw.txt`（本文件为提炼版）。
