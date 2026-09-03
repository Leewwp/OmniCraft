# OmniCraft 长时任务提示词 · 道 3：第 8 段 D 批清票（T23/T45/T46/T54/T25/T47）

创建日期：2026-09-03
**预计失效日期**: 2026-10-03

> 任务性质：**长时任务（常规通道，非闲时/offpeak）**。单会话内按序清票，逐票独立收口。用户已明确指派本队列。
> 使用方式：在新的 ZCode 会话中粘贴启动指令（见文末附录），或直接让 agent 阅读本文件并执行。

---

## 0. 会话上下文与唯一执行权威

- 仓库：`/Users/pp/Desktop/file/code/project/OmniCraft`，工作流权威 = `AGENTS.md`（先读）。
- 执行顺序权威 = `docs/working/2026-09-02-integrated-execution-order.md`（本队列 = 其第 8 段 D 批 + SP-11 两票，已由编排会话裁决可并行）。
- 每张票的完整范围以 **GitHub 票面**为准：`gh issue view <n> -R Leewwp/OmniCraft`；视觉契约以 `design/ui-spec.md` 对应章节为唯一权威；业务规则先读 `docs/reference/business-rules.md` 对应小节。

## 1. 启动前检查（一次性）

```bash
cd /Users/pp/Desktop/file/code/project/OmniCraft && git fetch origin && git log --oneline origin/main -1   # 记下最新 main
git worktree add ../OmniCraft-wt-l3 -b lane3/seg8d-tickets origin/main
cd ../OmniCraft-wt-l3/frontend && npm install
```

- 之后所有工作在 worktree `/Users/pp/Desktop/file/code/project/OmniCraft-wt-l3` 内，**文件操作一律绝对路径**（平台可能复位工作目录）。
- 状态笔记：`/Users/pp/Desktop/file/code/project/OmniCraft/artifacts/lane3-status.md`（artifacts/ 已 gitignore，不进 PR）。**每完成一票/一个阶段立即更新**，这是会话中断后的唯一恢复点。

## 2. 票据队列（按序执行；硬上限 = 主队列 6 张 + 备用 2 张，达到即收工）

| 序 | 票 | 标题 | 队内依赖 | 车道/要点 |
|----|----|------|----------|-----------|
| 1 | T23（#243） | 错误码→用户语言映射扩展（FIX-26） | — | light；映射表 ~25 高频码 zh/en 双语 + 发布表单按 code 分文案；`error.*` 死命名空间只删死码保留在用 key；code→key 全表单测 |
| 2 | T45（#265） | 评论 author 修复（Preload）（FIX-29a） | — | light + **TDD**（先红后绿），后端 Go |
| 3 | T46（#266） | 评论回复/编辑/删除 UI + 分页（FIX-29b） | T45 | light；前端 + ui-spec 评论相关章节同步更新 |
| 4 | T54（#274） | 评论举报入口（FIX-28b） | T46 | light；前端 |
| 5 | T25（#245） | 发布配置消费化（FIX-41） | T23（FILE_TOO_LARGE 映射入 T23 表） | light；后端 /config/public **增量**下发发布类型顺序与上传上限 + 前端动态消费；public config 契约测试更新 |
| 6 | T47（#267） | 评论折叠规则（FIX-29c） | T46 + T25 | light；折叠阈值复用 T25 的配置暴露模式 |
| 备 1 | T55（#275） | 通知细节修复包（FIX-31b） | — | 主队列全绿且时间充裕才开 |
| 备 2 | #309 | NLSearch（/api/v1/agent/search）下线清理 | — | 后端 routes/handler/service + 契约文档同步；改 routes.go 后跑 `cd tools/doc-validator && go run . --fix` |

**T23 票面适配注记（票面早于 A-07 落地）**：「搜索 Agent 输入改用映射」所指的 `SearchAgentInput` 已被 A-07（PR #306）退役删除——把该意图适配到现役错误面：`AgentWorkspace` 流式错误（`AgentStreamError`/429 文案已存在，勿回退其语义）与搜索页降级面按映射表消费。

**逐票收口纪律：完成一票立即 PR 合并关票，再开下一票**（不攒批）。每票新分支（如 `lane3/t23-error-mapping`），分支基线 = 开票时的最新 `origin/main`（先 `git fetch origin`）。

## 3. 并行红线（道 1 语料注入会话在跑，违反即停）

**绝对不碰：**
1. 默认端口本地栈：postgres 5432 / redis 6379 / backend 8080 / worker——道 1 独占；不得 restart/stop 其容器与进程。
2. 端口 **3001**（`playwright.mocked.config.ts` webServer 保留端口）与 **3002**（道 1 UI dev server）。
3. `artifacts/corpus-v2/`、`artifacts/lane1-status.md`、`scripts/corpus/`——道 1 的工作区。
4. 主检出未跟踪 rig：`frontend/a06-verify*.mjs`、`a07-verify.mjs`、`t35-verify.mjs`（只读勿改）。
5. **串行收口三文档**：`progress.txt`、`AGENTS.md`、`docs/working/2026-09-02-integrated-execution-order.md`——由编排会话统一更新，你的叙述写 `artifacts/lane3-status.md`。
6. 票 #291/#286（A-04）及其相关检索行为（RAG 冻结点护栏）；不做段边界 `verify-project.sh`（编排会话职责）。
7. 不 `git add .`；不动主检出分支；不开 `docker compose`（默认工程名属道 1）。

