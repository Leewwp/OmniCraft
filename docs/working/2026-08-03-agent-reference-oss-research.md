# 2026-08-03 开源参考项目调研骨架（客户端 Agent 闭环）

> **创建日期**: 2026-08-03
> **预计失效日期**: 2026-10-03
> **用途**: wayfinder 规划输入 + 面试素材（全栈 + agent 双岗位）
> **上游决策**: ADR-0002（客户端 Agent 扩展与活人感约束）；术语见 `docs/GLOSSARY.md`「多端与 Agent（规划中）」
> **调研完成**: 2026-08-03（本文档已完成填充；调研方法：GitHub API 元数据 + 仓库 LICENSE/源码/docs 一手来源，未使用二手博客）
> **二轮补充调研**: 2026-08-03（用户以「轻量化 / 技术新 / 技术全面 / 面向普通用户」四维否决首轮主参考 OpenClaw 后重审。新增 A 类消费级轻量候选 7 个：Cherry Studio / LobeHub / Jan / AnythingLLM / Ollama / Chatbox / Open WebUI；OpenClaw 裁决降级为「局部借鉴参考」；「总体主推荐」重写，见对应章节；新增事实核查 #12~#24）

## 调研目标

为长期目标（手机端 app + Tauri 桌面 + 客户端 agent 闭环：一句话 → 找到感兴趣内容 → 下载到本地 → 游戏 mod 导入 / 新 skills 配置）找到可模仿、可适配的开源参考，支撑"模仿开源 + 适配，拒绝自研简陋方案"的项目倾向。

## 调研范围（四类，按优先级）

### A. 会话式 Agent 产品形态与权限 UX

- 目标：回答"一句话入口如何与 L1-L4 分级执行衔接"
- 候选池方向：GitHub Copilot 系列（agentic 建议→预览→确认→执行）、开源 agent 客户端/IDE agent（如 OpenClaw、Cline、Continue 等）
- 调研问题：
  1. 动作预览-确认-执行的 UX 模式如何呈现给用户（逐步确认 vs 批量确认）？
  2. 低风险动作自动执行与高风险动作确认的分级在实现上如何表达（工具 schema 权限声明）？
  3. 无 LLM 时的降级路径长什么样（规则/检索直连）？
  4. 会话历史、可撤销性（undo）如何设计？
- 关键产出：可借鉴的「动作契约」与「权限声明」格式
- **二轮补充候选池（消费级轻量向）**：Cherry Studio / LobeHub（原 LobeChat）/ Jan / AnythingLLM / Ollama / Chatbox / Open WebUI（见 A-5 ~ A-11）

#### 调研结果：A 类

##### 候选 A-1: Cline（cline/cline）

- **概况**: VS Code/JetBrains 系开源 IDE agent 客户端，用户量最大的开源 agent 之一；栈 TypeScript + React（monorepo：`apps/`、`sdk/`、`docs/`）；License **Apache-2.0**（GitHub API license 字段，2026-08-03 核实）；活跃度 ★65.5k、最近提交 2026-08-03（核实于 API）
- **能力映射**:
  - L1/L2/L3/L4 → 工具按类别分权：Read / Edit / Terminal / Browser / MCP 五类，每类可独立设"自动批准"或"每次确认"（`docs/features/auto-approve.mdx`）
  - 高风险动作表达：模型为每条命令动态打 `requires_approval` 标记（如 `npm install`、`rm -rf`、`mv`、`sed -i` 需确认；`npm run build`、`git status` 视为安全），非固定 allowlist（同上文档）
  - 可撤销性：Checkpoints —— 独立 shadow git 仓库，每次工具调用后对项目文件打快照，可随时回滚到任一检查点且保留对话（`docs/core-workflows/checkpoints.mdx`）
- **可借鉴点**:
  - 权限分类表 + "基础开关 + 扩展开关"嵌套语义（"Read all files" 依赖 "Read project files" 开关）→ 可映射为 L1 自动 / L2 自动+摘要 / L3 确认 / L4 逐级确认
  - Checkpoints 的 shadow-git 快照模式 → 对齐 ADR M5 批量回滚（但 Cline 是文件级回滚，我们是数据级 trace 回滚，概念可借鉴）
  - 命令安全度由模型按命令+参数动态标注（而非静态白名单）→ 与我们"规则直连降级"中的静态安全规则表互补
- **风险**: Apache-2.0 无传染；强 LLM 依赖（无无模型模式）；代码量大，核心在 `apps/vscode/src/core/`，模块边界清晰度中等
- **采纳建议**: **借鉴**（权限分类 UX 与 checkpoint 回滚模式；不拷贝代码，移植模式到 Go/Tauri 栈）

##### 候选 A-2: Continue（continuedev/continue）

- **概况**: 开源 IDE agent（VS Code/JetBrains），TypeScript；License **Apache-2.0**（API 核实）；活跃度 ★35.3k、最近提交 2026-08-03（API 核实）
- **能力映射**:
  - 工具即「动作契约」：`core/tools/definitions/*.ts` 每个工具声明 `type`、`function`（name/description/JSON Schema parameters）、`readonly`、`isInstant`、`displayTitle`/`wouldLikeTo`/`isCurrently`/`hasAlready`（UI 展示文案）、`defaultToolPolicy`（如 `allowedWithPermission`）
  - 策略层独立：`core/tools/policies/fileAccess.ts` 文件访问策略；`definitions/`、`implementations/`、`policies/` 三目录分离（`core/tools/` 目录结构核实）
  - 示例：`core/tools/definitions/editFile.ts` 的 `defaultToolPolicy: "allowedWithPermission"` + `readonly: false`
- **可借鉴点**:
  - 「动作契约」schema 是四类中**最清晰的模板**：JSON Schema 入参 + 展示文案 + 默认策略三件套 → 直接对应我们工具集的 L1 自动（`readonly: true, defaultToolPolicy: allowed`）、L2 自动+摘要、L3 需确认（`allowedWithPermission`）、L4 签名 grant
  - 契约与实现分离（definitions/policies/implementations）→ 动作集扩展只需注册新工具定义，无需碰执行内核
- **风险**: Apache-2.0 友好；TypeScript 栈与 Go 后端不共享代码（模式移植）；默认策略粒度较粗（工具级，非命令参数级，不如 Cline 的 `requires_approval` 细）
- **采纳建议**: **借鉴**（动作契约 schema 格式，作为我们工具声明与权限声明的格式蓝本）

##### 候选 A-3: OpenClaw（openclaw/openclaw）

- **概况**: 个人 AI 助理（任意 OS/平台，网关 + 节点架构），TypeScript 巨型 monorepo（`packages/` 20+ 核心包 + `extensions/` 100+ 扩展）；License **MIT**（LICENSE 文件实读：MIT License Copyright (c) 2026 OpenClaw Foundation；GitHub API 标 NOASSERTION 疑因 LICENSE 文件含 THIRD_PARTY_NOTICES 引注，实读内容为 MIT）；活跃度 ★38.5 万（API 返回 384,995，2026-08-03）、最近提交 2026-08-03（API 核实）
- **能力映射**:
  - 权限五模式：`tools.exec.mode` = `deny` / `allowlist` / `ask` / `auto` / `full`，每档解析为 `security`（allowlist 严格度）+ `ask`（miss 时是否询问）二元组；`auto` 引入自动审查器，miss 先自动审再交人（`docs/tools/permission-modes.md`）
  - 审批栈：本地 approvals 文档只能**收紧**配置、不能放宽（"effective policy is the stricter of tools.exec.* and approvals defaults"）；批准时绑定 cwd/argv/可执行路径，文件在批准后变更则拒绝执行（防 drift）（`docs/tools/exec-approvals.md`）
  - skills：SKILL.md 技能包 + 6 级加载优先级（workspace > `.agents/skills` > `~/.agents/skills` > state-dir > bundled > extraDirs）（`docs/tools/skills.md`）
  - 技能市场：ClawHub CLI `openclaw skills search/install/update/verify` + 发布侧 `clawhub` CLI；release trust 三态（malicious 拒绝 / risky 需 `--acknowledge-clawhub-risk` / official 跳过）（`docs/clawhub/cli.md`）
- **可借鉴点**:
  - 权限模式枚举 + allowlist + ask-on-miss → 与 ADR-0002 的 L1（自动）、L2（自动+摘要）、L3（建议）、L4（确认）逐档对应；`auto` 模式 = 我们的"规则直连降级 + 低置信交人工"雏形
  - 审批文件绑定（批准后文件漂移即拒执）→ L4 签名 grant 的防篡改思路（我们可绑定 hash/路径快照）
  - ClawHub 信任模型 + 安装器 CLI → 一键安装（Mod/skills 下载安装）的安全门参考
