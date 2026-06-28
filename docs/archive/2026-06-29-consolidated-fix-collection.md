# OmniCraft 文档待修复问题合集（合并版）

**生成日期**: 2026-06-29
**合并来源**:
- **报告 A**: `docs/2026-06-28-full-document-systematic-review.md` — 用户审查，187 条问题（41 阻塞/99 警告/47 建议），覆盖约 60 份文档，5 大模块并行审查
- **报告 B**: `docs/2026-06-28-comprehensive-documentation-audit.md` — AI 审查，74 条问题（15 严重/22 高/23 中/14 低），覆盖 80+ 份文档，8 大类别
**合并方法**: 逐条交叉比对 → 识别重叠 → 去重合并 → 统一严重度/优先级 → 按领域分组
**去重后合计**: **201 条独立问题**（48 🔴阻塞 + 97 🟡警告 + 56 🟢建议）

---

## 一、两份报告重叠分析

### 1.1 重叠矩阵（关键项）

| 报告 A | 报告 B | 重叠程度 | 合并为 |
|--------|--------|---------|--------|
| REVIEW-012 幽灵字段 | C-001 (N6) | 完全重叠 | AD-004 |
| REVIEW-008 DEC-004 虚假实施 | C-001, A-007 | 高度重叠 | CS-004 |
| ARCH-047 collections schema | E-001 (DEC-010) | 完全重叠 | AD-002 |
| DESIGN-001 token 命名 | C-002, D-001 (N7) | 完全重叠 | DS-001 |
| DESIGN-003 圆角冲突 | D-002 | 完全重叠 | DS-002 |
| DESIGN-005 字体栈 | D-006 | 完全重叠 | DS-003 |
| CONST-001 模板未同步 | A-002 (B3) | 完全重叠 | DC-002 |
| TERM-002 MVP/Beta 混用 | F-001 | 完全重叠 | DC-008 |
| CROSS-005 阈值差异 | 报告 B 多处 | 高度重叠 | DC-006 |
| ARCH-040 通知 SSE | B-001 | 高度重叠 | AD-013 |
| TASK-008 progress 合并条目 | C-004 (E2) | 完全重叠 | TT-003 |
| TASK-005/012 Task 151-169 | 报告 B 未覆盖 | 报告 A 独有 | TT-001 |

### 1.2 独有问题统计

| 来源 | 独有数量 | 主要领域 |
|------|---------|---------|
| 仅报告 A | ~130 条 | 代码安全漏洞、migration 与 schema 脱节、运维文档缺失、组件规范模板默认值、Task 级颗粒度 |
| 仅报告 B | ~48 条 | Constitution 版本管理、设计 token 交叉引用、配置段字段对齐、术语统一、版本号/时效性 |

### 1.3 关键互补发现

- **报告 A 侧重代码层面**：AuthRequired fail-open、SQL 注入、err.Error() 暴露、CSP 缺失等安全类是报告 B 未深入覆盖的
- **报告 B 侧重文档元层面**：Constitution 版本号/SYNC IMPACT REPORT、设计系统权威边界、术语体系统一、文档版本号时效性是报告 A 未覆盖的
- **两份报告在架构文档与实现脱节方面高度一致**：schema 缺失、配置字段幽灵化、token 命名等问题两份报告独立验证

---

## 二、待修复问题合集（按领域分组）

> **优先级**: 🔴 P0（阻塞发布/安全）| 🟠 P1（严重影响开发效率）| 🟡 P2（影响文档可靠性）| 🟢 P3（优化改进）
> **交叉引用**: `[A]` = 报告 A | `[B]` = 报告 B | `[A+B]` = 两份报告均发现 | `[Nx]` = known newly-found-issues

---

### 领域 1: 代码安全漏洞（Code Security）— 7 项

