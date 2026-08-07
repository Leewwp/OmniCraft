# OmniCraft 后续工作协调规划（2026-08-07 会话结论）

> 创建日期：2026-08-07 | **预计失效日期**: 2026-10-07
> 范围：Ops-08 收尾 → 归档清理 → UI 体验修复（#64 系列）→ source-linkage → Web Agent Task 6 → IP history → 收藏集 cutover
> 依据：AGENTS.md 活计划注册表、`docs/superpowers/plans/` 各计划、`docs/superpowers/specs/2026-08-07-omnicraft-web-experience-corrections-design.md`、GitHub issues #1/#22/#28/#30/#31/#32/#64/#65~#76
> 本文件为会话结论汇总，不替代各计划文件与 AGENTS.md 的权威地位；任务来源始终以 AGENTS.md 注册表为准。

---

## 1. 当前状态盘点

### 1.1 当前状态：Ops-08 Step 5 blocked（审查修正）

- 当前审查基线：`main`/`origin/main` 为 `25670ae`；当前工作树存在本轮修复修改，未提交。
- 已确认：Ops-08 Step 1-4 的契约实现存在；原先声称的真实 Step 5、approved RPO/RTO 和双目的地 receipt 无法由当前仓库独立复核。
- 当前处理：修复真实 Compose deploy/rollback、registry digest、workflow protected drill、TLS 拓扑和文档状态；`release/recovery-objectives.json` 保持 `baseline_only`。
- 计划要求：真实 staging 环境 / OSS / offsite 输入，缺失时**阻塞**；staging-drill 需真实部署和原始恢复证据，不得模拟。
- Ops-09 随桌面范围暂缓，计划文件保留（不得归档、不得勾选）。

### 1.2 当前工作树状态（待提交）

- 当前工作树仅保留本轮审查修复及其回归测试/文档修改；没有把历史 main 遗留或已完成规格误列为待处理文件。
- 本轮仍未提交：Ops-08 Step 5 依赖真实 staging、OSS/off-site 和批准恢复输入；解除阻塞后再按 heavy 车道精确 stage。

### 1.3 Agent 功能页状态澄清（本轮确认）

- `/agent` 工作台页**已开发并合并进 main**（web-agent productization Task 4，PR #43）：`frontend/app/(protected)/agent/page.tsx` + `frontend/components/agent/*`
- 顶部导航不显示 Agent 入口的**原因**：`backend/config.yaml:174` 的 `web_agent_enabled=false`，`Header.tsx:133/384` 的 `AgentFeatureGate` 渲染 `fallback={null}`
- 这是计划要求的设计行为（Task 6 验证通过前默认关闭），**不是未合并或故障**
- 本地临时查看：改 `web_agent_enabled=true` 并重启后端（不提交；无真实 key 时会话走降级/报错，但页面可访问）

### 1.4 分支与 worktree 清单（清理候选）

本地分支：`codex/ops-integration`（已合并）、`codex/ops/ops-08`（Ops-08 合并后删）、`opencode/p01-ui-prototype`（原型证据分支）、`web/agent-t1/t2/t3/t5`（squash 合并残留）

