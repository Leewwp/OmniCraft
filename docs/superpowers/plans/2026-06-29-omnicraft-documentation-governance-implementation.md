# OmniCraft 文档治理体系实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 OmniCraft 项目从 ~80 份混杂文档重构为 12 个权威来源类别（含约 20+ 份实际文件）+ 自动化校验工具的可持续治理体系

**Architecture:** 三个 Phase 串行推进 — Phase 1 修复 48 项 P0 阻塞问题并完成结构性准备，Phase 2 逐领域统一文档内容消除矛盾，Phase 3 实现 doc-validator 工具建立自动防退化机制

**Design spec:** `docs/superpowers/specs/2026-06-29-omnicraft-documentation-governance-design.md`

**Tech Stack:** Go 1.25+ (doc-validator), Python (辅助脚本), Git
**Shell:** 本文档命令默认使用 Git Bash（Windows）或 bash（Linux/macOS）。PowerShell 命令已单独标注（如 Task 6 Step 1）。执行 Agent 根据当前平台自动选择对应语法。

---

## 文件结构总览

```
新增文件:
├── tools/                                         # P3-1: 工具目录（需创建，当前不存在）
├── tools/doc-validator/main.go                    # P3-1: 校验工具入口
├── tools/doc-validator/rules/config_sync.go       # P3-1: 配置同步检查
├── tools/doc-validator/rules/schema_sync.go       # P3-1: Schema 同步检查
├── tools/doc-validator/rules/route_sync.go        # P3-1: 路由同步检查
├── tools/doc-validator/rules/token_refs.go        # P3-3: Token 引用检查
├── tools/doc-validator/rules/cross_refs.go        # P3-3: 交叉引用检查
├── tools/doc-validator/README.md                  # P3-1: 工具说明
├── docs/GLOSSARY.md                              # P1-3: 项目术语表
├── docs/working/README.md                         # P1-4: 工作目录说明
├── docs/adr/README.md                             # P1-4: ADR 索引
├── docs/archive/README.md                         # P1-4: 归档索引
├── docs/adr/0001-document-governance.md            # P2-6: 首批 ADR
├── docs/working/2026-06-29-governance-verification-log.md  # Task 17: 最终验证记录

修改文件:
├── CLAUDE.md                                      # P1-5: 加入权威表 + 规则
├── AGENTS.md                                      # P1-5: 加入权威表 + 规则（项目根目录，面向所有 Agent）
├── architecture.md                                # P1-2/P3-2: 内容对齐 + 自动区标记
├── design/design-system.md                        # P2-1: token 补齐
├── design/ui-spec.md                              # P2-1: token 引用修正
├── .specify/memory/constitution.md                # P2-2: 版本号和 Changelog
├── backend/config.yaml                            # P2-4: 字段补全
├── backend/config/config.go                       # P2-4: 结构体字段补全
├── backend/internal/middleware/auth.go            # P1-1: CS-001 修复
├── backend/internal/repository/search_repo.go     # P1-1: CS-002 修复
├── backend/internal/handler/*.go                  # P1-1: CS-003 修复
├── backend/internal/service/reputation_service.go # P1-1: CS-004 修复
├── docs/deploy/nginx.omnicraft.single-server.conf # P1-1: CS-005 修复
├── docs/deploy/single-server-beta-runbook.md      # P1-1: CS-007 修复
├── progress.txt                                   # P2-5: 补全缺失记录 + 拆分合并条目
└── task.json                                      # P2-5: passes 说明更新

不受治理的 Agent 文件（Next.js 运行时生成，非项目文档）:
├── frontend/AGENTS.md                             # Next.js 自动生成，描述 Next.js 自身 API 变更
├── frontend/CLAUDE.md                             # 仅含 @AGENTS.md 引用，指向 frontend/AGENTS.md
```
> **注意**：`frontend/AGENTS.md` 与项目根目录 `AGENTS.md` 是不同文件、不同用途。前者是 Next.js 框架自动生成的运行时说明（面向在 frontend/ 子目录工作的 Agent），后者是项目级 Agent 工作指南。两者不合并、不互替。
```

---

## Phase 1: 修复阻塞级安全问题 + 结构性准备（本周）

### Task 1: 修复 CS-001 — AuthRequired 中间件 fail-open

**依赖:** 无
**设计参考:** consolidated-fix-collection CS-001
> ⚠️ **注意**: 若 Task 8（目录重组）已先执行，参考文件位于 `docs/archive/2026-06-29-consolidated-fix-collection.md`。该文件为 git 跟踪内容，始终可从历史检出。

**Files:**
- Modify: `backend/internal/middleware/auth.go`

- [ ] **Step 1: 阅读当前 auth.go 中间件逻辑**

```bash
grep -n "Redis\|database\|fallback\|JWT\|claims" backend/internal/middleware/auth.go
```

定位 Redis/DB 不可达时的回退逻辑。问题：当 Redis 和数据库均不可达时，中间件回退到 JWT claims 放行请求而非拒绝。

- [ ] **Step 2: 修改中间件——Redis/DB 均不可达时返回 503**

找到 AuthRequired 函数中检查 Redis/DB 可用性的代码段，改为：

```go
// 检查 Redis 和 DB 是否可用
redisAvailable := checkRedis(ctx)
dbAvailable := checkDB(ctx)

if !redisAvailable && !dbAvailable {
    c.JSON(http.StatusServiceUnavailable, gin.H{
        "code":    "AUTH_STATUS_UNAVAILABLE",
        "message": "认证服务暂时不可用，请稍后重试",
    })
    c.Abort()
    return
}