- **风险**: MIT 友好（附 THIRD_PARTY_NOTICES.md 引注义务）；monorepo 体量巨大、演进极快（活跃度说明一切），需按模块摘取；核心概念文档化程度高但代码层复杂度高
- **采纳建议**: **借鉴**（权限模式 + 审批栈 + ClawHub 信任模型；**主参考候选**）

##### 候选 A-4: GitHub Copilot 生态（闭源产品，官方文档一手来源）

- **概况**: 闭源 SaaS，无源码可读；但官方文档源码在开源仓库 `github/docs`（License **CC-BY-4.0**，★20.6k，2026-08-01 更新，API 核实）可作为一手来源
- **能力映射**:
  - 自动化治理：Rationale（理由）+ Confidence（高/中/低）+ Approvals 三级（automation level：Full control / Cautious / Balanced / Full automation），低于阈值自动应用、高于阈值自动执行；可对单条建议逐个 Accept/Decline 或批量 Accept all/Decline all（`content/copilot/concepts/agents/cloud-agent/about-automation-rationale-and-approvals.md`）
  - 关键论断（原文）："Approvals are a workflow convenience, not a security control"—— 审批不构成安全边界，边界必须靠工具/仓库权限（同上文件）→ 与 OpenClaw 的"approvals 只收紧"及我们 L4 签名 grant 的立场一致
  - Agent Skills：Copilot（Cloud Agent/CLI/App/agent mode）支持 Agent Skills 开放标准，项目技能目录 `.github/skills`、`.claude/skills`、`.agents/skills`，个人技能 `~/.copilot/skills`、`~/.agents/skills`；`gh skill` CLI 可从仓库安装（`content/copilot/concepts/agents/about-agent-skills.md`）
- **可借鉴点**:
  - Confidence 阈值 + 批量确认面板（Accept all/Decline all）→ L3 建议层与 L2 批量操作的确认 UX 直接对标
  - "审批是流程便利不是安全控制" → 设计原则：我们 L4 的签名 grant 才是安全边界，前端确认只是 UX
- **风险**: 闭源（模式可借鉴，代码不可得）；文档路径漂移（docs.github.com 路径 404，本次以 github/docs 仓库源码为准）；产品迭代快
- **采纳建议**: **参考**（模式层面对标，不构成代码/格式借鉴）

##### 候选 A-5: Cherry Studio（CherryHQ/cherry-studio）

- **概况**: 消费级 AI 生产力桌面客户端（多 LLM Provider 聚合 + 300+ 预置助手 + 知识库 + Agent + MCP），面向普通用户；栈 **Electron + React + TypeScript**（`electron.vite.config.ts` / `electron-builder.yml` / `src/{main,preload,renderer,shared}` 实读，**非 Tauri**）；License **AGPL-3.0**（`LICENSE` 全文实读：GNU Affero GPL v3；API license 字段 AGPL-3.0）；活跃度 ★49.3k、最近提交 2026-08-03（API 核实）；仅桌面三端（Win/Mac/Linux），无官方移动端
- **能力映射**:
  - L0-L4 → 无显式分级枚举；但具备完整 agent 运行时（`src/main/ai/agents/`：createAgent/runAgentTask）+ 任务调度（`AgentJobsService`：JobManager 注册 `agent.task` handler、事务写 + 定时触发器 + DEFAULT_TIMEOUT_MINUTES=2）+ MCP 工具启用开关（`src/renderer/types/mcp.ts`）——分级语义弱于 OpenClaw，确认粒度是"开关/会话"级
  - 一键安装 → **Skills 安装器**（`src/main/ai/skills/SkillInstaller.ts` + `SkillService.ts`）：来源含 claude-plugins.dev API + ClawHub（`ClawhubSkillDetailSchema` 实读）；ZIP 安装安全工程（MAX_EXTRACTED_SIZE=100MB / MAX_FILES_COUNT=1000 / `isOutsidePath` 防目录穿越）；安装到 `{dataPath}/Skills/` 后镜像到 `CLAUDE_CONFIG_DIR/skills`（SDK 发现），按会话白名单（`buildSkillWhitelist`）暴露
  - skills → SKILL.md 解析（`markdownParser.findSkillMdPath/parseSkillMetadata`）+ 技能库表 `agent_global_skill` + per-agent 启用
  - 检索 → knowledge base（`src/main/features/knowledge`，RAG）+ webLookup/fileLookup 工具
  - 内核内嵌 → **OpenClawService**（`src/main/services/OpenClawService.ts` 实读）：管理 OpenClaw 网关（local/remote 模式、端口 18790、auth token、`openclaw.json` 读写）→ 消费级外壳内嵌 agent 内核的成功验证
- **可借鉴点**:
  - **消费级 agent 客户端整体形态**：skills 安装器 + agent 任务编排 + MCP 工具开关 + 知识库，单应用（非 monorepo）结构、`src/{main,preload,renderer,shared}` 分层清晰 → 我们"Next.js 前端 + Go sidecar"的消费级产品形态蓝本
  - SkillService 安装安全工程（ZIP 炸弹上限 + 路径穿越校验 + 镜像目录）→ 我们 skills 一键安装的宿主工程清单（模式借鉴，不拷贝 AGPL 代码）
  - AgentJobsService 的 job handler 注册 + 事务写 + 超时 → 与 B-2 Modrinth install job 状态机互证的消费级实现（更薄）
  - OpenClawService 网关编排（模式/端口/健康探测/配置读写）→ 我们 Go sidecar 网关的服务编排形态
- **风险**: **AGPL-3.0**（仅模式借鉴；拷贝/链接源码触发全端开源义务）；Electron 栈（与我们 Tauri 不同，模式移植）；无移动端；仓库存在 `v2-refactor-temp/`（架构重构中，代码锚点可能漂移）
- **采纳建议**: **借鉴**（消费级 agent 客户端的整体形态蓝本；权限分级概念仍以 OpenClaw/Copilot 为准——Cherry 只做到开关级）

##### 候选 A-6: LobeHub（lobehub/lobehub，原 lobe-chat）

- **概况**: AI Agent 平台（"Chief Agent Operator"——多 agent 雇佣/调度/汇报的 7×24 运营形态），**Next.js App Router web + Electron 桌面 + 移动 web/PWA**；License **LobeHub Community License**（`LICENSE` 实读：基于 Apache-2.0 附加条款——不改源码的商用免费；**衍生作品商用须向 producer 购授权**；贡献者同意其代码可被商业使用）→ 非纯开源；活跃度 ★81.2k（API 核实，仓库已由 lobe-chat 改名 lobehub）、最近提交 2026-08-03
- **能力映射**:
  - L0-L4 → 无分级表达；Agent Builder（一句话描述 → 自动配置 agent）是"运营/创建"层而非"执行确认"层
  - 一键安装 → 插件市场索引仓库 `lobehub/lobe-chat-plugins`（★298）：`plugins/plugin-template.json` 条目 + **PR 提交 + 网站浏览**（README 实读）；agent 市场 `lobehub/lobe-chat-agents`（★1,191）；宣传口径"10,000+ Skills / MCP-compatible plugins"（README，数量未独立核实）
  - skills → 以 MCP 兼容插件承载（配套 `chat-plugin-sdk` / `chat-plugin-gateway` / `chat-plugin-template` 仓库）
  - 多端 → web（Next.js）+ 桌面（Electron，`apps/desktop` 用 electron-builder，**非 Tauri**）+ PWA/mobile web；另有 `lobe-ui-react-native`（RN 组件库，原生移动端迹象）
  - 检索 → web search + 文件上传 RAG（知识库）
- **可借鉴点**:
  - **消费级技能/插件市场形态**：索引仓库 + PR 提交 + 站内浏览安装（比 ClawHub CLI 更面向普通用户）→ 我们"内容仓库内索引 + 站内浏览 + 一键安装"的市场形态直接对标
  - Agent Builder（自然语言建 agent）→ 我们"一句话 → 生成 agent/skills 配置"的 onboarding 参考
  - Next.js + Electron 复用 web 代码（桌面内嵌 web 产物）→ 多端复用思路参考（我们走 Tauri 而非 Electron）
- **风险**: **License 非标准**（衍生作品商用需授权 + 贡献者权利让渡条款，代码级借鉴受限）；monorepo 偏大（src + packages + apps/{cli,desktop,server}）；产品形态已从"聊天客户端"转向"多 agent 运营平台"，与本项目会话式 agent 定位有偏差
- **采纳建议**: **参考**（插件市场形态 + Agent Builder 消费级 UX；不引入其平台架构与许可）

##### 候选 A-7: Jan（janhq/jan）

