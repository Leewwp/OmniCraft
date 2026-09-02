# 任务书：related-contents.test.tsx CI 不稳定修复（Phase 2 硬前置）

> 创建日期：2026-09-02 ｜ **预计失效日期**: 2026-09-16（修复合入即失效）
> 车道：light ｜ 基线：main@c73fc61（开场先 `git log --oneline -3` 对账，若已前进以最新为准）
> 完成后由原会话复核（用户返回原会话执行 Phase 2 判定）。

## 一、任务一句话

修复 `frontend/tests/related-contents.test.tsx` 在 GitHub CI（ubuntu-latest / Node 20 / 全新 `npm ci`）上的不稳定失败，使 Frontend gates 与 project-gate 在 CI 稳定转绿。本地默认环境当前全绿、复现不出——**先构造出可靠的红，再修**。

## 二、背景与证据链（已核实，勿重复调查）

- 该文件由 ec76a6c（#90 媒体票）引入；测试为 node:test TAP 风格，经 `npm run test:unit`（= `node scripts/run-tests.mjs unit`）执行。
- CI 三跑三败、**败的用例还在变**（相邻用例轮番挂 = 典型测试隔离/共享状态/异步泄漏特征，而非固定逻辑 bug）：
  - run 33643480394 首跑：`not ok 354 - empty branch: both rows empty renders only the end hint, no block titles`
  - run 33643480394 重跑：`not ok 353 - embedded related row shares the single block container (no nested bordered section)`
  - run 33646037373（fe3468f）：`not ok 353`（同上）
- 本地（macOS / Node v26.7.0 / 既有 node_modules）：`npm run test:unit` 全套 EXIT=0（含上述两用例 ✔），单套约 2 分钟。
- CI 日志获取：`gh run view <run_id> --repo Leewwp/OmniCraft --log-failed | grep "not ok"`。
- main 现已启用 push/PR 自动触发 CI（run 33643480394 起可查）；backend/docs/ops 三门稳定绿，仅 frontend 被 flaky 打红。

## 三、复现方案（环境分歧三轴：Node 版本 / 全新 npm ci / linux）

按可用工具择优，**至少复现一次红并留存输出**再动手修：

1. **首选 Docker（一次命中三轴）**：`docker run --rm -m 4g -v "$PWD:/w" -w /w/frontend docker.m.daocloud.io/library/node:20 bash -lc "npm ci && npm run test:unit"`。注意：本机 Docker Hub 直连不通，**必须用 docker.m.daocloud.io 镜像**。
2. **nvm/fnm/volta 钉 Node 20**（若本机有）：`nvm use 20 && rm -rf node_modules && npm ci && npm run test:unit`（命中两轴）。
3. flaky 具概率性：复现与回归验证都要**连跑 ≥3 次**（`for i in 1 2 3; do ... || break; done`）。
4. 若三轴都命中仍全绿，考虑 macOS↔linux 差异，用方案 1 兜底；仍不红则停下来向用户报告调查结论，**禁止在未复现的情况下盲改**。

## 四、修复边界

- 根因大概率在测试侧（隔离/顺序/异步泄漏/Node 20 行为差），也可能暴露组件真 bug——**先定位再决定改哪侧**；若确认组件真 bug，允许改 `frontend/` 组件代码。
- **禁止**：skip/quarantine 该文件或用例、删除测试、放宽断言、`|| true` 类达标手法——修复必须让测试真实通过。
- 若根因是测试依赖了 Node 26-only 行为：修测试兼容 Node 20（engines 口径 Node 20+，CI 钉 20 是权威基线）。
- **禁改**：`.github/workflows/*`、`scripts/ci/*`（CI 触发契约刚落地，契约测试必须保持绿）、`backend/**`（本任务纯前端域）。

## 五、工作流（按项目惯例）

1. 开一张 light 票：`gh issue create --repo Leewwp/OmniCraft --title "[QA] related-contents.test.tsx CI 不稳定修复（Phase 2 前置）" --body "背景见 docs/working/2026-09-02-related-contents-flaky-fix-prompt.md"`。
2. **推荐走 PR 路径**（顺便实弹验证 Phase 1 刚启用的 pull_request 触发）：`git checkout -b fix/related-contents-flaky` → 提交 → 推分支 → `gh pr create` → 等待 PR 上 CI 绿 → `gh pr merge --squash`。若 PR 流程受阻可回退直推 main，如实记录。
3. conventional commit（`fix(#票号): ...` 或 `test(#票号): ...`）；progress.txt 按标准格式补条目。
4. 关票带证据：`gh issue close <N> --repo Leewwp/OmniCraft --reason completed -c "完成于 <hash>；验证：<CI run ID 绿 + 本地/复现环境连跑结果>"`。

## 六、验收硬指标（全过才算完成）

- [ ] 复现成功：修复前在复现环境至少一次红（留存输出摘要）。
- [ ] 复现环境连跑 ≥3 次全绿（修复后）。
- [ ] 本地默认环境（Node 26/既有 node_modules）`npm run test:unit` / `lint` / `lint:ui` / `build` 全绿，pass 基线不降（当前 462 pass、0 新增 skip）。
- [ ] 合入后 GitHub CI：Frontend gates = success 且 project-gate = success（记录 run ID；若怀疑残余 flaky，rerun 一次再确认）。
- [ ] `bash scripts/ci/verify-workflows.tests.sh` 仍绿（证明没误伤 CI 契约）。
- [ ] progress.txt 条目 + 票据 CLOSED。

## 七、本机坑位清单（新会话必读）

- **git push 凭证**：非交互 push 必须内嵌 token（plain push 死于 ghfast 凭证缺失）：
  `git push "https://x-access-token:$(gh auth token)@ghfast.top/https://github.com/Leewwp/OmniCraft.git" <branch>`
- **gh CLI 为旧版**：命令需显式 `--repo Leewwp/OmniCraft`；部分新 flag（如 repo view 的 `-R` 简写）不可用，用位置参数。
- **测试必须走项目 runner**：`npm run test:unit`；**不要** `npx vitest`（项目未用，npx 拉最新版会因 rolldown binding 崩）。
- Docker 镜像一律走 docker.m.daocloud.io（Docker Hub/GCR 直连不通）。
- 开场先 `git status`：若出现非本任务的脏文件（并发会话所有物），停下报告，不得代提交。

## 八、汇报格式（完成后 ≤10 行）

根因一句话 / 改了什么（文件+行为）/ 复现证据（修复前红）/ 回归证据（环境×次数）/ CI run ID / 是否走通 PR 触发路径 / 遗留。