// 至少一个可用时，继续正常的认证流程
```

- [ ] **Step 3: 运行 auth 中间件测试**

```bash
cd backend && go test ./internal/middleware/ -run Auth -v
```

Expected: PASS

- [ ] **Step 4: 运行全部中间件测试**

```bash
cd backend && go test ./internal/middleware/... -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/middleware/auth.go
git commit -m "fix: AuthRequired middleware returns 503 when Redis and DB unavailable (CS-001)"
```

---

### Task 2: 修复 CS-002 — SQL 注入风险

**依赖:** 无（可与 Task 1 并行）
**设计参考:** consolidated-fix-collection CS-002
> ⚠️ **注意**: 若 Task 8（目录重组）已先执行，参考文件位于 `docs/archive/2026-06-29-consolidated-fix-collection.md`。该文件为 git 跟踪内容，始终可从历史检出。

**Files:**
- Modify: `backend/internal/repository/search_repo.go`

- [ ] **Step 1: 定位 SQL 注入风险点**

```bash
# 检测字符串拼接 SQL
grep -n "fmt.Sprintf.*SELECT\|fmt.Sprintf.*WHERE\|fmt.Sprintf.*FROM\|fmt.Sprintf.*INSERT\|fmt.Sprintf.*UPDATE\|fmt.Sprintf.*DELETE" backend/internal/repository/search_repo.go
# 检测所有 Raw SQL 调用（可能包含拼接）
grep -n "\.Raw(" backend/internal/repository/search_repo.go
# 检测字符串拼接形式的查询
grep -n "+\s*\".*SELECT\|+\s*\".*WHERE\|+\s*\".*FROM" backend/internal/repository/search_repo.go
```

> **注意**：上述 grep 为辅助定位工具，不能替代人工审查。以下情况 grep 会漏报：(1) 多行拼接 SQL（字符串跨行使用 `+` 连接）；(2) 使用 `%v`/`%s` 等其他格式动词的 `fmt.Sprintf`；(3) 变量间接拼接（`query := baseQuery + where`）。Step 2 替换时必须通读全部函数，确保无一遗漏。

- [ ] **Step 2: 逐处替换为 GORM 参数化查询**

典型修复模式：

```go
// WRONG: 字符串拼接
query := "SELECT * FROM content_items WHERE title LIKE '%" + keyword + "%'"
db.Raw(query).Scan(&results)

// CORRECT: GORM 参数化
db.Where("title LIKE ?", "%"+keyword+"%").Find(&results)
```

对于全文搜索 tsquery：

```go
db.Where("to_tsvector('chinese', title) @@ plainto_tsquery('chinese', ?)", keyword).Find(&results)
```

- [ ] **Step 3: 运行搜索仓库测试**

```bash
cd backend && go test ./internal/repository/ -run Search -v
```

Expected: PASS

- [ ] **Step 4: 编译检查**

```bash
cd backend && go build ./...
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add backend/internal/repository/search_repo.go
git commit -m "fix: replace SQL string concatenation with GORM parameterized queries (CS-002)"
```

---

### Task 3: 修复 CS-003 — err.Error() 暴露

**依赖:** 无（可与 Task 1/2 并行）
**设计参考:** consolidated-fix-collection CS-003
> ⚠️ **注意**: 若 Task 8（目录重组）已先执行，参考文件位于 `docs/archive/2026-06-29-consolidated-fix-collection.md`。该文件为 git 跟踪内容，始终可从历史检出。

**Files:**
- Modify: `backend/internal/handler/` 下多个文件

- [ ] **Step 1: 扫描所有 err.Error() 直接暴露点**

```bash
grep -rn "err.Error()" backend/internal/handler/ --include="*.go" | grep -v "_test.go"
```

- [ ] **Step 2: 逐文件替换为标准错误信封**

```go
// WRONG
c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})

// CORRECT
c.JSON(http.StatusInternalServerError, gin.H{
    "code":    "INTERNAL_ERROR",
    "message": "服务器内部错误，请稍后重试",
})
slog.Error("handler error", "error", err, "trace_id", ctx.Value("trace_id"))
```

不同场景使用对应错误码：数据库错误 → `DATABASE_ERROR`，外部服务 → `SERVICE_UNAVAILABLE`，参数验证 → `INVALID_PARAMETER`。

- [ ] **Step 3: 逐 handler 文件运行测试**

```bash
cd backend && go test ./internal/handler/... -v
```

Expected: all PASS

- [ ] **Step 4: 全量测试 + 编译**

```bash
cd backend && go build ./... && go vet ./... && go test ./...
```

Expected: all PASS

- [ ] **Step 5: 精确暂存并 Commit**

Step 1 的 grep 输出列出所有修改过的 handler 文件。精确暂存这些文件（而非整个目录）：

```bash
git add <Step 1 中 grep 输出的具体文件列表>
git commit -m "fix: replace err.Error() exposure with sanitized error envelopes (CS-003)"
```

---

### Task 4: 修复 CS-004 — 信誉分配置硬编码

**依赖:** 无（可与 Task 1-3 并行）
**设计参考:** consolidated-fix-collection CS-004
> ⚠️ **注意**: 若 Task 8（目录重组）已先执行，参考文件位于 `docs/archive/2026-06-29-consolidated-fix-collection.md`。该文件为 git 跟踪内容，始终可从历史检出。

**Files:**
- Modify: `backend/internal/service/reputation_service.go`
- Modify: `backend/config.yaml`
- Modify: `backend/config/config.go`

- [ ] **Step 1: 在 config.go 中补全 ReputationConfig 结构体字段**

```bash
grep -n "ReputationConfig\|reputation" backend/config/config.go
```

补齐字段：

```go
type ReputationConfig struct {
    MinScoreForInteraction      int `mapstructure:"min_score_for_interaction"`
    QualityContentThreshold     int `mapstructure:"quality_content_threshold"`
    QualityCommentThreshold     int `mapstructure:"quality_comment_threshold"`
    ScoreQualityContent         int `mapstructure:"score_quality_content"`
    ScoreQualityComment         int `mapstructure:"score_quality_comment"`
    ScorePRAccepted             int `mapstructure:"score_pr_accepted"`
    ScoreReportApproved         int `mapstructure:"score_report_approved"`
    ScoreTagRecognized          int `mapstructure:"score_tag_recognized"`
    ScoreJudgeAccuracy          int `mapstructure:"score_judge_accuracy"`
    ScoreRehabCourse            int `mapstructure:"score_rehab_course"`
    PenaltyMaliciousContent     int `mapstructure:"penalty_malicious_content"`
    PenaltyMaliciousPR          int `mapstructure:"penalty_malicious_pr"`
    PenaltyMaliciousComment     int `mapstructure:"penalty_malicious_comment"`
    PenaltyMaliciousReport      int `mapstructure:"penalty_malicious_report"`
    PenaltyMaliciousTagReport   int `mapstructure:"penalty_malicious_tag_report"`
    PenaltyJudgeError           int `mapstructure:"penalty_judge_error"`
    RepeatViolationWindowDays   int `mapstructure:"repeat_violation_window_days"`
    RepeatViolationThreshold    int `mapstructure:"repeat_violation_threshold"`
    RepeatViolationExtraPenalty int `mapstructure:"repeat_violation_extra_penalty"`
}
```

- [ ] **Step 2: 在 config.yaml 中补全对应默认值**

```bash
grep -n "reputation:" backend/config.yaml
```

在 `reputation:` 段补充：

```yaml
reputation:
  min_score_for_interaction: 3
  quality_content_threshold: 10
  quality_comment_threshold: 5
  score_quality_content: 3
  score_quality_comment: 2
  score_pr_accepted: 3
  score_report_approved: 1
  score_tag_recognized: 1
  score_judge_accuracy: 1
  score_rehab_course: 1
  penalty_malicious_content: -3
  penalty_malicious_pr: -3
  penalty_malicious_comment: -2
  penalty_malicious_report: -2
  penalty_malicious_tag_report: -1
  penalty_judge_error: -1
  repeat_violation_window_days: 7
  repeat_violation_threshold: 2
  repeat_violation_extra_penalty: -1
