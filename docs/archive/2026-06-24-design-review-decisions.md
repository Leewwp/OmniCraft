# OmniCraft 设计审查 — 讨论结论记录

> **文件用途**:记录设计审查过程中逐个问题讨论后得出的**最终决策与实施策略**。
> **维护规则**:每得到一个结论即追加到本文件,不修改历史结论(如需变更,新增"修订记录")。
> **关联文件**:
> - 问题清单(发现层):[`docs/2026-06-24-omnicraft-design-review-merged.md`](./2026-06-24-omnicraft-design-review-merged.md)
> - 修复计划(执行层):待撰写,将对照本文件与问题清单
>
> **使用方式**:撰写修复计划时,同时对照本文件(决策)与问题清单(问题全貌),确保决策不脱离原始问题语境。

---

## 决策索引

| 编号 | 关联问题 | 标题 | 确认状态 | 实施状态 | 讨论日期 |
|------|---------|------|---------|---------|---------|
| DEC-001 | B3 | 中文全文搜索查询侧配置对齐 | ✅ 已确认 | 🚧 实施中 | 2026-06-24 |
| DEC-002 | H1 | Token 存储方案降级为 P3 长期优化 | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-003 | H3 | 法律文本暂时搁置,用户自行处理 | ⏸️ 搁置 | — | 2026-06-24 |
| DEC-004 | C1 | 信誉分配置补全,阈值统一为 10 | ✅ 已确认 | ✅ 已实施 | 2026-06-24 |
| DEC-005 | D1 | Studio 存根页面完整迁移(方案 B) | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-006 | F1 | 瀑布流完整分页体验(方案 B) | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-007 | G6 | 私信 SSE 实时推送(方案 A,Beta 前实现) | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-008 | E1 | 草稿系统完整实现(方案 A,Beta 前实现) | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-009 | E3 | 用户数据导出完整实现(方案 A,Beta 前实现) | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-010 | E5 | 实现收藏集页面(方案 B) | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-011 | A1 | 设计系统统一,homepage-v0.html 归档 | ✅ 已确认 | ✅ 已实施 | 2026-06-24 |
| DEC-012 | A3 | 归档前标注 UI Design.md 过时内容 | ✅ 已确认 | ✅ 已实施 | 2026-06-24 |
| DEC-013 | A4 | 归档 UI Design.md,同步相关文档 | ✅ 已确认 | ✅ 已实施 | 2026-06-24 |
| DEC-014 | B2 | 后端 DDD 完整重构 | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-015 | E4 | 数据分析基础增强 | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-016 | G1 | Tauri 离线功能暂缓(降级 P3) | ⏸️ 暂缓 | — | 2026-06-24 |
| DEC-017 | G2 | PWA 支持暂缓(降级 P3) | ⏸️ 暂缓 | — | 2026-06-24 |
| DEC-018 | H4 | 验证码和邮件配置切换为生产模式 | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-019 | I4 | 启用队列系统 | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-020 | C5 | Agent LLM 配置修复 | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| — | B1/F2/F3/F4/G3/H2 | 已修复关闭(核查确认) | ✅ 已关闭 | ✅ 已实施 | 2026-06-24 |
| DEC-021 | A5 | 审查报告归档(先确认问题已解决) | ✅ 已确认 | ✅ 已实施 | 2026-06-24 |
| DEC-022 | A6 | task.json 保留作为历史记录 | ✅ 已确认 | ✅ 已实施 | 2026-06-24 |
| DEC-023 | D4 | 首页路由合并 | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-024 | C6 | 下载权限复用互动阈值 | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-025 | F5 | 移动端适配参考小红书 APP | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-026 | G4 | Tauri 账号同步暂缓 | ⏸️ 暂缓 | — | 2026-06-24 |
| DEC-027 | I1 | 推荐冷启动增加兴趣选择(可跳过) | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-028 | I2 | 向量索引升级为 HNSW | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-029 | I3 | 热门排行配置项暴露 | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-030 | J1 | 迁移验证范围更新 | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-031 | J2 | 软删除策略部分统一 | ✅ 已确认 | ✅ 已实施 | 2026-06-24 |
| DEC-032 | E6/E7 | 桌面端部署和客户端下载现在实施 | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |
| DEC-033 | G5 | 通知系统 SSE 实时推送 | ✅ 已确认 | ⏳ 待实施 | 2026-06-24 |

> **用户导向(2026-06-24)**:用户明确"尽量让功能全面,而不只是一个 beta 版本"。后续未讨论问题的默认倾向应为"完整实现"而非"最小改动",除非实现成本过高或有明确的技术阻塞。

---

## DEC-001:中文全文搜索查询侧配置对齐

**关联问题**:B3(中文全文搜索对中文的处理方案不够可靠)— 见问题清单"类别 B:架构设计"

**讨论日期**:2026-06-24

### 问题核实结论

经代码核查,问题清单中 B3 的原始描述需要修正。实际情况比"完全未分词"更细致:

- **索引侧已正确实现**:`041_content_search_vector.sql`、`042_ips_search_vector.sql`、`045_search_tag_trigger.sql` 已采用 `jiebacfg` 分词(含 simple fallback),并配合 `pg_trgm` 索引(`047`、`049`)作为补充
- **查询侧配置不匹配**:`search_repo.go` 中查询使用 `to_tsquery('simple', ?)`,与索引的 `jiebacfg` 不匹配,导致 tsquery 匹配不到 jieba 分词后的索引
- **查询分词函数不支持中文**:`splitAndNormalize` 仅按空格和 `&|!()` 分词,中文连续文本被当作单个 token,生成 `美食推荐:*` 前缀,无法匹配索引中的 "美食"+"推荐"
- **ILIKE fallback 掩盖了问题**:`title ILIKE` 和 `content_tags.tag ILIKE` 使搜索"勉强可用",但 ts_rank_cd 排序和 ts_headline 高亮失效
- **遗留未升级点**:`discussion_repo.go:77` 仍用 `to_tsvector('simple', ...)` 实时计算(无 search_vector 列);`ip_repo.go:80` 未使用 ips 表已有的 search_vector 列

### 决策

1. **pg_jieba 可用性前提已确认**:生产 Docker 中 pg_jieba 可用(待用户上服务器运行检查指令最终确认)
   - 检查指令见本决策"附录:pg_jieba 检查指令"

2. **采用最小改动方案**:以当前改动更小的方式进行修复,不引入新的分词依赖(不引入 gojieba)

3. **修复策略**:

   **策略 1:查询配置对齐索引(核心修复)**
   - 将 `search_repo.go` 中所有 `to_tsquery('simple', ?)` / `phraseto_tsquery('simple', ?)` / `ts_headline('simple', ...)` 改为动态选择配置
   - 实现一个辅助函数(如 `tsConfig()`),在初始化时探测 pg_jieba 是否可用,缓存结果
   - 查询时使用探测到的配置名(`jiebacfg` 或 `simple`),与索引侧 trigger 函数的逻辑保持一致
   - 涉及位置:`search_repo.go:149`、`169`、`170`、`171`、`175`、`328`、`329`

   **策略 2:查询分词函数支持中文(配合策略 1)**
   - 改造 `splitAndNormalize`,当 pg_jieba 可用时,利用 PG 的 `jiebacfg` 配置做查询分词
   - 具体做法:不再在 Go 层手动拼 tsquery,改为将原始查询词传入 `plainto_tsquery(<cfg>, ?)` 或 `to_tsquery(<cfg>, ?)`,让 PG 的 jieba 分词处理
   - 这样索引和查询都由 PG 的 jieba 统一分词,保证一致性
   - 注意:`plainto_tsquery` 不支持 `:*` 前缀语法,需评估是否保留前缀匹配;若需前缀,改用 `to_tsquery` 并接受 jieba 分词后的 token 拼接

   **策略 3:保留 ILIKE fallback 作为兜底**
   - 认可保留 ILIKE 作为兜底路径,但修复后它不再是主路径
   - tsquery 匹配为主(走 GIN 索引,性能好),ILIKE 为辅(覆盖分词边界 case)

   **策略 4:修复遗留未升级点**
   - `discussion_repo.go`:为 discussions 表增加 search_vector 列(参考 041 迁移模式),查询改用列而非实时计算
   - `ip_repo.go`:查询改用 ips 表已有的 `search_vector` 列(迁移 042 已建),不再实时 `to_tsvector`

   **策略 5:确认 Docker 镜像与文档**
   - 确认生产 Docker 镜像预装 pg_jieba(用户将上服务器验证)
   - 在 `architecture.md §15.4` 或部署文档中明确 pg_jieba 为生产环境必需扩展,记录安装方式

### 实施要点

- **迁移编号**:新增迁移应为 `056_discussions_search_vector.sql`(讨论区 search_vector)等,遵循编号递增
- **向后兼容**:查询配置动态探测,确保开发环境(可能无 pg_jieba)仍可 fallback 到 simple
- **测试验证**:修复后需验证以下场景
  - 搜索"美食"能匹配标题含"美食推荐"的内容(jieba 分词后 "美食" 是独立 token)
  - ts_rank_cd 排序生效(有内容相关性排序)
  - ts_headline 高亮生效(搜索词在摘要中高亮)
  - 讨论区搜索和 IP 搜索功能正常
- **不引入 gojieba**:明确不采用 Go 层分词方案,避免增加二进制依赖和构建复杂度

### 风险与注意事项

1. **plainto_tsquery vs to_tsquery**:切换到 jieba 分词后,需确认 `:*` 前缀匹配的行为。jieba 分词后的多 token 查询,前缀匹配语义可能变化。建议优先用 `plainto_tsquery`(更安全,自动处理特殊字符),放弃 `:*` 前缀匹配(中文搜索前缀匹配价值有限)
2. **ts_headline 配置**:`ts_headline` 的第一个参数也需同步改为动态配置,否则高亮分词与索引不一致
3. **SQLite 测试环境**:`searchContentsWithQueryLike`(SQLite 路径)不受影响,保持现状

