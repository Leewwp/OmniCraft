# OmniCraft 交互式长会话开发提示词（2026-09-02，段5 收尾 + 段6 实现线）

> 创建日期：2026-09-02 ｜ **预计失效日期**: 2026-10-31（原表述：段 6 实现部分 A-01/A-02/A-05/A-03 全部落地后即失效）
> 用途：粘贴到 zcode **交互式本地会话**（用户在场，正常套餐通道）的首条消息，或作为开场消息指向的执行文档。与闲时版（`2026-09-02-idle-goal-prompt.md`）的区别：无闲时票据 3 小时硬墙与心跳互斥条款；新增「用户在场裁决点」与「上下文压力收尾」；其余治理条款（总序权威 / 验证门 / heavy 两阶段审查 / 精确提交）不变。

---

（以下为提示词正文，从此行下一行开始复制）

你是 OmniCraft 项目的交互式长会话开发 Agent（用户在场）。本轮任务按已定稿的整合执行总序逐票完成开发、测试、验证、提交与关票，范围限定在下方「工作边界」内，直到边界内票据全部收口或上下文压力触发收尾。你不重新做规划、不重开设计讨论——所有设计均已由用户逐条拍板；有方向性疑问时向用户提问（这是交互会话的优势，不要自行发挥）。

## 工作边界（本会话唯一执行范围）

- **核心（必做，按序）**：
  1. **T17（#237 收窄版，light）**：admin IP 状态变更缓存失效 + B-004 折叠（根因 `admin.go:66` 自建无 rdb 的 IPService 致 `InvalidateIPCacheForAdmin` 静默 no-op，admin 审批后 IP 缓存 ~5 分钟不失效）。收口即第 5 段完结。
  2. **A-01（#283，heavy）**：后端会话模型重构（title/pinned_at 迁移 074+、续写、自动标题、停止保留）——worktree + TDD + 两阶段审查。
  3. **A-02（#284，light）**：SSE 契约 v2（真流式 + 思考块转发 + 工具步骤事件；同步 `docs/specs/web-agent-v0.4-mvp.md`；**继承 T19 行缓冲语义不得回退**）。
- **弹性（核心全收口且上下文仍充裕时才做，按序）**：A-05（#287 护栏补口：chat 输入前置 Green + 输出事后异步审核）→ A-03（#285 检索管线升级：DashScope v4@1536 零 DDL + hybrid on Postgres + 查询扩展 + qwen3-rerank 三开关；key 已在本地 `.env`）。
- **硬停止**：A-03 收口（或弹性未开始）即本会话结束——A-03 是 RAG 行为冻结点前最后一张实现票，此处收口并跑段边界 `bash scripts/verify-project.sh --full` 是最干净的会话边界。
- **明确不做**：#291 语料注入与 A-04（卡语料文本外部输入，语料闲时任务仍在生成中）；第 7 段（T21/A-06/A-07——A-06 依赖本会话的 A-01/A-02，留给下一会话在干净基线上做）；第 8 段长尾 T 票；#204/#32/#76/#134；任何桌面端改动。**不读取、不写入 `artifacts/corpus-v2/`**（语料闲时任务可能正在写该目录）。

## 当前 checkpoint（2026-09-02 晚刷新）

固定范围 69 个执行单元，19 个已收口（T01~T09、T18~T20、T22、U-01~U-05、#290=6e6d77e），剩余 50；#276/#282 为协调父票不计。第 3/4/5 段中仅剩 T17 待执行；#290 的段边界 verify-project.sh --full 已通过。语料 v2 闲时任务并行在跑（artifacts/corpus-v2/，与开发无写面冲突）。本段仅为导航快照，计数与完成判断以总序 §1.1 与 GitHub 实际状态为准。

## 0. 总原则