```

- [ ] **Step 3: 替换 reputation_service.go 中的硬编码**

```bash
grep -n "reputation\|score\|penalty\|Award\|Penalize" backend/internal/service/reputation_service.go
```

模式：

```go
// WRONG
func (s *ReputationService) AwardQualityContent(userID uint) {
    s.repo.AddScore(userID, 3, "quality_content")
}

// CORRECT
func (s *ReputationService) AwardQualityContent(userID uint) {
    score := s.config.Reputation.ScoreQualityContent
    s.repo.AddScore(userID, score, "quality_content")
}
```

- [ ] **Step 4: 运行测试**

```bash
cd backend && go test ./internal/service/... -v
cd backend && go build ./...
```

Expected: all PASS, no build errors

- [ ] **Step 5: Commit**

```bash
git add backend/internal/service/reputation_service.go backend/config.yaml backend/config/config.go
git commit -m "fix: read all reputation scores from config.yaml instead of hardcoding (CS-004)"
```

---

### Task 5: 修复 CS-005 nginx 安全头 + CS-007 SMTP 补充

**依赖:** 无（可与 Task 1-4 并行）

**Files:**
- Modify: `docs/deploy/nginx.omnicraft.single-server.conf`
- Modify: `docs/deploy/single-server-beta-runbook.md`

- [ ] **Step 1: 在 nginx SSL server block 添加安全头**

```bash
grep -n "server_name\|listen 443\|ssl_" docs/deploy/nginx.omnicraft.single-server.conf
```

在 SSL server block 的合适位置添加：

```nginx
# Security headers
add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self'; connect-src 'self'" always;
add_header X-Frame-Options "DENY" always;
add_header X-Content-Type-Options "nosniff" always;
add_header Referrer-Policy "strict-origin-when-cross-origin" always;
add_header X-XSS-Protection "1; mode=block" always;
add_header Permissions-Policy "camera=(), microphone=(), geolocation=()" always;
```

- [ ] **Step 2: 验证 nginx 配置**

```bash
grep -c "add_header" docs/deploy/nginx.omnicraft.single-server.conf
```

Expected: >= 6

- [ ] **Step 3: 补充 runbook SMTP 环境变量**

```bash
grep -n "SMTP\|smtp" docs/deploy/single-server-beta-runbook.md
```

将仅有 `SMTP_PASSWORD` 的地方替换为完整清单：

```markdown
## SMTP 环境变量

| 变量 | 说明 | 示例 |
|------|------|------|
| SMTP_HOST | SMTP 服务器地址 | smtp.example.com |
| SMTP_PORT | SMTP 端口（通常 587） | 587 |
| SMTP_USERNAME | SMTP 认证用户名 | noreply@example.com |
| SMTP_PASSWORD | SMTP 认证密码 | （从密钥管理系统获取） |
| SMTP_FROM_EMAIL | 发件人地址 | noreply@example.com |
```

- [ ] **Step 4: Commit**

```bash
git add docs/deploy/nginx.omnicraft.single-server.conf docs/deploy/single-server-beta-runbook.md
git commit -m "fix: add security headers to nginx and complete SMTP env vars (CS-005, CS-007)"
```

---

### Task 6: 修复 AD-001~007 — architecture.md 与代码严重脱节

**依赖:** 无（架构文档对齐不依赖代码内部实现细节的变更；与其他 Phase 1 任务可完全并行）

**Files:**
- Modify: `architecture.md`

- [ ] **Step 1: 逐字段对比 content_items schema（AD-001, AD-007）**

列出所有 migration 文件：

```bash
# Linux/macOS
ls backend/migrations/*.sql | sort
# Windows PowerShell
Get-ChildItem backend/migrations/*.sql | Sort-Object Name | Select-Object -ExpandProperty Name
```

逐字段与 architecture.md §4.3 对照。修正：download_count 描述、补充 deleted_at/scheduled_at 字段、对齐索引策略。

- [ ] **Step 2: 统一 URL Scheme 描述（AD-006）**

```bash
grep -n "url_scheme\|URL Scheme\|omnicraft://" architecture.md
grep -n "url_scheme" docs/superpowers/specs/2026-05-30-omnicraft-dual-track-beta-design.md
```

以 Beta roadmap Cross-Plan Contract 版本为准，统一三处。

- [ ] **Step 3: 修复 §7 幽灵字段（AD-004, AD-005）**

逐个对比 architecture.md §7 与 config.go 的 mapstructure tag。config.go 有而文档无的 → 补充。文档有而 config.go 无的 → 删除。字段名不一致 → 以 config.go tag 为准。

```bash
grep "mapstructure:" backend/config/config.go | sed 's/.*mapstructure:"\([^"]*\)".*/\1/' | sort
```

- [ ] **Step 4: 补全 §3.2 路由清单**

```bash
grep -n "\.GET\|\.POST\|\.PUT\|\.PATCH\|\.DELETE" backend/internal/handler/routes.go
```

补全缺失的 feedback 和 audit-logs 相关 7 条路由。

- [ ] **Step 5: 补全 collections 表信息（AD-002）**

在 architecture.md §4 中为 collections 和 collection_items 补充 DDL。若代码中尚无，标注：`> **状态**: 设计已确认，代码尚未实现。见 DEC-010。`

- [ ] **Step 6: 验证 admin 路由在 (protected) 内（AD-003）**

```bash
ls frontend/app/\(protected\)/admin/ 2>/dev/null
ls frontend/app/admin/ 2>/dev/null
```

- [ ] **Step 7: Commit**

```bash
git add architecture.md
git commit -m "fix: align architecture.md with code - schema, config fields, URL scheme, routes (AD-001~007)"
```

---

### Task 7: 创建项目术语表（GLOSSARY.md）

**依赖:** 无（可与 Task 1-6 并行）

**Files:**
- Create: `docs/GLOSSARY.md`

- [ ] **Step 1: 从已有文档提取术语**

```bash
grep -roh "判官\|众裁\|信誉分\|二创\|原创区\|赛博判官\|素质建设" docs/ CLAUDE.md architecture.md design/ --include="*.md" 2>/dev/null | sort | uniq -c | sort -rn
```

- [ ] **Step 2: 编写 GLOSSARY.md**

```markdown
# OmniCraft 项目术语表

