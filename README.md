# OmniCraft 万象工坊

全民创意分享平台 —— 以 IP 二创内容聚合为核心，Agent 自动化为增值能力，GitHub 式 PR 协同为社区护城河。

技术栈：**Next.js** + **Go/Gin** + **PostgreSQL** + **Redis** + **阿里云 OSS** + **Tauri**

---

## 目录

- [本地开发](#本地开发)
- [Docker Compose 部署](#docker-compose-部署)
- [环境变量说明](#环境变量说明)
- [数据库初始化](#数据库初始化)
- [数据库备份](#数据库备份)
- [项目结构](#项目结构)
- [API 文档](#api-文档)
- [Tauri 客户端](#tauri-客户端)

---

## 本地开发

### 前提条件

| 工具 | 版本要求 | 说明 |
|------|---------|------|
| Go | 1.22+ | 后端 API 服务 |
| Node.js | 20+ | 前端 Next.js |
| pnpm | 9+ (或 npm 10+) | 前端包管理 |
| PostgreSQL | 16+ | 需 pgvector ≥ 0.7 |
| Redis | 7+ | 缓存与会话 |
| Rust | 1.75+ | 仅 Tauri 客户端需要 |

### 1. 克隆仓库

```bash
git clone <repo-url>
cd OmniCraft
```

### 2. 启动基础设施

```bash
# 启动 PostgreSQL + Redis（Docker）
docker compose up -d postgres redis
```

### 3. 初始化数据库

```bash
chmod +x scripts/init-db.sh
./scripts/init-db.sh
```

或者手动运行迁移：

```bash
for f in backend/migrations/*.sql; do
  psql -h localhost -U omnicraft -d omnicraft -f "$f"
done
```

### 4. 配置环境变量

```bash
cp .env.example .env
# 编辑 .env，填入本地开发配置
```

### 5. 启动后端

```bash
cd backend
go mod tidy
go run cmd/server/main.go
# API 运行在 http://localhost:8080
```

### 6. 启动前端

```bash
cd frontend
pnpm install        # 或 npm install
pnpm dev            # 或 npm run dev
# 前端运行在 http://localhost:3000
```

### 7. 项目验证

项目统一验证入口会在任一子命令失败时立即停止，并向调用方返回非零退出码。

```powershell
# Default：日常确定性工程门
powershell -ExecutionPolicy Bypass -File scripts/verify-project.ps1

# Full：Default + mocked Playwright contracts
powershell -ExecutionPolicy Bypass -File scripts/verify-project.ps1 -Full

# Release：Default + 完整 Playwright E2E
powershell -ExecutionPolicy Bypass -File scripts/verify-project.ps1 -Release
```

| 层级 | 覆盖范围 | 前置条件 |
|------|----------|----------|
| `default` | 后端 `test/vet/build`、前端 `unit/lint/build`、doc-validator tests 与严格 `release` profile | Go、Node.js 和已安装的锁定依赖 |
| `full` | `default` + mocked Playwright contract suite | Playwright 浏览器、PostgreSQL、Redis 和可启动的本地前后端配置 |
| `release` | `default` + desktop/mobile/mocked/cross-stack 完整 Playwright suite | 发布候选配置、Playwright 浏览器、PostgreSQL、Redis、测试数据及计划要求的外部服务 |

`-Full` 与 `-Release` 是互斥层级。Tauri 是可叠加维度；修改桌面端时增加 `-Tauri`，也可与 `-Full` 或 `-Release` 组合：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-project.ps1 -Tauri
```

归档链接债务保持可见，但不阻塞当前发布真相：

```powershell
cd tools/doc-validator
go run . --check --profile archive
```

聚合命令不能替代任务要求的浏览器截图、真实外部服务 smoke、Tauri 安装包验证或人工发布证据。

---

## Docker Compose 部署

### 快速启动（全部服务）

```bash
# 1. 配置环境变量
cp .env.example .env
# 编辑 .env，填入生产环境配置

# 2. 构建并启动
docker compose up -d --build

# 3. 查看状态
docker compose ps
docker compose logs -f
```

### 服务清单

| 服务 | 端口 | 说明 |
|------|------|------|
| nginx | 80, 443 | 反向代理 + SSL 终止 |
| frontend | 3000 | Next.js SSR |
| backend | 8080 | Go API |
| postgres | 5432 | PostgreSQL 16 (pgvector) |
| pgbouncer | 6432 | 数据库连接池（宿主机端口 6432 → 容器内 5432） |
| redis | 6379 | Redis 7 |

### 本地地址约定

| 调用方 | 地址 | 用途 |
|------|------|------|
| 浏览器访问前端 | http://localhost:3000 | 手动浏览与浏览器端请求 |
| 浏览器访问后端 | http://localhost:8080 | 浏览器端 API 请求 |
| Docker 前端 SSR | http://backend:8080 | Next.js 服务端渲染请求 |
| Docker 后端 | 容器内 :8080 | 宿主机映射为 8080 |

NEXT_PUBLIC_API_URL 面向浏览器并在前端构建阶段注入；INTERNAL_API_URL 只面向 Next.js 服务端运行时。Docker Compose 本地配置使用 NEXT_PUBLIC_API_URL=http://localhost:8080 和 INTERNAL_API_URL=http://backend:8080：前者供宿主机浏览器访问，后者供容器内 SSR 访问。不要把 backend 作为 NEXT_PUBLIC_API_URL，否则浏览器无法解析 Compose 服务名。

本地 Compose 为了便于调试会映射 3000（前端）和 8080（API）两个宿主机端口。上线时不要求用户访问两个端口；生产 Compose 只对外发布 Nginx 的 80/443，前端与 API 由域名或反向代理统一暴露。由于 NEXT_PUBLIC_API_URL 在前端构建阶段写入浏览器 bundle，修改它后必须重新构建前端镜像。

### 首次部署步骤

1. **配置 SSL 证书**（Let's Encrypt）：

```bash
# 安装 certbot
apt-get install certbot

# 生成证书（standalone 模式）
certbot certonly --standalone -d your-domain.com

# 或使用 Docker certbot
docker run -it --rm -v /etc/letsencrypt:/etc/letsencrypt \
  -v /var/lib/letsencrypt:/var/lib/letsencrypt \
  -p 80:80 certbot/certbot certonly --standalone -d your-domain.com
```

2. **更新 nginx.conf**：将 `omnicraft.example.com` 替换为实际域名

3. **数据库初始化**：

```bash
docker compose exec postgres psql -U omnicraft -d omnicraft -c "SELECT 1;"
# 迁移文件在容器启动时通过 /docker-entrypoint-initdb.d 自动执行
```

4. **启动服务**：

```bash
docker compose up -d
```

### 生产环境检查清单

- [ ] `.env` 中所有占位符已替换为真实值
- [ ] SSL 证书已配置（`/etc/letsencrypt/live/`）
- [ ] `nginx.conf` 中 `server_name` 已设为实际域名
- [ ] JWT_SECRET 已生成强随机密钥
- [ ] 阿里云 OSS Bucket 已创建并配置访问权限
- [ ] 阿里云内容安全（Green）服务已开通
- [ ] 数据库定期备份已配置（cron + `scripts/backup-db.sh`）
- [ ] 防火墙仅开放 80/443 端口

---

## 环境变量说明

所有环境变量定义在 `.env.example`（本地开发）和 `.env.production`（生产部署）中。

### 必需变量

| 变量 | 说明 | 示例 |
|------|------|------|
| `DB_DSN` | PostgreSQL 连接字符串 | `host=localhost port=5432 user=omnicraft password=... dbname=omnicraft sslmode=disable` |
| `REDIS_ADDR` | Redis 地址 | `localhost:6379` |
| `JWT_SECRET` | JWT 签名密钥（生成: `openssl rand -base64 64`） | — |
| `ALIYUN_ACCESS_KEY_ID` | 阿里云 AccessKey（OSS + 内容安全共用） | — |
| `ALIYUN_ACCESS_KEY_SECRET` | 阿里云 AccessKey Secret | — |
| `OSS_ENDPOINT` | OSS 服务端点 | `https://oss-cn-hangzhou.aliyuncs.com` |
| `OSS_BUCKET_NAME` | OSS Bucket 名称 | `omnicraft-prod` |
| `GREEN_ACCESS_KEY_ID` | 内容安全 AccessKey（可与 OSS 共用） | — |
| `GREEN_ACCESS_KEY_SECRET` | 内容安全 AccessKey Secret | — |

### 可选变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `DB_READ_DSN` | 读从库连接字符串（P1 阶段） | 留空走主库 |
| `REDIS_PASSWORD` | Redis 密码 | 留空无密码 |
| `OSS_CDN_DOMAIN` | OSS CDN 加速域名 | 留空直连 OSS |
| `AGENT_LLM_API_KEY` | LLM API Key（DeepSeek / 通义千问） | — |
| `AGENT_LLM_API_BASE` | LLM API 地址 | `https://api.deepseek.com` |
| `AGENT_LLM_MODEL` | LLM 模型名称 | `deepseek-chat` |
| `AGENT_HMAC_SECRET` | 已禁用 Desktop 原型的兼容构建变量；不得用于生产发布，D-03 后由 Ed25519 配置替代 | — |
| `GREEN_CALLBACK_URL` | 内容安全审核回调地址 | — |
| `FRONTEND_URL` | 前端 URL（用于 CORS/OAuth） | `http://localhost:3000` |
| `INTERNAL_API_URL` | Next.js SSR 访问后端的容器内地址；本地进程可留空 | `http://backend:8080`（Docker） |

---

## 数据库初始化

```bash
# 确保 PostgreSQL 运行中，然后执行：
./scripts/init-db.sh

# 自定义数据库连接：
DB_HOST=prod-db.example.com DB_PASSWORD=secret ./scripts/init-db.sh
```

迁移文件位于 `backend/migrations/`，按编号顺序执行。Docker Compose 会在 postgres 容器首次启动时自动执行迁移。

### 收藏集与旧收藏对账

迁移 `058_create_collections.sql` 后，在保留旧 `favorites` 双写兼容期间运行：

```bash
cd backend
DB_DSN="host=localhost port=5432 user=omnicraft password=... dbname=omnicraft sslmode=disable" \
  go run ./cmd/collection-reconcile
```

命令默认只读，按用户与内容分区输出缺失默认集、双向缺失项和重复逻辑项；零漂移退出 `0`，存在漂移退出 `1`，参数、连接或执行错误退出 `2`。确认报告后可执行只增不删的幂等修复：

```bash
# 1. 先停止所有会写入 favorites / collection_items 的后端实例或进入等效写停维护窗口
# 2. 在同一 DB_DSN 上执行修复；显式进程环境变量优先于工作目录中的 .env
go run ./cmd/collection-reconcile --apply --maintenance-window-confirmed
```

`--apply` 缺少 `--maintenance-window-confirmed` 时会在任何数据库写入前退出 `2`。该确认表示所有应用写入者已停止；仅限制流量但仍允许收藏变更不满足条件。修复结束并再次只读检查为零漂移后，方可恢复后端实例。

旧 `favorites` 写路径只能在以下条件全部满足后，由单独的前向清理计划和迁移移除：

1. 所有受支持的前端和客户端构建均不再调用旧收藏变更接口；
2. 对账命令连续七次每日检查均报告零漂移；
3. 可回滚版本不再依赖旧表；
4. 推荐系统仅从 `collection_items` 读取时相关测试仍通过；
5. 删除由独立、可审查的前向清理计划与迁移执行。

---

## 数据库备份

```bash
# 手动备份
./scripts/backup-db.sh

# 保留最近 7 个备份
./scripts/backup-db.sh --retain 7

# 自定义备份目录
BACKUP_DIR=/mnt/backups ./scripts/backup-db.sh
```

### 配置定时备份（cron）

```bash
# 编辑 crontab
crontab -e

# 添加：每天凌晨 2 点备份，保留 30 天
0 2 * * * /path/to/OmniCraft/scripts/backup-db.sh >> /var/log/omnicraft-backup.log 2>&1
```

---

## 项目结构

```
OmniCraft/
├── README.md                # 本文件
├── architecture.md          # 技术架构设计
├── task.json                # 任务列表
├── CLAUDE.md                # Agent 工作指南
├── progress.txt             # 开发进度日志
├── .env.example             # 环境变量模板（开发）
├── .env.production          # 环境变量模板（生产）
├── docker-compose.yml       # Docker Compose 编排
├── nginx/
│   └── nginx.conf           # Nginx 反向代理配置
├── scripts/
│   ├── init-db.sh           # 数据库初始化脚本
│   └── backup-db.sh         # 数据库备份脚本
├── backend/                 # Go 后端
│   ├── cmd/server/main.go
│   ├── config/
│   ├── internal/
│   │   ├── handler/         # HTTP 处理器
│   │   ├── service/         # 业务逻辑
│   │   ├── repository/      # 数据访问
│   │   ├── model/           # GORM 模型
│   │   ├── middleware/      # 中间件
│   │   └── pkg/             # 工具包
│   ├── migrations/          # SQL 迁移文件
│   ├── config.yaml          # 应用配置
│   └── Dockerfile
├── frontend/                # Next.js 前端
│   ├── app/                 # App Router 页面
│   ├── components/          # React 组件
│   ├── lib/                 # 工具库
│   │   └── api.ts           # API 请求封装
│   └── Dockerfile
├── tauri-client/            # Tauri PC 客户端
│   ├── src-tauri/
│   └── src/
├── k8s/                     # K8s 配置（P2 预留）
└── design/
    ├── design-system.md     # 设计系统（色彩/字体/间距，唯一设计权威）
    ├── ui-spec.md           # UI 规格书（页面和组件规格）
    ├── ui-design-prompt.md  # UI 设计生成提示词
    └── doc-review-prompt.md # 文档校验提示词
```

---

## API 文档

```bash
# 启动后端后，访问健康检查
curl http://localhost:8080/healthz

# 完整 API 清单见 architecture.md §3.2
```

主要 API 路径：
- `/api/v1/auth/*` — 用户认证
- `/api/v1/contents/*` — 内容管理
- `/api/v1/ips/*` — IP 管理
- `/api/v1/social/*` — 社交互动
- `/api/v1/judge/*` — 赛博判官
- `/api/v1/admin/*` — 管理员后台

---

## Tauri 客户端

```bash
cd tauri-client

# 安装依赖
pnpm install

# 开发模式
pnpm tauri dev

# 生产构建
pnpm tauri build
```

客户端通过 `omnicraft://` URL Scheme 与 Web 前端联动。当前仓库仅完成了不安全原型的关闭（D-01）；HMAC 验签和 WebView 直接文件命令仍属于禁止发布的旧实现。`features.desktop_deploy_enabled` 必须保持 `false`，直至 D-02～D-05 与 R-02 完成短时单次 grant、Ed25519 canonical script、严格 Rust schema/路径边界、原生确认和端到端安全验证。

Web/桌面 Agent 的产品边界见 `docs/superpowers/specs/2026-07-16-omnicraft-dual-surface-agent-productization-design.md`。Web Agent 产品化计划见 `docs/superpowers/plans/2026-07-16-omnicraft-web-agent-productization.md`。
