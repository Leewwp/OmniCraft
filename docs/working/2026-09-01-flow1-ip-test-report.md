# 流程 1（IP 创建与审批链）E2E 测试报告 — 阶段 A 探查

- 创建日期：2026-09-01
- **预计失效日期**: 2026-09-15
- 被测环境：云端真实验收环境（https://app.leeppp.online / https://api.leeppp.online/api/v1，后端 environment=release）
- 测试账号：创作者 demo01@leeppp.online（user_id=3「林间星光」）、管理员 admin@leeppp.online（user_id=2「万象站长」，role=admin）。密码不入本文件。
- 红线遵守：未触碰 weipei08@outlook.com（id=1）任何数据；本测试只产生带「QA探查-」前缀的实体。
- 浏览器驱动方式说明：subagent 内无法调用 browser-use 浏览器后端（`Browser is not available in subagent`），改用仓库本地 Playwright + Chromium（frontend/node_modules/playwright，真实浏览器渲染）完成全部 UI 用例；截图均来自真实页面渲染。API 用例用 curl。

## 结果一览

| 用例 | 结果 | 摘要 |
|------|------|------|
| T1.1 浏览器登录 demo01 | **PASS** | 登录成功跳首页，右上角显示头像+「林间星光」 |
| T1.2 浏览器创建 IP | **BLOCKED（UI）+ PASS（API 路径）** | 前端不存在任何「创建 IP」入口（代码与页面双向证实）；经 API+真实 OSS 上传链路创建成功（id=1，status=pending，HTTP 201） |
| T1.3 IP 详情页字段 | **PASS（名称/简介/分类）+ FAIL（封面）** | 名称、简介、分类展示正确；封面 `<Image>` 加载失败（OSS 私有 bucket → `/_next/image` 403）；页面无标签区（与代码一致：tags 既不持久化也不展示） |
| T1.4 可见性与审批链 | **PASS** | pending 不在公开列表 → GetIP 直连 200 → admin UI 待审列表可见 → 「通过」+确认弹窗 → API 200 → 公开列表/IP 库页可见。AI 审核（阿里云 Green）真实调用且 pass，但不自动 approve |
| T1.5 二创 picker 引用 | **PASS** | /studio/publish/fanwork 选「图片」类型后，「关联 IP」picker 输入完整名称出现建议并可选中（未提交，符合范围约定） |
| T1.E1 未登录 POST /ips | **PASS** | 401 `{"code":"UNAUTHORIZED","message":"authorization header required"}` |
| T1.E2 缺必填字段 | **PASS（API）/ 不适用（UI）** | 缺 name / 空 name 均 400 `VALIDATION_ERROR`；UI 侧因无创建表单无从测表单校验 |
| T1.E3 GET /ips/99999999 | **PASS** | 404 `{"code":"IP_NOT_FOUND","message":"ip not found"}`，结构化错误非 500 |
| T1.E4 无 CSRF POST | **PASS** | 403 `{"code":"CSRF_TOKEN_INVALID","message":"CSRF token missing or invalid"}` |

## 用例详情

### T1.1 浏览器登录 demo01 — PASS

- 场景：Playwright Chromium 1440x900 → `https://app.leeppp.online/login` → 填 `#email`/`#password`（选择器依据 `frontend/app/(public)/login/page.tsx:62-80`）→ 点 `button[type=submit]`。
- 期望：登录成功离开 /login，页面可见用户身份（AuthContext.login → POST /auth/login 200 → setUser）。
- 实际：URL `/login` → `/`，页面 body 含「林间星光」，右上角头像+用户名（截图视觉确认）。
- 截图：`T1.1-login-filled.png`、`T1.1-login-success.png`。

### T1.2 创建 IP「QA探查-星语森林」— BLOCKED(UI) + PASS(API)

