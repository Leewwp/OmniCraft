# 安全策略

## 支持的版本

当前处于积极开发阶段，安全修复只针对最新 `main` 分支。

## 报告漏洞

**请勿通过公开 Issue、讨论区或 PR 披露安全漏洞。**

请使用 GitHub 私有漏洞报告通道：

1. 进入仓库页面 → **Security** 标签 → **Report a vulnerability**；
2. 或直接访问 `https://github.com/Leewwp/OmniCraft/security/advisories/new`。

报告时请尽量包含：

- 问题类型（如越权访问、注入、信息泄露、依赖漏洞利用路径）；
- 复现步骤与最小化 PoC；
- 影响范围与您认为的严重程度；
- 受影响的 commit / 版本。

我们会在收到报告后尽快确认，并通过同一渠道反馈处理进展。修复发布前请勿公开披露细节。

## 安全基线（贡献者必读）

- 任何真实凭证、密钥、内网地址不得提交；`.env` 仅保留占位符模板；
- CI 安全门（`.github/workflows/security.yml`）在 PR、push 到 main 与每日定时运行：
  Go `govulncheck`、`npm audit`、`cargo audit`、gitleaks（工作树 + 全历史）、
  Trivy filesystem/IaC/container 扫描；
  secret 命中与 release 级 Critical 不可豁免；High 豁免必须登记
  `security/exceptions.json`（版本/digest、补偿控制、审批人与到期日）；
- 生产发布候选必须通过 `scripts/release/preflight.sh` 配置预检与 staging
  部署/回滚演练（`docs/deploy/single-server-beta-runbook.md`）；
- 归档文件（Mod 压缩包）经过结构校验 + ClamAV 扫描双门，隔离对象永不签名下载。