远端分支：`codex/ops/ops-04`、`codex/ops/ops-08`、`web/agent-t2/t3/t4`、`web/gap-15-ui-spec`、`web/gap-16-overlay`、`web/gap-17-19`、`opencode/p01-ui-prototype`、dependabot/* 若干

Worktree：`/private/tmp/OmniCraft-ops08`（Ops-08 后删）、`/private/tmp/OmniCraft-prev`（detached HEAD，可删）

---

## 2. 推荐执行顺序

### Phase 0：Ops-08 审查修复与阻塞收口（进行中）

1. 完成本轮脚本、Compose、workflow、deployment evidence 归档和文档状态修复
2. 运行针对性契约与项目验证门；真实外部输入缺失时不得提交或勾选 Step 5
3. 提供真实 staging/OSS/offsite/measured 输入后，执行真实 drill、归档 receipt 和两阶段审查
4. 仅在 Step 5 全部证据可复核后更新 AGENTS.md 注册表与 progress.txt 的完成状态
5. 当前阻塞按 B1/B5 输出，等待人工提供外部输入

### Phase 1：归档与清理

1. 提交 main 遗留：`docs/GLOSSARY.md` + 08-07 spec + 08-07 audit 报告
2. 归档：audit 报告保留至失效日；production-readiness 计划**不归档**（Ops-09 段保留）；spec 无需归档
3. 在 AGENTS.md 注册新计划「Web 体验修复与数据契约收口」（来源 #64，Ticket 01–12），light/heavy 拆分按规格 Decision 34
4. 分支清理（确认 PR 均已合并后）：
   - 本地删：`codex/ops-integration`、`web/agent-t1/t2/t3/t5`、`opencode/p01-ui-prototype`、`codex/ops/ops-08`
   - 远端删：`codex/ops/ops-04`、`codex/ops/ops-08`、`web/agent-t2/t3/t4`、`web/gap-15-ui-spec`、`web/gap-16-overlay`、`web/gap-17-19`、`opencode/p01-ui-prototype`、dependabot/*
5. Worktree 清理：删除 `/private/tmp/OmniCraft-ops08`、`/private/tmp/OmniCraft-prev`
6. 迁移编号规划落位：`061`（source-linkage）、`063`（collaboration-invites）已保留；新迁移从 `064` 起（IP history = `064_ip_visit_history.sql`；favorites 删除 = `065_drop_legacy_favorites.sql`，以实际实现为准）

### Phase 2：UI 高收益修复（light 车道）

顺序：T01 对齐 P-01 重叠边界与 UI 权威契约（依赖根）→ T02 统一导航壳、品牌入口与筛选状态 → 建议顺带 T06 补齐 IP 讨论入口与关注状态反馈（audit A 类缺陷，仅依赖 T01）

- 覆盖 audit 问题：侧边栏背景不一致、logo 对齐、品牌入口跳推荐、下一章溢出、封面边框对齐、关注按钮样式、点赞/点踩状态丢失、筛选选中态不统一
- 前置：调和 `design/ui-spec.md`（Header/页面外壳/筛选/FollowButton 等章节）
- 验证：浏览器实测 + 截图（screenshots/）

### Phase 3：source-linkage（light 车道）

- 计划：`docs/superpowers/plans/2026-06-30-omnicraft-community-source-linkage.md`（64 项）
- 迁移：`061_source_fanwork_id.sql`
- 硬约束：必须先于 collaboration-invites（共享 `content_repo.go`、`zh.json`/`en.json`，串行执行）
- 与 #64 共享 ContentDetail/路由/翻译面 → 发布精确文件预留，串行编辑

### Phase 4：Reaction / 浮窗 / 目录 / 排序（light 车道）

顺序：T07 收口查看者反应 API 与 UI 契约 → T03 浮窗共享元素转场原型 → T04 接入浮窗转场与媒体加载 → T05 浮窗内系列目录与章节导航 → T08 跨页面共享排序下拉组件

- 依赖边：T04←T03←T05 线性；T07/T08 仅依赖 T01
- 前置：ui-spec 调和（ContentDetailOverlay/SeriesNav/ReactionBar/SortSelect 章节）
- **插入点：Web Agent Task 6 建议在此阶段完成后执行**（见 3.2）

### Phase 5：IP history 独立表（heavy 车道）

- Ticket 09（#73）：独立 IP 访问历史模型 + 匿名合并，heavy 任务，迁移独立于 favorites 清理
- 迁移：`064_ip_visit_history.sql`
- heavy 要求：先写失败测试确认预期失败 → 最小实现 → 两阶段审查

### Phase 6：收藏集 cutover + 删除 favorites（heavy 车道）

顺序：T10 收藏成员关系成为唯一收藏状态源 → T11 退役旧 favorites 运行时依赖 → T12 forward-only 清理与云端 cutover

- 依赖边：T12←T11←T10；T12 另有云端人工门（访问日志确认无旧端点调用 + 可恢复迁移前备份）
- 迁移：`065_drop_legacy_favorites.sql`（forward-only，历史迁移与 checksum fixture 不可修改）
- 云端数据允许丢弃（无对账零漂移要求），但必须有可恢复备份
- 低风险视觉修复不得混入本阶段

### 后续（不在本轮主序列，视用户安排）

- **collaboration-invites**（注册表优先级 3，62 项）：须在 source-linkage 合并后串行执行（Phase 3 后可随时插入）
- **Web Agent Task 6**（见 3.2）
- **wayfinder 设计线**（#22/#28/#30/#31/#32）：作品集导向设计项，需用户 grill/决策，非编码任务，与开发线无文件冲突可并行；#30 落地时与 #64 共享前端文件需串行

---

## 3. 阻塞项（必须人工介入，不得模拟）

| # | 阻塞项 | 涉及 | 解除方式 |
|---|--------|------|----------|
| B1 | 真实 staging 环境 / OSS / offsite 输入缺失 | Ops-08 | 提供真实部署环境与凭证后执行真实演练 |
| B2 | 真实 `agent.llm_api_key`（真实 LLM Provider）缺失 | Web Agent Task 6 | 提供密钥与生产配置（origin/限流/预算/可观测性） |
| B3 | 云端人工门：访问日志确认无旧 favorites 端点调用 + 可恢复迁移前备份 | T12 | 云端日志查询 + 备份后放行 |
| B4 | 桌面范围暂缓 | Ops-09、D-02~D-05、R-02、Tauri Agent 页 | 用户明确恢复桌面开发（当前保持 `desktop_deploy_enabled=false`、`client.download_enabled=false`） |
| B5 | SMTP/验证码/HTTPS 证书/Allowed Origins/正式域名等发布输入 | 发布相关 | 由用户提供（与 Ops-08 重叠部分并入 B1） |

阻塞期间：禁止 commit、勾选 checkbox、假装完成；在 progress.txt 记录进度与原因。

---

## 4. 建议处理的问题（行动清单）

1. **提交 main 工作树遗留**（GLOSSARY + spec + audit）——Phase 1 首批
2. **注册新计划**到 AGENTS.md：Web 体验修复与数据契约收口（#64），注明 light/heavy 拆分与迁移编号
3. **清理分支/worktree**（清单见 1.4）——避免后续新分支误基于残留分支
4. **Task 6 与 #64 的契约共享面**：Task 6 Step 2 的 Playwright 断言覆盖共享 detail-overlay、Header 搜索、widget 挂载等，这些契约正被 T01–T04 重定义 → 建议 Task 6 在 Phase 4 之后、Phase 5 之前插队，一次验证最终形态；若 B2 未解除则继续阻塞，不占开发队列
5. **Task 6 解除后**：从最新 main 开全新 `web/agent-t6`（不复用旧分支）；Step 5 仅通过部署配置开启功能，仓库默认保持 `false`；不 stage 密钥
6. **迁移编号纪律**：`061`/`063` 已保留，新迁移 064 起；改 config/routes/migrations 后跑 `cd tools/doc-validator && go run . --fix`
7. **ui-spec 调和前置**：#64 规格 Note 明确 Header/页面外壳/IP 讨论/ContentDetailOverlay/SeriesNav/ReactionBar/FollowButton/筛选/SortSelect 章节必须在对应实现前调和
8. **每个阶段验证门**：`go build/vet/test`、`npm run build/lint`、`bash scripts/verify-project.sh`（--full）；UI 必做浏览器实测 + 截图；heavy 任务先失败测试再实现
9. **本地临时查看 Agent 页**：`web_agent_enabled=true` + 重启后端（不提交，改回 `false` 再提交）；正式解封只能走 Task 6
10. **进度记录**：每阶段完成更新 `progress.txt`（近 30 天，超 64KB 轮换归档）

---

## 5. 关键约束（全程生效）

- 任务来源：AGENTS.md 活计划注册表；计划完成后归档到 `docs/archive/plans/`（Ops-09 所在文件除外）
- 车道纪律：涉及安全/发布门/迁移结构/生产配置 = heavy（TDD + 两阶段审查）；否则 light（自查替代正式审查）
- 多计划共享面：source-linkage、collaboration-invites、#64 共享 content-detail/路由/翻译 → 精确文件预留 + 串行
- 文档冲突处理：以权威源为准（宪法 > 生产代码 > architecture.md > design/ > specs/ > plans/），矛盾记录为 issue，不做自行发挥
- 新增 UI 字符串走 next-intl；视觉改动用 design-system token；禁止 `err.Error()` 直达客户端
