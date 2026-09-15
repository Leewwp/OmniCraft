# 更新日志

本项目的显著变更记录于此文件。

格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [Semantic Versioning](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### Added

- Agent 接入层：匿名只读 REST / OpenAPI 3.1 / MCP（Streamable HTTP）三通道、
  PAT（download/upload scope）认证与 `agent-access/` Skill 包（#445~#451）。
- Agent IP 搜索工具（`search_ips`）与 IP 引用卡片（#523）。
- Markdown 编辑器（Milkdown/Crepe）：发布正文、讨论、PR、IP、反馈等多编辑位
  统一接入，内容详情 Markdown/纯文本双渲染（#520~#522）。
- 用户悬浮卡（创作者区/讨论区作者入口）与登录浮窗（登录成功自动续做原动作）（#489~#493）。
- 消息中心 B 站式三栏改造：私信会话列表、回复跳原内容锚点（#509）。
- 个人主页统计与收藏集 Tab（#508）。

### Changed

- Go 工具链升级 1.26（CI 固定 1.26.8），x/crypto v0.57 / grpc v1.83 安全修复（#497）。
- React 19 / react-dom 19.2 对齐升级（#502）。
- 仓库开源治理：部署模板与配置示例全面占位符化（域名/IP/邮箱），
  补齐 CONTRIBUTING / SECURITY / CODE_OF_CONDUCT 与 Issue/PR 模板。

### Security

- CI 安全门矩阵收紧：gitleaks 语义矩阵、verdict 对「module 级无修复版公告」
  降级 informational、High 豁免双人审批门（#499）。

---

更早的开发期变更未逐条追溯；首次正式发布（0.x）起将按版本分段记录。