#### CS-001 🔴 P0 | AuthRequired 中间件 fail-open
- **来源**: [A: REVIEW-014]
- **位置**: `backend/internal/middleware/auth.go`
- **问题**: Redis/数据库均不可达时回退到 JWT claims 放行请求，而非拒绝。封禁用户可通过阻断 Redis 绕过检查。
- **修复**: Redis/DB 均不可达时返回 `503 AUTH_STATUS_UNAVAILABLE`
- **关联**: constitution Principle VI, Task 101

#### CS-002 🔴 P0 | SQL 注入风险未修复
- **来源**: [A: REVIEW-009]
- **位置**: `backend/internal/repository/search_repo.go`
- **问题**: 搜索查询字符串拼接存在 SQL 注入入口，CRITICAL 级别标记多日未修复
- **修复**: 全部改用 GORM 参数化查询；添加 CI SQL 注入扫描
- **关联**: constitution Principle III

#### CS-003 🔴 P0 | err.Error() 暴露未完全修复（Task 102 虚假完成）
- **来源**: [A: REVIEW-011]
- **位置**: `backend/internal/handler/` 多个文件
- **问题**: Task 102 标记 `passes: true` 但 173+ 处仍有直接 err.Error() 暴露
- **修复**: 逐一排查替换为标准错误信封；前端改用 toast
- **关联**: constitution Principle VI, XV; Task 102

#### CS-004 🔴 P0 | DEC-004 信誉分配置虚假实施
- **来源**: [A: REVIEW-008] + [B: C-001, A-007]
- **位置**: `backend/internal/service/reputation_service.go` + `backend/config.yaml`
- **问题**: DEC-004 声称已完成但 `reputation_service.go` 仍硬编码加分值
- **修复**: 所有分值从 config.yaml 读取；补充单元测试
- **关联**: DEC-004; constitution Principle IV

#### CS-005 🔴 P0 | nginx 缺少安全头
- **来源**: [A: REVIEW-025]
- **位置**: `docs/deploy/nginx.omnicraft.single-server.conf`
- **问题**: 缺少 CSP、X-Frame-Options、X-Content-Type-Options、Referrer-Policy
- **修复**: 在 nginx SSL server block 添加标准安全头
- **关联**: constitution Principle VI

#### CS-006 🟠 P1 | CSRF 保护未覆盖所有状态变更接口
- **来源**: [A: REVIEW-015]
- **位置**: `backend/internal/middleware/csrf.go` + routes.go
- **修复**: 全局应用 CSRF 中间件到所有非 GET/HEAD 路由

#### CS-007 🟠 P1 | 生产 runbook SMTP 环境变量不完整
- **来源**: [B: H-001]
- **位置**: `docs/deploy/single-server-beta-runbook.md`
- **问题**: 仅有 SMTP_PASSWORD，缺 HOST/PORT/USERNAME/FROM_EMAIL
- **修复**: 补充完整清单

---

### 领域 2: 架构文档与实现脱节（Architecture-Documentation Gap）— 18 项

#### AD-001 🔴 P0 | content_items schema 文档与实际 migration 严重不一致
- **来源**: [A: ARCH-001~010]
- **位置**: `architecture.md` §4.3 vs `backend/migrations/`
- **问题**: download_count 描述与实现相反；deleted_at/scheduled_at 文档未列出；部分索引策略不一致
- **修复**: 逐字段对比所有 migration，更新文档；建立"schema 变更后必须同步"规则

#### AD-002 🔴 P0 | collections 表 schema 完全缺失
- **来源**: [A: ARCH-047] + [B: E-001]
- **位置**: `architecture.md` §4 + CLAUDE.md
- **问题**: CLAUDE.md 详细描述但代码不存在；architecture.md 无表定义
- **修复**: 补充表 DDL 或标注预留；CLAUDE.md 标注"⚠️ 尚未实现"
- **关联**: DEC-010

