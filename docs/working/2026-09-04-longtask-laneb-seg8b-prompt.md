# OmniCraft 闲时任务提示词 · 道 B：第 8 段 B 批审核链路清票（T10/T11/T41/T42/T43/T44）

创建日期：2026-09-04
**预计失效日期**: 2026-10-04

> 任务性质：**闲时通道（offpeak/idle）长时清票**。闲时通道可能被平台侧随时切断（不可自动重派），因此本提示词按「可中断、可续跑」设计：逐票即时收口、WIP 定期 commit、状态板每步落盘。本文件幂等可重复使用——每个闲时窗口开始时先查 GitHub 与状态板，跳过已收口票，从断点继续。
> 队列来源：编排会话 2026-09-04 派发（总序第 8 段 B 批的审核链路子链；T29/T30/T31/T32/T33 留给后续常规会话——T30 触及迁移属 heavy、T32 为 auth 域且须并 #322，均不适合闲时通道）。

---

## 0. 会话上下文与唯一执行权威

- 仓库：`/Users/pp/Desktop/file/code/project/OmniCraft`，工作流权威 = `AGENTS.md`（先读）。
- 执行顺序权威 = `docs/working/2026-09-02-integrated-execution-order.md`（本队列 = 其第 8 段 B 批子链，票内 `## Blocked by` 优先）。
- 每张票的完整范围以 **GitHub 票面**为准：`gh issue view <n> -R Leewwp/OmniCraft`；业务规则先读 `docs/reference/business-rules.md` 对应小节（审核/通知/判官）；前端票（T44）改前必读 `design/ui-spec.md` 对应 `## Component:`/`## Page:` 章节。

## 1. 启动前检查（每个闲时窗口开始时做一次）

```bash
cd /Users/pp/Desktop/file/code/project/OmniCraft && git fetch origin && git log --oneline origin/main -1
cat artifacts/laneb-status.md 2>/dev/null   # 断点与已完成票
# worktree 若已存在则复用；不存在则创建：
git worktree add ../OmniCraft-wt-lb -b laneb/seg8b-tickets origin/main 2>/dev/null || true
cd ../OmniCraft-wt-lb/frontend && npm install   # 首次或依赖变更时
```

- 之后所有工作在 worktree `/Users/pp/Desktop/file/code/project/OmniCraft-wt-lb` 内，**文件操作一律绝对路径**（平台会复位 Bash 工作目录）。
- 状态笔记：`/Users/pp/Desktop/file/code/project/OmniCraft/artifacts/laneb-status.md`（artifacts/ 已 gitignore，不进 PR）。**每票开始/每个验证门/每次 PR 事件立即更新**——这是会话被切断后的唯一恢复点。
- 续跑判定：`gh issue view <票号> -R Leewwp/OmniCraft --json state` 为 OPEN 且状态板无「实现完毕待合并」标记的队列第一张 = 本窗口起点。

## 2. 票据队列（按序执行；硬上限 = 主队列 6 张 + 备用 2 张，达到即收工）

| 序 | 票 | 标题 | 队内依赖 | 车道/要点 |
|----|----|------|----------|-----------|
| 1 | T10（#230） | 审核路径缓存失效补全（FIX-38） | — | light + TDD；提炼共享缓存失效 helper（对齐内容域已有的作者编辑/删除失效与 T17 的 `NewIPServiceWithInvalidation` 模式，勿另造一套），挂 admin ban/restore、申诉批准、举报 auto-hide 四写点；helper 需容忍本地无 redis（nil 不 panic）；验收 = 预热缓存 → admin ban → 立即 404 |
| 2 | T11（#231） | NotifyContentStatus helper + AI/admin 挂接（FIX-17a） | — | light + TDD；helper 契约（type=content_status、channel=system、body 带 reason）是 T41/T42/T53/T55 共用契约，**先于下游合入**；本票只挂 AI 审核结果 + admin ban/restore 写点；IP 级联下架按受影响作者去重各一条（防通知风暴）；文案沿用后端中文模板（locale 模板化为登记不修项）；worker 运行下端到端：内容被 ban → 作者通知落库 |
| 3 | T41（#261） | 判官闭案回写内容状态 + 作者通知（FIX-10） | T10+T11 | light + TDD 四例（approve/reject/守卫/作者通知）；closed_approve→published（守卫防覆盖 admin/AI 终态 banned）+ 发索引事件；closed_reject→banned+ban_reason（不扣信誉分）；通知用 T11 契约、失效用 T10 helper；`docs/reference/business-rules.md` 补一句口径；**与 T39（#259，E 批）同文件不同函数——本票不动 T39 范围** |
| 4 | T42（#262） | 举报自动隐藏触发众裁（FIX-11） | T10+T11 | light + TDD；举报达阈值自动创建判官案件（提炼 AI 审核路径的 ensureJudgeCase 为共享方法，行为等价回归）+ 作者通知；auto-hide 加终态守卫（banned 不被打回 under_review） |
| 5 | T43（#263） | 编辑触发再审核 + banned 禁改（FIX-13） | —（T09 已完成） | light + TDD；banned 内容 PATCH → 403（删除仍允许）；published 的 title+cover 变更走增量复审（PATCH 契约无 description 字段，正文仅经 PR merge 变更——Phase 6 修正口径）；封面 URL 校验平台 OSS 域/键（防外链投毒） |
| 6 | T44（#264） | 作者可知性：studio 全状态 + 编辑/删除/申诉入口（FIX-14） | —（T08/T09 已完成） | light；前端为主；本人视角返回全部状态内容（现状只回 published）+ 状态徽标（i18n zh/en，药丸形态遵循 U-05 定稿）+ banned 行显示封禁原因 + 「去申诉」预填跳转 + 编辑/删除接线（现状死按钮）；StatusBadge 在 ui-spec 登记；浏览器验证 + 截图 `screenshots/t44-*` |
| 备 1 | T53（#273） | 举报反馈闭环（FIX-28a） | T11 | 主队列全绿且时间充裕才开；reports/me 端点（隔离他人数据）+ admin 处理完成通知举报者 + 前端「我的举报」 |
| 备 2 | T55（#275） | 通知细节修复包（FIX-31b） | — | 同上；深链映射/已读联动/广播角标/私信摘要/轮询间隔对齐等杂项，浏览器验证为主 |