- **概况**: 本地优先 ChatGPT 替代（100% 离线可跑，Llama.cpp/MLX 推理），**Tauri 2 + React**（`src-tauri/` + `web-app/` 实读）+ 移动端（package.json scripts：`tauri ios dev` / `tauri android build`，Tauri 2 mobile 特性）；License **Apache-2.0**（`LICENSE` 实读：Copyright 2025 Menlo Research，附"署名请求"条款）；活跃度 ★43.8k、最近提交 2026-08-03（API 核实）
- **能力映射**:
  - L0-L4 → 无分级；MCP 集成（README）+ 扩展体系：`extensions/{assistant,conversational,download,llamacpp,mlx,rag,vector-db}-extension`（每扩展一职责）
  - 一键安装 → **Model Hub**（`web-app/src/routes/hub/`：模型浏览/搜索/详情/下载安装，配合 download-extension 与 llama.cpp/mlx 运行时）
  - skills → 无 SKILL.md 技能体系（以助手/扩展形态承载，未采用 Agent Skills 规格）
  - 检索 → rag-extension + vector-db-extension（本地 RAG）
  - 多端 → **桌面（Tauri 2）+ iOS + Android 一套核心**（移动为 tauri `--features mobile` 构建）+ Local API Server（`routes/local-api-server/`，OpenAI 兼容）