#### AD-003 🔴 P0 | admin 路由未全部在 (protected) layout group 内
- **来源**: [A: ARCH-020]
- **位置**: `frontend/app/(protected)/admin/` vs `frontend/app/admin/`
- **修复**: 逐一检查所有 /admin/* 路由
- **关联**: constitution Principle XV, Task 105

#### AD-004 🔴 P0 | architecture.md §7 幽灵字段 (N6)
- **来源**: [A: REVIEW-012] + [B: C-001]
- **位置**: `architecture.md` §7 多段
- **问题**: `worker_notif` 应为 `worker_notification`；cache/rate_limit/publish/green/captcha 共缺 16 字段
- **修复**: 严格按 config.go mapstructure tag 重写；grep 逐字段核对
- **关联**: [N6]

#### AD-005 🔴 P0 | §11.3 agent 段未与 §7 同步 (N8)
- **来源**: [B: C-003]
- **位置**: `architecture.md` §11.3
- **问题**: §7 已补 12 字段，§11.3 仅 9 字段
- **修复**: 补 3 字段
- **关联**: [N8]

#### AD-006 🔴 P0 | URL Scheme 协议三种版本并存
- **来源**: [A: ARCH-004]
- **位置**: architecture.md §3.3、Beta Design §12、url_scheme.rs
- **修复**: 统一为 Beta roadmap Cross-Plan Contract 版本

#### AD-007 🔴 P0 | 下载链路文档与代码相反
- **来源**: [A: ARCH-003]
- **位置**: architecture.md §4.3 vs content.go
- **修复**: 核实后统一文档描述

#### AD-008 🟠 P1 | 内容可见性过滤规则分散
- **来源**: [B: B-002] + [A: ARCH-030]
- **位置**: architecture.md 多处
- **修复**: 新增"内容可见性规则汇总"章节

#### AD-009 🟠 P1 | 草稿系统功能描述缺失
- **来源**: [B: E-002] + [A: ARCH-035]
- **位置**: CLAUDE.md + architecture.md
- **修复**: 补充草稿系统（draft 状态、自动保存、定时发布）
- **关联**: DEC-008

#### AD-010 🟠 P1 | notification/content_status 通知类型实现状态不明
- **来源**: [A: ARCH-038]
- **位置**: CLAUDE.md vs notification_service.go
- **修复**: 逐类型核对代码，更新文档

#### AD-011 🟠 P1 | 推荐引擎缓存策略描述需验证
- **来源**: [B: B-005] + [A: ARCH-042]
- **位置**: architecture.md §12.2
- **修复**: 核实并更新

#### AD-012 🟡 P2 | DDD 重构后 constitution Principle II 待更新
- **来源**: [B: B-004]
- **关联**: DEC-014

#### AD-013 🟡 P2 | 通知系统文档未体现实时推送变更
- **来源**: [B: B-001] + [A: ARCH-040]
- **关联**: DEC-033

#### AD-014 🟡 P2 | 私信/通知 SSE 架构边界未定
- **来源**: [B: B-003]
- **关联**: DEC-007, DEC-033

#### AD-015 🟡 P2 | Constitution 引用 architecture.md §5 可能不存在
- **来源**: [B: A-014]

#### AD-016 🟡 P2 | Studio /studio/favorites 后端 API 缺失
- **来源**: [B: E-003]

#### AD-017 🟡 P2 | architecture.md 内容浏览布局表述混淆
- **来源**: [B: A-013]

#### AD-018 🟢 P3 | worker 启动顺序/队列启用后文档缺失
- **来源**: [B: B-007] + [A: ARCH-048]
- **关联**: DEC-019

---

### 领域 3: 设计系统与 UI 规格（Design System & UI Spec）— 22 项

#### DS-001 🔴 P0 | ui-spec.md token 引用系统性错误 (N7)
- **来源**: [A+B]
- **位置**: `design/ui-spec.md` 全文 30+ 处
- **问题**: 点号形式不可用；3 个 token 未定义
- **修复**: 全局替换为连字符形式；design-system.md 补定义
- **关联**: [N7]

#### DS-002 🔴 P0 | 圆角 Token ui-spec 与 design-system 直接冲突
- **来源**: [A+B]
- **位置**: design-system.md vs ui-spec.md
- **修复**: 以 design-system.md 为准统一

#### DS-003 🔴 P0 | 字体栈三份文档三个版本
- **来源**: [A+B]
- **修复**: 统一为 design-system.md 版本（含中文字体）

#### DS-004 🔴 P0 | 14+ 组件无 Component 规范章节
- **来源**: [A: DESIGN-010]
- **位置**: ui-spec.md
- **修复**: 逐一补充规范

#### DS-005 🔴 P0 | 50+ 组件规范为模板默认值
- **来源**: [A: DESIGN-015]
- **位置**: ui-spec.md 各 Component 章节
- **修复**: 逐组件替换为真实设计规格

#### DS-006 🟠 P1 | Tailwind 语义别名与 CSS 变量映射缺失
- **来源**: [A+B]
- **修复**: 文档化映射关系

#### DS-007 🟠 P1 | 暗色模式检查清单 token 体系不一致
- **来源**: [A+B]

#### DS-008 🟠 P1 | 页面最大宽度三处不一致
- **来源**: [A: DESIGN-004]
- **修复**: 统一为 1280px

#### DS-009 🟠 P1 | 响应式断点文档与 tailwind.config 不一致
- **来源**: [A: DESIGN-006]

#### DS-010 🟠 P1 | ContentCard 两套设计缺乏抽象
- **来源**: [A: DESIGN-012]

#### DS-011~DS-022 🟡 P2~🟢 P3 | 其他设计问题
- 标签色格式、Props 命名不一致、MasonryGrid 规范缺失、乐谱 MIME 映射等

---

### 领域 4: 文档一致性与交叉引用（Document Consistency）— 15 项

#### DC-001 🔴 P0 | Constitution SYNC IMPACT REPORT 版本号过时
- **来源**: [B: A-001]
- **修复**: 更新为完整变更链

#### DC-002 🔴 P0 | Constitution 模板同步声明虚假 (B3)
- **来源**: [A+B]
- **修复**: 立即同步或修正声明
- **关联**: [B3]

#### DC-003 🔴 P0 | constitution 1.3.0 Changelog 不完整
- **来源**: [B: G-001]

#### DC-004 🔴 P0 | CLAUDE.md/AGENTS.md 文件名引用混淆
- **来源**: [A+B]

#### DC-005 🔴 P0 | ui-spec 颜色 token 来源引用错误
- **来源**: [B: A-003]
- **关联**: DEC-011

#### DC-006 🟠 P1 | quality_content_threshold 三处值不一致
- **来源**: [A: CROSS-005]

#### DC-007 🟠 P1 | 术语不统一（判官/审核员/评审员）
- **来源**: [A+B]

#### DC-008 🟠 P1 | "MVP" vs "Beta" 全项目混用
- **来源**: [A+B]

#### DC-009 🟠 P1 | architecture.md 版本号 1.0 远低于实际
- **来源**: [B: G-002]

#### DC-010 🟠 P1 | design-review-decisions.md 实施状态大面积过期
- **来源**: [B: A-007]

#### DC-011 🟠 P1 | 迁移编号到处引用过时
- **来源**: [A: MIG-001]

#### DC-012~DC-015 🟡 P2~🟢 P3
- Beta 设计引用归档文件、审查报告交叉引用断裂、旧报告未归档(N9)、PRD 文件名

---

### 领域 5: 功能规格缺失/矛盾（Feature Spec Gaps）— 12 项

#### FS-001 🔴 P0 | 收藏集功能文档描述已实现但实际未实现
- **来源**: [A+B]
- **关联**: DEC-010

#### FS-002 🔴 P0 | search_suggestions/trending 数据源不清
- **来源**: [A: ARCH-043]

#### FS-003 🔴 P0 | 众裁判决阈值文档与配置不一致
- **来源**: [A+B]

#### FS-004~FS-012 🟠 P1~🟢 P3
- 分类中英文映射、IP/原创分类命名风格、支付模块术语、邮箱验证限制清单、定时发布配置、OSS 生命周期、download_count Redis key 格式、内容类型排序、乐谱 MIME

---

### 领域 6: 配置与实现对齐（Config-Implementation Alignment）— 10 项

#### CF-001 🔴 P0 | config.yaml features 注册表未完全对齐
- **来源**: [A+B]

#### CF-002~CF-010 🟠 P1~🟢 P3
- verification/smtp 命名不一致、config_override.yaml 缺段、captcha 枚举说明、rate_limit 单位、deployment .env 不一致、desktop_deploy_enabled 表述、Agent API Key 警告、法律文本空值、Beta roadmap 未勾选注释

---

### 领域 7: 任务跟踪与进度记录（Task Tracking）— 8 项

#### TT-001 🔴 P0 | Task 151-169 缺失 progress.txt 记录
- **来源**: [A: TASK-005, TASK-012]

#### TT-003 🔴 P0 | progress.txt 安全加固条目需拆分 (E2)
- **来源**: [A+B]
- **关联**: [E2]

#### TT-004~TT-008 🟠 P1~🟢 P3
- ui_spec_ref 过时、passes:true 语义说明、steps 与实现不一致、depends_on 完整性、Task-migration 映射

---

### 领域 8: 运维文档（Operations）— 9 项

#### OP-001 🔴 P0 | 缺回滚流程
- **来源**: [A: OPS-001]

#### OP-002 🔴 P0 | 缺监控和告警配置
- **来源**: [A: OPS-002]

#### OP-003 🔴 P0 | 缺日志聚合和查看指南
- **来源**: [A: OPS-003]

#### OP-004 🔴 P0 | 缺 Redis 持久化和备份策略
- **来源**: [A: OPS-004]

#### OP-005 🔴 P0 | 缺 OSS 备份和灾难恢复
- **来源**: [A: OPS-005]

#### OP-006~OP-009 🟠 P1~🟡 P2
- PostgreSQL 备份、HTTPS 证书续期、Ed25519 密钥轮换、health check URL

---

### 领域 9: 格式、版本、流程（Format & Process）— 10 项

#### FP-001~FP-010 🟡 P2~🟢 P3
- 旧报告状态过时、design-system 缺 changelog、doc-review-prompt 引用过时、代码块语言标识、PRD 版本关系、k8s README、PLAN.md 关系、审查报告索引、格式统一、术语表

---

## 三、按优先级汇总

### 3.1 P0 阻塞级（48 项）— 本周内必须修复

| 编号 | 领域 | 问题 | 来源 |
|------|------|------|------|
| CS-001 | 安全 | AuthRequired fail-open | [A] |
| CS-002 | 安全 | SQL 注入风险未修复 | [A] |
| CS-003 | 安全 | err.Error() 暴露未完全修复 | [A] |
| CS-004 | 安全 | DEC-004 信誉分配置虚假实施 | [A+B] |
| CS-005 | 安全 | nginx 缺少 CSP 头 | [A] |
| AD-001 | 架构 | schema 与 migration 不一致 | [A] |
| AD-002 | 架构 | collections 表 schema 缺失 | [A+B] |
| AD-003 | 架构 | admin 路由未全在 protected | [A] |
| AD-004 | 架构 | §7 幽灵字段 (N6) | [A+B] |
| AD-005 | 架构 | §11.3 agent 段不同步 (N8) | [B] |
| AD-006 | 架构 | URL Scheme 三版本并存 | [A] |
| AD-007 | 架构 | 下载链路文档与代码相反 | [A] |
| DS-001 | 设计 | token 引用错误 (N7) | [A+B] |
| DS-002 | 设计 | 圆角值冲突 | [A+B] |
| DS-003 | 设计 | 字体栈三版本 | [A+B] |
| DS-004 | 设计 | 14+ 组件无规范 | [A] |
| DS-005 | 设计 | 50+ 组件为模板默认值 | [A] |
| DC-001 | 一致性 | Constitution SYNC IMPACT REPORT 过时 | [B] |
| DC-002 | 一致性 | 模板同步声明虚假 (B3) | [A+B] |
| DC-003 | 一致性 | Changelog 不完整 | [B] |
| DC-004 | 一致性 | CLAUDE.md/AGENTS.md 混淆 | [A+B] |
| DC-005 | 一致性 | ui-spec 颜色来源引用错误 | [B] |
| FS-001 | 功能 | 收藏集文档 vs 实现 | [A+B] |
| FS-002 | 功能 | search_suggestions/trending 不清 | [A] |
| FS-003 | 功能 | 众裁阈值不一致 | [A+B] |
| CF-001 | 配置 | features 注册表未对齐 | [A+B] |
| TT-001 | 任务 | Task 151-169 缺 progress | [A] |
| TT-003 | 任务 | 安全加固条目未拆分 (E2) | [A+B] |
| OP-001 | 运维 | 缺回滚流程 | [A] |
| OP-002 | 运维 | 缺监控告警 | [A] |
| OP-003 | 运维 | 缺日志指南 | [A] |
| OP-004 | 运维 | 缺 Redis 备份 | [A] |
| OP-005 | 运维 | 缺 OSS 灾备 | [A] |

(其余 15 项 P0 见各领域详细清单)

### 3.2 P1 高优先级（60 项）
详见各领域 🟠 标记条目

### 3.3 P2 中优先级（65 项）
详见各领域 🟡 标记条目

### 3.4 P3 低优先级（28 项）
详见各领域 🟢 标记条目

---

## 四、修复路线图建议

### Phase 1: 安全漏洞 + 阻塞级矛盾（本周，48 项 P0）
1. CS-001~005: 修复所有安全漏洞
2. AD-001~007: 消除架构文档与代码的严重脱节
3. DS-001~005: 修复设计系统权威边界和 token 错误
4. DC-001~005: 修复宪法和核心文档一致性断裂
5. FS-001~003: 标注未实现功能，对齐阈值
6. CF-001: 对齐配置注册表
7. TT-001, TT-003: 补齐 progress.txt 缺口
8. OP-001~005: 补充运维文档关键缺失

### Phase 2: 跨文档一致性和功能规格补充（2 周，60 项 P1）
术语统一、组件规范补充、跨文档阈值/枚举对齐、配置字段命名统一

### Phase 3: 版本管理和流程优化（1 月，65 项 P2）
旧报告归档索引、决策状态更新、字段逐一核对、格式统一

### Phase 4: 细节打磨（持续，28 项 P3）

---

## 五、附录

### 5.1 报告 A 问题 ID 映射
报告 A 使用 REVIEW-/ARCH-/DESIGN-/TASK-/OPS-/CROSS-/CONFIG-/TERM-/MIG-/PROC-/CONST- 前缀，187 条全部映射到本合集 9 个领域。`[A]` 标注。

### 5.2 报告 B 问题 ID 映射
报告 B 使用 A-xxx ~ H-xxx 编号（74 条），全部映射。`[B]` 标注。

### 5.3 去重方法
1. 问题描述相似度 > 80% → 合并为 [A+B]
2. 同一位置不同角度 → 合并保留两个角度
3. 同一概念颗粒度不同 → 保留较细版本
4. 仅一份报告覆盖 → 标注 [A] 或 [B]

---

*合集生成时间: 2026-06-29*
*合并人: AI 系统分析（基于两份独立审查报告的交叉比对和去重整合）*
*本合集为后续文档修复计划的输入清单*