**环境隔离方案：**
- Go 单测用 sqlite，**不需要起栈**（先写测试先跑通）。
- 浏览器验证/契约内需要后端时，自建隔离栈：`docker run -d --name lane3-pg -p 5434:5432 -e POSTGRES_PASSWORD=... postgres:16` + redis `-p 6381:6379`，backend 从 worktree 起在 **8084**（DB_DSN 指 5434），前端 `npm run dev -- -p 3200`。起栈前 `lsof -nP -iTCP:<port> -sTCP:LISTEN` 确认空闲。
- **已知平台坑**：后台运行的服务可能被平台任务清理误杀——容器与二进制重启即可恢复，测试数据可重建，无需慌张；这也要求验证脚本可重跑。
- 本地全量 `npm run test:contracts` 需要 3001 空闲：若被占（道 1 UI 阶段短暂使用 3002，通常不冲突），等待重试即可。

## 4. 每票执行纪律（摘自 AGENTS，逐条适用）

- light 车道：实现 + 与改动直接相关的测试 + 自查；**前端改动前必读 ui-spec 对应 `## Component:`/`## Page:` 章节，改了行为/视觉必须同步更新该章节**。
- i18n 强制：新 UI 字符串走 next-intl，zh/en 双语 parity；错误码映射遵守票面的 append 约定。
- 后端：gofmt、错误必须处理、统一 `{code,message}`、GORM 参数化；禁止裸 panic。
- 测试全绿基线：Go `go build/vet/test ./...`；前端 `npm run lint`（tsc）+ `npm run build` + 单测 + **本地全量 `npm run test:contracts`**（84/84 基线；CI scoped 门不覆盖 mocked contracts——t72 教训）。
- UI 票必须浏览器验证 + 截图（`screenshots/t<票号小写>-*`，存 worktree 内）：无头 Playwright `chromium.launch` 真实事件优先。
- **flaky 反模式（三次 CI 事故的教训，写测试时遵守）**：点击/键盘断言必须等「副作用生效」而非「DOM 出现」（waitFor 内轮询补点 / 等焦点落位再发键）；DOM 节点断言用布尔恒等勿 `assert.equal`。
- 后端验收类（T45 的 Preload、T25 的配置下发）用 curl 或浏览器在隔离栈上验证正常+错误路径。

## 5. git / PR / 关票流程（每票）

1. 分支 → `git add <精确文件列表>` → commit（light 单票可单 commit）。
2. 推送（ghfast 凭证坑，URL 直推不更新 origin/main 引用）：`git push "https://x-access-token:$(gh auth token)@ghfast.top/https://github.com/Leewwp/OmniCraft.git" <branch>`，推后 `git fetch origin`。
3. `gh pr create -R Leewwp/OmniCraft --base main`。
4. **仓库未开 auto-merge**（`--auto` 报错）：轮询 `gh pr checks <n> -R Leewwp/OmniCraft` 五门全绿 → `gh pr merge <n> -R Leewwp/OmniCraft --squash`。
5. `gh issue close <票号> -R Leewwp/OmniCraft -c "<测试证据评论>"`；删除远程+本地分支；`git fetch origin && git checkout main 同步`（在 worktree 内基于最新 origin/main 开下一票分支）。
6. 若 PR 与他道分支冲突（不应该发生）：rebase 自己分支解决，不动他人文件；解决不了按阻塞协议停。

## 6. 终止条件与交接

- **正常完成**：队列（或达硬上限）清完 → `artifacts/lane3-status.md` 写终态（每票：PR/commit/验证证据/坑位）→ 最终消息输出交接总结。
- **阻塞**（外部依赖失效、与他道冲突、票面与现状不可调和）：立即停当前票，状态写盘，按 AGENTS「阻塞处理」格式输出，**跳过该票继续队列下一张**（与道 1 冲突类除外——冲突即整体停）。
- **会话需提前结束**：把当前票的 WIP（分支+commit）与恢复步骤写进 lane3-status.md 后正常收束，不要留未说明的半成品。
- 红线自查（收工前）：未冻结任何 golden set/manifest；未关 #291/#286；未碰三文档；未动道 1 资产。

## 附录：启动指令（粘贴到新会话即可）

```
读取 /Users/pp/Desktop/file/code/project/OmniCraft/docs/working/2026-09-03-longtask-lane3-seg8d-prompt.md 并严格按其执行「道 3」长时清票任务。你是独立执行代理，从该文档的「1. 启动前检查」开始，逐票执行至队列完成或终止条件触发。全程中文记录。
```
