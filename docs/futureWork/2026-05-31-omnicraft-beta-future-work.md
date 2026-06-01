# OmniCraft 双轨 Beta 后续工作清单

> 日期：2026-05-31  
> 用途：给维护者阅读的后续工作摘要。下次继续开发时，先读本文，再读 `docs/superpowers/plans/2026-05-30-omnicraft-dual-track-beta-roadmap.md`。

## 1. 今天已经确定的事项

公开 Web Beta 使用以下正式地址：

```text
Web: https://app.leeppp.online
API: https://api.leeppp.online
```

账号验证使用：

```text
阿里云验证码 2.0
```

首轮 Web Beta 不开放桌面端一键部署：

```text
features.desktop_deploy_enabled=false
client.download_enabled=false
```

桌面端仍然保留为后续路线。首轮 Web Beta 只需要完成 `D-01`，移除不安全的旧部署入口。`D-02` 至 `D-05` 和 `R-02` 暂缓。

用户协议和隐私政策尚未准备。它们不会阻塞当前基础开发，但会阻塞 `V-02`、`V-04` 和最终验收 `R-01`。

## 2. 当前开发进度

`F-01` 和 `F-02` 已经提交：

```text
846ddc4 Beta F-01: capture public beta baseline - completed
02ce10b Beta F-02: enforce fail-closed interaction eligibility
```

`F-01` 的路线图状态已经补记为完成。`F-02` 的实现已经进入 `main`，但路线图仍然保留为未勾选状态。这是有意保留的安全闸门：`F-02` 提交发生在本次半自动协调脚本建立之前，尚未留下规格审查和代码质量审查记录。脚本会阻止直接开始依赖它的 `F-03`。

当前主工作区未提交的内容是本次新增的协调脚本、维护者决策同步和未来工作文档。它们应作为独立的文档与工具改动提交，不要混入业务任务 commit。

不要执行 `git add .`、自动 stash 或 reset。提交前使用 `git status --short` 和 `git diff --stat` 精确确认文件范围。

## 3. 下次开始开发时的顺序

### 第一步：复核 `F-02` 并解除安全闸门

1. 阅读 `AGENTS.md`。
2. 检查提交 `02ce10b` 的文件范围和测试证据。
3. 运行 `F-02` 的后端 focused tests 和仓库级 gate。
4. 完成规格符合性审查。
5. 完成代码质量审查。
6. 处理所有 `DONE_WITH_CONCERNS`。
7. 确认没有遗留问题后，将路线图中的 `F-02` 状态改为 `[x]`。

如果审查发现问题，先使用单独的修复 commit 处理，不要提前勾选路线图。

### 第二步：启用半自动协调流程

本次新增脚本位于：

```text
scripts/beta/bootstrap.ps1
scripts/beta/run-task.ps1
scripts/beta/integrate.ps1
scripts/beta/README.md
```

主工作区干净后，初始化集成 worktree：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\beta\bootstrap.ps1
```

需要安装依赖时：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\beta\bootstrap.ps1 -InstallDependencies
```

该命令只安装首轮 Web Beta 所需的 Go 后端和 Web 前端依赖。以后恢复桌面端任务时，再追加 `-InstallDesktopDependencies` 安装 Tauri/Rust 依赖。

准备任务但不启动 Claude：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\beta\run-task.ps1 -TaskId F-03
```

检查生成的 prompt 后，再启动 Claude：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\beta\run-task.ps1 -TaskId F-03 -Resume -LaunchClaude
```

合并前先检查：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\beta\integrate.ps1 -TaskId F-03
```

确认后才执行完整验证和 fast-forward 合并：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\beta\integrate.ps1 -TaskId F-03 -ConfirmMerge
```

默认保持串行。不要急着开启 `-AllowParallel`。先完成 `F-02` 复核，再稳定执行 `F-03` 和 `F-04`。

## 4. 推荐的开发路线

首轮 Web Beta 推荐按以下顺序推进：

```text
F-02
  -> F-03
  -> F-04
  -> F-05 / F-06 / A-01
  -> V-01
  -> V-02
  -> V-03 / V-05
  -> V-04
  -> V-06
  -> A-02
  -> A-04
  -> A-03
  -> A-05
  -> G-01
  -> G-02
  -> G-03
  -> G-04
  -> G-05
  -> D-01
  -> R-01
```

