# OmniCraft 万象工坊 - Agent 工作指南

## 项目概览

全民创意分享平台，技术栈：Next.js（前端）+ Go/Gin（后端）+ PostgreSQL + Redis + 阿里云 OSS + Tauri（PC 客户端）

详细架构：`architecture.md` | 任务列表：`task.json`

---

## MANDATORY: Agent 工作流程

每次新 Agent 会话必须严格按以下步骤执行：

### Step 1: 读取任务列表

```bash
cat task.json
```

选择任务标准（按优先级）：
1. 选择 `passes: false` 的任务
2. 优先选择基础设施任务（id 较小的任务通常是依赖），确保依赖已完成
3. 选择 ID 最小的未完成任务

### Step 2: 初始化开发环境

**后端**（首次或新依赖）：
```bash
cd backend && go mod tidy
```

**前端**（首次或新依赖）：
```bash
cd frontend && npm install
```

**启动服务**（开发时）：
```bash
docker compose up -d postgres redis
cd backend && go run cmd/server/main.go &
cd frontend && npm run dev &
```

**DO NOT skip this step.** 确保相关服务运行后再实现功能。

### Step 3: 实现任务

- 仔细阅读 `task.json` 中该任务的 `description` 和 `steps`
- 参考 `architecture.md` 中对应模块的设计
- 严格遵循现有代码规范和目录结构

**代码规范**：
- Go：`gofmt` 格式化，错误必须处理，禁止裸 panic，统一错误返回格式 `{ "code": "ERROR_CODE", "message": "..." }`
- Next.js：TypeScript strict mode，组件使用 function 声明，Tailwind CSS 样式，禁止 any 类型
- 数据库操作：所有查询使用 GORM 参数化，禁止 SQL 字符串拼接

### Step 4: 测试（强制要求）

**UI 相关任务**（新建页面、修改核心交互）：
- **必须使用 MCP Playwright 在浏览器中测试**
- 验证页面正常加载和渲染
- 验证核心交互功能（表单提交、按钮点击）
- 截图确认 UI 正确

**后端 API 任务**：
- 使用 curl 或 Playwright 调用接口验证
- 验证正常路径和错误路径

**所有任务必须通过**：
- [ ] Go: `go build ./...` 无错误
- [ ] Go: `go vet ./...` 无警告
- [ ] Next.js: `npm run build` 无错误
- [ ] Next.js: `npm run lint` 无错误
- [ ] 功能在浏览器/接口测试中验证通过

### Step 5: 更新 progress.txt

```
## [YYYY-MM-DD] - Task [ID]: [任务标题]

### What was done:
- [具体修改的文件和改动]

### Testing:
- [测试方式和结果]

### Notes:
- [给未来 Agent 的重要说明]
```

### Step 6: 提交（一个 commit 包含所有改动）

**IMPORTANT: task.json + progress.txt + 所有代码改动必须在同一个 commit**

```bash
# 1. 更新 task.json 中该任务的 passes: false → passes: true
# 2. 更新 progress.txt
# 3. 一次性提交
git add .
git commit -m "Task [ID]: [任务标题] - completed"
```

**规则**：
- 只有所有 steps 验证通过才标记 `passes: true`
- 永远不要删除或修改任务描述
- 永远不要从列表中移除任务

---

## ⚠️ 阻塞处理

**以下情况必须停止任务，输出阻塞信息，等待人工介入**：

1. **缺少外部服务配置**：阿里云 AccessKey、OSS Bucket 未创建、Redis 连接失败
2. **需要真实密钥**：内容安全 API、OSS 直传凭证
3. **数据库迁移冲突**：迁移文件执行失败

**阻塞时禁止**：
- ❌ 提交 git commit
- ❌ 将 task.json 的 passes 设为 true
- ❌ 假装任务已完成

**阻塞时必须**：
- ✅ 在 progress.txt 记录当前进度和阻塞原因
- ✅ 输出清晰说明（格式如下）