**逐票收口纪律：完成一票立即 PR 合并关票，再开下一票**（不攒批）。每票新分支（如 `laneb/t10-cache-invalidation`），分支基线 = 开票时最新 `origin/main`（先 `git fetch origin`）。

## 3. 并行红线（多会话并行在跑，违反即停）

**绝对不碰：**
1. 默认端口本地栈：postgres 5432 / redis 6379 / backend 8080 / worker——**道 1 注入栈仍活着**，供用户 golden set 复核查证与后续 A-04 使用；不得 restart/stop 其容器与进程。
2. 端口 **3001**（playwright mocked contracts 保留端口）；**8081**（占用中，非本队）。
3. `artifacts/corpus-v2/`、`artifacts/lane1-status.md`、`scripts/corpus/`——语料与 golden set 工作区；**golden set 复核进行中，任何 artifacts/corpus-v2/ 下文件只读**。
4. 道 A bug 批会话（worktree `OmniCraft-wt-la`，端口 5436/6383/8085/3201/9095）——不碰其 worktree 与端口。
5. 道 3 遗留容器（lane3-pg@5435 / lane3-redis@6382 / backend 8084 / frontend 3200 / metrics 9094）与 worktree `OmniCraft-wt-l3`——闲置但非本队资产，不 restart 不清理。
6. 主检出未跟踪 rig：`frontend/a06-verify*.mjs`、`a07-verify.mjs`、`t35-verify.mjs`（只读勿改）。
7. **串行收口三文档**：`progress.txt`、`AGENTS.md`、`docs/working/2026-09-02-integrated-execution-order.md`——由编排会话统一更新，你的叙述写 `artifacts/laneb-status.md`（逐票 PR 内的 progress.txt 新增条目属正常提交，不改历史条目）。
8. 票 #291/#286（A-04）及一切检索行为（chunker/embedding/rerank/查询扩展——RAG 冻结点护栏）；不做段边界 `verify-project.sh`（编排会话职责）。
9. 若复核冻结发生、A-04 会话启动：origin/main 出现检索行为相关合并属正常，`git fetch origin` 后 rebase 自己分支即可，**不与 A-04 抢默认栈**。
10. 不 `git add .`；不动主检出与其他 worktree 分支；不开 `docker compose`（默认工程名属道 1）。

**环境隔离方案：**
- 本队端口：**pg 5437 / redis 6384 / backend 8086 / metrics 9096 / 前端 3202**。起栈前逐口 `lsof -nP -iTCP:<port> -sTCP:LISTEN` 确认空闲（polyu 容器占 5434/6381，勿动勿用）。
- 隔离栈：`docker run -d --name laneb-pg -p 5437:5432 -e POSTGRES_PASSWORD=omnicraft -e POSTGRES_DB=omnicraft pgvector/pgvector:pg16` + `docker run -d --name laneb-redis -p 6384:6379 redis:7-alpine`；迁移全量应用后 backend 从 worktree 起在 **8086**（DB_DSN 指 5437、REDIS_ADDR 指 6384，参照 lane3-status.md 环境实况段的 env 写法；CONFIG_OVERRIDE_PATH 放行 3202 CORS + metrics 9096——注意 metrics_port 在 observability 节）。前端 `NEXT_PUBLIC_API_URL=http://localhost:8086 npm run dev -- -p 3202`。
- Go 单测用 sqlite，不需要起栈（先写测试先跑通）；起栈仅 T11/T41/T42 的 worker 端到端与 T44 浏览器验证需要。
- 本地全量 `npm run test:contracts` 需要 3001 空闲：被占则等待重试。
- **平台坑**：后台服务可能被平台任务清理误杀——容器/二进制重启即可，验证脚本须可重跑。

