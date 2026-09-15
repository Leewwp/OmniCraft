# 贡献指南

感谢你对 OmniCraft 的兴趣！欢迎通过 Issue 与 Pull Request 参与贡献。

## 环境要求

| 工具 | 版本 | 说明 |
|------|------|------|
| Go | 1.26+ | 后端 API 服务 |
| Node.js | 20+ | 前端（推荐 pnpm 9+ 或 npm 10+） |
| PostgreSQL | 16+ | 需 pgvector ≥ 0.7 扩展 |
| Redis | 7+ | 缓存、会话与异步队列 |
| Rust | 1.75+ | 仅 Tauri 桌面客户端需要 |

CI 使用 `.github/workflows/ci.yml` 中精确固定的工具链版本；本地满足上表最低版本即可。

## 本地启动

```bash
# 1. 克隆并启动基础设施
git clone https://github.com/Leewwp/OmniCraft.git
cd OmniCraft
docker compose up -d postgres redis

# 2. 初始化数据库
./scripts/init-db.sh

# 3. 配置环境变量
cp .env.example .env   # 按需填写本地配置

# 4. 启动后端
cd backend && go mod tidy && go run cmd/server/main.go

# 5. 启动异步 worker（评论/点赞等通知依赖；不开则通知滞留在队列）
go run cmd/worker/main.go

# 6. 启动前端
cd ../frontend && pnpm install && pnpm dev   # http://localhost:3000
```

## 提交前必须通过的验证

```bash
bash scripts/verify-project.sh          # 聚合入口：后端 test/vet/build + 前端 unit/lint/build + 文档门
bash scripts/verify-project.sh --full   # 追加 mocked Playwright 契约测试
```

后端单项：`go test ./...`、`go vet ./...`、`go build ./...`
前端单项：`npm run build`、`npm run lint`

聚合命令不替代 UI 任务的浏览器验证与截图，也不替代涉及真实外部服务的 smoke。

## Pull Request 流程

1. 从最新 `main` 切出特性分支（基线过旧的分支请先 rebase，否则 PR 不触发 CI）；
2. 提交信息用祈使句概述改动（例如 `feat(agent): add IP search tool`）；
3. PR 需要通过全部分支保护检查（`project-gate` 等），CI 全绿后以 squash 方式合并；
4. 修改 `backend/config.yaml`、`migrations/` 或 `routes.go` 的 PR 需同步运行
   `cd tools/doc-validator && go run . --fix` 并提交生成的文档修正。

## 代码规范

- **Go**：`gofmt` 格式化；所有错误必须处理，禁止裸 panic；错误响应统一
  `{ "code": "ERROR_CODE", "message": "..." }`；日志使用 `slog` 结构化输出。
- **TypeScript/React**：strict 模式，函数组件 + Tailwind CSS，禁止 `any`；
  新增 UI 字符串必须走 `next-intl`（`frontend/messages/{zh,en}.json`），禁止硬编码文案。
- **数据库**：迁移只增不改（forward-only）；GORM 参数化查询，禁止 SQL 字符串拼接。
- **安全**：不得提交真实凭证、密钥或个人敏感信息；`.env` 类文件不入库，
  仅 `.env.example` / `.env.production.example` 占位符模板可提交。

## 报告问题

- 缺陷与功能建议：先搜索既有 Issue，再按 Issue 模板新建；
- 安全漏洞：勿在公开 Issue 中提交，参见 [SECURITY.md](SECURITY.md)。