**UI 探查（BLOCKED——功能缺失，非环境问题）**：
- 代码依据：全前端仓库 grep `POST /api/v1/ips` 的调用为零（`frontend/components`、`frontend/app`、`frontend/lib` 均无）；zh.json `ip` 命名空间无任何创建相关 key；后端 `POST /ips`（`backend/internal/router/routes.go:84`）为孤立的 API。
- 页面证实（demo01 已登录）：
  - `/ips` 浏览页全文无「创建/新建/提交 IP」字样；
  - 顶栏「+ 发布」→ `/studio/publish/original`（发布内容，非创建 IP）；
  - `/studio` 侧边栏 16 个链接全部枚举，无 IP 创建项。
- 截图：`T1.2-ips-browse-page.png`、`T1.2-studio-overview.png`。
- 结论：创作者在 UI 上无法创建 IP，属功能缺口，建议登记 issue。

**API 路径（PASS，封面走真实 OSS 上传链路）**：
1. `POST /api/v1/contents/oss-token`（file_type=image, mime=image/jpeg, size=88626）→ 200，`upload_url` + `oss_key=uploads/3/image/2026/08/31/1788203236_4d680bc3e00ac1d7.jpg`（上传链路依据 `backend/internal/handler/content.go:99` GenerateOSSToken、`backend/internal/service/oss_service.go:34-46`，与 `frontend/app/(protected)/settings/page.tsx:88-98` 头像上传同链路）。
2. `PUT` 文件至预签名 URL（编码/解码两种路径均 200）。
3. `POST /api/v1/ips`（Bearer demo01）→ **201**，响应：`{"ip":{"id":1,"name":"QA探查-星语森林","slug":"qa",...,"status":"pending","creator_id":3}}`。
- 期望依据：`backend/internal/handler/ip.go:88`（201 + `{"ip":...}`）、`backend/internal/service/ip_service.go:81`（Status 初始 "pending"）。
- 创建的 IP **id=1**（线上 ips 表此前为空——公开列表创建前 total=0，id 1-5 均 404）。
- 后端日志：`19:08:07 POST /api/v1/ips status=201 duration_ms=66`。
- 附带观察：① slug=`qa`——`generateSlug` 正则 `[^a-z0-9-]`（ip_service.go:295,306）剔除全部中文，中文名 IP 的 slug 语义信息丢失；② 请求带 `tags:["QA","森林","星空"]` 但响应无 tags 字段（见「附带发现」1）。

### T1.3 IP 详情页 — PASS(名称/简介/分类) + FAIL(封面)

- 场景：浏览器（未登录态亦可）访问 `https://app.leeppp.online/ip/1`（pending 状态时访问）。
- 期望依据：`frontend/components/ip/IPDetail.tsx:48-59`（封面/名称/分类行/简介），GetIP 不筛状态（`backend/internal/handler/ip.go:91-109`、service 139-162）。
- 实际：
  - URL 保持 `/ip/1`，`<title>QA探查-星语森林 — 万象工坊`，名称、简介全文正确；
  - 分类行「分类：图片」（category=`image` → `home.image` → zh.json:185「图片」）正确；
  - **封面 FAIL**：`<img src="/_next/image?url=https://omnicraft.oss-cn-guangzhou.aliyuncs.com/uploads/3/image/...jpg">` naturalWidth=0；直接请求 `/_next/image` 返回 **403** `"url parameter is valid but upstream response is invalid"`；OSS 直读该对象 403 `AccessDenied ... bucket acl`。即：上传成功且对象存在，但 bucket 私有 → Next.js 图片代理服务端拉取被 OSS 拒绝 → 封面永不显示。该失败不经 Go 后端（后端日志无 4xx/5xx 相关记录），属 OSS bucket ACL / 图片访问链路配置问题。
  - 标签：详情页无标签区域——与代码一致（IPDetail.tsx 无 tags 渲染；模型 `model.IP` 无 Tags 字段，ip.go:5-17）。
- 截图：`T1.3-ip-detail-pending.png`。

### T1.4 可见性与审批链 — PASS

状态流转记录（全部有实证）：