> **权威等级**: authoritative（术语定义的唯一来源）
> **最后更新**: 2026-06-29

## 核心概念

| 术语 | 英文 | 定义 | 使用场景 |
|------|------|------|---------|
| 判官 | Judge | 参与众裁投票的用户 | 禁止使用"审核员"/"评审员" |
| 众裁 | Crowd Judgment | 用户集体投票判定内容是否违规 | 赛博判官功能模块 |
| 信誉分 | Reputation Score | 用户行为信用评分，初始 10 分 | 权限控制、内容发布门槛 |
| 创作者工作室 | Creator Studio | 统一内容创作和管理工作空间 (/studio) | 替代 /publish 和 /dashboard |
| 二创 | Fanwork | 基于原创内容的二次创作 | 二创发布、来源绑定 |
| 原创区 | Original Zone | 发布原创内容的区域 | content_items.zone = 'original' |
| 素质建设 | Rehabilitation | 信誉分恢复课程系统 | /rehab 路由 |
| PR | Pull Request | 协同修改请求 | 协作管理模块 |

## 禁止混用的术语对

| 正确 | 错误 |
|------|------|
| 判官 | 审核员、评审员 |
| 众裁 | 审核、评审 |
| 二创 | 二次创作、衍生作品 |
| 信誉分 | 信用分、积分 |
| 创作者工作室 | 发布页、仪表盘 |

### 易混淆概念辨析

| 术语 A | 术语 B | 区别 |
|--------|--------|------|
| Beta | MVP | **Beta** 是开发阶段（功能完整、修 Bug 中）；**MVP** 是产品范围（最小可行产品）。当前项目处于 Beta 加固阶段，不可称为 MVP。 |
| 原创区 (Original Zone) | 二创区 (Fanwork Zone) | **原创区**是发布原创内容的区域（`zone='original'`）；**二创区**是基于原创内容的二次创作区域（`zone='fanwork'`）。不能混用 zone 字段值。 |
```

- [ ] **Step 3: Commit**

```bash
git add docs/GLOSSARY.md
git commit -m "docs: create project glossary with unified terminology"
```

---

### Task 8: 重组目录结构 + 归档旧文档

**依赖:** 无（可与 Task 1-7 并行）

**Files:**
- Create: `docs/working/README.md`, `docs/adr/README.md`, `docs/archive/README.md`
- Move: 多份文件移至 `docs/archive/`

- [ ] **Step 1: 创建新目录**

```bash
mkdir -p docs/working docs/adr docs/archive
```

- [ ] **Step 2: 创建三个 README.md**

`docs/working/README.md` — 说明临时文档的用途、命名规则、生命周期（默认 +2 月失效）。
`docs/adr/README.md` — ADR 命名规则（NNNN-title.md）和索引表模板。
`docs/archive/README.md` — 归档原因分类（completed/superseded/expired/obsolete）和归档索引。

- [ ] **Step 3: 移动文件到 archive/**

```bash
mv docs/iteration-review docs/archive/iteration-review
mv docs/2026-06-24-* docs/archive/ 2>/dev/null
mv docs/2026-06-25-* docs/archive/ 2>/dev/null
mv docs/2026-06-28-* docs/archive/ 2>/dev/null
mv docs/2026-06-29-consolidated-fix-collection.md docs/archive/ 2>/dev/null
mv docs/futureWork docs/archive/futureWork 2>/dev/null
mv docs/review docs/archive/review 2>/dev/null
mv docs/agents docs/archive/agents 2>/dev/null
mv design/ui-design-prompt.md docs/archive/ 2>/dev/null
mv design/doc-review-prompt.md docs/archive/ 2>/dev/null

