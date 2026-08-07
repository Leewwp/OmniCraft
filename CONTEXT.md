# OmniCraft context map

`docs/GLOSSARY.md` 是产品术语和定义的唯一权威。本文件只提供面向开发与测试的导航，不复制术语定义；涉及名称、边界或 Avoid 词汇时必须读取 Glossary 原文。

## Domain map

- 内容发现：推荐流、原创/二创分区、IP 库与 IP 详情页；决策入口见 `docs/GLOSSARY.md` 和 `docs/working/2026-08-04-content-discovery-gap-plan.md`。
- 内容浏览：所有卡片入口最终复用内容详情浮层，完整详情页保留给直达 URL；媒体集/媒体查看器/连续浏览/相关内容规范见 `docs/superpowers/specs/2026-08-08-omnicraft-media-experience-design.md`；路由决策见 `docs/working/2026-07-25-wayfinder-ticket-content-modal-routing.md`。
- Web Agent：顶部导航进入受保护的 `/agent` 全页工作台，全站搜索保持关键词职责；见 `docs/adr/0003-web-agent-dedicated-workspace.md`。
- 社区互动：用户资料浮层、私信聊天浮层和冷启动私信共享消息事实源；详细规则见 `docs/superpowers/specs/2026-06-29-omnicraft-community-features-design.md` §1。

## Test seams

- HTTP/API 行为通过 handler/router 边界验证，业务事务与缓存恢复通过 service 集成测试补充。
- 前端页面行为通过可访问角色、路由和用户可见状态验证，不依赖组件内部实现。
- 发布门通过统一 verifier、workflow contract 和 evidence schema 验证。

## Authority

权威冲突顺序与任务来源以根目录 `AGENTS.md` 为准；架构决策索引见 `docs/adr/README.md`。