- **可借鉴点**:
  - **Tauri 2 多端工程形态**（Rust 核心 + web UI 复用，桌面/移动共用）→ 我们"手机 app + Tauri 桌面"多端的工程架构蓝本（栈契合）
  - 扩展体系（core + extensions/* 按职责拆分）→ 我们"内容类型安装器"按类型注册扩展
  - Hub 的模型浏览-下载-安装闭环 → "注册表 → 一键安装"消费级流程参考
- **风险**: 本地推理引擎体积大、价值集中于本地模型（非平台内容闭环）；Hub 只覆盖模型（无技能市场）；Apache-2.0 友好（可借鉴）
- **采纳建议**: **借鉴**（Tauri 2 桌面+移动多端工程形态 + Hub 下载安装流）

##### 候选 A-8: AnythingLLM（Mintplex-Labs/anything-llm）

- **概况**: 本地优先一体化 AI 助手（聊天 + RAG + Agents + 多用户 + 调度），**Electron + Node.js + React**（`frontend/` + `server/`，JavaScript 栈）；License **MIT**（API license 字段）；活跃度 ★64.3k、最近提交 2026-07-31（API 核实）；形态：桌面 app + Docker（多用户）+ 浏览器扩展 + 可嵌入 widget + Open Computer（子模块，计算机操作 agent，README 预告）
- **能力映射**:
  - L0-L4 → 无显式分级；工作区内 Agents + **智能技能选择**（`frontend/src/components/.../AgentSkills/skillRegistry.js` 实读：按 agent 启用/禁用子技能、技能偏好开关、clarifying questions、reranker、MaxToolCallStack）
  - 一键安装 → 无技能市场；"skills" = 应用连接器（filesystem / gmail / google-calendar / outlook 等，管理员面板配置，OAuth/文件系统授权型）
  - skills → AgentSkillSettings 面板（每 agent 技能开关 + 子技能偏好）→ 消费级的"技能开关"UX
  - 检索 → 多向量库 RAG（collector + 文档流水线）+ 多文档格式
  - 异步 → Scheduled Tasks（cron 调度 + agent 能力，README 引用 docs.anythingllm.com/scheduled-jobs）
- **可借鉴点**:
  - **智能技能选择 + 每 agent 子技能偏好** → 我们 L2 自动+摘要的"技能按需加载"UX（避免技能全开、控 token）
  - 连接器式技能（OAuth 授权型，如 gmail/calendar）→ 我们 L2 安全写（收藏/关注等平台动作）的授权形态参考
  - Scheduled Tasks（cron + agent）→ 定时/批量动作编排参考
- **风险**: Electron + 全 Node 栈（前端 React + 后端 Node，与 Go 无共享）；UI 偏生产力工具风格（非"活人感"美学）；一体形态体积大
- **采纳建议**: **参考**（技能开关面板 + 连接器授权形态 + 定时任务；整体架构不采用）

##### 候选 A-9: Ollama（ollama/ollama）

- **概况**: 本地 LLM 引擎（Kimi/GLM-5.2/MiniMax/DeepSeek/gpt-oss/Qwen/Gemma 等，README 最新描述），**Go 后端 + llama.cpp** + 桌面 app（**Go + 内嵌 WebView + React UI**，`app/` 目录实读：`webview/webview.go`、`ui/`、`wintray`、`updater`——非 Tauri，单二进制形态）；License **MIT**（API license 字段）；活跃度 ★177.7k、最近提交 2026-07-31（API 核实）
- **能力映射**:
  - L0-L4 → **无消费级权限确认 UX**（agent 会话自动调用 tools，无确认层；`agent/` 目录实读）
  - 一键安装 → **模型注册表闭环**：ollama.com/library 浏览 → 客户端拉取安装（`app/server` + `store` 下载管理）→ "浏览-下载-安装-运行"消费级流程
  - skills → **Go 原生 SKILL.md 系统**（`agent/skills.go` 实读）：扫描 `~/.ollama/skills` + **跨客户端约定 `.agents/skills/`** + 项目级 `.ollama/skills/`，名称冲突时 Ollama 自有目录 > `.agents/skills`；frontmatter（`name` 1-64 小写+连字符、`description` 非空）；目录名 = 技能名；`maxSkillBytes` 1MB；内置 `skill-creator` 技能；原则声明（实读原文）："Skills provide instructions only. They do not grant filesystem, network, shell, or approval privileges"
  - 检索 → 经 function calling 的 tools（无内置 RAG）
- **可借鉴点**:
  - **Go 版 SKILL.md 加载器**（目录优先级、frontmatter 解析、大小上限、跨客户端 `.agents/skills` 约定）→ 我们 Go 后端技能加载器可直接照抄此工程模式（MIT，可读源码）
  - "技能仅指令、不授予权限"声明 → 与 C-1/C-2 一致的规范叙事：权限只由 L 分级给
  - 单二进制 + 内嵌 WebView 桌面 → "轻量"架构范式（Go 全栈桌面备选形态）
- **风险**: 无权限确认层（消费级 agent UX 薄弱，分级参考价值低）；无技能市场（只有模型注册表）；桌面 UI 功能少（引擎定位，非闭环产品）
- **采纳建议**: **借鉴**（Go skills 加载器工程 + 模型注册表一键安装闭环；不引入其引擎定位）

##### 候选 A-10: Chatbox（chatboxai/chatbox）

- **概况**: 跨平台 AI 客户端（"Powerful AI Client"），**Electron + React + TypeScript**（`electron.vite.config.ts` + `vite.config.web.ts` 实读，桌面 + web）；License **GPL-3.0**（`LICENSE` 全文实读，GNU GPL v3——注意非坊间常见的 MIT 印象）；活跃度 ★41.3k、最近提交 2026-08-02（API 核实；仓库已由 Bin-Huang/chatbox 改名 chatboxai/chatbox）
- **能力映射**: 多 Provider、AI web search、跨设备同步、团队共享（`team-sharing/` 目录）；**无 skills / 无 agent 分级 / 无插件市场** → 与 L0-L4 能力映射弱
- **可借鉴点**: 消费级轻量客户端的**极简形态**（单一职责聊天客户端、代码量小、UI 克制）→ "轻量化"标杆观察
- **风险**: **GPL-3.0**（不拷贝）；功能面窄（无 agent/skills），不构成闭环参考
- **采纳建议**: **参考**（消费级 UX 极简性观察；不构成能力借鉴）

##### 候选 A-11: Open WebUI（open-webui/open-webui）

- **概况**: 自托管 AI 界面（接 Ollama / OpenAI 兼容 API），**Python (FastAPI) + Svelte**（`backend/open_webui/` 实读）；License **Open WebUI License**（`LICENSE` 实读：BSD-3-Clause 基础 + **品牌保留条款**——总用户 ≤50 的部署方可去品牌，否则须保留品牌或获书面许可）→ 非标准 BSD；活跃度 ★147.7k、最近提交 2026-08-02（API 核实）
- **能力映射**: RAG/检索、tools/functions 子系统（`backend/open_webui/tools/` + `functions.py` 实读）、多用户、Docker 部署为主
- **可借鉴点**: 社区化工具/函数生态的提交-安装形态（类似插件市场但面向自托管人群）
- **风险**: 自托管属性（部署门槛 = 开发者向，**不符合"面向普通用户"维度**）；Python/Svelte 栈不契合；许可非标准（品牌条款约束分发）；核心形态是网页控制台而非消费级客户端
- **采纳建议**: **放弃**（栈不契合 + 非消费级分发形态 + 许可非标准；仅作产品观察）

##### A 类小结：动作契约与权限声明格式（首轮四候选 + 二轮七候选提炼）

1. **动作契约格式**（借鉴 Continue）：每个工具 = `name + JSON Schema 入参 + 展示文案三元组 + readonly 标记 + 默认策略`；契约定义与执行实现分离（definitions/policies/implementations 三目录）
2. **权限声明格式**（借鉴 OpenClaw/Cline）：策略枚举（deny/allowlist/ask/auto/full 或 每类自动批准开关）+ 命令/参数级动态标注（`requires_approval`）+ 审批只能收紧不能放宽 + 批准绑定执行上下文（cwd/argv/文件快照）
3. **确认 UX**（借鉴 Copilot）：置信度阈值驱动"自动 vs 建议"，面板逐个/批量确认；**审批是 UX 不是安全边界**，安全边界在工具权限与签名层
4. **无 LLM 降级路径**：四候选均无（全部强依赖 LLM），开源生态无成熟先例 → 我们的"规则/检索直连降级"是差异化点，无参考可抄（见「调研缺口」）
5. **消费级权限确认 UX（二轮）**：A-5~A-11 七个消费级候选均无签名/凭证级授权实现——最高只到"工具开关 + 每次询问 + 插件/技能安装确认"（Cherry Studio 的 MCP 工具开关、LobeHub 插件安装确认、AnythingLLM 连接器 OAuth）→ L4 签名 grant 仍无开源先例，自研路线不变（见调研缺口 7）
6. **插件/技能市场形态（二轮）**：消费级市场 = **索引仓库 + PR 提交 + 网站浏览/一键启用**（LobeHub plugins/agents index），与 ClawHub CLI（开发者向）形成对照 → 我们技能市场采用"内容仓库内索引 + 站内浏览 + 一键安装"形态
7. **多端形态（二轮）**：Tauri 2 一套核心多端（Jan：桌面 + iOS + Android）；Electron 系（Cherry/LobeHub/Chatbox）仅桌面+web；Ollama 走 Go+webview 单二进制 → 我们（Tauri 桌面 + app 端）以 Jan 为工程形态参考
8. **栈契合红利（二轮）**：**Ollama 是 Go 生态中唯一 MIT 的 SKILL.md 加载器实现**（agent/skills.go）→ 直接工程借鉴，无需移植 TS 实现

### B. Mod 部署/管理客户端（对应 L4 + 一键安装）

- 目标：回答"游戏 mod 导入"的工程形态
- 候选池方向：Vortex、Modrinth App、CurseForge App、GDLauncher
- 调研问题：
  1. 游戏目录如何发现/检测（registry？用户配置？扫描）？
  2. mod 启停、冲突检测、备份回滚的实现方式？
  3. 下载完整性校验与安全模型（签名/哈希）？
  4. License 与可借鉴程度（MIT/GPL/AGPL 对闭源自研的限制）？
- 关键产出：动作集扩展候选（mod 启停/冲突/更新）与安全设计对照

#### 调研结果：B 类

##### 候选 B-1: Vortex（Nexus-Mods/Vortex）

- **概况**: Nexus Mods 官方 mod 管理器，Electron + TypeScript + React（`src/renderer/src/extensions/` 模块化）；License **GPL-3.0**（API license 字段核实，仓库 LICENSE.md）；活跃度 ★1.45k（npm 用户量大但 repo star 少）、最近提交 2026-08-03（API 核实）
- **能力映射**:
  - L4 文件操作：mod 通过"链接部署"写入游戏目录（`src/renderer/src/extensions/mod_management/LinkingDeployment.ts`：`IDeployment` 描述目标文件映射，部署上下文 `previousDeployment → newDeployment` 做差异处理，备份标签 `BACKUP_TAG = ".vortex_backup"`）
  - 游戏目录发现：每游戏一个扩展（`extensions/games/` 50+ 游戏，如 `game-baldursgate3`）+ 平台 store 检测（`extensions/gameinfo-steam/src/index.ts`：`util.steam.allGames()` 读 Steam 库 + `steamAppId` 回退；另有 `gameinfo-gog`/`gamestore-*` 扩展）→ registry/扫描/用户配置混合
  - mod 启停与依赖：`extensions/gamebryo-plugin-management`（插件启停）、`extensions/mod-dependency-manager`（依赖顺序/冲突）
- **可借鉴点**:
  - 差异部署 + `.vortex_backup` 备份标签 → 一键安装的回滚机制参考（部署前快照、失败/卸载恢复）
  - per-game 扩展 + per-store 发现模块 → 我们的"内容类型（图文/视频/游戏 mod）"安装器可按类型注册扩展
  - 部署状态机（previous → new diff）→ 我们的 install job 预览-确认-回滚管线
- **风险**: **GPL-3.0 传染**（仅借鉴概念/模式、不拷贝代码可放宽；若拷贝源码必须开源）；Electron/TS 栈与我们 Tauri/Rust 不同；代码规模大、文档质量中等（以 AGENTS-*.md 为主）
- **采纳建议**: **参考**（部署差异 + 备份回滚概念；不拷贝代码；架构可读性不如 Modrinth App 干净）

##### 候选 B-2: Modrinth App（modrinth/code/apps/app）

- **概况**: Modrinth 官方桌面应用，**Tauri 2 + Rust + Vue**（`frontendDist: ../app-frontend/dist` 于 `apps/app/tauri.conf.json`）；位于 modrinth/code monorepo（★2.27k，2026-08-03 活跃，API 核实）；License **GPL-3.0-only**（`apps/app/LICENSE` 全文 GPL-3.0 实读；`apps/app/Cargo.toml` `license = "GPL-3.0-only"`；COPYING.md 佐证）—— **注意：坊间常传"Modrinth App 是 MIT"，不实**
- **能力映射**:
  - 一键安装：安装即任务队列（`apps/app/src/api/install.rs`：`install_get_modpack_preview` → `install_create_instance` → job 状态机 `install_job_list/retry/cancel/dismiss`，`InstallJobSnapshot` 快照），底层基于 theseus 库（`packages/app-lib/Cargo.toml` `name = "theseus"`）
  - 实例/文件管理：`apps/app/src/api/instance.rs`、`files.rs`、`jre.rs`（Java 运行时管理）
  - Tauri 权限声明：`apps/app/capabilities/{core,ads,plugins,updater}.json` 显式声明窗口/插件权限（Tauri 2 capabilities 模型）
  - UI：`apps/app-frontend/src/pages/{instance,library,project,Browse}.vue`（实例、mod 库、详情、浏览）
- **可借鉴点**:
  - **install job 系统（preview → create → retry/cancel/dismiss）** → 一键安装的动作集与异步任务编排直接蓝本
  - **Tauri 2 capabilities 权限声明文件** → 我们 L4 签名 grant 的前端权限声明格式参考（声明式、按 window/插件细分）
  - Rust 侧结构（`src/api/*.rs` 每模块一个 Tauri command 组）→ 我们的 Go sidecar + Tauri 边界划分参考
- **风险**: **GPL-3.0-only**（借鉴模式可，拷贝源码会传染；fork 须移除 Modrinth branding）；Rust 1.90+ 版本要求偏高；monorepo 庞大（整个 modrinth/code 含后端 labrinth，需只看 apps/）
- **采纳建议**: **借鉴**（Tauri capabilities + install job 系统模式；不拷贝代码）

##### 候选 B-3: GDLauncher（gorilla-devs/GDLauncher v1 + GDLauncher-Carbon）

- **概况**: Minecraft 启动器 + mod 管理。v1（GPL-3.0，★1.2k）**已停更**（最后推送 2024-06-14，API 核实）；继任者 GDLauncher-Carbon（Rust + Vue，★228，2026-07-28 活跃）License 为 **Business Source License 1.1**（`LICENSE` 实读：GorillaDevs 商业条款，非商业生产使用受限，Change Date 后转 GPL-3.0）
- **能力映射**: 与 B-1/B-2 重叠（启动器内 mod 管理、实例）；无独特可借鉴点高于上述两者
- **可借鉴点**: 无显著增量（如需可参考 v1 的 mod 安装历史实现，但已停更且 GPL）
- **风险**: v1 停更；Carbon **BSL-1.1** 生产使用受限（闭源自研不可直接依赖其代码）；两版本许可都不适合代码级借鉴
- **采纳建议**: **放弃**（维护状态 + 许可限制；仅作产品形态观察）

##### 候选 B-4: CurseForge App（考察后放弃）

- **概况**: 闭源商业应用，**无官方开源仓库**（GitHub 搜索仅见第三方非官方拆解/工具，无一手来源；2026-08-03 搜索核实）
- **采纳建议**: **放弃**（闭源，无一手来源可调研；仅作为产品形态观察对象）

##### B 类小结

1. **目录发现**（Q1）：per-game/per-store 扩展 + 平台注册表/Steam 库/用户配置混合（Vortex 模式）；我们可简化为"游戏目录由用户配置 + 常见路径扫描"
2. **启停/冲突/回滚**（Q2）：差异部署 + `.vortex_backup` 备份标签 + job 快照 retry/cancel（Vortex/Modrinth 模式）→ 对应我们动作集扩展（mod 启停/更新）与一键安装回滚
3. **完整性校验**（Q3）：**未找到签名级先例**——Vortex/Modrinth 靠平台 API 下载 + 依赖版本锁定，未发现下载后 hash 校验/签名验证的明确实现（缺口，见文末）；我们 L4 的签名 grant 仍需自研（HMAC→Ed25519 路径不受影响）
4. **License**（Q4）：主流 mod 管理器全线 **GPL-3.0 系**（Vortex GPL-3.0、Modrinth App GPL-3.0-only、GDLauncher v1 GPL-3.0、Carbon BSL-1.1）→ 均不可代码级复用，只能模式借鉴；无 MIT/Apache 合格替代（这是 B 类的结构性约束）

### C. Agent Skills 生态（对应"新 skills 配置"）

- 目标：回答"skill 是什么、平台如何承载并安装它"
- 候选池方向：Claude Code skills（SKILL.md 格式）、OpenClaw skills、MCP 工具注册、GPTs/自定义指令生态
- 调研问题：
  1. 技能包的文件格式、目录约定与校验方式？
  2. 平台 `prompt` 类型内容如何映射为可安装的技能包？
  3. 技能安装目录写入的安全边界（对齐 L4 动作集 write_config）？
  4. 是否存在可复用的开源"技能市场/安装器"实现？
- 关键产出：技能包格式选型 + 平台内容类型映射设计

#### 调研结果：C 类

##### 候选 C-1: Agent Skills 规格 + anthropics/skills

- **概况**: Anthropic 主导的开放技能标准（站点 agentskills.io，规格源自 anthropics/skills 仓库 `spec/agent-skills-spec.md`，现指向 https://agentskills.io/specification）；anthropics/skills 官方示例仓库 ★16.6 万、2026-07-24 活跃（API 核实）；**无统一 LICENSE**（API license 字段 null；README 声明各技能自含许可，多数 Apache-2.0，其中 docx/pdf/pptx/xlsx 为 source-available 非开源）
- **能力映射**:
  - 格式：技能 = 目录 + 必需 `SKILL.md`（YAML frontmatter：`name`（1-64 字符小写+连字符、须与目录名一致）/`description`（≤1024）/`license`/`compatibility`/`metadata`/`allowed-tools`（实验性：预授权工具白名单）+ Markdown 正文；可选 `scripts/`、`references/`、`assets/`（规格全文）
  - 渐进披露（progressive disclosure）：metadata（~100 token，启动加载）→ 正文（<5000 token，激活时加载）→ 资源（按需加载）
  - 校验：`skills-ref` 参考库 CLI `skills-ref validate ./my-skill`（https://github.com/agentskills/agentskills）
- **可借鉴点**:
  - **技能包格式选型直接采用 Agent Skills**：开放标准 + 多实现采纳（Copilot 已声明支持，见 A-4），避免自造格式
  - `allowed-tools` 字段（实验性）→ 与我们"技能包声明所需动作集"（如安装型技能需 L4 write_config 授权）概念对齐
- **风险**: 规格由 Anthropic/agentskills 组织治理（开放标准但主导方单一，须跟踪 governance 变化）；`allowed-tools` 跨实现支持不保证；示例仓库部分技能 source-available（注意区分）
- **采纳建议**: **借鉴**（技能包格式唯一选型；格式与工具生态直接采用，内容自产）

##### 候选 C-2: OpenClaw skills（openclaw/openclaw，MIT）

- **概况**: 见 A-3 概况（MIT、★38.5 万、活跃）；skills 是 SKILL.md 兼容实现 + 生态扩展（顶层 `skills/` 目录含官方技能，如 `skills/github/SKILL.md`）
- **能力映射**:
  - 加载优先级 6 级：workspace skills > `.agents/skills` > `~/.agents/skills` > managed/state-dir > bundled > extraDirs（`docs/tools/skills.md` 表格）；技能名以 frontmatter `name` 为准（目录只作组织）
  - 安装器：ClawHub CLI（search/install `@owner/<slug>` /update/verify，`--global` 装到共享目录；`skills-sh:` 源由 ClawHub 解析为 commit-pinned GitHub 仓库，本地从不直连 skills.sh）（`docs/clawhub/cli.md`）
  - 信任模型：malicious/blocked 直接拒绝；risky 需 `--acknowledge-clawhub-risk`；official/bundled 跳过（同上文档）
  - gating/allowlist：技能按环境/配置/二进制存在性过滤（`docs/tools/skills.md`）；技能可由 agent 起草、人类审批（Skill Workshop，`docs/tools/creating-skills.md` 引用）
- **可借鉴点**:
  - **技能市场/安装器实现参考**（ClawHub）：搜索/安装/版本/更新/验证 + 三态信任门 → 我们"skills 配置安装"的安装器与安全门蓝本（Q4 直接命中）
  - 技能加载优先级 + name 归一化 → 多端（workspace/个人/托管）技能冲突解决
  - Skill Workshop（agent 起草 → 人工审批）→ 与 L3 建议不执行模式天然契合
- **风险**: MIT 友好；随 OpenClaw 演进快、API 可能漂移；依赖 OpenClaw 生态概念（gateway/node）部分不适用
- **采纳建议**: **借鉴**（技能安装器 + 信任模型 + 加载优先级；不引入 OpenClaw 运行时）

##### 候选 C-3: MCP 工具注册（modelcontextprotocol/modelcontextprotocol）

- **概况**: 开放工具调用协议规范；License **MIT→Apache-2.0 过渡期**（LICENSE 文件实读：新贡献 Apache-2.0、旧贡献保持 MIT、文档贡献 CC-BY-4.0，混合状态需注意）；活跃度 ★8.8k、2026-08-03（API 核实）
- **能力映射**:
  - 工具声明：`tools` capability + 每工具 `inputSchema`（JSON Schema **2020-12 方言**，见 seps/2106-json-schema-2020-12）（`docs/specification/draft/server/tools.mdx`）
  - 人工在环（原文要求）："there SHOULD always be a human in the loop with the ability to deny tool invocations"；UI 须展示哪些工具暴露给模型、调用时有可见指示、确认提示（同上文档）
- **可借鉴点**:
  - **工具 schema 方言统一用 JSON Schema 2020-12**（与 A-2 Continue 的动作契约兼容）
  - "人工在环 + 工具暴露可见性"条款 → 我们 L 分级 + M4 透明徽标的叙事支撑（行业规范级先例）
- **风险**: 双许可混合期（引用规范文本时注意区分 MIT/Apache 部分）；协议只定义调用通道，不解决权限/授权层（L4 仍要自研）
- **采纳建议**: **借鉴**（工具 schema 方言与人工在环条款；不引入 MCP 作为唯一工具通道——我们有服务端工具集）

##### 候选 C-4: GPTs / 自定义指令生态（考察后放弃）

- **概况**: OpenAI GPTs（自定义 GPT 市场）无公开格式规范（闭源生态）；Claude 的官方 skills 使用文档站（code.claude.com / support.claude.com）本次抓取超时不可达，但格式已由 agentskills.io 规格完整覆盖（不构成缺口）
- **采纳建议**: **放弃**（GPTs 无一手格式来源；Claude skills 格式以 agentskills.io 规格为准，C-1 已覆盖）

##### C 类小结

1. **格式与目录**（Q1）：**Agent Skills（SKILL.md）为事实标准**——Claude、Copilot、OpenClaw 三方采纳；目录约定 `skills/<name>/SKILL.md` + scripts/references/assets；`skills-ref validate` 校验
2. **prompt 类内容映射**（Q2）：平台内容类型（如"AI 技能"教程/配置模板）→ 打包为 SKILL.md 技能包（frontmatter metadata 可带平台类型标签）；Copilot/OpenClaw 均支持从 GitHub 仓库直接安装 → 我们的内容即可作为技能包分发
3. **写入安全边界**（Q3）：OpenClaw 安装器信任门（三态）+ 装到 workspace/共享目录由用户显式指定（`--global`）；对齐 L4 write_config：安装动作 = 签名 grant + 目标目录校验（无现成"沙箱技能执行"先例，需自研）
4. **市场/安装器**（Q4）：**ClawHub 是唯一合格开源参考**（搜索/安装/验证 + 信任状态）；无其他成熟开源技能市场

### D. 移动端内容社区 / 轻量 agent（对应 L1/L2 + app）

- 目标：回答"手机端 agent 的安全操作边界"
- 候选池方向：内容社区 app 的收藏/关注交互（小红书系）、离线阅读（Pocket/Instapaper）、移动端 agent 检索 UX
- 调研问题：
  1. 移动端收藏集/关注操作的 agent 化边界与撤销体验？
  2. 移动端离线内容库（下载到本地）的配额与存储管理？
  3. 推送与后台限制下，异步任务（如批量收藏）如何编排？
- 关键产出：移动端闭环的最小形态（发现 + 收藏 + 应用内下载）

> 注：本类骨架为方向描述，调研中具体化为可核查项目：**wallabag**（离线阅读/收藏，Pocket 开源替代）、**Mastodon**（内容社区 app + 细粒度写权限 API）、**Mobile-Agent**（移动端 agent 检索/操作 UX）。小红书/Pocket/Instapaper 均闭源无一手来源，作观察对象。

#### 调研结果：D 类

##### 候选 D-1: wallabag + wallabag/android-app

- **概况**: 自托管"稍后读"（Pocket/Instapaper 的开源替代）：PHP 后端 ★12.9k、2026-08-03 活跃、License **MIT**（API 核实）；Android 客户端仓库 wallabag/android-app ★583、2026-07-24 活跃、License **GPL-3.0**（API 核实）
- **能力映射**:
  - 收藏/标注：REST API `POST/GET /api/entries`，条目带 `is_archived`/`is_starred`/tags/annotations 字段；分页 `perPage`（`doc/content/developer/api/methods.md`）
  - 移动端离线：app 本地缓存文章与图片供离线阅读、"Needs no special permissions on Android 6.0+"（`wallabag/android-app` README）
- **可借鉴点**:
  - 收藏条目的标注字段模型（archived/starred/tags）→ 我们 L2 安全写（收藏集 CRUD + 放内容）的数据形态参考
  - 离线阅读的"缓存图片 + 无特权"模式 → 应用内下载的最小形态
- **风险**: PHP 栈（仅借鉴 API/数据形态）；后端 MIT 可参考、**app 为 GPL-3.0**（不拷贝）；无批量操作先例（逐条 API）
- **采纳建议**: **参考**（收藏/离线内容库的 API 与数据模型）

##### 候选 D-2: Mastodon（mastodon/mastodon）

- **概况**: 去中心化微博社区，Rails + PostgreSQL；License **AGPL-3.0（强传染，明确标注）**（API 核实）；活跃度 ★50.2k、2026-08-03（API 核实）
- **能力映射**:
  - 细粒度写权限：OAuth scope 体系 `write:favourites` / `write:bookmarks` / `write:follows` 等按动作域授权（`app/controllers/api/v1/statuses/favourites_controller.rb`、`bookmarks_controller.rb` 的 `doorkeeper_authorize!` 实读）
  - 异步编排：写操作后 `UnfavouriteWorker.perform_async`（Sidekiq 队列），API 同步返回最终态（同文件实读）
  - 取消语义：favourite 再次调用即取消（幂等 toggle 风格），计数 `max(count-1, 0)` 防负
- **可借鉴点**:
  - **OAuth scope = 权限声明的最小粒度参照**：`write:favourites` 这类按动作域细分的 scope → 我们 L2 安全写工具（收藏/取关/收藏集）的权限声明可直接借鉴此粒度与命名
  - 异步 worker + 同步返回 → 移动端"批量收藏"的后台任务编排形态
- **风险**: **AGPL-3.0 传染**（绝不拷贝代码；仅接口语义与权限模型观察）；Rails 栈与 Go 无关
- **采纳建议**: **参考**（OAuth scope 权限模型 + 收藏/书签端点语义）

##### 候选 D-3: Mobile-Agent（X-PLUG/MobileAgent）

- **概况**: 阿里巴巴通义实验室 GUI Agent 家族（Mobile-Agent-v3.5 + GUI-Owl 视觉模型），研究导向；License **MIT**（API 核实）；活跃度 ★9.0k、2026-07-07（API 核实）
- **能力映射**:
  - 移动端检索/操作 UX：视觉模型感知手机屏幕 → 规划/执行（多 agent 编排：planning、progress management、reflection、memory）（README NEWS 节）
  - OSWorld-MCP：MCP 工具调用能力的真实场景基准（README）
- **可借鉴点**:
  - 移动端 agent 的"计划-执行-反思"编排思想 → 我们手机端 agent 检索-确认-收藏闭环的交互叙事参考
- **风险**: 研究项目（演示导向，生产落地需大量工程）；强依赖 GUI 视觉模型（与我们的服务端检索 agent 路线不同）
- **采纳建议**: **参考**（形态与编排思想；不落地其模型栈）

##### D 类小结

1. **agent 化边界与撤销**（Q1）：收藏/关注 = 幂等 toggle + 同步返回 + scope 化授权（Mastodon 模式）→ L2 安全写边界；撤销走"再调用即取消"语义，配合 ADR M5 批量回滚
2. **离线配额**（Q2）：wallabag 无硬配额（自托管无限制）→ 配额机制无开源先例，需自研（对齐隐性配额 M2 思路）
3. **异步编排**（Q3）：worker 队列 + 同步返回（Mastodon）；移动端后台限制下的批量编排无开源成熟先例（缺口）
4. **最小闭环形态**：发现（L1 检索）→ 收藏集/关注（L2 scope 化写）→ 应用内下载/离线（配额 + 缓存）

## 选型标准

| 维度 | 说明 |
|------|------|
| 架构可读性 | 代码结构清晰、可局部借鉴（模块级复用），而非整体搬运 |
| 轻量化 | 代码规模可控、可整体读懂学透；巨型 monorepo / 重依赖降权（用户 2026-08-03 追加：OpenClaw 等开发者向全功能项目偏臃肿） |
| 技术新 | 新技术栈/架构模式加分（Tauri 2、新协议、新框架）；维护停滞项目降权 |
| 面向普通用户 | 消费级（consumer-grade）产品形态优先：UI/UX 非开发者工具、一键安装、普通人可用；开发者/geek 工具降权（用户 2026-08-03 追加） |
| 栈契合 | Go/Rust/TypeScript 优先；React/Tauri 生态加分；需重新实现的语言栈降权 |
| 能力对应 | 与 L0-L4 分级、一键安装、skills 生态、活人感约束的具体能力映射 |
| License·活跃度·文档质量 | 优先 MIT/Apache；回避 GPL/AGPL 传染（若仅借鉴不拷贝代码可放宽）；star/commit 活跃度；文档与架构说明完整性 |

## 输出模板（每候选一节）

```
## [项目名]
- 概况: 一句话 + 栈 + License + 活跃度
- 能力映射: L0-L4 对应表 / 一键安装 / skills / 检索
- 可借鉴点: 具体模块/模式（附文件或文档锚点）
- 风险: 许可、栈、维护状态
- 采纳建议: 借鉴 / 参考 / 放弃
```

## 完成标准（门）

调研完成判定（2026-08-03 用户确认），不满足任一即追加调研：

1. 每类候选池（A/B/C/D）**至少 2 个候选**完成模板填充；不足 2 个的类别须注明原因 —— 满足：A 11 个（首轮 4 + 二轮新增 7）/ B 4 个（2 个考察后放弃但已注明原因）/ C 4 个（1 个考察后放弃已注明）/ D 3 个
2. 每个候选必须有明确「采纳建议」：借鉴 / 参考 / 放弃（含理由）—— 满足
3. 全篇给出**总体主推荐**：至少 1 个主参考（架构蓝本）+ 2~3 个辅助参考（分模块借鉴）—— 满足（见下节）
4. 每个论断附一手来源引用（GitHub 仓库 / LICENSE 文件 / 官方文档 / 源码锚点），License 须以仓库实际 LICENSE 文件为准，不以 README 声称代替 —— 满足（引用清单见「关键事实核查记录」）
5. 每候选标注活跃度证据（star 量级 + 最近提交日期，如可获取）—— 满足（均来自 GitHub API 2026-08-03 拉取）
6. 面试叙事：每个主/辅助参考提炼"它解决了什么问题 → 我们怎么适配 → 我们的差异化（活人感约束）"三句话 —— 满足（见下节）

## 总体主推荐（2026-08-03 二轮修订）

> **二轮修订说明**：用户以「轻量化 / 技术新 / 技术全面 / 面向普通用户」四维否决首轮主参考 OpenClaw（巨型 monorepo + 开发者向）。OpenClaw 裁决降级为「局部借鉴参考」（见下节），主参考改为 **Cherry Studio**（消费级 agent 客户端整体形态），辅助参考补入 **Ollama**（Go 版 SKILL.md 加载器 + 注册表一键安装）、**LobeHub**（消费级技能/插件市场）、**Jan**（Tauri 2 多端工程）；首轮仍成立的结论（Continue 动作契约 / Modrinth App / Agent Skills 规格 / Mastodon OAuth scope / Copilot 确认 UX）**沿用首轮**。

### 主参考（架构蓝本）: Cherry Studio（CherryHQ/cherry-studio，AGPL-3.0，模式借鉴）

- **一句话理由**: 唯一同时具备"消费级桌面 UX + Skills 安装器（含 ZIP 安全工程）+ Agent 任务编排（JobManager）+ MCP 工具开关 + 知识库检索 + 内核网关内嵌（OpenClawService）"的**单应用**形态，体积与可读性在可学透范围（`src/{main,preload,renderer,shared}` 分层，非 monorepo），是 L0-L4 执行层之外"产品形态"的最佳参照系
- **三句话叙事**:
  1. **它解决了什么问题**: 一个普通用户可用的 agent 客户端如何把"技能安装（ZIP 安全上限 + 路径穿越校验 + 镜像目录）、agent 任务编排（job handler + 事务写 + 超时）、MCP 工具启用、知识库检索"收进一个桌面应用，并让会话式 UI 保持消费级（300+ 预置助手、一键切换模型）——同时证明 agent 内核可整体内嵌（它自己就内嵌 OpenClaw 网关，`OpenClawService.ts` 管 local/remote 模式 + 端口 + token）
  2. **我们怎么适配**: 以 Cherry 的产品分层为形态蓝本（会话 UI 层 / agent 运行时层 / 工具与技能层 / 网关层），把内核替换为我们的 Go sidecar（对齐 OpenClawService 的网关编排模式）；Skills 安装器的安全工程清单（解压上限、路径校验、安装后镜像、按会话白名单暴露）移植为 Go 实现；agent 任务编排对齐 Modrinth install job 状态机（沿用首轮）与 AgentJobsService 的事务式 job 注册
  3. **我们的差异化（活人感约束）**: Cherry 的分级确认只到"工具开关/每次询问"，无签名授权、无配额、无信号隔离、无按 trace 回滚——我们补上 M1 数据标注（`assisted_by_agent` + `trace_id`）、M2 隐性配额（收藏宽松/点赞严格/单批 ≤5）、M3 信号隔离、M4 透明徽标（手改去标）、M5 按 trace 批量回滚、L4 签名 grant；"活人感 + 签名级授权"仍是开源 agent 生态空白，是面试叙事差异点

### 辅助参考 1: Ollama（ollama/ollama，MIT）—— Go 版 SKILL.md 加载器 + 注册表一键安装

- **一句话理由**: Go 生态唯一 MIT 的 SKILL.md 技能加载器（`agent/skills.go`：目录扫描优先级、frontmatter 解析、1MB 上限、跨客户端 `.agents/skills/` 约定、"技能仅指令不授予权限"原则）+ 模型注册表"浏览-下载-安装"闭环，与我们 Go 后端栈完全契合
- **三句话叙事**: 它解决了"本地引擎如何承载可移植技能"——用与 Agent Skills 规格一致的文件约定（`~/.ollama/skills` + `.agents/skills` + 项目级 `.ollama/skills`，目录名=技能名，frontmatter 校验），并明确技能不越权 → 我们 Go 后端的技能加载器直接照抄该工程模式（MIT 可读源码），一键安装的"注册表 → 客户端下载 → 校验安装"流程映射我们的 Mod/skills 下载安装 → 差异化：加载器之上加 L 分级授权（技能不授权、权限由签名 grant 给）、M2 配额与 M4 徽标，这是 Ollama 完全没有的层

### 辅助参考 2: LobeHub（lobehub/lobehub，LobeHub Community License）—— 消费级技能/插件市场 + Agent Builder

- **一句话理由**: 消费级技能/插件市场的完整工程形态（索引仓库 + PR 提交 + 站内浏览安装，`lobe-chat-plugins`/`lobe-chat-agents`），以及"一句话建 agent"的 Agent Builder UX
- **三句话叙事**: 它解决了"普通用户如何发现、安装、启用技能"——市场不是 CLI 而是网站索引（可浏览、可一键启用）→ 我们技能市场采用同形态：内容仓库内索引 + 站内浏览 + 一键安装（安全门沿用 OpenClaw 三态信任模型，局部借鉴）→ 差异化：每个技能包安装前做 L4 级路径/脚本审查（agent 起草 → 人工审批，对齐 Skill Workshop 与 L3 建议不执行），安装动作本身是可回滚的签名 job

### 辅助参考 3: Jan（janhq/jan，Apache-2.0）—— Tauri 2 桌面+移动多端工程

- **一句话理由**: Tauri 2 一套核心跑桌面 + iOS + Android 的工程形态（`src-tauri/` + `web-app/` + `--features mobile`），与我们"手机 app + Tauri 桌面"多端目标同栈；Model Hub 的浏览-下载-安装流是消费级一键安装的又一佐证
- **三句话叙事**: 它解决了"同一套 UI/核心如何跨桌面与移动分发"——Rust 核心 + web UI 复用、扩展按职责拆分（assistant/download/rag/vector-db…）→ 我们以此为多端工程蓝本，内容类型安装器按扩展注册 → 差异化：移动端动作全部收敛到 L1/L2 低风险域 + M2 配额（对齐"手机端 agent 的安全操作边界"，D 类目标）

### 沿用首轮（结论不变，不再展开）

- **Continue**（Apache-2.0）：动作契约格式（definitions/policies/implementations 三目录 + JSON Schema 入参 + 展示文案 + defaultToolPolicy）——我们工具集与 L1-L4 权限声明的统一格式蓝本
- **Modrinth App**（GPL-3.0-only）：Tauri 2 capabilities 权限声明文件 + install job 状态机（preview → create → retry/cancel/dismiss）——L4 桌面执行与一键安装任务管线蓝本
- **Agent Skills 规格**（agentskills.io + anthropics/skills）：SKILL.md 开放标准，技能包格式唯一选型
- **Mastodon**（AGPL-3.0）：OAuth scope 粒度（`write:favourites` 等）——L2 安全写权限声明粒度参照
- **Copilot 文档**（github/docs，CC-BY-4.0）：Confidence 阈值 + 批量确认面板 + "审批是流程便利不是安全控制"——L3 建议层 UX 对标（附面试备选叙事）

### OpenClaw 重审裁决（2026-08-03）: 主参考 → 局部借鉴参考（降级）

- **裁决**: **降级**——不再作为主参考（架构蓝本），保留为**局部借鉴参考**（模块级，不整体搬运）
- **理由**:
  1. **轻量化维度不合格**：38.5 万 star 的巨型 monorepo（`packages/` 20+ 核心包 + `extensions/` 100+ 扩展），无法整体读透，与"代码规模可控"标准直接冲突
  2. **面向普通用户维度不合格**：CLI/gateway/node 架构 + YAML 配置 + 开发者向文档（docs/tools/*.md 面向开发者配置），无消费级 UI 形态——它解决的是"开发者如何配置个人助理"，不是"普通用户如何一句话用起来"
  3. **其定位被 Cherry Studio 的实践佐证**：Cherry Studio（消费级客户端）选择**内嵌 OpenClaw 作为内核**（OpenClawService 管网关），说明 OpenClaw 的正确角色是"内核/引擎"而非"产品形态蓝本"——我们同构：概念借鉴其内核设计，产品形态学 Cherry
  4. **演进过快风险**：最近提交 2026-08-03、API 持续漂移，作为长期架构蓝本不稳定
- **保留价值（模块级借鉴，不引入运行时）**:
  - skills 生态：6 级加载优先级、SKILL.md 兼容（并入 C-2，结论不变）
  - 权限分级：`tools.exec.mode` 五模式 + 审批只能收紧 + 批准绑定 cwd/argv/文件快照（漂移即拒执）——L1-L4 分级语义的最细粒度参照
  - ClawHub 三态信任模型（malicious/risky/official）——下载校验门参考（并入 C-2，结论不变）

### 附: 面试备选叙事（Copilot 文档, github/docs, CC-BY-4.0）——沿用首轮

- "Confidence 阈值 + 批量确认（Accept all/Decline all）+ 'approvals 是流程便利不是安全控制'"（content/copilot/concepts/agents/cloud-agent/about-automation-rationale-and-approvals.md）→ 我们 L3 建议层与 L2 批量确认 UX 的对标；活人感差异化同上

## 关键事实核查记录（2026-08-03，均为一手来源实读）

| # | 事实 | 证据 |
|---|------|------|
| 1 | **OpenClaw License 实为 MIT**（GitHub API 标 NOASSERTION，因 LICENSE 文件后附 THIRD_PARTY_NOTICES 引注段） | openclaw/openclaw `LICENSE` 文件全文实读（MIT License, Copyright (c) 2026 OpenClaw Foundation） |
| 2 | **Modrinth App 是 GPL-3.0-only，坊间"MIT"传闻不实** | modrinth/code `apps/app/LICENSE` 全文（GPL-3.0）、`apps/app/Cargo.toml` `license = "GPL-3.0-only"`、COPYING.md 声明 |
| 3 | theseus 库（Modrinth 安装内核）随 monorepo 一并 GPL-3.0 | modrinth/code `packages/app-lib/COPYING.md`（原独立仓库时代为 MIT 的历史不适用于当前代码） |
| 4 | GDLauncher v1 已停更（最后推送 2024-06-14）；Carbon 为 BSL-1.1（非开源许可，Change License GPL-3.0） | gorilla-devs/GDLauncher API `pushed_at: 2024-06-14`；GDLauncher-Carbon `LICENSE` 实读（BSL-1.1 + Additional Use Grant: None） |
| 5 | CurseForge App 闭源，无官方开源仓库 | GitHub 搜索（2026-08-03）无一手来源 |
| 6 | anthropics/skills 无顶层 LICENSE，各技能自含许可（多数 Apache-2.0；docx/pdf/pptx/xlsx 为 source-available） | anthropics/skills API license 字段 null；README 声明 |
| 7 | MCP 规范 License 处 MIT→Apache-2.0 过渡期（新旧贡献混合，文档贡献 CC-BY-4.0） | modelcontextprotocol/modelcontextprotocol `LICENSE` 实读 |
| 8 | Mastodon AGPL-3.0（强传染） | API license 字段 |
| 9 | wallabag 后端 MIT；Android app 仓库为 wallabag/android-app（GPL-3.0，非 wallabag-android） | API 核实；`wallabag-android` 仓库已不存在（404） |
| 10 | Cline/Continue 均为 Apache-2.0 | API license 字段 |
| 11 | GitHub Copilot 文档一手来源 = github/docs 仓库（CC-BY-4.0，★20.6k） | API license 字段；docs.github.com 在线路径漂移（404）改用仓库源码 |
| 12 | **Cherry Studio License 实为 AGPL-3.0**（消费级客户端中的罕见选择；模式借鉴须不拷贝代码） | CherryHQ/cherry-studio `LICENSE` 全文实读（GNU AGPL v3）；API license 字段 AGPL-3.0 |
| 13 | **LobeChat 仓库已改名 lobehub/lobehub**（★81.2k）；License 为 **LobeHub Community License**（Apache-2.0 + 附加条款：不改源码的商用免费、衍生作品商用须授权、贡献者代码可被商业使用）→ 非纯 Apache-2.0 | API 重定向（full_name: lobehub/lobehub）；`LICENSE` 实读 |
| 14 | **LobeHub 桌面端为 Electron**（apps/desktop 用 electron-builder），非 Tauri | apps/desktop/package.json 实读（electron-builder scripts；无 tauri.conf.json） |
| 15 | **Cherry Studio 内嵌 OpenClaw**：OpenClawService 管理本地/远程网关（默认端口 18790、auth token、openclaw.json 读写）；Skills 系统支持 ClawHub 技能（`ClawhubSkillDetailSchema`）+ claude-plugins.dev 源 | src/main/services/OpenClawService.ts、src/main/ai/skills/SkillService.ts 实读 |
| 16 | **Cherry Studio 为 Electron 栈**（electron.vite.config.ts + electron-builder.yml），非 Tauri | 仓库顶层实读 |
| 17 | **Jan License 为 Apache-2.0**（Copyright 2025 Menlo Research，附署名请求条款）；Tauri 2 桌面+移动（iOS/Android，`tauri ios dev` / `tauri android build`） | janhq/jan `LICENSE` 实读；package.json scripts 实读 |
| 18 | **Chatbox 仓库已改名 chatboxai/chatbox，License 为 GPL-3.0**（非坊间常见印象）；Electron + React | API 重定向；`LICENSE` 全文实读（GNU GPL v3） |
| 19 | **AnythingLLM 为 MIT**；其"skills" = 应用连接器（filesystem/gmail/calendar/outlook 等管理员面板配置 + 每 agent 子技能偏好），非 SKILL.md 市场 | API license 字段；frontend/src/components/.../AgentSkills/skillRegistry.js 实读 |
| 20 | **Ollama 有 Go 原生 skills 系统**：SKILL.md + frontmatter（name 1-64 小写+连字符、description 非空、目录名=技能名）；扫描 `~/.ollama/skills` + 跨客户端 `.agents/skills/` + 项目级 `.ollama/skills`；maxSkillBytes 1MB；内置 skill-creator；原则声明"技能仅指令，不授予文件系统/网络/shell/审批权限" | ollama/ollama `agent/skills.go` 实读 |
| 21 | **Ollama 桌面 = Go + 内嵌 WebView + React**（app/ 目录：webview/webview.go、ui/、wintray、updater），非 Tauri，单二进制形态 | app/README.md + app/webview/* 实读 |
| 22 | **Open WebUI License 非标准**：BSD-3-Clause 基础 + 品牌保留条款（总用户 ≤50 的部署方可去品牌，否则须保留或获书面许可） | open-webui/open-webui `LICENSE` 实读 |
| 23 | LobeHub 插件/agent 市场 = 独立索引仓库（lobehub/lobe-chat-plugins ★298、lobehub/lobe-chat-agents ★1,191），`plugins/plugin-template.json` 条目 + PR 提交 + 站内浏览 | 仓库 README + API 核实 |
| 24 | Cherry Studio"300+ 预置助手"、LobeHub"10,000+ Skills"为 README 宣传口径，数量未独立核实 | 双方 README 实读 |

## 调研缺口（未找到答案的问题）

1. **无 LLM 降级路径**：A 类四候选（Cline/Continue/OpenClaw/Copilot）全部强依赖 LLM，无"规则/检索直连"降级先例 → 我们 ADR 的"无 LLM 演示路径"无开源参考，需自研（已按"差异化点"处理）
2. **下载完整性校验**：Vortex/Modrinth App 未发现下载后 hash 校验/签名验证的明确实现（依赖平台 API 下载 + 版本锁定）→ L4 的签名 grant（HMAC→Ed25519）仍无开源可抄，保持自研路线
3. **移动端批量操作的后台编排**：Mastodon 有 worker 队列先例（UnfavouriteWorker）但无"agent 批量收藏 + 移动端后台限制"成熟方案；离线配额（Pocket 式 500 上限）无开源实现 → 配额机制需自研（对齐 M2 隐性配额）
4. **Claude Code 官方 skills 使用文档**：code.claude.com / support.claude.com 抓取超时不可达 → 以 agentskills.io 规格（更权威）替代，不构成实质缺口
5. **Vortex 游戏目录发现的完整实现细节**：确认存在 per-game 扩展 + `util.steam.allGames()` 读 Steam 库（gameinfo-steam/src/index.ts），但 registry 读取等其他路径未逐一核实源码
6. **Vortex/Modrinth 冲突检测算法**（mod 覆盖/依赖冲突判定）未深入源码 → 若需实现"冲突检测"动作集，需二轮专项调研
7. **消费级权限确认深度**（二轮新缺口）：A-5~A-11 七个消费级候选均无签名/凭证级授权实现，最高只到工具开关/每次询问/插件安装确认 → L4 签名 grant（HMAC→Ed25519）仍无开源可抄，自研路线不变（这同时是我们的差异化点）
8. **Cherry Studio 架构重构中**（仓库含 `v2-refactor-temp/` 目录）→ 其代码锚点可能漂移，借鉴以产品形态与模式为准，不以具体路径为准

## 交付与后续

- 本文件填充完成后 → wayfinder 生成长期决策地图（平台选型、技能格式、动作集扩展、规格修订任务）
- 面试素材：每个候选提炼"它解决了什么问题 → 我们怎么适配 → 我们的差异化（活人感约束）"三句话叙事（主/辅助参考叙事见「总体主推荐」，其他候选可同理套用）
- 相关规格修订（修订 `2026-07-16-omnicraft-dual-surface-agent-productization-design.md` §4.2 只读→L2 安全写）需在 wayfinder 计划内串行登记