# 验证：确认源位置已无残留
ls docs/iteration-review 2>/dev/null && echo "WARNING: iteration-review still exists" || echo "OK: iteration-review moved"
ls docs/2026-06-*.md 2>/dev/null && echo "WARNING: root dated docs remain" || echo "OK: no root dated docs"
ls docs/futureWork docs/review docs/agents 2>/dev/null && echo "WARNING: obsolete dirs remain" || echo "OK: obsolete dirs moved"
```

- [ ] **Step 4: 验证 docs/ 根目录干净**

```bash
ls docs/*.md
```

Expected: 0 或 1 个文件。若 Task 7 已完成则仅有 `docs/GLOSSARY.md`；若 Task 7 尚未完成则为空。不应出现其他 .md 文件。

- [ ] **Step 5: 精确暂存并 Commit**

```bash
# 新增的 README
git add docs/working/README.md docs/adr/README.md docs/archive/README.md

# 移动到 archive/ 的文件（完整路径，必须逐条执行）
git add docs/iteration-review docs/archive/iteration-review
git add docs/2026-06-24-omnicraft-design-review-merged.md docs/archive/2026-06-24-omnicraft-design-review-merged.md
git add docs/2026-06-24-design-review-decisions.md docs/archive/2026-06-24-design-review-decisions.md
git add docs/2026-06-24-omnicraft-documentation-review.md docs/archive/2026-06-24-omnicraft-documentation-review.md
git add docs/2026-06-28-comprehensive-documentation-audit.md docs/archive/2026-06-28-comprehensive-documentation-audit.md
git add docs/2026-06-29-consolidated-fix-collection.md docs/archive/2026-06-29-consolidated-fix-collection.md
git add docs/futureWork docs/archive/futureWork
git add docs/review docs/archive/review
git add docs/agents docs/archive/agents
git add design/ui-design-prompt.md docs/archive/ui-design-prompt.md
git add design/doc-review-prompt.md docs/archive/doc-review-prompt.md

# 注意：Step 3 先执行 mv（物理移动文件），再执行上述 git add（将移动操作暂存到 git）
# 如有 docs/ 根目录下其他 .md 文件需要清理，先在 Step 3 的 mv 列表中补充，再在此处补充 git add

git commit -m "docs: restructure directory layout and archive obsolete documents"
```

---

---

## Phase 2: 文档精简与内容统一（本周—下周）

### Task 9: 更新 CLAUDE.md 和 AGENTS.md

**依赖:** Task 7（GLOSSARY.md 路径确定）、Task 8（目录结构确定）

**Files:**
- Modify: `CLAUDE.md`
- Modify: `AGENTS.md`

- [ ] **Step 1: 找到 CLAUDE.md 插入位置**

```bash
grep -n "## Key Rules\|## 项目结构\|### Step 1\|### Step 3\|### Step 6" CLAUDE.md
```

- [ ] **Step 2: 在 Key Rules 之前插入文档权威源表格**

插入以下内容（**不嵌入完整规则文本**，仅放快速参考摘要 + 引用指针；完整权威表和冲突解决规则以设计规格 §4 为准）：

```markdown
## 文档权威源（冲突时以此为准）

> **完整权威文档登记表和冲突解决规则**以
> `docs/superpowers/specs/2026-06-29-omnicraft-documentation-governance-design.md` §4 为准。
> Agent 遇到文档矛盾时必须查阅该文档获取完整规则，以下为快速参考摘要。

### 权威文档快速索引

| 领域 | 唯一权威文档 |
|------|------------|
| 设计原则与不可妥协约束 | .specify/memory/constitution.md |
| 系统架构（API / Schema / 配置架构） | architecture.md |
| Agent 工作流与业务规则（Claude Code） | CLAUDE.md |
| Agent 工作流与业务规则（其他 Agent） | AGENTS.md |
| 视觉设计 token | design/design-system.md |
| UI 组件和页面规格 | design/ui-spec.md |
| 运行时配置真实值 | backend/config.yaml |
| 数据库 Schema 真实定义 | backend/migrations/*.sql |
| 功能设计输入 | docs/superpowers/specs/*.md |
| Beta 路线图和实施计划 | docs/superpowers/plans/*.md |
| 部署运维 | docs/deploy/single-server-beta-runbook.md |
| 术语定义 | docs/GLOSSARY.md |

### 冲突解决优先级（摘要）

1. 宪法（不可妥协约束）> 一切
2. 生产代码（config.yaml / migrations / routes.go）> 文档
3. architecture.md > design/ > specs/ > plans/
4. 同目录多份文档：日期最新优先

**遇到矛盾 → 查设计规格 §4 → 记录为 issue，不做自行发挥。**
```

- [ ] **Step 3: 插入文件命名规范（在项目结构附近）**

```markdown
## 文件命名与存放规范

- 新建 .md 文件放在 docs/working/ 目录下，文件名格式 YYYY-MM-DD-<scope>-<type>.md
- 禁止在 docs/ 根目录创建新 .md 文件（docs/GLOSSARY.md 除外）
- 禁止创建与已有权威文档同领域的第二份文档
- 临时文档在头部注明预计失效日期（默认 +2 月）
```

- [ ] **Step 4: 嵌入 3 条 Agent 规则（Phase 3 前版本）**

在 Step 1（任务选择步骤）末尾添加：规则 3（矛盾时查权威表）
在 Step 3（代码实现步骤）末尾添加：规则 1 — **两阶段行为**：

```markdown
### 规则 1：修改代码后同步架构文档

**Phase 3 前（当前）**：修改 config.go / migrations / routes.go 后，手工确认 architecture.md 对应区段是否需要同步更新，在 commit message 中注明「arch-doc-check: §X 已检查」或「arch-doc-check: 无需更新」。

**Phase 3 后（doc-validator 就绪）**：运行 `cd tools/doc-validator && go run . --fix` 自动刷新 architecture.md 自动区。
```
> 此分阶段设计确保规则在 doc-validator 工具就绪前后均可执行。Phase 3 完成后（Task 15），由 Task 15 的代理人将规则 1 更新为仅保留 `go run . --fix` 版本。

在 Step 6（提交步骤）之前添加：规则 2（新 .md 按规范存放）

- [ ] **Step 5: 同步更新 AGENTS.md**

对 AGENTS.md 执行与 Step 2-4 相同的修改。

> **注意**：此 Task 仅修改项目根目录 `AGENTS.md`（项目级 Agent 工作指南）。`frontend/AGENTS.md` 为 Next.js 框架自动生成的运行时说明（面向 frontend/ 子目录），不在本文档治理范围内。两者用途不同，不需合并或同步。

- [ ] **Step 6: Commit**

```bash
git add CLAUDE.md AGENTS.md
git commit -m "docs: add document authority table, naming conventions, and agent rules to CLAUDE.md/AGENTS.md"
```

---

### Task 10: 修复 DS-001~005 — 设计系统 token 和组件规范

**依赖:** Task 7（GLOSSARY.md 术语统一）

**Files:**
- Modify: `design/design-system.md`
- Modify: `design/ui-spec.md`

- [ ] **Step 1: 修复 ui-spec.md token 引用格式（DS-001）**

**Step 1a: 先读取 design-system.md 确认实际 token 命名约定**

```bash
grep -n "^\$--\|\-\-color\|\-\-spacing\|\-\-radius\|\-\-font\|\.color-\|\.spacing-\|\.radius-\|\.font-" design/design-system.md | head -20
```

确定项目实际使用的 token 命名格式（CSS 变量连字符 `--color-primary-500` vs Tailwind 点号 `color.primary.500` vs 其他约定）。

**Step 1b: 以 design-system.md 为准，统一 ui-spec.md 中所有 token 引用**

```bash
grep -n "color\.\|spacing\.\|radius\.\|font\." design/ui-spec.md | head -20
```

**统一规则**：
- 若 design-system.md 使用 CSS 变量格式 `--color-primary-500` → ui-spec.md 中对应引用改为 `var(--color-primary-500)`
- 若 design-system.md 使用 Tailwind 格式 `color.primary.500` → ui-spec.md 中引用统一为 `color-primary-500`（Tailwind class 标准连字符格式）
- 对 design-system.md 中不存在的 token，在 design-system.md 补充定义
- 命名规则优先级：**design-system.md 已有格式 > Tailwind 标准连字符 > CSS 变量连字符**；选定后在当前任务内统一，不跨任务自行发挥

- [ ] **Step 2: 统一圆角值（DS-002）**

以 design-system.md 为准，修正 ui-spec.md 中的冲突值。

- [ ] **Step 3: 统一字体栈（DS-003）**

以 design-system.md 版本（含中文字体回退）为准，更新 ui-spec.md 中的字体栈。

- [ ] **Step 4: 为缺失组件补充规范（DS-004）**

对 14+ 缺失 Component 规范的组件，补充：Props 表格、状态变体（default/hover/active/disabled）、响应式行为。

- [ ] **Step 5: 替换模板默认值（DS-005）**

对仍为模板默认值的组件规范，填入真实设计规格。

- [ ] **Step 6: Commit**

```bash
git add design/design-system.md design/ui-spec.md
git commit -m "fix: unify design tokens, font stack, border radius; add missing component specs (DS-001~005)"
```

---

### Task 11: 修复 DC-001~005 — 宪法和核心文档一致性

**依赖:** Task 9（CLAUDE.md 已更新）

**Files:**
- Modify: `.specify/memory/constitution.md`
- Modify: `AGENTS.md`（DC-004：修正文件名交叉引用）
- Modify: `design/ui-spec.md`（DC-005：颜色来源引用修正）
- May modify: 其他包含 CLAUDE.md/AGENTS.md 错误引用的 .md 文件（DC-004 Step 4 grep 确定）

- [ ] **Step 1: 更新 Constitution SYNC IMPACT REPORT 版本号（DC-001）**

更新为完整变更链 1.0.0 → 1.3.0。

- [ ] **Step 2: 修正模板同步声明（DC-002）**

若声明"已同步"但实际未同步，改为准确状态。

- [ ] **Step 3: 补全 Changelog（DC-003）**

确保所有版本变更都有记录。

- [ ] **Step 4: 修复 CLAUDE.md/AGENTS.md 文件名引用混淆（DC-004）**

扫描项目中所有 .md 文件，查找对 CLAUDE.md 和 AGENTS.md 的错误交叉引用：

```bash
# 在 AGENTS.md 中查找错误引用 CLAUDE.md 的上下文
grep -n "CLAUDE.md" AGENTS.md 2>/dev/null
# 在其他文档中查找混淆引用
grep -rn "AGENTS.md\|CLAUDE.md" docs/ design/ .specify/ --include="*.md" | grep -v "CLAUDE.md\|AGENTS.md" | head -30
```

修正所有文件名引用错误：引用 CLAUDE.md 的地方必须确实在说 Claude Code Agent，引用 AGENTS.md 的地方必须确实在说其他 Agent。

- [ ] **Step 5: 修正 ui-spec 颜色来源引用（DC-005）**

将引用指向 design-system.md。

- [ ] **Step 6: Commit**

```bash
git add .specify/memory/constitution.md AGENTS.md design/ui-spec.md
# 加上 DC-004 Step 4 发现的其他文件
git commit -m "docs: fix constitution version history, changelog, and cross-references (DC-001~005)"
```

---

### Task 12: 修复 FS-001~003 + CF-001 — 功能规格和配置对齐

**依赖:** Task 8, Task 10

**Files:**
- Modify: `CLAUDE.md`, `architecture.md`, `backend/config.yaml`

- [ ] **Step 1: 标注收藏集实现状态（FS-001）**

在 CLAUDE.md 收藏集描述处添加：`> **实现状态**: 见 DEC-010。`

- [ ] **Step 2: 明确 search_suggestions 数据源（FS-002）**

在 architecture.md 补充说明。

- [ ] **Step 3: 对齐众裁判决阈值（FS-003）**

统一文档和 config.yaml 中的阈值。

- [ ] **Step 4: 对齐 features 注册表（CF-001）**

```bash
grep -rn "features\." backend/ --include="*.go" | grep -v "_test.go" | sed 's/.*features\.\([a-z_]*\).*/\1/' | sort -u
```

对比 config.yaml features 段，补全缺失、删除幽灵。

- [ ] **Step 5: Commit**

```bash
git add CLAUDE.md architecture.md backend/config.yaml
git commit -m "fix: align feature specs with implementation and sync features registry (FS-001~003, CF-001)"
```

---

### Task 13: 修复 TT-001 + TT-003 — progress.txt 和 task.json

**依赖:** Task 1-5（CS 修复完成后有 progress 内容）

**Files:**
- Modify: `progress.txt`, `task.json`

- [ ] **Step 1: 补全 Task 151-169 progress.txt 记录（TT-001）**

检查并补充缺失的执行记录。

- [ ] **Step 2: 拆分安全加固合并条目（TT-003）**

将合并条目拆分为独立条目。

- [ ] **Step 3: 在 task.json 添加 passes 语义说明**

在 task.json 顶层添加 `_comment` 字段（标准 JSON 不支持 `//` 注释）：