路线图中的依赖关系仍然是最终依据。上面的顺序用于降低共享文件冲突，不用于绕过 roadmap 前置条件。

## 5. 你需要亲自准备的内容

### 5.1 DNS 和 HTTPS

在域名控制台添加：

```text
app.leeppp.online -> Web 服务器或负载均衡入口
api.leeppp.online -> API 服务器或负载均衡入口
```

为两个域名配置 HTTPS 证书。

生产 CORS 只允许：

```text
https://app.leeppp.online
```

不要使用 `*`，不要把本地 `localhost` 混入生产 Allowed Origins。

### 5.2 阿里云验证码 2.0

在阿里云验证码 2.0 控制台创建 Web/H5 场景，准备：

```text
CAPTCHA_PROVIDER=aliyun_v2
CAPTCHA_PREFIX=
CAPTCHA_SCENE_ID=
CAPTCHA_REGION=cn
CAPTCHA_ACCESS_KEY_ID=
CAPTCHA_ACCESS_KEY_SECRET=
```

其中 `prefix`、`scene_id` 和 `region` 可以作为公共配置下发给前端。AccessKey 只能放在后端运行环境中。

建议为验证码创建专用 RAM 用户并限制权限，不要使用阿里云主账号 AccessKey。

### 5.3 SMTP 邮件发送

用于发送邮箱验证和密码重置链接。准备：

```text
SMTP_MODE=smtp
SMTP_HOST=
SMTP_PORT=
SMTP_USER=
SMTP_PASSWORD=
SMTP_FROM_ADDRESS=
```

开发环境允许 logger fake。正式验收必须接入真实邮件服务。

### 5.4 用户协议和隐私政策

准备两份正式文档：

```text
docs/legal/terms.zh.md
docs/legal/privacy.zh.md
```

准备两个版本号，建议使用正式发布日期：

```text
legal.current_terms_version=YYYY-MM-DD
legal.current_privacy_version=YYYY-MM-DD
```

隐私政策至少说明：

- 收集哪些个人信息。
- 邮箱、IP、日志、上传内容分别用于什么目的。
- OSS、验证码、邮件服务和 LLM 服务如何参与处理。
- 数据保存时间和账号删除方式。
- 用户如何查询、更正和删除个人信息。
- 联系方式。

正式公开 Beta 前建议进行法律审阅。

### 5.5 OSS、PostgreSQL 和 Redis

正式环境准备：

```text
OSS_ENDPOINT=
OSS_ACCESS_KEY_ID=
OSS_ACCESS_KEY_SECRET=
OSS_BUCKET_NAME=
OSS_DOMAIN=

DB_DSN=
REDIS_ADDR=
REDIS_PASSWORD=
REDIS_DB=0
```

OSS Bucket 建议保持私有，通过短期签名 URL 下载。开发、预发布和生产数据库必须分离。

## 6. 密钥安全规则

可以写入文档或聊天：

```text
域名
法律文案路径
法律版本号
验证码供应商
Captcha prefix
Captcha scene_id
OSS 下载 hostname
```

不要写入文档、聊天或 Git：

```text
数据库密码
Redis 密码
JWT_SECRET
OSS AccessKey Secret
SMTP 密码
Captcha AccessKey Secret
Ed25519 私钥
```

秘密只放在本地被忽略的 `.env`、CI Secret 或部署平台密钥管理器中。提交到 Git 的 `.env.example` 只保留变量名和占位符。

## 7. 暂缓事项

以下内容暂时不要投入时间：

- 桌面端一键部署 `D-02` 至 `D-05`。
- 桌面端验收 `R-02`。
- Ed25519 正式密钥轮换。
- 桌面端 OSS 下载 host allowlist。
- 客户端下载发布。
- P1 范围中的收藏集、公告编辑器、自由 Agent 工具调用、OAuth、2FA 和支付。

## 8. 下次最小行动清单

下次只做以下三件事即可：

1. 复核已经提交的 `F-02`，处理审查意见并解除路线图安全闸门。
2. 在阿里云控制台创建 `app`、`api` DNS 记录和验证码 2.0 Web/H5 场景。
3. 主工作区干净后运行 `scripts/beta/bootstrap.ps1`，开始使用半自动协调流程。
