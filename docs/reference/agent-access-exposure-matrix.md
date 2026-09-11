# Agent 接入暴露面清单（匿名端点 × 可见性规则矩阵）

> SP-16 P0（#446）交付物。本文档是全部匿名可达（optAuth）端点的**唯一对照基线**：
> P3（MCP 工具）与任何新公开面实现前，必须对照本矩阵核验返回面；新增 optAuth 端点时
> 必须同步登记本表与后端注册表测试（`backend/internal/router/optauth_registry_test.go`，
> 该测试会强制本表与 routes.go 保持一致）。
>
> 内容可见性统一口径（`repository.ApplyContentVisibilityScope`）：
> `status='published'` 且未软删、作者未封禁/未注销、所属 IP 未封禁、`is_public=true`
> （作者本人可见自己的非公开内容）。内容状态词汇表：`pending → under_review → published | banned`，
> 无独立 draft 状态；"受限/私密" 由 `is_public=false` 表达。

## 匿名端点矩阵

| 端点 | 返回数据 | 可见性机制 |
|---|---|---|
| GET /api/v1/users/:id | 公开用户投影（无 email） | 公开档案字段（role/封禁标记为治理信号） |
| GET /api/v1/users/:id/reputation | 信誉记录 | 公开档案数据 |
| GET /api/v1/users/:id/contents | 内容列表 | ListContents + ApplyContentVisibilityScope(viewer) |
| GET /api/v1/users/:id/followers | 粉丝用户投影 | 关注图公开数据 |
| GET /api/v1/users/:id/following | 关注 id 列表 | 关注图公开数据 |
| GET /api/v1/users/search | 用户搜索 | 排除封禁/注销用户；无 email |
| GET /api/v1/users/:id/discussions | 讨论列表 | discussion status=published + 父内容可见性过滤 |
| GET /api/v1/ips | IP 列表 | 仅 status=approved |
| GET /api/v1/ips/:id | IP 详情 | IP 门：approved，或创作者/admin |
| GET /api/v1/ips/:id/contents | IP 下二创列表 | IP 门 + ApplyContentVisibilityScope(viewer) |
| GET /api/v1/ips/:id/proposals | 共治提案列表 | IP 门：approved，或创作者（提案随 IP 可见性走） |
| GET /api/v1/ips/:id/proposals/:proposalId | 提案详情 | IP 门：approved，或创作者 |
| GET /api/v1/ips/:id/versions | IP 版本列表 | IP 门：approved，或创作者；快照字段不序列化 |
| GET /api/v1/ips/:id/discussions | IP 讨论列表 | IP 门 + discussion status=published |
| GET /api/v1/ips/:id/discussions/search | IP 讨论搜索 | IP 门 + discussion status=published |
| GET /api/v1/contents | 内容列表 | ApplyContentVisibilityScope(viewer) |
| GET /api/v1/contents/:id | 内容详情 | 作者/admin/持格判官可读 under_review；其余仅 published+公开+作者未封+IP 未封；来源引用（source_original/source_fanwork）仅在其本身可见时携带标题 |
| GET /api/v1/contents/:id/related-fanworks | 关联二创 | 来源走 GetVisibleContent，子项走 scope |
| GET /api/v1/contents/:id/versions | 版本谱系 | 内容可见性门（storage_type=full 的行携带全文正文） |
| GET /api/v1/contents/:id/prs | PR 列表 | 内容可见性门（message/reject_reason 为派生文本） |
| GET /api/v1/contents/:id/guide | 使用指导合并视图 | 内容可见性门（作者/admin/匿名可见）；ETag + s-maxage=300（#447） |
| GET /api/v1/contents/search | 内容搜索 | search repo 内联可见性谓词 |
| GET /api/v1/versions/:id | 版本详情 | 参与者门：作者/提案提交者/admin（F-056） |
| GET /api/v1/pr/:id | PR 详情 | 参与者门：作者/提交者/admin |
| GET /api/v1/social/comments | 内容评论 | 评论 status=published + 父内容可见性过滤 |
| GET /api/v1/social/discussions | 讨论列表 | discussion status=published + content_id 路径的父内容可见性过滤 |
| GET /api/v1/social/discussions/:id | 讨论详情 | 仅 status=published（与 /discussions/:id 同口径） |
| GET /api/v1/social/reactions | 反应计数 | 仅聚合计数 |
| GET /api/v1/collections | 收藏夹列表 | 匿名须带 owner_id；公开收藏夹 + 条目计数走 ContentVisibilitySQL |
| GET /api/v1/collections/:id | 收藏夹详情 | is_public 或属主；条目按 ContentVisibilitySQL 过滤 |
| GET /api/v1/series/:id | 系列详情 | 条目按 ContentVisibilitySQL 过滤（系列元数据设计为公开） |
| GET /api/v1/judge/exam/:category | 判官考试 | 需登录（匿名 401） |
| GET /api/v1/judge/cases/:id/verdict | 判官判决详情 | 治理透明记录（非内容派生）；见「记录在案的暴露」 |
| GET /api/v1/stats/summary | 站点统计 | 计数仅统计匿名可见内容 |
| GET /api/v1/ips/stats/category_counts | IP 类目计数 | approved IP 类目哈希（非内容派生） |
| GET /api/v1/categories | 类目表 | 类目静态数据 |
| GET /api/v1/openapi.json | OpenAPI 3.1 契约文档 | 静态文档（无实数据；#448） |
| POST /api/v1/mcp | MCP 工具调用 | 每个工具内置 ApplyContentVisibilityScope(viewer=0)；限流 mcp_per_minute（#449 暴露面复核） |
| GET /api/v1/mcp | MCP SSE 流通道 | 协议通道（无 POST 不出工具数据） |
| DELETE /api/v1/mcp | MCP 会话终止 | 协议通道 |
| GET /api/v1/tags/faceted | 标签分面 | 共现计数仅统计匿名可见内容 |
| GET /api/v1/tags/search | 标签搜索 | 全局使用计数（非按内容） |
| GET /api/v1/search/suggestions | 搜索建议 | 内联可见性谓词（status/软删/作者/IP/is_public） |
| GET /api/v1/search/trending | 趋势内容 | ApplyContentVisibilityScope(viewer) |
| GET /api/v1/discussions/:id | 讨论详情（canonical） | 仅 status=published |
| POST /api/v1/feedback | 工单提交 | 匿名写：captcha 门（无读取面） |
| POST /api/v1/feedback/attachments/presign | 工单附件预签名 | 匿名写：captcha 门、仅图片、≤20MB（无读取面） |

## 记录在案的暴露（经审计、定性为可接受或已知的）

- **判官判决详情匿名可读**（judge_id + 投票理由）：治理透明性设计，非内容派生数据；
  如需收紧应另立票裁决。
- **用户投影含 role 与 is_banned**：作为公开治理信号保留；email 等敏感字段在模型层
  即不序列化。
- **粉丝列表含封禁用户投影**：关注图公开数据；不含 email。
- **IP 门不含 admin 旁路的提案端点**：admin 治理走 /admin/* 队列；公开提案端点仅
  approved IP 的提案对匿名开放。

## 回归门

- 注册表测试：`go test ./internal/router/ -run TestOptAuthRouteRegistry` —— routes.go
  新增 optAuth 端点而未登记本表/注册表时测试变红。
- 零泄漏遍历：`go test ./internal/router/ -run TestOptAuthAnonymousSurfaceZeroLeak` ——
  五状态夹具（pending/under_review/private/软删/banned + 封禁作者）× 全匿名端点遍历，
  断言标记零外泄（正文、版本全文、PR 文本、评论、附件元数据、计数、IP 治理文本）。