### 附录:pg_jieba 检查指令

用户将上服务器运行以下指令确认 pg_jieba 可用性:

```sql
-- 1. 确认扩展已安装
SELECT extname, extversion FROM pg_extension WHERE extname = 'pg_jieba';

-- 2. 确认 jiebacfg 文本搜索配置可用
SELECT cfgname FROM pg_ts_config WHERE cfgname = 'jiebacfg';

-- 3. 测试分词效果(应输出 '美食':1 '推荐':2)
SELECT to_tsvector('jiebacfg', '美食推荐');

-- 4. 抽样检查 content_items 索引实际是否走了 jieba(应看到中文被拆分)
SELECT id, left(title, 20) AS title, search_vector
FROM content_items
WHERE search_vector IS NOT NULL
LIMIT 5;
```

**预期结果**:
- 第 3 步返回 `'美食':1 '推荐':2` → jieba 生效
- 第 3 步返回 `'美食推荐':1` → fallback 到 simple,需排查 pg_jieba 安装
- 第 4 步 search_vector 中中文被拆分为多个 token → 索引侧 jieba 生效

---

## DEC-002:Token 存储方案降级为 P3 长期优化

**关联问题**:H1(前端 Token 存储方案存在 XSS 泄露风险)— 见问题清单"类别 H:安全与合规"

**讨论日期**:2026-06-24

### 问题核实结论

经代码核查,问题清单中 H1 的原始描述需要修正。实际架构比文档 B 描述的安全得多:

| 安全措施 | 状态 | 实现位置 |
|---------|------|---------|
| Access Token 存内存(不挂 window) | ✅ 已实现 | `frontend/lib/api.ts:69` 模块级闭包变量 |
| Refresh Token 在 HttpOnly Cookie | ✅ 已实现 | `frontend/lib/api.ts:88` `credentials: "include"` |
| 页面刷新自动恢复 | ✅ 已实现 | `frontend/contexts/AuthContext.tsx:80-85` 调用 `/auth/refresh` |
| Token 过期自动刷新 | ✅ 已实现 | `frontend/contexts/AuthContext.tsx:86-91` |
| CSRF 保护 | ✅ 已实现 | `frontend/lib/api.ts:48-65` 双重 token |
| 5 分钟状态轮询 | ✅ 已实现 | `frontend/contexts/AuthContext.tsx:107-109` |

文档 B 描述"Refresh Token 计划迁移到 HttpOnly Cookie"实际上已经实现。

### 决策

**降级为 P3 长期优化**。当前架构对 Beta 阶段足够安全,不在 Beta 前改造为 BFF 模式。

### 剩余风险(记录备查)

- XSS 仍可通过模块引用读取内存中的 Access Token(2 小时有效)
- 这是所有 SPA 的通病,除非采用 BFF(Backend For Frontend)模式
- 长期优化方向:Next.js API Routes 作为代理 + `iron-session` 服务端 session,前端 Cookie 仅含 session ID(HttpOnly)

### 实施要点

- Beta 阶段不改动
- 在问题清单中将 H1 优先级从 P0 调整为 P3
- 长期优化时参考问题清单 H1 的方案 B(BFF 模式)

---

## DEC-003:法律文本暂时搁置,用户自行处理

**关联问题**:H3(法律文本完全缺失)— 见问题清单"类别 H:安全与合规"

**讨论日期**:2026-06-24

### 问题核实结论

- `backend/config.yaml` 中 `legal.current_terms_version: ""` 和 `current_privacy_version: ""` 均为空
- `frontend/app/(public)/register/page.tsx:102-103` 提交 `accepted_terms_version: config.legal.current_terms_version || undefined`,空版本号导致提交 `undefined`
- 注册页面有版本号提交逻辑,但无实际法律文本展示(无链接、无弹窗、无文本内容)

### 决策

**暂时搁置**。法律文本起草是法律工作,用户将自行处理。代码层面的修复待法律文本定稿后再实施。

### 搁置期间的处理

- 在问题清单中标记 H3 为"⏸️ 搁置待用户处理"
- 不阻塞其他 P0 修复工作
- 法律文本定稿后需实施的代码改动(备忘):
  1. `config.yaml` 填入版本号(如 `v1.0`)
  2. 前端注册页增加法律文本展示(链接到 `/legal/terms` 和 `/legal/privacy`)
  3. 新增 `/legal/terms` 和 `/legal/privacy` 页面展示文本
  4. 登录流程增加法律文本确认(如版本更新)

### 解除搁置条件

用户完成法律文本起草后,告知版本号和文本内容,即可启动代码实施。

---

## DEC-004:信誉分配置补全,阈值统一为 10

**关联问题**:C1(信誉分配置完全缺失,违反硬编码禁令)+ C2(信誉分阈值差异巨大)+ C3(配置字段命名不统一)+ C4(features 字段缺失)— 见问题清单"类别 C:配置与实现不一致"

**讨论日期**:2026-06-24

### 问题核实结论

- `backend/config.yaml` reputation 部分缺失所有加减分值字段
- `backend/internal/service/reputation_service.go` 硬编码加分值(`3`、`2` 等)
- `architecture.md §7` 与 `config.yaml` 阈值差异 5 倍(50/20 vs 10/5)
- 配置字段命名不统一(`video_max_size_mb` vs `video_max_mb` 等)

### 决策

1. **阈值统一为 10**:以 `config.yaml` 现有值为准,`quality_content_threshold: 10`、`quality_comment_threshold: 5`,更新 `architecture.md` 对齐
2. **增减分值为个位数**:所有信誉分加减值保持个位数(1-3),符合实际业务规模
3. **补全 config.yaml**:将 `architecture.md §7` 中所有 reputation 加减分值字段补入 config.yaml,值参照 architecture.md(均为个位数)
4. **移除硬编码**:重构 `reputation_service.go`,所有分值从配置读取
5. **统一字段命名**:以 `config.yaml` 现有命名为准(`video_max_mb` 等),更新 `architecture.md` 对齐

### 实施策略

**策略 1:补全 config.yaml reputation 配置**

在 `config.yaml` 的 `reputation` 部分补全以下字段(值参照 architecture.md §7,均为个位数):

```yaml
reputation:
  # 现有字段保留
  min_score_for_interaction: 3
  quality_content_threshold: 10
  quality_comment_threshold: 5
  repeat_violation_window_days: 7
  repeat_violation_threshold: 2
  repeat_violation_extra_penalty: -1
  
  # 新增:加分值
  quality_content_bonus: 3          # 发布优质创作(获赞≥阈值)
  quality_comment_bonus: 2          # 发布优质评论(获赞≥阈值)
  contribution_accepted: 3          # PR 被接受合并
  valid_report: 1                   # 成功举报或添加标签被认可
  judge_accuracy_bonus: 1           # 判官准确率奖励
  rehab_course_completed: 1         # 完成素质建设课程
  
  # 新增:扣分值
  malicious_contribution: -3        # 发布恶意/抄袭内容
  malicious_contribution_pr: -3     # 恶意贡献(PR 篡改)
  malicious_comment: -2             # 发布恶意评论
  malicious_report_comment: -2      # 恶意举报正常评论
  malicious_report_tag: -1          # 恶意举报标签
  judge_error: -1                   # 判官错误
```

**策略 2:重构 reputation_service.go**

- 新增 `ReputationConfig` 结构体,从 config.yaml 加载所有分值
- `AwardQualityContent` 等方法改为 `s.config.QualityContentBonus` 而非硬编码 `3`
- 所有加减分方法统一从配置读取

**策略 3:统一字段命名**

以 `config.yaml` 现有命名为准,更新 `architecture.md §7`:
- `video_max_size_mb` → `video_max_mb`
- `mod_archive_max_size_mb` → `mod_max_mb`
- `min_score_for_publish` → `min_score_for_interaction`
- 阈值 `quality_content_threshold: 50` → `10`,`quality_comment_threshold: 20` → `5`

**策略 4:补全 features 字段**

在 `config.yaml` 的 `features` 部分补全:
```yaml
features:
  agent_enabled: true
  judge_enabled: true
  ad_enabled: false
  # 现有字段保留
  payment_enabled: false
  creator_support_enabled: false
  desktop_deploy_enabled: false
```

### 实施要点

- 遵循 AGENTS.md "Read config, not hardcode" 规则
- 配置加载需有默认值兜底(配置缺失时使用 architecture.md 的值,不 panic)
- 重构需 TDD:先写测试验证配置读取,再移除硬编码
- 同时修复 C2(阈值差异)和 C3(字段命名),一并完成

### 验证标准

- `reputation_service.go` 中无硬编码分值(grep 搜索数字常量)
- config.yaml 包含所有 reputation 字段
- `go test ./...` 通过
- `go vet ./...` 无警告

---

## DEC-005:Studio 存根页面完整迁移(方案 B)

**关联问题**:D1(Studio 迁移严重不完整,空存根页面)+ D2(后端 API 路由未迁移)+ D3(新旧路由并存无重定向)— 见问题清单"类别 D:路由系统与页面迁移问题"

**讨论日期**:2026-06-24

### 问题核实结论

- `frontend/app/(protected)/studio/contributors/page.tsx` — 空存根(EmptyState)
- `frontend/app/(protected)/studio/pr-requests/page.tsx` — 空存根
- `frontend/app/(protected)/studio/tag-suggestions/page.tsx` — 空存根
- 功能完整的页面在旧路由 `frontend/app/(protected)/dashboard/*`
- 后端 `routes.go` 只有 `/dashboard/contributors` 路由,无 `/studio/contributors`
- `architecture.md §3.2` 声称已迁移至 `/studio/*` 但实际未完成

### 决策

**方案 B:完整迁移**。将 dashboard 三个页面的逻辑迁移到 Studio,实现真正的迁移。

### 实施策略

**策略 1:前端页面迁移**

将以下 dashboard 页面的完整逻辑(组件、API 调用、状态管理)迁移到对应 Studio 页面:

