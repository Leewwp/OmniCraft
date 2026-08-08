# 阿里云内容安全扫描结果回调修复设计（Spec 草稿）

> **创建日期**: 2026-08-08
> **预计失效日期**: 2026-10-08
> **状态**: 待审核（grilling 会话产物，用户审核后决定是否正式化为 `docs/superpowers/specs/` 并实施）
> **调研依据**: `docs/working/2026-08-08-aliyun-content-safety-callback-research.md`
> **术语**: 扫描结果回调（阿里云入站通知）≠ 审核结果处理（应用内部流程），见 `CONTEXT.md` Language 节

---

## Problem Statement

视频内容发布后，AI 审核结果从未真实闭环：

1. **回调契约错误**：应用自定义 JSON 契约接收阿里云回调，而阿里云实际推送 `application/x-www-form-urlencoded`（`checksum` + `content`）。回调即使送达也会解析失败；且端点无任何来源认证（任何人可伪造 `result: block` 封禁他人内容，或伪造 `pass` 放行违规内容）。
2. **提交侧参数缺失**：`VideoModeration` 请求只传 `url` + `callback`，未传官方要求的 `seed`（使用 callback 时必填），回调大概率从未被投递；也未传 `dataId`，回调即使到达也无法关联回业务对象。
3. **无轮询兜底**：从不调用 `VideoModerationResult`，真实结果永不落地。结果：所有视频永久卡在 `under_review` 等人工判官——违规视频不会被 AI 封禁，合规视频不会自动放行。
4. **重复回调惩罚重复执行**：`ProcessAICallback` 对 block 结果每次重复扣信誉分、重复触发发布冻结（阿里云最多重试 16 次投递）。
5. **错误部署阻塞**：`GREEN_CALLBACK_ALLOWED_IPS` 被设为 release 必需且文档标注"阿里云控制台获取"，但官方从未发布回调来源网段（`114.115.0.0/16` 无官方依据），生产 .env 留空导致 release gate 拒绝启动（#92 收口阻塞项，前提已被调研证伪）。

## Solution

把视频审核结果获取链改为阿里云官方原生契约：

- 提交侧：`VideoModeration` 补传 `seed`（签名种子）与 `dataId`（业务标识 `content:<id>`），cryptType 保持默认 SHA256。
- 入站侧：`/api/v1/internal/ai-callback` 改为解析 `application/x-www-form-urlencoded`，校验 `checksum = SHA256(uid + seed + content)`（uid 为阿里云账号 UID），解析 `content` JSON 还原 `dataId` 与审核结果，复用现有审核结果处理流程。
- 认证：checksum 为唯一主认证；删除 `GREEN_CALLBACK_ALLOWED_IPS` 全链路（代码、配置、release gate、schema、文档、脚本），不做 IP 白名单。
- 幂等：`ai_review_records` 新增 `provider_task_id` 列与唯一索引，重复回调（同 taskId）直接忽略，不重复插入记录、不重复执行信誉惩罚。
- 配置：新增 `green.seed`（应用生成，release 必需）与 `green.uid`（阿里云账号 UID，控制台右上角账号信息获取，release 必需）。

## User Stories

1. 作为发布视频内容的创作者，我希望视频审核结果真实落地，合规视频能自动变为已发布状态，而不是永久卡在审核中。
2. 作为发布违规视频的创作者，我希望内容被 AI 审核封禁时信誉惩罚只执行一次，而不是因重复回调被重复扣分/重复冻结。
3. 作为平台运营，我希望回调端点只能被真正的阿里云内容安全服务触发，伪造回调无法封禁或放行任何内容。
4. 作为部署方，我希望 release 配置校验不依赖任何"阿里云回调来源网段"这类不存在的外部输入，部署不再被此阻塞。
5. 作为开发 Agent，我希望回调契约与阿里云官方文档一致，调用链（提交→扫描→回调→落库→状态变更）可以端到端验证。
6. 作为发布评论或讨论的用户，我希望违规文案在发布时被直接拒绝，而不是事后靠举报处理。

## Implementation Decisions

1. **提交侧参数**（`aliyun.GreenClient.VideoAsyncScan`）：`ServiceParameters` 增加 `seed` 与 `dataId`（`content:<target_id>`）；`callback` 保留；cryptType 不传（默认 SHA256）。`dataId` 格式 `content:<id>`，字符集仅安全字符。
2. **入站契约**（`AICallback` handler）：
   - 解析 `application/x-www-form-urlencoded` 表单，字段 `checksum`、`content`（JSON 字符串）；
   - 校验失败 → `403 FORBIDDEN`（不泄露细节，保留阿里云重试语义：非 200 视为失败）；
   - 校验通过 → 解析 `content` JSON 中的 `dataId`（还原 target_type/target_id）、审核结果（复用现有 suggestion/risk 归一化逻辑取最严）与 `taskId`；
   - 内部仍构造现有 `AICallbackInput` 结构，同步或经队列走审核结果处理，队列分支保留（`content.review` / `ip.review` topic 语义不变）；
   - 删除 `isAllowedSourceIP` 及默认 `127.0.0.1` 放行逻辑。