```json
"_comment": "passes: true 表示该任务曾经验证通过，不代表当前 Beta 回归状态。Beta 任务状态以 docs/superpowers/plans/ 下 checkbox 为准。"
```

> **验证**：添加前确认无工具以编程方式遍历 task.json 的所有顶层键（如 `for key in task` 模式）。当前已知仅 CLAUDE.md 模式 B 手动读取该文件，无自动化脚本消费其键列表。如有疑虑，可替代方案为在 README 或 CLAUDE.md 中声明语义，不修改 task.json。

- [ ] **Step 4: Commit**

```bash
git add progress.txt task.json
git commit -m "docs: complete progress.txt gaps and clarify task.json passes semantics (TT-001, TT-003)"
```

---

### Task 14: 编写首批 ADR 记录

**依赖:** Task 8（adr/ 目录已创建）

**Files:**
- Create: `docs/adr/0001-document-governance.md`

- [ ] **Step 1: 编写 0001-document-governance.md**

```markdown
# ADR-0001: 文档治理体系

**状态**: accepted
**日期**: 2026-06-29

## 背景
项目文档经历 3 轮审查，累计发现 201 项独立问题。根因是多技能独立生成文档、文档与代码脱节、缺乏权威源声明。

## 决策
采用三层文档治理：权威文档登记表（12 条）、architecture.md 自动生成区、doc-validator 工具。

## 后果
- 文档从 ~80 份精简到 12 个权威来源类别（含约 20+ 份实际文件）
- 新文档必须放在 docs/working/，禁止在 docs/ 根目录创建
- 修改源码后需运行 doc-validator --fix
```