## 4. 每票执行纪律（摘自 AGENTS，逐条适用）

- light 车道：实现 + 与改动直接相关的测试 + 自查；票面标注 TDD 的先写失败测试确认预期失败，再最小实现。
- 后端：gofmt、错误必须处理、统一 `{code,message}`、GORM 参数化；禁止裸 panic；改 config.go / migrations / routes.go 后跑 `cd tools/doc-validator && go run . --fix`。
- i18n 强制：新 UI 字符串走 next-intl，zh/en 双语 parity。
- 测试全绿基线：Go `go build/vet/test ./...`；前端 `npm run lint` + `npm run build` + 单测 + 本地全量 `npm run test:contracts`（基线 84/84，以最近 main 为准）。
- UI 票（T44）必须浏览器验证 + 截图（无头 Playwright `chromium.launch` 真实事件优先，存 worktree 内 `screenshots/t44-*`）。
- **flaky 反模式（CI 事故教训）**：点击/键盘断言必须等「副作用生效」而非「DOM 出现」；DOM 节点断言用布尔恒等勿 `assert.equal`；i18n 夹具必须镜像真实 catalog（勿发明生产不存在的 key）。
- 后端验收用 curl 在隔离栈上验证正常 + 错误路径。

## 5. git / PR / 关票流程（每票）

1. 分支 → `git add <精确文件列表>` → commit（light 单票可单 commit）。
2. 推送（ghfast 凭证坑，URL 直推不更新 origin/main 引用）：`git push "https://x-access-token:$(gh auth token)@ghfast.top/https://github.com/Leewwp/OmniCraft.git" <branch>`，推后 `git fetch origin`。
3. `gh pr create -R Leewwp/OmniCraft --base main`。
4. **仓库未开 auto-merge**：轮询 `gh pr checks <n> -R Leewwp/OmniCraft` 五门全绿 → `gh pr merge <n> -R Leewwp/OmniCraft --squash`。他道插队合并致 PR BEHIND → `gh pr update-branch <n> -R Leewwp/OmniCraft` 后等 CI 重跑。SeriesNav/t72 家族已知闪断（#323 待修）：门挂了先重跑一次再判。
5. **确认 `state=MERGED` 之前绝不删远程分支**（删分支自动 CLOSE PR）。合并后关票：`gh issue close <票号> -R Leewwp/OmniCraft -c "<测试证据评论>"`（squash 带 `(#票号)` 通常自动关）→ 删分支 → `git fetch origin` → 下一票从最新 `origin/main` 开分支。
6. PR 冲突（不应发生）：rebase 自己分支解决，不动他人文件；解决不了按阻塞协议停。

## 6. 闲时通道特化协议（重要）

- **WIP commit**：当前票做到任何可编译节点就 commit 到票分支（哪怕测试未全绿，commit message 注明 WIP）；平台切断后工作不丢。
- **状态板即恢复点**：每完成一个动作（开票分支/测试红/测试绿/PR 创建/门绿/合并）立即写 `artifacts/laneb-status.md`；格式参照 `artifacts/lane3-status.md`。
- **不派子代理**：闲时通道的子代理派发脆弱；所有工作主会话直接做。
- **窗口尾部收口**：估计剩余时间不足一票完整周期（实现+验证+PR+CI 轮询 ≈ 40-60 分钟）时，不开新票；当前票能收则收，收不了则 WIP commit + 状态板写清断点后正常收束。
- **终止条件**：① 队列（含备票，达硬上限）清完；② 连续两票阻塞（外部依赖失效/与他道冲突/票面与现状不可调和）——单票阻塞则记录后跳过继续队列；③ 用户在会话中出现并叫停。任一触发 → 状态板写终态（每票 PR/commit/验证证据/坑位）+ 最终消息输出交接总结。
- **红线自查（收工前）**：未冻结任何 golden set/manifest；未关 #291/#286；未碰三文档与道 1/道 A 资产；未动默认栈。

## 附录：启动指令（粘贴到闲时任务即可，可重复使用）

```
读取 /Users/pp/Desktop/file/code/project/OmniCraft/docs/working/2026-09-04-longtask-laneb-seg8b-prompt.md 并严格按其执行「道 B」闲时清票任务。你是独立执行代理：先做「1. 启动前检查」里的续跑判定（查 artifacts/laneb-status.md 与 GitHub 票状态，跳过已收口票），然后从队列第一张未完成票执行起，直到队列完成或终止条件触发。闲时通道随时可能被切断——遵守文档「6. 闲时通道特化协议」的 WIP commit 与状态板纪律。全程中文记录。
```