| 步骤 | 期望（代码依据） | 实际 |
|------|------------------|------|
| 创建后 status | `pending`（ip_service.go:81） | 201 响应 status=`pending` ✓ |
| pending 公开列表可见性 | 不可见——ListIPs 无 Status 时强制 `status='approved'`（ip_repo.go:69-73） | `GET /api/v1/ips` total=0 ✓ |
| pending 详情直连 | 200（GetIP 不筛状态） | `GET /ips/1` 200 ✓ |
| AI 审核 | `submitIPForAIReview`（ip_service.go:89） | 后端日志：创建 1s 后真实阿里云 Green 调用并落库 `INSERT INTO ai_review_records ... ('ip',1,'aliyun','pass',...)`，结果 pass；**pass 不改变 pending**（需 admin 人工审批） |
| admin 待审列表 | `Status:"pending"` 过滤（admin.go:130-143） | admin API `GET /admin/ips` total=1，含 id=1/creator_id=3 ✓；浏览器 `/admin/ips` 表格显示该 IP（琥珀 pending 徽标）✓ |
| admin UI approve | `POST /admin/ips/:id/approve` → 200 `{"message":"ip approved"}`（admin.go:145-169） | 浏览器点「通过」（aria-label 依据 admin/ips/page.tsx:136，文案 zh.json:798「通过」）→ ConfirmModal「确认通过」（zh.json:804）→ API 200（浏览器抓包 + 后端日志 `19:15:44 POST /api/v1/admin/ips/:id/approve status=200`）→ 列表移除 ✓ |
| approve 后公开可见 | status=approved 进列表 | `GET /ips/1` status=`approved`；`GET /ips` total=1 含该 IP；浏览器 `/ips` 页出现「QA探查-星语森林」卡片 ✓ |

- 截图：`T1.4-admin-pending-list.png`、`T1.4-admin-approve-confirm-modal.png`、`T1.4-admin-after-approve.png`、`T1.4-public-ips-visible-after-approve.png`。

### T1.5 二创 picker 引用 — PASS

- 场景：demo01 登录 → `/studio/publish/fanwork`（两步向导，先选内容类型，`frontend/app/(protected)/studio/publish/fanwork/page.tsx:81-90`）→ 点「图片」→ PublishForm 出现「关联 IP」picker（aria-label=`关联 IP`，zh.json:1823；组件 `frontend/components/studio/IPPicker.tsx:43` 调 `GET /api/v1/ips?q=`）。
- 期望：新 IP 已 approved → 应出现在建议且可选；注意 picker 依赖公开列表（approved-only）+ Postgres `simple` 分词全文检索（ip_repo.go:80），完整名称应命中。
- 实际：输入「QA探查-星语森林」→ 网络 `GET /api/v1/ips?q=QA探查-星语森林` 200 → 建议按钮恰好 1 个 → 点击后输入框值锁定为「QA探查-星语森林」（IPPicker.handleSelect，IPPicker.tsx:72-76）。未提交发布（流程 2 范围）。
- 附带观察：部分词「星语」搜索 0 结果（`to_tsvector('simple')` 不切中文词，整串匹配才命中）——已知检索限制，非本用例失败。
- 截图：`T1.5-step1-content-type-grid.png`、`T1.5-fanwork-publish-form.png`、`T1.5-picker-suggestions.png`、`T1.5-picker-selected.png`。

### 边界用例（curl）

| 用例 | 请求 | 期望（代码依据） | 实际（状态+响应体） | 结果 |
|------|------|------------------|---------------------|------|
| T1.E1 | `POST /api/v1/ips`（无 Authorization，带合法 CSRF） | 401 + `{"code","message"}`（auth.go:27-32） | 401 `{"code":"UNAUTHORIZED","message":"authorization header required"}` | PASS |
| T1.E2a | `POST /ips`（已登录，body 无 name） | 400（ip.go:77-80 → response.go:36-38 `VALIDATION_ERROR`） | 400 `{"code":"VALIDATION_ERROR","message":"invalid request parameters"}` | PASS |
| T1.E2b | `POST /ips`（已登录，`{"name":""}`，binding min=1，ip_service.go:56） | 400 同上 | 400 同上 | PASS |
| T1.E3 | `GET /api/v1/ips/99999999` | 404 `IP_NOT_FOUND`（ip.go:100-102） | 404 `{"code":"IP_NOT_FOUND","message":"ip not found"}` | PASS |
| T1.E4 | `POST /auth/login`（带 cookie 不带 `X-CSRF-Token`） | 403（csrf.go:55-63） | 403 `{"code":"CSRF_TOKEN_INVALID","message":"CSRF token missing or invalid"}` | PASS |