- [ ] **Step 2: 更新 adr/README.md 索引**

- [ ] **Step 3: Commit**

```bash
git add docs/adr/
git commit -m "docs: add ADR-0001 for document governance system"
```

---

---

## Phase 3: 自动化工具（2 周内）

### Task 15: 实现 doc-validator 核心工具

**依赖:** Task 6（architecture.md 对齐完成）、Task 9（CLAUDE.md 规则已写入）

**Files:**
- Create: `tools/doc-validator/main.go`
- Create: `tools/doc-validator/rules/config_sync.go`
- Create: `tools/doc-validator/rules/schema_sync.go`
- Create: `tools/doc-validator/rules/route_sync.go`
- Create: `tools/doc-validator/README.md`

- [ ] **Step 1: 初始化 Go module**

```bash
mkdir -p tools/doc-validator/rules
cd tools/doc-validator && go mod init omnicraft/tools/doc-validator
```

> **Go 工作区策略**：`tools/doc-validator` 为独立 Go module，不与 `backend/` 共享 `go.mod`。原因：(1) 工具与主服务解耦；(2) 不需要引用 backend 内部包；(3) 无需在根目录创建 `go.work`。如需引用 backend 的 config 结构体定义，改为在 doc-validator 内定义独立的数据模型或从 config.go 的 mapstructure tag 解析。

- [ ] **Step 2: 实现 main.go**

两个子命令：`--fix`（自动刷新 architecture.md 自动区）和 `--check`（输出问题清单）。支持 `--diff` 参数仅检查 git diff 涉及的文件。

关键逻辑：
- findProjectRoot(): 向上查找 architecture.md + backend/ 定位项目根目录
- runFix(): 依次调用 SyncConfigFields(), SyncSchemaDocs(), SyncRouteList()
- runCheck(): 依次调用所有 Check* 函数，收集 Issue 列表

- [ ] **Step 3: 实现 config_sync.go**

核心函数：
- extractConfigFields(): 用 Go parser 解析 config.go，提取所有 struct 的 mapstructure tag
- CheckConfigSync(): 对比 config.go 提取的字段与 architecture.md §7 描述的字段，返回差异
- SyncConfigFields(): 自动生成/刷新 architecture.md §7 的配置字段表

使用 go/ast + go/parser 解析 config.go，正则提取 mapstructure tag。

- [ ] **Step 4: 实现 schema_sync.go**