| 旧路由(迁移源) | 新路由(迁移目标) |
|----------------|------------------|
| `dashboard/contributors/page.tsx` | `studio/contributors/page.tsx` |
| `dashboard/pr-requests/page.tsx` | `studio/pr-requests/page.tsx` |
| `dashboard/tag-suggestions/page.tsx` | `studio/tag-suggestions/page.tsx` |

迁移后,旧 dashboard 页面改为 301 重定向到对应 Studio 路由。

**策略 2:后端 API 路由迁移**

在 `backend/internal/handler/routes.go` 中:
- 新增 `/studio/contributors`、`/studio/pr-requests`、`/studio/tag-suggestions` 路由组
- 复用现有 dashboard handler 逻辑(可路由别名或提取共享 handler)
- 旧 `/dashboard/*` 路由保留兼容(标记 deprecated),返回 301 或继续可用

**策略 3:前端 API 调用更新**

迁移后的 Studio 页面中,API 调用从 `/api/v1/dashboard/*` 改为 `/api/v1/studio/*`。

**策略 4:301 重定向**

旧路由页面改为重定向:
```tsx
// dashboard/contributors/page.tsx
import { redirect } from 'next/navigation';
export default function Page() { redirect('/studio/contributors'); }
```

**策略 5:文档对齐**

`architecture.md §3.2` 已描述 `/studio/*` 路由,迁移后文档与实现一致,无需改动。

### 实施要点

- 遵循 AGENTS.md "Surgical changes only" — 迁移逻辑,不重构
- 前端 API 调用路径需全局搜索替换,确保无遗漏
- 后端路由迁移注意中间件挂载(auth、ratelimit 等)
- 301 重定向需覆盖所有旧路由(dashboard/contents、dashboard/pr-requests 等)
- TDD:迁移后验证功能等价(PR 列表、贡献者管理、标签建议)

### 验证标准

- Studio 三个页面功能完整,与原 dashboard 行为一致
- 旧 dashboard 路由 301 重定向到 Studio
- 后端 `/studio/*` 路由可访问
- `npm run build` 无错误
- `go test ./...` 通过

### 同时修复

- D2(后端 API 路由未迁移)— 策略 2 解决
- D3(新旧路由并存无重定向)— 策略 4 解决

---

## DEC-006:瀑布流完整分页体验(方案 B)

**关联问题**:F1(瀑布流无限滚动没有 URL 状态同步和滚动位置恢复)— 见问题清单"类别 F:UI/UX 交互问题"

**讨论日期**:2026-06-24

### 问题核实结论

经代码核查,问题清单中 F1 的原始描述需要修正。实际情况比"无限滚动无状态恢复"更严重:

- `frontend/app/(public)/original/page.tsx:71-80` — 服务端渲染,只请求 `page_size: "24"`,无 page 参数
- `frontend/components/content/MasonryGrid.tsx` — 纯展示组件,无 loadMore、无分页、无无限滚动
- **实际问题不是"无限滚动无状态恢复",而是根本没有无限滚动** — 只显示前 24 条内容,用户看不到第 25 条以后

这是一个功能缺失,比"状态恢复"更严重。

### 决策

**方案 B:完整体验**。实现无限滚动 + URL page 参数同步 + sessionStorage 滚动位置恢复。

### 实施策略

**策略 1:改造为客户端分页混合模式**

原创区页面当前是纯 SSR(一次性获取 24 条)。改为混合模式:
- SSR 首屏:服务端获取第 1 页(24 条),渲染初始内容(SEO 友好)
- 客户端追加:客户端组件接管分页,滚动到底部时请求下一页

新增客户端组件 `OriginalInfiniteScroll.tsx`,接收 SSR 首屏数据作为初始值。

**策略 2:无限滚动实现**

- 使用 IntersectionObserver 检测底部哨兵元素
- 触发时请求下一页(`/api/v1/contents?zone=original&page=N&page_size=24`)
- 追加到现有列表,更新 `hasMore` 状态
- 加载中显示骨架屏,加载失败显示重试按钮

**策略 3:URL 状态同步**

- 当前页码写入 URL:`/original?category=food&sort=hot&page=3`
- 使用 `history.replaceState` 更新 URL(不触发页面重载)
- 从 URL 恢复:页面初始化时读取 `page` 参数,若 > 1 则需加载到对应页(跳过中间内容或顺序加载)

**策略 4:滚动位置恢复**

- 使用 `sessionStorage` 记录滚动状态:`{ route, scrollY, page }`
- 离开页面(点击内容卡片)时记录当前滚动位置和页码
- 返回页面时:
  1. 读取 sessionStorage 中的 page 和 scrollY
  2. 加载到对应页(若 page > 1,顺序请求填充)
  3. 恢复滚动位置
- 配合 Next.js 的 `scrollRestoration` 或手动实现

**策略 5:内容卡片点击优化**

- 内容卡片点击使用 `router.push`(保留 history 栈)
- 浏览器返回时自动触发滚动位置恢复逻辑

**策略 6:回到顶部按钮**

- 滚动超过 3 页(约 72 条内容)时显示「回到顶部」浮动按钮
- 点击平滑滚动回顶部,重置 page 为 1

### 实施要点

- SSR 首屏保留(SEO 和首屏速度),客户端接管后续分页
- IntersectionObserver 需处理浏览器兼容(现代浏览器均支持)
- sessionStorage 有大小限制,只存 `{route, scrollY, page}`,不存内容数据
- URL page 参数同步注意:分类/排序切换时重置 page 为 1
- 防抖处理:滚动事件和 IntersectionObserver 回调需防抖,避免重复请求
- 二创区(IP 浏览)如有类似问题,可复用此方案

### 验证标准

- 滚动到底部自动加载下一页
- URL 中 page 参数随滚动更新
- 点击内容卡片 → 返回 → 恢复到原滚动位置和页码
- 分享带 page 参数的 URL → 对方打开后定位到对应页
- 切换分类/排序时重置为第 1 页
- `npm run build` 无错误

### 技术风险

- SSR + 客户端分页混合模式需注意 hydration 一致性
- 滚动位置恢复在快速返回时可能闪屏(需 loading 占位)
- URL page 参数 > 1 时的首屏加载策略需权衡(顺序加载 vs 跳转加载)

---

## DEC-007:私信 SSE 实时推送(方案 A,Beta 前实现)

**关联问题**:G6(私信系统无实时推送)— 见问题清单"类别 G:多端/跨平台问题"

**讨论日期**:2026-06-24

### 问题核实结论

- `backend/internal/handler/routes.go:271-277` 有私信 API,但纯 HTTP,无 SSE 端点
- `gin-contrib/sse v1.1.0` 在 go.mod 中作为间接依赖存在,但未被实际使用
- 私信完全依赖 5 分钟轮询,对即时通讯"完全不合格"

### 决策

**方案 A:Beta 前实现 SSE**。`gin-contrib/sse` 已在依赖中,实现成本中等。

### 实施策略

**策略 1:后端 SSE 端点**

新增 `GET /api/v1/messages/stream` SSE 端点:
- 客户端建立 SSE 连接,服务端保持长连接
- 新消息到达时,通过 channel 推送到所有活跃 SSE 连接
- 连接超时 30 分钟,客户端自动重连
- 使用 `gin-contrib/sse` 包实现

消息推送流程:`SendMessage` handler → 写入 DB → 投递到 message broker(channel/Redis pubsub)→ SSE 端点推送

**策略 2:Redis Pub/Sub 支持多实例**

- 单实例可用 Go channel,但多实例部署需 Redis Pub/Sub
- `POST /api/v1/messages` 发送消息后,PUBLISH 到 `dm:user:{recipient_id}` 频道
- SSE 端点 SUBSCRIBE 用户自己的频道,收到消息即推送
- config.yaml 中 `redis.enabled: true` 时用 Pub/Sub,否则降级为本地 channel

**策略 3:前端 SSE 客户端**

- 使用浏览器原生 `EventSource` API
- 私信页面打开时建立 SSE 连接,关闭时断开
- 收到新消息事件 → 追加到消息列表 → 播放提示音(可选)
- 连接断开自动重连(指数退避)

**策略 4:通知系统保持轮询**(已被 DEC-033 更新)

