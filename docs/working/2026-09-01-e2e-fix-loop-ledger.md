# E2E 修复循环状态台账（六流程云端验收 + 演示数据生产）

> 创建：2026-09-01 ｜ 预计失效：2026-10-01（面试窗口 2026-09-03 后本台账转为归档素材）
> 本文件是跨会话续跑的唯一依据。每完成一个子 agent 任务立即更新。
> 编排协议、环境事实、发布流程见任务目标（2026-09-01 /goal）与 AGENTS.md；凭证在仓库外 `~/Desktop/file/note/项目/omnicraft-测试账号.md`，绝不写入本文件。

## 六流程状态矩阵

| # | 流程 | 状态 | 最近验收时间 | 证据索引 |
|---|------|------|--------------|----------|
| 1 | IP 创建（登录→创建→详情→可被二创引用） | ✅ **云端全绿**（B-001/002/003 闭环；全量用例 T1.1~T1.5、E1~E4 PASS） | 2026-09-01 发布验收（fecef1b） | 回归证据：screenshots/qa-release-r*.png、t1*.png；R1~R5 详见发布验收报告（子 agent 会话）；测试报告：docs/working/2026-09-01-flow1-ip-test-report.md |
| 2 | 原创及二创内容发布（含 OSS 真实上传、来源归属） | 🟡 testing（测试子 agent 进行中） | — | — |
| 3 | 内容浏览（feed/详情/筛选/搜索/媒体显示） | ⏳ pending | — | — |
| 4 | 评论/点赞/收藏/互动 | ⏳ pending | — | — |
| 5 | Agent 对话（指导类问答，无降级横幅） | ⏳ pending | — | — |
| 6 | Agent 检索（返回真实站内内容+引用） | ⏳ pending | — | — |

状态图例：⏳ 未开始 ｜ 🟡 测试/修复中 ｜ 🔴 有未闭环缺陷 ｜ ✅ 云端全绿（用例全过+缺陷全关）

## 阶段 B（演示数据生产）

状态：⏳ 未开始（前置：六流程全绿）。规范见任务目标：7 创作者账号、3~5 IP、15~20 条内容（含封面/附件/跨账号互动/1~2 讨论话题）、admin 至少一次管理动作、内容严禁 test/E2E/测试/demo 字样。

## 缺陷登记表

| 编号 | 流程 | 状态 | 场景一句话 | 根因一句话 | 修复 commit | 回归证据 |
|------|------|------|------------|------------|-------------|----------|
| B-001 | 1 | **closed** | 前端无「创建 IP」入口 | 前端 IP 表面全部建成唯独创建表单从未实现（历史遗漏，非有意裁剪）；设计输入/ui-spec 均无对应节 | **fecef1b**（feat(ip): add IP creation entry and studio publish form） | R5 全链 PASS（toolbar→表单→浏览器直传封面→pending 反馈→approve→公开可见），截图 qa-release-r5-* |
| B-002 | 1（波及 2/3/全部媒体展示） | **closed** | OSS 私有读 → 封面/图片经 /_next/image 全部 403 不渲染 | 展示型媒体 URL 以未签名裸 URL 入库/出 API，违反 architecture.md §6.2 契约 | **774994f**（fix(media): sign private-OSS display URLs at API boundary） | R1 签名 URL 200/裸 URL 403 实证；R2 详情页+列表封面渲染，截图 qa-release-r2-* |
| B-003 | 1 | **closed** | IP tags 被接收但从不持久化、不展示 | ip_tags 表(005)+IPTag 模型自始存在但 IPRepository/IPService 从未实现 tag 读写，CreateIP 静默丢弃 | **6d1d781**（feat(ip): persist and display IP tags） | R3 规范化+psql 3 行实证；R4 TagBadge 渲染，截图 qa-release-r4 |
| B-004 | 1（波及内容审批同款路径？待流程2确认） | open | admin approve/reject 后 IP 详情/列表缓存不失效，公开可见性延迟 ≤300s（实测 ~4m50s） | **存量缺陷**（先于本批）：admin.go:66 `service.NewIPService(...)` 自建无 rdb 实例，`InvalidateIPCacheForAdmin` 的 `if s.rdb==nil return` 静默 no-op；读路径却是带缓存实例 | — | 发布验收 X1 实测记录（IP2 ~4m50s、IP3 ~4m35s）；修复方向：AdminHandler 注入容器级 ctr.IPService 或 NewIPServiceWithCache；并入流程 2 修复批次 |

### 并入修复的附带项

