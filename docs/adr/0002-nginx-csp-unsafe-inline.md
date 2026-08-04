# ADR-0002: 单服务器 Nginx 允许 App Router 内联引导脚本

**状态**: accepted
**日期**: 2026-08-04
**决策者**: Overnight Agent

## 背景

2026-07-24 在单服务器 Web 部署（`docs/deploy/nginx.omnicraft.single-server.conf`）为 App vhost 启用 CSP 时发现：Next.js App Router（生产构建）会为每个页面注入内联 bootstrap payload（RSC flight data、hydration script），这些内联脚本没有 `nonce`/`hash` 属性。若 `script-src` 严格为 `'self'`，生产页面将全部白屏。

同时存在一个反例可对照：`api.leeppp.online` vhost 不承载 Next.js 页面，其 CSP 保持严格 `script-src 'self'`。

## 决策

App vhost 的 `script-src` 采用 `'self' 'unsafe-inline'`；`style-src` 同样保留 `'unsafe-inline'`（Tailwind/样式内联属 App Router 既有行为）。其余指令保持严格：`default-src 'self'`、`img-src 'self' data: https:`、`connect-src 'self' https://api.leeppp.online`、`font-src 'self'`、无 `object-src` 兜底于 default。

不采用 nonce 方案的代价：nonce 需由应用在每次响应中生成并与 Nginx 静态头协作，当前单服务器静态配置无法下发动态 nonce；改为让 Next.js 完全接管 CSP 头会导致与 Nginx `add_header` 双头并存（浏览器取交集，实际效果更严而不可控）。为内联引导脚本引入 hash 清单则需要随每个部署构建产物动态更新 nginx 配置，运维代价与出错面不成比例。

## 后果

### 正面
- 生产页面在 CSP 启用下可正常渲染（此前为无 CSP）
- 与 API vhost 的严格策略分离，攻击面隔离

### 风险
- `'unsafe-inline'` 削弱了针对 XSS 注入内联脚本的防线；缓解措施：应用自身仍输出 `X-Frame-Options DENY`、`X-Content-Type-Options nosniff`，且该 vhost 仅承载自有渲染数据，无第三方脚本源
- 若未来引入第三方内联脚本，需重新评估（nonce/hash 迁移），届时应新建 ADR

### 迁移路径
- 若 Next.js 部署形态演进（如 standalone + 中间件 nonce 方案、或静态导出），将 CSP 收紧为 `'self' 'nonce-...'` 并更新本记录

## 参考

- 配置：`docs/deploy/nginx.omnicraft.single-server.conf`（App vhost 与 API vhost 两处 CSP）
- 引入提交：`15e6525`（Fix Next.js CSP on single-server web）