3. **幂等**：新迁移 `068_add_ai_review_records_task_id.sql`：`ai_review_records` 加 `provider_task_id`（`size:128`，可空）+ 唯一索引；`ProcessAICallback` 收到带 `taskId` 的回调时先查重，已存在则直接返回成功（HTTP 200），不重复落库与惩罚。
4. **配置与 gate**：
   - 删除 `green.callback_allowed_ips`（`config.yaml`、`Config` 结构、env 覆盖、`ValidateRelease` 校验、`release/production-config.schema.json`、preflight 的 `checkCallbackIPs`、各测试脚本的 `GREEN_CALLBACK_ALLOWED_IPS`、`.env.example`、`.env.production.example`、部署模板与 runbook）；
   - 新增 `green.seed` 与 `green.uid`，release 模式必填且拒绝占位符；
   - seed 由实现时生成随机值（字母数字下划线，≤64）写入配置模板，部署方可在首次上线前更换；
   - uid 在配置模板标注来源（阿里云控制台账号信息，非 RAM UID）。
5. **评论/讨论文本审核（新增）**：`PostComment` 与 `PostDiscussion` 发布前同步调用 `TextModeration`（复用现有 `ReviewService`），block 结果拒绝发布并返回明确错误（不落库）；review 结果可正常发布（当前无评论审核状态机，不引入 pending 流转）；错误路径保持现有行为（不因审核服务不可用阻塞发布，与 `SubmitForAIReview` 的 ErrGreenNotConfigured 容忍语义一致）。纯文本场景，无附件与回调。
6. **文档同步**：`docs/reference/config.md`、`architecture.md`、`README.md`、部署文档、`progress.txt` 的 #92 阻塞说明（前提证伪，收口解除）、归档 progress 中 `114.115.0.0/16` 错误认知的修正说明。改动 config/routes/migrations 后运行 doc-validator `--fix`。
7. **不做**：不引入轮询兜底（`VideoModerationResult`）——回调失败有官方最多 16 次重试；轮询作为后续增强另行评估。存量内容不重扫（当前生产无真实用户数据）。

## Testing Decisions

- 测试原则：只测外部行为（HTTP 契约、结果状态、记录数），不测实现细节。
- Handler 层（路由级，prior art：`backend/internal/router/routes_security_test.go`）：
  - 合法回调（正确 checksum）→ 200，目标内容状态按结果变更；
  - 伪造/篡改回调（错误 checksum、错误 seed）→ 403，无副作用；
  - 重复回调（同 taskId）→ 第二次 200 且 `ai_review_records` 不新增记录、信誉分只扣一次；
  - 缺 seed/uid 配置时启动被 release gate 拒绝。
- Service 层（prior art：`review_service` 既有集成测试）：幂等短路、状态机（pass/review/block）回归。
- 评论/讨论审核（service 层，prior art：`social_service` 既有测试）：block 文案拒绝发布不落库、审核服务不可用时发布不阻塞、正常文案发布不受影响。
- 配置层（`config_test.go`）：`callback_allowed_ips` 校验移除、`seed`/`uid` required 与占位符拒绝。
- 发布门：preflight / deployment-contract / staging-drill 脚本同步更新并通过；`bash scripts/verify-project.sh`。

## Out of Scope

- 轮询兜底（`VideoModerationResult`）与定时重扫机制。
- 存量卡 `under_review` 视频的批量修复（生产无真实用户数据，只修未来）。
- 机器审核/人工审核控制台消息通知方案（本应用走异步扫描回调路径，不使用控制台通知方案）。
- `GREEN_CALLBACK_URL` 域名与路径本身（保持 `https://api.leeppp.online/api/v1/internal/ai-callback`）。
- 评论/私信图片审核（生产未实现图片模型与上传链路，为既有 heavy 缺口，另行追踪）。
- **zip 模组包病毒/木马扫描**：阿里云 Green 不提供病毒检测（`FileModeration` 为文档内容检测非杀毒），需新集成（ClamAV 自建或云服务），**另开 spec 评估**（见追踪 issue）。
- zip 模组包内容审核（包内资源文本/图片的违规检测，Green 无解压扫描能力，纳入病毒扫描 spec 一并评估）。

## Further Notes

- 调研报告结论 3/4/5 已核验：`GREEN_CALLBACK_ALLOWED_IPS` 非官方配置、`114.115.0.0/16` 无官方依据，本 spec 的删除决定以调研与本次 grilling 确认为准。
- 官方文档对重试次数存在差异（通用通知 3 次 vs 视频接口 16 次）：本实现按"可能重复投递"设计（幂等），不依赖固定次数。
- 实施车道：涉及安全认证与数据库迁移，按 heavy 车道（TDD、一任务一 commit、两阶段审查）。