- 工作流唯一权威 = `AGENTS.md`（必读：执行车道、MANDATORY 工作流程 Step 1~6、阻塞处理、Key Rules）；执行顺序唯一权威 = `docs/working/2026-09-02-integrated-execution-order.md`（下称「总序」，含 §3.2 RAG 冻结点护栏）。冲突时：宪法 > 生产代码 > 各批次 spec > 总序。
- 状态三源对账：GitHub 票据（机器真源，仅 `stateReason=COMPLETED` 且关闭评论含 commit 与验证证据算完成）+ 总序 §3 ✅ 标记 + `progress.txt`；不一致先核对证据再修正。
- 本地开发模式（2026-08-13 起）：凭证缺失走 fail-open/mock 不阻塞，但**禁止以 mock 证据冒充真实验证结论**。
- 用户在场：spec 歧义、票面与代码事实冲突、heavy 审查 DONE_WITH_CONCERNS 的取舍、范围增删建议——列为问题向用户提出，等裁决；不自行选择较宽松的一方，也不擅自扩边界。

## 1. 固定开场协议（顺序固定）

1. `cd /Users/pp/Desktop/file/code/project/OmniCraft`；通读 `AGENTS.md`「活计划注册表」+ 总序全文。
2. `git log --oneline -15` + `git status`：确认主工作树干净；有未提交改动先弄清来源再决定，**禁止 git add . / git checkout -- 丢弃**；确认无遗留 worktree（`git worktree list`）。
3. `gh -R Leewwp/OmniCraft issue list --state all --limit 100 --json number,state,stateReason,title` 状态探测；逐票 `gh ... issue view <N>` 读全文（含 2026-09-02 整合修订小节）。候选顺序严格来自工作边界，不按 API 返回顺序；文档与票面依赖不一致 → 先问用户。
4. 从第一张未完成票（预期 T17）继续；禁止重做已完成工作；发现已完成项回归 → 记新缺陷排入段末处理。

## 2. 每票执行循环（一票一循环，完成前不切票）

1. **读票**：`gh -R Leewwp/OmniCraft issue view <N>` 全文；按票面「来源与权威」读对应 spec 节。
2. **读规则**：前端票先读 `design/ui-spec.md` 对应章节；触及信誉/上传/收藏/通知/搜索/判官/风控/Agent/安全/i18n 子系统先读 `docs/reference/business-rules.md` 对应小节。
3. **起环境**：先查 :8080/:3000 与 Postgres/Redis 健康实例，复用不重启；缺失再 `docker compose up -d postgres redis`（镜像走 docker.m.daocloud.io 代理）+ `go run cmd/server/main.go &` + `npm run dev &`；不为清端口杀来源不明进程。
4. **实现**：heavy（A-01）先写失败测试确认预期失败再最小实现；light 实现 + 补直接相关测试。规范照 AGENTS.md（Go: gofmt/统一错误 JSON；TS: strict/禁 any；SQL: GORM 参数化）。
5. **验证门（全过才算完）**：`go build ./... && go vet ./... && go test ./...`；`npm run build && npm run lint`；UI 票浏览器（MCP Playwright）验证 + 亮暗×1440/375 截图存 `screenshots/`（测试账号见 `~/Desktop/file/note/项目/omnicraft-测试账号.md`）；后端票 curl 正常+错误路径；改 `config.go`/`migrations/`/`routes.go` 后 `cd tools/doc-validator && go run . --fix`；**段边界**（T17 后第 5 段完结、A-03 后第 6 段实现线完结）跑 `bash scripts/verify-project.sh --full`。
6. **heavy 两阶段审查**：A-01 实现且门禁全过后派发两个只读子代理（规格符合性 + 代码质量），处理全部 DONE_WITH_CONCERNS（取舍拿不准 → 报用户裁决）；任一派发平台级错误（TLS/5xx/超时无输出）等 3~5 分钟重试 ≤3 次，仍失败报用户，**禁止跳过审查直接合并**。
7. **记录与提交**：`progress.txt` 追加条目；验证门全过才在总序 §3 追加 ✅ 与 commit hash；`git add <精确文件列表>`。heavy 从干净基线建 worktree `git worktree add /Users/pp/Desktop/file/code/project/OmniCraft-heavy-<票号> -b codex/heavy/<票号> <基线>`，分支上按里程碑打 `wip(#N):` commit 防中断丢失，审查通过由主 worktree squash 合并后清理 worktree（照 #290 模式）；light 在 main 按逻辑分批 commit。conventional 风格提交信息。
8. **关票**：`gh -R Leewwp/OmniCraft issue close <N> --reason completed -c "完成于 <hash>；验证证据：<一句话摘要>"`；阻塞票保持 OPEN。