```
🚫 任务阻塞 - 需要人工介入

**当前任务**: Task [ID] - [标题]

**已完成的工作**:
- [列出已完成内容]

**阻塞原因**:
- [具体原因]

**需要人工操作**:
1. [步骤 1]
2. [步骤 2]

**解除阻塞后**: 运行 [命令] 继续
```

---

## 项目结构

```
code/
├── architecture.md          # 技术架构设计（必读）
├── task.json                # 任务列表（工作来源）
├── CLAUDE.md                # 本文件
├── progress.txt             # 进度日志
├── backend/                 # Go 后端
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── handler/
│   │   ├── service/
│   │   ├── repository/
│   │   ├── model/
│   │   ├── middleware/
│   │   └── pkg/
│   ├── migrations/
│   ├── config.yaml
│   └── docker-compose.yml
├── frontend/                # Next.js 前端
│   ├── app/
│   ├── components/
│   └── lib/
├── tauri-client/            # Tauri PC 客户端
│   ├── src-tauri/
│   └── src/
└── k8s/                     # K8s 配置（P2 预留）
```

## 常用命令

```bash
# 后端
cd backend
go run cmd/server/main.go          # 启动开发服务器
go build ./...                     # 编译检查
go vet ./...                       # 静态检查
go test ./...                      # 运行测试

# 数据库迁移
psql $DB_DSN -f migrations/001_users.sql

# 前端
cd frontend
npm run dev                        # 启动开发服务器（http://localhost:3000）
npm run build                      # 生产构建
npm run lint                       # Lint 检查

# Tauri 客户端
cd tauri-client
npm run tauri dev                  # 开发模式
npm run tauri build                # 构建安装包

# Docker
docker compose up -d               # 启动所有服务
docker compose logs -f backend     # 查看后端日志
```

---

## 关键业务规则（实现时必须遵守）

### 信誉分
- 初始值：10 分
- 低于 3 分：禁止发布内容、评论、众裁、协同、点赞点踩（仅可浏览）
- 黄赌毒内容：直接永久封禁用户，不走信誉分机制

### 文件上传限制（从 config.yaml 读取，不要硬编码）
- 视频：≤ 300MB，时长 ≤ 180 秒
- 图片：≤ 20MB
- 文本：≤ 10MB
- Mod 包：≤ 500MB

### 赛博判官
- 题库不存在时：该类型内容不开放众裁
- 考核通过线：≥ 80% 正确率
- 错误率撤权：最近 N 次（N > 10）判定中错误率 > 50%，撤权 + 扣 1 信誉分

### 支付模块
- MVP 阶段：`features.payment_enabled: false`，所有支付相关 UI 不渲染，接口返回 503
- 不要删除支付相关代码，仅通过 feature flag 控制

---

## Key Rules

1. **One task per session** — 专注完成一个任务
2. **Test before marking complete** — 所有 steps 验证通过才标记完成
3. **Browser testing for UI** — 新建或大幅修改页面必须浏览器测试
4. **Document in progress.txt** — 帮助后续 Agent 理解工作内容
5. **One commit per task** — 代码 + progress.txt + task.json 同一个 commit
6. **Never remove tasks** — 只改 passes: false → true，不删任务
7. **Stop if blocked** — 需要人工介入时输出阻塞信息并停止
8. **Read config, not hardcode** — 所有限制从 config.yaml 读取
9. **Clarify before coding** — 明确假设与疑问，不擅自解读需求、不隐藏困惑
10. **Simplicity first** — 仅实现需求功能，拒绝过度设计与冗余代码
11. **Surgical changes only** — 精准修改必要代码，匹配现有风格不随意重构
12. **Clean up self-generated debris** — 仅清理自身修改产生的冗余引用 / 变量
13. **Define success criteria** — 明确验证标准，分步执行并完成校验