后端日志佐证（`ssh omnicraft-server docker logs omnicraft-backend-1`）：`19:06:29 POST /api/v1/ips 401`、`19:06:41 POST /api/v1/ips 400`（×2）、`19:08:07 POST /api/v1/ips 201`、`19:15:44 POST /api/v1/admin/ips/:id/approve 200`；测试窗口内无 `level:ERROR/WARN`、无 5xx。

## FAIL / BLOCKED 汇总

1. **T1.2 UI 创建入口 BLOCKED**：前端无「创建 IP」功能（零代码路径 + 页面实证）。后端 API 完备且验证可用。影响：普通创作者无法通过 UI 建立 IP，二创引用只能依赖存量/种子 IP。
2. **T1.3 封面展示 FAIL**：OSS bucket 私有（匿名 GET 403 AccessDenied）→ Next `/_next/image` 代理 403 → 详情页封面不渲染。上传链路本身完好（presign PUT 200、对象存在）。修复方向（供参考，未实施）：bucket/路径级公开读、或签名 URL 输出、或后端图片代理路由。

## 附带发现（不判 FAIL，供登记）

1. **IP tags 双端缺口**：`CreateIPInput.Tags`（ip_service.go:60）被接收但从不持久化（无 IPTag 写入路径，grep 全后端仅模型定义 ip.go:34-39）；前端也无展示。API 契约上「接受但静默丢弃」。
2. **中文 slug 失真**：`generateSlug`（ip_service.go:295-317）剔除所有非 `[a-z0-9-]`，中文名 IP slug 仅剩拉丁残留（本例 `qa`）；同名并发时 `-<creatorID>` 后缀仍可能撞车（仅按 slug 去重一次）。
3. **IP 搜索 simple 分词限制**：`GET /ips?q=` 用 `to_tsvector('simple', name)`（ip_repo.go:80），中文仅整串命中，部分词无结果（picker 输入「星语」= 0 结果）。
4. **登录前 refresh 序列**：未登录首页加载时 `POST /auth/refresh` 固定产生一次 403（CSRF 未建立）再 401（无 refresh_token）——符合 api.ts:74-119 的 CSRF 初始化重试设计，但会在浏览器 console 持续产生红色资源错误，观测体验欠佳。
5. **线上种子数据为空**：测试前 ips 表 0 行（公开列表 total=0、id1-5 404），IP 库/发现页空态；「QA探查-星语森林」成为首个 IP。

## 创建的实体清单（供清理登记）

| 类型 | id | 名称/标识 | 位置 |
|------|----|-----------|------|
| IP | 1 | QA探查-星语森林（slug=qa，status=approved，creator=3） | 线上 DB `ips` |
| OSS 对象 | — | `uploads/3/image/2026/08/31/1788203236_4d680bc3e00ac1d7.jpg`（88,626B JPEG，源=frontend/public/seed-media/real/ips/ip-witcher.jpg） | omnicraft bucket（广州） |
| ai_review_records | 1 条 | target_type=ip, target_id=1, provider=aliyun, result=pass | 线上 DB（随 IP 删除清理） |

未创建任何内容/评论/讨论；weipei08@outlook.com 数据未触碰；未做任何写操作到 weipei08 名下。

## 截图目录

`/Users/pp/Desktop/file/code/project/OmniCraft/screenshots/flow1-ip-creation-20260901/`（15 张，文件名=用例编号；截图内无密码明文——登录截图仅显示已填邮箱，密码字段为掩码；如需更严格可将 T1.1-login-filled.png 删除）
