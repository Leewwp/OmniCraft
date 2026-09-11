"use client";

import { useTranslations } from "next-intl";
import { BookOpen, Plug, Puzzle, TerminalSquare, ShieldCheck, Gauge, GitBranch } from "lucide-react";

// SP-16 #448：外部 Agent 接入落地页（对标 aihot.news/agent 的三通道安装引导）。
// REST 通道随本票可用；MCP 与 Skill 通道为 P3 占位（上线后回填地址与脚本）。
export default function AgentAccessPage() {
  const t = useTranslations();

  const channels = [
    {
      icon: TerminalSquare,
      titleKey: "agentAccess.rest.title",
      statusKey: "agentAccess.status.live",
      live: true,
      points: [
        "agentAccess.rest.point1",
        "agentAccess.rest.point2",
        "agentAccess.rest.point3",
      ],
    },
    {
      icon: Plug,
      titleKey: "agentAccess.mcp.title",
      statusKey: "agentAccess.status.planned",
      live: false,
      points: [
        "agentAccess.mcp.point1",
        "agentAccess.mcp.point2",
      ],
    },
    {
      icon: Puzzle,
      titleKey: "agentAccess.skill.title",
      statusKey: "agentAccess.status.planned",
      live: false,
      points: [
        "agentAccess.skill.point1",
        "agentAccess.skill.point2",
      ],
    },
  ];

  return (
    <div className="mx-auto w-full max-w-4xl px-4 py-10 md:px-6">
        <div className="flex items-center gap-3">
          <BookOpen className="h-8 w-8 text-primary" />
          <div>
            <h1 className="text-2xl font-bold text-foreground">{t("agentAccess.hero.title")}</h1>
            <p className="mt-1 text-sm text-muted-foreground">{t("agentAccess.hero.subtitle")}</p>
          </div>
        </div>

        <section className="mt-8 grid gap-4 md:grid-cols-3" aria-label={t("agentAccess.channels.aria")}>
          {channels.map((ch) => (
            <div key={ch.titleKey} className="rounded-lg border border-border bg-card p-4">
              <div className="flex items-center justify-between">
                <ch.icon className="h-5 w-5 text-primary" />
                <span className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${ch.live ? "bg-primary/10 text-primary" : "bg-muted text-muted-foreground"}`}>
                  {t(ch.statusKey)}
                </span>
              </div>
              <h2 className="mt-3 text-sm font-semibold text-foreground">{t(ch.titleKey)}</h2>
              <ul className="mt-2 space-y-1.5">
                {ch.points.map((p) => (
                  <li key={p} className="flex gap-1.5 text-xs text-muted-foreground">
                    <span aria-hidden className="mt-1 h-1 w-1 shrink-0 rounded-full bg-muted-foreground/60" />
                    {t(p)}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </section>

        <section className="mt-8 rounded-lg border border-border bg-card p-5">
          <h2 className="text-sm font-semibold text-foreground">{t("agentAccess.verify.title")}</h2>
          <p className="mt-1 text-xs text-muted-foreground">{t("agentAccess.verify.subtitle")}</p>
          <pre className="mt-3 overflow-x-auto rounded-lg bg-canvas-subtle p-3 text-xs leading-relaxed text-foreground"><code>{`curl "https://app.example.com/api/v1/contents/search?q=` + t("agentAccess.verify.keyword") + `&page_size=5"

# 200 → {"items":[{"id":123,"title":"...","content_type":"sheet_music",...}],"total":1}
curl "https://app.example.com/api/v1/contents/123/guide"   # 使用指导（requirements/steps/safety）`}</code></pre>
        </section>

        <section className="mt-4 grid gap-4 md:grid-cols-2">
          <div className="rounded-lg border border-border bg-card p-5">
            <div className="flex items-center gap-2">
              <Gauge className="h-4 w-4 text-muted-foreground" />
              <h2 className="text-sm font-semibold text-foreground">{t("agentAccess.limits.title")}</h2>
            </div>
            <ul className="mt-2 space-y-1.5 text-xs text-muted-foreground">
              <li>{t("agentAccess.limits.point1")}</li>
              <li>{t("agentAccess.limits.point2")}</li>
              <li>{t("agentAccess.limits.point3")}</li>
            </ul>
          </div>
          <div className="rounded-lg border border-border bg-card p-5">
            <div className="flex items-center gap-2">
              <GitBranch className="h-4 w-4 text-muted-foreground" />
              <h2 className="text-sm font-semibold text-foreground">{t("agentAccess.versioning.title")}</h2>
            </div>
            <p className="mt-2 text-xs text-muted-foreground">{t("agentAccess.versioning.body")}</p>
          </div>
        </section>

        <section className="mt-4 rounded-lg border border-border bg-card p-5">
          <div className="flex items-center gap-2">
            <ShieldCheck className="h-4 w-4 text-muted-foreground" />
            <h2 className="text-sm font-semibold text-foreground">{t("agentAccess.boundary.title")}</h2>
          </div>
          <p className="mt-2 text-xs text-muted-foreground">{t("agentAccess.boundary.body")}</p>
        </section>
    </div>
  );
}