- 通知系统(点赞、评论、系统公告)原计划保持 5 分钟轮询,不改为 SSE
- **更新(DEC-033)**:P3 讨论后决定通知系统也改为 SSE 推送,与私信 SSE 统一基础设施
- 详见 [DEC-033](#dec-033通知系统-sse-实时推送)

### 实施要点

- SSE 连接需 auth 中间件校验(从 cookie 获取 refresh token,换取 access token)
- SSE 连接数限制:每用户最多 3 个并发连接(多标签页)
- 心跳机制:每 30 秒发送 `:heartbeat` 注释,防止代理超时
- 遵循 AGENTS.md:goroutine 内 panic recovery(Task 104)

### 验证标准

- 用户 A 发私信,用户 B 在线时 < 2 秒收到
- 用户 B 离线时,重连后能拉取离线消息
- 多标签页同时打开,消息不重复
- `go test ./...` 通过

---

## DEC-008:草稿系统完整实现(方案 A,Beta 前实现)

**关联问题**:E1(缺少内容草稿系统)— 见问题清单"类别 E:功能缺失"

**讨论日期**:2026-06-24

### 问题核实结论

- `backend/internal/model/content.go:19` status 默认 `pending`,无 `draft` 枚举
- 无 `scheduled_at` 字段
- 完全不支持草稿

### 决策

**方案 A:Beta 前完整实现**。草稿是创作者平台的核心体验。

### 实施策略

**策略 1:数据库变更**

迁移 `057_content_drafts.sql`:
- `content_items.status` 增加 `draft` 枚举值(无需改列类型,已是 VARCHAR)
- 新增 `content_items.scheduled_at TIMESTAMPTZ` 字段(nullable,定时发布用)
- **复用 content_items 表**(不新建 content_drafts 表):`status='draft'` 时为草稿
  - 草稿不触发审核、不公开、不计入统计
  - 草稿的 search_vector 不更新(或 trigger 中判断 status='draft' 时跳过)

**策略 2:后端 API**

- `POST /api/v1/contents` 支持 `status: "draft"` 参数,创建草稿
- `PUT /api/v1/contents/:id` 支持更新草稿
- `POST /api/v1/contents/:id/publish` 草稿转发布(`draft → pending`)
- `GET /api/v1/studio/contents?status=draft` 获取草稿列表(仅作者)
- 草稿不进入搜索索引(search_vector 不更新或标记)
- 草稿不触发 AI 审核、不计入统计

**策略 3:前端自动保存**

- 发布表单组件增加「保存草稿」按钮
- 自动保存:localStorage 每 30 秒备份 + 服务端每 2 分钟同步
- 自动保存时显示"已保存"状态指示器
- 离开页面时若有未保存内容,弹出确认

**策略 4:Studio 草稿管理**

- Studio 内容管理页增加「草稿」Tab
- 草稿列表显示标题、最后编辑时间、操作(编辑/发布/删除)
- 草稿编辑复用发布表单,zone + content_type 锁定

**策略 5:定时发布(顺带实现 E2)**

- 发布表单增加「定时发布」选项(日期时间选择器)
- `scheduled_at` 有值时,按钮文案从「发布」变为「定时发布」
- 后台定时任务每分钟扫描 `status='draft' AND scheduled_at <= NOW()`,自动 `draft → pending`
- Studio 内容管理页显示定时发布状态和倒计时

### 实施要点

- 草稿仅创建者可见,不进入 ContentVisibilitySQL 的公开查询
- 草稿的 search_vector 不更新(或 trigger 中判断 status='draft' 时跳过)
- 遵循 AGENTS.md:软删除,草稿删除也是软删除
- TDD:先写测试验证草稿不可被其他用户访问

### 验证标准

- 创建草稿 → 不出现在公开列表 → Studio 草稿 Tab 可见
- 草稿发布 → 出现在公开列表(经审核后)
- 自动保存正常工作(localStorage + 服务端)
- 定时发布到时间自动转为 pending
- `go test ./...` 通过,`npm run build` 无错误

### 同时修复

- E2(缺少内容定时发布功能)— 策略 5 一并实现

### 风险提示

- **高风险:草稿状态查询污染**。复用 `content_items` 表存储草稿后,所有公开内容查询(列表、详情、搜索、推荐、统计)必须显式过滤 `status != 'draft'`。任何遗漏过滤的查询路径都会导致草稿泄露给非作者用户。实施时必须逐一审计 `ContentVisibilitySQL` 及所有 repository 查询,不能仅依赖 search_vector 跳过。
- **中风险:自动保存竞态**。localStorage 每 30 秒 + 服务端每 2 分钟的自动保存可能与用户手动保存或定时发布转换并发,需用乐观锁或版本号防止覆盖。
- **建议**:TDD 必须覆盖"草稿不可被其他用户访问"的跨用户查询测试,且搜索索引触发器需有针对 `status='draft'` 的回归测试。
- **工作量**:中

---

## DEC-009:用户数据导出完整实现(方案 A,Beta 前实现)

**关联问题**:E3(缺少用户数据导出功能,个人信息保护法合规)— 见问题清单"类别 E:功能缺失"

**讨论日期**:2026-06-24

### 问题核实结论

- 搜索 `export.*user`、`user.*export` 均无结果
- 无任何数据导出 API
- 个保法第 45 条要求支持数据导出

### 决策

**方案 A:Beta 前完整实现**。法律合规是硬性要求。

### 实施策略

**策略 1:后端导出 API**

- `POST /api/v1/users/me/export` → `202 Accepted`,触发异步导出任务
- 限流:每用户每天最多 1 次导出请求
- 异步任务打包用户数据:
  - 个人信息(email、username、avatar_url、bio、reputation、created_at)
  - 发布内容(含附件 OSS 临时下载链接,24h 有效)
  - 评论历史
  - 收藏列表
  - 关注/粉丝列表
  - 信誉分变动日志
  - 通知历史
- 导出格式:JSON 元数据 + 附件清单

**策略 2:异步任务 + 通知**

- 导出任务入队列(config.yaml `queue.enabled: true` 时用 Redis,否则本地 goroutine)
- 任务完成后:
  - 生成 OSS 临时下载链接(24h 有效)
  - 通过通知系统告知用户("您的数据导出已就绪,点击下载")
  - 通知包含下载链接
- 任务失败时通知用户失败原因

**策略 3:前端入口**

- 设置页 `/settings/privacy` 增加「导出我的数据」区块
- 显示导出说明 + 「申请导出」按钮
- 显示历史导出记录(最近 5 次,含状态和下载链接)
- 下载链接过期时显示"已过期,请重新申请"

**策略 4:Admin 审计**

- 每次导出操作记录到 admin_audit_logs
- Admin 后台可查看导出审计记录

### 实施要点

- 导出任务需 panic recovery(Task 104)
- OSS 临时链接使用 STS Token,24h 过期
- 大用户(内容多)导出可能耗时,需分批处理
- 遵循 AGENTS.md:结构化日志记录导出操作
- 附件不打包进 ZIP(太大),仅提供 OSS 临时链接清单

### 验证标准

- 申请导出 → 收到通知 → 下载链接有效
- JSON 包含完整用户数据
- 附件链接 24h 内可下载
- 限流生效(每天 1 次)
- Admin 审计日志有记录
- `go test ./...` 通过

---

## DEC-010:实现收藏集页面(方案 B)

**关联问题**:E5(OriginalSidebar 导航链接错误)— 见问题清单"类别 E:功能缺失"

**讨论日期**:2026-06-24

### 问题核实结论

- `frontend/components/original/OriginalSidebar.tsx` 中"收藏"和"我的原创"都指向 `/studio/contents`
- 搜索 `/collections` 路由 — 不存在
- 根因:收藏集功能(Task 122-123)未实现,所以"收藏"无处可指

### 决策

**方案 B:实现收藏集页面**,接入导航。

### 实施策略

**策略 1:后端收藏集 API**(参照 AGENTS.md Task 122-123 规范)

- `collections` 表:id, user_id, title, description, is_public, created_at, updated_at
- `collection_items` 关联表:collection_id, content_id, added_at, note
- UNIQUE 约束 `(collection_id, content_id)` 防止重复添加
- API:
  - `GET /api/v1/collections` — 我的收藏集列表
  - `POST /api/v1/collections` — 创建收藏集
  - `GET /api/v1/collections/:id` — 收藏集详情(含内容列表)
  - `PUT /api/v1/collections/:id` — 更新收藏集
  - `DELETE /api/v1/collections/:id` — 删除收藏集(软删除)
  - `POST /api/v1/collections/:id/items` — 添加内容到收藏集
  - `DELETE /api/v1/collections/:id/items/:contentId` — 移除内容
  - `GET /api/v1/collections?content_id=X` — 查询某内容已加入的收藏集(用于详情页"添加到收藏集"按钮)

**策略 2:前端收藏集页面**

- `/collections` — 我的收藏集列表页(公开/私有筛选)
- `/collections/:id` — 收藏集详情页(内容瀑布流 + 编辑)
- `/studio/collections` — Studio 内的收藏集管理(创建/编辑/删除)
- 内容详情页 ReactionBar 区域增加「添加到收藏集」按钮(弹窗选择收藏集)

**策略 3:OriginalSidebar 链接修复**

- "收藏" → `/collections`(我的收藏集列表)
- "我的原创" → `/studio/contents?zone=original`(我的原创内容)

**策略 4:权限控制**

- 私有收藏集仅创建者可见
- 公开收藏集所有用户可浏览
- API 层校验 is_public + user_id

### 实施要点

- 遵循 AGENTS.md Task 122-123 规范
- 收藏集筛选:支持按 content_type 过滤收藏集内内容
- 软删除:删除收藏集是软删除,不删除内容本身
- TDD:先写测试验证去重和权限

### 验证标准

- 创建收藏集 → 添加内容 → 收藏集详情页展示
- 同一收藏集内不能添加重复内容
- 私有收藏集其他用户不可见
- OriginalSidebar 链接正确
- `go test ./...` 通过,`npm run build` 无错误

---

## DEC-011:设计系统统一,homepage-v0.html 归档

**关联问题**:A1(设计系统三重冲突)— 见问题清单"类别 A:文档一致性与维护问题"

**讨论日期**:2026-06-24

### 问题核实结论

三套设计系统并存:
- `architecture.md §10.4`:GitHub blue `#0969da`(过时)
- `design/design-system.md`:Indigo `#4F46E5`(与实际实现一致)
- `design/homepage-v0.html`:Muted blue-gray `#526B8C` + Sora 字体(早期设计,未集成)

实际实现使用 Indigo,匹配 design-system.md。

### 决策

1. **以 `design/design-system.md` 为唯一设计系统权威**
2. **`architecture.md §10.4` 色值表更新**:删除过时的 GitHub blue 色值表,改为指向 design-system.md
3. **`homepage-v0.html` 归档**:移至 `docs/archive/homepage-v0.html`,标记为"早期设计探索,非当前实现参考"
4. **同步 A2(侧边栏宽度矛盾)**:统一为 design-system.md 的数值(228px/48px),更新 architecture.md §13.2

### 实施策略

**策略 1:更新 architecture.md**

- §10.4 色值表:删除或替换为"设计系统详见 `design/design-system.md`"
- §13.2 侧边栏宽度:更新为 228px(展开)/ 48px(折叠)
- §13.1 Header 高度:确认并更新为 52px(与实际实现一致)

**策略 2:归档 homepage-v0.html**

- 将 `design/homepage-v0.html` 移至 `docs/archive/homepage-v0.html`
- 在 `docs/archive/README.md` 中说明:"早期首页设计探索,暖色调 + Sora 字体方案,未采纳。当前实现以 design-system.md 为准。"

**策略 3:更新引用**

- 检查所有引用 homepage-v0.html 的文档,更新或移除引用
- 本审查报告(问题清单)中保留引用作为历史记录,但标注"已归档"

### 同时修复

- A2(侧边栏宽度矛盾)— 策略 1 一并解决

### 风险提示

- **中风险:第三方工具色值引用残留**。`architecture.md §10.4` 历史上使用 GitHub blue `#0969da` 色值表,被部分第三方工具(如截图标注、设计稿导出)直接引用。删除色值表后,这些工具的配置需同步更新,否则可能出现色值不一致。
- **低风险:homepage-v0.html 归档后引用失效**。归档后原路径 `design/homepage-v0.html` 不再存在,任何硬编码该路径的链接会 404。需在 `docs/archive/README.md` 中说明并检查所有引用。
- **建议**:归档操作后全局搜索 `homepage-v0.html` 和 `#0969da`(非 chart 上下文)确认无残留引用;`globals.css` 中 `--chart-1: #0969da` 属图表色板,与品牌主色无关,不需修改但建议加注释说明。
- **工作量**:小

---

## DEC-012:归档前标注 UI Design.md 过时内容（原计划:移除二级筛选描述）

**关联问题**:A3(原创区导航结构文档矛盾)— 见问题清单"类别 A:文档一致性与维护问题"

**讨论日期**:2026-06-24

### 问题核实结论

- `architecture.md §10.6`:明确"无二级内容类型筛选",单层分类 Tab
- `design/UI Design.md P06`:描述有"二级内容类型子筛选"(已过时)
- `task.json Task 64`:标注"已被 Task 90 覆盖"
- 实际实现:单层分类 Tab,符合 architecture.md

### 决策

更新 UI Design.md P06,移除二级筛选描述,与 Task 90 设计对齐。

### 实施策略

**注意**:此决策与 DEC-013(归档 UI Design.md)冲突。DEC-013 决定归档 UI Design.md,因此 DEC-012 的"更新"操作改为"归档前标注过时内容"。

**最终处理**(与 DEC-013 协调):
- UI Design.md 整体归档(DEC-013)
- 归档时在文件顶部添加标注:"此文档已过时,原创区导航已改为单层分类 Tab(Task 90),路由已迁移至 /studio/*。当前 UI 设计权威为 `design/ui-spec.md`。"
- 不逐条修正过时内容(因为整个文档都过时了)

### 同时修复

- A3 由 DEC-013 的归档操作一并解决

---

## DEC-013:归档 UI Design.md,同步相关文档

**关联问题**:A4(UI Design.md 严重过时)— 见问题清单"类别 A:文档一致性与维护问题"

**讨论日期**:2026-06-24

### 问题核实结论

`design/UI Design.md` 严重过时:
- 仍描述 `/publish` 和 `/dashboard/*` 路由(未提及 `/studio/*`)
- 仍描述原创区"二级内容类型子筛选"(已被 Task 90 移除)
- 已被 `design/ui-spec.md` 取代

引用 UI Design.md 的文档:
- `design/doc-review-prompt.md`(多处引用)
- `CLAUDE.md`、`frontend/CLAUDE.md`(需确认)
- `AGENTS.md` 引用的是 `design/ui-spec.md`(小写,非 UI Design.md)

### 决策

**归档 UI Design.md,以 `design/ui-spec.md` 为唯一 UI 设计权威**。同步更新所有引用文档。

### 实施策略

**策略 1:归档 UI Design.md**

- 将 `design/UI Design.md` 移至 `docs/archive/UI Design.md`
- 在文件顶部添加归档标注:
  ```
  > ⚠️ 此文档已归档(2026-06-24)。
  > 内容已过时:路由仍为 /publish、/dashboard(已迁移至 /studio/*),
  > 原创区仍描述二级筛选(已改为单层分类 Tab,Task 90)。
  > 当前 UI 设计权威:`design/ui-spec.md`。
  > 当前设计系统权威:`design/design-system.md`。
  ```

**策略 2:更新引用文档**

需检查并更新以下文档中对 `UI Design.md` 的引用:

| 文档 | 引用位置 | 更新方式 |
|------|---------|---------|
| `design/doc-review-prompt.md` | 第 39、53、66、72-74 行 | 替换为 `design/ui-spec.md`,或标注"UI Design.md 已归档" |
| `CLAUDE.md` | 需检查 | 若有引用,替换为 `design/ui-spec.md` |
| `frontend/CLAUDE.md` | 需检查 | 若有引用,替换为 `design/ui-spec.md` |
| `AGENTS.md` | 已用 `ui-spec.md` | 无需改动 |

**策略 3:确认 ui-spec.md 完整性**

- 确认 `design/ui-spec.md` 覆盖了 UI Design.md 的所有页面定义(P01-P21+)
- 若有缺失,需补充到 ui-spec.md
- 确认 ui-spec.md 已同步 Task 90 设计变更(单层分类 Tab)

**策略 4:归档目录说明**

- 创建/更新 `docs/archive/README.md`,说明归档文件的状态和原因
- 归档文件保留但标注"非当前参考"

### 实施要点

- 归档不等于删除,保留历史记录
- 更新引用时注意路径变化(`design/UI Design.md` → `docs/archive/UI Design.md`)
- 同步检查 task.json 中的 `ui_spec_ref` 字段是否引用了 UI Design.md(应引用 ui-spec.md)

### 验证标准

- `design/UI Design.md` 已移至 `docs/archive/`
- 所有引用文档已更新
- `design/ui-spec.md` 为唯一 UI 设计权威
- `grep -r "UI Design" --include="*.md"` 无活跃引用(仅归档目录和历史审查报告中有)

### 同时修复

- A3(原创区导航文档矛盾)— 归档时标注过时,一并解决
- A4(UI Design.md 过时)— 归档解决

---

## DEC-014:后端 DDD 完整重构

**讨论日期**:2026-06-24
**关联问题**:B2(代码组织缺乏领域边界)
**决策**:方案 B — 完整重构为 DDD 结构,后续开发也按 DDD 结构

### 问题核实结论

后端完全按技术层组织(handler/service/repository/model),同一领域代码分散在 4 个目录。40+ handler、35+ service、22+ repository 文件跨目录维护,领域边界模糊。

### 决策

一次性完整重构为领域驱动设计(DDD)结构,且**后续所有新功能开发也必须按 DDD 结构组织**。

### 实施策略

**目标目录结构**:
```
backend/internal/
├── domain/                    # 领域层(核心业务逻辑)
│   ├── content/              # 内容领域
│   │   ├── entity.go         # 领域实体
│   │   ├── repository.go     # 仓储接口
│   │   ├── service.go        # 领域服务
│   │   └── value_object.go   # 值对象
│   ├── user/                 # 用户领域
│   ├── judge/                # 判官领域
│   ├── ip/                   # IP 领域
│   ├── notification/         # 通知领域
│   ├── search/               # 搜索领域
│   ├── reputation/           # 信誉分领域
│   ├── studio/               # 创作者工作室领域
│   └── shared/               # 共享内核(通用值对象、事件等)
├── application/               # 应用层(用例编排)
│   ├── command/              # 命令处理器
│   └── query/                # 查询处理器
├── infrastructure/            # 基础设施层
│   ├── persistence/          # 仓储实现(GORM)
│   ├── cache/                # Redis 缓存
│   ├── queue/                # 消息队列
│   ├── oss/                  # 阿里云 OSS
│   ├── llm/                  # LLM 调用
│   └── config/               # 配置加载
├── interfaces/                # 接口层
│   ├── http/                 # HTTP handler + 中间件 + 路由
│   └── dto/                  # 数据传输对象
└── container/                 # DI 容器(保留)
```

**重构原则**:
1. **保持 API 契约不变**:所有 HTTP 路由和响应格式不变,仅内部代码重组
2. **分领域迁移**:按 content → user → judge → ip → notification → search → reputation → studio 顺序逐领域迁移
3. **测试先行**:每个领域迁移前先确保现有测试覆盖,迁移后测试全部通过
4. **一次一个领域**:每个领域迁移完成后提交,不混合多个领域

**风险控制**:
- 重构期间保持 `go build ./...` 和 `go test ./...` 通过
- 每个领域迁移后运行完整测试套件
- 如果某个领域迁移导致测试失败,回退该领域迁移

### 验证标准

- [ ] `go build ./...` 无错误
- [ ] `go test ./...` 无错误
- [ ] `go vet ./...` 无警告
- [ ] 所有 API 路由和响应格式不变
- [ ] 新目录结构符合 DDD 分层(domain/application/infrastructure/interfaces)
- [ ] architecture.md 更新为新目录结构说明
- [ ] AGENTS.md 更新代码规范,要求新功能按 DDD 结构开发

### 风险提示

- **高风险**:涉及 100+ 文件移动和 import 路径修改
- **建议**:在独立分支上执行,完整测试通过后再合并
- **工作量**:大

---

## DEC-015:数据分析基础增强

**讨论日期**:2026-06-24
**关联问题**:E4(数据分析不足)
**决策**:方案 B — 基础增强,在现有 overview 页面增加更多指标

### 问题核实结论

仅有 overview 页面的基础统计(4 个卡片 + 趋势图 + 排行),无留存、转化、流量来源等深度分析。

### 决策

在现有 overview 页面增加更多指标,不新增专门的分析页面。

### 实施策略

**新增指标**:
1. **7 日 / 30 日对比**:在现有 StatsCard 中增加环比数据(如"浏览量 +12%")
2. **分类分布**:饼图展示创作者内容在各分类的分布
3. **互动率趋势**:点赞率、评论率、收藏率趋势图
4. **最佳发布时间**:热力图展示各时段发布内容的平均互动量

**后端 API**:
- 扩展 `GET /api/v1/stats/summary` 返回环比数据
- 新增 `GET /api/v1/me/content-distribution` 返回分类分布
- 新增 `GET /api/v1/me/engagement-trend?days=30` 返回互动率趋势

**前端**:
- 在 `studio/overview/page.tsx` 中增加新组件
- 复用现有 Charts 组件库

### 验证标准

- [ ] overview 页面显示 7 日/30 日环比数据
- [ ] overview 页面显示分类分布饼图
- [ ] overview 页面显示互动率趋势
- [ ] 后端 API 返回正确数据
- [ ] `npm run build` 无错误
- [ ] `go test ./...` 无错误

### 风险提示

- 后端聚合查询可能影响数据库性能,需添加适当索引
- **工作量**:中

---

## DEC-016:Tauri 离线功能暂缓

**讨论日期**:2026-06-24
**关联问题**:G1(Tauri 无离线)
**决策**:方案 B — 暂缓,标记为 P3 长期优化

### 问题核实结论

Tauri 客户端完全依赖在线 API,无任何离线缓存机制。

### 决策

桌面端当前定位是在线工具,离线非核心需求。标记为 P3 长期优化,不在当前修复计划中实施。

### 未来实施方向(供参考)

当需要实现时:
1. 引入 SQLite 本地数据库缓存已下载内容
2. Service Worker 或 Tauri 的 http 插件缓存层
3. 在线时同步,离线时读取本地缓存
4. 冲突解决策略(最后写入优先或手动合并)

### 验证标准

- 问题清单中 G1 优先级调整为 P3
- 标注"暂缓,长期优化"

---

## DEC-017:PWA 支持暂缓

**讨论日期**:2026-06-24
**关联问题**:G2(缺 PWA)
**决策**:方案 B — 暂缓,标记为 P3 长期优化

### 问题核实结论

前端无 manifest.json、无 Service Worker、无 next-pwa 配置。

### 决策

移动端当前通过浏览器访问,PWA 非必需。标记为 P3 长期优化。

### 未来实施方向(供参考)

当需要实现时:
1. 添加 `manifest.json`(名称、图标、主题色、显示模式)
2. 使用 `next-pwa` 或 `@serwist/next` 集成 Service Worker
3. 离线缓存策略:App Shell + API 数据 stale-while-revalidate
4. 添加"添加到主屏幕"提示

### 验证标准

- 问题清单中 G2 优先级调整为 P3
- 标注"暂缓,长期优化"

---

## DEC-018:验证码和邮件配置切换为生产模式

**讨论日期**:2026-06-24
**关联问题**:H4(验证码/邮件开发模式)
**决策**:用户提供真实配置,切换为生产模式

### 问题核实结论

config.yaml 中 `captcha.provider: "bypass"` + `smtp.mode: "logger"`,均为开发模式。`ValidateRelease()` 会拒绝这两种模式,但 config.yaml 本身需要更新。

### 决策

用户已有阿里云验证码和 SMTP 信息,切换为生产配置。

### 实施策略

**验证码配置**:
```yaml
captcha:
  provider: "aliyun_v2"
  aliyun_v2:
    access_key_id: ""        # 从 env: CAPTCHA_ALIYUN_ACCESS_KEY_ID 注入
    access_key_secret: ""    # 从 env: CAPTCHA_ALIYUN_ACCESS_KEY_SECRET 注入
    app_key: ""              # 从 env: CAPTCHA_ALIYUN_APP_KEY 注入
```

**SMTP 配置**:
```yaml
smtp:
  mode: "smtp"
  host: ""                   # 从 env: SMTP_HOST 注入
  port: 465
  username: ""               # 从 env: SMTP_USERNAME 注入
  password: ""               # 从 env: SMTP_PASSWORD 注入
  from_name: "OmniCraft"
  from_email: ""             # 从 env: SMTP_FROM_EMAIL 注入
```

**安全规则**:
- 敏感信息(AccessKey、密码)不写入 config.yaml,通过 env 注入
- config.go 确认 env 覆盖逻辑已实现(需核查)
- config.yaml 中只保留占位符和注释说明

### 验证标准

- [ ] config.yaml 中 `captcha.provider` 改为 `aliyun_v2`
- [ ] config.yaml 中 `smtp.mode` 改为 `smtp`
- [ ] 敏感字段通过 env 注入,不硬编码
- [ ] `ValidateRelease()` 校验通过
- [ ] 注册/登录验证码功能正常
- [ ] 密码重置邮件能实际发送

### 风险提示

- 需确认 config.go 是否已支持所有字段的 env 覆盖,缺失的需补充
- **工作量**:小

---

## DEC-019:启用队列系统

**讨论日期**:2026-06-24
**关联问题**:I4(队列系统禁用)
**决策**:方案 A — 启用队列系统

### 问题核实结论

`queue.enabled: false`,队列代码完整(Redis Stream Broker + 6 个 worker:review、notification、count、embedding、dlq)但未启用。禁用时消息被 NoopProducer 丢弃,worker 不启动。

### 决策

启用队列系统,将 AI 审核、通知发送、计数更新、向量化等异步任务改为队列处理。

### 实施策略

**配置变更**:
```yaml
queue:
  enabled: true
  consumer_group: "omnicraft-workers"
  max_retries: 3
  retry_delay: "5s"
```

**启动前检查**:
1. 确认 Redis Stream 可用(`XGROUP CREATE` 权限)
2. 确认 6 个 worker 的依赖服务正常(LLM API、OSS、PostgreSQL)
3. 确认 DLQ(死信队列)处理逻辑正确

**Worker 启动顺序**:
1. `dlq_worker`(死信处理,最先启动)
2. `review_worker`(AI 审核)
3. `notification_worker`(通知发送)
4. `count_worker`(计数更新)
5. `embedding_worker`(向量化)

**监控**:
- 添加队列积压监控(队列长度告警)
- Worker panic recovery(已有,确认覆盖)

### 验证标准

- [ ] `queue.enabled: true`
- [ ] 发布内容后,AI 审核通过队列异步执行(非同步阻塞)
- [ ] 评论/点赞后,通知通过队列异步发送
- [ ] 内容发布后,向量化通过队列异步执行
- [ ] DLQ 正确处理失败消息
- [ ] `go test ./...` 无错误
- [ ] 队列积压不超过阈值时系统正常

### 风险提示

- 启用后需监控 Redis 内存使用(Stream 数据持久化)
- 消息消费失败的重试逻辑需验证
- **工作量**:中

---

## DEC-020:Agent LLM 配置修复

**讨论日期**:2026-06-24
**关联问题**:C5(Agent LLM 配置混乱)
**决策**:去除过时注释,填入真实 API Key

### 问题核实结论

config.yaml 注释写"留空,改用 env var 注入(需修复 config.go)",但 config.go 早已实现 env 注入。用户已有 DeepSeek API Key 可用。

### 决策

去除过时注释,填入真实 DeepSeek API Key。

### 实施策略

**config.yaml 修改**:
```yaml
agent:
  llm_provider: "deepseek"
  llm_model: "deepseek-chat"
  llm_api_base: "https://api.deepseek.com/v1"
  llm_api_key: ""           # 从 env: AGENT_LLM_API_KEY 注入,或直接填入
  web_agent_enabled: true
```

**安全规则**:
- API Key 优先通过 env `AGENT_LLM_API_KEY` 注入
- 如果选择直接填入 config.yaml,确保 config.yaml 不被 git 追踪(检查 .gitignore)
- config.go 的 env 覆盖逻辑已正确实现,无需修改

**注释更新**:
- 删除"需修复 config.go"注释
- 改为"# 通过 AGENT_LLM_API_KEY env 注入,或直接填入"

### 验证标准

- [ ] config.yaml 注释更新
- [ ] `llm_provider` 设为 `deepseek`
- [ ] `llm_api_base` 设为 `https://api.deepseek.com/v1`
- [ ] API Key 通过 env 或 config.yaml 正确加载
- [ ] Agent 功能正常(如 AI 审核调用 LLM)
- [ ] `ValidateRelease()` 校验通过

### 风险提示

- 确保 API Key 不泄露到 git(检查 .gitignore 是否包含 config.yaml 或使用 config.local.yaml)
- **工作量**:小

---

## DEC-021:审查报告归档(先确认问题已解决)

**讨论日期**:2026-06-24
**关联问题**:A5(审查报告未整合)
**决策**:归档旧报告,但先确认 code-review-log.md 记载的问题已解决

### 问题核实结论

`docs/review/` 有 24 个报告,旧索引 `code-review-log.md` 仅覆盖到 Task 86,15+ 个新报告未纳入。

### 决策

1. **先核查**:逐一确认 `code-review-log.md` 中记录的问题是否已解决
2. **后归档**:确认全部解决后,将历史报告归档到 `docs/review/archive/`
3. **以本次设计审查的合并报告为唯一问题来源**

### 验证标准

- [ ] code-review-log.md 中所有问题确认已解决(或已纳入本次设计审查)
- [ ] 历史报告归档到 `docs/review/archive/`
- [ ] 本次设计审查合并报告为唯一问题来源

### 风险提示

- 核查工作量中等(需逐一比对 code-review-log.md 中记录的 86 个任务的问题)
- **工作量**:中

---

## DEC-022:task.json 保留作为历史记录

**讨论日期**:2026-06-24
**关联问题**:A6(task.json 与路线图并行)
**决策**:方案 B — 保留,不归档

### 问题核实结论

task.json 100 个任务全部 `passes: true`(已 100% 完成),但仍与 Beta roadmap 并行存在。

### 决策

保留 task.json 作为历史记录,不归档。在 AGENTS.md 中明确 task.json 为"已完成的历史 MVP 任务账本",Beta roadmap 为"当前活跃任务来源"。

### 验证标准

- [ ] AGENTS.md 模式 B 说明中添加"task.json 已 100% 完成,仅作历史记录"
- [ ] task.json 不做任何修改

---

## DEC-023:首页路由合并

**讨论日期**:2026-06-24
**关联问题**:D4(首页路由冗余)
**决策**:合并 — 删除 /home,品牌介绍功能合并到 /;删除 /ips,IP 浏览功能合并到 /

### 问题核实结论

存在三个功能重叠的路由:`/`(首页推荐流)、`/home`(品牌介绍页)、`/ips`(IP 浏览页)。

### 决策

1. **删除 `/home`**:将品牌介绍功能(PrototypeLanding 组件)合并到 `/` 的未登录状态
2. **删除 `/ips`**:将 IP 浏览功能合并到 `/` 的 IP Tab 中
3. **设置 301 重定向**:`/home` → `/`,`/ips` → `/`

### 实施策略

- 未登录用户访问 `/` 看到品牌介绍 + 热门内容预览
- 登录用户访问 `/` 看到推荐流(含原创和 IP 两个 Tab)
- `/ips` 的 IP 浏览功能作为 `/` 的一个 Tab 或子路由保留

### 验证标准

- [ ] `/home` 删除,301 重定向到 `/`
- [ ] `/ips` 删除,301 重定向到 `/`
- [ ] 未登录用户在 `/` 看到品牌介绍
- [ ] 登录用户在 `/` 看到推荐流
- [ ] IP 浏览功能可通过 `/` 的 Tab 访问
- [ ] `npm run build` 无错误

---

## DEC-024:下载权限复用互动阈值

**讨论日期**:2026-06-24
**关联问题**:C6(下载权限逻辑不清)
**决策**:复用 min_score_for_interaction,其他类似权限也同步使用此分数

### 问题核实结论

下载权限复用 `min_score_for_interaction=3`,无独立 `min_score_for_download` 字段。

### 决策

不新增独立字段,所有需要信誉分门槛的功能(下载、评论、发布、众裁、点赞点踩)统一使用 `min_score_for_interaction`。在文档中明确说明。

### 验证标准

- [ ] AGENTS.md 和 architecture.md 明确说明"所有信誉分门槛统一使用 min_score_for_interaction"
- [ ] config.yaml 中 min_score_for_interaction 注释说明"适用于发布、评论、众裁、点赞、下载等所有受保护操作"

---

## DEC-025:移动端适配参考小红书 APP

**讨论日期**:2026-06-24
**关联问题**:F5(移动端适配缺陷)
**决策**:参考小红书 APP 方式实现移动端导航

### 问题核实结论

Header 移动端适配完善(汉堡菜单),但 Sidebar 固定宽度 228px,无移动端抽屉模式,挤压主内容区。

### 决策

参考小红书 APP 的移动端导航模式实现。

### 实施策略

**小红书 APP 导航模式**:
1. **底部 Tab 栏**:首页、发现、发布(+)、消息、我
2. **无侧边栏**:移动端完全隐藏 Sidebar
3. **顶部搜索栏**:可点击展开全屏搜索
4. **内容全宽**:主内容区域占据全部宽度

**OmniCraft 适配方案**:
- **移动端(< sm 断点)**:
  - 隐藏 OriginalSidebar 和 StudioSidebar
  - 底部 Tab 栏:推荐、原创、发布(+)、消息、我的
  - 顶部 Header 保留搜索和头像
  - 内容区域全宽
- **桌面端(≥ sm 断点)**:
  - 保持现有 Sidebar 布局
  - 无底部 Tab 栏

**技术实现**:
- 使用 `sm:hidden` 和 `hidden sm:flex` 控制显示
- 底部 Tab 栏复用 StudioSidebar 已有的移动端底部 Tab 组件模式
- 发布按钮(+)点击后跳转到 `/studio/publish/original`

### 验证标准

- [ ] 移动端 OriginalSidebar 隐藏,显示底部 Tab 栏
- [ ] 底部 Tab 栏包含:推荐、原创、发布(+)、消息、我的
- [ ] 内容区域全宽,无侧边栏挤压
- [ ] 桌面端保持现有 Sidebar 布局
- [ ] `npm run build` 无错误
- [ ] 浏览器移动端模拟器测试通过

---

## DEC-026:Tauri 账号同步暂缓

**讨论日期**:2026-06-24
**关联问题**:G4(账号同步未验证)
**决策**:暂缓

### 问题核实结论

Tauri 客户端无与 Web 端账号同步机制,URL scheme 中的 token 是部署授权令牌,非 JWT 登录令牌。

### 决策

桌面端当前定位是部署工具,不需要完整登录态。暂缓,标记为长期优化。

### 验证标准

- 问题清单中 G4 标注"暂缓,长期优化"

---

## DEC-027:推荐冷启动增加兴趣选择(可跳过)

**讨论日期**:2026-06-24
**关联问题**:I1(推荐冷启动)
**决策**:方案 A — 注册时增加兴趣分类选择,但允许用户跳过

### 问题核实结论

已实现冷启动回退(互动 < 10 用纯热门),但无注册时兴趣选择,新用户需积累 10 次互动才能获得个性化推荐。

### 决策

注册时增加兴趣分类选择,从 11 个一级分类中选 3-5 个,用于冷启动推荐。**允许用户跳过**,跳过则使用纯热门推荐(现有逻辑)。

### 实施策略

**注册流程变更**:
1. 用户完成基本信息填写(邮箱/密码)后
2. 进入"选择你感兴趣的领域"页面
3. 展示 11 个一级分类(影视、游戏、文学、宠物、美食等)的卡片
4. 用户选择 3-5 个(可选,可跳过)
5. 完成注册

**后端变更**:
- `users` 表新增 `preferred_categories` 字段(JSON 数组)或新建 `user_preferences` 表
- 推荐引擎冷启动时:有偏好 → 按偏好分类的热门内容推荐;无偏好 → 纯热门(现有逻辑)
- 用户互动达到 10 次后,切换到个性化推荐(现有逻辑)

**前端变更**:
- 新增注册流程的兴趣选择步骤
- 用户可在设置页面修改兴趣偏好

### 验证标准

- [ ] 注册流程包含兴趣选择步骤(可跳过)
- [ ] 选择兴趣的新用户获得按偏好分类的热门推荐
- [ ] 跳过兴趣选择的新用户获得纯热门推荐(现有逻辑)
- [ ] 设置页面可修改兴趣偏好
- [ ] `go test ./...` 无错误
- [ ] `npm run build` 无错误

---

## DEC-028:向量索引升级为 HNSW

**讨论日期**:2026-06-24
**关联问题**:I2(向量索引升级路径)
**决策**:方案 A — 升级为 HNSW

### 问题核实结论

使用 IVFFlat(lists=100),无 HNSW。数据量增大时性能不如 HNSW。

### 决策

创建新迁移将 IVFFlat 改为 HNSW 索引。

### 实施策略

**新迁移文件**:
```sql
-- 058_vector_index_hnsw.sql
DROP INDEX IF EXISTS idx_content_embeddings_ivfflat;
CREATE INDEX idx_content_embeddings_hnsw
    ON content_embeddings USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);
```

**HNSW 参数说明**:
- `m = 16`:每个节点的最大连接数(平衡内存和搜索质量)
- `ef_construction = 64`:构建时的搜索宽度(平衡构建速度和索引质量)

**注意事项**:
- HNSW 索引构建比 IVFFlat 慢,但查询性能更好
- 构建期间需要锁表,建议在低峰期执行
- 构建完成后查询性能提升 2-5 倍(取决于数据量)

### 验证标准

- [ ] 新迁移创建 HNSW 索引
- [ ] 旧 IVFFlat 索引删除
- [ ] 向量搜索功能正常
- [ ] 查询性能不退化
- [ ] `go test ./...` 无错误

---

## DEC-029:热门排行配置项暴露

**讨论日期**:2026-06-24
**关联问题**:I3(热门排行更新频率)
**决策**:方案 A — 在 config.yaml 增加配置项

### 问题核实结论

默认 10 分钟更新,可配置,但 `rank_interval_min` 和 `hot_rank_zset_ttl` 未在 config.yaml 显式暴露。

### 决策

在 config.yaml 中暴露热门排行相关配置项。

### 实施策略

**config.yaml 新增**:
```yaml
hot_rank:
  rank_interval_min: 10        # 热门排行更新间隔(分钟)
  hot_rank_zset_ttl: 3600      # Redis ZSet TTL(秒)
```

**config.go 变更**:
- 确认 `HotRankConfig` 结构体已存在
- 确保 config.yaml 字段名与 config.go mapstructure tag 一致

### 验证标准

- [ ] config.yaml 包含 hot_rank 配置段
- [ ] 修改配置值后,热门排行更新频率随之变化
- [ ] `go test ./...` 无错误

---

## DEC-030:迁移验证范围更新

**讨论日期**:2026-06-24
**关联问题**:J1(迁移编号超出预留)
**决策**:方案 A — 更新 R-01 验证范围,覆盖 001-055

### 问题核实结论

最大编号 055,超出预留的 049-052 共 3 个,R-01 验证范围需更新。

### 决策

更新 Beta roadmap R-01 验证步骤,将迁移验证范围从 001-052 扩展到 001-055(或实际最大编号)。

### 实施策略

- 更新 `docs/superpowers/plans/2026-05-30-omnicraft-beta-release-validation.md` 中 R-01 的验证范围
- 在 roadmap 中更新预留编号说明
- 后续新迁移从 056 开始编号

### 验证标准

- [ ] R-01 验证步骤覆盖 001-055
- [ ] roadmap 预留编号说明更新
- [ ] 后续迁移从 056 开始

---

## DEC-031:软删除策略部分统一

**讨论日期**:2026-06-24
**关联问题**:J2(软删除策略分散)
**决策**:方案 B — 部分统一,以软删除为主,确保物理删除的数据完全无意义

### 问题核实结论

仅 `users` 和 `content_items` 有 `deleted_at`,`comments`/`discussions` 用 `Status` 字段,`reactions`/`favorites`/`follows` 物理删除。违反 AGENTS.md 规则 18。

### 决策

以软删除为主,仅对确定完全无意义的数据保留物理删除。

### 实施策略

**软删除(有数据保留价值)**:
| 表 | 理由 | 实施方式 |
|----|------|---------|
| comments | 评论可能有回复引用,删除后需保留"已删除"占位 | 新增 deleted_at 字段 |
| discussions | 讨论帖是用户生成内容,有保留价值 | 新增 deleted_at 字段 |
| reactions | 点赞/点踩历史是推荐算法数据来源 | 新增 deleted_at 字段 |
| favorites | 收藏记录有分析价值(用户偏好) | 新增 deleted_at 字段 |
| follows | 关注历史有分析价值(社交图谱) | 新增 deleted_at 字段 |

**物理删除(确定无意义)**:
| 表 | 理由 |
|----|------|
| browse_history | 浏览历史已有独立的保留策略(自动清理过期数据),且数据量巨大,软删除影响性能 |
| notifications | 通知删除即无意义,无分析价值 |

**查询变更**:
- 所有查询默认过滤 `deleted_at IS NULL`
- 统一使用 GORM 软删除机制(`gorm.DeletedAt`)

**迁移文件**:
```sql
-- 059_soft_delete_unified.sql
ALTER TABLE comments ADD COLUMN deleted_at TIMESTAMP;
ALTER TABLE discussions ADD COLUMN deleted_at TIMESTAMP;
ALTER TABLE reactions ADD COLUMN deleted_at TIMESTAMP;
ALTER TABLE favorites ADD COLUMN deleted_at TIMESTAMP;
ALTER TABLE follows ADD COLUMN deleted_at TIMESTAMP;
CREATE INDEX idx_comments_deleted_at ON comments(deleted_at);
-- ... 其他表索引
```

### 验证标准

- [ ] comments、discussions、reactions、favorites、follows 表有 deleted_at 字段
- [ ] 所有查询默认过滤软删除
- [ ] 删除评论后,回复中显示"该评论已删除"占位
- [ ] `go test ./...` 无错误
- [ ] 现有功能不受影响

### 风险提示

- reactions/favorites/follows 表数据量大,软删除会增加表大小
- 需定期清理已软删除超过 90 天的数据(定时任务)
- **工作量**:中

---

## DEC-032:桌面端部署和客户端下载现在实施

**讨论日期**:2026-06-24
**关联问题**:E6(桌面端部署禁用)、E7(客户端下载禁用)
**决策**:现在实施 D-02 到 D-05,启用桌面端部署和客户端下载

### 问题核实结论

- `desktop_deploy_enabled: false`,符合 AGENTS.md 规则 10(D-02 到 D-05 完成前保持禁用)
- `download_enabled: false`,前端 `/client` 页面显示不可用状态

### 决策

在修复计划中纳入 D-02 到 D-05 的实施,完成后启用桌面端部署和客户端下载。

### 实施策略

**D-02:桌面端部署核心功能**:
- 实现 Agent 一键部署流程(内容详情页 → 点击部署 → 生成部署令牌 → 唤起 Tauri 客户端)
- 后端新增 `POST /api/v1/contents/:id/deploy-grant` 生成部署授权令牌
- 前端内容详情页显示"一键部署"按钮(当 `desktop_deploy_enabled: true` 时)

**D-03:Ed25519 签名替换 HMAC**:
- 后端动作脚本签名从 HMAC-SHA256 替换为 Ed25519
- 客户端只持有公钥,验证签名
- AGENTS.md 规则:"D-03 完成后必须替换为 Ed25519"

**D-04:客户端文件操作安全加固**:
- 确认 7 种文件操作白名单实现完整
- 路径校验在白名单目录内
- backup_file 自动触发机制验证

**D-05:部署流程端到端测试**:
- 从 Web 端发起部署 → 唤起客户端 → 下载内容 → 配置 → 启动
- 完整流程测试

**启用配置**:
```yaml
features:
  desktop_deploy_enabled: true
client:
  download_enabled: true
  download_url: "https://download.omnicraft.example/client"  # 实际下载地址
  latest_version: "1.0.0"
```

### 验证标准

- [ ] D-02 到 D-05 全部完成
- [ ] `desktop_deploy_enabled: true`
- [ ] `download_enabled: true`
- [ ] 内容详情页显示"一键部署"按钮
- [ ] `/client` 页面显示下载链接
- [ ] 部署流程端到端测试通过
- [ ] Ed25519 签名验证通过
- [ ] `go test ./...` 无错误
- [ ] `cargo test --manifest-path src-tauri/Cargo.toml` 无错误

### 风险提示

- **高风险**:涉及安全签名机制变更(HMAC → Ed25519)
- 需要完整的端到端测试
- **工作量**:大

---

## DEC-033:通知系统 SSE 实时推送

**讨论日期**:2026-06-24
**关联问题**:G5(通知实时性差)
**决策**:方案 A — 为通知增加 SSE 推送,与 DEC-007 私信 SSE 一并实现

### 问题核实结论

通知未读数轮询 30 秒,仍为轮询而非实时推送。

### 决策

为通知增加 SSE 推送,与 DEC-007(私信 SSE)一并实现,统一 SSE 基础设施。

### 实施策略

**后端**:
- 新增 `GET /api/v1/notifications/stream` SSE 端点
- 复用 DEC-007 的 SSE 基础设施
- 通知创建时,通过 SSE 推送到对应用户

**前端**:
- AuthContext 中移除 30 秒轮询,改为 SSE 监听
- SSE 连接管理(断线重连、心跳)
- 收到通知事件后更新未读数和通知列表

**与 DEC-007 的协调**:
- 统一 SSE 连接管理(一个连接处理私信 + 通知)
- 或分开连接(私信 SSE + 通知 SSE),根据实现复杂度决定

### 验证标准

- [ ] `GET /api/v1/notifications/stream` SSE 端点可用
- [ ] 通知创建后,客户端实时收到(无需等待轮询)
- [ ] SSE 断线自动重连
- [ ] 移除通知未读数轮询代码
- [ ] `go test ./...` 无错误
- [ ] `npm run build` 无错误

### 风险提示

- SSE 连接管理增加服务器资源消耗
- 需处理断线重连和消息丢失问题
- **工作量**:中

---

## 已关闭问题(核查中发现已修复)

以下 6 个 P2 问题经代码核查确认已在实际代码中修复,文档描述过时:

| 编号 | 问题 | 核查结论 | 关闭依据 |
|------|------|---------|---------|
| B1 | 推荐缓存膨胀 | 有 2h TTL,不会无限增长 | recommendation_service.go:128-137 显式设置 TTL |
| F2 | 侧边栏不可发现 | 折叠/展开/图标/tooltip 完整 | StudioSidebar.tsx:135-141 |
| F3 | ContentCard 重复 | 仅 1 个实现 | 全局搜索确认 |
| F4 | 分类 Tab 不可见 | sticky 定位,滚动时可见 | original/page.tsx:109-124 |
| G3 | 深链兼容性 | URL Scheme 已实现 | url_scheme.rs 支持 omnicraft://deploy |
| H2 | 模糊响应困惑 | 三种状态提示 + 密码强度 | forgot-password/reset-password 页面 |

**处理方式**:问题清单中标注"✅ 已修复(核查确认)",不纳入修复计划。

---

## 待讨论问题队列

**全部优先级讨论完成(P0 + P1 + P2 + P3)。**

- [x] P0:B3 — 中文全文搜索(DEC-001)
- [x] P0:H1 — Token XSS 风险(DEC-002,降级 P3)
- [x] P0:H3 — 法律文本缺失(DEC-003,搁置)
- [x] P0:C1 — 信誉分配置缺失(DEC-004,同时修复 C2/C3/C4)
- [x] P0:D1 — Studio 空存根页面(DEC-005,同时修复 D2/D3)
- [x] P0:F1 — 瀑布流分页(DEC-006)
- [x] P1:G6 — 私信 SSE 实时推送(DEC-007)
- [x] P1:E1 — 草稿系统(DEC-008,同时修复 E2)
- [x] P1:E3 — 数据导出(DEC-009)
- [x] P1:E5 — 收藏集页面(DEC-010)
- [x] P1:A1 — 设计系统冲突(DEC-011,同时修复 A2)
- [x] P1:A3 — 原创区导航文档矛盾(DEC-012,由 DEC-013 解决)
- [x] P1:A4 — UI Design.md 过时(DEC-013)
- [x] P2:B2 — DDD 完整重构(DEC-014)
- [x] P2:E4 — 数据分析基础增强(DEC-015)
- [x] P2:G1 — Tauri 离线暂缓(DEC-016,降级 P3)
- [x] P2:G2 — PWA 暂缓(DEC-017,降级 P3)
- [x] P2:H4 — 验证码/邮件配置(DEC-018)
- [x] P2:I4 — 启用队列系统(DEC-019)
- [x] P2:C5 — Agent LLM 配置(DEC-020)
- [x] P2:B1/F2/F3/F4/G3/H2 — 已修复关闭(核查确认)
- [x] P3:A5 — 审查报告归档(DEC-021)
- [x] P3:A6 — task.json 保留(DEC-022)
- [x] P3:D4 — 首页路由合并(DEC-023)
- [x] P3:C6 — 下载权限复用阈值(DEC-024)
- [x] P3:F5 — 移动端适配小红书模式(DEC-025)
- [x] P3:G4 — Tauri 账号同步暂缓(DEC-026)
- [x] P3:I1 — 推荐冷启动兴趣选择(DEC-027)
- [x] P3:I2 — 向量索引升级 HNSW(DEC-028)
- [x] P3:I3 — 热门排行配置暴露(DEC-029)
- [x] P3:J1 — 迁移验证范围更新(DEC-030)
- [x] P3:J2 — 软删除策略部分统一(DEC-031)
- [x] P3:E6/E7 — 桌面端部署现在实施(DEC-032)
- [x] P3:G5 — 通知 SSE 实时推送(DEC-033)

**下一步**:撰写修复计划,对照本文件(决策)与问题清单(问题全貌)。

---

*最后更新:2026-06-24*
*维护者:项目架构师 + 产品经理协作*