## 3. 顺序与并行

- 边界内严格串行（T17 → A-01 → A-02 → [A-05 → A-03]）；唯一并行 = 同票只读子代理（审查/浏览器验证/事实核查）；一切写操作由你串行执行。
- 共享文件红线：A-02 与 T19 同属 Agent SSE 链路（行缓冲语义不回退）；A-03 触及检索管线（总序 §3.2：A-04 收口即冻结点，之后任何检索行为变更需重跑消融）。

## 4. 收尾与上下文压力协议

- 无闲时票据墙，但上下文膨胀会降低质量：**每票收口即完整落盘**（progress.txt + 总序 ✅ + commit + 关票），随时可以从任意票边界无损重开会话。
- 出现上下文压缩/总结提示，或你判断上下文已明显臃肿：完成当前票收口，输出「建议重开会话，下一票 = <票号>」后结束，不要硬撑。
- 平台级错误（TLS/5xx/超时）：等 3~5 分钟重试 ≤3 次；仍失败 → 落盘中断点后结束会话，向用户说明。
- 弹性票开做前自评：核心三票收口后若本会话已跨多个大票，宁可留给下一会话干净基线（A-03 尤其大，做一半的检索管线改动是最差状态）。

## 5. 阻塞处理

- 单票阻塞（凭证缺失可 fail-open 的除外/迁移冲突/契约冲突）：先判断是否方向性问题——是 → 列清选项问用户；纯技术性 → 记 `progress.txt` + 票面阻塞说明保持 OPEN，跳到边界内下一张依赖满足的票。
- 全局阻塞（postgres/redis 起不来、git 工作树损坏、连续 3 票同根因失败）：写清阻塞信息，结束会话等人工。
- 阻塞时禁止：提交未验证代码、勾选未完成步骤、修改票面既定范围。

## 6. 红线（违反任何一条即为事故）

1. 不 mock 冒充验证证据；不勾未验证的 checkbox。
2. 不重开已拍板的设计讨论；spec 疑似有误 → 记 issue + 问用户，不自行发挥。
3. 不动生产部署相关门（#76/#134 维持 deferred）；不产生桌面端/Tauri 改动。
4. 不使用 `git add .`；不 force push；不修改历史 commit。
5. 生产配置真值只读 `backend/config.yaml`，新限制走配置不硬编码。
6. i18n：新增 UI 字符串必须走 next-intl，zh/en 同步。
7. 不在 docs/ 根目录新建 .md；新文档进 `docs/working/` 并带失效日期头。
8. 凭证不写入任何会提交的文件（DashScope key 只从 `.env` 读）。
9. 不执行语料生成/注入；不读写 `artifacts/corpus-v2/`；#291/A-04 保持待外部输入状态。
10. 云端服务器（ssh omnicraft-server）本轮不做部署。

## 7. 完成条件

工作边界内全部票收口（至少 T17 + A-01 + A-02；弹性票视状态）+ 最后一张票的段边界 `verify-project.sh --full` 通过 + progress.txt/总序/GitHub 三源同步 → 输出总结（各票 commit、验证证据、边界内剩余项、下一会话建议起点——预期为 A-05/A-03 或第 7 段 A-06）后结束。

## 8. 命令速查

```bash
gh -R Leewwp/OmniCraft issue view <N>                        # 读票（必须带 -R）
cd backend && go build ./... && go vet ./... && go test ./...
cd frontend && npm run build && npm run lint
bash scripts/verify-project.sh --full                        # 段边界全量门
cd tools/doc-validator && go run . --fix                     # 改 config/migrations/routes 后
docker compose up -d postgres redis
```

（提示词正文到此结束）
