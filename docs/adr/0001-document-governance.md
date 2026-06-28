# ADR-0001: 文档治理体系

**状态**: accepted
**日期**: 2026-06-29
**决策者**: Overnight Agent

## 背景

OmniCraft 项目文档经历 3 轮独立审查：
- 2026-06-25 健康检查：168 项问题
- 2026-06-28 两轮独立审查：187 + 74 项（合并去重后 201 项）

根因分析确认五个系统性问题：
1. 多技能独立生成文档 → 多份设计稿并存 → Agent 读到矛盾信息自行发挥
2. 文档与代码脱节（architecture.md vs config.yaml/migrations/routes.go）
3. 缺乏权威源声明（Agent 不知道以哪份文档为准）
4. 缺乏文件组织规范（docs/ 根目录堆积 80+ 份文档）
5. 修复是点状的（无系统性治理）

## 决策

采用三层文档治理体系：

### 1. 文档权威登记表
12 条权威条目，每领域唯一真相源。冲突时按优先级：宪法 > 代码 > architecture.md > design/ > specs/ > plans/。

### 2. architecture.md 自动生成区
三个区段（§3.2 API 路由、§4 数据库 Schema、§7 配置字段）从源码自动提取，以 HTML 注释标记边界，Agent 不得手工编辑。

### 3. doc-validator 工具
Go CLI 工具，两个命令：
- `--fix`: 自动刷新 architecture.md 自动区
- `--check`: 6 项一致性校验（config/schema/路由/token/交叉引用/过期文档）

## 后果

### 正面
- 文档从 ~80 份精简到 12 份权威条目（约 20+ 个实际文件）
- architecture.md 与代码的脱节由工具自动检测
- 新 Agent 有明确的权威源参考表
- 术语统一在 GLOSSARY.md 中
- 文件名冲突和存放混乱问题得到规范

### 风险
- doc-validator 工具本身需要维护（Phase 3 实现）
- 治理规范需要 Agent 自觉遵守（无 pre-commit 强制）
- 治理文档自身也需要生命周期管理（见设计规格 §11）

### 迁移代价
- 一次性归档 ~50 份旧文档到 docs/archive/
- CLAUDE.md 和 AGENTS.md 需要同步更新
- architecture.md 首次自动生成需要人工审核

## 参考

- 设计规格：`docs/superpowers/specs/2026-06-29-omnicraft-documentation-governance-design.md`
- 实施计划：`docs/superpowers/plans/2026-06-29-omnicraft-documentation-governance-implementation.md`
- 问题清单：`docs/archive/2026-06-29-consolidated-fix-collection.md`