- ~~附件 DTO 缺 `oss_url` 字段~~（已随 B-002 修复：ContentAttachment.OSSURL 瞬态字段+签名，774994f）。
- `ip_repo.go:80` 搜索未消费 search_vector 列（tags 写入后不增强搜索）——记观察，不在本批修。
- IP cover 契约与 content 派生式不一致：CreateIP 仍接受客户端完整 CoverURL（content 链由后端从 upload grant 派生）。本批 UI 按现状契约走（前端用自己 presign grant 的结果拼 URL，与 avatar settings 先例一致，不扩大攻击面）；后续统一为 oss_key 派生。
- FeedbackTicket.Attachments 序列化裸 oss_key（B-002 审查发现的范围外项，前端若展示反馈截图会 403）——后续跟进。
- gofmt 存量欠账：main 上 content_download_test.go / version.go / admin_audit.go / config_public.go / feedback.go 五文件非本循环引入，不修（surgical changes only）。

### 观察清单（不立项，流程 3/5 再裁决）

- 中文 IP 名 slug 失真（generateSlug 剔中文，只剩 ascii 片段）——影响流程 3 URL 可读性。
- `GET /ips?q=` simple 分词，中文部分词 0 结果——影响流程 3 搜索体验。
- 未登录首页固定 console 403→401 噪音（api.ts 设计如此，观测体验差）。
- 创建 IP 触发真实阿里云 Green AI 审核 pass，但 pass 不自动 approve（需 admin 手动）——语义待确认是否符合设计（business-rules）。

状态流转：open → diagnosed → fixing → verifying → closed。同一缺陷 3 轮修复未过 → 停止硬磨，按停止条件上报。

## 探查垃圾清理清单（阶段 B 结束前必须下架/软删）

| 类型 | ID/名称 | 创建账号 | 状态 |
|------|---------|----------|------|
| IP | id=1「QA探查-星语森林」（slug=qa） | demo01(user_id=3) | approved，留站 |
| IP | id=2「QA探查-标签验证」（无封面） | demo01 | approved，留站 |
| IP | id=3「QA探查-UI全链」 | demo01 | approved，留站 |
| OSS 对象 | uploads/3/image/2026/08/31/1788203236_4d680bc3e00ac1d7.jpg（IP1 封面） | demo01 | 随 IP 处置 |
| OSS 对象 | uploads/3/image/2026/08/31/1788208407_6169cb67933e7c80.jpg（IP3 封面） | demo01 | 随 IP 处置 |
| ai_review_records | ip:1 与 ip:3 各 1 条（aliyun，pass） | 系统 | 随 IP 处置 |

（阶段 A 各流程测试产生的 IP/内容/评论等，逐一登记，创建时用「QA探查-」前缀便于识别）

## 发布批次记录

| 批次 | 日期 | 发布前 pg_dump 文件 | commit | 内容 | 结果 |
|------|------|---------------------|--------|------|------|
| R1 | 2026-09-01 | /home/ubuntu/omnicraft-backup-20260901-0418.sql（191K 全量） | fecef1b | B-001/002/003 三缺陷（774994f+6d1d781+fecef1b） | ✅ 8/8 容器 healthy、healthz 200、构建 2m57s；R1~R5 回归+流程 1 全量用例全绿，流程 1 闭环；CORS 未拦截 |

**发布运维注意项**：① 每次重建 backend 后必须 `docker compose restart nginx`（upstream DNS 缓存，否则 /api/v1 502 ~2 分钟）——批次 R1 已踩，后续发布命令后追加此步并验证 /healthz。② `.env` 改动必须 `up -d` 重建，`restart` 不重读环境变量。③ `/var/lib/omnicraft/config_override.yaml` 权限 ≥0644。④ CORS 实测：`https://app.leeppp.online` 源对 OSS 直传 PUT 200（无需 bucket 变更）；localhost:3000 源被拦（本地开发限制，非线上问题）。

## 当前批次进行到哪一步 / 下一步

- **批次 1（流程 1：IP 创建）**：✅ **闭环**。测试 → 诊断 → B-002 heavy 修复（774994f，两阶段审查 APPROVE）→ B-003/B-001 light 批次（6d1d781+fecef1b）→ 发布批次 R1 → 三缺陷回归 R1~R5 + 全量用例全绿 → 流程 1 标 ✅。
- **批次 2（流程 2： 内容发布）**：测试子 agent 派发中。B-004（admin 审批缓存不失效，存量）open，并入本批次修复。
- 下一步：流程 2 测试报告 → 诊断（含 B-004 影响面确认：内容审批是否同款 no-op）→ 修复 → 发布验收（记得 restart nginx）。

## 环境快照（2026-09-01 开工时）

- 云端 8 容器全部 Up/healthy（backend/frontend/nginx/pgbouncer/postgres/prometheus/redis/worker）。
- `https://app.leeppp.online` 200；`https://api.leeppp.online/healthz` 200；`GET /api/v1/ips` 返回空列表（total=0，站内无 IP）。
- 本地 main @ e97a2b3，工作树干净（仅未跟踪的 UX audit 草稿，与本循环无关）。
- 已知风险预告：流程 6 的 embedding_model(text-embedding-3-small, 1536维) 与 minimax provider 可能不匹配 + 云端无 opensearch（BM25 缺位），预期内工作。
