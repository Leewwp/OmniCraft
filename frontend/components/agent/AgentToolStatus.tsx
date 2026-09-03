"use client";

import { useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { Check, ChevronDown, Loader2, Minus, X } from "lucide-react";
import type { AgentStreamTool } from "@/lib/agent-stream";
import { cn } from "@/lib/utils";

interface AgentToolStatusProps {
  tools: AgentStreamTool[];
  /** 流式中自动展开跟随新增步骤；完成（false）自动折叠（A-06 工具步骤区形态）。 */
  live?: boolean;
}

const TOOL_RESULT_KEYS: Record<string, Partial<Record<AgentStreamTool["status"], string>>> = {
  search_content: {
    running: "agent.tools.searchContentRunning",
    success: "agent.tools.searchContentSuccess",
    failed: "agent.tools.searchContentFailed",
    error: "agent.tools.searchContentFailed",
    skipped: "agent.tools.searchContentSkipped",
  },
  get_content_detail: {
    running: "agent.tools.getContentDetailRunning",
    success: "agent.tools.getContentDetailSuccess",
    failed: "agent.tools.getContentDetailFailed",
    error: "agent.tools.getContentDetailFailed",
    skipped: "agent.tools.getContentDetailSkipped",
  },
  get_usage_guide: {
    running: "agent.tools.getUsageGuideRunning",
    success: "agent.tools.getUsageGuideSuccess",
    failed: "agent.tools.getUsageGuideFailed",
    error: "agent.tools.getUsageGuideFailed",
    skipped: "agent.tools.getUsageGuideSkipped",
  },
  suggest_publish_metadata: {
    running: "agent.tools.suggestPublishMetadataRunning",
    success: "agent.tools.suggestPublishMetadataSuccess",
    failed: "agent.tools.suggestPublishMetadataFailed",
    error: "agent.tools.suggestPublishMetadataFailed",
    skipped: "agent.tools.suggestPublishMetadataSkipped",
  },
};

const FALLBACK_RESULT_KEYS: Record<AgentStreamTool["status"], string> = {
  running: "agent.tools.unknownRunning",
  success: "agent.tools.unknownSuccess",
  failed: "agent.tools.unknownFailed",
  error: "agent.tools.unknownFailed",
  skipped: "agent.tools.unknownSkipped",
};

function StatusIcon({ status }: { status: AgentStreamTool["status"] }) {
  if (status === "running") return <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin text-accent-emphasis" aria-hidden="true" />;
  if (status === "success") return <Check className="h-3.5 w-3.5 shrink-0 text-accent-emphasis" aria-hidden="true" />;
  if (status === "failed" || status === "error") return <X className="h-3.5 w-3.5 shrink-0 text-destructive" aria-hidden="true" />;
  return <Minus className="h-3.5 w-3.5 shrink-0 text-fg-muted" aria-hidden="true" />;
}

/**
 * 工具步骤区（A-06 三层生成形态第二层）：流式中展开跟随 → 完成自动折叠；
 * 每步展示用户向短文案 + 服务端派生的参数摘要（检索词/查询扩展词）与命中数、
 * 耗时（≥1s 时），不暴露 raw args、system prompt 或内部推理。
 */
export function AgentToolStatus({ tools, live = false }: AgentToolStatusProps) {
  const t = useTranslations();
  const [open, setOpen] = useState(live);
  const wasLive = useRef(live);

  useEffect(() => {
    if (wasLive.current && !live) setOpen(false);
    wasLive.current = live;
  }, [live]);

  if (tools.length === 0) return null;

  return (
    <div className="rounded-md border border-border-default bg-card">
      <button
        type="button"
        aria-expanded={open}
        aria-label={t("agent.tools.title")}
        onClick={() => setOpen((value) => !value)}
        className="flex w-full items-center gap-2 px-3 py-2 text-xs text-fg-muted transition-colors hover:bg-canvas-subtle focus:outline-none focus:ring-2 focus:ring-inset focus:ring-ring"
      >
        <StatusIcon status={tools.some((tool) => tool.status === "running") ? "running" : "success"} />
        <span className="font-medium">{t("agent.tools.stepsSummary", { count: tools.length })}</span>
        <ChevronDown
          className={cn(
            "ml-auto h-3.5 w-3.5 shrink-0 transition-transform duration-150",
            open && "rotate-180",
          )}
          aria-hidden="true"
        />
      </button>
      {open && (
        <ul className="space-y-1.5 border-t border-border-default px-3 py-2">
          {tools.map((tool, index) => {
            const labelKey =
              TOOL_RESULT_KEYS[tool.name]?.[tool.status] ?? FALLBACK_RESULT_KEYS[tool.status];
            const seconds =
              tool.duration_ms !== undefined && tool.duration_ms >= 1000
                ? Math.round(tool.duration_ms / 1000)
                : null;
            return (
              <li
                key={`${tool.name}-${index}`}
                className="flex flex-col gap-0.5 text-xs"
                aria-label={t(labelKey)}
              >
                <span className="flex items-center gap-1.5">
                  <StatusIcon status={tool.status} />
                  <span className="text-fg-default">{t(labelKey)}</span>
                  {tool.hits !== undefined && (
                    <span className="rounded-full border border-border-default px-1.5 py-0.5 text-fg-muted">
                      {t("agent.tools.hits", { count: tool.hits })}
                    </span>
                  )}
                  {seconds !== null && <span className="text-fg-muted">{t("agent.tools.duration", { seconds })}</span>}
                </span>
                {tool.args_summary && (
                  <span className="truncate pl-5 font-mono text-fg-muted" title={tool.args_summary}>
                    {tool.args_summary}
                  </span>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