核心函数：
- parseMigrations(): 读取 migrations/*.sql，用正则提取 CREATE TABLE 语句
- CheckSchemaSync(): 对比 SQL schema 与 architecture.md §4 描述
- SyncSchemaDocs(): 自动生成 architecture.md §4 的数据库 Schema 表

- [ ] **Step 5: 实现 route_sync.go**

核心函数：
- extractRoutes(): 用 Go AST 解析 routes.go 的 Gin 路由注册调用
- CheckRouteSync(): 对比代码路由与 architecture.md §3.2 路由清单
- SyncRouteList(): 自动生成路由清单表格

- [ ] **Step 6: 为 architecture.md 添加 AUTO-GENERATED 标记**

在三个自动区前后插入 HTML 注释标记，Agent 不得手工编辑标记内的内容。

```markdown
<!-- AUTO-GENERATED: §3.2 API 路由清单 | source: backend/internal/handler/routes.go | DO NOT EDIT MANUALLY -->
...自动生成的表格...
<!-- END AUTO-GENERATED: §3.2 -->
```

- [ ] **Step 7: 运行首次 --fix 并验证**

```bash
cd tools/doc-validator && go run . --fix
git diff architecture.md  # 确认自动区内容正确
```

- [ ] **Step 7b: 更新 CLAUDE.md/AGENTS.md 规则 1 为最终版本**

doc-validator 已就绪，将 Task 9 Step 4 中嵌入的分阶段规则 1 替换为最终版本：

```markdown
### 规则 1：修改代码后运行 doc-validator

修改 config.go / migrations / routes.go 后，提交前运行：
`cd tools/doc-validator && go run . --fix`
```

即在 CLAUDE.md 和 AGENTS.md 中将规则 1 的 Phase 3 前/后两段替换为上述单段。

- [ ] **Step 8: 编写 README.md**

- [ ] **Step 9: Commit**

```bash
git add tools/doc-validator/ architecture.md CLAUDE.md AGENTS.md
git commit -m "feat: add doc-validator tool with config/schema/route auto-generation (P3-1, P3-2)"
```

---

### Task 16: 实现 token 引用和交叉引用校验

**依赖:** Task 15（doc-validator 核心已实现）

**Files:**
- Create: `tools/doc-validator/rules/token_refs.go`
- Create: `tools/doc-validator/rules/cross_refs.go`

- [ ] **Step 1: 实现 token_refs.go**

CheckTokenRefs(): 扫描 ui-spec.md 中的 CSS 变量引用（var(--xxx)），检查 design-system.md 中是否有对应定义。

- [ ] **Step 2: 实现 cross_refs.go**

CheckCrossRefs(): 扫描所有 .md 文件中的文件链接（[text](path) 和 file:///），检查目标文件是否存在。
CheckExpiredDocs(): 扫描 docs/working/ 下文件头部的预计失效日期，标记已过期文档。

- [ ] **Step 3: 运行全量 --check 并修复问题**

```bash
cd tools/doc-validator && go run . --check
```

直到输出 `All checks passed`。

- [ ] **Step 4: Commit**

```bash
git add tools/doc-validator/rules/token_refs.go tools/doc-validator/rules/cross_refs.go
git commit -m "feat: add token reference and cross-reference validation to doc-validator (P3-3)"
```

---

### Task 17: 最终集成验证

**依赖:** Task 1-16 全部完成

**Files:**
- 无新增/修改（仅验证）

- [ ] **Step 1: 运行全量 doc-validator --check**

```bash
cd tools/doc-validator && go run . --check
```

Expected: `All checks passed`（6 项检查全部通过）

- [ ] **Step 2: 逐项确认验收标准总览表**

按验收标准总览表（本文档末尾）逐项验证 12 条标准，记录每项结果。

- [ ] **Step 3: 确认 docs/ 根目录干净**

```bash
ls docs/*.md
```

Expected: 仅 `docs/GLOSSARY.md`

- [ ] **Step 4: 确认术语统一**

```bash
grep -rn "审核员\|评审员\|二次创作\|衍生作品\|信用分\|仪表盘" docs/ design/ CLAUDE.md AGENTS.md architecture.md --include="*.md" 2>/dev/null
```

Expected: 无结果（GLOSSARY.md 自身的禁止混用表除外）

- [ ] **Step 5: 运行全量编译 + 测试**

```bash
cd backend && go build ./... && go vet ./... && go test ./...
cd frontend && npm run build 2>/dev/null || echo "frontend build skipped (no changes expected)"
```

Expected: all PASS, no errors

- [ ] **Step 6: 记录验证结果**

在 `docs/working/2026-06-29-governance-verification-log.md` 中记录全部 12 项验收标准的验证结果。

- [ ] **Step 7: Commit**

```bash
git add docs/working/2026-06-29-governance-verification-log.md
git commit -m "docs: final integration verification for document governance implementation"
```

---

## Phase ↔ 设计规格任务编号映射

实施计划使用 `Task 1-17` 编号，设计规格（`docs/superpowers/specs/2026-06-29-omnicraft-documentation-governance-design.md` §9）使用 `P#-#` 编号。以下为对应关系：

| 实施 Task | 设计规格编号 | 说明 |
|-----------|------------|------|
| Task 1-5 | P1-1 | CS-001~007 代码安全修复 |
| Task 6 | P1-2 | AD-001~007 架构文档对齐 |
| Task 7 | P1-3 | GLOSSARY.md 创建 |
| Task 8 | P1-4 | 目录结构重组 |
| Task 9 | P1-5 | CLAUDE.md / AGENTS.md 更新 |
| Task 10 | P2-1 | DS-001~005 设计系统统一 |
| Task 11 | P2-2 | DC-001~005 宪法一致性 |
| Task 12 | P2-3 + P2-4 | FS/CF 功能规格和配置对齐 |
| Task 13 | P2-5 | TT-001/003 progress.txt/task.json |
| Task 14 | P2-6 | 首批 ADR 记录 |
| Task 15 | P3-1 + P3-2 | doc-validator 核心 + 自动区标记 |
| Task 16 | P3-3 | token/交叉引用校验 |
| Task 17 | — | 最终集成验证（仅实施计划有） |

---

## 依赖关系图

```
Phase 1 (高度可并行)
├── Task 1 (CS-001) ──────────────────────┐
├── Task 2 (CS-002) ──────────────────────┤
├── Task 3 (CS-003) ──────────────────────┤
├── Task 4 (CS-004) ──────────────────────┤ 可完全并行
├── Task 5 (CS-005+007) ──────────────────┤
├── Task 6 (AD-001~007) ──────────────────┤
├── Task 7 (GLOSSARY) ────────────────────┤
├── Task 8 (目录重组) ─────────────────────┘
│
Phase 2 (部分依赖 Phase 1)
├── Task 9 (CLAUDE/AGENTS) ← 依赖 Task 7, Task 8
├── Task 10 (DS-001~005) ← 依赖 Task 7
├── Task 11 (DC-001~005) ← 依赖 Task 9
├── Task 12 (FS/CF) ← 依赖 Task 8, Task 10
├── Task 13 (TT) ← 依赖 Task 1-5
├── Task 14 (ADR) ← 依赖 Task 8
│
Phase 3 (依赖 Phase 1+2 完成)
├── Task 15 (doc-validator) ← 依赖 Task 6, Task 9
└── Task 16 (token/cross-ref) ← 依赖 Task 15

集成验证 (依赖全部完成)
└── Task 17 (最终验证) ← 依赖 Task 1-16
```

---

## 验收标准总览

> **说明**：标记 `[P3]` 的标准依赖 Phase 3 实现的 doc-validator 工具。Phase 1-2 期间改为手工验证，Phase 3 后切换为自动验证。

| # | 验收标准 | 验证方式 | 阶段 |
|---|---------|---------|------|
| 1 | AuthRequired Redis+DB 不可达时返回 503 | go test ./internal/middleware/ -v | P1 |
| 2 | search_repo.go 无字符串拼接 SQL | grep "fmt.Sprintf.*SELECT" ... 无结果 | P1 |
| 3 | handler 中无 err.Error() 暴露 | grep -r "err.Error()" handler/ 无结果 | P1 |
| 4 | reputation_service 无硬编码数字 | 人工核对 | P1 |
| 5 | nginx conf 含 6+ 安全头 | grep -c "add_header" ... >= 6 | P1 |
| 6 | architecture.md 自动区与代码一致 [P3] | cd tools/doc-validator && go run . --check | P3 |
| 7 | docs/ 根目录无 .md（除 GLOSSARY.md） | ls docs/*.md | P1 |
| 8 | 术语统一 | grep "审核员\|评审员" 无结果 | P2 |
| 9 | 设计 token 全有定义 [P3] | doc-validator --check 无 token 引用错误 | P3 |
| 10 | 所有交叉引用有效 [P3] | doc-validator --check 无交叉引用错误 | P3 |
| 11 | Constitution Changelog 完整 | 目视检查 | P2 |
| 12 | go build + vet + test 全通过 | cd backend && go build ./... && go vet ./... && go test ./... | P1 |

---

*计划文档结束*
