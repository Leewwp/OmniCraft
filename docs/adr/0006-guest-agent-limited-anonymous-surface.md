# 受限匿名 Agent 面：设备配额、三层防刷与授权边界

- **状态**：accepted
- **日期**：2026-10-09
- **决策所有者**：GitHub #854（访客落地批 T2）
- **关联**：ADR 0003（Agent 独立工作台）、ADR 0005（Agent/RAG 运行时边界）

## Context

Web Agent 的账号通道是受保护能力：每用户配额、账号会话归属（`agent_conversations.user_id` 引用 `users`，非游客行必须有账号）、经过邮箱验证的互动门与完整的工具面。匿名通道使用独立的签名设备归属，供访客体验后注册。匿名化带来三重约束：成本必须硬封顶（LLM 调用按量计费）、会话归属必须有显式数据模型（不能伪造归属）、授权面必须最小（匿名者不得触碰任何写路径或身份绑定路径）。

## Decision

1. **受限匿名体验**：游客获得公开只读问答——每设备累计三轮、永不周期补充，7 天会话保留，单设备并发上限 3。能力面只有站内公开检索/读取工具（`search_content` / `search_ips` / `get_content_detail` / `get_usage_guide`），声明（schema 面）与执行（runtime 面）经同一份白名单双重过滤；发布辅助、图片生成、MCP 桥接、上传协助、下载/签名、PAT/MCP 身份路径全部保持原授权，不因游客面放宽。伪造的模型 tool call 在执行侧被拒绝为未知工具。
2. **身份与配额分离**：设备身份是 HMAC-SHA256 签名的随机 256-bit cookie（HttpOnly / SameSite=Lax / HTTPS Secure，release 用 `__Host-` 前缀），存储与 Redis 键只携带 id 的 SHA-256。累计轮数计数无 TTL；签名 cookie 存在但计数键被逐（allkeys-lru）或 Redis 故障时**fail-closed**——绝不重建为满额，恢复只能来自可靠保留记录。清 cookie 重新建身份是已知且被接受的匿名软肋（上限：每次清空重得三轮）。
3. **三层防刷**：① per-IP `agent_guest` 令牌桶（复用 #729 原子算法）独立于通用浏览限流开关——`rate_limit.enabled` 与 `features.guest_rate_limit_enabled` 都关闭也不能令成本桶失效，显式置零的桶配置在结构校验与运行时两侧都拒绝；② per-device 累计轮数 + 并发上限在单次 Lua 原子操作中预留；③ 总闸 `features.guest_agent_enabled`（出厂 false，还需 `agent.web_agent_enabled`）覆盖直调 API、Header、落地页与工作台全部入口。生成路径的任何 Redis 故障 fail-closed，不继承公开 GET 令牌桶的可用性优先 fail-open。
4. **显式 guest 归属，不伪造所有权**：迁移 089 将 `user_id` 放宽为可空并新增 `is_guest` + `guest_device_key`，CHECK 约束令两种归属形态互斥——禁止 `user_id=0` 哨兵、禁止伪用户行；游客会话按设备键索引与隔离，跨设备读写都是 owner-scoped 404，过期（7 天自创建，读写均不延期）即时拒绝。历史清理走 worker 生命周期且永不触碰累计计数。
5. **登录墙转化**：登录请求继续原 auth/interaction 守卫，任何凭证（含无效 Bearer）在游客面都被拒绝而非降级；`/agent` 是双主体壳（其余 protected 路由不放宽），余量只在工作台输入框下沿展示（Header/落地页一律不露轮数）。用尽后的转化经既有登录浮窗只续做尚未执行的动作（草稿交接 consume-once），已消费的游客请求不重放、账号历史不迁移、登录即取消在途游客流。

## Consequences

- 游客第一轮的真实成本封顶为 `3 × (单轮 LLM 成本)`，横向扩张只能靠清 cookie，且受 per-IP 成本桶约束；这足以覆盖注册转化的体验预算，不需要验证码或设备指纹等更重的反刷设施。
- Redis 成为游客生成面的强依赖：allkeys-lru 部署下计数键可能被逐，被逐设备会被锁出（fail-closed 显示用尽）。这是有意选择——误放行一轮 LLM 的成本高于误锁一个匿名身份。
- 设备 cookie 格式或签名密钥轮换会使既有游客身份失效（需清 cookie 重建）；可接受，因身份不绑定任何可恢复资产。
- 匿名会话不参与自动标题、输出脱敏复用既有 A-05 路径；游客 trace 行不携带 user id。

## Revisit Triggers

- 游客滥用成本超过预算模型（per-IP 桶被分布式绕过）→ 引入验证码/proof-of-work 前置；
- 配额键被逐率显著（allkeys-lru 命中计数键）→ 为额度键启用 noeviction 专用池或持久化保留记录；
- 游客面需要内容型写操作（收藏/反馈）→ 必须重新立 ADR，本决策的只读边界不自动延展。
